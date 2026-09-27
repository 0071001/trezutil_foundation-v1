package domain

import (
	"testing"
)

func TestNewRefund(t *testing.T) {
	r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)

	if r.TenantID != "org-1" {
		t.Errorf("expected tenant org-1, got %s", r.TenantID)
	}
	if r.PaymentID != "payment-1" {
		t.Errorf("expected payment payment-1, got %s", r.PaymentID)
	}
	if r.Status != RefundStatusPending {
		t.Errorf("expected status PENDING, got %s", r.Status)
	}
	if r.Amount.AmountMinor != 500 {
		t.Errorf("expected amount 500, got %d", r.Amount.AmountMinor)
	}
}

func TestRefund_Succeed(t *testing.T) {
	r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)

	if err := r.Succeed("refund_123", 1010, 1010); err != nil {
		t.Fatalf("succeed failed: %v", err)
	}
	if r.Status != RefundStatusSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", r.Status)
	}
	if r.ProviderRefundID != "refund_123" {
		t.Errorf("expected provider_refund_id refund_123, got %s", r.ProviderRefundID)
	}
	if r.ProcessedAt == nil {
		t.Error("expected processed_at to be set")
	}
}

func TestRefund_Fail(t *testing.T) {
	r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)

	if err := r.Fail("provider error", 1010); err != nil {
		t.Fatalf("fail failed: %v", err)
	}
	if r.Status != RefundStatusFailed {
		t.Errorf("expected FAILED, got %s", r.Status)
	}
	if r.FailureReason != "provider error" {
		t.Errorf("expected failure reason, got %s", r.FailureReason)
	}
}

func TestRefund_Cancel(t *testing.T) {
	r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)

	if err := r.Cancel(1010); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if r.Status != RefundStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", r.Status)
	}
}

func TestRefund_InvalidTransitions(t *testing.T) {
	r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)

	// PENDING -> SUCCEEDED (valid)
	if err := r.Succeed("refund_123", 1010, 1010); err != nil {
		t.Fatalf("succeed failed: %v", err)
	}

	// SUCCEEDED -> FAILED (invalid — terminal)
	if err := r.Fail("error", 1020); err == nil {
		t.Fatal("expected error transitioning from terminal SUCCEEDED")
	}

	// SUCCEEDED -> CANCELLED (invalid — terminal)
	if err := r.Cancel(1020); err == nil {
		t.Fatal("expected error transitioning from terminal SUCCEEDED")
	}
}

func TestRefund_IsTerminal(t *testing.T) {
	tests := []struct {
		status   RefundStatus
		terminal bool
	}{
		{RefundStatusPending, false},
		{RefundStatusSucceeded, true},
		{RefundStatusFailed, true},
		{RefundStatusCancelled, true},
	}

	for _, tt := range tests {
		r := &Refund{Status: tt.status}
		if r.IsTerminal() != tt.terminal {
			t.Errorf("status %s: expected terminal=%v, got %v", tt.status, tt.terminal, r.IsTerminal())
		}
	}
}

func TestRefund_ValidateInvariants(t *testing.T) {
	t.Run("valid refund", func(t *testing.T) {
		r := NewRefund("org-1", "payment-1", Money{AmountMinor: 500, Currency: "USD"}, "id-key-1", 1000)
		if err := r.ValidateInvariants(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		r := &Refund{
			TenantID:  "",
			PaymentID: "payment-1",
			Amount:    Money{AmountMinor: 500, Currency: "USD"},
			Status:    RefundStatusPending,
			Version:   1,
		}
		if err := r.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing tenant_id")
		}
	})

	t.Run("missing payment_id", func(t *testing.T) {
		r := &Refund{
			TenantID:  "org-1",
			PaymentID: "",
			Amount:    Money{AmountMinor: 500, Currency: "USD"},
			Status:    RefundStatusPending,
			Version:   1,
		}
		if err := r.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing payment_id")
		}
	})
}
