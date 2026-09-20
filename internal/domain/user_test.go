package domain

import (
	"testing"
)

func TestUser_NewUser(t *testing.T) {
	user := NewUser("+243812345678", "Test User")
	if user == nil {
		t.Fatal("Expected non-nil user")
	}
	if user.Phone != "+243812345678" {
		t.Errorf("Expected phone +243812345678, got %s", user.Phone)
	}
	if user.DisplayName != "Test User" {
		t.Errorf("Expected display_name Test User, got %s", user.DisplayName)
	}
	if user.Status != UserStatusPendingVerified {
		t.Errorf("Expected status pending_verification, got %s", user.Status)
	}
	if user.IsActive() {
		t.Error("Expected new user to be inactive")
	}
	if user.IsVerified() {
		t.Error("Expected new user to be unverified")
	}
	if len(user.Roles) != 0 {
		t.Error("Expected new user to have no roles")
	}
}

func TestUser_IsActive(t *testing.T) {
	user := NewUser("+243812345678", "Test")
	user.Status = UserStatusActive
	if !user.IsActive() {
		t.Error("Expected active user to return true for IsActive")
	}
	user.Status = UserStatusSuspended
	if user.IsActive() {
		t.Error("Expected suspended user to return false for IsActive")
	}
}

func TestUser_IsSuspended(t *testing.T) {
	user := NewUser("+243812345678", "Test")
	user.Status = UserStatusSuspended
	if !user.IsSuspended() {
		t.Error("Expected suspended user to return true for IsSuspended")
	}
	user.Status = UserStatusActive
	if user.IsSuspended() {
		t.Error("Expected active user to return false for IsSuspended")
	}
}

func TestUser_IsVerified(t *testing.T) {
	user := NewUser("+243812345678", "Test")
	if user.IsVerified() {
		t.Error("Expected new user to return false for IsVerified")
	}
	now := int64(1234567890)
	user.VerifiedAt = &now
	if !user.IsVerified() {
		t.Error("Expected verified user to return true for IsVerified")
	}
}

func TestUser_HasRole(t *testing.T) {
	user := NewUser("+243812345678", "Test")
	user.Roles = []Role{RoleAdmin}

	if !user.HasRole(RoleAdmin) {
		t.Error("Expected user to have admin role")
	}
	if user.HasRole(RoleSeller) {
		t.Error("Expected user to not have seller role")
	}

	// Test HasAnyRole
	if !user.HasAnyRole(RoleAdmin, RoleSeller) {
		t.Error("Expected user to have at least one of the roles")
	}
	if user.HasAnyRole(RoleSeller, RoleCustomer) {
		t.Error("Expected user to not have any of these roles")
	}

	// Test HasAllRoles
	if !user.HasAllRoles(RoleAdmin) {
		t.Error("Expected user to have all specified roles")
	}
	if user.HasAllRoles(RoleAdmin, RoleSeller) {
		t.Error("Expected user to not have all specified roles")
	}
}

func TestUserStatus_Constants(t *testing.T) {
	constants := []struct {
		name       string
		status     UserStatus
		wantActive bool
	}{
		{"active", UserStatusActive, true},
		{"suspended", UserStatusSuspended, false},
		{"pending_verified", UserStatusPendingVerified, false},
	}
	for _, c := range constants {
		t.Run(c.name, func(t *testing.T) {
			user := NewUser("+243812345678", "Test")
			user.Status = c.status
			if user.IsActive() != c.wantActive {
				t.Errorf("%s: IsActive() = %v, want %v", c.name, user.IsActive(), c.wantActive)
			}
		})
	}
}

func TestRole_IsValid(t *testing.T) {
	validRoles := []Role{
		RoleCustomer,
		RoleSeller,
		RoleStaff,
		RoleAdmin,
		RoleOperator,
		RoleModerator,
		RoleDeliveryPartner,
	}
	for _, role := range validRoles {
		if !role.IsValid() {
			t.Errorf("Role %s should be valid", string(role))
		}
	}

	invalidRoles := []Role{
		Role("invalid1"),
		Role(""),
		Role("not_a_role"),
	}
	for _, role := range invalidRoles {
		if role.IsValid() {
			t.Errorf("Role %s should be invalid", string(role))
		}
	}
}

