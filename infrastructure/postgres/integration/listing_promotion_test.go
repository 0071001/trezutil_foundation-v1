package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

// A sale on real PostgreSQL: the selling price is the sale price (so orders
// charge what the page shows), the regular price is remembered, only the
// owner can run it, and no client-supplied metadata can forge one.
func TestListingPromotion_LifecycleAndIsolation(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const (
		userA, userB = "5e11e700-0000-4000-8000-0000000000c1", "5e11e700-0000-4000-8000-0000000000d1"
		orgA, orgB   = "5e11e700-0000-4000-8000-00000000001a", "5e11e700-0000-4000-8000-00000000001b"
		sellerA      = "5e11e700-0000-4000-8000-0000000005c1"
		sellerB      = "5e11e700-0000-4000-8000-0000000005d1"
		storeA       = "5e11e700-0000-4000-8000-0000000007c1"
		category     = "5e11e700-0000-4000-8000-00000000c002"
		listingA     = "5e11e700-0000-4000-8000-0000000001c1"
	)
	sellerUser(t, db, ctx, userA, "+243810000011")
	sellerUser(t, db, ctx, userB, "+243810000012")
	createTestOrganization(t, db, ctx, orgA)
	createTestOrganization(t, db, ctx, orgB)
	orgs := postgres.NewOrganizationRepository(db.DB)
	for _, m := range []*domain.OrganizationMember{
		domain.NewOrganizationMember(userA, orgA, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(userB, orgB, domain.OrganizationRoleOwner),
	} {
		if err := orgs.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	createTestSeller(t, db, ctx, sellerA, userA, orgA)
	createTestSeller(t, db, ctx, sellerB, userB, orgB)
	createTestStore(t, db, ctx, storeA, orgA)
	createTestCategory(t, db, ctx, category)
	createTestListing(t, db, ctx, listingA, orgA, sellerA, storeA, category)

	listingRepo := postgres.NewListingRepository(db.DB)
	uc := application.NewListingUseCases(
		listingRepo, postgres.NewCategoryRepository(db.DB),
		postgres.NewAttributeDefinitionRepository(db.DB), postgres.NewAttributeValueRepository(db.DB),
		postgres.NewListingMediaRepository(db.DB), postgres.NewListingAvailabilityRepository(db.DB),
		postgres.NewLocationRepository(db.DB), postgres.NewListingLocationRepository(db.DB),
		orgs, postgres.NewSellerRepository(db.DB), postgres.NewStoreRepository(db.DB), postgres.NewUserRepository(db.DB), nil,
	)
	reload := func() *domain.Listing {
		t.Helper()
		l, err := listingRepo.FindByID(ctx, listingA)
		if err != nil {
			t.Fatal(err)
		}
		return l
	}

	regular := 200.0
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{PriceAmount: &regular}); err != nil {
		t.Fatal(err)
	}

	// Access control: another organization and anonymous callers cannot start a sale.
	if _, err := uc.SetListingPromotion(ctx, userB, listingA, 150); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("cross-organization promotion: %v", err)
	}
	if _, err := uc.SetListingPromotion(ctx, "", listingA, 150); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("anonymous promotion: %v", err)
	}
	if _, err := uc.SetListingPromotion(ctx, userA, "5e11e700-0000-4000-8000-00000000dead", 150); !errors.Is(err, domain.ErrListingNotFound) {
		t.Fatalf("unknown listing: %v", err)
	}

	// Invalid sale prices change nothing.
	for _, bad := range []float64{0, -1, 200, 250} {
		if _, err := uc.SetListingPromotion(ctx, userA, listingA, bad); !errors.Is(err, domain.ErrInvalidParameter) {
			t.Fatalf("sale price %v: %v", bad, err)
		}
	}
	if got := reload(); got.PriceAmount != 200 || got.Promotion() != nil {
		t.Fatalf("rejected sale altered the listing: price %v promotion %+v", got.PriceAmount, got.Promotion())
	}

	// A sale on a live listing needs no pause, and persists through the JSONB column.
	if err := uc.PublishListing(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.SetListingPromotion(ctx, userA, listingA, 150); err != nil {
		t.Fatalf("starting a sale on a live listing: %v", err)
	}
	got := reload()
	p := got.Promotion()
	if got.Status != domain.ListingStatusPublished || got.PriceAmount != 150 || p == nil || p.CompareAtAmount != 200 || p.DiscountPercent != 25 {
		t.Fatalf("after start: status %s price %v promotion %+v", got.Status, got.PriceAmount, p)
	}

	// Changing the sale price keeps the original regular price.
	if _, err := uc.SetListingPromotion(ctx, userA, listingA, 100); err != nil {
		t.Fatal(err)
	}
	if p := reload().Promotion(); p == nil || p.CompareAtAmount != 200 || p.DiscountPercent != 50 {
		t.Fatalf("after re-pricing the sale: %+v", p)
	}

	// Seller metadata edits (on a paused listing) can neither forge nor erase the sale.
	if err := uc.PauseListing(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	forged := map[string]interface{}{"colour": "red", domain.PromotionMetadataKey: map[string]interface{}{"compare_at_amount": 1e9}}
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{Metadata: forged}); err != nil {
		t.Fatal(err)
	}
	got = reload()
	if p := got.Promotion(); p == nil || p.CompareAtAmount != 200 {
		t.Fatalf("a forged metadata promotion replaced the real one: %+v", got.Metadata)
	}
	if got.Metadata["colour"] != "red" {
		t.Fatalf("ordinary metadata must still be saved: %+v", got.Metadata)
	}
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{Metadata: map[string]interface{}{"colour": "blue"}}); err != nil {
		t.Fatal(err)
	}
	if p := reload().Promotion(); p == nil || p.CompareAtAmount != 200 {
		t.Fatal("replacing metadata must not erase the sale")
	}

	// A price edit at or above the regular price ends the sale; below it, it is the new sale price.
	below := 120.0
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{PriceAmount: &below}); err != nil {
		t.Fatal(err)
	}
	if got := reload(); got.PriceAmount != 120 || got.Promotion() == nil || got.Promotion().DiscountPercent != 40 {
		t.Fatalf("repricing below the regular price: %v %+v", got.PriceAmount, got.Promotion())
	}
	above := 220.0
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{PriceAmount: &above}); err != nil {
		t.Fatal(err)
	}
	if got := reload(); got.PriceAmount != 220 || got.Promotion() != nil {
		t.Fatalf("repricing above the regular price must end the sale: %v %+v", got.PriceAmount, got.Metadata)
	}

	// Ending a sale restores the regular price; ending when none runs is a harmless no-op.
	if _, err := uc.SetListingPromotion(ctx, userA, listingA, 110); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.EndListingPromotion(ctx, userB, listingA); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("cross-organization end: %v", err)
	}
	if _, err := uc.EndListingPromotion(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	if got := reload(); got.PriceAmount != 220 || got.Promotion() != nil {
		t.Fatalf("after end: price %v promotion %+v", got.PriceAmount, got.Promotion())
	}
	if _, err := uc.EndListingPromotion(ctx, userA, listingA); err != nil {
		t.Fatalf("ending a sale that is not running: %v", err)
	}
	if got := reload(); got.PriceAmount != 220 {
		t.Fatalf("a no-op end changed the price to %v", got.PriceAmount)
	}

	// Closed listings are final.
	if err := uc.PublishListing(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	closed := reload()
	closed.MarkAsClosed()
	if err := listingRepo.Update(ctx, closed); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.SetListingPromotion(ctx, userA, listingA, 100); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("sale on a closed listing: %v", err)
	}
}
