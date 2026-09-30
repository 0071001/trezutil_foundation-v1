package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/availability"
	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/application/reservations"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

// The Hotel Commerce Pack on real PostgreSQL: room types are HOTEL listings,
// rooms are ROOM inventory, nights are capacity units, stays are core
// reservations and stay payments are core orders.

type hotelFixture struct {
	svc      *hotel.Service
	listings *application.ListingUseCases
	bookings *reservations.ReservationUseCasesImpl
	db       *TestDB
}

const (
	hOwnerA, hStaffA, hOwnerB, hGuest, hGuest2 = "40fe1000-0000-4000-8000-0000000000a1", "40fe1000-0000-4000-8000-0000000000a2", "40fe1000-0000-4000-8000-0000000000b1", "40fe1000-0000-4000-8000-0000000000c1", "40fe1000-0000-4000-8000-0000000000c2"
	hOrgA, hOrgB                               = "40fe1000-0000-4000-8000-00000000000a", "40fe1000-0000-4000-8000-00000000000b"
	hSellerA, hSellerB                         = "40fe1000-0000-4000-8000-0000000005a1", "40fe1000-0000-4000-8000-0000000005b1"
	hStoreA, hStoreB                           = "40fe1000-0000-4000-8000-0000000007a1", "40fe1000-0000-4000-8000-0000000007b1"
	hCategory                                  = "40fe1000-0000-4000-8000-00000000c001"
)

