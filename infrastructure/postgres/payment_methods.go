package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type paymentMethodRow struct {
	ID             string
	OrganizationID string
	MethodType     string
	TokenRef       string
	ProviderID     string
	DetailsJSON    []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type PaymentMethodRepository struct {
	db *sql.DB
}

func NewPaymentMethodRepository(db *sql.DB) *PaymentMethodRepository {
	return &PaymentMethodRepository{db: db}
}

func (r *PaymentMethodRepository) Create(ctx context.Context, method *domain.PaymentMethod) error {
	detailsJSON, _ := json.Marshal(method.Details)
	if detailsJSON == nil {
		detailsJSON = []byte("{}")
	}

	query := `INSERT INTO payment_methods (id, organization_id, method_type, token_ref, provider_id, details, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	result, err := r.db.ExecContext(ctx, query,
		method.ID, method.TenantID, string(method.MethodType), method.TokenRef,
		method.ProviderID, string(detailsJSON), toTime(method.CreatedAt), toTime(method.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create payment method: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 && method.ID == "" {
		return fmt.Errorf("no rows affected creating payment method")
	}
	return nil
}

func (r *PaymentMethodRepository) FindByID(ctx context.Context, tenantID, id string) (*domain.PaymentMethod, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, organization_id, method_type, token_ref, provider_id, details, created_at, updated_at
		FROM payment_methods WHERE id = $1 AND organization_id = $2
	`, id, tenantID)
	return r.scan(row)
}

func (r *PaymentMethodRepository) FindByTenantID(ctx context.Context, tenantID string, offset, limit int) ([]*domain.PaymentMethod, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, organization_id, method_type, token_ref, provider_id, details, created_at, updated_at
		FROM payment_methods WHERE organization_id = $1
		ORDER BY created_at DESC OFFSET $2 LIMIT $3
	`, tenantID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list payment methods: %w", err)
	}
	defer rows.Close()

	var methods []*domain.PaymentMethod
	for rows.Next() {
		var r paymentMethodRow
		if err := rows.Scan(
			&r.ID, &r.OrganizationID, &r.MethodType, &r.TokenRef, &r.ProviderID,
			&r.DetailsJSON, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan payment method: %w", err)
		}
		methods = append(methods, unmarshalPaymentMethod(r))
	}
	if methods == nil {
		methods = []*domain.PaymentMethod{}
	}
	return methods, nil
}

func (r *PaymentMethodRepository) Update(ctx context.Context, method *domain.PaymentMethod) error {
	detailsJSON, _ := json.Marshal(method.Details)
	if detailsJSON == nil {
		detailsJSON = []byte("{}")
	}

	query := `UPDATE payment_methods SET method_type = $1, token_ref = $2, provider_id = $3, details = $4, updated_at = $5
		WHERE id = $6 AND organization_id = $7`

	result, err := r.db.ExecContext(ctx, query,
		string(method.MethodType), method.TokenRef, method.ProviderID,
		string(detailsJSON), toTime(method.UpdatedAt), method.ID, method.TenantID,
	)
	if err != nil {
		return fmt.Errorf("failed to update payment method: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrOptimisticLock
	}
	return nil
}

func (r *PaymentMethodRepository) Delete(ctx context.Context, tenantID, id string) error {
	query := `DELETE FROM payment_methods WHERE id = $1 AND organization_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, tenantID)
	if err != nil {
		return fmt.Errorf("failed to delete payment method: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil
	}
	return nil
}

func (r *PaymentMethodRepository) scan(row *sql.Row) (*domain.PaymentMethod, error) {
	var pmr paymentMethodRow
	err := row.Scan(
		&pmr.ID, &pmr.OrganizationID, &pmr.MethodType, &pmr.TokenRef, &pmr.ProviderID,
		&pmr.DetailsJSON, &pmr.CreatedAt, &pmr.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrPaymentNotFound
		}
		return nil, fmt.Errorf("failed to scan payment method: %w", err)
	}
	return unmarshalPaymentMethod(pmr), nil
}

func unmarshalPaymentMethod(r paymentMethodRow) *domain.PaymentMethod {
	m := &domain.PaymentMethod{
		ID:         r.ID,
		TenantID:   r.OrganizationID,
		MethodType: domain.PaymentMethodType(r.MethodType),
		TokenRef:   r.TokenRef,
		ProviderID: r.ProviderID,
		CreatedAt:  toUnix(r.CreatedAt),
		UpdatedAt:  toUnix(r.UpdatedAt),
		Details:    make(map[string]interface{}),
	}
	if len(r.DetailsJSON) > 0 && string(r.DetailsJSON) != "null" {
		json.Unmarshal(r.DetailsJSON, &m.Details)
	}
	return m
}
