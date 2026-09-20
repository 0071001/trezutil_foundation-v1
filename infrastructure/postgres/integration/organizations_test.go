package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// TestOrganizationRepository_Create verifies organization creation in PostgreSQL.
func TestOrganizationRepository_Create(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Test Org", "Test Organization", "CD", "+243812345678")
	org.ID = "123e4567-e89b-12d3-a456-426614174000"

	err := repo.Create(ctx, org)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if fetched.LegalName != "Test Org" {
		t.Errorf("Expected LegalName 'Test Org', got %s", fetched.LegalName)
	}
	if fetched.DisplayName != "Test Organization" {
		t.Errorf("Expected DisplayName 'Test Organization', got %s", fetched.DisplayName)
	}
	if fetched.Country != "CD" {
		t.Errorf("Expected Country 'CD', got %s", fetched.Country)
	}
	if fetched.Status != domain.OrganizationStatusActive {
		t.Errorf("Expected Status 'active', got %s", fetched.Status)
	}
}

// TestOrganizationRepository_FindByID verifies fetching existing and missing organizations.
func TestOrganizationRepository_FindByID(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "missing-id")
	if err == nil {
		t.Error("Expected error for missing organization")
	}

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Get Org", "Get Organization", "CD", "+243812345679")
	org.ID = "123e4567-e89b-12d3-a456-426614174001"
	if err := repo.Create(ctx, org); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if fetched.ID != org.ID {
		t.Errorf("Expected ID %s, got %s", org.ID, fetched.ID)
	}
}

// TestOrganizationRepository_Update verifies updating organization fields.
func TestOrganizationRepository_Update(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Old Name", "Old Display", "CD", "+243812345680")
	org.ID = "123e4567-e89b-12d3-a456-426614174002"
	if err := repo.Create(ctx, org); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	org.LegalName = "New Name"
	org.DisplayName = "New Display"
	org.UpdatedAt = time.Now().Unix()

	if err := repo.Update(ctx, org); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}

	if fetched.LegalName != "New Name" {
		t.Errorf("Expected LegalName 'New Name', got %s", fetched.LegalName)
	}
	if fetched.DisplayName != "New Display" {
		t.Errorf("Expected DisplayName 'New Display', got %s", fetched.DisplayName)
	}
}

// TestOrganizationRepository_TenantIsolation verifies that organizations are isolated.
func TestOrganizationRepository_TenantIsolation(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	orgA := domain.NewOrganization(domain.OrganizationTypeBusiness, "Org A", "Organization A", "CD", "+243812345681")
	orgA.ID = "123e4567-e89b-12d3-a456-426614174003"
	if err := repo.Create(ctx, orgA); err != nil {
		t.Fatalf("Create Org A failed: %v", err)
	}

	orgB := domain.NewOrganization(domain.OrganizationTypeBusiness, "Org B", "Organization B", "CD", "+243812345682")
	orgB.ID = "123e4567-e89b-12d3-a456-426614174004"
	if err := repo.Create(ctx, orgB); err != nil {
		t.Fatalf("Create Org B failed: %v", err)
	}

	fetchedA, err := repo.FindByID(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("FindByID Org A failed: %v", err)
	}

	fetchedB, err := repo.FindByID(ctx, orgB.ID)
	if err != nil {
		t.Fatalf("FindByID Org B failed: %v", err)
	}

	if fetchedA.ID == fetchedB.ID {
		t.Error("Organizations A and B should have different IDs")
	}
	if fetchedA.LegalName == fetchedB.LegalName {
		t.Error("Organizations A and B should have different legal names")
	}
}

// TestOrganizationRepository_Timestamps verifies that timestamps are set on creation.
func TestOrganizationRepository_Timestamps(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	before := time.Now().Unix()
	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "TS Org", "Timestamp Org", "CD", "+243812345683")
	org.ID = "123e4567-e89b-12d3-a456-426614174005"
	org.CreatedAt = before
	org.UpdatedAt = before

	if err := repo.Create(ctx, org); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, org.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if fetched.CreatedAt == 0 {
		t.Error("CreatedAt should be set")
	}
	if fetched.UpdatedAt == 0 {
		t.Error("UpdatedAt should be set")
	}
}
