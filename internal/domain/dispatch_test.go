package domain

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestDistanceKm(t *testing.T) {
	gombe := GeoPoint{-4.3092, 15.3070}
	limete := GeoPoint{-4.3350, 15.3460}
	if d := DistanceKm(gombe, gombe); d != 0 {
		t.Errorf("same point = %v", d)
	}
	d := DistanceKm(gombe, limete)
	if d < 4.5 || d > 5.5 {
		t.Errorf("Gombe to Limete = %.2f km, want about 5", d)
	}
	if math.Abs(DistanceKm(gombe, limete)-DistanceKm(limete, gombe)) > 1e-9 {
		t.Error("distance is not symmetric")
	}
	// One degree of latitude is about 111 km.
	if d := DistanceKm(GeoPoint{0, 0}, GeoPoint{1, 0}); d < 110 || d > 112 {
		t.Errorf("1 degree = %.2f km", d)
	}
}

func TestGeoPointValidate(t *testing.T) {
	for _, p := range []GeoPoint{{0, 0}, {-90, 180}, {90, -180}, {-4.3, 15.3}} {
		if err := p.Validate(); err != nil {
			t.Errorf("%+v: %v", p, err)
		}
	}
	for _, p := range []GeoPoint{{91, 0}, {0, 181}, {math.NaN(), 0}, {0, math.Inf(1)}} {
		if !errors.Is(p.Validate(), ErrInvalidLocation) {
			t.Errorf("%+v should be invalid", p)
		}
	}
}

