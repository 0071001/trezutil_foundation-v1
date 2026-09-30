package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

// DispatchRepository is the PostgreSQL implementation of domain.DispatchRepository.
// Methods that change several rows run in one transaction and lock the rows
// they read (SELECT ... FOR UPDATE); the partial unique indexes of migration
// 000022 are the backstop: one active job per agent, one pending offer per job.
type DispatchRepository struct {
	db *sql.DB
}

// NewDispatchRepository creates a DispatchRepository.
func NewDispatchRepository(db *sql.DB) *DispatchRepository {
	return &DispatchRepository{db: db}
}

var _ domain.DispatchRepository = (*DispatchRepository)(nil)

const agentColumns = `id, user_id, kind, vehicle, status, lat, lng, position_at, version, created_at, updated_at`

type rowScanner interface{ Scan(dest ...any) error }

func scanAgent(s rowScanner) (*domain.DispatchAgent, error) {
	var (
		a        domain.DispatchAgent
		lat, lng sql.NullFloat64
		posAt    sql.NullTime
	)
	if err := s.Scan(&a.ID, &a.UserID, &a.Kind, &a.Vehicle, &a.Status, &lat, &lng, &posAt, &a.Version, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	if lat.Valid && lng.Valid {
		a.Position = &domain.GeoPoint{Latitude: lat.Float64, Longitude: lng.Float64}
	}
	if posAt.Valid {
		t := posAt.Time
		a.PositionAt = &t
	}
	return &a, nil
}

const jobColumns = `id, organization_id, kind, order_id, customer_id, pickup_lat, pickup_lng, pickup_address,
	dropoff_lat, dropoff_lng, dropoff_address, status, agent_id, fee_minor, currency, cash_due_minor,
	cash_collected_minor, proof_code, proof_attempts, COALESCE(cancel_reason, ''), version, created_at, updated_at`

func scanJob(s rowScanner) (*domain.DispatchJob, error) {
	var (
		j                     domain.DispatchJob
		org, order, agent     sql.NullString
		cashCollected         sql.NullInt64
		pickupLat, pickupLng  float64
		dropoffLat, dropoffLn float64
	)
	if err := s.Scan(&j.ID, &org, &j.Kind, &order, &j.CustomerID, &pickupLat, &pickupLng, &j.PickupAddress,
		&dropoffLat, &dropoffLn, &j.DropoffAddress, &j.Status, &agent, &j.FeeMinor, &j.Currency, &j.CashDueMinor,
		&cashCollected, &j.ProofCode, &j.ProofAttempts, &j.CancelReason, &j.Version, &j.CreatedAt, &j.UpdatedAt); err != nil {
		return nil, err
	}
	j.Pickup = domain.GeoPoint{Latitude: pickupLat, Longitude: pickupLng}
	j.Dropoff = domain.GeoPoint{Latitude: dropoffLat, Longitude: dropoffLn}
	if org.Valid {
		j.OrganizationID = &org.String
	}
	if order.Valid {
		j.OrderID = &order.String
	}
	if agent.Valid {
		j.AgentID = &agent.String
	}
	if cashCollected.Valid {
		v := cashCollected.Int64
		j.CashCollectedMinor = &v
	}
	return &j, nil
}

// ---------------------------------------------------------------- agents

func (r *DispatchRepository) CreateAgent(ctx context.Context, a *domain.DispatchAgent) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO dispatch_agents (id, user_id, kind, vehicle, status, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.UserID, a.Kind, a.Vehicle, a.Status, a.Version, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("%w: already registered", domain.ErrInvalidAgent)
		}
		return fmt.Errorf("failed to create dispatch agent: %w", err)
	}
	return nil
}

