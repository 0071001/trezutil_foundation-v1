package negotiation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

const (
	buyer     = "buyer-1"
	supplierA = "supplier-a" // user answering for org-a
	supplierB = "supplier-b" // user answering for org-b
	stranger  = "stranger"
)

type world struct {
	svc    *Service
	store  *memStore
	orders *fakeOrders
	now    time.Time
}

func newWorld(t *testing.T) *world {
	t.Helper()
	listings := fakeListings{
		"l-a":    {ID: "l-a", OrganizationID: "org-a", Title: "Ciment CPJ 42.5", PriceAmount: 50, PriceCurrency: "USD", Status: domain.ListingStatusPublished},
		"l-b":    {ID: "l-b", OrganizationID: "org-b", Title: "Ciment CPJ 42.5 (sac)", PriceAmount: 52, PriceCurrency: "USD", Status: domain.ListingStatusPublished},
		"l-cdf":  {ID: "l-cdf", OrganizationID: "org-b", Title: "Fer à béton", PriceAmount: 38000, PriceCurrency: "CDF", Status: domain.ListingStatusPublished},
		"l-demo": {ID: "l-demo", OrganizationID: "org-demo", Title: "Démo", PriceAmount: 10, PriceCurrency: "USD", Status: domain.ListingStatusPublished, Metadata: map[string]interface{}{"demo": true}},
		"l-own":  {ID: "l-own", OrganizationID: "org-buyer", Title: "Mine", PriceAmount: 10, PriceCurrency: "USD", Status: domain.ListingStatusPublished},
		"l-off":  {ID: "l-off", OrganizationID: "org-a", Title: "Paused", PriceAmount: 10, PriceCurrency: "USD", Status: domain.ListingStatusPaused},
	}
	orgs := fakeOrgs{
		orgs: map[string]*domain.Organization{
			"org-a": {ID: "org-a", DisplayName: "Quincaillerie A", Phone: "0821714747"},
			"org-b": {ID: "org-b", DisplayName: "Quincaillerie B", Phone: ""},
		},
		members: map[string]domain.OrganizationRole{
			"org-a|" + supplierA: domain.OrganizationRoleOwner,
			"org-b|" + supplierB: domain.OrganizationRoleSales,
			"org-buyer|" + buyer: domain.OrganizationRoleOwner,
		},
	}
	w := &world{store: newMemStore(), orders: &fakeOrders{}, now: time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)}
	w.svc = NewService(w.store, listings, orgs, orgs, w.orders, nil, Config{PortalURL: "https://trezutil.test/supplier/requests", WhatsApp: true}).
		WithClock(func() time.Time { return w.now })
	return w
}

func (w *world) start(t *testing.T, listings ...string) *Snapshot {
	t.Helper()
	snap, err := w.svc.Start(context.Background(), buyer, StartRequest{ListingIDs: listings, Quantity: 10, TargetPrice: 40, MaxPrice: 46, DeadlineHours: 48, MaxRounds: 2})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	return snap
}

func participantOf(t *testing.T, s *Snapshot, org string) Participant {
	t.Helper()
	for _, p := range s.Participants {
		if p.OrganizationID == org {
			return p
		}
	}
	t.Fatalf("no participant for %s", org)
	return Participant{}
}

func allText(s *Snapshot) string {
	var b strings.Builder
	for _, m := range s.Messages {
		b.WriteString(m.Body + "\n")
	}
	return b.String()
}

func TestStartAsksEverySupplierWithoutDisclosingTheCeiling(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a", "l-b")
	if snap.Negotiation.Status != StatusSoliciting || len(snap.Participants) != 2 {
		t.Fatalf("got %+v", snap.Negotiation)
	}
	if snap.Mandate.TargetUnitMinor != 4000 || snap.Mandate.MaxUnitMinor != 4600 {
		t.Fatalf("mandate %+v", snap.Mandate)
	}
	// Portal for both; WhatsApp for org-a (phone), skipped for org-b (none).
	var portal, wa, skipped int
	for _, m := range snap.Messages {
		switch {
		case m.Channel == "portal":
			portal++
		case m.Channel == "whatsapp" && m.State == "pending" && m.Recipient == "+243821714747":
			wa++
		case m.Channel == "whatsapp" && m.State == "skipped":
			skipped++
		}
	}
	if portal != 2 || wa != 1 || skipped != 1 {
		t.Fatalf("messages portal=%d wa=%d skipped=%d", portal, wa, skipped)
	}
	text := allText(snap)
	if strings.Contains(text, "46") {
		t.Fatalf("the ceiling leaked: %s", text)
	}
	if !strings.Contains(text, "pour le compte d'un acheteur") || !strings.Contains(text, "https://trezutil.test/supplier/requests") {
		t.Fatalf("request text: %s", text)
	}
}

