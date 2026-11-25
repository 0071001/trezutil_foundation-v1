package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/application/negotiation"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
)

// Negotiation on real PostgreSQL (migration 000037): the requests, the automatic counter within the mandate, the
// supplier's view, the outbox, the deadline queries, and the order at the agreed price.
func TestNegotiation_FromRequestToOrder(t *testing.T) {
	f := newHotelFixture(t)
	ctx := context.Background()
	db := f.db.DB
	mk := func(actor, store, seller, title string, price float64) string {
		rt, err := f.svc.CreateRoomType(ctx, actor, hotel.NewRoomType{
			StoreID: store, SellerID: seller, CategoryID: hCategory, Title: title, Description: "x",
			PriceAmount: price, PriceCurrency: "USD", MaxOccupancy: 2, Units: 5,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := f.listings.PublishListing(ctx, actor, rt.ListingID); err != nil {
			t.Fatal(err)
		}
		return rt.ListingID
	}
	a := mk(hOwnerA, hStoreA, hSellerA, "Room A", 50)
	b := mk(hOwnerB, hStoreB, hSellerB, "Room B", 52)

	orders := application.NewOrderUseCases(postgres.NewOrderRepository(db), postgres.NewListingRepository(db),
		postgres.NewSellerRepository(db), postgres.NewOrganizationRepository(db), postgres.NewUserRepository(db),
		postgres.NewInventoryRepository(db), postgres.NewFulfillmentRepository(db), nil)
	orgs := postgres.NewOrganizationRepository(db)
	store := postgres.NewNegotiationStore(db)
	now := time.Now()
	svc := negotiation.NewService(store, postgres.NewListingRepository(db), orgs, orgs, orders,
		audit.NewLogger(zerolog.Nop()).WithStore(postgres.NewAuditRepository(db)),
		negotiation.Config{PortalURL: "https://trezutil.test/supplier/requests", WhatsApp: true}).
		WithClock(func() time.Time { return now })

	snap, err := svc.Start(ctx, hGuest, negotiation.StartRequest{ListingIDs: []string{a, b}, Quantity: 3, TargetPrice: 40, MaxPrice: 46})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if len(snap.Participants) != 2 || snap.Mandate.MaxUnitMinor != 4600 || len(snap.Events) != 3 {
		t.Fatalf("created %+v events=%d", snap, len(snap.Events))
	}
	var pa, pb negotiation.Participant
	for _, p := range snap.Participants {
		if p.ListingID == a {
			pa = p
		} else {
			pb = p
		}
	}

	// The outbox holds the WhatsApp messages (or why they cannot leave); sending marks them.
	pending, err := store.PendingMessages(ctx, 10)
	if err != nil {
		t.Fatalf("outbox: %v", err)
	}
	for _, m := range pending {
		if err := store.MarkMessage(ctx, m.ID, "sent", "", "wamid."+m.ID); err != nil {
			t.Fatalf("mark: %v", err)
		}
	}
	if left, _ := store.PendingMessages(ctx, 10); len(left) != 0 {
		t.Fatalf("still pending %d", len(left))
	}

	// Supplier A answers above the target: Trezutil counters.
	if _, err := svc.SupplierOffer(ctx, hOwnerA, pa.ID, negotiation.OfferRequest{UnitPrice: 48}); err != nil {
		t.Fatalf("offer A: %v", err)
	}
	full, err := svc.Get(ctx, hGuest, snap.Negotiation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if c := full.OpenOffer(pa.ID, negotiation.AuthorBuyer); c == nil || c.UnitMinor != 4200 || full.Negotiation.Status != negotiation.StatusNegotiating {
		t.Fatalf("counter %+v status %s", c, full.Negotiation.Status)
	}
	inbox, err := svc.Inbox(ctx, hOwnerA)
	if err != nil || len(inbox) != 1 || len(inbox[0].Participants) != 1 || inbox[0].Mandate.MaxUnitMinor != 0 {
		t.Fatalf("inbox %v %+v", err, inbox)
	}

	// A stale change is refused (two answers at once).
	if err := store.Apply(ctx, negotiation.Change{NegotiationID: snap.Negotiation.ID, ExpectedVersion: 1}); !errors.Is(err, negotiation.ErrConflict) {
		t.Fatalf("stale version: %v", err)
	}

	// Reminders halfway, deadlines afterwards.
	if ids, err := store.DueReminders(ctx, now.Add(25*time.Hour), 10); err != nil || len(ids) != 1 {
		t.Fatalf("reminders %v %v", ids, err)
	}
	if ids, err := store.DueDeadlines(ctx, now.Add(49*time.Hour), 10); err != nil || len(ids) != 1 {
		t.Fatalf("deadlines %v %v", ids, err)
	}
	if ids, err := store.DueDeadlines(ctx, now, 10); err != nil || len(ids) != 0 {
		t.Fatalf("nothing due yet %v %v", ids, err)
	}

	// Supplier B meets the target; the buyer accepts: one order at the agreed price.
	if _, err := svc.SupplierOffer(ctx, hOwnerB, pb.ID, negotiation.OfferRequest{UnitPrice: 39.5}); err != nil {
		t.Fatalf("offer B: %v", err)
	}
	full, _ = svc.Get(ctx, hGuest, snap.Negotiation.ID)
	offer := full.OpenOffer(pb.ID, negotiation.AuthorSupplier)
	done, err := svc.Accept(ctx, hGuest, snap.Negotiation.ID, offer.ID)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if done.Negotiation.Status != negotiation.StatusAccepted || done.Negotiation.OrderID == "" || done.Negotiation.AcceptedOfferID != offer.ID {
		t.Fatalf("accepted %+v", done.Negotiation)
	}
	order, err := postgres.NewOrderRepository(db).FindByID(ctx, done.Negotiation.OrderID)
	if err != nil || order.TotalAmount.AmountMinor != 3*3950 || order.CustomerID != hGuest || len(order.Items) != 1 || order.Items[0].UnitPrice.AmountMinor != 3950 {
		t.Fatalf("order %+v %v", order, err)
	}
	list, err := svc.ListForBuyer(ctx, hGuest)
	if err != nil || len(list) != 1 {
		t.Fatalf("list %v %d", err, len(list))
	}
}
