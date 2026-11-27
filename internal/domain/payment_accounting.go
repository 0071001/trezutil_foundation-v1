package domain

import (
	"context"
	"fmt"
)

/*
 * Financial integrity: a capture or a refund exists in Trezutil only together with its accounting. Both are written in
 * one database transaction (PaymentAccounting); if the ledger cannot be written, the payment does not change and the
 * operation is retried (provider webhook, status read, or reconciliation). Nothing here moves money.
 */

// PaymentAccounting writes a financial state change and its ledger entries atomically: all of it, or none.
type PaymentAccounting interface {
	// CommitCapture saves a payment that has just become captured (optimistic version) with its entries.
	CommitCapture(ctx context.Context, p *Payment, entries []*LedgerEntry) error
	// CommitRefund saves a refund that has just succeeded, the payment it reduces and the refund's entries.
	CommitRefund(ctx context.Context, r *Refund, p *Payment, entries []*LedgerEntry) error
	// EnsureEntries posts the entries that are missing and changes nothing else (a repair). An entry already posted
	// for the same payment or refund, account and direction is never posted twice. It returns how many were posted.
	EnsureEntries(ctx context.Context, entries []*LedgerEntry) (int, error)
}

// CaptureEntries are the postings of a captured payment: cash in, revenue recognised.
func CaptureEntries(p *Payment, now int64) ([]*LedgerEntry, error) {
	return postings(now,
		NewLedgerEntry(p.TenantID, &p.ID, nil, LedgerAccountCash, LedgerEntryTypeDebit, p.Amount, "ledger_capture_"+p.ID, now),
		NewLedgerEntry(p.TenantID, &p.ID, nil, LedgerAccountRevenue, LedgerEntryTypeCredit, p.Amount, "ledger_capture_rev_"+p.ID, now),
	)
}

// RefundEntries are the postings of a succeeded refund: the refund recorded, cash out.
func RefundEntries(r *Refund, now int64) ([]*LedgerEntry, error) {
	return postings(now,
		NewLedgerEntry(r.TenantID, nil, &r.ID, LedgerAccountRefund, LedgerEntryTypeDebit, r.Amount, "ledger_refund_"+r.ID, now),
		NewLedgerEntry(r.TenantID, nil, &r.ID, LedgerAccountCash, LedgerEntryTypeCredit, r.Amount, "ledger_refund_cash_"+r.ID, now),
	)
}

func postings(now int64, entries ...*LedgerEntry) ([]*LedgerEntry, error) {
	for _, e := range entries {
		if err := e.Post(now); err != nil {
			return nil, fmt.Errorf("ledger entry post failed: %w", err)
		}
		if err := e.ValidateInvariants(); err != nil {
			return nil, fmt.Errorf("ledger invariant violated: %w", err)
		}
	}
	return entries, nil
}
