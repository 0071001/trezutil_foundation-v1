package domain

import (
	"testing"
)

func TestNewPayment(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if p.TenantID != "org-1" {
		t.Errorf("expected tenant org-1, got %s", p.TenantID)
	}
	if p.Status != PaymentStatusCreated {
		t.Errorf("expected status CREATED, got %s", p.Status)
	}
	if p.Amount.AmountMinor != 1000 {
		t.Errorf("expected amount 1000, got %d", p.Amount.AmountMinor)
	}
	if p.Version != 1 {
		t.Errorf("expected version 1, got %d", p.Version)
	}
}

func TestPayment_Transitions(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	// CREATED -> PENDING
	if err := p.TransitionTo(PaymentStatusPending, 1010); err != nil {
		t.Fatalf("transition CREATED->PENDING failed: %v", err)
	}
	if p.Status != PaymentStatusPending {
		t.Errorf("expected PENDING, got %s", p.Status)
	}

	// PENDING -> AUTHORIZED
	if err := p.Authorize("ext-123", 1020, 1020); err != nil {
		t.Fatalf("authorize failed: %v", err)
	}
	if p.Status != PaymentStatusAuthorized {
		t.Errorf("expected AUTHORIZED, got %s", p.Status)
	}
	if p.ProviderPaymentID != "ext-123" {
		t.Errorf("expected provider_payment_id ext-123, got %s", p.ProviderPaymentID)
	}

	// AUTHORIZED -> CAPTURED
	if err := p.Capture("ext-123", 1030, 1030); err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if p.Status != PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}
}

func TestPayment_InvalidTransitions(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	// CREATED -> FAILED (valid)
	if err := p.TransitionTo(PaymentStatusFailed, 1010); err != nil {
		t.Fatalf("transition CREATED->FAILED failed: %v", err)
	}

	// FAILED -> CAPTURED (invalid — terminal)
	if err := p.TransitionTo(PaymentStatusCaptured, 1020); err == nil {
		t.Fatal("expected error transitioning from terminal FAILED")
	}
}

func TestPayment_CancelFromCreated(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.Cancel(1010); err != nil {
		t.Fatalf("cancel from CREATED failed: %v", err)
	}
	if p.Status != PaymentStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", p.Status)
	}
	if p.CancelledAt == nil {
		t.Error("expected cancelled_at to be set")
	}
}

func TestPayment_CannotCancelCaptured(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.TransitionTo(PaymentStatusPending, 1010); err != nil {
		t.Fatal(err)
	}
	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}

	if err := p.Cancel(1040); err == nil {
		t.Fatal("expected error cancelling captured payment")
	}
}

func TestPayment_RefundFull(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}

	if err := p.MarkPartiallyRefunded(Money{AmountMinor: 1000, Currency: "USD"}, 1040); err != nil {
		t.Fatalf("full refund failed: %v", err)
	}
	if p.Status != PaymentStatusRefunded {
		t.Errorf("expected REFUNDED, got %s", p.Status)
	}
	if p.RefundedAmount.AmountMinor != 1000 {
		t.Errorf("expected refunded amount 1000, got %d", p.RefundedAmount.AmountMinor)
	}
}

func TestPayment_RefundPartial(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}

	if err := p.MarkPartiallyRefunded(Money{AmountMinor: 500, Currency: "USD"}, 1040); err != nil {
		t.Fatalf("partial refund failed: %v", err)
	}
	if p.Status != PaymentStatusPartiallyRefunded {
		t.Errorf("expected PARTIALLY_REFUNDED, got %s", p.Status)
	}
	remaining := p.RemainingRefundable()
	if remaining.AmountMinor != 500 {
		t.Errorf("expected remaining refundable 500, got %d", remaining.AmountMinor)
	}
}

func TestPayment_RefundExceedsCaptured(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}

	// Partial refund of 500
	if err := p.MarkPartiallyRefunded(Money{AmountMinor: 500, Currency: "USD"}, 1040); err != nil {
		t.Fatal(err)
	}
	// Remaining 500
	if p.RemainingRefundable().AmountMinor != 500 {
		t.Fatalf("expected remaining 500, got %d", p.RemainingRefundable().AmountMinor)
	}
	// Attempt to refund 600 — should fail (exceeds total)
	if err := p.MarkPartiallyRefunded(Money{AmountMinor: 600, Currency: "USD"}, 1050); err == nil {
		t.Fatal("expected error refunding more than captured amount")
	}
}