func TestStartRefusesWhatCannotBeNegotiated(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	base := StartRequest{Quantity: 1, TargetPrice: 5, MaxPrice: 6}
	cases := map[string]struct {
		ids  []string
		want error
	}{
		"demo supplier":  {[]string{"l-demo"}, ErrDemoSupplier},
		"own business":   {[]string{"l-own"}, ErrOwnOffer},
		"two currencies": {[]string{"l-a", "l-cdf"}, ErrMixedCurrency},
		"not published":  {[]string{"l-off"}, ErrUnavailable},
	}
	for name, c := range cases {
		req := base
		req.ListingIDs = c.ids
		if _, err := w.svc.Start(ctx, buyer, req); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", name, err, c.want)
		}
	}
	bad := base
	bad.ListingIDs = []string{"l-a"}
	bad.MaxPrice = 4
	var inv *InvalidError
	if _, err := w.svc.Start(ctx, buyer, bad); !errors.As(err, &inv) || inv.Field != "max_price" {
		t.Fatalf("a ceiling below the target: %v", err)
	}
}

func TestAnOfferAboveTheTargetIsCounteredWithinTheMandate(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	w.now = w.now.Add(time.Hour)
	view, err := w.svc.SupplierOffer(context.Background(), supplierA, p.ID, OfferRequest{UnitPrice: 48, ValidDays: 5})
	if err != nil {
		t.Fatalf("offer: %v", err)
	}
	full, _ := w.store.Load(context.Background(), snap.Negotiation.ID)
	counter := full.OpenOffer(p.ID, AuthorBuyer)
	if counter == nil || counter.Source != SourceStrategy {
		t.Fatalf("expected an automatic counter, got %+v", full.Offers)
	}
	// Round 1 of 2: a third of the way from the target (40) to the ceiling (46) → 42.
	if counter.UnitMinor != 4200 {
		t.Fatalf("counter %d", counter.UnitMinor)
	}
	if full.Negotiation.Status != StatusNegotiating || full.Participant(p.ID).Status != PCountered || full.Participant(p.ID).Round != 2 {
		t.Fatalf("state %s %+v", full.Negotiation.Status, full.Participant(p.ID))
	}
	if strings.Contains(allText(full), "46,00") {
		t.Fatal("the ceiling leaked in the counter")
	}
	// The supplier's view hides the mandate and the buyer.
	if view.Mandate.MaxUnitMinor != 0 || view.Mandate.TargetUnitMinor != 0 || view.Negotiation.BuyerID != "" {
		t.Fatalf("supplier view leaks: %+v %+v", view.Mandate, view.Negotiation)
	}

	// Second round: the supplier comes down to 45; two thirds of the way from 40 to 44.99 → 43.33, tidied to 43.
	if _, err := w.svc.SupplierOffer(context.Background(), supplierA, p.ID, OfferRequest{UnitPrice: 45}); err != nil {
		t.Fatalf("offer 2: %v", err)
	}
	full, _ = w.store.Load(context.Background(), snap.Negotiation.ID)
	if c := full.OpenOffer(p.ID, AuthorBuyer); c == nil || c.UnitMinor != 4300 {
		t.Fatalf("second counter %+v", c)
	}
	// Third answer: no round left → no counter; the buyer decides.
	if _, err := w.svc.SupplierOffer(context.Background(), supplierA, p.ID, OfferRequest{UnitPrice: 44.5}); err != nil {
		t.Fatalf("offer 3: %v", err)
	}
	full, _ = w.store.Load(context.Background(), snap.Negotiation.ID)
	if full.OpenOffer(p.ID, AuthorBuyer) != nil || full.Negotiation.Status != StatusAwaitingDecision {
		t.Fatalf("after the last round: %s %+v", full.Negotiation.Status, full.OpenOffer(p.ID, AuthorBuyer))
	}
}

