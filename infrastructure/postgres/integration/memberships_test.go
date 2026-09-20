package integration

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// TestMembershipRepository_Create verifies membership creation in PostgreSQL.
func TestMembershipRepository_Create(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Member Org", "Member Organization", "CD", "+243812345684")
	org.ID = "123e4567-e89b-12d3-a456-426614174010"
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345685", "Member User")
	user.ID = "123e4567-e89b-12d3-a456-426614174011"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	member := domain.NewOrganizationMember(user.ID, org.ID, domain.OrganizationRoleOwner)
	if err := orgRepo.AddMember(ctx, member); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	members, err := orgRepo.GetMembers(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetMembers failed: %v", err)
	}

	if len(members) != 1 {
		t.Fatalf("Expected 1 member, got %d", len(members))
	}
	if members[0].UserID != user.ID {
		t.Errorf("Expected UserID %s, got %s", user.ID, members[0].UserID)
	}
	if members[0].Role != domain.OrganizationRoleOwner {
		t.Errorf("Expected Role owner, got %s", members[0].Role)
	}
}

// TestMembershipRepository_UniqueConstraint verifies UNIQUE(organization_id, user_id).
func TestMembershipRepository_UniqueConstraint(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Unique Org", "Unique Organization", "CD", "+243812345686")
	org.ID = "123e4567-e89b-12d3-a456-426614174012"
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345687", "Unique User")
	user.ID = "123e4567-e89b-12d3-a456-426614174013"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	member := domain.NewOrganizationMember(user.ID, org.ID, domain.OrganizationRoleAdmin)
	if err := orgRepo.AddMember(ctx, member); err != nil {
		t.Fatalf("First AddMember failed: %v", err)
	}

	duplicate := domain.NewOrganizationMember(user.ID, org.ID, domain.OrganizationRoleManager)
	err := orgRepo.AddMember(ctx, duplicate)
	if err == nil {
		t.Fatal("Expected error for duplicate membership, got nil")
	}
}

// TestMembershipRepository_ForeignKeys verifies FK constraints.
func TestMembershipRepository_ForeignKeys(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	member := domain.NewOrganizationMember("123e4567-e89b-12d3-a456-426614174014", "123e4567-e89b-12d3-a456-426614174015", domain.OrganizationRoleStaff)
	err := orgRepo.AddMember(ctx, member)
	if err == nil {
		t.Fatal("Expected FK violation for non-existent user/org, got nil")
	}
}

// TestMembershipRepository_Delete verifies membership deletion.
func TestMembershipRepository_Delete(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "Delete Org", "Delete Organization", "CD", "+243812345688")
	org.ID = "123e4567-e89b-12d3-a456-426614174016"
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	user := domain.NewUser("+243812345689", "Delete User")
	user.ID = "123e4567-e89b-12d3-a456-426614174017"
	if err := postgres.NewUserRepository(testDB.DB).Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	member := domain.NewOrganizationMember(user.ID, org.ID, domain.OrganizationRoleOwner)
	if err := orgRepo.AddMember(ctx, member); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	if err := orgRepo.RemoveMember(ctx, org.ID, user.ID); err != nil {
		t.Fatalf("RemoveMember failed: %v", err)
	}

	members, err := orgRepo.GetMembers(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetMembers after delete failed: %v", err)
	}

	if len(members) != 0 {
		t.Fatalf("Expected 0 members after delete, got %d", len(members))
	}
}

// TestMembershipRepository_TenantIsolation verifies tenant isolation.
func TestMembershipRepository_TenantIsolation(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	userRepo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	orgA := domain.NewOrganization(domain.OrganizationTypeBusiness, "Org A", "Organization A", "CD", "+243812345690")
	orgA.ID = "123e4567-e89b-12d3-a456-426614174018"
	if err := orgRepo.Create(ctx, orgA); err != nil {
		t.Fatalf("Create Org A failed: %v", err)
	}

	orgB := domain.NewOrganization(domain.OrganizationTypeBusiness, "Org B", "Organization B", "CD", "+243812345691")
	orgB.ID = "123e4567-e89b-12d3-a456-426614174019"
	if err := orgRepo.Create(ctx, orgB); err != nil {
		t.Fatalf("Create Org B failed: %v", err)
	}

	userA := domain.NewUser("+243812345692", "User A")
	userA.ID = "123e4567-e89b-12d3-a456-426614174020"
	if err := userRepo.Create(ctx, userA); err != nil {
		t.Fatalf("Create User A failed: %v", err)
	}

	userB := domain.NewUser("+243812345693", "User B")
	userB.ID = "123e4567-e89b-12d3-a456-426614174021"
	if err := userRepo.Create(ctx, userB); err != nil {
		t.Fatalf("Create User B failed: %v", err)
	}

	memberA := domain.NewOrganizationMember(userA.ID, orgA.ID, domain.OrganizationRoleOwner)
	if err := orgRepo.AddMember(ctx, memberA); err != nil {
		t.Fatalf("AddMember A failed: %v", err)
	}

	memberB := domain.NewOrganizationMember(userB.ID, orgB.ID, domain.OrganizationRoleAdmin)
	if err := orgRepo.AddMember(ctx, memberB); err != nil {
		t.Fatalf("AddMember B failed: %v", err)
	}

	membersA, err := orgRepo.GetMembers(ctx, orgA.ID)
	if err != nil {
		t.Fatalf("GetMembers A failed: %v", err)
	}
	if len(membersA) != 1 || membersA[0].UserID != userA.ID {
		t.Error("Org A should only contain User A")
	}

	membersB, err := orgRepo.GetMembers(ctx, orgB.ID)
	if err != nil {
		t.Fatalf("GetMembers B failed: %v", err)
	}
	if len(membersB) != 1 || membersB[0].UserID != userB.ID {
		t.Error("Org B should only contain User B")
	}
}
