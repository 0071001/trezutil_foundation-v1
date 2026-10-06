package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// The attempts to run mission steps (domain.MissionExecutionRepository), on the same repository as the missions.
var _ domain.MissionExecutionRepository = (*MissionRepository)(nil)

const executionColumns = `id, step_id, mission_id, attempt, status, action_type, COALESCE(idempotency_key, ''), result_reference,
	result_payload, runtime_status, error_code, started_at, completed_at, created_at`

func scanExecution(row missionScanner) (*domain.MissionStepExecution, error) {
	var (
		e                    domain.MissionStepExecution
		status, action       string
		payload              []byte
		startedAt, createdAt time.Time
		completedAt          sql.NullTime
	)
	if err := row.Scan(&e.ID, &e.StepID, &e.MissionID, &e.Attempt, &status, &action, &e.IdempotencyKey, &e.ResultReference,
		&payload, &e.RuntimeStatus, &e.ErrorCode, &startedAt, &completedAt, &createdAt); err != nil {
		return nil, err
	}
	e.Status, e.ActionType = domain.ExecutionStatus(status), domain.CommerceActionType(action)
	e.ResultPayload = map[string]interface{}{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &e.ResultPayload)
	}
	e.StartedAt, e.CreatedAt = startedAt.Unix(), createdAt.Unix()
	if completedAt.Valid {
		e.CompletedAt = completedAt.Time.Unix()
	}
	return &e, nil
}

// GetStep returns a step of the mission; a step of another mission is not found.
func (r *MissionRepository) GetStep(ctx context.Context, missionID, stepID string) (*domain.MissionStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+stepColumns+` FROM mission_steps WHERE id = $1 AND mission_id = $2`, stepID, missionID)
	if err != nil {
		if isInvalidText(err) {
			return nil, domain.ErrMissionStepNotFound
		}
		return nil, fmt.Errorf("get step: %w", err)
	}
	defer rows.Close()
	steps, err := scanSteps(rows)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, domain.ErrMissionStepNotFound
	}
	return &steps[0], nil
}

