package domain

import (
	"testing"
)

func TestNewLedgerEntry(t *testing.T) {
	pid := "payment-123"
	e := NewLedgerEntry("org-1", &pid, nil, LedgerAccountCash, LedgerEntryTypeDebit,
		Money{AmountMinor: 1000, Currency: "USD"}, "ledger-key-1", 1000)

	if e.TenantID != "org-1" {
		t.Errorf("expected tenant org-1, got %s", e.TenantID)
	}
	if *e.PaymentID != "payment-123" {
		t.Errorf("expected payment_id payment-123, got %s", *e.PaymentID)
	}
	if e.Account != LedgerAccountCash {
		t.Errorf("expected cash account, got %s", e.Account)
	}
	if e.EntryType != LedgerEntryTypeDebit {
		t.Errorf("expected DEBIT, got %s", e.EntryType)
	}
	if e.Status != LedgerEntryStatusPending {
		t.Errorf("expected PENDING, got %s", e.Status)
	}
}

func TestLedgerEntry_Post(t *testing.T) {
	e := NewLedgerEntry("org-1", nil, stringPtr("refund-1"), LedgerAccountRefund, LedgerEntryTypeDebit,
		Money{AmountMinor: 500, Currency: "USD"}, "ledger-key-1", 1000)

	if err := e.Post(1010); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if e.Status != LedgerEntryStatusPosted {
		t.Errorf("expected POSTED, got %s", e.Status)
	}
	if e.PostedAt == nil || *e.PostedAt != 1010 {
		t.Error("expected posted_at to be set")
	}

	// Cannot post again
	if err := e.Post(1020); err == nil {
		t.Fatal("expected error posting already-posted entry")
	}
}

func TestLedgerEntry_MarkFailed(t *testing.T) {
	e := NewLedgerEntry("org-1", nil, stringPtr("refund-1"), LedgerAccountRefund, LedgerEntryTypeDebit,
		Money{AmountMinor: 500, Currency: "USD"}, "ledger-key-1", 1000)

	if err := e.MarkFailed("insufficient funds", 1010); err != nil {
		t.Fatalf("mark failed: %v", err)
	}
	if e.Status != LedgerEntryStatusFailed {
		t.Errorf("expected FAILED, got %s", e.Status)
	}
	if e.FailureReason != "insufficient funds" {
		t.Errorf("expected failure reason, got %s", e.FailureReason)
	}
}

func TestLedgerEntry_CannotMarkFailedAfterPosted(t *testing.T) {
	pid := "payment-123"
	e := NewLedgerEntry("org-1", &pid, nil, LedgerAccountCash, LedgerEntryTypeDebit,
		Money{AmountMinor: 1000, Currency: "USD"}, "ledger-key-1", 1000)

	if err := e.Post(1010); err != nil {
		t.Fatal(err)
	}

	if err := e.MarkFailed("error", 1020); err == nil {
		t.Fatal("expected error marking posted entry as failed")
	}
}

func TestLedgerEntry_ValidateInvariants(t *testing.T) {
	pid := "payment-123"

	t.Run("valid debit entry", func(t *testing.T) {
		e := NewLedgerEntry("org-1", &pid, nil, LedgerAccountCash, LedgerEntryTypeDebit,
			Money{AmountMinor: 1000, Currency: "USD"}, "ledger-key-1", 1000)
		if err := e.ValidateInvariants(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("missing payment and refund", func(t *testing.T) {
		e := NewLedgerEntry("org-1", nil, nil, LedgerAccountCash, LedgerEntryTypeDebit,
			Money{AmountMinor: 1000, Currency: "USD"}, "ledger-key-1", 1000)
		if err := e.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing payment_id and refund_id")
		}
	})

	t.Run("both payment and refund set", func(t *testing.T) {
		e := NewLedgerEntry("org-1", &pid, stringPtr("refund-1"), LedgerAccountCash, LedgerEntryTypeDebit,
			Money{AmountMinor: 1000, Currency: "USD"}, "ledger-key-1", 1000)
		if err := e.ValidateInvariants(); err == nil {
			t.Fatal("expected error for both payment_id and refund_id set")
		}
	})

	t.Run("zero amount", func(t *testing.T) {
		e := NewLedgerEntry("org-1", &pid, nil, LedgerAccountCash, LedgerEntryTypeDebit,
			Money{AmountMinor: 0, Currency: "USD"}, "ledger-key-1", 1000)
		if err := e.ValidateInvariants(); err == nil {
			t.Fatal("expected error for zero amount")
		}
	})

	t.Run("missing idempotency key", func(t *testing.T) {
		e := NewLedgerEntry("org-1", &pid, nil, LedgerAccountCash, LedgerEntryTypeDebit,
			Money{AmountMinor: 1000, Currency: "USD"}, "", 1000)
		if err := e.ValidateInvariants(); err == nil {
			t.Fatal("expected error for missing idempotency_key")
		}
	})
}

func TestAggregateRefundSummary(t *testing.T) {
	refunds := []*Refund{
		{Amount: Money{AmountMinor: 300, Currency: "USD"}, Status: RefundStatusSucceeded},
		{Amount: Money{AmountMinor: 200, Currency: "USD"}, Status: RefundStatusPending},
		{Amount: Money{AmountMinor: 100, Currency: "USD"}, Status: RefundStatusFailed},
	}

	total := AggregateRefundSummary(refunds)
	if total.AmountMinor != 500 {
		t.Errorf("expected total 500 (succeeded + pending only), got %d", total.AmountMinor)
	}
}

func stringPtr(s string) *string {
	return &s
}