func newHotelFixture(t *testing.T) *hotelFixture {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	db.Cleanup(t)
	ctx := context.Background()
	for i, id := range []string{hOwnerA, hStaffA, hOwnerB, hGuest, hGuest2} {
		sellerUser(t, db, ctx, id, "+24381000010"+string(rune('0'+i)))
	}
	createTestOrganization(t, db, ctx, hOrgA)
	createTestOrganization(t, db, ctx, hOrgB)
	orgs := postgres.NewOrganizationRepository(db.DB)
	for _, m := range []*domain.OrganizationMember{
		domain.NewOrganizationMember(hOwnerA, hOrgA, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(hStaffA, hOrgA, domain.OrganizationRoleStaff),
		domain.NewOrganizationMember(hOwnerB, hOrgB, domain.OrganizationRoleOwner),
	} {
		if err := orgs.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	createTestSeller(t, db, ctx, hSellerA, hOwnerA, hOrgA)
	createTestSeller(t, db, ctx, hSellerB, hOwnerB, hOrgB)
	stores := postgres.NewStoreRepository(db.DB)
	for _, st := range []struct{ id, org, slug string }{{hStoreA, hOrgA, "hotel-a"}, {hStoreB, hOrgB, "hotel-b"}} {
		s := domain.NewStore(st.org, "Hotel "+st.slug, st.slug, "CD", "+243810000100")
		s.ID = st.id
		city, address := "Kinshasa", "Avenue du Fleuve, Gombe"
		s.City, s.Address = &city, &address
		if err := stores.Create(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	createTestCategory(t, db, ctx, hCategory)

	listingRepo := postgres.NewListingRepository(db.DB)
	users := postgres.NewUserRepository(db.DB)
	sellers := postgres.NewSellerRepository(db.DB)
	media := postgres.NewListingMediaRepository(db.DB)
	inventory := postgres.NewInventoryRepository(db.DB)
	capacity := postgres.NewCapacityRepository(db.DB)
	reservationRepo := postgres.NewReservationRepository(db.DB)
	fulfillment := postgres.NewFulfillmentRepository(db.DB)
	listings := application.NewListingUseCases(
		listingRepo, postgres.NewCategoryRepository(db.DB),
		postgres.NewAttributeDefinitionRepository(db.DB), postgres.NewAttributeValueRepository(db.DB),
		media, postgres.NewListingAvailabilityRepository(db.DB),
		postgres.NewLocationRepository(db.DB), postgres.NewListingLocationRepository(db.DB),
		orgs, sellers, stores, users, nil,
	).WithVerticals(vertical.DefaultCatalog())
	provider := availability.NewCapacityAvailabilityProvider(inventory, capacity).WithReservations(reservationRepo)
	orders := application.NewOrderUseCases(postgres.NewOrderRepository(db.DB), listingRepo, sellers, orgs, users, inventory, fulfillment, nil)
	svc := hotel.NewService(hotel.Config{
		Listings: listings, ListingRepo: listingRepo, Media: media, RoomTypes: postgres.NewHotelRoomTypeRepository(db.DB),
		Inventory: inventory, Capacity: capacity, Availability: provider, Reservations: reservationRepo, Orders: orders,
		Stores: stores, Orgs: orgs, Users: users,
	})
	bookings := reservations.NewReservationUseCases(reservationRepo, listingRepo, sellers, users, orgs, provider, capacity, inventory, fulfillment, nil,
		reservations.ConfirmationPolicy{AutoConfirm: true}).WithRules(svc)
	return &hotelFixture{svc: svc, listings: listings, bookings: bookings, db: db}
}

func night(base time.Time, i int) string { return base.AddDate(0, 0, i).Format("2006-01-02") }

func TestHotelPack_SellerSetupAndCustomerStay(t *testing.T) {
	f := newHotelFixture(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 30)

	// Seller: room type → rooms → nightly capacity → publish.
	deluxe, err := f.svc.CreateRoomType(ctx, hOwnerA, hotel.NewRoomType{
		StoreID: hStoreA, SellerID: hSellerA, CategoryID: hCategory, Title: "Deluxe room", Description: "River view",
		PriceAmount: 90, PriceCurrency: "USD", MaxOccupancy: 2, BedConfiguration: "1 king bed",
		Amenities: []string{"Wi-Fi", "Air conditioning", "wi-fi"}, Units: 3, ImageURLs: []string{"https://images.example.com/deluxe.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if deluxe.Units != 3 || deluxe.MaxOccupancy != 2 || len(deluxe.Amenities) != 2 || deluxe.Status != "draft" || len(deluxe.Images) != 1 {
		t.Fatalf("room type = %+v", deluxe)
	}
	listing, _ := postgres.NewListingRepository(f.db.DB).FindByID(ctx, deluxe.ListingID)
	if listing.Vertical != domain.VerticalHotel {
		t.Fatalf("room type listing vertical = %s", listing.Vertical)
	}
	if _, err := f.svc.SetNightlyCapacity(ctx, hOwnerA, deluxe.ListingID, night(base, 0), night(base, 5), 3); err != nil {
		t.Fatal(err)
	}
	nights, err := f.svc.SetNightlyCapacity(ctx, hOwnerA, deluxe.ListingID, night(base, 1), night(base, 2), 2) // one room under repair
	if err != nil || len(nights) != 1 || nights[0].Capacity != 2 {
		t.Fatalf("override = %+v %v", nights, err)
	}
	if err := f.listings.PublishListing(ctx, hOwnerA, deluxe.ListingID); err != nil {
		t.Fatal(err)
	}

	// Customer: property → availability for 2 nights, 2 guests → the tightest night decides.
	property, err := f.svc.Property(ctx, hStoreA)
	if err != nil || len(property.RoomTypes) != 1 || property.City != "Kinshasa" {
		t.Fatalf("property = %+v %v", property, err)
	}
	stay, _ := domain.ParseStay(night(base, 0), night(base, 2))
	quote, err := f.svc.Availability(ctx, hStoreA, stay, 2, 0)
	if err != nil || len(quote.Offers) != 1 {
		t.Fatalf("quote = %+v %v", quote, err)
	}
	offer := quote.Offers[0]
	if !offer.Available || offer.AvailableRooms != 2 || offer.Rooms != 1 || offer.TotalPrice != 180 || quote.Nights != 2 {
		t.Fatalf("offer = %+v", offer)
	}

	// Stays draw from nightly capacity; several stays share a night.
	book := func(guest string, from, to, rooms, guests int, key string) (*domain.Reservation, error) {
		s, _ := domain.ParseStay(night(base, from), night(base, to))
		return f.bookings.CreateReservation(ctx, guest, deluxe.ListingID, s.StartAt(), s.EndAt(), rooms, guests, key)
	}
	first, err := book(hGuest, 0, 2, 2, 4, "stay-1") // 2 rooms, nights 0 and 1
	if err != nil {
		t.Fatal(err)
	}
	if !first.CapacityManaged {
		t.Fatal("a hotel stay is capacity-managed")
	}
	if _, err := book(hGuest2, 0, 1, 1, 1, "stay-2"); err != nil { // night 0 still has a room
		t.Fatalf("overlapping stay on a night with rooms left: %v", err)
	}
	if _, err := book(hGuest2, 1, 2, 1, 1, "stay-3"); !errors.Is(err, domain.ErrAvailabilityUnavailable) { // night 1: 2 rooms, both taken
		t.Fatalf("full night: %v", err)
	}
	if _, err := book(hGuest2, 3, 4, 1, 3, "stay-4"); !errors.Is(err, domain.ErrInvalidParameter) {
		t.Fatalf("3 guests in one double room: %v", err)
	}
	if _, err := book(hGuest2, 6, 7, 1, 1, "stay-5"); !errors.Is(err, domain.ErrAvailabilityUnavailable) {
		t.Fatalf("a night that is not open: %v", err)
	}
	s, _ := domain.ParseStay(night(base, 3), night(base, 4))
	if _, err := f.bookings.CreateReservation(ctx, hGuest2, deluxe.ListingID, s.StartAt()+14*3600, s.EndAt()+12*3600, 1, 1, "stay-6"); !errors.Is(err, domain.ErrInvalidParameter) {
		t.Fatalf("a stay must be whole nights: %v", err)
	}

	// The seller sees nights booked, cannot drop a night below its bookings nor units below a night.
	cal, err := f.svc.NightlyCapacity(ctx, hStaffA, deluxe.ListingID, night(base, 0), night(base, 3))
	if err != nil || cal[0].Reserved != 3 || cal[0].Available != 0 || cal[1].Reserved != 2 || cal[2].Reserved != 0 {
		t.Fatalf("calendar = %+v %v", cal, err)
	}
	if _, err := f.svc.SetNightlyCapacity(ctx, hOwnerA, deluxe.ListingID, night(base, 0), night(base, 1), 1); !errors.Is(err, domain.ErrCapacityConflict) {
		t.Fatalf("below bookings: %v", err)
	}
	if _, err := f.svc.SetUnits(ctx, hOwnerA, deluxe.ListingID, 2); !errors.Is(err, domain.ErrCapacityConflict) {
		t.Fatalf("units below a night: %v", err)
	}

	// Cancelling gives the rooms back.
	if err := f.bookings.CancelReservation(ctx, hGuest, first.ID); err != nil {
		t.Fatal(err)
	}
	cal, _ = f.svc.NightlyCapacity(ctx, hOwnerA, deluxe.ListingID, night(base, 0), night(base, 2))
	if cal[0].Reserved != 1 || cal[1].Reserved != 0 {
		t.Fatalf("after cancel = %+v", cal)
	}

	// Payment: the stay order is nights × rooms at the nightly rate, once, for the guest only.
	stayRes, err := book(hGuest, 3, 5, 1, 2, "stay-7")
	if err != nil {
		t.Fatal(err)
	}
	order, err := f.svc.CheckoutStay(ctx, hGuest, stayRes.ID)
	if err != nil || order.TotalAmount.AmountMinor != 18000 || order.OrganizationID != hOrgA {
		t.Fatalf("stay order = %+v %v", order, err)
	}
	again, err := f.svc.CheckoutStay(ctx, hGuest, stayRes.ID)
	if err != nil || again.ID != order.ID {
		t.Fatalf("checkout must be idempotent: %v %v", again, err)
	}
	if _, err := f.svc.CheckoutStay(ctx, hGuest2, stayRes.ID); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("another customer paying my stay: %v", err)
	}
}

func TestHotelPack_ConcurrentBookingsNeverOverbook(t *testing.T) {
	f := newHotelFixture(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 40)
	rt, err := f.svc.CreateRoomType(ctx, hOwnerA, hotel.NewRoomType{StoreID: hStoreA, SellerID: hSellerA, CategoryID: hCategory,
		Title: "Standard room", PriceAmount: 50, MaxOccupancy: 2, Units: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.SetNightlyCapacity(ctx, hOwnerA, rt.ListingID, night(base, 0), night(base, 2), 3); err != nil {
		t.Fatal(err)
	}
	if err := f.listings.PublishListing(ctx, hOwnerA, rt.ListingID); err != nil {
		t.Fatal(err)
	}
	stay, _ := domain.ParseStay(night(base, 0), night(base, 2))
	var wg sync.WaitGroup
	var mu sync.Mutex
	won := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			guest := hGuest
			if i%2 == 1 {
				guest = hGuest2
			}
			if _, err := f.bookings.CreateReservation(ctx, guest, rt.ListingID, stay.StartAt(), stay.EndAt(), 1, 1, "race-"+string(rune('a'+i))); err == nil {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	cal, _ := f.svc.NightlyCapacity(ctx, hOwnerA, rt.ListingID, night(base, 0), night(base, 2))
	if won != 3 || cal[0].Reserved != 3 || cal[1].Reserved != 3 {
		t.Fatalf("8 guests raced for 3 rooms: %d won, nights %+v", won, cal)
	}
	var active int
	_ = f.db.DB.QueryRowContext(ctx, `SELECT count(*) FROM reservations WHERE listing_id = $1 AND status <> 'cancelled'`, rt.ListingID).Scan(&active)
	if active != 3 {
		t.Fatalf("active reservations = %d", active)
	}
}

func TestHotelPack_OwnershipBoundaries(t *testing.T) {
	f := newHotelFixture(t)
	ctx := context.Background()
	base := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 50)
	rt, err := f.svc.CreateRoomType(ctx, hOwnerA, hotel.NewRoomType{StoreID: hStoreA, SellerID: hSellerA, CategoryID: hCategory,
		Title: "Suite", PriceAmount: 150, MaxOccupancy: 4, Units: 2})
	if err != nil {
		t.Fatal(err)
	}
	forbidden := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Errorf("%s: %v", what, err)
		}
	}
	// Another hotel's owner, a customer and the hotel's own staff cannot change A's rooms.
	for _, actor := range []string{hOwnerB, hGuest, hStaffA} {
		_, err := f.svc.SetNightlyCapacity(ctx, actor, rt.ListingID, night(base, 0), night(base, 1), 2)
		forbidden("capacity by "+actor, err)
		_, err = f.svc.SetUnits(ctx, actor, rt.ListingID, 1)
		forbidden("units by "+actor, err)
		occ := 1
		_, err = f.svc.UpdateRoomType(ctx, actor, rt.ListingID, hotel.RoomTypePatch{MaxOccupancy: &occ})
		forbidden("details by "+actor, err)
	}
	// Outsiders cannot read A's calendar or room list; staff can.
	for _, actor := range []string{hOwnerB, hGuest} {
		_, err := f.svc.NightlyCapacity(ctx, actor, rt.ListingID, night(base, 0), night(base, 1))
		forbidden("calendar by "+actor, err)
		_, err = f.svc.RoomTypes(ctx, actor, hOrgA)
		forbidden("room list by "+actor, err)
	}
	if _, err := f.svc.NightlyCapacity(ctx, hStaffA, rt.ListingID, night(base, 0), night(base, 1)); err != nil {
		t.Errorf("staff reads the calendar: %v", err)
	}
	// B cannot create a room type in A's store (the core listing rule).
	_, err = f.svc.CreateRoomType(ctx, hOwnerB, hotel.NewRoomType{StoreID: hStoreA, SellerID: hSellerB, CategoryID: hCategory,
		Title: "Hijack", PriceAmount: 10, MaxOccupancy: 1, Units: 1})
	forbidden("room type in another hotel's store", err)
	// A draft room type is not public, and a non-hotel listing ID is not a room type.
	if _, err := f.svc.Property(ctx, hStoreA); !errors.Is(err, hotel.ErrNoHotel) {
		t.Errorf("drafts are not public: %v", err)
	}
	if _, err := f.svc.NightlyCapacity(ctx, hOwnerA, "40fe1000-0000-4000-8000-0000deadbeef", night(base, 0), night(base, 1)); !errors.Is(err, hotel.ErrNotRoomType) {
		t.Errorf("unknown listing: %v", err)
	}
}
