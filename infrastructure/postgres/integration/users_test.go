package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// TestUserRepository_Create verifies user creation in PostgreSQL.
func TestUserRepository_Create(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345724", "Create User")
	user.ID = "123e4567-e89b-12d3-a456-426614174070"
	user.Email = strPtr("create@example.com")
	user.DisplayName = "Create User Display"

	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	fetched, err := repo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if fetched.Phone != user.Phone {
		t.Errorf("Expected Phone %s, got %s", user.Phone, fetched.Phone)
	}
	if fetched.DisplayName != "Create User Display" {
		t.Errorf("Expected DisplayName 'Create User Display', got %s", fetched.DisplayName)
	}
}

// TestUserRepository_FindByPhone verifies lookup by phone.
func TestUserRepository_FindByPhone(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345725", "Phone User")
	user.ID = "123e4567-e89b-12d3-a456-426614174071"
	if err := repo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	fetched, err := repo.FindByPhone(ctx, user.Phone)
	if err != nil {
		t.Fatalf("FindByPhone failed: %v", err)
	}

	if fetched.ID != user.ID {
		t.Errorf("Expected ID %s, got %s", user.ID, fetched.ID)
	}
}

// TestUserRepository_NotFound verifies missing user behavior.
func TestUserRepository_NotFound(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "non-existent")
	if err == nil {
		t.Error("Expected error for missing user")
	}

	_, err = repo.FindByPhone(ctx, "+243800000000")
	if err == nil {
		t.Error("Expected error for missing phone")
	}
}

// TestUserRepository_RolesLoaded verifies roles are loaded from user_roles.
func TestUserRepository_RolesLoaded(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	userRepo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345726", "Roles User")
	user.ID = "123e4567-e89b-12d3-a456-426614174072"
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174073", "seller", "Seller role", time.Now())
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}

	if err := userRepo.AssignRole(ctx, user.ID, "seller"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	fetched, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if len(fetched.Roles) != 1 || fetched.Roles[0] != domain.RoleSeller {
		t.Errorf("Expected roles [seller], got %v", fetched.Roles)
	}

	if err := userRepo.RemoveRole(ctx, user.ID, "seller"); err != nil {
		t.Fatalf("RemoveRole failed: %v", err)
	}

	fetched, err = userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID after remove failed: %v", err)
	}

	if len(fetched.Roles) != 0 {
		t.Errorf("Expected no roles after removal, got %v", fetched.Roles)
	}
}
