package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// ProviderExecutionRepository persists provider attempts (provider_executions,
// migrations 000011 and 000021). Every read is scoped to an organization.
type ProviderExecutionRepository struct {
	db *sql.DB
}

func NewProviderExecutionRepository(db *sql.DB) *ProviderExecutionRepository {
	return &ProviderExecutionRepository{db: db}
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// actorRef is the user who triggered the call; system callers (webhooks,
// background jobs) are stored as NULL.
func actorRef(actorID string) sql.NullString {
	return sql.NullString{String: actorID, Valid: uuidPattern.MatchString(actorID)}
}

func (r *ProviderExecutionRepository) Create(ctx context.Context, exec *domain.ProviderExecution) error {
	attempt := exec.Attempt
	if attempt < 1 {
		attempt = 1
	}
	var completed sql.NullTime
	if exec.CompletedAt > 0 {
		completed = sql.NullTime{Time: toTime(exec.CompletedAt), Valid: true}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO provider_executions (id, provider_id, organization_id, capability, operation, actor_id, idempotency_key,
			status, error_code, started_at, completed_at, duration_ms, external_reference, request_id, attempt, retry_class)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`, exec.ID, exec.ProviderID, exec.TenantID, string(exec.Capability), exec.Operation, actorRef(exec.ActorID),
		nullIfEmpty(exec.IdempotencyKey), string(exec.Status), nullIfEmpty(string(exec.ErrorCode)), toTime(exec.StartedAt),
		completed, exec.DurationMs, nullIfEmpty(exec.ExternalReference), nullIfEmpty(exec.RequestID), attempt,
		nullIfEmpty(string(exec.RetryClass)))
	if err != nil {
		return fmt.Errorf("failed to create provider execution: %w", err)
	}
	return nil
}

const providerExecutionColumns = `id, provider_id, organization_id, capability, operation, actor_id, idempotency_key, status,
	error_code, started_at, completed_at, duration_ms, external_reference, request_id, attempt, retry_class`

func scanProviderExecution(scan func(...interface{}) error) (*domain.ProviderExecution, error) {
	var e domain.ProviderExecution
	var actor, key, code, extRef, requestID, retryClass sql.NullString
	var started time.Time
	var completed sql.NullTime
	var duration sql.NullInt64
	var capability, status string
	if err := scan(&e.ID, &e.ProviderID, &e.TenantID, &capability, &e.Operation, &actor, &key, &status,
		&code, &started, &completed, &duration, &extRef, &requestID, &e.Attempt, &retryClass); err != nil {
		return nil, err
	}
	e.Capability, e.Status = domain.Capability(capability), domain.ProviderExecutionStatus(status)
	e.ActorID, e.IdempotencyKey, e.ExternalReference, e.RequestID = actor.String, key.String, extRef.String, requestID.String
	e.ErrorCode, e.RetryClass = domain.ProviderErrorCode(code.String), domain.RetryClassification(retryClass.String)
	e.StartedAt, e.DurationMs = toUnix(started), duration.Int64
	if completed.Valid {
		e.CompletedAt = toUnix(completed.Time)
	}
	return &e, nil
}

func (r *ProviderExecutionRepository) one(ctx context.Context, query string, args ...interface{}) (*domain.ProviderExecution, error) {
	e, err := scanProviderExecution(r.db.QueryRowContext(ctx, query, args...).Scan)
	if err == sql.ErrNoRows {
		return nil, domain.ErrProviderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to scan provider execution: %w", err)
	}
	return e, nil
}

func (r *ProviderExecutionRepository) many(ctx context.Context, query string, args ...interface{}) ([]*domain.ProviderExecution, error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list provider executions: %w", err)
	}
	defer rows.Close()
	out := []*domain.ProviderExecution{}
	for rows.Next() {
		e, err := scanProviderExecution(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("failed to scan provider execution: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *ProviderExecutionRepository) FindByID(ctx context.Context, orgID string, id string) (*domain.ProviderExecution, error) {
	return r.one(ctx, `SELECT `+providerExecutionColumns+` FROM provider_executions WHERE id = $1 AND organization_id = $2`, id, orgID)
}

// FindByIdempotencyKey returns the latest attempt made with the key.
func (r *ProviderExecutionRepository) FindByIdempotencyKey(ctx context.Context, orgID string, key string) (*domain.ProviderExecution, error) {
	return r.one(ctx, `SELECT `+providerExecutionColumns+` FROM provider_executions
		WHERE idempotency_key = $1 AND organization_id = $2 ORDER BY started_at DESC, attempt DESC LIMIT 1`, key, orgID)
}

func (r *ProviderExecutionRepository) FindByProviderID(ctx context.Context, orgID string, providerID string, offset, limit int) ([]*domain.ProviderExecution, error) {
	return r.many(ctx, `SELECT `+providerExecutionColumns+` FROM provider_executions
		WHERE provider_id = $1 AND organization_id = $2 ORDER BY started_at DESC, attempt DESC OFFSET $3 LIMIT $4`, providerID, orgID, offset, limit)
}

// FindByRequestID returns the attempts of one orchestrated call in order.
func (r *ProviderExecutionRepository) FindByRequestID(ctx context.Context, orgID, requestID string) ([]*domain.ProviderExecution, error) {
	return r.many(ctx, `SELECT `+providerExecutionColumns+` FROM provider_executions
		WHERE request_id = $1 AND organization_id = $2 ORDER BY attempt`, requestID, orgID)
}
