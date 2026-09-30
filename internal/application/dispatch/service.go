// Package dispatch matches agents (couriers and drivers) to jobs (food
// deliveries and rides) and moves those jobs to completion. It is shared by
// the RESTAURANT and TRANSPORT verticals; see
// docs/architecture/DISPATCH_FOOD_AND_RIDES.md.
//
// Supply is native: agents are users who go online in Trezutil. Matching asks
// an AgentSource for candidates, so external supply (a future Yango adapter)
// can be added behind the same port without changing jobs or the flows built
// on them.
package dispatch

import (
	"context"
	"errors"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// AgentSource finds free agents near a point. The native source reads the
// dispatch repository.
type AgentSource interface {
	Nearby(ctx context.Context, kind domain.AgentKind, near domain.GeoPoint, radiusKm float64, jobID string, now time.Time, limit int) ([]domain.AgentCandidate, error)
}

// nativeSource is the AgentSource backed by dispatch_agents.
type nativeSource struct{ repo domain.DispatchRepository }

func (s nativeSource) Nearby(ctx context.Context, kind domain.AgentKind, near domain.GeoPoint, radiusKm float64, jobID string, now time.Time, limit int) ([]domain.AgentCandidate, error) {
	return s.repo.NearbyFreeAgents(ctx, kind, near, radiusKm, jobID, now, limit)
}

// Members answers whether a user belongs to an organization.
type Members interface {
	HasMember(ctx context.Context, organizationID, userID string) (bool, error)
}

// Defaults.
const (
	DefaultMatchRadiusKm = 5.0
	// DefaultMatchWindow is how long a job keeps looking for an agent before
	// it is parked as NO_AGENT.
	DefaultMatchWindow = 10 * time.Minute
)

// Config wires the service.
type Config struct {
	Repo    domain.DispatchRepository
	Members Members
	// Source overrides the native agent source (tests, future providers).
	Source        AgentSource
	MatchRadiusKm float64
	MatchWindow   time.Duration
	OfferTTL      time.Duration
	Now           func() time.Time
}

// Service is the dispatch use case layer.
type Service struct {
	repo    domain.DispatchRepository
	members Members
	source  AgentSource
	radius  float64
	window  time.Duration
	ttl     time.Duration
	now     func() time.Time
}

// New creates the service.
func New(c Config) *Service {
	s := &Service{repo: c.Repo, members: c.Members, source: c.Source, radius: c.MatchRadiusKm,
		window: c.MatchWindow, ttl: c.OfferTTL, now: c.Now}
	if s.source == nil {
		s.source = nativeSource{repo: c.Repo}
	}
	if s.radius <= 0 {
		s.radius = DefaultMatchRadiusKm
	}
	if s.window <= 0 {
		s.window = DefaultMatchWindow
	}
	if s.ttl <= 0 {
		s.ttl = domain.OfferTTL
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

func agentKindFor(k domain.JobKind) domain.AgentKind {
	if k == domain.JobRide {
		return domain.AgentDriver
	}
	return domain.AgentCourier
}

// ------------------------------------------------------------ agent side

// RegisterAgent creates the caller agent profile of a kind.
func (s *Service) RegisterAgent(ctx context.Context, userID string, kind domain.AgentKind, vehicle domain.VehicleType) (*domain.DispatchAgent, error) {
	a, err := domain.NewDispatchAgent(userID, kind, vehicle, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateAgent(ctx, a); err != nil {
		return nil, err
	}
	return a, nil
}

// MyAgent returns the caller agent profile of a kind.
func (s *Service) MyAgent(ctx context.Context, userID string, kind domain.AgentKind) (*domain.DispatchAgent, error) {
	return s.repo.FindAgentByUser(ctx, userID, kind)
}

// GoOnline makes the caller available at a position. New jobs waiting for an
// agent are matched straight away.
func (s *Service) GoOnline(ctx context.Context, userID string, kind domain.AgentKind, p domain.GeoPoint) (*domain.DispatchAgent, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	if err := s.repo.GoOnline(ctx, a.ID, p, s.now()); err != nil {
		return nil, err
	}
	s.Tick(ctx)
	return s.repo.FindAgent(ctx, a.ID)
}

// GoOffline makes the caller unavailable; refused during an active job.
func (s *Service) GoOffline(ctx context.Context, userID string, kind domain.AgentKind) error {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return err
	}
	return s.repo.GoOffline(ctx, a.ID, s.now())
}

// ReportPosition records the caller position.
func (s *Service) ReportPosition(ctx context.Context, userID string, kind domain.AgentKind, p domain.GeoPoint) error {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return err
	}
	return s.repo.UpdatePosition(ctx, a.ID, p, s.now())
}

// OfferView is what an agent sees of an offered job: where, how much, how far.
// It never carries the customer identity or the confirmation code.
type OfferView struct {
	OfferID        string
	JobID          string
	Kind           domain.JobKind
	PickupAddress  string
	DropoffAddress string
	Pickup         domain.GeoPoint
	Dropoff        domain.GeoPoint
	FeeMinor       int64
	CashDueMinor   int64
	Currency       string
	ExpiresAt      time.Time
}

// PendingOffers lists the caller live offers.
func (s *Service) PendingOffers(ctx context.Context, userID string, kind domain.AgentKind) ([]OfferView, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	offers, err := s.repo.PendingOffersForAgent(ctx, a.ID, s.now())
	if err != nil {
		return nil, err
	}
	out := make([]OfferView, 0, len(offers))
	for _, o := range offers {
		j, err := s.repo.FindJob(ctx, o.JobID)
		if err != nil {
			continue
		}
		out = append(out, OfferView{OfferID: o.ID, JobID: j.ID, Kind: j.Kind, PickupAddress: j.PickupAddress,
			DropoffAddress: j.DropoffAddress, Pickup: j.Pickup, Dropoff: j.Dropoff, FeeMinor: j.FeeMinor,
			CashDueMinor: j.CashDueMinor, Currency: j.Currency, ExpiresAt: o.ExpiresAt})
	}
	return out, nil
}

// Accept takes an offered job; at most one agent can.
func (s *Service) Accept(ctx context.Context, userID string, kind domain.AgentKind, offerID string) (*domain.DispatchJob, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	return s.repo.AcceptOffer(ctx, offerID, a.ID, s.now())
}

// Decline refuses an offer; the job goes to the next agent.
func (s *Service) Decline(ctx context.Context, userID string, kind domain.AgentKind, offerID string) error {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return err
	}
	jobID, err := s.repo.DeclineOffer(ctx, offerID, a.ID, s.now())
	if err != nil {
		return err
	}
	s.match(ctx, jobID)
	return nil
}

// Advance moves the caller assigned job forward (at pickup, in transit,
// completed). The confirmation code is required where the job kind says so.
func (s *Service) Advance(ctx context.Context, userID string, kind domain.AgentKind, jobID string, to domain.JobStatus, proofCode string, cashCollected *int64) (*domain.DispatchJob, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	return s.repo.AdvanceJob(ctx, domain.AdvanceJobInput{JobID: jobID, AgentID: a.ID, To: to,
		ProofCode: proofCode, CashCollectedMinor: cashCollected, Now: s.now()})
}

// AgentJobs lists the caller jobs.
func (s *Service) AgentJobs(ctx context.Context, userID string, kind domain.AgentKind, activeOnly bool) ([]*domain.DispatchJob, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return nil, err
	}
	jobs, err := s.repo.ListJobsForAgent(ctx, a.ID, activeOnly, 50)
	for _, j := range jobs {
		j.ProofCode = "" // the code belongs to the customer
	}
	return jobs, err
}

