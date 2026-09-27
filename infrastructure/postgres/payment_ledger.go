package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type ledgerEntryRow struct {
	ID             string
	OrganizationID string
	PaymentID      sql.NullString
	RefundID       sql.NullString
	Account        string
	EntryType      string
	AmountMinor    int64
	Currency       string
	IdempotencyKey string
	Status         string
	PostedAt       sql.NullTime
	FailureReason  sql.NullString
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type LedgerRepository struct {
	db *sql.DB
}

func NewLedgerRepository(db *sql.DB) *LedgerRepository {
	return &LedgerRepository{db: db}
}

// Post attempts to insert a ledger entry. Uses ON CONFLICT DO NOTHING on the
// unique (organization_id, idempotency_key) to guarantee exactly-once posting
// even under retries, duplicate webhooks, or duplicate confirmation flows.
// Returns the persisted entry (existing if a conflict occurred).
func (r *LedgerRepository) Post(ctx context.Context, entry *domain.LedgerEntry) (*domain.LedgerEntry, error) {
	var paymentID, refundID interface{}
	if entry.PaymentID != nil {
		paymentID = *entry.PaymentID
	}
	if entry.RefundID != nil {
		refundID = *entry.RefundID
	}

	var id string
	var err error
	if entry.ID == "" {
		query := `INSERT INTO payment_ledger (
			organization_id, payment_id, refund_id, account, entry_type,
			amount_minor, currency, idempotency_key, status, posted_at, failure_reason,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (organization_id, idempotency_key) DO NOTHING
		RETURNING id`
		err = r.db.QueryRowContext(ctx, query,
			entry.TenantID, paymentID, refundID, string(entry.Account),
			string(entry.EntryType), entry.Amount.AmountMinor, entry.Amount.Currency,
			entry.IdempotencyKey, string(entry.Status), nullTimeFromPtr(entry.PostedAt),
			entry.FailureReason, toTime(entry.CreatedAt), toTime(entry.UpdatedAt),
		).Scan(&id)
	} else {
		query := `INSERT INTO payment_ledger (
			id, organization_id, payment_id, refund_id, account, entry_type,
			amount_minor, currency, idempotency_key, status, posted_at, failure_reason,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (organization_id, idempotency_key) DO NOTHING
		RETURNING id`
		err = r.db.QueryRowContext(ctx, query,
			entry.ID, entry.TenantID, paymentID, refundID, string(entry.Account),
			string(entry.EntryType), entry.Amount.AmountMinor, entry.Amount.Currency,
			entry.IdempotencyKey, string(entry.Status), nullTimeFromPtr(entry.PostedAt),
			entry.FailureReason, toTime(entry.CreatedAt), toTime(entry.UpdatedAt),
		).Scan(&id)
	}

	if err == sql.ErrNoRows {
		return r.FindByIdempotencyKey(ctx, entry.TenantID, entry.IdempotencyKey)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to post ledger entry: %w", err)
	}
	entry.ID = id
	return entry, nil
}

func (r *LedgerRepository) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.LedgerEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, payment_id, refund_id, account, entry_type,
			amount_minor, currency, idempotency_key, status, posted_at, failure_reason,
			created_at, updated_at
		FROM payment_ledger
		WHERE organization_id = $1 AND idempotency_key = $2
	`, tenantID, idempotencyKey)
	return r.scan(row)
}

func (r *LedgerRepository) FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*domain.LedgerEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, payment_id, refund_id, account, entry_type,
			amount_minor, currency, idempotency_key, status, posted_at, failure_reason,
			created_at, updated_at
		FROM payment_ledger
		WHERE organization_id = $1 AND payment_id = $2
		ORDER BY created_at ASC OFFSET $3 LIMIT $4
	`, tenantID, paymentID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []*domain.LedgerEntry
	for rows.Next() {
		var lr ledgerEntryRow
		if err := rows.Scan(
			&lr.ID, &lr.OrganizationID, &lr.PaymentID, &lr.RefundID,
			&lr.Account, &lr.EntryType, &lr.AmountMinor, &lr.Currency,
			&lr.IdempotencyKey, &lr.Status, &lr.PostedAt, &lr.FailureReason,
			&lr.CreatedAt, &lr.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
		}
		entries = append(entries, unmarshalLedgerEntry(lr))
	}
	if entries == nil {
		entries = []*domain.LedgerEntry{}
	}
	return entries, nil
}

func (r *LedgerRepository) scan(row *sql.Row) (*domain.LedgerEntry, error) {
	var lr ledgerEntryRow
	err := row.Scan(
		&lr.ID, &lr.OrganizationID, &lr.PaymentID, &lr.RefundID,
		&lr.Account, &lr.EntryType, &lr.AmountMinor, &lr.Currency,
		&lr.IdempotencyKey, &lr.Status, &lr.PostedAt, &lr.FailureReason,
		&lr.CreatedAt, &lr.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrLedgerAlreadyPosted
		}
		return nil, fmt.Errorf("failed to scan ledger entry: %w", err)
	}
	return unmarshalLedgerEntry(lr), nil
}

func unmarshalLedgerEntry(r ledgerEntryRow) *domain.LedgerEntry {
	e := &domain.LedgerEntry{
		ID:             r.ID,
		TenantID:       r.OrganizationID,
		Account:        domain.LedgerAccount(r.Account),
		EntryType:      domain.LedgerEntryType(r.EntryType),
		Amount:         domain.Money{AmountMinor: r.AmountMinor, Currency: r.Currency},
		IdempotencyKey: r.IdempotencyKey,
		Status:         domain.LedgerEntryStatus(r.Status),
		CreatedAt:      toUnix(r.CreatedAt),
		UpdatedAt:      toUnix(r.UpdatedAt),
	}
	if r.PaymentID.Valid {
		pid := r.PaymentID.String
		e.PaymentID = &pid
	}
	if r.RefundID.Valid {
		rid := r.RefundID.String
		e.RefundID = &rid
	}
	if r.PostedAt.Valid {
		ts := toUnix(r.PostedAt.Time)
		e.PostedAt = &ts
	}
	if r.FailureReason.Valid {
		e.FailureReason = r.FailureReason.String
	}
	return e
}
