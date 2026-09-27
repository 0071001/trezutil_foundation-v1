package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type refundRow struct {
	ID               string
	OrganizationID   string
	PaymentID        string
	AmountMinor      int64
	Currency         string
	ProviderRefundID sql.NullString
	Status           string
	IdempotencyKey   string
	RequestedAt      time.Time
	ProcessedAt      sql.NullTime
	FailedAt         sql.NullTime
	FailureReason    sql.NullString
	MetadataJSON     []byte
	Version          int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RefundRepository struct {
	db *sql.DB
}

func NewRefundRepository(db *sql.DB) *RefundRepository {
	return &RefundRepository{db: db}
}

func (r *RefundRepository) Create(ctx context.Context, refund *domain.Refund) error {
	metadataJSON, _ := json.Marshal(refund.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var providerRefundID interface{}
	if refund.ProviderRefundID != "" {
		providerRefundID = refund.ProviderRefundID
	}

	var refundID string
	var err error
	if refund.ID == "" {
		query := `INSERT INTO refunds (
			organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`
		err = r.db.QueryRowContext(ctx, query,
			refund.TenantID, refund.PaymentID, refund.Amount.AmountMinor, refund.Amount.Currency,
			providerRefundID, string(refund.Status), refund.IdempotencyKey,
			toTime(refund.RequestedAt), nullTimeFromPtr(refund.ProcessedAt),
			nullTimeFromPtr(refund.FailedAt), refund.FailureReason,
			string(metadataJSON), refund.Version,
			toTime(refund.CreatedAt), toTime(refund.UpdatedAt),
		).Scan(&refundID)
	} else {
		query := `INSERT INTO refunds (
			id, organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`
		_, err = r.db.ExecContext(ctx, query,
			refund.ID, refund.TenantID, refund.PaymentID, refund.Amount.AmountMinor, refund.Amount.Currency,
			providerRefundID, string(refund.Status), refund.IdempotencyKey,
			toTime(refund.RequestedAt), nullTimeFromPtr(refund.ProcessedAt),
			nullTimeFromPtr(refund.FailedAt), refund.FailureReason,
			string(metadataJSON), refund.Version,
			toTime(refund.CreatedAt), toTime(refund.UpdatedAt),
		)
		refundID = refund.ID
	}
	if err != nil {
		return fmt.Errorf("failed to create refund: %w", err)
	}
	refund.ID = refundID
	return nil
}

func (r *RefundRepository) FindByID(ctx context.Context, tenantID, refundID string) (*domain.Refund, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		FROM refunds WHERE id = $1 AND organization_id = $2
	`, refundID, tenantID)
	return r.scan(row)
}

func (r *RefundRepository) FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*domain.Refund, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		FROM refunds WHERE payment_id = $1 AND organization_id = $2
		ORDER BY created_at DESC OFFSET $3 LIMIT $4
	`, paymentID, tenantID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to find refunds by payment: %w", err)
	}
	defer rows.Close()

	var refunds []*domain.Refund
	for rows.Next() {
		var r refundRow
		if err := rows.Scan(
			&r.ID, &r.OrganizationID, &r.PaymentID, &r.AmountMinor, &r.Currency,
			&r.ProviderRefundID, &r.Status, &r.IdempotencyKey, &r.RequestedAt,
			&r.ProcessedAt, &r.FailedAt, &r.FailureReason, &r.MetadataJSON,
			&r.Version, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan refund: %w", err)
		}
		refunds = append(refunds, unmarshalRefund(r))
	}
	if refunds == nil {
		refunds = []*domain.Refund{}
	}
	return refunds, nil
}

func (r *RefundRepository) FindByProviderRefundID(ctx context.Context, tenantID, providerID, providerRefundID string) (*domain.Refund, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		FROM refunds WHERE provider_refund_id = $1 AND organization_id = $2
	`, providerRefundID, tenantID)
	return r.scan(row)
}

func (r *RefundRepository) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.Refund, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, payment_id, amount_minor, currency, provider_refund_id,
			status, idempotency_key, requested_at, processed_at, failed_at, failure_reason,
			metadata, version, created_at, updated_at
		FROM refunds WHERE organization_id = $1 AND idempotency_key = $2
	`, tenantID, idempotencyKey)
	return r.scan(row)
}

func (r *RefundRepository) Update(ctx context.Context, refund *domain.Refund) error {
	metadataJSON, _ := json.Marshal(refund.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var providerRefundID interface{}
	if refund.ProviderRefundID != "" {
		providerRefundID = refund.ProviderRefundID
	}

	query := `UPDATE refunds SET
		provider_refund_id = $1, status = $2, processed_at = $3, failed_at = $4,
		failure_reason = $5, metadata = $6, version = $7, updated_at = $8
		WHERE id = $9 AND organization_id = $10 AND version = $11`

	result, err := r.db.ExecContext(ctx, query,
		providerRefundID, string(refund.Status),
		nullTimeFromPtr(refund.ProcessedAt), nullTimeFromPtr(refund.FailedAt),
		refund.FailureReason, string(metadataJSON),
		refund.Version, toTime(refund.UpdatedAt),
		refund.ID, refund.TenantID, refund.Version-1,
	)
	if err != nil {
		return fmt.Errorf("failed to update refund: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *RefundRepository) scan(row *sql.Row) (*domain.Refund, error) {
	var rr refundRow
	err := row.Scan(
		&rr.ID, &rr.OrganizationID, &rr.PaymentID, &rr.AmountMinor, &rr.Currency,
		&rr.ProviderRefundID, &rr.Status, &rr.IdempotencyKey, &rr.RequestedAt,
		&rr.ProcessedAt, &rr.FailedAt, &rr.FailureReason, &rr.MetadataJSON,
		&rr.Version, &rr.CreatedAt, &rr.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrRefundNotFound
		}
		return nil, fmt.Errorf("failed to scan refund: %w", err)
	}
	return unmarshalRefund(rr), nil
}

func unmarshalRefund(r refundRow) *domain.Refund {
	refund := &domain.Refund{
		ID:             r.ID,
		TenantID:       r.OrganizationID,
		PaymentID:      r.PaymentID,
		Amount:         domain.Money{AmountMinor: r.AmountMinor, Currency: r.Currency},
		Status:         domain.RefundStatus(r.Status),
		IdempotencyKey: r.IdempotencyKey,
		RequestedAt:    toUnix(r.RequestedAt),
		FailureReason:  r.FailureReason.String,
		Version:        r.Version,
		CreatedAt:      toUnix(r.CreatedAt),
		UpdatedAt:      toUnix(r.UpdatedAt),
		Metadata:       make(map[string]interface{}),
	}
	if r.ProviderRefundID.Valid {
		refund.ProviderRefundID = r.ProviderRefundID.String
	}
	if r.ProcessedAt.Valid {
		ts := toUnix(r.ProcessedAt.Time)
		refund.ProcessedAt = &ts
	}
	if r.FailedAt.Valid {
		ts := toUnix(r.FailedAt.Time)
		refund.FailedAt = &ts
	}
	if len(r.MetadataJSON) > 0 && string(r.MetadataJSON) != "null" {
		json.Unmarshal(r.MetadataJSON, &refund.Metadata)
	}
	return refund
}