func TestAnOfferAtTheTargetGoesToTheBuyerAndAcceptingCreatesTheOrderAtThatPrice(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a", "l-b")
	pa, pb := participantOf(t, snap, "org-a"), participantOf(t, snap, "org-b")
	ctx := context.Background()
	if _, err := w.svc.SupplierOffer(ctx, supplierB, pb.ID, OfferRequest{UnitPrice: 39.5}); err != nil {
		t.Fatalf("offer: %v", err)
	}
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	if full.Negotiation.Status != StatusAwaitingDecision || full.OpenOffer(pb.ID, AuthorBuyer) != nil {
		t.Fatalf("a good offer is not countered: %s", full.Negotiation.Status)
	}
	// No supplier answer creates an order.
	if len(w.orders.made) != 0 {
		t.Fatal("an order was created without the buyer")
	}
	offer := full.OpenOffer(pb.ID, AuthorSupplier)
	if _, err := w.svc.Accept(ctx, stranger, snap.Negotiation.ID, offer.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("someone else accepted: %v", err)
	}
	done, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if done.Negotiation.Status != StatusAccepted || done.Negotiation.OrderID != "order-1" {
		t.Fatalf("accepted %+v", done.Negotiation)
	}
	if len(w.orders.made) != 1 || w.orders.made[0].unit != 3950 || w.orders.made[0].quantity != 10 || w.orders.made[0].listing != "l-b" || w.orders.made[0].key != "negotiation:"+snap.Negotiation.ID+":"+offer.ID {
		t.Fatalf("order %+v", w.orders.made)
	}
	if done.Participant(pa.ID).Status != PRejected || done.Participant(pb.ID).Status != PAccepted {
		t.Fatalf("participants %+v", done.Participants)
	}
	if !strings.Contains(allText(done), "a retenu une autre offre") {
		t.Fatal("the other supplier was not told")
	}
	if _, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("accepted twice: %v", err)
	}
}

func TestASupplierMayAcceptTheCounterButTheBuyerStillDecides(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	ctx := context.Background()
	if _, err := w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 50, DeliveryDays: ptr(3)}); err != nil {
		t.Fatal(err)
	}
	view, err := w.svc.SupplierAcceptCounter(ctx, supplierA, p.ID)
	if err != nil {
		t.Fatalf("accept counter: %v", err)
	}
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	offer := full.OpenOffer(p.ID, AuthorSupplier)
	if offer == nil || offer.UnitMinor != 4200 || offer.DeliveryDays == nil || *offer.DeliveryDays != 3 {
		t.Fatalf("offer at the counter price %+v", offer)
	}
	if full.OpenOffer(p.ID, AuthorBuyer) != nil || full.Negotiation.Status != StatusAwaitingDecision || len(w.orders.made) != 0 {
		t.Fatalf("state %s, orders %d", full.Negotiation.Status, len(w.orders.made))
	}
	if len(view.Participants) != 1 {
		t.Fatal("supplier view")
	}
}

func TestOnlyTheSupplierAnswersForItsOwnRequest(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a", "l-b")
	pa := participantOf(t, snap, "org-a")
	ctx := context.Background()
	if _, err := w.svc.SupplierOffer(ctx, supplierB, pa.ID, OfferRequest{UnitPrice: 30}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another supplier answered: %v", err)
	}
	inbox, err := w.svc.Inbox(ctx, supplierB)
	if err != nil || len(inbox) != 1 || len(inbox[0].Participants) != 1 || inbox[0].Participants[0].OrganizationID != "org-b" {
		t.Fatalf("inbox %v %+v", err, inbox)
	}
	if inbox[0].Mandate.MaxUnitMinor != 0 || inbox[0].Negotiation.BuyerID != "" {
		t.Fatal("the inbox leaks the mandate or the buyer")
	}
	if none, _ := w.svc.Inbox(ctx, stranger); len(none) != 0 {
		t.Fatal("a stranger sees requests")
	}
}

