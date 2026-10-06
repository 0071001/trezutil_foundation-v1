package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// The versions of a mission's plan (domain.MissionPlanRepository), on the same repository as the missions.
var _ domain.MissionPlanRepository = (*MissionRepository)(nil)

const planColumns = `id, mission_id, version, mission_type, planner, planner_version, status, fingerprint,
	COALESCE(idempotency_key, ''), created_at, superseded_at`

func scanPlan(row missionScanner) (*domain.MissionPlan, error) {
	var (
		p            domain.MissionPlan
		typ, status  string
		createdAt    time.Time
		supersededAt sql.NullTime
	)
	if err := row.Scan(&p.ID, &p.MissionID, &p.Version, &typ, &p.Planner, &p.PlannerVersion, &status, &p.Fingerprint,
		&p.IdempotencyKey, &createdAt, &supersededAt); err != nil {
		return nil, err
	}
	p.MissionType, p.Status = domain.MissionType(typ), domain.MissionPlanStatus(status)
	p.CreatedAt = createdAt.Unix()
	if supersededAt.Valid {
		p.SupersededAt = supersededAt.Time.Unix()
	}
	return &p, nil
}

// planSteps loads the steps of one plan version in order.
func (r *MissionRepository) planSteps(ctx context.Context, p *domain.MissionPlan) error {
	rows, err := r.db.QueryContext(ctx, `SELECT `+stepColumns+` FROM mission_steps WHERE mission_id = $1 AND plan_version = $2 ORDER BY position`, p.MissionID, p.Version)
	if err != nil {
		return fmt.Errorf("plan steps: %w", err)
	}
	defer rows.Close()
	steps, err := scanSteps(rows)
	if err != nil {
		return err
	}
	p.Steps = steps
	deps, err := r.PlanDependencies(ctx, p.MissionID, p.Version)
	if err != nil {
		return err
	}
	p.Dependencies = deps
	return nil
}

func (r *MissionRepository) onePlan(ctx context.Context, where string, args ...interface{}) (*domain.MissionPlan, error) {
	p, err := scanPlan(r.db.QueryRowContext(ctx, `SELECT `+planColumns+` FROM mission_plans WHERE `+where, args...))
	if errors.Is(err, sql.ErrNoRows) || (err != nil && isInvalidText(err)) {
		return nil, domain.ErrMissionPlanNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get plan: %w", err)
	}
	if err := r.planSteps(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// CurrentPlan returns the plan in force with its steps.
func (r *MissionRepository) CurrentPlan(ctx context.Context, missionID string) (*domain.MissionPlan, error) {
	return r.onePlan(ctx, `mission_id = $1 AND status = 'current'`, missionID)
}

// PlanVersion returns one version of the plan with its steps.
func (r *MissionRepository) PlanVersion(ctx context.Context, missionID string, version int) (*domain.MissionPlan, error) {
	return r.onePlan(ctx, `mission_id = $1 AND version = $2`, missionID, version)
}

// PlanByIdempotencyKey returns the plan an earlier request with that key stored.
func (r *MissionRepository) PlanByIdempotencyKey(ctx context.Context, missionID, key string) (*domain.MissionPlan, error) {
	return r.onePlan(ctx, `mission_id = $1 AND idempotency_key = $2`, missionID, key)
}

// PlanVersions lists the versions of the mission's plan, oldest first.
func (r *MissionRepository) PlanVersions(ctx context.Context, missionID string) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT version FROM mission_plans WHERE mission_id = $1 ORDER BY version`, missionID)
	if err != nil {
		if isInvalidText(err) {
			return []int{}, nil
		}
		return nil, fmt.Errorf("plan versions: %w", err)
	}
	defer rows.Close()
	out := []int{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("plan versions: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// SavePlan stores a new plan version and its steps and supersedes the current one, in one transaction. The mission
// row is locked, so two requests at once cannot both store the next version: the second sees a different current
// version and gets ErrMissionPlanConflict.
func (r *MissionRepository) SavePlan(ctx context.Context, plan *domain.MissionPlan, expectedCurrent int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM missions WHERE id = $1 FOR UPDATE`, plan.MissionID).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidText(err) {
			return domain.ErrMissionNotFound
		}
		return fmt.Errorf("save plan: %w", err)
	}
	var current int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE((SELECT version FROM mission_plans WHERE mission_id = $1 AND status = 'current'), 0)`, plan.MissionID).Scan(&current); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	if current != expectedCurrent {
		return domain.ErrMissionPlanConflict
	}
	if current != 0 { // the old version stays, as history
		if _, err := tx.ExecContext(ctx, `UPDATE mission_plans SET status = 'superseded', superseded_at = $3 WHERE mission_id = $1 AND version = $2`,
			plan.MissionID, current, time.Unix(plan.CreatedAt, 0)); err != nil {
			return fmt.Errorf("save plan: %w", err)
		}
	}
	var key interface{}
	if plan.IdempotencyKey != "" {
		key = plan.IdempotencyKey
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO mission_plans (id, mission_id, version, mission_type, planner, planner_version, status, fingerprint, idempotency_key, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		plan.ID, plan.MissionID, plan.Version, string(plan.MissionType), plan.Planner, plan.PlannerVersion, string(plan.Status), plan.Fingerprint, key,
		time.Unix(plan.CreatedAt, 0)); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrIdempotencyConflict
		}
		return fmt.Errorf("save plan: %w", err)
	}
	for i := range plan.Steps {
		if err := insertStep(ctx, tx, &plan.Steps[i]); err != nil {
			return err
		}
	}
	// The graph is stored with the steps it connects, in the same transaction: a plan never exists half-linked.
	for _, d := range plan.Dependencies {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO mission_step_dependencies (mission_id, plan_version, step_id, depends_on_step_id, created_at)
			VALUES ($1, $2, $3, $4, $5)`, plan.MissionID, plan.Version, d.StepID, d.DependsOnID, time.Unix(plan.CreatedAt, 0)); err != nil {
			return fmt.Errorf("save plan dependency: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE missions SET updated_at = $2 WHERE id = $1`, plan.MissionID, time.Unix(plan.CreatedAt, 0)); err != nil {
		return fmt.Errorf("save plan: %w", err)
	}
	return tx.Commit()
}