func TestPayment_CanRefund(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if p.CanRefund() {
		t.Fatal("CREATED payment should not be refundable")
	}

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if p.CanRefund() {
		t.Fatal("AUTHORIZED payment should not be refundable")
	}

	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}
	if !p.CanRefund() {
		t.Fatal("CAPTURED payment should be refundable")
	}
}

func TestPayment_CanCancel(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if !p.CanCancel() {
		t.Fatal("CREATED payment should be cancellable")
	}

	if err := p.TransitionTo(PaymentStatusPending, 1010); err != nil {
		t.Fatal(err)
	}
	if !p.CanCancel() {
		t.Fatal("PENDING payment should be cancellable")
	}

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if !p.CanCancel() {
		t.Fatal("AUTHORIZED payment should be cancellable")
	}

	if err := p.Capture("ext-1", 1020, 1030); err != nil {
		t.Fatal(err)
	}
	if p.CanCancel() {
		t.Fatal("CAPTURED payment should not be cancellable")
	}
}

func TestPayment_IsCaptured(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if p.IsCaptured() {
		t.Fatal("CREATED payment should not be captured")
	}

	if err := p.Capture("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if !p.IsCaptured() {
		t.Fatal("CAPTURED payment should be captured")
	}
}

func TestPayment_IsTerminal(t *testing.T) {
	tests := []struct {
		status   PaymentStatus
		terminal bool
	}{
		{PaymentStatusCreated, false},
		{PaymentStatusPending, false},
		{PaymentStatusAuthorized, false},
		{PaymentStatusCaptured, false},
		{PaymentStatusPartiallyRefunded, false},
		{PaymentStatusFailed, true},
		{PaymentStatusCancelled, true},
		{PaymentStatusExpired, true},
		{PaymentStatusRefunded, true},
	}

	for _, tt := range tests {
		p := &Payment{Status: tt.status}
		if p.IsTerminal() != tt.terminal {
			t.Errorf("status %s: expected terminal=%v, got %v", tt.status, tt.terminal, p.IsTerminal())
		}
	}
}

func TestPayment_ValidateInvariants(t *testing.T) {
	t.Run("valid payment", func(t *testing.T) {
		p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
			PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)
		if err := p.ValidateInvariants(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("missing tenant", func(t *testing.T) {
		p := &Payment{
			TenantID: "",
			Status:   PaymentStatusCreated,
			Version:  1,
		}
		if err := p.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing tenant_id")
		}
	})

	t.Run("negative amount", func(t *testing.T) {
		p := NewPayment("org-1", "order-1", Money{AmountMinor: -100, Currency: "USD"},
			PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)
		if err := p.ValidateInvariants(); err == nil {
			t.Fatal("expected error for negative amount")
		}
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
			PaymentMethodCard, "method_123", "provider-1", "", 1000)
		if err := p.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing idempotency_key")
		}
	})
}

func TestPayment_ExpireFromPending(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.TransitionTo(PaymentStatusPending, 1010); err != nil {
		t.Fatal(err)
	}
	if err := p.Expire(1020); err != nil {
		t.Fatalf("expire from PENDING failed: %v", err)
	}
	if p.Status != PaymentStatusExpired {
		t.Errorf("expected EXPIRED, got %s", p.Status)
	}
}

func TestPayment_FailFromAuthorized(t *testing.T) {
	p := NewPayment("org-1", "order-1", Money{AmountMinor: 1000, Currency: "USD"},
		PaymentMethodCard, "method_123", "provider-1", "idem-key-1", 1000)

	if err := p.Authorize("ext-1", 1010, 1020); err != nil {
		t.Fatal(err)
	}
	if err := p.Fail("provider error", 1030); err != nil {
		t.Fatalf("fail from AUTHORIZED failed: %v", err)
	}
	if p.Status != PaymentStatusFailed {
		t.Errorf("expected FAILED, got %s", p.Status)
	}
	if p.FailureReason != "provider error" {
		t.Errorf("expected failure reason 'provider error', got %s", p.FailureReason)
	}
}
