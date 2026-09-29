// Package stripe implements domain.PaymentProviderAdapter against the Stripe
// REST API (PaymentIntents + Refunds) without the Stripe SDK (decision D4 in
// docs/sprint-20/MONETIZATION_ARCHITECTURE.md).
//
// Only tokenized payment-method references (pm_…) and PaymentIntent IDs pass
// through Trezutil; card data is collected by Stripe.js in the browser and
// never reaches Trezutil servers.
package stripe

import (
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
const ProviderCode = "stripe"

// Config configures the adapter for one Trezutil IntegrationProvider.
type Config struct {
	// ProviderID is the IntegrationProvider.ID this adapter serves.
	ProviderID string
	// SecretKey is the Stripe secret API key (sk_live_… / sk_test_…).
	SecretKey string
	// PublishableKey (pk_…) is public; it is handed to the paying browser
	// together with the PaymentIntent client secret for Stripe.js.
	PublishableKey string
	// BaseURL defaults to https://api.stripe.com.
	BaseURL string
	// CaptureMethod is "automatic" (default: charge on confirmation) or
	// "manual" (authorize now, capture later by organization staff).
	CaptureMethod string
	Timeout       time.Duration
	HTTPClient    *http.Client
}

// Adapter is a Stripe payment provider adapter.
type Adapter struct {
	cfg    Config
	client *http.Client
}

// NewAdapter creates a Stripe adapter.
func NewAdapter(cfg Config) (*Adapter, error) {
	if cfg.ProviderID == "" {
		return nil, errors.New("stripe: provider id is required")
	}
	if cfg.SecretKey == "" {
		return nil, errors.New("stripe: secret key is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.stripe.com"
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.CaptureMethod != "manual" {
		cfg.CaptureMethod = "automatic"
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = 20 * time.Second
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

// Health implements domain.ProviderAdapter. Configuration is validated at
// construction; request failures are classified per call.
func (a *Adapter) Health(ctx context.Context) domain.ProviderHealthStatus {
	return domain.ProviderHealthHealthy
}

// paymentIntent is the subset of the Stripe PaymentIntent object used here.
type paymentIntent struct {
	ID               string `json:"id"`
	Status           string `json:"status"`
	Amount           int64  `json:"amount"`
	AmountReceived   int64  `json:"amount_received"`
	AmountCapturable int64  `json:"amount_capturable"`
	Currency         string `json:"currency"`
	ClientSecret     string `json:"client_secret"`
	LastPaymentError *struct {
		Code        string `json:"code"`
		DeclineCode string `json:"decline_code"`
		Message     string `json:"message"`
	} `json:"last_payment_error"`
}

type refundObject struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	PaymentIntent string `json:"payment_intent"`
	FailureReason string `json:"failure_reason"`
}

// CreatePayment creates a PaymentIntent. With a tokenized payment method
// (pm_…) it is confirmed server-side; otherwise the browser confirms it with
// Stripe.js using the returned client secret (Data["client_secret"]).
func (a *Adapter) CreatePayment(ctx context.Context, req domain.PaymentProviderRequest) (domain.PaymentProviderResponse, error) {
	if req.Amount.AmountMinor <= 0 || req.Amount.Currency == "" {
		return domain.PaymentProviderResponse{ErrorCode: domain.ErrCodeInvalidRequest}, errors.New("stripe invalid request: amount and currency are required")
	}
	form := url.Values{}
	form.Set("amount", strconv.FormatInt(req.Amount.AmountMinor, 10))
	form.Set("currency", strings.ToLower(req.Amount.Currency))
	form.Set("capture_method", a.cfg.CaptureMethod)
	if strings.HasPrefix(req.PaymentMethodID, "pm_") {
		form.Set("payment_method", req.PaymentMethodID)
		form.Set("confirm", "true")
		// Redirect-based methods need a return URL; server-side confirmation
		// is limited to methods that complete without one.
		form.Set("automatic_payment_methods[enabled]", "true")
		form.Set("automatic_payment_methods[allow_redirects]", "never")
	} else {
		form.Set("automatic_payment_methods[enabled]", "true")
	}
	for k, v := range req.Metadata {
		if s, ok := v.(string); ok && s != "" && len(metadataKeys(form)) < 20 {
			form.Set("metadata["+k+"]", s)
		}
	}

	var pi paymentIntent
	if err := a.post(ctx, "/v1/payment_intents", form, req.IdempotencyKey, &pi); err != nil {
		return a.failedResponse(pi, err), err
	}
	return a.intentResponse(pi), nil
}

// GetPayment retrieves a PaymentIntent.
func (a *Adapter) GetPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	var pi paymentIntent
	if err := a.do(ctx, http.MethodGet, "/v1/payment_intents/"+url.PathEscape(providerPaymentID), nil, "", &pi); err != nil {
		return a.failedResponse(pi, err), err
	}
	return a.intentResponse(pi), nil
}

// CancelPayment cancels an uncaptured PaymentIntent.
func (a *Adapter) CancelPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	var pi paymentIntent
	key := keyOr(ctx, "cancel:"+providerPaymentID)
	if err := a.post(ctx, "/v1/payment_intents/"+url.PathEscape(providerPaymentID)+"/cancel", url.Values{}, key, &pi); err != nil {
		return a.failedResponse(pi, err), err
	}
	return a.intentResponse(pi), nil
}

// CapturePayment captures an authorized PaymentIntent (manual capture).
func (a *Adapter) CapturePayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.PaymentProviderResponse, error) {
	form := url.Values{}
	if amount != nil && amount.AmountMinor > 0 {
		form.Set("amount_to_capture", strconv.FormatInt(amount.AmountMinor, 10))
	}
	var pi paymentIntent
	key := keyOr(ctx, "capture:"+providerPaymentID)
	if err := a.post(ctx, "/v1/payment_intents/"+url.PathEscape(providerPaymentID)+"/capture", form, key, &pi); err != nil {
		return a.failedResponse(pi, err), err
	}
	return a.intentResponse(pi), nil
}

// RefundPayment refunds a captured PaymentIntent fully or partially.
func (a *Adapter) RefundPayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.RefundProviderResponse, error) {
	form := url.Values{}
	form.Set("payment_intent", providerPaymentID)
	if amount != nil && amount.AmountMinor > 0 {
		form.Set("amount", strconv.FormatInt(amount.AmountMinor, 10))
	}
	// Refund requests must carry an idempotency key: without one a retried
	// refund could return money twice.
	key := domain.ProviderIdempotencyKey(ctx)
	if key == "" {
		return domain.RefundProviderResponse{ErrorCode: domain.ErrCodeInvalidRequest}, errors.New("stripe invalid request: refund requires an idempotency key")
	}
	var rf refundObject
	if err := a.post(ctx, "/v1/refunds", form, key, &rf); err != nil {
		code := domain.ErrCodeProviderRejected
		var se *apiError
		if errors.As(err, &se) {
			code = se.code
		}
		return domain.RefundProviderResponse{ErrorCode: code, ErrorMessage: err.Error()}, err
	}
	resp := domain.RefundProviderResponse{
		ProviderRefundID: rf.ID,
		Status:           refundStatus(rf.Status),
		Amount:           domain.Money{AmountMinor: rf.Amount, Currency: strings.ToUpper(rf.Currency)},
	}
	if amount != nil && resp.Amount.Currency == "" {
		resp.Amount.Currency = amount.Currency
	}
	return resp, nil
}

