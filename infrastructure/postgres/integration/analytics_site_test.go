package integration

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// The dashboards and the home page's trust strip read real PostgreSQL. These
// tests build a small, fully known platform and check every figure, including
// the awkward ones: Kinshasa day boundaries, drafts, cancelled orders that are
// not revenue, two currencies that must never be added, two businesses that
// must never see each other, and a period before the current one.

const (
	aOwnerA, aOwnerB = "5e11e700-0000-4000-8000-00000000f001", "5e11e700-0000-4000-8000-00000000f002"
	aCust1, aCust2   = "5e11e700-0000-4000-8000-00000000f011", "5e11e700-0000-4000-8000-00000000f012"
	aCust3           = "5e11e700-0000-4000-8000-00000000f013"
	aOrgA, aOrgB     = "5e11e700-0000-4000-8000-00000000f0a1", "5e11e700-0000-4000-8000-00000000f0b1"
	aOrgC            = "5e11e700-0000-4000-8000-00000000f0c1" // a business with no activity at all
	aSellerA         = "5e11e700-0000-4000-8000-00000000f5a1"
	aSellerB         = "5e11e700-0000-4000-8000-00000000f5b1"
	aStoreA          = "5e11e700-0000-4000-8000-00000000f7a1"
	aStoreB          = "5e11e700-0000-4000-8000-00000000f7b1"
	aStoreC          = "5e11e700-0000-4000-8000-00000000f7c1"
	aStoreD          = "5e11e700-0000-4000-8000-00000000f7d1"
	aCategory        = "5e11e700-0000-4000-8000-00000000fc01"
	aL1, aL2, aL3    = "5e11e700-0000-4000-8000-00000000f1a1", "5e11e700-0000-4000-8000-00000000f1a2", "5e11e700-0000-4000-8000-00000000f1b1"
	aL4, aL5         = "5e11e700-0000-4000-8000-00000000f1b2", "5e11e700-0000-4000-8000-00000000f1b3"
)

func exec(t *testing.T, db *TestDB, query string, args ...interface{}) {
	t.Helper()
	if _, err := db.DB.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
}

type analyticsFixture struct {
	db  *TestDB
	now int64
}

