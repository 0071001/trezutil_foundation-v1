package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

// Seller console isolation on real PostgreSQL: a seller can only edit and
// list their own organization's data, whatever IDs the client sends.

func sellerUser(t *testing.T, db *TestDB, ctx context.Context, id, phone string) {
	t.Helper()
	if err := postgres.NewUserRepository(db.DB).Create(ctx, &domain.User{
		ID: id, Phone: phone, DisplayName: "Seller " + id[len(id)-2:], Status: domain.UserStatusActive,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(), Roles: []domain.Role{},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSellerConsole_ListingIsolation(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const (
		userA, userB, staffA = "5e11e700-0000-4000-8000-0000000000a1", "5e11e700-0000-4000-8000-0000000000b1", "5e11e700-0000-4000-8000-0000000000a2"
		orgA, orgB           = "5e11e700-0000-4000-8000-00000000000a", "5e11e700-0000-4000-8000-00000000000b"
		sellerA, sellerB     = "5e11e700-0000-4000-8000-0000000005a1", "5e11e700-0000-4000-8000-0000000005b1"
		storeA, storeB       = "5e11e700-0000-4000-8000-0000000007a1", "5e11e700-0000-4000-8000-0000000007b1"
		category             = "5e11e700-0000-4000-8000-00000000c001"
		listingA, listingB   = "5e11e700-0000-4000-8000-0000000001a1", "5e11e700-0000-4000-8000-0000000001b1"
	)
	sellerUser(t, db, ctx, userA, "+243810000001")
	sellerUser(t, db, ctx, userB, "+243810000002")
	sellerUser(t, db, ctx, staffA, "+243810000003")
	createTestOrganization(t, db, ctx, orgA)
	createTestOrganization(t, db, ctx, orgB)
	orgs := postgres.NewOrganizationRepository(db.DB)
	for _, m := range []*domain.OrganizationMember{
		domain.NewOrganizationMember(userA, orgA, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(staffA, orgA, domain.OrganizationRoleStaff),
		domain.NewOrganizationMember(userB, orgB, domain.OrganizationRoleOwner),
	} {
		if err := orgs.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	createTestSeller(t, db, ctx, sellerA, userA, orgA)
	createTestSeller(t, db, ctx, sellerB, userB, orgB)
	createTestStore(t, db, ctx, storeA, orgA)
	storeBRow := domain.NewStore(orgB, "Store B", "store-b", "CD", "+243810000002")
	storeBRow.ID = storeB
	if err := postgres.NewStoreRepository(db.DB).Create(ctx, storeBRow); err != nil {
		t.Fatal(err)
	}
	createTestCategory(t, db, ctx, category)
	createTestListing(t, db, ctx, listingA, orgA, sellerA, storeA, category)
	createTestListing(t, db, ctx, listingB, orgB, sellerB, storeB, category)

	listingRepo := postgres.NewListingRepository(db.DB)
	uc := application.NewListingUseCases(
		listingRepo, postgres.NewCategoryRepository(db.DB),
		postgres.NewAttributeDefinitionRepository(db.DB), postgres.NewAttributeValueRepository(db.DB),
		postgres.NewListingMediaRepository(db.DB), postgres.NewListingAvailabilityRepository(db.DB),
		postgres.NewLocationRepository(db.DB), postgres.NewListingLocationRepository(db.DB),
		orgs, postgres.NewSellerRepository(db.DB), postgres.NewStoreRepository(db.DB), postgres.NewUserRepository(db.DB), nil,
	)
	title := func(s string) application.ListingPatch { return application.ListingPatch{Title: &s} }

	// Another seller cannot edit, pause or publish A's listing.
	if _, err := uc.UpdateListing(ctx, userB, listingA, title("hijacked")); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("cross-organization edit: %v", err)
	}
	if err := uc.PauseListing(ctx, userB, listingA); err == nil {
		t.Fatal("cross-organization pause must fail")
	}
	if err := uc.PublishListing(ctx, userB, listingA); err == nil {
		t.Fatal("cross-organization publish must fail")
	}
	// Staff can read but not edit.
	if _, err := uc.UpdateListing(ctx, staffA, listingA, title("staff edit")); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("staff edit: %v", err)
	}
	// Anonymous callers never reach the store.
	if _, err := uc.UpdateListing(ctx, "", listingA, title("anon")); !errors.Is(err, domain.ErrInsufficientPermissions) {
		t.Fatalf("anonymous edit: %v", err)
	}

	// The owner edits a draft; invalid values are refused.
	price := -5.0
	if _, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{PriceAmount: &price}); !errors.Is(err, domain.ErrInvalidParameter) {
		t.Fatalf("negative price: %v", err)
	}
	price = 250
	updated, err := uc.UpdateListing(ctx, userA, listingA, application.ListingPatch{Title: strPtr("Sea-view flat"), PriceAmount: &price})
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := listingRepo.FindByID(ctx, listingA)
	if updated.Title != "Sea-view flat" || stored.Title != "Sea-view flat" || stored.PriceAmount != 250 ||
		stored.OrganizationID != orgA || stored.SellerID != sellerA {
		t.Fatalf("edit not persisted or ownership changed: %+v", stored)
	}

	// A live listing must be paused before editing.
	if err := uc.PublishListing(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.UpdateListing(ctx, userA, listingA, title("live edit")); !errors.Is(err, domain.ErrInvalidStatusTransition) {
		t.Fatalf("editing a live listing: %v", err)
	}
	if err := uc.PauseListing(ctx, userA, listingA); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.UpdateListing(ctx, userA, listingA, title("after pause")); err != nil {
		t.Fatalf("editing a paused listing: %v", err)
	}

	// Organization-scoped lists never include another organization's rows.
	items, err := listingRepo.FindByOrganizationID(ctx, orgA, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != listingA {
		t.Fatalf("org A listings = %+v", items)
	}
	orders, err := postgres.NewOrderRepository(db.DB).FindByOrganizationID(ctx, orgB, 50, 0)
	if err != nil || len(orders) != 0 {
		t.Fatalf("org B orders = %v %v", orders, err)
	}
	reservations, err := postgres.NewReservationRepository(db.DB).FindByOrganizationID(ctx, orgB, 50, 0)
	if err != nil || len(reservations) != 0 {
		t.Fatalf("org B reservations = %v %v", reservations, err)
	}
}

// A buyer must never push their own order through seller-only transitions.
func TestSellerConsole_OrderConfirmationIsSellerOnly(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const (
		owner, staff, buyer = "5e11e700-0000-4000-8000-0000000000c1", "5e11e700-0000-4000-8000-0000000000c2", "5e11e700-0000-4000-8000-0000000000c3"
		org                 = "5e11e700-0000-4000-8000-00000000000c"
		seller, store       = "5e11e700-0000-4000-8000-0000000005c1", "5e11e700-0000-4000-8000-0000000007c1"
		category, listing   = "5e11e700-0000-4000-8000-00000000c002", "5e11e700-0000-4000-8000-0000000001c1"
	)
	sellerUser(t, db, ctx, owner, "+243810000011")
	sellerUser(t, db, ctx, staff, "+243810000012")
	sellerUser(t, db, ctx, buyer, "+243810000013")
	createTestOrganization(t, db, ctx, org)
	orgs := postgres.NewOrganizationRepository(db.DB)
	for _, m := range []*domain.OrganizationMember{
		domain.NewOrganizationMember(owner, org, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(staff, org, domain.OrganizationRoleStaff),
	} {
		if err := orgs.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	createTestSeller(t, db, ctx, seller, owner, org)
	createTestStore(t, db, ctx, store, org)
	createTestCategory(t, db, ctx, category)
	createTestListing(t, db, ctx, listing, org, seller, store, category)
	if _, err := db.DB.ExecContext(ctx, `UPDATE listings SET status='published' WHERE id=$1`, listing); err != nil {
		t.Fatal(err)
	}

	orders := application.NewOrderUseCases(postgres.NewOrderRepository(db.DB), postgres.NewListingRepository(db.DB),
		postgres.NewSellerRepository(db.DB), orgs, postgres.NewUserRepository(db.DB),
		postgres.NewInventoryRepository(db.DB), postgres.NewFulfillmentRepository(db.DB), nil)
	order, err := orders.CreateOrder(ctx, buyer, listing, 1)
	if err != nil {
		t.Fatalf("buyer creates order: %v", err)
	}
	if _, err := db.DB.ExecContext(ctx, `UPDATE orders SET status='paid' WHERE id=$1`, order.ID); err != nil {
		t.Fatal(err)
	}

	for name, actor := range map[string]string{"buyer": buyer, "staff": staff} {
		if _, err := orders.ConfirmOrder(ctx, actor, order.ID); !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Fatalf("%s confirmed the order: %v", name, err)
		}
		if _, err := orders.CompleteOrder(ctx, actor, order.ID); !errors.Is(err, domain.ErrInsufficientPermissions) {
			t.Fatalf("%s completed the order: %v", name, err)
		}
	}
	// The buyer keeps read access to their own order.
	if _, err := orders.GetOrder(ctx, buyer, order.ID); err != nil {
		t.Fatalf("buyer reads own order: %v", err)
	}
	confirmed, err := orders.ConfirmOrder(ctx, owner, order.ID)
	if err != nil || confirmed.Status != domain.OrderStatusConfirmed {
		t.Fatalf("owner confirms: %v %v", confirmed, err)
	}
}
