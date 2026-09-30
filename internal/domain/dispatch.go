package domain

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math"
	"math/big"
	"time"
)

// Dispatch: an agent (courier or driver) is matched to a job (a food delivery
// or a ride), moves, and is tracked. Shared by the RESTAURANT and TRANSPORT
// verticals. See docs/architecture/DISPATCH_FOOD_AND_RIDES.md.

// AgentKind is what an agent does.
type AgentKind string

const (
	AgentCourier AgentKind = "COURIER"
	AgentDriver  AgentKind = "DRIVER"
)

// VehicleType is how an agent moves.
type VehicleType string

const (
	VehicleBicycle   VehicleType = "BICYCLE"
	VehicleMotorbike VehicleType = "MOTORBIKE"
	VehicleCar       VehicleType = "CAR"
)

// AgentStatus is an agent availability.
type AgentStatus string

const (
	AgentOffline AgentStatus = "OFFLINE"
	AgentOnline  AgentStatus = "ONLINE"
	AgentBusy    AgentStatus = "BUSY"
)

// JobKind is what a job delivers.
type JobKind string

const (
	JobFoodDelivery JobKind = "FOOD_DELIVERY"
	JobRide         JobKind = "RIDE"
)

// JobStatus is a job lifecycle state.
type JobStatus string

const (
	JobRequested JobStatus = "REQUESTED"
	JobOffered   JobStatus = "OFFERED"
	JobAssigned  JobStatus = "ASSIGNED"
	JobAtPickup  JobStatus = "AT_PICKUP"
	JobInTransit JobStatus = "IN_TRANSIT"
	JobCompleted JobStatus = "COMPLETED"
	JobCancelled JobStatus = "CANCELLED"
	JobNoAgent   JobStatus = "NO_AGENT"
)

// OfferStatus is the state of a job offer to one agent.
type OfferStatus string

const (
	OfferPending   OfferStatus = "PENDING"
	OfferAccepted  OfferStatus = "ACCEPTED"
	OfferDeclined  OfferStatus = "DECLINED"
	OfferExpired   OfferStatus = "EXPIRED"
	OfferWithdrawn OfferStatus = "WITHDRAWN"
)

// Dispatch limits.
const (
	OfferTTL         = 30 * time.Second
	MaxProofAttempts = 5
	MaxAddressLength = 300
	// StalePositionAfter is how old an agent last position may be and still
	// count for matching.
	StalePositionAfter = 2 * time.Minute
)

var (
	ErrAgentNotFound         = errors.New("dispatch agent not found")
	ErrJobNotFound           = errors.New("dispatch job not found")
	ErrOfferNotFound         = errors.New("dispatch offer not found")
	ErrOfferExpired          = errors.New("the offer is no longer available")
	ErrAgentUnavailable      = errors.New("the agent cannot take this job now")
	ErrInvalidJobTransition  = errors.New("invalid job status transition")
	ErrInvalidLocation       = errors.New("invalid location")
	ErrInvalidAgent          = errors.New("invalid dispatch agent")
	ErrWrongProofCode        = errors.New("wrong confirmation code")
	ErrProofAttemptsExceeded = errors.New("too many wrong confirmation codes")
	ErrJobAlreadyExists      = errors.New("a dispatch job already exists for this order")
)

// Validate rejects NaN, infinite and out-of-range coordinates.
// (GeoPoint is the WGS84 coordinate type declared in search.go.)
func (p GeoPoint) Validate() error {
	if math.IsNaN(p.Latitude) || math.IsNaN(p.Longitude) || math.IsInf(p.Latitude, 0) || math.IsInf(p.Longitude, 0) ||
		p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		return ErrInvalidLocation
	}
	return nil
}

// DistanceKm is the great-circle distance between two points (haversine).
func DistanceKm(a, b GeoPoint) float64 {
	const earthRadiusKm = 6371.0088
	rad := math.Pi / 180
	dLat := (b.Latitude - a.Latitude) * rad
	dLng := (b.Longitude - a.Longitude) * rad
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(a.Latitude*rad)*math.Cos(b.Latitude*rad)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusKm * math.Asin(math.Min(1, math.Sqrt(h)))
}

// DispatchAgent is a user acting as a courier or driver.
type DispatchAgent struct {
	ID         string
	UserID     string
	Kind       AgentKind
	Vehicle    VehicleType
	Status     AgentStatus
	Position   *GeoPoint
	PositionAt *time.Time
	Version    int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// NewDispatchAgent creates an OFFLINE agent profile.
func NewDispatchAgent(userID string, kind AgentKind, vehicle VehicleType, now time.Time) (*DispatchAgent, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: user is required", ErrInvalidAgent)
	}
	if kind != AgentCourier && kind != AgentDriver {
		return nil, fmt.Errorf("%w: kind must be COURIER or DRIVER", ErrInvalidAgent)
	}
	switch vehicle {
	case VehicleBicycle, VehicleMotorbike, VehicleCar:
	default:
		return nil, fmt.Errorf("%w: unknown vehicle", ErrInvalidAgent)
	}
	if kind == AgentDriver && vehicle == VehicleBicycle {
		return nil, fmt.Errorf("%w: a driver needs a motorised vehicle", ErrInvalidAgent)
	}
	return &DispatchAgent{ID: NewID(), UserID: userID, Kind: kind, Vehicle: vehicle, Status: AgentOffline, Version: 1, CreatedAt: now, UpdatedAt: now}, nil
}

