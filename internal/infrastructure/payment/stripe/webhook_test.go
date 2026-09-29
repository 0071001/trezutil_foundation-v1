package stripe

import (
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe/stripefake"
)

func TestWebhookNormalizer(t *testing.T) {
	n := &WebhookNormalizer{SecretFor: func(string) string { return "whsec_test" }}
	body := stripefake.Event("evt_1", "payment_intent.succeeded", map[string]interface{}{"id": "pi_1", "object": "payment_intent", "status": "succeeded", "amount": 500, "currency": "usd"})

	p, err := n.Normalize("org", Sign("whsec_test", time.Now(), body), body)
	if err != nil || p == nil {
		t.Fatalf("valid event rejected: %v", err)
	}
	if p.ProviderMessageID != "evt_1" || p.ProviderPaymentID != "pi_1" || p.Status != "captured" || p.EventType != string(domain.WebhookEventPaymentSucceeded) {
		t.Fatalf("unexpected mapping %+v", p)
	}

	if _, err := n.Normalize("org", Sign("whsec_other", time.Now(), body), body); err != domain.ErrWebhookSignatureInvalid {
		t.Fatalf("wrong secret must be rejected, got %v", err)
	}
	if _, err := n.Normalize("org", Sign("whsec_test", time.Now().Add(-10*time.Minute), body), body); err != domain.ErrWebhookSignatureInvalid {
		t.Fatalf("stale (replayed) signature must be rejected, got %v", err)
	}
	tampered := append([]byte{}, body...)
	tampered[len(tampered)-2] = ' '
	if _, err := n.Normalize("org", Sign("whsec_test", time.Now(), body), tampered); err != domain.ErrWebhookSignatureInvalid {
		t.Fatal("tampered body must be rejected")
	}
	if _, err := (&WebhookNormalizer{SecretFor: func(string) string { return "" }}).Normalize("org", Sign("", time.Now(), body), body); err != domain.ErrWebhookSignatureInvalid {
		t.Fatal("missing secret must reject every event")
	}

	ignored := stripefake.Event("evt_2", "customer.created", map[string]interface{}{"id": "cus_1", "object": "customer"})
	if p, err := n.Normalize("org", Sign("whsec_test", time.Now(), ignored), ignored); err != nil || p != nil {
		t.Fatalf("irrelevant events must be acknowledged without processing: %+v %v", p, err)
	}

	refund := stripefake.Event("evt_3", "refund.updated", map[string]interface{}{"id": "re_1", "object": "refund", "status": "succeeded", "amount": 200, "currency": "usd", "payment_intent": "pi_1"})
	p, err = n.Normalize("org", Sign("whsec_test", time.Now(), refund), refund)
	if err != nil || p.ProviderRefundID != "re_1" || p.ProviderPaymentID != "pi_1" || p.EventType != string(domain.WebhookEventRefundSucceeded) {
		t.Fatalf("refund mapping: %+v %v", p, err)
	}
}
