package integration

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// strPtr creates a pointer to a string.
func strPtr(s string) *string {
	return &s
}

// TestSellerRepository_Create verifies seller creation in PostgreSQL.
func TestSellerRepository_Create(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewSellerRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Seller Org", "Seller Organization", "CD", "+243812345694")
	org.ID = "123e4567-e89b-12d3-a456-426614174030"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345695", "Seller User")
	user.ID = "123e4567-e89b-12d3-a456-426614174031"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	seller := domain.NewSellerProfile(domain.SellerTypeBusiness, "Test Seller", "+243812345696", strPtr(user.ID), strPtr(org.ID))
	seller.ID = "123e4567-e89b-12d3-a456-426614174032"
	seller.Bio = strPtr("Bio text")
	seller.Email = strPtr("seller@example.com")

	if err := repo.Create(ctx, seller); err != nil {
		t.Fatalf("Create seller failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, seller.ID)
	if err != nil {
		t.Fatalf("FindByID seller failed: %v", err)
	}

	if fetched.DisplayName != "Test Seller" {
		t.Errorf("Expected DisplayName 'Test Seller', got %s", fetched.DisplayName)
	}
	if fetched.SellerType != domain.SellerTypeBusiness {
		t.Errorf("Expected SellerType business, got %s", fetched.SellerType)
	}
	if *fetched.UserID != user.ID {
		t.Errorf("Expected UserID %s, got %s", user.ID, *fetched.UserID)
	}
	if *fetched.OrganizationID != org.ID {
		t.Errorf("Expected OrganizationID %s, got %s", org.ID, *fetched.OrganizationID)
	}
}

// TestSellerRepository_FindByID verifies fetching existing and missing sellers.
func TestSellerRepository_FindByID(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewSellerRepository(testDB.DB)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "missing-id")
	if err == nil {
		t.Error("Expected error for missing seller")
	}

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Get Seller Org", "Get Seller Organization", "CD", "+243812345697")
	org.ID = "123e4567-e89b-12d3-a456-426614174033"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345698", "Get Seller User")
	user.ID = "123e4567-e89b-12d3-a456-426614174034"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	seller := domain.NewSellerProfile(domain.SellerTypeIndividual, "Get Seller", "+243812345699", strPtr(user.ID), strPtr(org.ID))
	seller.ID = "123e4567-e89b-12d3-a456-426614174035"
	if err := repo.Create(ctx, seller); err != nil {
		t.Fatalf("Create seller failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, seller.ID)
	if err != nil {
		t.Fatalf("FindByID seller failed: %v", err)
	}

	if fetched.ID != seller.ID {
		t.Errorf("Expected ID %s, got %s", seller.ID, fetched.ID)
	}
}

// TestSellerRepository_Update verifies updating seller fields.
func TestSellerRepository_Update(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewSellerRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Update Seller Org", "Update Seller Organization", "CD", "+243812345700")
	org.ID = "123e4567-e89b-12d3-a456-426614174036"
	if err := postgres.NewOrganizationRepository(testDB.DB).Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345701", "Update Seller User")
	user.ID = "123e4567-e89b-12d3-a456-426614174037"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	seller := domain.NewSellerProfile(domain.SellerTypeBusiness, "Old Name", "+243812345702", strPtr(user.ID), strPtr(org.ID))
	seller.ID = "123e4567-e89b-12d3-a456-426614174038"
	if err := repo.Create(ctx, seller); err != nil {
		t.Fatalf("Create seller failed: %v", err)
	}

	seller.DisplayName = "New Name"
	seller.Bio = strPtr("New bio")

	if err := repo.Update(ctx, seller); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, seller.ID)
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}

	if fetched.DisplayName != "New Name" {
		t.Errorf("Expected DisplayName 'New Name', got %s", fetched.DisplayName)
	}
}

// TestSellerRepository_ForeignKeys verifies FK constraints.
func TestSellerRepository_ForeignKeys(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewSellerRepository(testDB.DB)
	ctx := context.Background()

	seller := domain.NewSellerProfile(domain.SellerTypeBusiness, "FK Seller", "+243812345703", strPtr("123e4567-e89b-12d3-a456-426614174039"), strPtr("123e4567-e89b-12d3-a456-426614174040"))
	seller.ID = "123e4567-e89b-12d3-a456-426614174041"
	err := repo.Create(ctx, seller)
	if err == nil {
		t.Fatal("Expected FK violation for non-existent user/org, got nil")
	}
}

// TestSellerRepository_TenantIsolation verifies tenant isolation.
func TestSellerRepository_TenantIsolation(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	sellerRepo := postgres.NewSellerRepository(testDB.DB)
	userRepo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	orgA := domain.NewOrganization(domain.OrganizationTypeBusiness, "Seller Org A", "Seller Organization A", "CD", "+243812345704")
	orgA.ID = "123e4567-e89b-12d3-a456-426614174042"
	if err := orgRepo.Create(ctx, orgA); err != nil {
		t.Fatalf("Create Org A failed: %v", err)
	}

	orgB := domain.NewOrganization(domain.OrganizationTypeBusiness, "Seller Org B", "Seller Organization B", "CD", "+243812345705")
	orgB.ID = "123e4567-e89b-12d3-a456-426614174043"
	if err := orgRepo.Create(ctx, orgB); err != nil {
		t.Fatalf("Create Org B failed: %v", err)
	}

	userA := domain.NewUser("+243812345706", "Seller User A")
	userA.ID = "123e4567-e89b-12d3-a456-426614174044"
	if err := userRepo.Create(ctx, userA); err != nil {
		t.Fatalf("Create User A failed: %v", err)
	}

	userB := domain.NewUser("+243812345707", "Seller User B")
	userB.ID = "123e4567-e89b-12d3-a456-426614174045"
	if err := userRepo.Create(ctx, userB); err != nil {
		t.Fatalf("Create User B failed: %v", err)
	}

	sellerA := domain.NewSellerProfile(domain.SellerTypeBusiness, "Seller A", "+243812345708", strPtr(userA.ID), strPtr(orgA.ID))
	sellerA.ID = "123e4567-e89b-12d3-a456-426614174046"
	if err := sellerRepo.Create(ctx, sellerA); err != nil {
		t.Fatalf("Create Seller A failed: %v", err)
	}

	sellerB := domain.NewSellerProfile(domain.SellerTypeBusiness, "Seller B", "+243812345709", strPtr(userB.ID), strPtr(orgB.ID))
	sellerB.ID = "123e4567-e89b-12d3-a456-426614174047"
	if err := sellerRepo.Create(ctx, sellerB); err != nil {
		t.Fatalf("Create Seller B failed: %v", err)
	}

	fetchedA, err := sellerRepo.FindByID(ctx, sellerA.ID)
	if err != nil {
		t.Fatalf("FindByID Seller A failed: %v", err)
	}
	if *fetchedA.OrganizationID != orgA.ID {
		t.Errorf("Seller A should belong to Org A, got %s", *fetchedA.OrganizationID)
	}

	fetchedB, err := sellerRepo.FindByID(ctx, sellerB.ID)
	if err != nil {
		t.Fatalf("FindByID Seller B failed: %v", err)
	}
	if *fetchedB.OrganizationID != orgB.ID {
		t.Errorf("Seller B should belong to Org B, got %s", *fetchedB.OrganizationID)
	}
}