// DispatchJob is one unit of work for an agent.
type DispatchJob struct {
	ID                 string
	OrganizationID     *string
	Kind               JobKind
	OrderID            *string
	CustomerID         string
	Pickup             GeoPoint
	PickupAddress      string
	Dropoff            GeoPoint
	DropoffAddress     string
	Status             JobStatus
	AgentID            *string
	FeeMinor           int64
	Currency           string
	CashDueMinor       int64
	CashCollectedMinor *int64
	ProofCode          string
	ProofAttempts      int
	CancelReason       string
	Version            int64
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// NewJob describes a job to create.
type NewJob struct {
	OrganizationID *string
	Kind           JobKind
	OrderID        *string
	CustomerID     string
	Pickup         GeoPoint
	PickupAddress  string
	Dropoff        GeoPoint
	DropoffAddress string
	FeeMinor       int64
	Currency       string
	CashDueMinor   int64
}

// NewDispatchJob validates a request and creates a REQUESTED job with a fresh
// confirmation code.
func NewDispatchJob(n NewJob, now time.Time) (*DispatchJob, error) {
	if n.Kind != JobFoodDelivery && n.Kind != JobRide {
		return nil, fmt.Errorf("unknown job kind %q", n.Kind)
	}
	if n.CustomerID == "" {
		return nil, fmt.Errorf("customer is required")
	}
	if n.Kind == JobFoodDelivery && (n.OrderID == nil || n.OrganizationID == nil) {
		return nil, fmt.Errorf("a food delivery needs an order and an organization")
	}
	if err := n.Pickup.Validate(); err != nil {
		return nil, err
	}
	if err := n.Dropoff.Validate(); err != nil {
		return nil, err
	}
	if len(n.PickupAddress) > MaxAddressLength || len(n.DropoffAddress) > MaxAddressLength {
		return nil, fmt.Errorf("%w: address too long", ErrInvalidLocation)
	}
	if n.FeeMinor < 0 || n.CashDueMinor < 0 || len(n.Currency) != 3 {
		return nil, fmt.Errorf("invalid fee or currency")
	}
	code, err := NewProofCode()
	if err != nil {
		return nil, err
	}
	return &DispatchJob{
		ID: NewID(), OrganizationID: n.OrganizationID, Kind: n.Kind, OrderID: n.OrderID, CustomerID: n.CustomerID,
		Pickup: n.Pickup, PickupAddress: n.PickupAddress, Dropoff: n.Dropoff, DropoffAddress: n.DropoffAddress,
		Status: JobRequested, FeeMinor: n.FeeMinor, Currency: n.Currency, CashDueMinor: n.CashDueMinor,
		ProofCode: code, Version: 1, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// NewProofCode returns a random 4-digit confirmation code.
func NewProofCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", n.Int64()), nil
}

var jobTransitions = map[JobStatus][]JobStatus{
	JobRequested: {JobOffered, JobNoAgent, JobCancelled},
	JobOffered:   {JobAssigned, JobRequested, JobNoAgent, JobCancelled},
	JobAssigned:  {JobAtPickup, JobCancelled},
	JobAtPickup:  {JobInTransit, JobCancelled},
	JobInTransit: {JobCompleted},
	JobNoAgent:   {JobRequested, JobCancelled},
}

// CanTransitionTo reports whether a job may move to the target status.
func (s JobStatus) CanTransitionTo(target JobStatus) bool {
	for _, t := range jobTransitions[s] {
		if t == target {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the job can no longer change.
func (s JobStatus) IsTerminal() bool { return s == JobCompleted || s == JobCancelled }

// IsActive reports whether an agent is committed to the job.
func (s JobStatus) IsActive() bool { return s == JobAssigned || s == JobAtPickup || s == JobInTransit }

// CheckProof compares a confirmation code in constant time, counting wrong
// attempts; after MaxProofAttempts the code is locked.
func (j *DispatchJob) CheckProof(code string) error {
	if j.ProofAttempts >= MaxProofAttempts {
		return ErrProofAttemptsExceeded
	}
	if subtle.ConstantTimeCompare([]byte(code), []byte(j.ProofCode)) != 1 {
		j.ProofAttempts++
		return ErrWrongProofCode
	}
	return nil
}

// DispatchOffer is a job offered to one agent for a limited time.
type DispatchOffer struct {
	ID        string
	JobID     string
	AgentID   string
	Status    OfferStatus
	ExpiresAt time.Time
	CreatedAt time.Time
}

// Usable reports whether the offer can still be accepted at the given time.
func (o *DispatchOffer) Usable(now time.Time) bool {
	return o.Status == OfferPending && now.Before(o.ExpiresAt)
}

var (
	ErrCashMismatch      = errors.New("the cash collected must equal the cash due")
	ErrAgentHasActiveJob = errors.New("the agent has an active job")
)

// ProofRequiredFor reports whether moving to the target status needs the
// customer confirmation code: a delivery is confirmed on hand-over, a ride
// when the trip starts (the passenger confirms it is the right car).
func (j *DispatchJob) ProofRequiredFor(target JobStatus) bool {
	switch j.Kind {
	case JobFoodDelivery:
		return target == JobCompleted
	case JobRide:
		return target == JobInTransit
	}
	return false
}

// AgentCandidate is a free agent near a point.
type AgentCandidate struct {
	Agent      *DispatchAgent
	DistanceKm float64
}

// AdvanceJobInput is an agent moving its job forward.
type AdvanceJobInput struct {
	JobID     string
	AgentID   string
	To        JobStatus
	ProofCode string
	// CashCollectedMinor is required to complete a job that has cash due.
	CashCollectedMinor *int64
	Now                time.Time
}

// DispatchRepository persists dispatch state. Every method that changes more
// than one row is atomic and takes the row locks it needs, so concurrent
// agents can never both take a job and an agent never holds two.
type DispatchRepository interface {
	CreateAgent(ctx context.Context, a *DispatchAgent) error
	FindAgent(ctx context.Context, id string) (*DispatchAgent, error)
	FindAgentByUser(ctx context.Context, userID string, kind AgentKind) (*DispatchAgent, error)
	// GoOnline makes a non-busy agent available at a position.
	GoOnline(ctx context.Context, agentID string, p GeoPoint, now time.Time) error
	// GoOffline makes an agent unavailable; refused while it has an active job.
	GoOffline(ctx context.Context, agentID string, now time.Time) error
	// UpdatePosition records a position for an ONLINE or BUSY agent.
	UpdatePosition(ctx context.Context, agentID string, p GeoPoint, now time.Time) error
	// NearbyFreeAgents lists ONLINE agents of a kind within radiusKm with a
	// fresh position, nearest first, skipping agents already offered the job.
	NearbyFreeAgents(ctx context.Context, kind AgentKind, near GeoPoint, radiusKm float64, jobID string, now time.Time, limit int) ([]AgentCandidate, error)

	CreateJob(ctx context.Context, j *DispatchJob) error
	FindJob(ctx context.Context, id string) (*DispatchJob, error)
	FindJobByOrder(ctx context.Context, orderID string) (*DispatchJob, error)
	ListJobsForCustomer(ctx context.Context, customerID string, limit int) ([]*DispatchJob, error)
	ListJobsForAgent(ctx context.Context, agentID string, activeOnly bool, limit int) ([]*DispatchJob, error)
	// JobsNeedingMatch lists REQUESTED jobs (and NO_AGENT ones are not included).
	JobsNeedingMatch(ctx context.Context, limit int) ([]*DispatchJob, error)

	// OfferJob moves a REQUESTED job to OFFERED and records one pending offer.
	OfferJob(ctx context.Context, jobID, agentID string, expiresAt, now time.Time) (*DispatchOffer, error)
	FindOffer(ctx context.Context, id string) (*DispatchOffer, error)
	PendingOffersForAgent(ctx context.Context, agentID string, now time.Time) ([]*DispatchOffer, error)
	// AcceptOffer assigns the job to the agent atomically.
	AcceptOffer(ctx context.Context, offerID, agentID string, now time.Time) (*DispatchJob, error)
	// DeclineOffer marks the offer declined and returns the job to REQUESTED.
	DeclineOffer(ctx context.Context, offerID, agentID string, now time.Time) (jobID string, err error)
	// ExpireOffers expires overdue offers and returns their jobs to REQUESTED.
	ExpireOffers(ctx context.Context, now time.Time) (jobIDs []string, err error)
	// MarkNoAgent parks a REQUESTED job nobody can take.
	MarkNoAgent(ctx context.Context, jobID string, now time.Time) error
	// Requeue puts a NO_AGENT job back to REQUESTED.
	Requeue(ctx context.Context, jobID string, now time.Time) error

	// AdvanceJob moves an assigned job forward for its own agent, checking the
	// confirmation code and cash where required. Wrong codes are counted even
	// though the call fails.
	AdvanceJob(ctx context.Context, in AdvanceJobInput) (*DispatchJob, error)
	// CancelJob cancels a non-terminal job that is not yet in transit,
	// withdrawing pending offers and freeing its agent.
	CancelJob(ctx context.Context, jobID, reason string, now time.Time) (*DispatchJob, error)

	// LatestPosition is the last known position of an agent.
	LatestPosition(ctx context.Context, agentID string) (*GeoPoint, *time.Time, error)
	// AgentEarnings sums completed jobs and cash still held by an agent.
	AgentEarnings(ctx context.Context, agentID string) (AgentEarnings, error)
}

// AgentEarnings summarises an agent completed work.
type AgentEarnings struct {
	Currency      string
	CompletedJobs int64
	FeesMinor     int64
	CashHeldMinor int64
}
