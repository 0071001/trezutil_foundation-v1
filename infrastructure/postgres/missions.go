package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

// MissionRepository stores missions and their steps (domain.MissionRepository).
type MissionRepository struct {
	db *sql.DB
}

// NewMissionRepository creates a MissionRepository.
func NewMissionRepository(db *sql.DB) *MissionRepository {
	return &MissionRepository{db: db}
}

var _ domain.MissionRepository = (*MissionRepository)(nil)

const missionColumns = `id, owner_id, mission_type, title, objective, status, location, start_date, end_date,
	participants, budget_minor, currency, preferences, COALESCE(idempotency_key, ''), created_at, updated_at`

// missionScanner is a *sql.Row or *sql.Rows.
type missionScanner interface {
	Scan(dest ...interface{}) error
}

func scanMission(row missionScanner) (*domain.Mission, error) {
	var (
		m                    domain.Mission
		typ, status          string
		start, end           sql.NullTime
		prefs                []byte
		createdAt, updatedAt time.Time
	)
	if err := row.Scan(&m.ID, &m.OwnerID, &typ, &m.Title, &m.Objective, &status, &m.Location, &start, &end,
		&m.Participants, &m.BudgetMinor, &m.Currency, &prefs, &m.IdempotencyKey, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	m.Type, m.Status = domain.MissionType(typ), domain.MissionStatus(status)
	if start.Valid {
		m.StartDate = start.Time.UTC().Unix()
	}
	if end.Valid {
		m.EndDate = end.Time.UTC().Unix()
	}
	m.Preferences = map[string]string{}
	if len(prefs) > 0 {
		_ = json.Unmarshal(prefs, &m.Preferences)
	}
	m.CreatedAt, m.UpdatedAt = createdAt.Unix(), updatedAt.Unix()
	return &m, nil
}

// dateOrNil stores a unix date as a DATE, or NULL when it is not set.
func dateOrNil(unix int64) interface{} {
	if unix == 0 {
		return nil
	}
	return time.Unix(unix, 0).UTC().Format("2006-01-02")
}

// isInvalidText reports a malformed UUID (22P02): an id that cannot exist.
func isInvalidText(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "22P02"
}

// Create stores a new mission. A second mission with the same owner and idempotency key is
// domain.ErrIdempotencyConflict: the caller reads the first one back.
func (r *MissionRepository) Create(ctx context.Context, m *domain.Mission) error {
	prefs, err := json.Marshal(m.Preferences)
	if err != nil {
		return fmt.Errorf("create mission: %w", err)
	}
	var key interface{}
	if m.IdempotencyKey != "" {
		key = m.IdempotencyKey
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO missions (id, owner_id, mission_type, title, objective, status, location, start_date, end_date,
		                      participants, budget_minor, currency, preferences, idempotency_key, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		m.ID, m.OwnerID, string(m.Type), m.Title, m.Objective, string(m.Status), m.Location, dateOrNil(m.StartDate), dateOrNil(m.EndDate),
		m.Participants, m.BudgetMinor, m.Currency, prefs, key, time.Unix(m.CreatedAt, 0), time.Unix(m.UpdatedAt, 0))
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrIdempotencyConflict
		}
		return fmt.Errorf("create mission: %w", err)
	}
	return nil
}

// FindByIdempotencyKey returns the owner's mission created with that key.
func (r *MissionRepository) FindByIdempotencyKey(ctx context.Context, ownerID, key string) (*domain.Mission, error) {
	m, err := scanMission(r.db.QueryRowContext(ctx, `SELECT `+missionColumns+` FROM missions WHERE owner_id = $1 AND idempotency_key = $2`, ownerID, key))
	if errors.Is(err, sql.ErrNoRows) || (err != nil && isInvalidText(err)) {
		return nil, domain.ErrMissionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find mission: %w", err)
	}
	return m, nil
}

