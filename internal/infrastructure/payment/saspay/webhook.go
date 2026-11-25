package saspay

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

// SignatureTolerance is SasPay's documented replay window.
const SignatureTolerance = 5 * time.Minute

// WebhookNormalizer verifies SasPay webhooks and maps their events onto Payment Core's WebhookPayload.
//
// SasPay signs each delivery with HMAC-SHA256(signing_secret, "{X-Webhook-Timestamp}.{raw body}") in hex, sent in
// X-Webhook-Signature. The HTTP handler passes both headers as "t=<timestamp>,v1=<signature>".
type WebhookNormalizer struct {
	// SecretFor returns the webhook signing secret configured for an organisation.
	SecretFor func(orgID string) string
	Now       func() time.Time
}

type event struct {
	Event string `json:"event"`
	Data  struct {
		ID        string          `json:"id"`
		Reference string          `json:"reference"`
		Status    string          `json:"status"`
		Charged   json.RawMessage `json:"charged"`
		Amount    json.RawMessage `json:"amount"`
		Currency  string          `json:"currency"`
	} `json:"data"`
}

// Normalize implements payment.WebhookNormalizer. A nil payload with a nil error means the event is not about a
// payment's outcome and is acknowledged.
func (n *WebhookNormalizer) Normalize(orgID, signature string, body []byte) (*payment.WebhookPayload, error) {
	secret := ""
	if n.SecretFor != nil {
		secret = n.SecretFor(orgID)
	}
	if secret == "" || n.verify(secret, signature, body) != nil {
		return nil, domain.ErrWebhookSignatureInvalid
	}
	var ev event
	if err := json.Unmarshal(body, &ev); err != nil || ev.Event == "" {
		return nil, errors.New("saspay webhook: malformed event")
	}
	p := &payment.WebhookPayload{ProviderPaymentID: ev.Data.ID, Currency: strings.ToUpper(ev.Data.Currency)}
	switch ev.Event {
	case "transaction.success":
		p.EventType, p.Status = string(domain.WebhookEventPaymentSucceeded), "captured"
	case "transaction.failed":
		p.EventType, p.Status = string(domain.WebhookEventPaymentFailed), "failed"
	case "transaction.cancelled":
		p.EventType, p.Status = string(domain.WebhookEventPaymentCanceled), "cancelled"
	default: // transaction.created, settlements, wallet transfers, webhook.test
		return nil, nil
	}
	if ev.Data.ID == "" {
		return nil, errors.New("saspay webhook: event without a transaction id")
	}
	// One outcome per transaction: SasPay's retries of the same delivery are recognised as duplicates.
	p.ProviderMessageID = ev.Event + ":" + ev.Data.ID
	return p, nil
}

func (n *WebhookNormalizer) verify(secret, header string, body []byte) error {
	var ts int64
	var sig string
	for _, part := range strings.Split(header, ",") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch k {
		case "t":
			ts, _ = strconv.ParseInt(v, 10, 64)
		case "v1":
			sig = strings.ToLower(v)
		}
	}
	if ts == 0 || sig == "" {
		return errors.New("missing timestamp or signature")
	}
	now := time.Now
	if n.Now != nil {
		now = n.Now
	}
	if d := now().Sub(time.Unix(ts, 0)); d > SignatureTolerance || d < -SignatureTolerance {
		return errors.New("timestamp outside tolerance")
	}
	if !hmac.Equal([]byte(Sign(secret, ts, body)), []byte(sig)) {
		return errors.New("signature mismatch")
	}
	return nil
}

// Sign computes SasPay's signature (tests and local harness; production deliveries are signed by SasPay).
func Sign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10)))
	mac.Write([]byte{'.'})
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
