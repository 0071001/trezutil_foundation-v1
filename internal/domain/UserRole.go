package domain

// UserRole represents the assignment of a role to a user.
type UserRole struct {
	UserID string
	Role   Role
}

// RolePermission represents the assignment of a permission to a role.
type RolePermission struct {
	Role       Role
	Permission Permission
}

// PermissionChecker evaluates permissions based on actual role assignments.
// IMPORTANT: This checks REAL roles from user_roles, NOT user.Status hacks.
type PermissionChecker struct {
	rolePermissions map[string]map[Permission]bool
}

// NewPermissionChecker creates a new PermissionChecker.
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		rolePermissions: make(map[string]map[Permission]bool),
	}
}

// Grant grants a permission to a role.
func (pc *PermissionChecker) Grant(role Role, perm Permission) {
	if pc.rolePermissions[string(role)] == nil {
		pc.rolePermissions[string(role)] = make(map[Permission]bool)
	}
	pc.rolePermissions[string(role)][perm] = true
}

// Revoke revokes a permission from a role.
func (pc *PermissionChecker) Revoke(role Role, perm Permission) {
	if perms, ok := pc.rolePermissions[string(role)]; ok {
		delete(perms, perm)
		if len(perms) == 0 {
			delete(pc.rolePermissions, string(role))
		}
	}
}

// HasPermission checks if a user has a specific permission based on their ACTUAL roles.
// This is the ONLY way to check permissions - via user.Roles, NOT user.Status.
func (pc *PermissionChecker) HasPermission(user *User, perm Permission) bool {
	if user == nil {
		return false
	}

	// Check each of the user's actual roles
	for _, role := range user.Roles {
		if perms, ok := pc.rolePermissions[string(role)]; ok {
			if perms[perm] {
				return true
			}
		}
	}

	return false
}