// newAnalyticsFixture builds the platform described in the comments below.
// "Now" is 2026-10-04 12:00 UTC, which is 13:00 in Kinshasa, so a 3-day window
// covers 2026-10-02 .. 2026-10-04 and the one before it 2026-09-29 .. 2026-10-01.
func newAnalyticsFixture(t *testing.T) *analyticsFixture {
	t.Helper()
	db := NewTestDB(t)
	ctx := context.Background()
	sellerUser(t, db, ctx, aOwnerA, "+243810000101")
	sellerUser(t, db, ctx, aOwnerB, "+243810000102")
	sellerUser(t, db, ctx, aCust1, "+243810000111")
	sellerUser(t, db, ctx, aCust2, "+243810000112")
	sellerUser(t, db, ctx, aCust3, "+243810000113")
	// Accounts: the owners are old; c1 and c2 joined in the window (c2 at 00:30 Kinshasa on the 4th), c3 in the period before.
	exec(t, db, `UPDATE users SET created_at = '2026-01-01T00:00:00Z'`)
	exec(t, db, `UPDATE users SET created_at = '2026-10-04T08:00:00Z' WHERE id = $1`, aCust1)
	exec(t, db, `UPDATE users SET created_at = '2026-10-03T23:30:00Z' WHERE id = $1`, aCust2)
	exec(t, db, `UPDATE users SET created_at = '2026-09-30T09:00:00Z' WHERE id = $1`, aCust3)

	createTestOrganization(t, db, ctx, aOrgA)
	createTestOrganization(t, db, ctx, aOrgB)
	createTestOrganization(t, db, ctx, aOrgC)
	exec(t, db, `UPDATE organizations SET display_name = 'Gombe Stays', verification_status = 'verified' WHERE id = $1`, aOrgA)
	exec(t, db, `UPDATE organizations SET display_name = 'Kasa Market', verification_status = 'unverified' WHERE id = $1`, aOrgB)
	exec(t, db, `UPDATE organizations SET display_name = 'Quiet Shop' WHERE id = $1`, aOrgC)
	createTestSeller(t, db, ctx, aSellerA, aOwnerA, aOrgA)
	createTestSeller(t, db, ctx, aSellerB, aOwnerB, aOrgB)
	createTestCategory(t, db, ctx, aCategory)

	store := func(id, org, slug, city string) {
		exec(t, db, `INSERT INTO stores (id, organization_id, name, slug, phone, country, city) VALUES ($1, $2, $3, $4, '+243812345678', 'CD', $5)`, id, org, slug, slug, city)
	}
	store(aStoreA, aOrgA, "store-a", "Kinshasa")
	store(aStoreB, aOrgB, "store-b", "kinshasa ") // the same city, spelled differently
	store(aStoreC, aOrgB, "store-c", "Lubumbashi")
	store(aStoreD, aOrgA, "store-d", "Goma") // no published listing: not "covered"

	listing := func(id, org, seller, store, title, status, vertical, metadata, created string, price float64) {
		exec(t, db, `INSERT INTO listings (id, organization_id, seller_id, store_id, category_id, title, listing_type, pricing_model, price_amount, price_currency, status, vertical, metadata, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'physical_product', 'fixed', $7, 'USD', $8, $9, $10::jsonb, $11::timestamptz)`,
			id, org, seller, store, aCategory, title, price, status, vertical, metadata, created)
	}
	listing(aL1, aOrgA, aSellerA, aStoreA, "Café", "published", "RETAIL", `{"promotion":{"compare_at_amount":150}}`, "2026-10-03T10:00:00Z", 100) // a real sale, created in the window
	listing(aL2, aOrgA, aSellerA, aStoreA, "Sofa", "draft", "SERVICES", `{}`, "2026-01-01T00:00:00Z", 80)
	listing(aL3, aOrgB, aSellerB, aStoreB, "Room", "published", "HOTEL", `{}`, "2026-01-01T00:00:00Z", 60)
	listing(aL4, aOrgB, aSellerB, aStoreC, "Lamp", "published", "RETAIL", `{"promotion":{"compare_at_amount":80}}`, "2026-01-01T00:00:00Z", 100) // "compare at" below the price: no saving
	listing(aL5, aOrgB, aSellerB, aStoreB, "Fan", "published", "RETAIL", `{"promotion":"oops"}`, "2026-01-01T00:00:00Z", 100)                    // not a promotion object

	order := func(id, org, seller, store, customer, created, status, currency string, amount int64, items string) {
		exec(t, db, `INSERT INTO orders (id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9, $10::timestamptz, $10::timestamptz)`,
			id, org, seller, customer, store, items, amount, currency, status, created)
	}
	item := func(listing, title string, qty int, total int64) string {
		return `{"ListingID":"` + listing + `","Title":"` + title + `","Quantity":` + strconv.Itoa(qty) + `,"TotalPrice":{"Currency":"USD","AmountMinor":` + strconv.FormatInt(total, 10) + `}}`
	}
	a := func(n string) string { return "5e11e700-0000-4000-8000-00000000e0" + n }
	order(a("a1"), aOrgA, aSellerA, aStoreA, aCust1, "2026-10-04T10:00:00Z", "paid", "USD", 1000, "["+item(aL1, "Café", 2, 1000)+"]")
	order(a("a2"), aOrgA, aSellerA, aStoreA, aCust2, "2026-10-03T23:30:00Z", "completed", "USD", 2500, "["+item(aL1, "Café", 1, 500)+","+item(aL2, "Sofa", 1, 2000)+"]") // 00:30 on the 4th in Kinshasa
	order(a("a3"), aOrgA, aSellerA, aStoreA, aCust2, "2026-10-03T22:30:00Z", "cancelled", "USD", 700, "["+item(aL1, "Café", 1, 700)+"]")                                 // 23:30 on the 3rd: still the 3rd
	order(a("a4"), aOrgA, aSellerA, aStoreA, aCust3, "2026-10-02T08:00:00Z", "draft", "USD", 300, "[]")                                                                  // a cart, never an order
	order(a("a5"), aOrgA, aSellerA, aStoreA, aCust1, "2026-09-30T09:00:00Z", "paid", "USD", 400, "["+item(aL1, "Café", 1, 400)+"]")                                      // the period before
	order(a("a6"), aOrgA, aSellerA, aStoreA, aCust3, "2026-10-04T11:00:00Z", "paid", "CDF", 50000, "[]")                                                                 // a second currency
	order(a("b1"), aOrgB, aSellerB, aStoreB, aCust1, "2026-10-04T10:30:00Z", "paid", "USD", 1234, "[]")                                                                  // another business, same customer

	res := func(id, org, customer, listing, status, created, start string) {
		exec(t, db, `INSERT INTO reservations (id, organization_id, customer_id, listing_id, status, start_at, end_at, created_at)
			VALUES ($1, $2, $3, $4, $5, $6::timestamptz, $6::timestamptz + interval '1 day', $7::timestamptz)`, id, org, customer, listing, status, start, created)
	}
	res(a("c1"), aOrgA, aCust1, aL1, "confirmed", "2026-10-03T10:00:00Z", "2026-11-01T12:00:00Z")
	res(a("c2"), aOrgA, aCust2, aL1, "pending", "2026-10-04T09:00:00Z", "2026-11-05T12:00:00Z")
	res(a("c3"), aOrgA, aCust3, aL1, "completed", "2026-09-30T09:00:00Z", "2026-12-01T12:00:00Z")
	res(a("c4"), aOrgB, aCust1, aL3, "cancelled", "2026-10-04T09:30:00Z", "2026-11-01T12:00:00Z")

	return &analyticsFixture{db: db, now: time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC).Unix()}
}

