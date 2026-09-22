package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type ProviderExecutionRepository struct {
	db *sql.DB
}

func NewProviderExecutionRepository(db *sql.DB) *ProviderExecutionRepository {
	return &ProviderExecutionRepository{db: db}
}

func (r *ProviderExecutionRepository) Create(ctx context.Context, exec *domain.ProviderExecution) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provider_executions (id, provider_id, organization_id, capability, operation, actor_id, idempotency_key, status, error_code, started_at, completed_at, duration_ms, external_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`, exec.ID, exec.ProviderID, exec.TenantID, string(exec.Capability), exec.Operation, exec.ActorID, exec.IdempotencyKey,
		string(exec.Status), exec.ErrorCode, toTime(exec.StartedAt), toTime(exec.CompletedAt), exec.DurationMs, exec.ExternalReference)
	if err != nil {
		return fmt.Errorf("failed to create provider execution: %w", err)
	}
	return nil
}

func (r *ProviderExecutionRepository) FindByID(ctx context.Context, orgID string, id string) (*domain.ProviderExecution, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, provider_id, organization_id, capability, operation, actor_id, idempotency_key, status, error_code, started_at, completed_at, duration_ms, external_reference
		FROM provider_executions WHERE id = $1 AND organization_id = $2
	`, id, orgID)
	return r.scan(row)
}

func (r *ProviderExecutionRepository) FindByIdempotencyKey(ctx context.Context, orgID string, key string) (*domain.ProviderExecution, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, provider_id, organization_id, capability, operation, actor_id, idempotency_key, status, error_code, started_at, completed_at, duration_ms, external_reference
		FROM provider_executions WHERE idempotency_key = $1 AND organization_id = $2
	`, key, orgID)
	return r.scan(row)
}

func (r *ProviderExecutionRepository) FindByProviderID(ctx context.Context, orgID string, providerID string, offset, limit int) ([]*domain.ProviderExecution, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, provider_id, organization_id, capability, operation, actor_id, idempotency_key, status, error_code, started_at, completed_at, duration_ms, external_reference
		FROM provider_executions WHERE provider_id = $1 AND organization_id = $2
		ORDER BY started_at DESC OFFSET $3 LIMIT $4
	`, providerID, orgID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list provider executions: %w", err)
	}
	defer rows.Close()

	var executions []*domain.ProviderExecution
	for rows.Next() {
		var e domain.ProviderExecution
		err := rows.Scan(
			&e.ID, &e.ProviderID, &e.TenantID, &e.Capability, &e.Operation, &e.ActorID, &e.IdempotencyKey,
			&e.Status, &e.ErrorCode, &e.StartedAt, &e.CompletedAt, &e.DurationMs, &e.ExternalReference,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution: %w", err)
		}
		executions = append(executions, &e)
	}
	if executions == nil {
		executions = []*domain.ProviderExecution{}
	}
	return executions, nil
}

func (r *ProviderExecutionRepository) scan(row *sql.Row) (*domain.ProviderExecution, error) {
	var e domain.ProviderExecution
	err := row.Scan(
		&e.ID, &e.ProviderID, &e.TenantID, &e.Capability, &e.Operation, &e.ActorID, &e.IdempotencyKey,
		&e.Status, &e.ErrorCode, &e.StartedAt, &e.CompletedAt, &e.DurationMs, &e.ExternalReference,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrProviderNotFound
		}
		return nil, fmt.Errorf("failed to scan provider execution: %w", err)
	}
	return &e, nil
}
