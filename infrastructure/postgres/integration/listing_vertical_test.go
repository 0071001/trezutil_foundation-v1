package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

// Listing verticals on real PostgreSQL (migration 000019): a listing keeps the
// vertical it was created with, clients that send none get the listing type's
// default, and rows written without a vertical read back as that default.

func TestListingVertical_CreateResolveAndPersist(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const (
		owner    = "7e47ca10-0000-4000-8000-0000000000a1"
		org      = "7e47ca10-0000-4000-8000-00000000000a"
		seller   = "7e47ca10-0000-4000-8000-0000000005a1"
		store    = "7e47ca10-0000-4000-8000-0000000007a1"
		category = "7e47ca10-0000-4000-8000-00000000c001"
		legacy   = "7e47ca10-0000-4000-8000-0000000001ff"
	)
	sellerUser(t, db, ctx, owner, "+243810000011")
	createTestOrganization(t, db, ctx, org)
	orgs := postgres.NewOrganizationRepository(db.DB)
	if err := orgs.AddMember(ctx, domain.NewOrganizationMember(owner, org, domain.OrganizationRoleOwner)); err != nil {
		t.Fatal(err)
	}
	createTestSeller(t, db, ctx, seller, owner, org)
	createTestStore(t, db, ctx, store, org)
	createTestCategory(t, db, ctx, category)

	listingRepo := postgres.NewListingRepository(db.DB)
	uc := application.NewListingUseCases(
		listingRepo, postgres.NewCategoryRepository(db.DB),
		postgres.NewAttributeDefinitionRepository(db.DB), postgres.NewAttributeValueRepository(db.DB),
		postgres.NewListingMediaRepository(db.DB), postgres.NewListingAvailabilityRepository(db.DB),
		postgres.NewLocationRepository(db.DB), postgres.NewListingLocationRepository(db.DB),
		orgs, postgres.NewSellerRepository(db.DB), postgres.NewStoreRepository(db.DB), postgres.NewUserRepository(db.DB), nil,
	).WithVerticals(vertical.DefaultCatalog())

	create := func(listingType domain.ListingType, requested domain.Vertical) (*domain.Listing, error) {
		l := domain.NewListing(org, seller, store, category, "Offer", string(listingType), string(domain.PricingModelFixed), "USD", 50)
		l.Vertical = requested // "" is what a client that predates verticals sends
		return l, uc.CreateListing(ctx, owner, l, nil, nil, nil, nil)
	}
	storedVertical := func(id string) string {
		var v string
		if err := db.DB.QueryRowContext(ctx, `SELECT COALESCE(vertical, '') FROM listings WHERE id = $1`, id).Scan(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	// A client that sends no vertical gets the listing type's default.
	apartment, err := create(domain.ListingTypeRealEstate, "")
	if err != nil {
		t.Fatal(err)
	}
	if apartment.Vertical != domain.VerticalRealEstate || storedVertical(apartment.ID) != "REAL_ESTATE" {
		t.Fatalf("default vertical: got %q, stored %q", apartment.Vertical, storedVertical(apartment.ID))
	}

	// One seller operates several verticals: rooms (HOTEL) and a menu (RESTAURANT).
	room, err := create(domain.ListingTypeRealEstate, "hotel")
	if err != nil {
		t.Fatal(err)
	}
	dish, err := create(domain.ListingTypeFood, domain.VerticalRestaurant)
	if err != nil {
		t.Fatal(err)
	}
	read, err := listingRepo.FindByID(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	if room.Vertical != domain.VerticalHotel || read.Vertical != domain.VerticalHotel || dish.Vertical != domain.VerticalRestaurant {
		t.Fatalf("explicit verticals: room %q (read %q), dish %q", room.Vertical, read.Vertical, dish.Vertical)
	}

	// Unknown verticals and incompatible listing types are refused before anything is written.
	before, _ := listingRepo.CountByOrganizationID(ctx, org)
	if _, err := create(domain.ListingTypeRealEstate, "SPACESHIPS"); !errors.Is(err, domain.ErrInvalidParameter) {
		t.Fatalf("unknown vertical: %v", err)
	}
	if _, err := create(domain.ListingTypeRealEstate, domain.VerticalRetail); !errors.Is(err, domain.ErrInvalidParameter) {
		t.Fatalf("RETAIL real_estate listing: %v", err)
	}
	if after, _ := listingRepo.CountByOrganizationID(ctx, org); after != before {
		t.Fatalf("refused listings were written: %d -> %d", before, after)
	}

	// A row written by a binary that predates the column reads back as its
	// listing type's default.
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO listings (id, organization_id, seller_id, store_id, category_id, title, listing_type, pricing_model, price_amount, price_currency, status)
		VALUES ($1, $2, $3, $4, $5, 'Plumbing', 'service', 'fixed', 20, 'USD', 'draft')`, legacy, org, seller, store, category); err != nil {
		t.Fatal(err)
	}
	if got, err := listingRepo.FindByID(ctx, legacy); err != nil || got.Vertical != domain.VerticalServices {
		t.Fatalf("legacy row vertical: %v %v", got, err)
	}
	all, err := listingRepo.FindByOrganizationID(ctx, org, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[domain.Vertical]int{}
	for _, l := range all {
		seen[l.Vertical]++
	}
	if len(all) != 4 || seen[domain.VerticalHotel] != 1 || seen[domain.VerticalRestaurant] != 1 ||
		seen[domain.VerticalRealEstate] != 1 || seen[domain.VerticalServices] != 1 {
		t.Fatalf("organization verticals = %v", seen)
	}

	// Re-applying the migration is safe and backfills rows without a vertical,
	// leaving explicit verticals alone.
	content, err := os.ReadFile(filepath.Join(migrationsDir, "000019_listing_verticals.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.ExecContext(ctx, extractUpSection(string(content))); err != nil {
		t.Fatalf("re-applying 000019: %v", err)
	}
	if storedVertical(legacy) != "SERVICES" || storedVertical(room.ID) != "HOTEL" {
		t.Fatalf("backfill: legacy %q, room %q", storedVertical(legacy), storedVertical(room.ID))
	}

	// The database refuses malformed codes; which codes exist is the catalogue's job.
	if _, err := db.DB.ExecContext(ctx, `UPDATE listings SET vertical = 'hotel' WHERE id = $1`, room.ID); err == nil {
		t.Fatal("malformed vertical code accepted by the database")
	}
}