func counts(series []domain.DayCount) []int {
	out := make([]int, len(series))
	for i, d := range series {
		out[i] = d.Count
	}
	return out
}

func equal(t *testing.T, name string, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestAnalytics_PlatformFiguresMatchTheData(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	a, err := postgres.NewAnalyticsRepository(f.db.DB).Platform(context.Background(), f.now, 3)
	if err != nil {
		t.Fatal(err)
	}

	equal(t, "window", []string{a.From, a.To}, []string{"2026-10-02", "2026-10-04"})
	equal(t, "days", []string{a.Orders[0].Date, a.Orders[1].Date, a.Orders[2].Date}, []string{"2026-10-02", "2026-10-03", "2026-10-04"})

	// Orders placed: the draft is not one; 23:30 on the 3rd and 00:30 on the 4th (Kinshasa) fall on different days.
	equal(t, "orders by day", counts(a.Orders), []int{0, 1, 4})
	equal(t, "orders current/previous", []int{a.Current.Orders, a.Previous.Orders}, []int{5, 1})

	// Revenue: earning orders only (the cancelled one is not), each currency on its own.
	equal(t, "revenue current", a.Current.Revenue, map[string]int64{"USD": 4734, "CDF": 50000})
	equal(t, "revenue previous", a.Previous.Revenue, map[string]int64{"USD": 400})
	equal(t, "revenue on the 3rd", a.Revenue[1].Amounts, map[string]int64{})
	equal(t, "revenue on the 4th", a.Revenue[2].Amounts, map[string]int64{"USD": 4734, "CDF": 50000})
	equal(t, "currencies: USD first, the rest alphabetical", a.RevenueCurrencies(), []string{"USD", "CDF"})

	equal(t, "orders by status", a.OrdersByStatus, map[string]int{"paid": 3, "completed": 1, "cancelled": 1})

	equal(t, "reservations by day", counts(a.Reservations), []int{0, 1, 2})
	equal(t, "reservations current/previous", []int{a.Current.Reservations, a.Previous.Reservations}, []int{3, 1})
	equal(t, "reservations by status", a.ReservationsByStatus, map[string]int{"confirmed": 1, "pending": 1, "cancelled": 1})

	equal(t, "new users by day", counts(a.Users), []int{0, 0, 2}) // c2 joined at 00:30 on the 4th in Kinshasa
	equal(t, "new users current/previous", []int{a.Current.NewUsers, a.Previous.NewUsers}, []int{2, 1})
	equal(t, "conversations", counts(a.Conversations), []int{0, 0, 0})

	equal(t, "new listings", []int{a.Current.NewListings, a.Previous.NewListings}, []int{1, 0})
	equal(t, "listings by status", a.ListingsByStatus, map[string]int{"published": 4, "draft": 1})
	equal(t, "published by vertical", a.ListingsByVertical, map[string]int{"RETAIL": 3, "HOTEL": 1})

	// Buyers: c1 (also bought before the window), c2, c3.
	equal(t, "customers current", []int{a.Current.Customers, a.Current.Returning}, []int{3, 1})
	equal(t, "customers previous", []int{a.Previous.Customers, a.Previous.Returning}, []int{1, 0})

	if len(a.TopOrganizations) != 2 || a.TopOrganizations[0].Name != "Gombe Stays" || a.TopOrganizations[0].Orders != 4 {
		t.Fatalf("top organizations = %+v", a.TopOrganizations)
	}
	equal(t, "top organization revenue", a.TopOrganizations[0].Revenue, map[string]int64{"USD": 3500, "CDF": 50000})
	equal(t, "second organization", []interface{}{a.TopOrganizations[1].Name, a.TopOrganizations[1].Orders, a.TopOrganizations[1].Revenue}, []interface{}{"Kasa Market", 1, map[string]int64{"USD": 1234}})
	if a.TopListings != nil {
		t.Error("top listings are an organization-level list")
	}
}

func TestAnalytics_OrganizationFiguresNeverIncludeAnotherBusiness(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	repo := postgres.NewAnalyticsRepository(f.db.DB)
	ctx := context.Background()

	a, err := repo.Organization(ctx, aOrgA, f.now, 3)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "A orders by day", counts(a.Orders), []int{0, 1, 3})
	equal(t, "A orders current/previous", []int{a.Current.Orders, a.Previous.Orders}, []int{4, 1})
	equal(t, "A revenue", a.Current.Revenue, map[string]int64{"USD": 3500, "CDF": 50000}) // not B's 1234
	equal(t, "A orders by status", a.OrdersByStatus, map[string]int{"paid": 2, "completed": 1, "cancelled": 1})
	equal(t, "A reservations", []int{a.Current.Reservations, a.Previous.Reservations}, []int{2, 1})
	equal(t, "A reservations by status", a.ReservationsByStatus, map[string]int{"confirmed": 1, "pending": 1})
	equal(t, "A listings by status", a.ListingsByStatus, map[string]int{"published": 1, "draft": 1})
	equal(t, "A published by vertical", a.ListingsByVertical, map[string]int{"RETAIL": 1})
	equal(t, "A new listings", a.Current.NewListings, 1)
	equal(t, "A customers", []int{a.Current.Customers, a.Current.Returning}, []int{3, 1})
	if a.Users != nil || a.Conversations != nil || a.UsersByRole != nil || a.TopOrganizations != nil {
		t.Error("platform-wide parts must not be in a business's answer")
	}

	// Best sellers count earning orders only: the cancelled order and the other period are out.
	if len(a.TopListings) != 2 {
		t.Fatalf("top listings = %+v", a.TopListings)
	}
	equal(t, "best seller", []interface{}{a.TopListings[0].ID, a.TopListings[0].Name, a.TopListings[0].Orders, a.TopListings[0].Units, a.TopListings[0].Revenue},
		[]interface{}{aL1, "Café", 2, 3, map[string]int64{"USD": 1500}})
	equal(t, "second best", []interface{}{a.TopListings[1].Name, a.TopListings[1].Units, a.TopListings[1].Revenue}, []interface{}{"Sofa", 1, map[string]int64{"USD": 2000}})

	b, err := repo.Organization(ctx, aOrgB, f.now, 3)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "B orders", []int{b.Current.Orders, b.Previous.Orders}, []int{1, 0})
	equal(t, "B revenue", b.Current.Revenue, map[string]int64{"USD": 1234})
	// c1 bought from A before, but never from B: not a returning customer of B.
	equal(t, "B customers", []int{b.Current.Customers, b.Current.Returning}, []int{1, 0})
	equal(t, "B listings by status", b.ListingsByStatus, map[string]int{"published": 3})
	if len(b.TopListings) != 0 {
		t.Errorf("B sold no itemised listing: %+v", b.TopListings)
	}
}

