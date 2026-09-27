package domain

import (
	"context"
	"fmt"
)

// LedgerEntryType represents the direction of a ledger posting.
type LedgerEntryType string

const (
	LedgerEntryTypeDebit  LedgerEntryType = "DEBIT"
	LedgerEntryTypeCredit LedgerEntryType = "CREDIT"
)

// LedgerEntryStatus represents the posting status of a ledger entry.
type LedgerEntryStatus string

const (
	LedgerEntryStatusPending LedgerEntryStatus = "pending"
	LedgerEntryStatusPosted  LedgerEntryStatus = "posted"
	LedgerEntryStatusFailed  LedgerEntryStatus = "failed"
)

// LedgerAccount identifies the accounting account for a posting.
type LedgerAccount string

const (
	LedgerAccountCash      LedgerAccount = "cash"
	LedgerAccountRevenue   LedgerAccount = "revenue"
	LedgerAccountLiability LedgerAccount = "liability"
	LedgerAccountRefund    LedgerAccount = "refund"
	LedgerAccountFee       LedgerAccount = "fee"
)

// LedgerEntry represents a single accounting posting for a payment event.
//
// The Ledger is the authoritative accounting record. Payments orchestrate
// external money movement; the Ledger records the internal accounting truth.
//
// Posting is exactly-once: each entry has an IdempotencyKey that is unique
// within a tenant. Concurrent or duplicate posting attempts with the same key
// are rejected by the database (INSERT ... ON CONFLICT DO NOTHING).
type LedgerEntry struct {
	ID             string
	TenantID       string
	PaymentID      *string
	RefundID       *string
	Account        LedgerAccount
	EntryType      LedgerEntryType
	Amount         Money
	IdempotencyKey string
	Status         LedgerEntryStatus
	PostedAt       *int64
	FailureReason  string
	CreatedAt      int64
	UpdatedAt      int64
}

// NewLedgerEntry creates a new ledger entry in PENDING status.
func NewLedgerEntry(
	tenantID string,
	paymentID, refundID *string,
	account LedgerAccount,
	entryType LedgerEntryType,
	amount Money,
	idempotencyKey string,
	now int64,
) *LedgerEntry {
	return &LedgerEntry{
		ID:             "",
		TenantID:       tenantID,
		PaymentID:      paymentID,
		RefundID:       refundID,
		Account:        account,
		EntryType:      entryType,
		Amount:         amount,
		IdempotencyKey: idempotencyKey,
		Status:         LedgerEntryStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Post marks the ledger entry as POSTED.
func (e *LedgerEntry) Post(at int64) error {
	if e.Status != LedgerEntryStatusPending {
		return fmt.Errorf("cannot post ledger entry in status %s", e.Status)
	}
	e.Status = LedgerEntryStatusPosted
	e.PostedAt = &at
	e.FailureReason = ""
	e.UpdatedAt = at
	return nil
}

// MarkFailed marks the ledger entry as FAILED.
func (e *LedgerEntry) MarkFailed(reason string, now int64) error {
	if e.Status == LedgerEntryStatusPosted {
		return fmt.Errorf("cannot fail an already-posted ledger entry")
	}
	e.Status = LedgerEntryStatusFailed
	e.FailureReason = reason
	e.UpdatedAt = now
	return nil
}

// ValidateInvariants checks all domain invariants for a LedgerEntry.
func (e *LedgerEntry) ValidateInvariants() error {
	if e.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if (e.PaymentID == nil && e.RefundID == nil) || (e.PaymentID != nil && e.RefundID != nil) {
		return fmt.Errorf("exactly one of payment_id or refund_id must be set")
	}
	if e.Amount.AmountMinor <= 0 {
		return fmt.Errorf("ledger amount must be positive")
	}
	if e.Amount.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if e.IdempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}
	return nil
}

// LedgerRepository defines the interface for ledger persistence.
type LedgerRepository interface {
	// Post attempts to insert a ledger entry. If an entry with the same
	// idempotency key already exists for the tenant, it is a no-op (returns
	// the existing entry). This guarantees exactly-once accounting.
	Post(ctx context.Context, entry *LedgerEntry) (*LedgerEntry, error)
	// FindByIdempotencyKey finds a ledger entry by its idempotency key.
	FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*LedgerEntry, error)
	// FindByPaymentID lists ledger entries for a payment.
	FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*LedgerEntry, error)
}
