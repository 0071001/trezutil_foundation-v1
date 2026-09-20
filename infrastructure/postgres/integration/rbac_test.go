package integration

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// TestRoleRepository_Exists verifies role existence check.
func TestRoleRepository_Exists(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewRoleRepository(testDB.DB)
	ctx := context.Background()

	exists, err := repo.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Expected nonexistent role to not exist")
	}

	now := time.Now()
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174080", "exists_role", "Test role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}

	exists, err = repo.Exists(ctx, "exists_role")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected existing role to exist")
	}
}

// TestRoleRepository_GetAll verifies fetching all roles.
func TestRoleRepository_GetAll(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewRoleRepository(testDB.DB)
	ctx := context.Background()

	roles, err := repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(roles) != 0 {
		t.Fatalf("Expected 0 roles initially, got %d", len(roles))
	}

	now := time.Now()
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174081", "role1", "Role 1", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174082", "role2", "Role 2", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}

	roles, err = repo.GetAll(ctx)
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}

	if len(roles) != 2 {
		t.Fatalf("Expected 2 roles, got %d", len(roles))
	}
}

// TestRoleRepository_GetPermissions verifies fetching permissions for a role.
func TestRoleRepository_GetPermissions(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	repo := postgres.NewRoleRepository(testDB.DB)
	ctx := context.Background()

	perms, err := repo.GetPermissions(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}
	if len(perms) != 0 {
		t.Fatalf("Expected 0 permissions for nonexistent role, got %d", len(perms))
	}

	now := time.Now()
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174083", "test_role", "Test role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO permissions (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174084", "test.permission", "Test permission", now)
	if err != nil {
		t.Fatalf("Failed to insert test permission: %v", err)
	}
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO role_permissions (role_id, permission_id, granted_at) VALUES ($1, $2, $3)",
		"123e4567-e89b-12d3-a456-426614174083", "123e4567-e89b-12d3-a456-426614174084", now)
	if err != nil {
		t.Fatalf("Failed to insert role_permission: %v", err)
	}

	perms, err = repo.GetPermissions(ctx, "test_role")
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 || perms[0] != domain.Permission("test.permission") {
		t.Errorf("Expected permissions [test.permission], got %v", perms)
	}
}

// TestUserRoleJunction verifies user_roles junction table.
func TestUserRoleJunction(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	userRepo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345727", "Junction User")
	user.ID = "123e4567-e89b-12d3-a456-426614174085"
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	now := time.Now()
	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174086", "junction_role", "Junction role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}

	if err := userRepo.AssignRole(ctx, user.ID, "junction_role"); err != nil {
		t.Fatalf("AssignRole failed: %v", err)
	}

	fetched, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if len(fetched.Roles) != 1 || fetched.Roles[0] != domain.Role("junction_role") {
		t.Errorf("Expected roles [junction_role], got %v", fetched.Roles)
	}

	if err := userRepo.RemoveRole(ctx, user.ID, "junction_role"); err != nil {
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

// TestUserRoleJunction_PrimaryKey verifies PK(user_id, role_id).
func TestUserRoleJunction_PrimaryKey(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	userRepo := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345728", "PK User")
	user.ID = "123e4567-e89b-12d3-a456-426614174087"
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	now := time.Now()
	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174088", "pk_user_role", "PK user role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}

	if err := userRepo.AssignRole(ctx, user.ID, "pk_user_role"); err != nil {
		t.Fatalf("First AssignRole failed: %v", err)
	}

	err = userRepo.AssignRole(ctx, user.ID, "pk_user_role")
	if err == nil {
		t.Fatal("Expected PK violation for duplicate user_role, got nil")
	}
}

