package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// sqlExecer is a connection pool or a transaction.
type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// PaymentAccounting implements domain.PaymentAccounting: the payment, the refund and their ledger entries are written
// in one transaction.
type PaymentAccounting struct {
	db       *sql.DB
	payments *PaymentRepository
	refunds  *RefundRepository
}

// NewPaymentAccounting builds the store.
func NewPaymentAccounting(db *sql.DB) *PaymentAccounting {
	return &PaymentAccounting{db: db, payments: NewPaymentRepository(db), refunds: NewRefundRepository(db)}
}

var _ domain.PaymentAccounting = (*PaymentAccounting)(nil)

func (s *PaymentAccounting) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("accounting begin: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("accounting commit: %w", err)
	}
	return nil
}

// CommitCapture saves the captured payment with its entries.
func (s *PaymentAccounting) CommitCapture(ctx context.Context, p *domain.Payment, entries []*domain.LedgerEntry) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.payments.update(ctx, tx, p); err != nil {
			return err
		}
		_, err := insertMissingEntries(ctx, tx, entries)
		return err
	})
}

// CommitRefund saves the succeeded refund, the reduced payment and the entries.
func (s *PaymentAccounting) CommitRefund(ctx context.Context, r *domain.Refund, p *domain.Payment, entries []*domain.LedgerEntry) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.refunds.update(ctx, tx, r); err != nil {
			return err
		}
		if err := s.payments.update(ctx, tx, p); err != nil {
			return err
		}
		_, err := insertMissingEntries(ctx, tx, entries)
		return err
	})
}

// EnsureEntries posts the missing entries only.
func (s *PaymentAccounting) EnsureEntries(ctx context.Context, entries []*domain.LedgerEntry) (int, error) {
	posted := 0
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		var err error
		posted, err = insertMissingEntries(ctx, tx, entries)
		return err
	})
	return posted, err
}

// insertMissingEntries posts each entry unless the same posting (payment or refund, account, direction) exists, under
// any key: entries written by earlier versions used other keys and must not be doubled. The payment (or refund) row is
// locked first, so two writers cannot both find a posting missing.
func insertMissingEntries(ctx context.Context, tx *sql.Tx, entries []*domain.LedgerEntry) (int, error) {
	posted := 0
	for _, e := range entries {
		var paymentID, refundID interface{}
		lock, target := `SELECT 1 FROM payments WHERE id = $1 AND organization_id = $2 FOR UPDATE`, ""
		if e.PaymentID != nil {
			paymentID, target = *e.PaymentID, *e.PaymentID
		}
		if e.RefundID != nil {
			refundID, target = *e.RefundID, *e.RefundID
			lock = `SELECT 1 FROM refunds WHERE id = $1 AND organization_id = $2 FOR UPDATE`
		}
		var one int
		if err := tx.QueryRowContext(ctx, lock, target, e.TenantID).Scan(&one); err != nil {
			return posted, fmt.Errorf("lock for posting: %w", err)
		}
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM payment_ledger
			WHERE organization_id = $1 AND account = $2 AND entry_type = $3 AND status = 'posted'
			  AND payment_id IS NOT DISTINCT FROM $4::uuid AND refund_id IS NOT DISTINCT FROM $5::uuid)`,
			e.TenantID, string(e.Account), string(e.EntryType), paymentID, refundID).Scan(&exists); err != nil {
			return posted, fmt.Errorf("look up posting: %w", err)
		}
		if exists {
			continue
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO payment_ledger (
				organization_id, payment_id, refund_id, account, entry_type, amount_minor, currency, idempotency_key,
				status, posted_at, failure_reason, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT (organization_id, idempotency_key) DO NOTHING`,
			e.TenantID, paymentID, refundID, string(e.Account), string(e.EntryType), e.Amount.AmountMinor, e.Amount.Currency,
			e.IdempotencyKey, string(e.Status), nullTimeFromPtr(e.PostedAt), e.FailureReason, toTime(e.CreatedAt), toTime(e.UpdatedAt))
		if err != nil {
			return posted, fmt.Errorf("post ledger entry: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			// The key exists with another posting: never silently skip an accounting line.
			return posted, fmt.Errorf("ledger key %s already used by a different posting", e.IdempotencyKey)
		}
		posted++
	}
	return posted, nil
}