func (r *DispatchRepository) FindAgent(ctx context.Context, id string) (*domain.DispatchAgent, error) {
	a, err := scanAgent(r.db.QueryRowContext(ctx, `SELECT `+agentColumns+` FROM dispatch_agents WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAgentNotFound
	}
	return a, err
}

func (r *DispatchRepository) FindAgentByUser(ctx context.Context, userID string, kind domain.AgentKind) (*domain.DispatchAgent, error) {
	a, err := scanAgent(r.db.QueryRowContext(ctx, `SELECT `+agentColumns+` FROM dispatch_agents WHERE user_id = $1 AND kind = $2`, userID, kind))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrAgentNotFound
	}
	return a, err
}

func (r *DispatchRepository) GoOnline(ctx context.Context, agentID string, p domain.GeoPoint, now time.Time) error {
	if err := p.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE dispatch_agents SET status = 'ONLINE', lat = $2, lng = $3, position_at = $4, version = version + 1, updated_at = $4
		WHERE id = $1 AND status <> 'BUSY'`, agentID, p.Latitude, p.Longitude, now)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM dispatch_agents WHERE id = $1)`, agentID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return domain.ErrAgentNotFound
		}
		return domain.ErrAgentHasActiveJob
	}
	if err := recordPosition(ctx, tx, agentID, p, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *DispatchRepository) GoOffline(ctx context.Context, agentID string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM dispatch_agents WHERE id = $1 FOR UPDATE`, agentID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrAgentNotFound
		}
		return err
	}
	if status == string(domain.AgentBusy) {
		return domain.ErrAgentHasActiveJob
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_agents SET status = 'OFFLINE', version = version + 1, updated_at = $2 WHERE id = $1`, agentID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func recordPosition(ctx context.Context, tx *sql.Tx, agentID string, p domain.GeoPoint, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO dispatch_positions (agent_id, lat, lng, at) VALUES ($1, $2, $3, $4)`,
		agentID, p.Latitude, p.Longitude, now); err != nil {
		return err
	}
	// Breadcrumbs are for live tracking only: keep the last hour.
	_, err := tx.ExecContext(ctx, `DELETE FROM dispatch_positions WHERE agent_id = $1 AND at < $2`, agentID, now.Add(-time.Hour))
	return err
}

func (r *DispatchRepository) UpdatePosition(ctx context.Context, agentID string, p domain.GeoPoint, now time.Time) error {
	if err := p.Validate(); err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `
		UPDATE dispatch_agents SET lat = $2, lng = $3, position_at = $4, updated_at = $4
		WHERE id = $1 AND status IN ('ONLINE', 'BUSY')`, agentID, p.Latitude, p.Longitude, now)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrAgentUnavailable
	}
	if err := recordPosition(ctx, tx, agentID, p, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *DispatchRepository) NearbyFreeAgents(ctx context.Context, kind domain.AgentKind, near domain.GeoPoint, radiusKm float64, jobID string, now time.Time, limit int) ([]domain.AgentCandidate, error) {
	if err := near.Validate(); err != nil {
		return nil, err
	}
	if radiusKm <= 0 || limit <= 0 {
		return nil, nil
	}
	// Bounding box in SQL, exact distance in Go: no geo extension is required.
	dLat := radiusKm / 110.574
	cos := math.Max(0.01, math.Cos(near.Latitude*math.Pi/180))
	dLng := radiusKm / (111.320 * cos)
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+agentColumns+` FROM dispatch_agents a
		WHERE kind = $1 AND status = 'ONLINE'
		  AND lat BETWEEN $2 AND $3 AND lng BETWEEN $4 AND $5
		  AND position_at > $6
		  AND NOT EXISTS (SELECT 1 FROM dispatch_offers o WHERE o.job_id = $7 AND o.agent_id = a.id)`,
		kind, near.Latitude-dLat, near.Latitude+dLat, near.Longitude-dLng, near.Longitude+dLng,
		now.Add(-domain.StalePositionAfter), jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AgentCandidate
	for rows.Next() {
		a, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		if a.Position == nil {
			continue
		}
		if d := domain.DistanceKm(near, *a.Position); d <= radiusKm {
			out = append(out, domain.AgentCandidate{Agent: a, DistanceKm: d})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DistanceKm != out[j].DistanceKm {
			return out[i].DistanceKm < out[j].DistanceKm
		}
		return out[i].Agent.ID < out[j].Agent.ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *DispatchRepository) LatestPosition(ctx context.Context, agentID string) (*domain.GeoPoint, *time.Time, error) {
	a, err := r.FindAgent(ctx, agentID)
	if err != nil {
		return nil, nil, err
	}
	return a.Position, a.PositionAt, nil
}

// ------------------------------------------------------------------ jobs

func (r *DispatchRepository) CreateJob(ctx context.Context, j *domain.DispatchJob) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO dispatch_jobs (id, organization_id, kind, order_id, customer_id, pickup_lat, pickup_lng, pickup_address,
			dropoff_lat, dropoff_lng, dropoff_address, status, fee_minor, currency, cash_due_minor, proof_code, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)`,
		j.ID, j.OrganizationID, j.Kind, j.OrderID, j.CustomerID, j.Pickup.Latitude, j.Pickup.Longitude, j.PickupAddress,
		j.Dropoff.Latitude, j.Dropoff.Longitude, j.DropoffAddress, j.Status, j.FeeMinor, j.Currency, j.CashDueMinor,
		j.ProofCode, j.Version, j.CreatedAt, j.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrJobAlreadyExists
		}
		return fmt.Errorf("failed to create dispatch job: %w", err)
	}
	return nil
}

func (r *DispatchRepository) FindJob(ctx context.Context, id string) (*domain.DispatchJob, error) {
	j, err := scanJob(r.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM dispatch_jobs WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrJobNotFound
	}
	return j, err
}

func (r *DispatchRepository) FindJobByOrder(ctx context.Context, orderID string) (*domain.DispatchJob, error) {
	j, err := scanJob(r.db.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM dispatch_jobs WHERE order_id = $1`, orderID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrJobNotFound
	}
	return j, err
}