func TestAnalytics_ABusinessWithNoActivityGetsRealZeros(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	c, err := postgres.NewAnalyticsRepository(f.db.DB).Organization(context.Background(), aOrgC, f.now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Orders) != 7 || len(c.Revenue) != 7 || len(c.Reservations) != 7 {
		t.Fatalf("every day is present even when empty: %d / %d / %d", len(c.Orders), len(c.Revenue), len(c.Reservations))
	}
	equal(t, "orders", counts(c.Orders), make([]int, 7))
	if c.Current.Orders != 0 || len(c.Current.Revenue) != 0 || c.Current.Customers != 0 {
		t.Errorf("totals must be zero: %+v", c.Current)
	}
	if c.OrdersByStatus == nil || c.ListingsByStatus == nil || c.Current.Revenue == nil {
		t.Error("maps must be empty, not nil, so the API sends {} and not null")
	}
}

func TestAnalytics_WindowIsClamped(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	repo := postgres.NewAnalyticsRepository(f.db.DB)
	for days, want := range map[int]int{0: 1, -3: 1, 1: 1, 30: 30, 180: 180, 5000: 180} {
		a, err := repo.Platform(context.Background(), f.now, days)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Orders) != want || a.Days != want {
			t.Errorf("days=%d: %d points, Days=%d, want %d", days, len(a.Orders), a.Days, want)
		}
	}
	if _, err := repo.Organization(context.Background(), "", f.now, 7); err == nil {
		t.Error("an organization analytics request without an organization must fail")
	}
}