func TestTheBuyerCountersThemselvesAndCanReject(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	ctx := context.Background()
	w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 39}) // good: no automatic counter
	if _, err := w.svc.Counter(ctx, buyer, snap.Negotiation.ID, CounterRequest{ParticipantID: p.ID, UnitPrice: 39}); err == nil {
		t.Fatal("a counter at the supplier's own price")
	}
	got, err := w.svc.Counter(ctx, buyer, snap.Negotiation.ID, CounterRequest{ParticipantID: p.ID, UnitPrice: 37})
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	if c := got.OpenOffer(p.ID, AuthorBuyer); c == nil || c.Source != SourceBuyer || c.UnitMinor != 3700 {
		t.Fatalf("counter %+v", c)
	}
	got, err = w.svc.Reject(ctx, buyer, snap.Negotiation.ID, p.ID)
	if err != nil || got.Negotiation.Status != StatusRejected {
		t.Fatalf("reject: %v %s", err, got.Negotiation.Status)
	}
}

func TestDeadlinesAndReminders(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a", "l-b")
	pa := participantOf(t, snap, "org-a")
	ctx := context.Background()

	w.now = w.now.Add(25 * time.Hour) // past half of 48 h
	if _, err := w.svc.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	reminders := 0
	for _, e := range full.Events {
		if e.Kind == "supplier.reminded" {
			reminders++
		}
	}
	if reminders != 2 {
		t.Fatalf("reminders %d", reminders)
	}
	w.svc.Tick(ctx) // once only
	full, _ = w.store.Load(ctx, snap.Negotiation.ID)
	again := 0
	for _, e := range full.Events {
		if e.Kind == "supplier.reminded" {
			again++
		}
	}
	if again != 2 {
		t.Fatalf("reminded twice: %d", again)
	}

	w.svc.SupplierOffer(ctx, supplierA, pa.ID, OfferRequest{UnitPrice: 60})
	w.now = w.now.Add(24 * time.Hour) // deadline
	w.svc.Tick(ctx)
	full, _ = w.store.Load(ctx, snap.Negotiation.ID)
	if full.Negotiation.Status != StatusAwaitingDecision {
		t.Fatalf("with an offer in hand: %s", full.Negotiation.Status)
	}
	if _, err := w.svc.SupplierOffer(ctx, supplierA, pa.ID, OfferRequest{UnitPrice: 41}); !errors.Is(err, ErrClosed) {
		t.Fatalf("answered after the deadline: %v", err)
	}
	w.now = w.now.Add(8 * 24 * time.Hour)
	w.svc.Tick(ctx)
	full, _ = w.store.Load(ctx, snap.Negotiation.ID)
	if full.Negotiation.Status != StatusExpired {
		t.Fatalf("a week later: %s", full.Negotiation.Status)
	}
}

func TestDelivererSendsOnlyWithAnApprovedTemplate(t *testing.T) {
	w := newWorld(t)
	w.start(t, "l-a")
	ctx := context.Background()
	d := NewDeliverer(w.store, nil, "", "fr", zeroLogger())
	d.Run(ctx)
	pending, _ := w.store.PendingMessages(ctx, 10)
	if len(pending) != 0 {
		t.Fatal("messages left pending without a sender")
	}
	for _, s := range w.store.snap {
		for _, m := range s.Messages {
			if m.Channel == "whatsapp" && m.Recipient != "" && (m.State != "skipped" || !strings.Contains(m.LastError, "template")) {
				t.Fatalf("not honest about WhatsApp: %+v", m)
			}
		}
	}

	w2 := newWorld(t)
	w2.start(t, "l-a")
	sender := &fakeSender{}
	sent, _ := NewDeliverer(w2.store, sender, "trezutil_supplier_update", "fr", zeroLogger()).Run(ctx)
	if sent != 1 || len(sender.to) != 1 || sender.to[0] != "+243821714747" || sender.template != "trezutil_supplier_update" {
		t.Fatalf("sent %d %+v", sent, sender)
	}
}

