package stripe

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/domain"
)

// DefaultSignatureTolerance bounds how old a signed event may be (replay window).
const DefaultSignatureTolerance = 5 * time.Minute

// WebhookNormalizer verifies Stripe-Signature headers and maps Stripe events
// onto Payment Core's provider-neutral WebhookPayload.
type WebhookNormalizer struct {
	// SecretFor returns the endpoint signing secret (whsec_…) for an organization.
	SecretFor func(orgID string) string
	Tolerance time.Duration
	Now       func() time.Time
}

type stripeEvent struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Created int64  `json:"created"`
	Data    struct {
		Object json.RawMessage `json:"object"`
	} `json:"data"`
}

type eventObject struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Status        string `json:"status"`
	Amount        int64  `json:"amount"`
	Currency      string `json:"currency"`
	PaymentIntent string `json:"payment_intent"`
}

// Normalize implements payment.WebhookNormalizer. A nil payload with a nil
// error means the event is irrelevant and should be acknowledged.
func (n *WebhookNormalizer) Normalize(orgID, signature string, body []byte) (*payment.WebhookPayload, error) {
	secret := ""
	if n.SecretFor != nil {
		secret = n.SecretFor(orgID)
	}
	if secret == "" {
		return nil, domain.ErrWebhookSignatureInvalid
	}
	if err := n.verify(secret, signature, body); err != nil {
		return nil, domain.ErrWebhookSignatureInvalid
	}

	var ev stripeEvent
	if err := json.Unmarshal(body, &ev); err != nil || ev.ID == "" {
		return nil, errors.New("stripe webhook: malformed event")
	}
	var obj eventObject
	if err := json.Unmarshal(ev.Data.Object, &obj); err != nil {
		return nil, errors.New("stripe webhook: malformed event object")
	}

	p := &payment.WebhookPayload{
		ProviderMessageID: ev.ID,
		Currency:          strings.ToUpper(obj.Currency),
		Timestamp:         ev.Created,
	}
	if obj.Amount > 0 {
		p.Amount = json.RawMessage(strconv.FormatInt(obj.Amount, 10))
	}

	switch ev.Type {
	case "payment_intent.succeeded":
		p.EventType, p.Status, p.ProviderPaymentID = string(domain.WebhookEventPaymentSucceeded), "captured", obj.ID
	case "payment_intent.amount_capturable_updated":
		p.EventType, p.Status, p.ProviderPaymentID = string(domain.WebhookEventPaymentSucceeded), "authorized", obj.ID
	case "payment_intent.payment_failed":
		p.EventType, p.Status, p.ProviderPaymentID = string(domain.WebhookEventPaymentFailed), "failed", obj.ID
	case "payment_intent.canceled":
		p.EventType, p.Status, p.ProviderPaymentID = string(domain.WebhookEventPaymentCanceled), "cancelled", obj.ID
	case "refund.created", "refund.updated", "charge.refund.updated":
		if obj.Object != "refund" {
			return nil, nil
		}
		switch obj.Status {
		case "succeeded":
			p.EventType, p.Status = string(domain.WebhookEventRefundSucceeded), "succeeded"
		case "failed", "canceled":
			p.EventType, p.Status = string(domain.WebhookEventRefundFailed), "failed"
		default:
			return nil, nil // still pending
		}
		p.ProviderRefundID, p.ProviderPaymentID = obj.ID, obj.PaymentIntent
	default:
		return nil, nil
	}
	return p, nil
}

// verify checks "t=<unix>,v1=<hex hmac(secret, t.body)>" within the tolerance.
func (n *WebhookNormalizer) verify(secret, header string, body []byte) error {
	var ts int64
	var sigs []string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			sigs = append(sigs, v)
		}
	}
	if ts == 0 || len(sigs) == 0 {
		return errors.New("missing timestamp or signature")
	}
	now := time.Now
	if n.Now != nil {
		now = n.Now
	}
	tolerance := n.Tolerance
	if tolerance <= 0 {
		tolerance = DefaultSignatureTolerance
	}
	if d := now().Sub(time.Unix(ts, 0)); d > tolerance || d < -tolerance {
		return errors.New("timestamp outside tolerance")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	for _, s := range sigs {
		if hmac.Equal([]byte(expected), []byte(s)) {
			return nil
		}
	}
	return errors.New("signature mismatch")
}

// Sign produces a Stripe-Signature header value. It is used by tests and the
// local E2E harness to emulate Stripe; production events are signed by Stripe.
func Sign(secret string, ts time.Time, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts.Unix(), 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return "t=" + strconv.FormatInt(ts.Unix(), 10) + ",v1=" + hex.EncodeToString(mac.Sum(nil))
}
