package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

// The administration lists read the real schema: orders, bookings and businesses across the platform, with the
// names involved, a text search, a status filter and paging, and with a search word that is only ever data.
func TestAdminCommerceLists_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const (
		owner, buyerA, buyerB = "5e11e700-0000-4000-8000-0000000000b1", "5e11e700-0000-4000-8000-0000000000b2", "5e11e700-0000-4000-8000-0000000000b3"
		org                   = "5e11e700-0000-4000-8000-00000000000b"
		seller, store         = "5e11e700-0000-4000-8000-0000000005b1", "5e11e700-0000-4000-8000-0000000007b1"
		category, listing     = "5e11e700-0000-4000-8000-00000000b002", "5e11e700-0000-4000-8000-0000000001b1"
	)
	sellerUser(t, db, ctx, owner, "+243810000051")
	sellerUser(t, db, ctx, buyerA, "+243810000052")
	sellerUser(t, db, ctx, buyerB, "+243810000053")
	if _, err := db.DB.ExecContext(ctx, `UPDATE users SET display_name = CASE id WHEN $1 THEN 'Marie Kalala' WHEN $2 THEN 'Paul Ilunga' ELSE display_name END`, buyerA, buyerB); err != nil {
		t.Fatal(err)
	}
	createTestOrganization(t, db, ctx, org)
	orgs := postgres.NewOrganizationRepository(db.DB)
	if err := orgs.AddMember(ctx, domain.NewOrganizationMember(owner, org, domain.OrganizationRoleOwner)); err != nil {
		t.Fatal(err)
	}
	createTestSeller(t, db, ctx, seller, owner, org)
	createTestStore(t, db, ctx, store, org)
	createTestCategory(t, db, ctx, category)
	createTestListing(t, db, ctx, listing, org, seller, store, category)
	if _, err := db.DB.ExecContext(ctx, `UPDATE listings SET status='published', title='Double room with a view' WHERE id=$1`, listing); err != nil {
		t.Fatal(err)
	}

	uc := application.NewOrderUseCases(postgres.NewOrderRepository(db.DB), postgres.NewListingRepository(db.DB), postgres.NewSellerRepository(db.DB),
		orgs, postgres.NewUserRepository(db.DB), postgres.NewInventoryRepository(db.DB), postgres.NewFulfillmentRepository(db.DB), nil)
	var orderIDs []string
	for _, buyer := range []string{buyerA, buyerA, buyerB} {
		o, err := uc.CreateOrder(ctx, buyer, listing, 2)
		if err != nil {
			t.Fatalf("order: %v", err)
		}
		orderIDs = append(orderIDs, o.ID)
	}
	// orders are timestamped to the second: give them distinct creation times, since the list is newest first
	for i, id := range orderIDs {
		if _, err := db.DB.ExecContext(ctx, `UPDATE orders SET created_at = NOW() - make_interval(mins => $2) WHERE id = $1`, id, 30-10*i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, orderIDs[0]); err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 1, 14, 0, 0, 0, time.UTC)
	for i, status := range []string{"confirmed", "cancelled"} {
		if _, err := db.DB.ExecContext(ctx, `
			INSERT INTO reservations (organization_id, customer_id, listing_id, status, start_at, end_at, quantity, party_size)
			VALUES ($1, $2, $3, $4, $5, $6, 1, 2)`, org, buyerA, listing, status, start.AddDate(0, 0, 10*i), start.AddDate(0, 0, 10*i+2)); err != nil {
			t.Fatal(err)
		}
	}

	admin := postgres.NewAdminRepository(db.DB)

	orders, total, err := admin.ListOrders(ctx, domain.AdminListFilter{}, 25, 0)
	if err != nil || total != 3 || len(orders) != 3 {
		t.Fatalf("orders: %d of %d, %v", len(orders), total, err)
	}
	if orders[0].ID != orderIDs[2] {
		t.Fatalf("newest first: %s, want %s", orders[0].ID, orderIDs[2])
	}
	o := orders[0]
	if o.CustomerName != "Paul Ilunga" || o.OrganizationName == "" || o.Title == "" || o.ItemCount != 1 || o.Currency == "" || o.TotalMinor == 0 || o.CreatedAt == 0 {
		t.Fatalf("order row: %+v", o)
	}

	count := func(name string, f domain.AdminListFilter, want int) {
		t.Helper()
		_, n, err := admin.ListOrders(ctx, f, 25, 0)
		if err != nil || n != want {
			t.Errorf("orders %s: %d, want %d (%v)", name, n, want, err)
		}
	}
	count("by status", domain.AdminListFilter{Status: "paid"}, 1)
	count("by customer name", domain.AdminListFilter{Query: "marie"}, 2)
	count("by business name", domain.AdminListFilter{Query: orders[0].OrganizationName[:4]}, 3)
	count("by id", domain.AdminListFilter{Query: orderIDs[1][:8]}, 1)
	count("search and status", domain.AdminListFilter{Query: "marie", Status: "paid"}, 1)
	count("nothing matches", domain.AdminListFilter{Query: "nobody at all"}, 0)
	count("a wildcard is a letter", domain.AdminListFilter{Query: "%"}, 0)
	count("an injection is only text", domain.AdminListFilter{Query: "'; DROP TABLE orders; --"}, 0)
	if _, n, _ := admin.ListOrders(ctx, domain.AdminListFilter{}, 25, 0); n != 3 {
		t.Fatalf("the orders table was touched: %d", n)
	}
	page, total, err := admin.ListOrders(ctx, domain.AdminListFilter{}, 2, 2)
	if err != nil || total != 3 || len(page) != 1 || page[0].ID != orderIDs[0] {
		t.Fatalf("second page of two: %v total %d %v", page, total, err)
	}

	bookings, total, err := admin.ListReservations(ctx, domain.AdminListFilter{}, 25, 0)
	if err != nil || total != 2 || len(bookings) != 2 {
		t.Fatalf("bookings: %d of %d, %v", len(bookings), total, err)
	}
	b := bookings[0]
	if b.Title != "Double room with a view" || b.CustomerName != "Marie Kalala" || b.OrganizationName == "" || b.PartySize != 2 || b.EndAt <= b.StartAt {
		t.Fatalf("booking row: %+v", b)
	}
	if _, n, _ := admin.ListReservations(ctx, domain.AdminListFilter{Status: "cancelled"}, 25, 0); n != 1 {
		t.Fatalf("cancelled bookings: %d", n)
	}
	if _, n, _ := admin.ListReservations(ctx, domain.AdminListFilter{Query: "double room"}, 25, 0); n != 2 {
		t.Fatalf("bookings by listing title: %d", n)
	}

	businesses, total, err := admin.ListOrganizations(ctx, domain.AdminListFilter{}, 25, 0)
	if err != nil || total != 1 || len(businesses) != 1 {
		t.Fatalf("organizations: %d of %d, %v", len(businesses), total, err)
	}
	g := businesses[0]
	if g.Members != 1 || g.Stores != 1 || g.PublishedListings != 1 || g.Orders != 3 || g.Status != "active" || g.DisplayName == "" {
		t.Fatalf("organization row: %+v", g)
	}
	if _, n, _ := admin.ListOrganizations(ctx, domain.AdminListFilter{Status: "suspended"}, 25, 0); n != 0 {
		t.Fatalf("suspended businesses: %d", n)
	}
	if _, n, _ := admin.ListOrganizations(ctx, domain.AdminListFilter{Query: strings.ToLower(g.DisplayName[:3])}, 25, 0); n != 1 {
		t.Fatalf("business by name: %d", n)
	}
}
