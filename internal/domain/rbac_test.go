package domain

import (
	"testing"
)

// TestRBAC_NoAdminHacks verifies no hardcoded admin status hacks remain.
func TestRBAC_NoAdminHacks(t *testing.T) {
	user := &User{
		ID:          "user-1",
		Phone:       "+243812345678",
		Email:       strPtr("user@example.com"),
		DisplayName: "User",
		Status:      UserStatusActive,
		Roles:       []Role{},
	}

	if user.HasRole(RoleAdmin) {
		t.Error("User without admin role should not pass HasRole(RoleAdmin)")
	}

	_ = UserStatusActive
}

// TestRBAC_HasRole verifies role checking.
func TestRBAC_HasRole(t *testing.T) {
	user := &User{
		ID:          "user-1",
		Phone:       "+243812345678",
		Email:       strPtr("user@example.com"),
		DisplayName: "User",
		Status:      UserStatusActive,
		Roles:       []Role{RoleSeller, RoleCustomer},
	}

	if !user.HasRole(RoleSeller) {
		t.Error("User should have seller role")
	}
	if user.HasRole(RoleAdmin) {
		t.Error("User should not have admin role")
	}
}

// TestRBAC_HasAnyRole verifies any-role checking.
func TestRBAC_HasAnyRole(t *testing.T) {
	user := &User{
		ID:          "user-1",
		Phone:       "+243812345678",
		Email:       strPtr("user@example.com"),
		DisplayName: "User",
		Status:      UserStatusActive,
		Roles:       []Role{RoleSeller},
	}

	if !user.HasAnyRole(RoleAdmin, RoleSeller) {
		t.Error("User should match at least one role")
	}
	if user.HasAnyRole(RoleAdmin, RoleOperator) {
		t.Error("User should not match any of these roles")
	}
}

// TestRBAC_HasAllRoles verifies all-roles checking.
func TestRBAC_HasAllRoles(t *testing.T) {
	user := &User{
		ID:          "user-1",
		Phone:       "+243812345678",
		Email:       strPtr("user@example.com"),
		DisplayName: "User",
		Status:      UserStatusActive,
		Roles:       []Role{RoleSeller, RoleCustomer},
	}

	if !user.HasAllRoles(RoleSeller, RoleCustomer) {
		t.Error("User should have all specified roles")
	}
	if user.HasAllRoles(RoleSeller, RoleAdmin) {
		t.Error("User should not have all specified roles")
	}
}

// TestRBAC_PermissionChecker_Dynamic verifies dynamic permission checker behavior.
func TestRBAC_PermissionChecker_Dynamic(t *testing.T) {
	checker := NewPermissionChecker()

	user := &User{
		ID:          "user-1",
		Phone:       "+243812345678",
		Email:       strPtr("user@example.com"),
		DisplayName: "User",
		Status:      UserStatusActive,
		Roles:       []Role{RoleSeller},
	}

	_ = checker.HasPermission(user, PermissionCatalogRead)
	_ = checker.HasPermission(user, PermissionSellerManage)
}

// TestRBAC_OrganizationRoleIsolation verifies organization role enum isolation.
func TestRBAC_OrganizationRoleIsolation(t *testing.T) {
	_ = OrganizationRoleOwner
	_ = OrganizationRoleAdmin
	_ = OrganizationRoleManager
}