func (r *DispatchRepository) listJobs(ctx context.Context, where string, args ...any) ([]*domain.DispatchJob, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+jobColumns+` FROM dispatch_jobs WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.DispatchJob
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func clampLimit(limit int) int {
	if limit <= 0 || limit > 100 {
		return 50
	}
	return limit
}

func (r *DispatchRepository) ListJobsForCustomer(ctx context.Context, customerID string, limit int) ([]*domain.DispatchJob, error) {
	return r.listJobs(ctx, `customer_id = $1 ORDER BY created_at DESC LIMIT $2`, customerID, clampLimit(limit))
}

func (r *DispatchRepository) ListJobsForAgent(ctx context.Context, agentID string, activeOnly bool, limit int) ([]*domain.DispatchJob, error) {
	if activeOnly {
		return r.listJobs(ctx, `agent_id = $1 AND status IN ('ASSIGNED', 'AT_PICKUP', 'IN_TRANSIT') ORDER BY created_at DESC LIMIT $2`, agentID, clampLimit(limit))
	}
	return r.listJobs(ctx, `agent_id = $1 ORDER BY created_at DESC LIMIT $2`, agentID, clampLimit(limit))
}

func (r *DispatchRepository) JobsNeedingMatch(ctx context.Context, limit int) ([]*domain.DispatchJob, error) {
	return r.listJobs(ctx, `status = 'REQUESTED' ORDER BY created_at ASC LIMIT $1`, clampLimit(limit))
}

// lockJob reads a job inside a transaction with a row lock.
func lockJob(ctx context.Context, tx *sql.Tx, id string) (*domain.DispatchJob, error) {
	j, err := scanJob(tx.QueryRowContext(ctx, `SELECT `+jobColumns+` FROM dispatch_jobs WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrJobNotFound
	}
	return j, err
}

func (r *DispatchRepository) MarkNoAgent(ctx context.Context, jobID string, now time.Time) error {
	return r.moveJob(ctx, jobID, domain.JobRequested, domain.JobNoAgent, now)
}

func (r *DispatchRepository) Requeue(ctx context.Context, jobID string, now time.Time) error {
	return r.moveJob(ctx, jobID, domain.JobNoAgent, domain.JobRequested, now)
}

func (r *DispatchRepository) moveJob(ctx context.Context, jobID string, from, to domain.JobStatus, now time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE dispatch_jobs SET status = $3, version = version + 1, updated_at = $4 WHERE id = $1 AND status = $2`,
		jobID, from, to, now)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrInvalidJobTransition
	}
	return nil
}

// ---------------------------------------------------------------- offers

func (r *DispatchRepository) OfferJob(ctx context.Context, jobID, agentID string, expiresAt, now time.Time) (*domain.DispatchOffer, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job, err := lockJob(ctx, tx, jobID)
	if err != nil {
		return nil, err
	}
	if job.Status != domain.JobRequested {
		return nil, domain.ErrInvalidJobTransition
	}
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM dispatch_agents WHERE id = $1 FOR UPDATE`, agentID).Scan(&status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrAgentNotFound
		}
		return nil, err
	}
	if status != string(domain.AgentOnline) {
		return nil, domain.ErrAgentUnavailable
	}
	offer := &domain.DispatchOffer{ID: domain.NewID(), JobID: jobID, AgentID: agentID, Status: domain.OfferPending, ExpiresAt: expiresAt, CreatedAt: now}
	if _, err := tx.ExecContext(ctx, `INSERT INTO dispatch_offers (id, job_id, agent_id, status, expires_at, created_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		offer.ID, offer.JobID, offer.AgentID, offer.Status, offer.ExpiresAt, offer.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAgentUnavailable
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'OFFERED', version = version + 1, updated_at = $2 WHERE id = $1`, jobID, now); err != nil {
		return nil, err
	}
	return offer, tx.Commit()
}

