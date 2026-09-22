package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestInventoryConcurrency(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()

	orgID := testUUID()
	listingID := testUUID()
	categoryID := testUUID()

	orgRepo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Concurrency Test Org", "Concurrency Test Org", "US", "+15551234567")
	org.ID = orgID
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	sellerID := testUUID()
	userID := testUUID()
	storeID := testUUID()

	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)
	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Inventory Concurrency Test", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()
	if err := listingRepo.Create(ctx, listing); err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	inventoryRepo := postgres.NewInventoryRepository(db.DB)
	inventory := domain.NewInventoryResource(orgID, listingID, domain.ResourceTypeCountable, 1, time.Now().Unix())
	inventory.ID = testUUID()
	if err := inventoryRepo.Create(ctx, inventory); err != nil {
		t.Fatalf("failed to create inventory: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inv, err := inventoryRepo.FindByListingIDForUpdate(ctx, orgID, listingID)
			if err != nil {
				results <- err
				return
			}
			now := time.Now().Unix()
			if err := inv.Consume(1, now); err != nil {
				results <- err
				return
			}
			if err := inventoryRepo.Update(ctx, inv); err != nil {
				results <- err
				return
			}
			results <- nil
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else if err == domain.ErrOptimisticLock || err == domain.ErrInsufficientInventory {
			conflictCount++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
	if conflictCount != 1 {
		t.Errorf("expected 1 conflict, got %d", conflictCount)
	}

	finalInv, err := inventoryRepo.FindByListingID(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	if finalInv.Consumed != 1 {
		t.Errorf("expected consumed=1, got %d", finalInv.Consumed)
	}
	if finalInv.Available() != 0 {
		t.Errorf("expected available=0, got %d", finalInv.Available())
	}
}

func TestCapacityConcurrency(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()

	orgID := testUUID()
	listingID := testUUID()
	categoryID := testUUID()

	orgRepo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Capacity Concurrency Test Org", "Capacity Concurrency Test Org", "US", "+15551234567")
	org.ID = orgID
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	sellerID := testUUID()
	userID := testUUID()
	storeID := testUUID()

	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)
	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Capacity Concurrency Test", string(domain.ListingTypeService), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()
	if err := listingRepo.Create(ctx, listing); err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	inventoryRepo := postgres.NewInventoryRepository(db.DB)
	inventory := domain.NewInventoryResource(orgID, listingID, domain.ResourceTypeCapacity, 1, time.Now().Unix())
	inventory.ID = testUUID()
	if err := inventoryRepo.Create(ctx, inventory); err != nil {
		t.Fatalf("failed to create inventory: %v", err)
	}

	capacityRepo := postgres.NewCapacityRepository(db.DB)
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()
	capacityUnit := domain.NewCapacityUnit(inventory.ID, startAt, endAt, 1, time.Now().Unix())
	capacityUnit.ID = testUUID()
	if err := capacityRepo.Create(ctx, capacityUnit); err != nil {
		t.Fatalf("failed to create capacity unit: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			units, err := capacityRepo.FindByInventoryIDAndRangeForUpdate(ctx, orgID, inventory.ID, startAt, endAt)
			if err != nil {
				results <- err
				return
			}
			if len(units) == 0 {
				results <- domain.ErrCapacityNotFound
				return
			}
			unit := units[0]
			if unit.Available() < 1 {
				results <- domain.ErrInsufficientCapacity
				return
			}
			if err := capacityRepo.Reserve(ctx, unit.ID, 1, unit.Version); err != nil {
				results <- err
				return
			}
			results <- nil
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else if err == domain.ErrOptimisticLock || err == domain.ErrInsufficientCapacity {
			conflictCount++
		} else {
			t.Errorf("unexpected error: %v", err)
		}
	}

	if successCount != 1 {
		t.Errorf("expected 1 success, got %d", successCount)
	}
	if conflictCount != 1 {
		t.Errorf("expected 1 conflict, got %d", conflictCount)
	}

	finalUnits, err := capacityRepo.FindByInventoryIDAndRange(ctx, orgID, inventory.ID, startAt, endAt)
	if err != nil {
		t.Fatalf("failed to find capacity units: %v", err)
	}
	if len(finalUnits) != 1 {
		t.Fatalf("expected 1 capacity unit, got %d", len(finalUnits))
	}
	if finalUnits[0].Reserved != 1 {
		t.Errorf("expected reserved=1, got %d", finalUnits[0].Reserved)
	}
	if finalUnits[0].Available() != 0 {
		t.Errorf("expected available=0, got %d", finalUnits[0].Available())
	}
}

func TestInventoryIdempotency(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()

	orgID := testUUID()
	listingID := testUUID()
	categoryID := testUUID()

	orgRepo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Idempotency Test Org", "Idempotency Test Org", "US", "+15551234567")
	org.ID = orgID
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	sellerID := testUUID()
	userID := testUUID()
	storeID := testUUID()

	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)
	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Idempotency Test", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()
	if err := listingRepo.Create(ctx, listing); err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	inventoryRepo := postgres.NewInventoryRepository(db.DB)
	inventory := domain.NewInventoryResource(orgID, listingID, domain.ResourceTypeCountable, 5, time.Now().Unix())
	inventory.ID = testUUID()
	if err := inventoryRepo.Create(ctx, inventory); err != nil {
		t.Fatalf("failed to create inventory: %v", err)
	}

	inv, err := inventoryRepo.FindByListingIDForUpdate(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	now := time.Now().Unix()
	if err := inv.Consume(2, now); err != nil {
		t.Fatalf("failed to consume inventory: %v", err)
	}
	if err := inventoryRepo.Update(ctx, inv); err != nil {
		t.Fatalf("failed to update inventory: %v", err)
	}

	inv2, err := inventoryRepo.FindByListingID(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	if inv2.Consumed != 2 {
		t.Errorf("expected consumed=2 after first request, got %d", inv2.Consumed)
	}
}

func TestReservationCancellationReleasesCapacity(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()

	orgID := testUUID()
	listingID := testUUID()
	categoryID := testUUID()

	orgRepo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Cancel Test Org", "Cancel Test Org", "US", "+15551234567")
	org.ID = orgID
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	sellerID := testUUID()
	userID := testUUID()
	storeID := testUUID()

	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)
	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Cancel Test", string(domain.ListingTypeService), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()
	if err := listingRepo.Create(ctx, listing); err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	inventoryRepo := postgres.NewInventoryRepository(db.DB)
	inventory := domain.NewInventoryResource(orgID, listingID, domain.ResourceTypeCapacity, 10, time.Now().Unix())
	inventory.ID = testUUID()
	if err := inventoryRepo.Create(ctx, inventory); err != nil {
		t.Fatalf("failed to create inventory: %v", err)
	}

	capacityRepo := postgres.NewCapacityRepository(db.DB)
	startAt := time.Now().Add(24 * time.Hour).Unix()
	endAt := time.Now().Add(48 * time.Hour).Unix()
	capacityUnit := domain.NewCapacityUnit(inventory.ID, startAt, endAt, 10, time.Now().Unix())
	capacityUnit.ID = testUUID()
	if err := capacityRepo.Create(ctx, capacityUnit); err != nil {
		t.Fatalf("failed to create capacity unit: %v", err)
	}

	reservationRepo := postgres.NewReservationRepository(db.DB)

	reservationID := testUUID()

	if err := capacityRepo.ReserveCapacity(ctx, orgID, listingID, startAt, endAt, 3, reservationID); err != nil {
		t.Fatalf("failed to reserve capacity: %v", err)
	}

	units, err := capacityRepo.FindByInventoryIDAndRange(ctx, orgID, inventory.ID, startAt, endAt)
	if err != nil {
		t.Fatalf("failed to find capacity units: %v", err)
	}
	if units[0].Reserved != 3 {
		t.Errorf("expected reserved=3 after reservation, got %d", units[0].Reserved)
	}

	reservation := domain.NewReservation(orgID, userID, listingID, startAt, endAt, 3, 1, time.Now().Unix())
	reservation.ID = reservationID
	if err := reservationRepo.Create(ctx, reservation); err != nil {
		t.Fatalf("failed to create reservation: %v", err)
	}

	if err := capacityRepo.ReleaseCapacity(ctx, orgID, listingID, startAt, endAt, 3, reservationID); err != nil {
		t.Fatalf("failed to release capacity: %v", err)
	}

	units, err = capacityRepo.FindByInventoryIDAndRange(ctx, orgID, inventory.ID, startAt, endAt)
	if err != nil {
		t.Fatalf("failed to find capacity units: %v", err)
	}
	if units[0].Reserved != 0 {
		t.Errorf("expected reserved=0 after cancellation, got %d", units[0].Reserved)
	}
	if units[0].Available() != 10 {
		t.Errorf("expected available=10 after cancellation, got %d", units[0].Available())
	}
}

func TestOrderCancellationRestoresInventory(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()

	orgID := testUUID()
	listingID := testUUID()
	categoryID := testUUID()

	orgRepo := postgres.NewOrganizationRepository(db.DB)
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Order Cancel Test Org", "Order Cancel Test Org", "US", "+15551234567")
	org.ID = orgID
	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("failed to create org: %v", err)
	}

	sellerID := testUUID()
	userID := testUUID()
	storeID := testUUID()

	createTestUser(t, db, ctx, userID)
	createTestSeller(t, db, ctx, sellerID, userID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)
	createTestCategory(t, db, ctx, categoryID)

	listingRepo := postgres.NewListingRepository(db.DB)
	listing := domain.NewListing(orgID, sellerID, storeID, categoryID, "Order Cancel Test", string(domain.ListingTypePhysicalProduct), string(domain.PricingModelFixed), "USD", 100.0)
	listing.ID = listingID
	listing.CreatedAt = time.Now().Unix()
	listing.UpdatedAt = time.Now().Unix()
	if err := listingRepo.Create(ctx, listing); err != nil {
		t.Fatalf("failed to create listing: %v", err)
	}

	inventoryRepo := postgres.NewInventoryRepository(db.DB)
	inventory := domain.NewInventoryResource(orgID, listingID, domain.ResourceTypeCountable, 5, time.Now().Unix())
	inventory.ID = testUUID()
	if err := inventoryRepo.Create(ctx, inventory); err != nil {
		t.Fatalf("failed to create inventory: %v", err)
	}

	orderRepo := postgres.NewOrderRepository(db.DB)

	orderID := testUUID()
	order := domain.NewOrder(orgID, sellerID, userID, storeID, []*domain.OrderItem{
		domain.NewOrderItem("", listingID, "Test", 2, domain.NewMoney(10000, "USD"), time.Now().Unix()),
	}, "USD", time.Now().Unix())
	order.ID = orderID
	if err := orderRepo.Create(ctx, order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	inv, err := inventoryRepo.FindByListingID(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	if inv.Consumed != 0 || inv.Reserved != 0 {
		t.Errorf("expected no inventory change for DRAFT order, got consumed=%d reserved=%d", inv.Consumed, inv.Reserved)
	}

	inv, err = inventoryRepo.FindByListingIDForUpdate(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory for update: %v", err)
	}
	if err := inv.Consume(2, time.Now().Unix()); err != nil {
		t.Fatalf("failed to consume inventory: %v", err)
	}
	if err := inventoryRepo.Update(ctx, inv); err != nil {
		t.Fatalf("failed to update inventory: %v", err)
	}

	inv, err = inventoryRepo.FindByListingID(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	if inv.Consumed != 2 {
		t.Errorf("expected consumed=2 after confirmation, got %d", inv.Consumed)
	}

	inv, err = inventoryRepo.FindByListingIDForUpdate(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory for update: %v", err)
	}
	if err := inv.Restore(2, time.Now().Unix()); err != nil {
		t.Fatalf("failed to restore inventory: %v", err)
	}
	if err := inventoryRepo.Update(ctx, inv); err != nil {
		t.Fatalf("failed to update inventory: %v", err)
	}

	inv, err = inventoryRepo.FindByListingID(ctx, orgID, listingID)
	if err != nil {
		t.Fatalf("failed to find inventory: %v", err)
	}
	if inv.Consumed != 0 {
		t.Errorf("expected consumed=0 after refund, got %d", inv.Consumed)
	}
	if inv.Available() != 5 {
		t.Errorf("expected available=5 after refund, got %d", inv.Available())
	}
}