func TestSiteStats_AreComputedFromThePlatformsOwnData(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	stats, err := postgres.NewSiteContentRepository(f.db.DB).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := domain.SiteStats{
		domain.SiteMetricVerifiedSellers:   1, // only Gombe Stays is verified
		domain.SiteMetricLiveListings:      4, // L1, L3, L4, L5 (L2 is a draft)
		domain.SiteMetricDealsLive:         1, // only L1 is a real saving: L4's "compare at" is below its price, L5's promotion is not an object
		domain.SiteMetricCitiesCovered:     2, // Kinshasa (spelled two ways) and Lubumbashi; Goma has no published listing
		domain.SiteMetricBookingsConfirmed: 2, // one confirmed, one completed; the pending and the cancelled do not count
		domain.SiteMetricOrdersCompleted:   1, // the completed order; paid, cancelled and draft do not count
		domain.SiteMetricVerticalsLive:     2, // RETAIL and HOTEL
		domain.SiteMetricMerchants:         2, // Gombe Stays and Kasa Market sell; Quiet Shop has nothing published
		domain.SiteMetricTransactions:      3, // the completed order and the two confirmed or completed bookings
		domain.SiteMetricRegisteredUsers:   0, // the accounts of this fixture have no role: see the dedicated test below
		domain.SiteMetricConversations:     0, // nobody has talked to Trezutil in this fixture: see the dedicated test below
		domain.SiteMetricConnections:       3, // Cust2 with Gombe Stays (the completed order), Cust1 and Cust3 with Gombe Stays (the confirmed and the completed booking)
	}
	for _, m := range domain.SiteMetrics {
		if m == domain.SiteMetricPartners {
			continue // counted from the managed partner list, not from the database
		}
		if _, ok := stats[m]; !ok {
			t.Errorf("metric %q has no figure", m)
		}
	}
	equal(t, "stats", stats, want)
}

