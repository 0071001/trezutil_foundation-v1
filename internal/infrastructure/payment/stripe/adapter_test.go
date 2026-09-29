package stripe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe/stripefake"
)

const testKey = "sk_test_trezutil"

func newTestAdapter(t *testing.T, capture string) (*Adapter, *stripefake.Server) {
	t.Helper()
	fake := stripefake.New(testKey)
	t.Cleanup(fake.Close)
	a, err := NewAdapter(Config{ProviderID: "prov-1", SecretKey: testKey, PublishableKey: "pk_test_x", BaseURL: fake.URL, CaptureMethod: capture, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return a, fake
}

func usd(n int64) domain.Money { return domain.Money{AmountMinor: n, Currency: "USD"} }

func TestAdapter_CreateConfirmedAutomaticCapture(t *testing.T) {
	a, fake := newTestAdapter(t, "automatic")
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(1999), PaymentMethodID: "pm_card_visa", IdempotencyKey: "k1"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Status != "captured" || resp.ProviderPaymentID == "" {
		t.Fatalf("unexpected response %+v", resp)
	}
	pi, _ := fake.Intent(resp.ProviderPaymentID)
	if pi.Amount != 1999 || pi.Currency != "usd" {
		t.Fatalf("amount/currency not sent as minor units / lowercase: %+v", pi)
	}
	// Same idempotency key -> same intent (no second charge).
	again, _ := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(1999), PaymentMethodID: "pm_card_visa", IdempotencyKey: "k1"})
	if again.ProviderPaymentID != resp.ProviderPaymentID {
		t.Fatal("idempotent create returned a different intent")
	}
	if fake.CallCount("POST /v1/payment_intents") != 2 {
		t.Fatalf("expected 2 create calls, got %d", fake.CallCount("POST /v1/payment_intents"))
	}
}

func TestAdapter_ManualCaptureAndRefund(t *testing.T) {
	a, _ := newTestAdapter(t, "manual")
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(5000), PaymentMethodID: "pm_card_visa", IdempotencyKey: "k2"})
	if err != nil || resp.Status != "authorized" {
		t.Fatalf("expected authorized, got %+v %v", resp, err)
	}
	amt := usd(5000)
	captured, err := a.CapturePayment(context.Background(), resp.ProviderPaymentID, &amt)
	if err != nil || captured.Status != "captured" {
		t.Fatalf("capture: %+v %v", captured, err)
	}
	if _, err := a.RefundPayment(context.Background(), resp.ProviderPaymentID, &amt); err == nil {
		t.Fatal("refund without idempotency key must be refused")
	}
	ctx := domain.WithProviderIdempotencyKey(context.Background(), "refund-1")
	rf, err := a.RefundPayment(ctx, resp.ProviderPaymentID, &amt)
	if err != nil || rf.Status != "succeeded" || rf.Amount.AmountMinor != 5000 {
		t.Fatalf("refund: %+v %v", rf, err)
	}
	// Replayed refund (same key) must not refund twice.
	rf2, err := a.RefundPayment(ctx, resp.ProviderPaymentID, &amt)
	if err != nil || rf2.ProviderRefundID != rf.ProviderRefundID {
		t.Fatalf("refund replay not idempotent: %+v %v", rf2, err)
	}
	// Over-refund is rejected by the provider.
	if _, err := a.RefundPayment(domain.WithProviderIdempotencyKey(context.Background(), "refund-2"), resp.ProviderPaymentID, &amt); err == nil {
		t.Fatal("over-refund must fail")
	}
}

func TestAdapter_DeclineIsDefinitive(t *testing.T) {
	a, _ := newTestAdapter(t, "automatic")
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(100), PaymentMethodID: "pm_card_chargeDeclined", IdempotencyKey: "k3"})
	if !errors.Is(err, domain.ErrProviderDeclined) {
		t.Fatalf("expected ErrProviderDeclined, got %v", err)
	}
	if resp.ProviderPaymentID == "" || resp.Status != "failed" {
		t.Fatalf("decline should keep the intent id and report failed: %+v", resp)
	}
	if !strings.Contains(err.Error(), "invalid request") {
		t.Fatal("declines must classify as non-retryable")
	}
}

func TestAdapter_TransientErrorsAreRetryableAndNeverLeakKey(t *testing.T) {
	a, fake := newTestAdapter(t, "automatic")
	_, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(100), PaymentMethodID: "pm_5xx", IdempotencyKey: "k4"})
	if err == nil || !strings.Contains(err.Error(), "temp unavailable") {
		t.Fatalf("5xx must be retryable, got %v", err)
	}
	fake.Delay = 3 * time.Second
	_, err = a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(100), PaymentMethodID: "pm_timeout", IdempotencyKey: "k5"})
	if err == nil || !(strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "temp unavailable")) {
		t.Fatalf("timeout must be retryable, got %v", err)
	}
	bad, _ := NewAdapter(Config{ProviderID: "p", SecretKey: "sk_test_wrong_secret", BaseURL: fake.URL})
	_, err = bad.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(100), IdempotencyKey: "k6"})
	if err == nil || strings.Contains(err.Error(), "wrong_secret") || !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("auth failure must be non-retryable and never echo the key: %v", err)
	}
}

func TestAdapter_ClientSecretForBrowserConfirmation(t *testing.T) {
	a, _ := newTestAdapter(t, "automatic")
	resp, err := a.CreatePayment(context.Background(), domain.PaymentProviderRequest{Amount: usd(100), PaymentMethodID: "card", IdempotencyKey: "k7"})
	if err != nil || resp.Status != "pending" {
		t.Fatalf("expected pending intent, got %+v %v", resp, err)
	}
	if resp.Data["client_secret"] == nil || resp.Data["publishable_key"] != "pk_test_x" {
		t.Fatalf("expected client action data, got %+v", resp.Data)
	}
}