// Get returns a mission by id.
func (r *MissionRepository) Get(ctx context.Context, id string) (*domain.Mission, error) {
	m, err := scanMission(r.db.QueryRowContext(ctx, `SELECT `+missionColumns+` FROM missions WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) || (err != nil && isInvalidText(err)) {
		return nil, domain.ErrMissionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get mission: %w", err)
	}
	return m, nil
}

// ListByOwner returns the owner's missions, most recently changed first, with the total.
func (r *MissionRepository) ListByOwner(ctx context.Context, ownerID string, status domain.MissionStatus, page domain.Page) ([]domain.Mission, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM missions WHERE owner_id = $1 AND ($2 = '' OR status = $2)`, ownerID, string(status)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count missions: %w", err)
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+missionColumns+` FROM missions
		WHERE owner_id = $1 AND ($2 = '' OR status = $2)
		ORDER BY updated_at DESC, id LIMIT $3 OFFSET $4`, ownerID, string(status), page.Limit, page.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list missions: %w", err)
	}
	defer rows.Close()
	out := []domain.Mission{}
	for rows.Next() {
		m, err := scanMission(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("list missions: %w", err)
		}
		out = append(out, *m)
	}
	return out, total, rows.Err()
}

// Update stores the mission's editable fields and status.
func (r *MissionRepository) Update(ctx context.Context, m *domain.Mission) error {
	prefs, err := json.Marshal(m.Preferences)
	if err != nil {
		return fmt.Errorf("update mission: %w", err)
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE missions SET mission_type = $2, title = $3, objective = $4, status = $5, location = $6, start_date = $7,
		       end_date = $8, participants = $9, budget_minor = $10, currency = $11, preferences = $12, updated_at = $13
		WHERE id = $1`,
		m.ID, string(m.Type), m.Title, m.Objective, string(m.Status), m.Location, dateOrNil(m.StartDate), dateOrNil(m.EndDate),
		m.Participants, m.BudgetMinor, m.Currency, prefs, time.Unix(m.UpdatedAt, 0))
	if err != nil {
		return fmt.Errorf("update mission: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrMissionNotFound
	}
	return nil
}

// AddStep stores the step at the mission's next free position. The mission row is locked for the insert so two
// additions at once cannot take the same position or pass the limit together.
func (r *MissionRepository) AddStep(ctx context.Context, s *domain.MissionStep, maxSteps int) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("add step: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM missions WHERE id = $1 FOR UPDATE`, s.MissionID).Scan(&locked); err != nil {
		if errors.Is(err, sql.ErrNoRows) || isInvalidText(err) {
			return domain.ErrMissionNotFound
		}
		return fmt.Errorf("add step: %w", err)
	}
	// Steps added by hand are plan version 0: the limit and the positions are theirs, apart from any plan's.
	var count, next int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(position), 0) + 1 FROM mission_steps WHERE mission_id = $1 AND plan_version = 0`, s.MissionID).Scan(&count, &next); err != nil {
		return fmt.Errorf("add step: %w", err)
	}
	if maxSteps > 0 && count >= maxSteps {
		return domain.ErrMissionStepLimit
	}
	s.Position, s.PlanVersion = next, 0
	if err := insertStep(ctx, tx, s); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE missions SET updated_at = $2 WHERE id = $1`, s.MissionID, time.Unix(s.UpdatedAt, 0)); err != nil {
		return fmt.Errorf("add step: %w", err)
	}
	return tx.Commit()
}

// Steps lists the mission's steps in order: those added by hand, then those of its current plan. Steps of
// superseded plan versions are history and are read with the plan version.
func (r *MissionRepository) Steps(ctx context.Context, missionID string) ([]domain.MissionStep, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+stepColumns+` FROM mission_steps
		WHERE mission_id = $1 AND (plan_version = 0 OR plan_version = COALESCE(
			(SELECT version FROM mission_plans WHERE mission_id = $1 AND status = 'current'), -1))
		ORDER BY plan_version, position`, missionID)
	if err != nil {
		if isInvalidText(err) {
			return []domain.MissionStep{}, nil
		}
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer rows.Close()
	return scanSteps(rows)
}

const stepColumns = `id, mission_id, step_type, title, description, status, position, action_type, action_support, plan_version, action_ref, result_ref, created_at, updated_at`

// stepInserter is a *sql.DB or *sql.Tx.
type stepInserter interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func insertStep(ctx context.Context, db stepInserter, s *domain.MissionStep) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO mission_steps (id, mission_id, step_type, title, description, status, position, action_type, action_support, plan_version, action_ref, result_ref, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		s.ID, s.MissionID, s.Type, s.Title, s.Description, string(s.Status), s.Position, string(s.ActionType), string(s.ActionSupport), s.PlanVersion,
		s.ActionRef, s.ResultRef, time.Unix(s.CreatedAt, 0), time.Unix(s.UpdatedAt, 0)); err != nil {
		return fmt.Errorf("add step: %w", err)
	}
	return nil
}

func scanSteps(rows *sql.Rows) ([]domain.MissionStep, error) {
	out := []domain.MissionStep{}
	for rows.Next() {
		var (
			s                       domain.MissionStep
			status, action, support string
			createdAt, updatedAt    time.Time
		)
		if err := rows.Scan(&s.ID, &s.MissionID, &s.Type, &s.Title, &s.Description, &status, &s.Position, &action, &support, &s.PlanVersion,
			&s.ActionRef, &s.ResultRef, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("list steps: %w", err)
		}
		s.Status, s.ActionType, s.ActionSupport = domain.MissionStepStatus(status), domain.CommerceActionType(action), domain.StepActionSupport(support)
		s.CreatedAt, s.UpdatedAt = createdAt.Unix(), updatedAt.Unix()
		out = append(out, s)
	}
	return out, rows.Err()
}