// ExecutionByKey returns the step's execution made with that key.
func (r *MissionRepository) ExecutionByKey(ctx context.Context, stepID, key string) (*domain.MissionStepExecution, error) {
	return oneExecution(ctx, r.db, `step_id = $1 AND idempotency_key = $2`, stepID, key)
}

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func oneExecution(ctx context.Context, db queryRower, where string, args ...interface{}) (*domain.MissionStepExecution, error) {
	e, err := scanExecution(db.QueryRowContext(ctx, `SELECT `+executionColumns+` FROM mission_step_executions WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) || (err != nil && isInvalidText(err)) {
		return nil, domain.ErrMissionExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	return e, nil
}

// BeginExecution claims an attempt to run the step. The step row is locked, so two requests at once cannot both
// start: the second finds the first in progress. A request repeating a key returns the attempt that key made.
func (r *MissionRepository) BeginExecution(ctx context.Context, in domain.BeginExecutionInput) (*domain.MissionStepExecution, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var statusRaw string
	err = tx.QueryRowContext(ctx, `SELECT status FROM mission_steps WHERE id = $1 AND mission_id = $2 FOR UPDATE`, in.StepID, in.MissionID).Scan(&statusRaw)
	if errors.Is(err, sql.ErrNoRows) || (err != nil && isInvalidText(err)) {
		return nil, false, domain.ErrMissionStepNotFound
	}
	if err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}

	if in.IdempotencyKey != "" {
		if e, err := oneExecution(ctx, tx, `step_id = $1 AND idempotency_key = $2`, in.StepID, in.IdempotencyKey); err == nil {
			return e, true, nil
		} else if !errors.Is(err, domain.ErrMissionExecutionNotFound) {
			return nil, false, err
		}
	}

	var attempts int
	var latest *domain.MissionStepExecution
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(attempt), 0) FROM mission_step_executions WHERE step_id = $1`, in.StepID).Scan(&attempts); err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	if attempts > 0 {
		if latest, err = oneExecution(ctx, tx, `step_id = $1 AND attempt = $2`, in.StepID, attempts); err != nil {
			return nil, false, err
		}
	}
	step := &domain.MissionStep{ID: in.StepID, MissionID: in.MissionID, Status: domain.MissionStepStatus(statusRaw)}
	abandon, err := domain.ExecutionClaim(step, latest, in.Now, in.StaleAfter)
	if err != nil {
		return nil, false, err
	}
	now := time.Unix(in.Now, 0)
	if abandon && latest != nil && latest.Status == domain.ExecutionInProgress { // the old attempt never ended
		if _, err := tx.ExecContext(ctx, `UPDATE mission_step_executions SET status = 'failed', error_code = $2, completed_at = $3 WHERE id = $1 AND status = 'in_progress'`,
			latest.ID, domain.ErrorCodeAbandoned, now); err != nil {
			return nil, false, fmt.Errorf("begin execution: %w", err)
		}
	}

	exec := &domain.MissionStepExecution{
		ID: domain.NewID(), StepID: in.StepID, MissionID: in.MissionID, Attempt: attempts + 1, Status: domain.ExecutionInProgress,
		ActionType: in.ActionType, IdempotencyKey: in.IdempotencyKey, ResultPayload: map[string]interface{}{}, StartedAt: in.Now, CreatedAt: in.Now,
	}
	var key interface{}
	if exec.IdempotencyKey != "" {
		key = exec.IdempotencyKey
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mission_step_executions (id, step_id, mission_id, attempt, status, action_type, idempotency_key, started_at, created_at)
		VALUES ($1, $2, $3, $4, 'in_progress', $5, $6, $7, $7)`,
		exec.ID, exec.StepID, exec.MissionID, exec.Attempt, string(exec.ActionType), key, now); err != nil {
		if isUniqueViolation(err) { // another request got in first
			return nil, false, domain.ErrStepInProgress
		}
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mission_steps SET status = 'in_progress', action_ref = $2, result_ref = '', updated_at = $3 WHERE id = $1`, in.StepID, exec.ID, now); err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE missions SET updated_at = $2 WHERE id = $1`, in.MissionID, now); err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("begin execution: %w", err)
	}
	return exec, false, nil
}

// FinishExecution records how an attempt ended and moves its step to completed or failed.
func (r *MissionRepository) FinishExecution(ctx context.Context, exec *domain.MissionStepExecution, now int64) error {
	if exec.Status != domain.ExecutionCompleted && exec.Status != domain.ExecutionFailed {
		return fmt.Errorf("finish execution: %w", domain.ErrInvalidStepTransition)
	}
	payload, err := json.Marshal(exec.ResultPayload)
	if err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	when := time.Unix(now, 0)
	res, err := tx.ExecContext(ctx, `
		UPDATE mission_step_executions SET status = $2, result_reference = $3, result_payload = $4, runtime_status = $5, error_code = $6, completed_at = $7
		WHERE id = $1 AND status = 'in_progress'`,
		exec.ID, string(exec.Status), exec.ResultReference, payload, exec.RuntimeStatus, exec.ErrorCode, when)
	if err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrExecutionSuperseded // it was given up on while it ran
	}
	stepStatus, resultRef := domain.StepFailed, ""
	if exec.Status == domain.ExecutionCompleted {
		stepStatus, resultRef = domain.StepCompleted, exec.ResultReference
	}
	if _, err := tx.ExecContext(ctx, `UPDATE mission_steps SET status = $2, result_ref = $3, updated_at = $4 WHERE id = $1`, exec.StepID, string(stepStatus), resultRef, when); err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE missions SET updated_at = $2 WHERE id = $1`, exec.MissionID, when); err != nil {
		return fmt.Errorf("finish execution: %w", err)
	}
	exec.CompletedAt = now
	return tx.Commit()
}

// LatestExecutions returns the latest attempt of each step of the mission that has one.
func (r *MissionRepository) LatestExecutions(ctx context.Context, missionID string) (map[string]*domain.MissionStepExecution, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT DISTINCT ON (step_id) `+executionColumns+` FROM mission_step_executions
		WHERE mission_id = $1 ORDER BY step_id, attempt DESC`, missionID)
	if err != nil {
		if isInvalidText(err) {
			return map[string]*domain.MissionStepExecution{}, nil
		}
		return nil, fmt.Errorf("latest executions: %w", err)
	}
	defer rows.Close()
	out := map[string]*domain.MissionStepExecution{}
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, fmt.Errorf("latest executions: %w", err)
		}
		out[e.StepID] = e
	}
	return out, rows.Err()
}