func TestNewDispatchAgent(t *testing.T) {
	now := time.Now()
	a, err := NewDispatchAgent("u1", AgentCourier, VehicleMotorbike, now)
	if err != nil || a.Status != AgentOffline || a.Position != nil {
		t.Fatalf("agent = %+v %v", a, err)
	}
	for name, args := range map[string]struct {
		user string
		kind AgentKind
		veh  VehicleType
	}{
		"no user":             {"", AgentCourier, VehicleCar},
		"bad kind":            {"u", "PILOT", VehicleCar},
		"bad vehicle":         {"u", AgentCourier, "HORSE"},
		"driver on a bicycle": {"u", AgentDriver, VehicleBicycle},
	} {
		if _, err := NewDispatchAgent(args.user, args.kind, args.veh, now); !errors.Is(err, ErrInvalidAgent) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewDispatchAgent("u", AgentCourier, VehicleBicycle, now); err != nil {
		t.Errorf("a courier may ride a bicycle: %v", err)
	}
}

func validJob() NewJob {
	o, org := "order-1", "org-1"
	return NewJob{OrganizationID: &org, Kind: JobFoodDelivery, OrderID: &o, CustomerID: "c1",
		Pickup: GeoPoint{-4.31, 15.30}, Dropoff: GeoPoint{-4.33, 15.34}, FeeMinor: 300, Currency: "USD"}
}

func TestNewDispatchJob(t *testing.T) {
	j, err := NewDispatchJob(validJob(), time.Now())
	if err != nil || j.Status != JobRequested || len(j.ProofCode) != 4 || j.AgentID != nil {
		t.Fatalf("job = %+v %v", j, err)
	}
	mut := map[string]func(*NewJob){
		"unknown kind":       func(n *NewJob) { n.Kind = "TELEPORT" },
		"no customer":        func(n *NewJob) { n.CustomerID = "" },
		"food without order": func(n *NewJob) { n.OrderID = nil },
		"food without org":   func(n *NewJob) { n.OrganizationID = nil },
		"bad pickup":         func(n *NewJob) { n.Pickup = GeoPoint{100, 0} },
		"bad dropoff":        func(n *NewJob) { n.Dropoff = GeoPoint{0, 200} },
		"negative fee":       func(n *NewJob) { n.FeeMinor = -1 },
		"negative cash":      func(n *NewJob) { n.CashDueMinor = -1 },
		"bad currency":       func(n *NewJob) { n.Currency = "US" },
		"address too long":   func(n *NewJob) { n.DropoffAddress = string(make([]byte, MaxAddressLength+1)) },
	}
	for name, f := range mut {
		n := validJob()
		f(&n)
		if _, err := NewDispatchJob(n, time.Now()); err == nil {
			t.Errorf("%s should be refused", name)
		}
	}
	// A ride needs neither an order nor an organization.
	ride := validJob()
	ride.Kind, ride.OrderID, ride.OrganizationID = JobRide, nil, nil
	if _, err := NewDispatchJob(ride, time.Now()); err != nil {
		t.Errorf("ride: %v", err)
	}
}

func TestJobTransitions(t *testing.T) {
	ok := [][2]JobStatus{
		{JobRequested, JobOffered}, {JobOffered, JobAssigned}, {JobOffered, JobRequested},
		{JobAssigned, JobAtPickup}, {JobAtPickup, JobInTransit}, {JobInTransit, JobCompleted},
		{JobNoAgent, JobRequested}, {JobAssigned, JobCancelled}, {JobRequested, JobCancelled},
	}
	for _, p := range ok {
		if !p[0].CanTransitionTo(p[1]) {
			t.Errorf("%s -> %s should be allowed", p[0], p[1])
		}
	}
	bad := [][2]JobStatus{
		{JobRequested, JobAssigned},  // an agent must accept an offer first
		{JobAssigned, JobCompleted},  // no skipping pickup and transit
		{JobInTransit, JobCancelled}, // once on the way, only completion
		{JobCompleted, JobRequested}, // terminal
		{JobCancelled, JobOffered},   // terminal
		{JobAtPickup, JobAssigned},   // no going back
	}
	for _, p := range bad {
		if p[0].CanTransitionTo(p[1]) {
			t.Errorf("%s -> %s should be refused", p[0], p[1])
		}
	}
	if !JobCompleted.IsTerminal() || !JobCancelled.IsTerminal() || JobNoAgent.IsTerminal() {
		t.Error("terminal states")
	}
	if !JobAssigned.IsActive() || !JobInTransit.IsActive() || JobOffered.IsActive() {
		t.Error("active states")
	}
}

func TestProofCodeLockout(t *testing.T) {
	j, _ := NewDispatchJob(validJob(), time.Now())
	j.ProofCode = "4821"
	if err := j.CheckProof("4821"); err != nil || j.ProofAttempts != 0 {
		t.Fatalf("right code: %v attempts=%d", err, j.ProofAttempts)
	}
	for i := 0; i < MaxProofAttempts; i++ {
		if err := j.CheckProof("0000"); !errors.Is(err, ErrWrongProofCode) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	// Locked: even the right code is refused now.
	if err := j.CheckProof("4821"); !errors.Is(err, ErrProofAttemptsExceeded) {
		t.Fatalf("locked: %v", err)
	}
	if err := (&DispatchJob{ProofCode: "4821"}).CheckProof("482"); !errors.Is(err, ErrWrongProofCode) {
		t.Errorf("short code: %v", err)
	}
}

func TestOfferUsable(t *testing.T) {
	now := time.Now()
	o := &DispatchOffer{Status: OfferPending, ExpiresAt: now.Add(time.Second)}
	if !o.Usable(now) {
		t.Error("fresh pending offer")
	}
	if o.Usable(now.Add(2 * time.Second)) {
		t.Error("expired offer")
	}
	o.Status = OfferDeclined
	if o.Usable(now) {
		t.Error("declined offer")
	}
}

func TestNewProofCodeIsFourDigits(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := NewProofCode()
		if err != nil || len(c) != 4 {
			t.Fatalf("code %q %v", c, err)
		}
		seen[c] = true
	}
	if len(seen) < 100 {
		t.Errorf("codes are not random enough: %d distinct of 200", len(seen))
	}
}