// intentStatus maps Stripe PaymentIntent statuses onto the Payment Core
// vocabulary understood by parseProviderStatus.
func intentStatus(s string) string {
	switch s {
	case "requires_capture":
		return "authorized"
	case "succeeded":
		return "captured"
	case "canceled":
		return "cancelled"
	case "requires_payment_method", "requires_confirmation", "requires_action", "processing":
		return "pending"
	default:
		return s
	}
}

func refundStatus(s string) string {
	switch s {
	case "succeeded":
		return "succeeded"
	case "failed", "canceled":
		return "failed"
	default: // pending, requires_action
		return "pending"
	}
}

func (a *Adapter) intentResponse(pi paymentIntent) domain.PaymentProviderResponse {
	data := map[string]interface{}{"stripe_status": pi.Status}
	if pi.ClientSecret != "" && (pi.Status == "requires_payment_method" || pi.Status == "requires_confirmation" || pi.Status == "requires_action") {
		// Returned to the paying customer only, so Stripe.js can collect and
		// confirm the payment method; never persisted.
		data["client_secret"] = pi.ClientSecret
		data["provider_code"] = ProviderCode
		if a.cfg.PublishableKey != "" {
			data["publishable_key"] = a.cfg.PublishableKey
		}
	}
	return domain.PaymentProviderResponse{ProviderPaymentID: pi.ID, Status: intentStatus(pi.Status), Data: data}
}

