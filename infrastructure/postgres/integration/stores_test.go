package integration

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// TestStoreRepository_Create verifies store creation in PostgreSQL.
func TestStoreRepository_Create(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewStoreRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Store Org", "Store Organization", "CD", "+243812345710")
	org.ID = "123e4567-e89b-12d3-a456-426614174050"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	store := domain.NewStore(org.ID, "Test Store", "test-store", "CD", "+243812345711")
	store.ID = "123e4567-e89b-12d3-a456-426614174051"
	store.Description = strPtr("Test description")

	if err := repo.Create(ctx, store); err != nil {
		t.Fatalf("Create store failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, store.ID)
	if err != nil {
		t.Fatalf("FindByID store failed: %v", err)
	}

	if fetched.Name != "Test Store" {
		t.Errorf("Expected Name 'Test Store', got %s", fetched.Name)
	}
	if fetched.Slug != "test-store" {
		t.Errorf("Expected Slug 'test-store', got %s", fetched.Slug)
	}
	if fetched.OrganizationID != org.ID {
		t.Errorf("Expected OrganizationID %s, got %s", org.ID, fetched.OrganizationID)
	}
}

// TestStoreRepository_FindByID verifies fetching existing and missing stores.
func TestStoreRepository_FindByID(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewStoreRepository(testDB.DB)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "missing-id")
	if err == nil {
		t.Error("Expected error for missing store")
	}

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Get Store Org", "Get Store Organization", "CD", "+243812345712")
	org.ID = "123e4567-e89b-12d3-a456-426614174052"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	store := domain.NewStore(org.ID, "Get Store", "get-store", "CD", "+243812345713")
	store.ID = "123e4567-e89b-12d3-a456-426614174053"
	if err := repo.Create(ctx, store); err != nil {
		t.Fatalf("Create store failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, store.ID)
	if err != nil {
		t.Fatalf("FindByID store failed: %v", err)
	}

	if fetched.ID != store.ID {
		t.Errorf("Expected ID %s, got %s", store.ID, fetched.ID)
	}
}

// TestStoreRepository_Update verifies updating store fields.
func TestStoreRepository_Update(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewStoreRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Update Store Org", "Update Store Organization", "CD", "+243812345714")
	org.ID = "123e4567-e89b-12d3-a456-426614174054"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	store := domain.NewStore(org.ID, "Old Store", "old-store", "CD", "+243812345715")
	store.ID = "123e4567-e89b-12d3-a456-426614174055"
	if err := repo.Create(ctx, store); err != nil {
		t.Fatalf("Create store failed: %v", err)
	}

	store.Name = "New Store"
	store.Slug = "new-store"

	if err := repo.Update(ctx, store); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, store.ID)
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}

	if fetched.Name != "New Store" {
		t.Errorf("Expected Name 'New Store', got %s", fetched.Name)
	}
	if fetched.Slug != "new-store" {
		t.Errorf("Expected Slug 'new-store', got %s", fetched.Slug)
	}
}

// TestStoreRepository_SlugUnique verifies UNIQUE(stores.slug) constraint.
func TestStoreRepository_SlugUnique(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewStoreRepository(testDB.DB)
	ctx := context.Background()

	orgA := domain.NewOrganization(domain.OrganizationTypeBusiness, "Slug Org A", "Slug Organization A", "CD", "+243812345716")
	orgA.ID = "123e4567-e89b-12d3-a456-426614174056"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, orgA); err != nil {
		t.Fatalf("Create Org A failed: %v", err)
	}

	orgB := domain.NewOrganization(domain.OrganizationTypeBusiness, "Slug Org B", "Slug Organization B", "CD", "+243812345717")
	orgB.ID = "123e4567-e89b-12d3-a456-426614174057"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, orgB); err != nil {
		t.Fatalf("Create Org B failed: %v", err)
	}

	storeA := domain.NewStore(orgA.ID, "Store A", "duplicate-slug", "CD", "+243812345718")
	storeA.ID = "123e4567-e89b-12d3-a456-426614174058"
	if err := repo.Create(ctx, storeA); err != nil {
		t.Fatalf("Create Store A failed: %v", err)
	}

	storeB := domain.NewStore(orgB.ID, "Store B", "duplicate-slug", "CD", "+243812345719")
	storeB.ID = "123e4567-e89b-12d3-a456-426614174059"
	err := repo.Create(ctx, storeB)
	if err == nil {
		t.Fatal("Expected UNIQUE constraint violation for duplicate slug, got nil")
	}
}

// TestStoreRepository_TenantIsolation verifies tenant isolation.
func TestStoreRepository_TenantIsolation(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	storeRepo := postgres.NewStoreRepository(testDB.DB)
	ctx := context.Background()

	orgA := domain.NewOrganization(domain.OrganizationTypeBusiness, "Store Tenant Org A", "Store Tenant Organization A", "CD", "+243812345720")
	orgA.ID = "123e4567-e89b-12d3-a456-426614174060"
	if err := orgRepo.Create(ctx, orgA); err != nil {
		t.Fatalf("Create Org A failed: %v", err)
	}

	orgB := domain.NewOrganization(domain.OrganizationTypeBusiness, "Store Tenant Org B", "Store Tenant Organization B", "CD", "+243812345721")
	orgB.ID = "123e4567-e89b-12d3-a456-426614174061"
	if err := orgRepo.Create(ctx, orgB); err != nil {
		t.Fatalf("Create Org B failed: %v", err)
	}

	storeA := domain.NewStore(orgA.ID, "Store A", "store-tenant-a", "CD", "+243812345722")
	storeA.ID = "123e4567-e89b-12d3-a456-426614174062"
	if err := storeRepo.Create(ctx, storeA); err != nil {
		t.Fatalf("Create Store A failed: %v", err)
	}

	storeB := domain.NewStore(orgB.ID, "Store B", "store-tenant-b", "CD", "+243812345723")
	storeB.ID = "123e4567-e89b-12d3-a456-426614174063"
	if err := storeRepo.Create(ctx, storeB); err != nil {
		t.Fatalf("Create Store B failed: %v", err)
	}

	fetchedA, err := storeRepo.FindByID(ctx, storeA.ID)
	if err != nil {
		t.Fatalf("FindByID Store A failed: %v", err)
	}
	if fetchedA.OrganizationID != orgA.ID {
		t.Errorf("Store A should belong to Org A, got %s", fetchedA.OrganizationID)
	}

	fetchedB, err := storeRepo.FindByID(ctx, storeB.ID)
	if err != nil {
		t.Fatalf("FindByID Store B failed: %v", err)
	}
	if fetchedB.OrganizationID != orgB.ID {
		t.Errorf("Store B should belong to Org B, got %s", fetchedB.OrganizationID)
	}
}