// TestRolePermissionJunction verifies role_permissions junction table.
func TestRolePermissionJunction(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	now := time.Now()
	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174089", "perm_junction", "Permission Junction role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO permissions (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174090", "perm_junction.read", "Read permission junction", now)
	if err != nil {
		t.Fatalf("Failed to insert test permission: %v", err)
	}

	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO role_permissions (role_id, permission_id, granted_at) VALUES ($1, $2, $3)",
		"123e4567-e89b-12d3-a456-426614174089", "123e4567-e89b-12d3-a456-426614174090", now)
	if err != nil {
		t.Fatalf("Failed to insert role_permission: %v", err)
	}

	repo := postgres.NewRoleRepository(testDB.DB)
	perms, err := repo.GetPermissions(ctx, "perm_junction")
	if err != nil {
		t.Fatalf("GetPermissions failed: %v", err)
	}

	if len(perms) != 1 || perms[0] != domain.Permission("perm_junction.read") {
		t.Errorf("Expected permissions [perm_junction.read], got %v", perms)
	}

	_, err = testDB.DB.ExecContext(ctx, "DELETE FROM role_permissions WHERE role_id = $1 AND permission_id = $2",
		"123e4567-e89b-12d3-a456-426614174089", "123e4567-e89b-12d3-a456-426614174090")
	if err != nil {
		t.Fatalf("Failed to delete role_permission: %v", err)
	}

	perms, err = repo.GetPermissions(ctx, "perm_junction")
	if err != nil {
		t.Fatalf("GetPermissions after delete failed: %v", err)
	}

	if len(perms) != 0 {
		t.Errorf("Expected no permissions after delete, got %v", perms)
	}
}

// TestRolePermissionJunction_PrimaryKey verifies PK(role_id, permission_id).
func TestRolePermissionJunction_PrimaryKey(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	ctx := context.Background()

	now := time.Now()
	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174091", "pk_role", "PK role", now)
	if err != nil {
		t.Fatalf("Failed to insert test role: %v", err)
	}
	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO permissions (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174092", "pk_perm.read", "PK permission", now)
	if err != nil {
		t.Fatalf("Failed to insert test permission: %v", err)
	}

	_, err = testDB.DB.ExecContext(ctx, "INSERT INTO role_permissions (role_id, permission_id, granted_at) VALUES ($1, $2, $3)",
		"123e4567-e89b-12d3-a456-426614174091", "123e4567-e89b-12d3-a456-426614174092", now)
	if err != nil {
		t.Fatalf("First role_permission insert failed: %v", err)
	}

	err = testDB.DB.QueryRowContext(ctx, "INSERT INTO role_permissions (role_id, permission_id, granted_at) VALUES ($1, $2, $3) RETURNING id",
		"123e4567-e89b-12d3-a456-426614174091", "123e4567-e89b-12d3-a456-426614174092", now).Scan(new(sql.NullString))
	if err == nil {
		t.Fatal("Expected PK violation for duplicate role_permission, got nil")
	}
}

// TestRBAC_PlatformVsOrganizationRoles verifies separation of Platform Role and Organization Role.
func TestRBAC_PlatformVsOrganizationRoles(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()

	userRepo := postgres.NewUserRepository(testDB.DB)
	orgRepo := postgres.NewOrganizationRepository(testDB.DB)
	ctx := context.Background()

	user := domain.NewUser("+243812345729", "RBAC User")
	user.ID = "123e4567-e89b-12d3-a456-426614174093"
	if err := userRepo.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	now := time.Now()
	_, err := testDB.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"123e4567-e89b-12d3-a456-426614174094", "admin", "Platform admin", now)
	if err != nil {
		t.Fatalf("Failed to insert platform role: %v", err)
	}

	if err := userRepo.AssignRole(ctx, user.ID, "admin"); err != nil {
		t.Fatalf("Assign platform role failed: %v", err)
	}

	fetched, err := userRepo.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	if len(fetched.Roles) != 1 || fetched.Roles[0] != domain.RoleAdmin {
		t.Errorf("Expected platform role [admin], got %v", fetched.Roles)
	}

	org := domain.NewOrganization(domain.OrganizationTypeBusiness, "RBAC Org", "RBAC Organization", "CD", "+243812345730")
	org.ID = "123e4567-e89b-12d3-a456-426614174095"
	if err := orgRepo.Create(ctx, org); err != nil {
		t.Fatalf("Create organization failed: %v", err)
	}

	member := domain.NewOrganizationMember(user.ID, org.ID, domain.OrganizationRoleOwner)
	if err := orgRepo.AddMember(ctx, member); err != nil {
		t.Fatalf("Create membership failed: %v", err)
	}

	members, err := orgRepo.GetMembers(ctx, org.ID)
	if err != nil {
		t.Fatalf("GetMembers failed: %v", err)
	}

	if len(members) != 1 || members[0].Role != domain.OrganizationRoleOwner {
		t.Errorf("Expected organization role owner, got %s", members[0].Role)
	}

	platformRoles := fetched.Roles
	orgRoles := []domain.OrganizationRole{members[0].Role}

	if len(platformRoles) > 0 && len(orgRoles) > 0 {
		if string(platformRoles[0]) == string(orgRoles[0]) {
			t.Error("Platform role and Organization role should be distinct concepts")
		}
	}
}