func (a *Adapter) failedResponse(pi paymentIntent, err error) domain.PaymentProviderResponse {
	resp := domain.PaymentProviderResponse{ProviderPaymentID: pi.ID, Status: "failed", ErrorMessage: err.Error(), ErrorCode: domain.ErrCodeProviderRejected}
	var se *apiError
	if errors.As(err, &se) {
		resp.ErrorCode = se.code
		if se.intentID != "" {
			resp.ProviderPaymentID = se.intentID
		}
	}
	return resp
}

// apiError is a normalized Stripe API error. Its message wording drives the
// orchestrator's retry classification: "temp unavailable"/"timeout" are
// retried (safe because every mutation carries an idempotency key), "invalid
// request"/"unauthorized" are not.
type apiError struct {
	status   int
	code     domain.ProviderErrorCode
	message  string
	intentID string
	declined bool
}

func (e *apiError) Error() string { return e.message }

// Unwrap exposes definitive declines to Payment Core.
func (e *apiError) Unwrap() error {
	if e.declined {
		return domain.ErrProviderDeclined
	}
	return nil
}

func (a *Adapter) post(ctx context.Context, path string, form url.Values, idempotencyKey string, out interface{}) error {
	return a.do(ctx, http.MethodPost, path, form, idempotencyKey, out)
}

func (a *Adapter) do(ctx context.Context, method, path string, form url.Values, idempotencyKey string, out interface{}) error {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, body)
	if err != nil {
		return err
	}
	req.SetBasicAuth(a.cfg.SecretKey, "")
	req.Header.Set("Stripe-Version", "2024-06-20")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := a.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "Timeout") {
			return &apiError{code: domain.ErrCodeProviderTimeout, message: "stripe timeout: " + sanitize(err.Error())}
		}
		return &apiError{code: domain.ErrCodeProviderUnavailable, message: "stripe temp unavailable: " + sanitize(err.Error())}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if err := json.Unmarshal(raw, out); err != nil {
			return &apiError{status: resp.StatusCode, code: domain.ErrCodeProviderRejected, message: "stripe invalid request: malformed response"}
		}
		return nil
	}
	return decodeError(resp.StatusCode, raw, out)
}

func decodeError(status int, raw []byte, out interface{}) error {
	var body struct {
		Error struct {
			Type          string         `json:"type"`
			Code          string         `json:"code"`
			DeclineCode   string         `json:"decline_code"`
			Message       string         `json:"message"`
			PaymentIntent *paymentIntent `json:"payment_intent"`
		} `json:"error"`
	}
	_ = json.Unmarshal(raw, &body)
	e := &apiError{status: status}
	detail := body.Error.Code
	if body.Error.DeclineCode != "" {
		detail += "/" + body.Error.DeclineCode
	}
	if body.Error.PaymentIntent != nil {
		e.intentID = body.Error.PaymentIntent.ID
		if pi, ok := out.(*paymentIntent); ok {
			*pi = *body.Error.PaymentIntent
		}
	}
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		e.code, e.message = domain.ErrCodeProviderAuthFailed, "stripe unauthorized: check the configured secret key"
	case status == http.StatusTooManyRequests:
		e.code, e.message = domain.ErrCodeRateLimited, "stripe temp unavailable: rate limited"
	case status >= 500:
		e.code, e.message = domain.ErrCodeProviderUnavailable, fmt.Sprintf("stripe temp unavailable: status %d", status)
	case status == http.StatusPaymentRequired || body.Error.Type == "card_error":
		e.code, e.message, e.declined = domain.ErrCodeProviderRejected, "stripe invalid request: payment declined ("+detail+")", true
	case status == http.StatusNotFound:
		e.code, e.message = domain.ErrCodeResourceNotFound, "stripe invalid request: not found"
	case status == http.StatusConflict:
		e.code, e.message = domain.ErrCodeConflict, "stripe invalid request: conflict ("+detail+")"
	default:
		e.code, e.message = domain.ErrCodeInvalidRequest, "stripe invalid request: "+sanitize(body.Error.Message)
	}
	return e
}

// sanitize keeps provider messages short and strips anything that looks like
// a key (Stripe never echoes full keys, but be defensive).
func sanitize(s string) string {
	for _, prefix := range []string{"sk_", "rk_", "pk_"} {
		if i := strings.Index(s, prefix); i >= 0 {
			s = s[:i] + prefix + "***"
		}
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func keyOr(ctx context.Context, def string) string {
	if k := domain.ProviderIdempotencyKey(ctx); k != "" {
		return k
	}
	return def
}

func metadataKeys(form url.Values) []string {
	var keys []string
	for k := range form {
		if strings.HasPrefix(k, "metadata[") {
			keys = append(keys, k)
		}
	}
	return keys
}

var _ domain.PaymentProviderAdapter = (*Adapter)(nil)
