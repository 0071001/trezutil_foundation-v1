// Package saspay implements domain.PaymentProviderAdapter against the SasPay REST API (https://docs.saspay.me):
// mobile money in DR Congo (M-Pesa through Vodacom, Airtel Money, Orange Money) and cards (USD, on SasPay's hosted
// page), through POST /payments/softpay/ and GET /payments/{id}/verify/.
//
// Trezutil's own SasPay account collects for every order (decision of 2026-11-24): its provider row is marked
// scope "platform" and serves any organisation that has no payment provider of its own (see provider.ProviderRegistry).
//
// SasPay documents no refund, cancel or capture for a collection: those operations are refused here, never simulated.
// A refund is done by the Trezutil team from the SasPay dashboard.
package saspay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ProviderCode is the code used in the providers table and webhook URLs.
const ProviderCode = "saspay"

// Networks Trezutil offers (DR Congo mobile money, and cards which SasPay takes in USD only).
var Networks = map[string]bool{"vodacom_cd": true, "airtel_cd": true, "orange_cd": true, "card": true}

// Config configures the adapter for one providers row.
type Config struct {
	ProviderID string
	// SecretKey is the SasPay secret key (sk_live_… or sk_test_…).
	SecretKey string
	// BaseURL defaults to https://api.saspay.me/api/v1 (sandbox: https://sandbox-api.saspay.me/api/v1).
	BaseURL string
	// Country is the ISO code of the mobile money networks (CD).
	Country string
	// ReturnURL is where SasPay's hosted page sends the payer after a successful card payment. A return URL sent by
	// the browser is used only when it starts with this one.
	ReturnURL  string
	Timeout    time.Duration
	HTTPClient *http.Client
}

// Adapter is a SasPay payment provider adapter.
type Adapter struct {
	cfg    Config
	client *http.Client
}

// NewAdapter creates a SasPay adapter.
func NewAdapter(cfg Config) (*Adapter, error) {
	if cfg.ProviderID == "" {
		return nil, errors.New("saspay: provider id is required")
	}
	if !strings.HasPrefix(cfg.SecretKey, "sk_") {
		return nil, errors.New("saspay: a secret key (sk_live_… or sk_test_…) is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.saspay.me/api/v1"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.Country == "" {
		cfg.Country = "CD"
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 30 * time.Second
		}
		client = &http.Client{Timeout: timeout}
	}
	return &Adapter{cfg: cfg, client: client}, nil
}

// ProviderID implements domain.ProviderAdapter.
func (a *Adapter) ProviderID() string { return a.cfg.ProviderID }

// Capabilities implements domain.ProviderAdapter.
func (a *Adapter) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapabilityPayment}
}

// Health implements domain.ProviderAdapter; request failures are classified per call.
func (a *Adapter) Health(ctx context.Context) domain.ProviderHealthStatus {
	return domain.ProviderHealthHealthy
}

type sasPayment struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	CheckoutURL  string `json:"checkout_url"`
	Instructions string `json:"instructions"`
	NetAmount    string `json:"net_amount"`
	Currency     string `json:"currency"`
}

type customer struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// CreatePayment pushes a mobile money payment to the payer's phone (they approve it with their PIN), or opens
// SasPay's hosted page for a card. PaymentMethodID is the network code; Metadata carries the payer
// (phone, email, first_name, last_name) and, for a card, return_url.
func (a *Adapter) CreatePayment(ctx context.Context, req domain.PaymentProviderRequest) (domain.PaymentProviderResponse, error) {
	network := strings.ToLower(strings.TrimSpace(req.PaymentMethodID))
	if !Networks[network] {
		return invalid("unknown network " + strconv.Quote(network))
	}
	if req.Amount.AmountMinor <= 0 || req.Amount.Currency == "" {
		return invalid("amount and currency are required")
	}
	currency := strings.ToUpper(req.Amount.Currency)
	if network == "card" && currency != "USD" {
		return invalid("cards are paid in USD only")
	}
	meta := func(k string) string {
		s, _ := req.Metadata[k].(string)
		return strings.TrimSpace(s)
	}
	c := customer{Email: meta("email"), FirstName: meta("first_name"), LastName: meta("last_name"), Phone: Phone(meta("phone"))}
	if c.Email == "" || c.FirstName == "" || c.LastName == "" || (network != "card" && c.Phone == "") {
		return invalid("the payer's email, first name, last name and phone are required")
	}
	body := map[string]interface{}{
		"amount":      decimal(req.Amount.AmountMinor),
		"currency":    currency,
		"country":     a.cfg.Country,
		"network":     network,
		"description": firstNonEmpty(meta("description"), "Commande Trezutil"),
		"customer":    c,
	}
	if ref := meta("reference"); ref != "" {
		body["metadata"] = map[string]string{"trezutil_reference": ref}
	}
	if network == "card" {
		ret := a.cfg.ReturnURL
		if r := meta("return_url"); r != "" && a.cfg.ReturnURL != "" && strings.HasPrefix(r, a.cfg.ReturnURL) {
			ret = r
		}
		if ret == "" {
			return invalid("a return URL is required for a card payment")
		}
		body["return_url"] = ret
	}
	var out sasPayment
	if err := a.do(ctx, http.MethodPost, "/payments/softpay/", body, req.IdempotencyKey, &out); err != nil {
		return failed(err), err
	}
	return a.response(out), nil
}

// GetPayment re-verifies a payment with SasPay (the documented source of truth for its status).
func (a *Adapter) GetPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	var out sasPayment
	if err := a.do(ctx, http.MethodGet, "/payments/"+url.PathEscape(providerPaymentID)+"/verify/", nil, "", &out); err != nil {
		return failed(err), err
	}
	if out.ID == "" {
		out.ID = providerPaymentID
	}
	return a.response(out), nil
}

