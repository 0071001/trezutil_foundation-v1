package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type paymentRow struct {
	ID                string
	OrganizationID    string
	OrderID           string
	AmountMinor       int64
	Currency          string
	PaymentMethodType string
	PaymentMethodID   string
	ProviderID        string
	ProviderPaymentID sql.NullString
	Status            string
	IdempotencyKey    string
	AuthorizedAt      sql.NullTime
	CapturedAt        sql.NullTime
	CancelledAt       sql.NullTime
	FailedAt          sql.NullTime
	FailureReason     sql.NullString
	RefundedAmount    int64
	RefundedCurrency  sql.NullString
	MetadataJSON      []byte
	Version           int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	metadataJSON, _ := json.Marshal(payment.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var providerPaymentID interface{}
	if payment.ProviderPaymentID != "" {
		providerPaymentID = payment.ProviderPaymentID
	}

	var paymentID string
	var err error
	if payment.ID == "" {
		query := `INSERT INTO payments (
			organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21) RETURNING id`
		err = r.db.QueryRowContext(ctx, query,
			payment.TenantID, payment.OrderID, payment.Amount.AmountMinor, payment.Amount.Currency,
			string(payment.PaymentMethodType), payment.PaymentMethodID, payment.ProviderID, providerPaymentID,
			string(payment.Status), payment.IdempotencyKey,
			nullTimeFromPtr(payment.AuthorizedAt), nullTimeFromPtr(payment.CapturedAt),
			nullTimeFromPtr(payment.CancelledAt), nullTimeFromPtr(payment.FailedAt),
			payment.FailureReason,
			payment.RefundedAmount.AmountMinor, payment.RefundedAmount.Currency,
			string(metadataJSON), payment.Version,
			toTime(payment.CreatedAt), toTime(payment.UpdatedAt),
		).Scan(&paymentID)
	} else {
		query := `INSERT INTO payments (
			id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`
		_, err = r.db.ExecContext(ctx, query,
			payment.ID, payment.TenantID, payment.OrderID, payment.Amount.AmountMinor, payment.Amount.Currency,
			string(payment.PaymentMethodType), payment.PaymentMethodID, payment.ProviderID, providerPaymentID,
			string(payment.Status), payment.IdempotencyKey,
			nullTimeFromPtr(payment.AuthorizedAt), nullTimeFromPtr(payment.CapturedAt),
			nullTimeFromPtr(payment.CancelledAt), nullTimeFromPtr(payment.FailedAt),
			payment.FailureReason,
			payment.RefundedAmount.AmountMinor, payment.RefundedAmount.Currency,
			string(metadataJSON), payment.Version,
			toTime(payment.CreatedAt), toTime(payment.UpdatedAt),
		)
		paymentID = payment.ID
	}
	if err != nil {
		return fmt.Errorf("failed to create payment: %w", err)
	}
	payment.ID = paymentID
	return nil
}

func (r *PaymentRepository) FindByID(ctx context.Context, tenantID, paymentID string) (*domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		FROM payments WHERE id = $1 AND organization_id = $2
	`, paymentID, tenantID)
	return r.scan(row)
}

func (r *PaymentRepository) FindByIDForUpdate(ctx context.Context, tenantID, paymentID string) (*domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		FROM payments WHERE id = $1 AND organization_id = $2 FOR UPDATE
	`, paymentID, tenantID)
	return r.scan(row)
}

func (r *PaymentRepository) FindByOrderID(ctx context.Context, tenantID, orderID string) (*domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		FROM payments WHERE order_id = $1 AND organization_id = $2
	`, orderID, tenantID)
	return r.scan(row)
}

func (r *PaymentRepository) FindByProviderPaymentID(ctx context.Context, tenantID, providerID, providerPaymentID string) (*domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		FROM payments WHERE provider_id = $1 AND provider_payment_id = $2 AND organization_id = $3
	`, providerID, providerPaymentID, tenantID)
	return r.scan(row)
}

