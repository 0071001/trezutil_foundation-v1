package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/dispatch"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/domain"
)

// Dispatch on real PostgreSQL: matching, offers, the accept race, the
// confirmation code, cash and who may see what.

type dispatchFixture struct {
	*hotelFixture
	svc   *dispatch.Service
	repo  *postgres.DispatchRepository
	clock *fakeClock
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

var (
	gombe  = domain.GeoPoint{Latitude: -4.3092, Longitude: 15.3070}
	nearby = domain.GeoPoint{Latitude: -4.3110, Longitude: 15.3090} // about 300 m away
	limete = domain.GeoPoint{Latitude: -4.3350, Longitude: 15.3460} // about 5 km away
	far    = domain.GeoPoint{Latitude: -4.5000, Longitude: 15.6000} // tens of km away
)

func newDispatchFixture(t *testing.T) *dispatchFixture {
	h := newHotelFixture(t)
	repo := postgres.NewDispatchRepository(h.db.DB)
	clock := &fakeClock{t: time.Now().UTC().Truncate(time.Second)}
	svc := dispatch.New(dispatch.Config{Repo: repo, Members: postgres.NewOrganizationRepository(h.db.DB), Now: clock.Now, MatchRadiusKm: 3})
	return &dispatchFixture{hotelFixture: h, svc: svc, repo: repo, clock: clock}
}

func (f *dispatchFixture) courier(t *testing.T, user string, at domain.GeoPoint) {
	t.Helper()
	ctx := context.Background()
	if _, err := f.svc.RegisterAgent(ctx, user, domain.AgentCourier, domain.VehicleMotorbike); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GoOnline(ctx, user, domain.AgentCourier, at); err != nil {
		t.Fatal(err)
	}
}

func (f *dispatchFixture) ride(t *testing.T, customer string, cash int64) *domain.DispatchJob {
	t.Helper()
	j, err := f.svc.Request(context.Background(), domain.NewJob{Kind: domain.JobRide, CustomerID: customer,
		Pickup: gombe, PickupAddress: "Gombe", Dropoff: limete, DropoffAddress: "Limete", FeeMinor: 500, Currency: "USD", CashDueMinor: cash})
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func (f *dispatchFixture) offerFor(t *testing.T, user string, kind domain.AgentKind) dispatch.OfferView {
	t.Helper()
	offers, err := f.svc.PendingOffers(context.Background(), user, kind)
	if err != nil || len(offers) != 1 {
		t.Fatalf("offers for %s = %+v %v", user, offers, err)
	}
	return offers[0]
}

func TestDispatch_NearestDriverGetsTheOffer(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	for user, at := range map[string]domain.GeoPoint{hStaffA: nearby, hOwnerB: limete, hGuest2: far} {
		if _, err := f.svc.RegisterAgent(ctx, user, domain.AgentDriver, domain.VehicleCar); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.GoOnline(ctx, user, domain.AgentDriver, at); err != nil {
			t.Fatal(err)
		}
	}
	job := f.ride(t, hGuest, 0)
	if job.Status != domain.JobOffered {
		t.Fatalf("job status = %s", job.Status)
	}
	// The nearest driver is offered the ride; the driver 5 km away and the one
	// tens of km away are not.
	f.offerFor(t, hStaffA, domain.AgentDriver)
	for _, u := range []string{hOwnerB, hGuest2} {
		if offers, _ := f.svc.PendingOffers(ctx, u, domain.AgentDriver); len(offers) != 0 {
			t.Errorf("%s should have no offer: %+v", u, offers)
		}
	}
}

func TestDispatch_CouriersAreNotOfferedRides(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	f.courier(t, hStaffA, nearby)
	f.courier(t, hOwnerB, gombe)
	f.courier(t, hGuest2, gombe)

	job, err := f.svc.Request(ctx, domain.NewJob{Kind: domain.JobRide, CustomerID: hGuest, Pickup: gombe, Dropoff: limete, Currency: "USD", FeeMinor: 100})
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.JobRequested {
		t.Fatalf("a ride with no driver online is %s, want REQUESTED", job.Status)
	}
	// Courier agents do not serve rides: nobody is offered a RIDE.
	if offers, _ := f.svc.PendingOffers(ctx, hStaffA, domain.AgentCourier); len(offers) != 0 {
		t.Fatalf("a courier was offered a ride: %+v", offers)
	}
}

func TestDispatch_DeclineExpiryAndNoAgent(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	for _, u := range []string{hStaffA, hOwnerB} {
		if _, err := f.svc.RegisterAgent(ctx, u, domain.AgentDriver, domain.VehicleCar); err != nil {
			t.Fatal(err)
		}
	}
	at := map[string]domain.GeoPoint{hStaffA: gombe, hOwnerB: nearby}
	goOnline := func(u string) {
		if _, err := f.svc.GoOnline(ctx, u, domain.AgentDriver, at[u]); err != nil {
			t.Fatal(err)
		}
	}
	goOnline(hStaffA)
	job := f.ride(t, hGuest, 0)
	first := f.offerFor(t, hStaffA, domain.AgentDriver)

	// Declining sends the job on: with nobody else online it keeps waiting.
	if err := f.svc.Decline(ctx, hStaffA, domain.AgentDriver, first.OfferID); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.svc.Job(ctx, hGuest, job.ID); v.Job.Status != domain.JobRequested {
		t.Fatalf("after decline = %s", v.Job.Status)
	}
	// A declined offer cannot be accepted or declined again.
	if _, err := f.svc.Accept(ctx, hStaffA, domain.AgentDriver, first.OfferID); !errors.Is(err, domain.ErrOfferExpired) {
		t.Fatalf("accept after decline: %v", err)
	}

	// A second driver comes online: the waiting job is offered to them, never
	// again to the one who declined.
	goOnline(hOwnerB)
	second := f.offerFor(t, hOwnerB, domain.AgentDriver)
	if offers, _ := f.svc.PendingOffers(ctx, hStaffA, domain.AgentDriver); len(offers) != 0 {
		t.Fatalf("declined driver was offered again: %+v", offers)
	}

	// The offer times out: it expires and nobody is left to try.
	f.clock.Advance(domain.OfferTTL + time.Second)
	f.svc.Tick(ctx)
	if _, err := f.svc.Accept(ctx, hOwnerB, domain.AgentDriver, second.OfferID); !errors.Is(err, domain.ErrOfferExpired) {
		t.Fatalf("accept after expiry: %v", err)
	}
	if v, _ := f.svc.Job(ctx, hGuest, job.ID); v.Job.Status != domain.JobRequested {
		t.Fatalf("after expiry = %s", v.Job.Status)
	}

	// After the match window the job is parked, and the customer can retry.
	f.clock.Advance(dispatch.DefaultMatchWindow)
	f.svc.Tick(ctx)
	if v, _ := f.svc.Job(ctx, hGuest, job.ID); v.Job.Status != domain.JobNoAgent {
		t.Fatalf("after the window = %s", v.Job.Status)
	}
	if err := f.svc.Retry(ctx, hOwnerB, job.ID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("a stranger retried the job: %v", err)
	}
}

func TestDispatch_OnlyOneAgentEverTakesAJob_AndAnAgentNeverHoldsTwo(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	if _, err := f.svc.RegisterAgent(ctx, hStaffA, domain.AgentDriver, domain.VehicleCar); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GoOnline(ctx, hStaffA, domain.AgentDriver, gombe); err != nil {
		t.Fatal(err)
	}
	// Two rides are offered to the same driver, who accepts both at once.
	a := f.ride(t, hGuest, 0)
	repo := f.repo
	agent, _ := repo.FindAgentByUser(ctx, hStaffA, domain.AgentDriver)
	if _, err := repo.OfferJob(ctx, mustRequestRide(t, f).ID, agent.ID, f.clock.Now().Add(time.Minute), f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	offers, _ := f.svc.PendingOffers(ctx, hStaffA, domain.AgentDriver)
	if len(offers) != 2 {
		t.Fatalf("offers = %d (job %s)", len(offers), a.ID)
	}
	var wg sync.WaitGroup
	results := make([]error, len(offers))
	for i, o := range offers {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, results[i] = f.svc.Accept(ctx, hStaffA, domain.AgentDriver, id)
		}(i, o.OfferID)
	}
	wg.Wait()
	won := 0
	for _, err := range results {
		if err == nil {
			won++
		} else if !errors.Is(err, domain.ErrAgentUnavailable) && !errors.Is(err, domain.ErrOfferExpired) {
			t.Errorf("unexpected error: %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("an agent took %d jobs at once, want exactly 1", won)
	}
	var active int
	if err := f.db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM dispatch_jobs WHERE agent_id = $1 AND status IN ('ASSIGNED','AT_PICKUP','IN_TRANSIT')`, agent.ID).Scan(&active); err != nil || active != 1 {
		t.Fatalf("active jobs = %d %v", active, err)
	}
	// The database refuses a second active job even if the code were wrong.
	if _, err := f.db.DB.ExecContext(ctx, `UPDATE dispatch_jobs SET status = 'ASSIGNED', agent_id = $1 WHERE status = 'OFFERED'`, agent.ID); err == nil {
		t.Fatal("the unique index should refuse a second active job for one agent")
	}
	// A busy agent cannot go offline.
	if err := f.svc.GoOffline(ctx, hStaffA, domain.AgentDriver); !errors.Is(err, domain.ErrAgentHasActiveJob) {
		t.Fatalf("busy agent went offline: %v", err)
	}
}

func mustRequestRide(t *testing.T, f *dispatchFixture) *domain.DispatchJob {
	t.Helper()
	j, err := domain.NewDispatchJob(domain.NewJob{Kind: domain.JobRide, CustomerID: hGuest, Pickup: gombe, Dropoff: limete, Currency: "USD", FeeMinor: 100}, f.clock.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.CreateJob(context.Background(), j); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestDispatch_ManyAgentsRaceForOffers(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	drivers := []string{hStaffA, hOwnerB, hGuest2}
	for _, u := range drivers {
		if _, err := f.svc.RegisterAgent(ctx, u, domain.AgentDriver, domain.VehicleCar); err != nil {
			t.Fatal(err)
		}
		if _, err := f.svc.GoOnline(ctx, u, domain.AgentDriver, gombe); err != nil {
			t.Fatal(err)
		}
	}
	// Six rides for three drivers; each ride is offered to one driver at a time.
	for i := 0; i < 6; i++ {
		f.ride(t, hGuest, 0)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	taken := map[string]int{}
	for _, u := range drivers {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			offers, _ := f.svc.PendingOffers(ctx, u, domain.AgentDriver)
			for _, o := range offers {
				if j, err := f.svc.Accept(ctx, u, domain.AgentDriver, o.OfferID); err == nil {
					mu.Lock()
					taken[u]++
					mu.Unlock()
					_ = j
				}
			}
		}(u)
	}
	wg.Wait()
	total := 0
	for u, n := range taken {
		if n > 1 {
			t.Errorf("%s holds %d jobs", u, n)
		}
		total += n
	}
	if total > len(drivers) {
		t.Errorf("%d jobs taken by %d drivers", total, len(drivers))
	}
	var dup int
	if err := f.db.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT agent_id FROM dispatch_jobs WHERE status IN ('ASSIGNED','AT_PICKUP','IN_TRANSIT') GROUP BY agent_id HAVING COUNT(*) > 1) x`).Scan(&dup); err != nil || dup != 0 {
		t.Fatalf("agents with two active jobs = %d %v", dup, err)
	}
}

func TestDispatch_RideLifecycle_CodeAndCash(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	if _, err := f.svc.RegisterAgent(ctx, hStaffA, domain.AgentDriver, domain.VehicleCar); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GoOnline(ctx, hStaffA, domain.AgentDriver, nearby); err != nil {
		t.Fatal(err)
	}
	job := f.ride(t, hGuest, 500) // cash ride: 5.00 due
	offer := f.offerFor(t, hStaffA, domain.AgentDriver)
	if offer.CashDueMinor != 500 || offer.PickupAddress != "Gombe" {
		t.Fatalf("offer = %+v", offer)
	}
	// Someone else cannot accept the driver offer.
	if _, err := f.svc.Accept(ctx, hOwnerB, domain.AgentDriver, offer.OfferID); !errors.Is(err, domain.ErrAgentNotFound) {
		t.Fatalf("a non-agent accepted: %v", err)
	}
	if _, err := f.svc.Accept(ctx, hStaffA, domain.AgentDriver, offer.OfferID); err != nil {
		t.Fatal(err)
	}

	// The customer sees the code and the driver, and the driver never sees the code.
	view, err := f.svc.Job(ctx, hGuest, job.ID)
	if err != nil || len(view.ProofCode) != 4 || view.Agent == nil || view.Agent.Position == nil || view.Job.Status != domain.JobAssigned {
		t.Fatalf("customer view = %+v %v", view, err)
	}
	code := view.ProofCode
	if jobs, _ := f.svc.AgentJobs(ctx, hStaffA, domain.AgentDriver, true); len(jobs) != 1 || jobs[0].ProofCode != "" {
		t.Fatalf("agent jobs leak the code: %+v", jobs)
	}

	// No skipping steps.
	if _, err := f.svc.Advance(ctx, hStaffA, domain.AgentDriver, job.ID, domain.JobCompleted, code, nil); !errors.Is(err, domain.ErrInvalidJobTransition) {
		t.Fatalf("skipped to completed: %v", err)
	}
	if _, err := f.svc.Advance(ctx, hStaffA, domain.AgentDriver, job.ID, domain.JobAtPickup, "", nil); err != nil {
		t.Fatal(err)
	}
	// A ride starts only with the passenger code; wrong codes are counted and lock.
	for i := 0; i < domain.MaxProofAttempts; i++ {
		if _, err := f.svc.Advance(ctx, hStaffA, domain.AgentDriver, job.ID, domain.JobInTransit, "0000x", nil); !errors.Is(err, domain.ErrWrongProofCode) {
			t.Fatalf("wrong code %d: %v", i, err)
		}
	}
	if _, err := f.svc.Advance(ctx, hStaffA, domain.AgentDriver, job.ID, domain.JobInTransit, code, nil); !errors.Is(err, domain.ErrProofAttemptsExceeded) {
		t.Fatalf("locked code: %v", err)
	}
	// The customer can still cancel before the trip starts; that frees the driver.
	if _, err := f.svc.Cancel(ctx, hGuest, job.ID, "changed my mind"); err != nil {
		t.Fatal(err)
	}
	if a, _ := f.repo.FindAgentByUser(ctx, hStaffA, domain.AgentDriver); a.Status != domain.AgentOnline {
		t.Fatalf("agent after cancel = %s", a.Status)
	}

	// A fresh ride goes all the way with the right code and exact cash.
	job2 := f.ride(t, hGuest, 500)
	offer2 := f.offerFor(t, hStaffA, domain.AgentDriver)
	if _, err := f.svc.Accept(ctx, hStaffA, domain.AgentDriver, offer2.OfferID); err != nil {
		t.Fatal(err)
	}
	view2, _ := f.svc.Job(ctx, hGuest, job2.ID)
	adv := func(to domain.JobStatus, code string, cash *int64) error {
		_, err := f.svc.Advance(ctx, hStaffA, domain.AgentDriver, job2.ID, to, code, cash)
		return err
	}
	if err := adv(domain.JobAtPickup, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := adv(domain.JobInTransit, view2.ProofCode, nil); err != nil {
		t.Fatal(err)
	}
	// Once on the way a trip cannot be cancelled by the customer.
	if _, err := f.svc.Cancel(ctx, hGuest, job2.ID, "x"); !errors.Is(err, domain.ErrInvalidJobTransition) {
		t.Fatalf("cancel in transit: %v", err)
	}
	wrong, exact := int64(400), int64(500)
	if err := adv(domain.JobCompleted, "", &wrong); !errors.Is(err, domain.ErrCashMismatch) {
		t.Fatalf("wrong cash: %v", err)
	}
	if err := adv(domain.JobCompleted, "", nil); !errors.Is(err, domain.ErrCashMismatch) {
		t.Fatalf("no cash: %v", err)
	}
	if err := adv(domain.JobCompleted, "", &exact); err != nil {
		t.Fatal(err)
	}
	e, err := f.svc.Earnings(ctx, hStaffA, domain.AgentDriver)
	if err != nil || e.CompletedJobs != 1 || e.FeesMinor != 500 || e.CashHeldMinor != 500 || e.Currency != "USD" {
		t.Fatalf("earnings = %+v %v", e, err)
	}
	if a, _ := f.repo.FindAgentByUser(ctx, hStaffA, domain.AgentDriver); a.Status != domain.AgentOnline {
		t.Fatalf("agent after completion = %s", a.Status)
	}
	// A finished job cannot be cancelled or advanced.
	if _, err := f.svc.Cancel(ctx, hGuest, job2.ID, "x"); !errors.Is(err, domain.ErrInvalidJobTransition) {
		t.Fatalf("cancel after completion: %v", err)
	}
}

func TestDispatch_Isolation(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	for _, u := range []string{hStaffA, hOwnerB} {
		if _, err := f.svc.RegisterAgent(ctx, u, domain.AgentDriver, domain.VehicleCar); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.GoOnline(ctx, hStaffA, domain.AgentDriver, gombe); err != nil {
		t.Fatal(err)
	}
	job := f.ride(t, hGuest, 0)
	offer := f.offerFor(t, hStaffA, domain.AgentDriver)
	if _, err := f.svc.Accept(ctx, hStaffA, domain.AgentDriver, offer.OfferID); err != nil {
		t.Fatal(err)
	}

	// Another customer, and another agent, cannot read, cancel or advance it.
	if _, err := f.svc.Job(ctx, hGuest2, job.ID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("stranger read the job: %v", err)
	}
	if _, err := f.svc.Cancel(ctx, hGuest2, job.ID, "x"); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("stranger cancelled the job: %v", err)
	}
	if _, err := f.svc.Advance(ctx, hOwnerB, domain.AgentDriver, job.ID, domain.JobAtPickup, "", nil); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("another driver advanced the job: %v", err)
	}
	if _, err := f.svc.Job(ctx, "", job.ID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Errorf("anonymous read the job: %v", err)
	}
	// The driver position is the customer view only while the driver holds the job.
	if v, _ := f.svc.Job(ctx, hGuest, job.ID); v.Agent == nil || v.Agent.Position == nil {
		t.Error("the customer should see the driver position")
	}
	// Positions are recorded for online agents only.
	if err := f.svc.ReportPosition(ctx, hOwnerB, domain.AgentDriver, nearby); !errors.Is(err, domain.ErrAgentUnavailable) {
		t.Errorf("an offline agent reported a position: %v", err)
	}
	if err := f.svc.ReportPosition(ctx, hStaffA, domain.AgentDriver, domain.GeoPoint{Latitude: 200, Longitude: 0}); !errors.Is(err, domain.ErrInvalidLocation) {
		t.Errorf("invalid position accepted: %v", err)
	}
	if err := f.svc.ReportPosition(ctx, hStaffA, domain.AgentDriver, nearby); err != nil {
		t.Errorf("valid position refused: %v", err)
	}
	// One profile per user and kind.
	if _, err := f.svc.RegisterAgent(ctx, hStaffA, domain.AgentDriver, domain.VehicleCar); !errors.Is(err, domain.ErrInvalidAgent) {
		t.Errorf("duplicate agent: %v", err)
	}
}

func TestDispatch_StalePositionIsNotMatched(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	if _, err := f.svc.RegisterAgent(ctx, hStaffA, domain.AgentDriver, domain.VehicleCar); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.GoOnline(ctx, hStaffA, domain.AgentDriver, gombe); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(domain.StalePositionAfter + time.Second) // the driver went silent
	f.ride(t, hGuest, 0)
	if offers, _ := f.svc.PendingOffers(ctx, hStaffA, domain.AgentDriver); len(offers) != 0 {
		t.Fatalf("a silent driver was offered a ride: %+v", offers)
	}
	// A fresh position brings them back into matching.
	if err := f.svc.ReportPosition(ctx, hStaffA, domain.AgentDriver, gombe); err != nil {
		t.Fatal(err)
	}
	f.svc.Tick(ctx)
	if offers, _ := f.svc.PendingOffers(ctx, hStaffA, domain.AgentDriver); len(offers) != 1 {
		t.Fatalf("offers after a fresh position = %+v", offers)
	}
}

func TestDispatch_FoodDeliveryFollowsAnOrder(t *testing.T) {
	f := newDispatchFixture(t)
	ctx := context.Background()
	rt, err := f.hotelFixture.svc.CreateRoomType(ctx, hOwnerA, hotel.NewRoomType{StoreID: hStoreA, SellerID: hSellerA, CategoryID: hCategory,
		Title: "Poulet mayo", Description: "x", PriceAmount: 8, PriceCurrency: "USD", MaxOccupancy: 2, Units: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.listings.PublishListing(ctx, hOwnerA, rt.ListingID); err != nil {
		t.Fatal(err)
	}
	orders := application.NewOrderUseCases(postgres.NewOrderRepository(f.db.DB), postgres.NewListingRepository(f.db.DB),
		postgres.NewSellerRepository(f.db.DB), postgres.NewOrganizationRepository(f.db.DB), postgres.NewUserRepository(f.db.DB),
		postgres.NewInventoryRepository(f.db.DB), postgres.NewFulfillmentRepository(f.db.DB), nil)
	order, err := orders.CreateOrderWithLines(ctx, hGuest, []domain.OrderLine{{ListingID: rt.ListingID, Quantity: 2}}, "")
	if err != nil {
		t.Fatal(err)
	}
	f.courier(t, hGuest2, nearby)

	org, orderID := hOrgA, order.ID
	req := domain.NewJob{Kind: domain.JobFoodDelivery, OrganizationID: &org, OrderID: &orderID, CustomerID: hGuest,
		Pickup: gombe, Dropoff: limete, Currency: "USD", FeeMinor: 300, CashDueMinor: 1900}
	job, err := f.svc.Request(ctx, req)
	if err != nil || job.Status != domain.JobOffered {
		t.Fatalf("job = %+v %v", job, err)
	}
	// One delivery job per order.
	if _, err := f.svc.Request(ctx, req); !errors.Is(err, domain.ErrJobAlreadyExists) {
		t.Fatalf("second job for one order: %v", err)
	}
	// The restaurant team sees the job but never the code or the courier position.
	view, err := f.svc.JobForOrder(ctx, hOwnerA, order.ID)
	if err != nil || view.ProofCode != "" {
		t.Fatalf("team view = %+v %v", view, err)
	}
	// The other restaurant does not see it at all.
	if _, err := f.svc.JobForOrder(ctx, hOwnerB, order.ID); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("other restaurant: %v", err)
	}
	// A food delivery is confirmed at hand-over, with cash.
	offer := f.offerFor(t, hGuest2, domain.AgentCourier)
	if _, err := f.svc.Accept(ctx, hGuest2, domain.AgentCourier, offer.OfferID); err != nil {
		t.Fatal(err)
	}
	cv, _ := f.svc.JobForOrder(ctx, hGuest, order.ID)
	cash := int64(1900)
	for _, to := range []domain.JobStatus{domain.JobAtPickup, domain.JobInTransit} {
		if _, err := f.svc.Advance(ctx, hGuest2, domain.AgentCourier, job.ID, to, "", nil); err != nil {
			t.Fatalf("%s: %v", to, err)
		}
	}
	if _, err := f.svc.Advance(ctx, hGuest2, domain.AgentCourier, job.ID, domain.JobCompleted, "abcd", &cash); !errors.Is(err, domain.ErrWrongProofCode) {
		t.Fatal("hand-over accepted a wrong code")
	}
	if done, err := f.svc.Advance(ctx, hGuest2, domain.AgentCourier, job.ID, domain.JobCompleted, cv.ProofCode, &cash); err != nil || done.Status != domain.JobCompleted {
		t.Fatalf("hand-over: %+v %v", done, err)
	}
}