func scanOffer(s rowScanner) (*domain.DispatchOffer, error) {
	var o domain.DispatchOffer
	if err := s.Scan(&o.ID, &o.JobID, &o.AgentID, &o.Status, &o.ExpiresAt, &o.CreatedAt); err != nil {
		return nil, err
	}
	return &o, nil
}

const offerColumns = `id, job_id, agent_id, status, expires_at, created_at`

func (r *DispatchRepository) FindOffer(ctx context.Context, id string) (*domain.DispatchOffer, error) {
	o, err := scanOffer(r.db.QueryRowContext(ctx, `SELECT `+offerColumns+` FROM dispatch_offers WHERE id = $1`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrOfferNotFound
	}
	return o, err
}

func (r *DispatchRepository) PendingOffersForAgent(ctx context.Context, agentID string, now time.Time) ([]*domain.DispatchOffer, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+offerColumns+` FROM dispatch_offers WHERE agent_id = $1 AND status = 'PENDING' AND expires_at > $2 ORDER BY created_at`, agentID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*domain.DispatchOffer
	for rows.Next() {
		o, err := scanOffer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// lockOffer reads an offer inside a transaction with a row lock.
func lockOffer(ctx context.Context, tx *sql.Tx, id string) (*domain.DispatchOffer, error) {
	o, err := scanOffer(tx.QueryRowContext(ctx, `SELECT `+offerColumns+` FROM dispatch_offers WHERE id = $1 FOR UPDATE`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrOfferNotFound
	}
	return o, err
}

func (r *DispatchRepository) AcceptOffer(ctx context.Context, offerID, agentID string, now time.Time) (*domain.DispatchJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	offer, err := lockOffer(ctx, tx, offerID)
	if err != nil {
		return nil, err
	}
	// An offer belongs to one agent; anyone else sees it as not found.
	if offer.AgentID != agentID {
		return nil, domain.ErrOfferNotFound
	}
	if !offer.Usable(now) {
		return nil, domain.ErrOfferExpired
	}
	job, err := lockJob(ctx, tx, offer.JobID)
	if err != nil {
		return nil, err
	}
	if job.Status != domain.JobOffered {
		return nil, domain.ErrOfferExpired
	}
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM dispatch_agents WHERE id = $1 FOR UPDATE`, agentID).Scan(&status); err != nil {
		return nil, err
	}
	if status != string(domain.AgentOnline) {
		return nil, domain.ErrAgentUnavailable
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_offers SET status = 'ACCEPTED' WHERE id = $1`, offerID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_agents SET status = 'BUSY', version = version + 1, updated_at = $2 WHERE id = $1`, agentID, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'ASSIGNED', agent_id = $2, version = version + 1, updated_at = $3 WHERE id = $1`, job.ID, agentID, now); err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrAgentUnavailable
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindJob(ctx, job.ID)
}

func (r *DispatchRepository) DeclineOffer(ctx context.Context, offerID, agentID string, now time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	offer, err := lockOffer(ctx, tx, offerID)
	if err != nil {
		return "", err
	}
	if offer.AgentID != agentID {
		return "", domain.ErrOfferNotFound
	}
	if offer.Status != domain.OfferPending {
		return "", domain.ErrOfferExpired
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_offers SET status = 'DECLINED' WHERE id = $1`, offerID); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'REQUESTED', version = version + 1, updated_at = $2 WHERE id = $1 AND status = 'OFFERED'`, offer.JobID, now); err != nil {
		return "", err
	}
	return offer.JobID, tx.Commit()
}

func (r *DispatchRepository) ExpireOffers(ctx context.Context, now time.Time) ([]string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `UPDATE dispatch_offers SET status = 'EXPIRED' WHERE status = 'PENDING' AND expires_at <= $1 RETURNING job_id`, now)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'REQUESTED', version = version + 1, updated_at = $2 WHERE id = ANY($1::uuid[]) AND status = 'OFFERED'`, pq.Array(ids), now); err != nil {
			return nil, err
		}
	}
	return ids, tx.Commit()
}

// ------------------------------------------------- advance, cancel, stats

func (r *DispatchRepository) AdvanceJob(ctx context.Context, in domain.AdvanceJobInput) (*domain.DispatchJob, error) {
	switch in.To {
	case domain.JobAtPickup, domain.JobInTransit, domain.JobCompleted:
	default:
		return nil, domain.ErrInvalidJobTransition
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job, err := lockJob(ctx, tx, in.JobID)
	if err != nil {
		return nil, err
	}
	// Only the assigned agent moves a job; others see it as not found.
	if job.AgentID == nil || *job.AgentID != in.AgentID {
		return nil, domain.ErrJobNotFound
	}
	if !job.Status.CanTransitionTo(in.To) {
		return nil, domain.ErrInvalidJobTransition
	}
	if job.ProofRequiredFor(in.To) {
		if err := job.CheckProof(in.ProofCode); err != nil {
			if errors.Is(err, domain.ErrWrongProofCode) {
				// Count the wrong attempt even though the call fails.
				if _, uerr := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET proof_attempts = $2, updated_at = $3 WHERE id = $1`, job.ID, job.ProofAttempts, in.Now); uerr != nil {
					return nil, uerr
				}
				if cerr := tx.Commit(); cerr != nil {
					return nil, cerr
				}
			}
			return nil, err
		}
	}
	var cash any
	if in.To == domain.JobCompleted && job.CashDueMinor > 0 {
		if in.CashCollectedMinor == nil || *in.CashCollectedMinor != job.CashDueMinor {
			return nil, domain.ErrCashMismatch
		}
		cash = *in.CashCollectedMinor
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = $2, cash_collected_minor = COALESCE($3, cash_collected_minor), version = version + 1, updated_at = $4 WHERE id = $1`,
		job.ID, in.To, cash, in.Now); err != nil {
		return nil, err
	}
	if in.To == domain.JobCompleted {
		if _, err := tx.ExecContext(ctx, `UPDATE dispatch_agents SET status = 'ONLINE', version = version + 1, updated_at = $2 WHERE id = $1 AND status = 'BUSY'`, in.AgentID, in.Now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindJob(ctx, job.ID)
}

func (r *DispatchRepository) CancelJob(ctx context.Context, jobID, reason string, now time.Time) (*domain.DispatchJob, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	job, err := lockJob(ctx, tx, jobID)
	if err != nil {
		return nil, err
	}
	if !job.Status.CanTransitionTo(domain.JobCancelled) {
		return nil, domain.ErrInvalidJobTransition
	}
	if len(reason) > 200 {
		reason = reason[:200]
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_offers SET status = 'WITHDRAWN' WHERE job_id = $1 AND status = 'PENDING'`, jobID); err != nil {
		return nil, err
	}
	if job.AgentID != nil && job.Status.IsActive() {
		if _, err := tx.ExecContext(ctx, `UPDATE dispatch_agents SET status = 'ONLINE', version = version + 1, updated_at = $2 WHERE id = $1 AND status = 'BUSY'`, *job.AgentID, now); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'CANCELLED', cancel_reason = $2, version = version + 1, updated_at = $3 WHERE id = $1`, jobID, reason, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindJob(ctx, jobID)
}

func (r *DispatchRepository) AgentEarnings(ctx context.Context, agentID string) (domain.AgentEarnings, error) {
	var e domain.AgentEarnings
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(currency), ''), COUNT(*), COALESCE(SUM(fee_minor), 0), COALESCE(SUM(cash_collected_minor), 0)
		FROM dispatch_jobs WHERE agent_id = $1 AND status = 'COMPLETED'`, agentID).
		Scan(&e.Currency, &e.CompletedJobs, &e.FeesMinor, &e.CashHeldMinor)
	return e, err
}