func (r *PaymentRepository) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.Payment, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, order_id, amount_minor, currency, payment_method_type,
			payment_method_id, provider_id, provider_payment_id, status, idempotency_key,
			authorized_at, captured_at, cancelled_at, failed_at, failure_reason,
			refunded_amount_minor, refunded_currency, metadata, version, created_at, updated_at
		FROM payments WHERE organization_id = $1 AND idempotency_key = $2
	`, tenantID, idempotencyKey)
	return r.scan(row)
}

func (r *PaymentRepository) Update(ctx context.Context, payment *domain.Payment) error {
	metadataJSON, _ := json.Marshal(payment.Metadata)
	if metadataJSON == nil {
		metadataJSON = []byte("{}")
	}

	var providerPaymentID interface{}
	if payment.ProviderPaymentID != "" {
		providerPaymentID = payment.ProviderPaymentID
	}

	query := `UPDATE payments SET
		provider_payment_id = $1, status = $2, authorized_at = $3, captured_at = $4,
		cancelled_at = $5, failed_at = $6, failure_reason = $7,
		refunded_amount_minor = $8, refunded_currency = $9, metadata = $10,
		version = $11, updated_at = $12
		WHERE id = $13 AND organization_id = $14 AND version = $15`

	result, err := r.db.ExecContext(ctx, query,
		providerPaymentID,
		string(payment.Status),
		nullTimeFromPtr(payment.AuthorizedAt),
		nullTimeFromPtr(payment.CapturedAt),
		nullTimeFromPtr(payment.CancelledAt),
		nullTimeFromPtr(payment.FailedAt),
		payment.FailureReason,
		payment.RefundedAmount.AmountMinor, payment.RefundedAmount.Currency,
		string(metadataJSON),
		payment.Version, toTime(payment.UpdatedAt),
		payment.ID, payment.TenantID, payment.Version-1,
	)
	if err != nil {
		return fmt.Errorf("failed to update payment: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *PaymentRepository) scan(row *sql.Row) (*domain.Payment, error) {
	var pr paymentRow
	err := row.Scan(
		&pr.ID, &pr.OrganizationID, &pr.OrderID, &pr.AmountMinor, &pr.Currency,
		&pr.PaymentMethodType, &pr.PaymentMethodID, &pr.ProviderID, &pr.ProviderPaymentID,
		&pr.Status, &pr.IdempotencyKey, &pr.AuthorizedAt, &pr.CapturedAt, &pr.CancelledAt,
		&pr.FailedAt, &pr.FailureReason, &pr.RefundedAmount, &pr.RefundedCurrency,
		&pr.MetadataJSON, &pr.Version, &pr.CreatedAt, &pr.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to scan payment: %w", err)
	}
	return unmarshalPayment(pr), nil
}

func unmarshalPayment(r paymentRow) *domain.Payment {
	p := &domain.Payment{
		ID:                r.ID,
		TenantID:          r.OrganizationID,
		OrderID:           r.OrderID,
		Amount:            domain.Money{AmountMinor: r.AmountMinor, Currency: r.Currency},
		PaymentMethodType: domain.PaymentMethodType(r.PaymentMethodType),
		PaymentMethodID:   r.PaymentMethodID,
		ProviderID:        r.ProviderID,
		Status:            domain.PaymentStatus(r.Status),
		IdempotencyKey:    r.IdempotencyKey,
		FailureReason:     r.FailureReason.String,
		RefundedAmount:    domain.Money{AmountMinor: r.RefundedAmount, Currency: r.Currency},
		Version:           r.Version,
		CreatedAt:         toUnix(r.CreatedAt),
		UpdatedAt:         toUnix(r.UpdatedAt),
		Metadata:          make(map[string]interface{}),
	}
	if r.ProviderPaymentID.Valid {
		p.ProviderPaymentID = r.ProviderPaymentID.String
	}
	if r.AuthorizedAt.Valid {
		ts := toUnix(r.AuthorizedAt.Time)
		p.AuthorizedAt = &ts
	}
	if r.CapturedAt.Valid {
		ts := toUnix(r.CapturedAt.Time)
		p.CapturedAt = &ts
	}
	if r.CancelledAt.Valid {
		ts := toUnix(r.CancelledAt.Time)
		p.CancelledAt = &ts
	}
	if r.FailedAt.Valid {
		ts := toUnix(r.FailedAt.Time)
		p.FailedAt = &ts
	}
	if len(r.MetadataJSON) > 0 && string(r.MetadataJSON) != "null" {
		json.Unmarshal(r.MetadataJSON, &p.Metadata)
	}
	return p
}

func nullTimeFromPtr(t *int64) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: toTime(*t), Valid: true}
}