var errUnsupported = &apiError{code: domain.ErrCodeInvalidRequest, message: "saspay invalid request: not supported by SasPay (do it from the SasPay dashboard)"}

// CancelPayment is not offered by SasPay for a collection.
func (a *Adapter) CancelPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	return failed(errUnsupported), errUnsupported
}

// CapturePayment: SasPay collections are captured when the payer approves them.
func (a *Adapter) CapturePayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.PaymentProviderResponse, error) {
	return failed(errUnsupported), errUnsupported
}

// RefundPayment: SasPay documents no refund API.
func (a *Adapter) RefundPayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.RefundProviderResponse, error) {
	return domain.RefundProviderResponse{ErrorCode: domain.ErrCodeInvalidRequest, ErrorMessage: errUnsupported.Error()}, errUnsupported
}

// Status maps SasPay statuses onto the Payment Core vocabulary.
func Status(s string) string {
	switch strings.ToUpper(s) {
	case "SUCCESS":
		return "captured"
	case "FAILED":
		return "failed"
	case "CANCELLED", "CANCELED":
		return "cancelled"
	default: // PENDING
		return "pending"
	}
}

func (a *Adapter) response(p sasPayment) domain.PaymentProviderResponse {
	data := map[string]interface{}{"saspay_status": p.Status, "provider_code": ProviderCode}
	if p.CheckoutURL != "" {
		data["checkout_url"] = p.CheckoutURL
	}
	if p.Instructions != "" {
		data["instructions"] = p.Instructions
	}
	return domain.PaymentProviderResponse{ProviderPaymentID: p.ID, Status: Status(p.Status), Data: data}
}

// Phone puts a Congolese number in the international form SasPay expects (+243…); other numbers are left as given.
func Phone(s string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	switch {
	case digits == "":
		return ""
	case strings.HasPrefix(strings.TrimSpace(s), "+"):
		return "+" + digits
	case strings.HasPrefix(digits, "00"):
		return "+" + digits[2:]
	case strings.HasPrefix(digits, "243") && len(digits) == 12:
		return "+" + digits
	case strings.HasPrefix(digits, "0") && len(digits) == 10:
		return "+243" + digits[1:]
	case len(digits) == 9:
		return "+243" + digits
	}
	return "+" + digits
}

// decimal writes minor units as SasPay's decimal string ("2500.00").
func decimal(minor int64) string {
	return fmt.Sprintf("%d.%02d", minor/100, minor%100)
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

// apiError is a normalized SasPay error. Its wording drives the orchestrator's retry classification ("temp
// unavailable"/"timeout" are retried, safe with an idempotency key; "invalid request"/"unauthorized" are not).
type apiError struct {
	status int
	code   domain.ProviderErrorCode
	// Code is SasPay's own error code (e.g. no_exchange_rate, invalid_method), kept for the payer-facing message.
	Code    string
	message string
}

func (e *apiError) Error() string { return e.message }

func invalid(msg string) (domain.PaymentProviderResponse, error) {
	err := &apiError{code: domain.ErrCodeInvalidRequest, message: "saspay invalid request: " + msg}
	return failed(err), err
}

func failed(err error) domain.PaymentProviderResponse {
	resp := domain.PaymentProviderResponse{Status: "failed", ErrorMessage: err.Error(), ErrorCode: domain.ErrCodeProviderRejected}
	var se *apiError
	if errors.As(err, &se) {
		resp.ErrorCode = se.code
	}
	return resp
}

func (a *Adapter) do(ctx context.Context, method, path string, body interface{}, idempotencyKey string, out *sasPayment) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.SecretKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		if len(idempotencyKey) > 255 {
			idempotencyKey = idempotencyKey[:255]
		}
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Timeout") {
			return &apiError{code: domain.ErrCodeProviderTimeout, message: "saspay timeout: " + sanitize(err.Error())}
		}
		return &apiError{code: domain.ErrCodeProviderUnavailable, message: "saspay temp unavailable: " + sanitize(err.Error())}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &envelope)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && envelope.Success {
		if err := json.Unmarshal(envelope.Data, out); err != nil {
			return &apiError{status: resp.StatusCode, code: domain.ErrCodeProviderRejected, message: "saspay invalid request: malformed response"}
		}
		return nil
	}
	e := &apiError{status: resp.StatusCode, Code: envelope.Error.Code}
	detail := envelope.Error.Code
	if detail == "" {
		detail = sanitize(envelope.Error.Message)
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		e.code, e.message = domain.ErrCodeProviderAuthFailed, "saspay unauthorized: check the configured secret key and its scope ("+detail+")"
	case resp.StatusCode == http.StatusTooManyRequests:
		e.code, e.message = domain.ErrCodeRateLimited, "saspay temp unavailable: rate limited"
	case resp.StatusCode >= 500:
		e.code, e.message = domain.ErrCodeProviderUnavailable, fmt.Sprintf("saspay temp unavailable: status %d", resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		e.code, e.message = domain.ErrCodeResourceNotFound, "saspay invalid request: not found"
	case resp.StatusCode == http.StatusConflict:
		e.code, e.message = domain.ErrCodeConflict, "saspay invalid request: conflict ("+detail+")"
	default:
		e.code, e.message = domain.ErrCodeInvalidRequest, "saspay invalid request: "+detail
	}
	return e
}

// sanitize keeps provider messages short and strips anything that looks like a key.
func sanitize(s string) string {
	for _, prefix := range []string{"sk_live_", "sk_test_", "Bearer "} {
		if i := strings.Index(s, prefix); i >= 0 {
			s = s[:i] + prefix + "***"
		}
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

var _ domain.PaymentProviderAdapter = (*Adapter)(nil)