// Earnings summarises the caller completed work and cash collected.
func (s *Service) Earnings(ctx context.Context, userID string, kind domain.AgentKind) (domain.AgentEarnings, error) {
	a, err := s.repo.FindAgentByUser(ctx, userID, kind)
	if err != nil {
		return domain.AgentEarnings{}, err
	}
	return s.repo.AgentEarnings(ctx, a.ID)
}

// ---------------------------------------------------------- customer side

// Request creates a job and starts matching. It is called by the flows that
// own the business rules (food orders, ride requests), never straight from a
// client with a customer ID of its choosing.
func (s *Service) Request(ctx context.Context, n domain.NewJob) (*domain.DispatchJob, error) {
	j, err := domain.NewDispatchJob(n, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.repo.CreateJob(ctx, j); err != nil {
		return nil, err
	}
	s.match(ctx, j.ID)
	return s.repo.FindJob(ctx, j.ID)
}

// JobView is a job as its customer sees it.
type JobView struct {
	Job *domain.DispatchJob
	// ProofCode is set only for the customer.
	ProofCode string
	// Agent is set once an agent holds the job.
	Agent *AgentView
}

// AgentView is the part of an agent a customer may see.
type AgentView struct {
	Kind       domain.AgentKind
	Vehicle    domain.VehicleType
	Position   *domain.GeoPoint
	PositionAt *time.Time
}

// canSee reports whether an actor may read a job: its customer, or a member of
// the store organization it delivers for.
func (s *Service) canSee(ctx context.Context, actor string, j *domain.DispatchJob) (customer bool, ok bool) {
	if actor == "" {
		return false, false
	}
	if actor == j.CustomerID {
		return true, true
	}
	if j.OrganizationID != nil && s.members != nil {
		if has, err := s.members.HasMember(ctx, *j.OrganizationID, actor); err == nil && has {
			return false, true
		}
	}
	return false, false
}

// Job returns a job to its customer or the store team. Anyone else gets
// ErrJobNotFound. Only the customer sees the confirmation code and the live
// agent position, and the position only while an agent holds the job.
func (s *Service) Job(ctx context.Context, actor, jobID string) (*JobView, error) {
	j, err := s.repo.FindJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	customer, ok := s.canSee(ctx, actor, j)
	if !ok {
		return nil, domain.ErrJobNotFound
	}
	view := &JobView{Job: j}
	if customer && !j.Status.IsTerminal() {
		view.ProofCode = j.ProofCode
	}
	j.ProofCode = ""
	if j.AgentID != nil {
		if a, err := s.repo.FindAgent(ctx, *j.AgentID); err == nil {
			av := &AgentView{Kind: a.Kind, Vehicle: a.Vehicle}
			if customer && j.Status.IsActive() {
				av.Position, av.PositionAt = a.Position, a.PositionAt
			}
			view.Agent = av
		}
	}
	return view, nil
}

// JobForOrder returns the delivery job of an order to its customer or team.
func (s *Service) JobForOrder(ctx context.Context, actor, orderID string) (*JobView, error) {
	j, err := s.repo.FindJobByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	return s.Job(ctx, actor, j.ID)
}

// MyJobs lists the caller jobs as a customer.
func (s *Service) MyJobs(ctx context.Context, actor string) ([]*domain.DispatchJob, error) {
	jobs, err := s.repo.ListJobsForCustomer(ctx, actor, 50)
	for _, j := range jobs {
		j.ProofCode = ""
	}
	return jobs, err
}

// Cancel cancels a job that is not yet on its way. Only the customer or the
// store team may; anyone else sees ErrJobNotFound.
func (s *Service) Cancel(ctx context.Context, actor, jobID, reason string) (*domain.DispatchJob, error) {
	j, err := s.repo.FindJob(ctx, jobID)
	if err != nil {
		return nil, err
	}
	if _, ok := s.canSee(ctx, actor, j); !ok {
		return nil, domain.ErrJobNotFound
	}
	out, err := s.repo.CancelJob(ctx, jobID, reason, s.now())
	if err != nil {
		return nil, err
	}
	out.ProofCode = ""
	return out, nil
}

// Retry puts a parked NO_AGENT job back into matching (customer only).
func (s *Service) Retry(ctx context.Context, actor, jobID string) error {
	j, err := s.repo.FindJob(ctx, jobID)
	if err != nil {
		return err
	}
	if customer, ok := s.canSee(ctx, actor, j); !ok || !customer {
		return domain.ErrJobNotFound
	}
	if err := s.repo.Requeue(ctx, jobID, s.now()); err != nil {
		return err
	}
	s.match(ctx, jobID)
	return nil
}

// ---------------------------------------------------------------- matching

// Tick expires overdue offers and matches every job waiting for an agent. It
// is safe to call from several processes: each step is a conditional update.
func (s *Service) Tick(ctx context.Context) {
	if ids, err := s.repo.ExpireOffers(ctx, s.now()); err == nil {
		for _, id := range ids {
			s.match(ctx, id)
		}
	}
	if jobs, err := s.repo.JobsNeedingMatch(ctx, 50); err == nil {
		for _, j := range jobs {
			s.match(ctx, j.ID)
		}
	}
}

// Run calls Tick every interval until the context ends.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.Tick(ctx)
		}
	}
}

// match offers a REQUESTED job to the nearest free agent that has not seen it
// yet. With nobody in range the job keeps waiting until the match window ends,
// then it is parked as NO_AGENT so the customer can decide.
func (s *Service) match(ctx context.Context, jobID string) {
	j, err := s.repo.FindJob(ctx, jobID)
	if err != nil || j.Status != domain.JobRequested {
		return
	}
	now := s.now()
	cands, err := s.source.Nearby(ctx, agentKindFor(j.Kind), j.Pickup, s.radius, j.ID, now, 3)
	if err != nil {
		return
	}
	for _, c := range cands {
		_, err := s.repo.OfferJob(ctx, j.ID, c.Agent.ID, now.Add(s.ttl), now)
		if err == nil {
			return
		}
		if errors.Is(err, domain.ErrInvalidJobTransition) {
			return // someone else moved the job
		}
		// The agent went away or was taken; try the next one.
	}
	if now.Sub(j.CreatedAt) > s.window {
		_ = s.repo.MarkNoAgent(ctx, j.ID, now)
	}
}