type fakeSender struct {
	to       []string
	template string
}

func (f *fakeSender) SendTemplate(ctx context.Context, to, name, lang string, params []string) (string, error) {
	f.to = append(f.to, to)
	f.template = name
	return "wamid.1", nil
}

func ptr(i int) *int { return &i }

func TestNothingChangesWhileAnAcceptedOfferBecomesAnOrder(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	ctx := context.Background()
	w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 39})
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	offer := full.OpenOffer(p.ID, AuthorSupplier)
	// While the order is being written, the supplier raises the price and the buyer withdraws: both come too late.
	var during []error
	w.orders.hook = func() error {
		_, errOffer := w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 45})
		_, errCancel := w.svc.Cancel(ctx, buyer, snap.Negotiation.ID)
		during = []error{errOffer, errCancel}
		return nil
	}
	done, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	for _, e := range during {
		if !errors.Is(e, ErrClosed) {
			t.Fatalf("the negotiation changed during the acceptance: %v", during)
		}
	}
	if len(w.orders.made) != 1 || w.orders.made[0].unit != 3900 || done.Negotiation.OrderID != "order-1" || done.Negotiation.Status != StatusAccepted {
		t.Fatalf("order %+v negotiation %+v", w.orders.made, done.Negotiation)
	}
}

func TestAFailedOrderReopensTheNegotiation(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	ctx := context.Background()
	w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 39})
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	offer := full.OpenOffer(p.ID, AuthorSupplier)
	messages := len(full.Messages)
	w.orders.hook = func() error { return domain.ErrListingUnavailable }
	if _, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID); !errors.Is(err, domain.ErrListingUnavailable) {
		t.Fatalf("accept: %v", err)
	}
	full, _ = w.store.Load(ctx, snap.Negotiation.ID)
	if full.Negotiation.Status != StatusAwaitingDecision || full.Negotiation.AcceptedOfferID != "" || full.Negotiation.OrderID != "" ||
		full.OpenOffer(p.ID, AuthorSupplier) == nil || len(full.Messages) != messages {
		t.Fatalf("not reopened: %+v", full.Negotiation)
	}
	w.orders.hook = nil
	if done, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID); err != nil || done.Negotiation.OrderID != "order-1" {
		t.Fatalf("second try: %v", err)
	}
}

func TestAnInterruptedAcceptanceIsFinishedByAcceptingAgain(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a")
	p := participantOf(t, snap, "org-a")
	ctx := context.Background()
	w.svc.SupplierOffer(ctx, supplierA, p.ID, OfferRequest{UnitPrice: 39})
	full, _ := w.store.Load(ctx, snap.Negotiation.ID)
	offer := full.OpenOffer(p.ID, AuthorSupplier)
	// The process stopped after recording the choice, before the order.
	if err := w.store.Apply(ctx, Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: full.Negotiation.Version, Status: StatusAccepted,
		AcceptedOfferID: offer.ID, OfferStatuses: map[string]string{offer.ID: OfferAccepted}}); err != nil {
		t.Fatal(err)
	}
	done, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID)
	if err != nil || done.Negotiation.OrderID != "order-1" || done.Participant(p.ID).Status != PAccepted {
		t.Fatalf("resume: %v %+v", err, done)
	}
	if _, err := w.svc.Accept(ctx, buyer, snap.Negotiation.ID, offer.ID); !errors.Is(err, ErrClosed) {
		t.Fatalf("accepted twice: %v", err)
	}
}

func TestASupplierNeverReadsAnotherSuppliersOffer(t *testing.T) {
	w := newWorld(t)
	snap := w.start(t, "l-a", "l-b")
	pb := participantOf(t, snap, "org-b")
	view, err := w.svc.SupplierGet(context.Background(), supplierB, pb.ID)
	if err != nil {
		t.Fatal(err)
	}
	if view.Negotiation.Title != "Ciment CPJ 42.5 (sac)" {
		t.Fatalf("title %q", view.Negotiation.Title)
	}
}
