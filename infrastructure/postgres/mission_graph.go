package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// The execution graph of a mission and its run lock (domain.MissionGraphRepository, domain.MissionRunStore), on the
// same repository as the missions.
var (
	_ domain.MissionGraphRepository = (*MissionRepository)(nil)
	_ domain.MissionRunStore        = (*MissionRepository)(nil)
)

// PlanDependencies returns the dependencies of one plan version of the mission.
func (r *MissionRepository) PlanDependencies(ctx context.Context, missionID string, planVersion int) ([]domain.StepDependency, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT step_id, depends_on_step_id FROM mission_step_dependencies
		WHERE mission_id = $1 AND plan_version = $2 ORDER BY step_id, depends_on_step_id`, missionID, planVersion)
	if err != nil {
		if isInvalidText(err) {
			return []domain.StepDependency{}, nil
		}
		return nil, fmt.Errorf("plan dependencies: %w", err)
	}
	defer rows.Close()
	out := []domain.StepDependency{}
	for rows.Next() {
		var d domain.StepDependency
		if err := rows.Scan(&d.StepID, &d.DependsOnID); err != nil {
			return nil, fmt.Errorf("plan dependencies: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// Upstream returns the steps a step depends on, each with its latest execution (nil when it has none).
func (r *MissionRepository) Upstream(ctx context.Context, stepID string) ([]domain.UpstreamStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+prefixed("s", stepColumns)+`
		FROM mission_step_dependencies d JOIN mission_steps s ON s.id = d.depends_on_step_id
		WHERE d.step_id = $1 ORDER BY s.position`, stepID)
	if err != nil {
		if isInvalidText(err) {
			return []domain.UpstreamStep{}, nil
		}
		return nil, fmt.Errorf("upstream: %w", err)
	}
	steps, err := scanSteps(rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := make([]domain.UpstreamStep, 0, len(steps))
	for _, s := range steps {
		up := domain.UpstreamStep{Step: s}
		latest, err := oneExecution(ctx, r.db, `step_id = $1 AND attempt = (SELECT MAX(attempt) FROM mission_step_executions WHERE step_id = $1)`, s.ID)
		switch {
		case err == nil:
			up.Execution = latest
		case errors.Is(err, domain.ErrMissionExecutionNotFound):
		default:
			return nil, err
		}
		out = append(out, up)
	}
	return out, nil
}

// prefixed puts a table alias before every column of a comma-separated list.
func prefixed(alias, columns string) string {
	out, start := "", 0
	cols := []string{}
	for i := 0; i <= len(columns); i++ {
		if i == len(columns) || columns[i] == ',' {
			cols = append(cols, columns[start:i])
			start = i + 1
		}
	}
	for i, c := range cols {
		for len(c) > 0 && (c[0] == ' ' || c[0] == '\n' || c[0] == '\t') {
			c = c[1:]
		}
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}

// AcquireRun takes the mission's run lock for ttl in one atomic statement. A lease that has run out is taken over.
func (r *MissionRepository) AcquireRun(ctx context.Context, missionID, token string, now time.Time, ttl time.Duration) (bool, error) {
	var got string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO mission_run_locks (mission_id, token, acquired_at, expires_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (mission_id) DO UPDATE SET token = EXCLUDED.token, acquired_at = EXCLUDED.acquired_at, expires_at = EXCLUDED.expires_at
		WHERE mission_run_locks.expires_at <= $3
		RETURNING token::text`, missionID, token, now, now.Add(ttl)).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil // held by another orchestration whose lease is still good
	}
	if err != nil {
		if isInvalidText(err) {
			return false, domain.ErrMissionNotFound
		}
		return false, fmt.Errorf("acquire run: %w", err)
	}
	return got == token, nil
}

// ExtendRun renews the lease of the lock the holder still has.
func (r *MissionRepository) ExtendRun(ctx context.Context, missionID, token string, now time.Time, ttl time.Duration) error {
	res, err := r.db.ExecContext(ctx, `UPDATE mission_run_locks SET expires_at = $4 WHERE mission_id = $1 AND token = $2 AND expires_at > $3`,
		missionID, token, now, now.Add(ttl))
	if err != nil {
		return fmt.Errorf("extend run: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrMissionBusy
	}
	return nil
}

// ReleaseRun gives up the holder's own lock.
func (r *MissionRepository) ReleaseRun(ctx context.Context, missionID, token string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM mission_run_locks WHERE mission_id = $1 AND token = $2`, missionID, token); err != nil {
		return fmt.Errorf("release run: %w", err)
	}
	return nil
}

// RunInProgress reports whether the mission's run lock is held and its lease is still good.
func (r *MissionRepository) RunInProgress(ctx context.Context, missionID string, now time.Time) (bool, error) {
	var held bool
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM mission_run_locks WHERE mission_id = $1 AND expires_at > $2)`, missionID, now).Scan(&held)
	if err != nil {
		if isInvalidText(err) {
			return false, nil
		}
		return false, fmt.Errorf("run in progress: %w", err)
	}
	return held, nil
}

// TransitionMission moves the mission from one status to another only if it is in the first.
func (r *MissionRepository) TransitionMission(ctx context.Context, missionID string, from, to domain.MissionStatus, now time.Time) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE missions SET status = $3, updated_at = $4 WHERE id = $1 AND status = $2`, missionID, string(from), string(to), now)
	if err != nil {
		return false, fmt.Errorf("transition mission: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