func TestSiteStats_AnEmptyPlatformIsAllZeros(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	stats, err := postgres.NewSiteContentRepository(db.DB).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range domain.SiteMetrics {
		if m == domain.SiteMetricPartners {
			continue // counted from the managed partner list, not from the database
		}
		if v, ok := stats[m]; !ok || v != 0 {
			t.Errorf("%q = %d (present=%v), want a real zero", m, v, ok)
		}
	}
}

// "Users" on the home page are the people who shop or sell, never the staff,
// never a suspended account, and a person with two roles counts once.
func TestSiteStats_UsersAreActiveAccountsThatShopOrSell(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	id := func(n string) string { return "5e11e700-0000-4000-8000-00000000" + n }
	person := func(n, phone string, status string, roles ...string) {
		sellerUser(t, db, ctx, id(n), phone)
		exec(t, db, `UPDATE users SET status = $2 WHERE id = $1`, id(n), status)
		for _, role := range roles {
			exec(t, db, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`, id(n), role)
		}
	}
	person("0a01", "+243810000201", "active", "customer")           // counted
	person("0a02", "+243810000202", "active", "customer", "seller") // both roles: counted once
	person("0a03", "+243810000203", "active", "seller")             // counted
	person("0a04", "+243810000204", "suspended", "customer")        // suspended: not counted
	person("0a05", "+243810000205", "active", "admin")              // staff only: not counted
	person("0a06", "+243810000206", "active", "operator", "staff")  // staff only: not counted
	person("0a07", "+243810000207", "active")                       // no role yet: not counted
	stats, err := postgres.NewSiteContentRepository(db.DB).Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	equal(t, "registered users", stats[domain.SiteMetricRegisteredUsers], 3)
}

// A merchant is a business that is active and has something published: an
// empty or suspended shop is not a merchant, and a business with several
// listings counts once.
func TestSiteStats_MerchantsAreActiveBusinessesWithSomethingPublished(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	equal(t, "merchants before", mustStats(t, f)[domain.SiteMetricMerchants], 2) // Gombe Stays (one live listing) and Kasa Market (three): each once

	exec(t, f.db, `UPDATE organizations SET status = 'suspended' WHERE id = $1`, aOrgB)
	equal(t, "merchants once Kasa Market is suspended", mustStats(t, f)[domain.SiteMetricMerchants], 1)

	exec(t, f.db, `UPDATE listings SET status = 'draft' WHERE id = $1`, aL1)
	equal(t, "merchants once Gombe Stays has nothing live", mustStats(t, f)[domain.SiteMetricMerchants], 0)
}

// A connection is a customer and a business that went through with something
// together. A pair counts once however many orders and bookings it holds, and
// nothing pending, cancelled or still a cart counts.
func TestSiteStats_ConnectionsAreCustomersAndBusinessesThatWentThroughWithSomething(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	equal(t, "connections before", mustStats(t, f)[domain.SiteMetricConnections], 3)

	order := func(id, org, seller, store, customer, status string) {
		exec(t, f.db, `INSERT INTO orders (id, organization_id, seller_id, customer_id, store_id, items, total_amount, currency, status, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, '[]'::jsonb, 100, 'USD', $6, now(), now())`, id, org, seller, customer, store, status)
	}
	a := func(n string) string { return "5e11e700-0000-4000-8000-00000000e1" + n }

	order(a("01"), aOrgA, aSellerA, aStoreA, aCust2, "completed") // Cust2 and Gombe Stays again: still one pair
	equal(t, "the same pair again", mustStats(t, f)[domain.SiteMetricConnections], 3)

	order(a("02"), aOrgB, aSellerB, aStoreB, aCust2, "paid") // paid is not yet done
	order(a("03"), aOrgB, aSellerB, aStoreB, aCust2, "cancelled")
	order(a("04"), aOrgB, aSellerB, aStoreB, aCust2, "draft")
	equal(t, "orders that did not go through", mustStats(t, f)[domain.SiteMetricConnections], 3)

	order(a("05"), aOrgB, aSellerB, aStoreB, aCust2, "fulfilled") // Cust2 now also went through with Kasa Market
	equal(t, "a second business for the same customer", mustStats(t, f)[domain.SiteMetricConnections], 4)

	// a booking and an order of the same pair are one connection
	exec(t, f.db, `INSERT INTO reservations (id, organization_id, customer_id, listing_id, status, start_at, end_at, created_at)
		VALUES ($1, $2, $3, $4, 'confirmed', now() + interval '10 days', now() + interval '11 days', now())`, a("06"), aOrgB, aCust2, aL3)
	equal(t, "an order and a booking of one pair", mustStats(t, f)[domain.SiteMetricConnections], 4)
}

// Conversations are the times someone started telling Trezutil what they need.
func TestSiteStats_ConversationsAreCountedFromTheConversationsTable(t *testing.T) {
	f := newAnalyticsFixture(t)
	defer f.db.Close()
	equal(t, "conversations before", mustStats(t, f)[domain.SiteMetricConversations], 0)
	for i, customer := range []string{aCust1, aCust1, aCust2} { // one person can start several
		exec(t, f.db, `INSERT INTO conversations (organization_id, customer_id, channel, external_id) VALUES ($1, $2, 'web', $3)`, aOrgA, customer, "web-"+strconv.Itoa(i))
	}
	equal(t, "conversations after three", mustStats(t, f)[domain.SiteMetricConversations], 3)
}

func mustStats(t *testing.T, f *analyticsFixture) domain.SiteStats {
	t.Helper()
	stats, err := postgres.NewSiteContentRepository(f.db.DB).Stats(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return stats
}

func TestSiteContent_SaveWithOptimisticConcurrency(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	sellerUser(t, db, ctx, aOwnerA, "+243810000101")
	repo := postgres.NewSiteContentRepository(db.DB)

	first, err := repo.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stored || first.Version != 0 || len(first.Partners) != len(domain.DefaultSiteContent().Partners) {
		t.Fatalf("nothing saved yet: the defaults come back unsaved: stored=%v version=%d partners=%d", first.Stored, first.Version, len(first.Partners))
	}

	edit := domain.DefaultSiteContent()
	edit.Partners = edit.Partners[:3]
	edit.Trust[0].MinValue = 25
	saved, err := repo.Save(ctx, &edit, 0, aOwnerA)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Version != 1 || !saved.Stored || saved.UpdatedBy != aOwnerA {
		t.Errorf("first save: %+v", saved)
	}

	// A stale editor still names version 0: it loses, and nothing changes.
	if _, err := repo.Save(ctx, &edit, 0, aOwnerA); !errors.Is(err, domain.ErrSiteContentConflict) {
		t.Errorf("a stale save must conflict, got %v", err)
	}

	edit.Partners = edit.Partners[:1]
	if saved, err = repo.Save(ctx, &edit, 1, "system"); err != nil || saved.Version != 2 { // a non-user actor is stored as NULL, not rejected
		t.Fatalf("second save: %+v %v", saved, err)
	}

	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 || !got.Stored || len(got.Partners) != 1 || got.Trust[0].MinValue != 25 || got.UpdatedBy != "" {
		t.Errorf("round trip: version=%d partners=%d min=%d by=%q", got.Version, len(got.Partners), got.Trust[0].MinValue, got.UpdatedBy)
	}
	if got.UpdatedAt == 0 {
		t.Error("the time of the last change is recorded")
	}
}

func TestSiteContent_TwoEditorsSavingTogetherCannotBothWin(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := postgres.NewSiteContentRepository(db.DB)
	base := domain.DefaultSiteContent()
	if _, err := repo.Save(ctx, &base, 0, ""); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := domain.DefaultSiteContent()
			_, err := repo.Save(ctx, &c, 1, "")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	wins, conflicts := 0, 0
	for err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, domain.ErrSiteContentConflict):
			conflicts++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if wins != 1 || conflicts != 7 {
		t.Errorf("exactly one of eight simultaneous saves of version 1 may win: wins=%d conflicts=%d", wins, conflicts)
	}
}

func TestSiteContent_AnOlderSavedDocumentIsCompletedOnRead(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	exec(t, db, `INSERT INTO site_settings (key, value, version) VALUES ('home', '{"trust":[{"metric":"live_listings","enabled":true,"min_value":3}],"partners":[],"sections":[{"key":"deals","visible":true},{"key":"nearby","visible":false}]}', 4)`)
	got, err := postgres.NewSiteContentRepository(db.DB).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 4 || len(got.Sections) != len(domain.HomeSections) || len(got.Trust) != len(domain.SiteMetrics) {
		t.Fatalf("version=%d sections=%d trust=%d", got.Version, len(got.Sections), len(got.Trust))
	}
	// "deals" was removed from the product after this document was saved: it is dropped, and the editor's other choice survives.
	if got.Sections[0].Key != domain.SectionNearby || got.Sections[0].Visible {
		t.Errorf("the editor's choice survives: %+v", got.Sections[0])
	}
	enabled := 0
	for _, it := range got.Trust {
		if it.Enabled {
			enabled++
		}
	}
	if enabled != 1 {
		t.Errorf("metrics added after the document was saved must stay off the site, %d enabled", enabled)
	}
	if err := got.Validate(); err != nil {
		t.Errorf("a completed document is valid: %v", err)
	}
}

func TestSiteAssets_AreStoredOnceAndReadBackExactly(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	repo := postgres.NewSiteContentRepository(db.DB)

	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1, 2, 3}, 50)...)
	id1, err := repo.SaveAsset(ctx, "image/png", png, "")
	if err != nil {
		t.Fatal(err)
	}
	id2, err := repo.SaveAsset(ctx, "image/png", png, "")
	if err != nil || id2 != id1 {
		t.Errorf("the same bytes are stored once: %q vs %q (%v)", id1, id2, err)
	}
	other, err := repo.SaveAsset(ctx, "image/png", append(png, 9), "")
	if err != nil || other == id1 {
		t.Errorf("different bytes get their own id: %q (%v)", other, err)
	}
	contentType, data, err := repo.Asset(ctx, id1)
	if err != nil || contentType != "image/png" || !bytes.Equal(data, png) {
		t.Errorf("read back: %q, %d bytes, %v", contentType, len(data), err)
	}

	for _, id := range []string{"5e11e700-0000-4000-8000-0000000000ee", "not-a-uuid", "", "1; DROP TABLE site_assets"} {
		if _, _, err := repo.Asset(ctx, id); !errors.Is(err, domain.ErrSiteAssetNotFound) {
			t.Errorf("asset %q: %v, want not found", id, err)
		}
	}
	// The database itself refuses what the application should already have refused.
	if _, err := repo.SaveAsset(ctx, "image/svg+xml", []byte("<svg/>"), ""); err == nil {
		t.Error("an SVG must never be stored")
	}
	if _, err := repo.SaveAsset(ctx, "image/png", make([]byte, domain.SiteAssetMaxBytes+1), ""); err == nil {
		t.Error("an oversized file must never be stored")
	}
	if _, err := repo.SaveAsset(ctx, "image/png", nil, ""); err == nil {
		t.Error("an empty file must never be stored")
	}
}