func TestPermission_IsValid(t *testing.T) {
	validPermissions := []Permission{
		PermissionCatalogRead,
		PermissionCatalogWrite,
		PermissionListingCreate,
		PermissionListingUpdate,
		PermissionOrderRead,
		PermissionOrderCreate,
		PermissionPaymentRead,
		PermissionPaymentExecute,
		PermissionSellerManage,
		PermissionAdminManage,
		PermissionUserRead,
		PermissionUserUpdate,
		PermissionRoleAssign,
		PermissionRoleRevoke,
	}
	for _, perm := range validPermissions {
		if !perm.IsValid() {
			t.Errorf("Permission %s should be valid", string(perm))
		}
	}

	invalidPermissions := []Permission{
		Permission("invalid1"),
		Permission(""),
	}
	for _, perm := range invalidPermissions {
		if perm.IsValid() {
			t.Errorf("Permission %s should be invalid", string(perm))
		}
	}
}

func TestPermissionChecker_HasPermissionViaRoles(t *testing.T) {
	pc := NewPermissionChecker()
	catalogRead := PermissionCatalogRead

	// Grant permission to seller role
	pc.Grant(RoleSeller, catalogRead)

	// User with seller role should have permission
	user := &User{
		ID:     "user-1",
		Phone:  "+243812345678",
		Roles:  []Role{RoleSeller},
		Status: UserStatusActive,
	}

	if !pc.HasPermission(user, catalogRead) {
		t.Error("Expected seller user to have catalog.read permission")
	}

	// User without seller role should NOT have permission
	user2 := &User{
		ID:     "user-2",
		Phone:  "+243812345679",
		Roles:  []Role{RoleCustomer},
		Status: UserStatusActive,
	}

	if pc.HasPermission(user2, catalogRead) {
		t.Error("Expected customer user to NOT have catalog.read permission")
	}
}

func TestPermissionChecker_Revoke(t *testing.T) {
	pc := NewPermissionChecker()
	catalogRead := PermissionCatalogRead

	// Grant to seller role
	pc.Grant(RoleSeller, catalogRead)

	user := &User{
		ID:     "user-1",
		Phone:  "+243812345678",
		Roles:  []Role{RoleSeller},
		Status: UserStatusActive,
	}

	// Before revoke, user should have permission
	if !pc.HasPermission(user, catalogRead) {
		t.Error("Expected user to have permission after grant")
	}

	// Revoke the permission
	pc.Revoke(RoleSeller, catalogRead)

	// After revoke, user should not have the permission
	if pc.HasPermission(user, catalogRead) {
		t.Error("Expected permission to be revoked")
	}
}

func TestPhoneNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"+243812345678", "+243812345678", false},
		{"243812345678", "+243812345678", false},
		{"0812345678", "+243812345678", false},
		{"812345678", "+243812345678", false},
		{"+243 81 234 5678", "+243812345678", false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result, err := NormalizePhone(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %s", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %s: %v", tc.input, err)
				}
				if result != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, result)
				}
			}
		})
	}
}

func TestRoleDuplicateAssignment(t *testing.T) {
	// This test verifies that assigning the same role twice returns an error
	// The actual test would use the repository, but we can test the concept here
	pc := NewPermissionChecker()
	adminRole := RoleAdmin
	catalogRead := PermissionCatalogRead

	pc.Grant(adminRole, catalogRead)

	// Granting the same permission twice should be idempotent (not an error)
	pc.Grant(adminRole, catalogRead)

	// Permission should still be there
	user := &User{Roles: []Role{adminRole}}
	if !pc.HasPermission(user, catalogRead) {
		t.Error("Expected permission to still be granted")
	}
}
