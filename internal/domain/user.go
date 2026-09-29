package domain

import "time"

// User represents a human identity in the system.
type User struct {
	ID          string
	Phone       string
	Email       *string
	DisplayName string
	Status      UserStatus
	CreatedAt   int64
	UpdatedAt   int64
	VerifiedAt  *int64
	Roles       []Role // Loaded separately, not from status
}

// UserStatus represents the status of a user account.
type UserStatus string

const (
	UserStatusActive          UserStatus = "active"
	UserStatusSuspended       UserStatus = "suspended"
	UserStatusPendingVerified UserStatus = "pending_verification"
)

// User statuses - admin/operator are NOT user statuses, they are ROLES
// This is a critical distinction for proper RBAC

func NewUser(phone string, displayName string) *User {
	return &User{
		ID:          NewID(),
		Phone:       phone,
		DisplayName: displayName,
		Status:      UserStatusPendingVerified,
		CreatedAt:   time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
		VerifiedAt:  nil,
		Roles:       []Role{},
	}
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

func (u *User) IsSuspended() bool {
	return u.Status == UserStatusSuspended
}

func (u *User) IsVerified() bool {
	return u.VerifiedAt != nil
}

// HasRole checks if the user has a specific role.
// This is the CORRECT way to check roles - NOT via user.Status
func (u *User) HasRole(role Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasAnyRole checks if the user has any of the specified roles.
func (u *User) HasAnyRole(roles ...Role) bool {
	for _, role := range roles {
		if u.HasRole(role) {
			return true
		}
	}
	return false
}

// HasAllRoles checks if the user has all of the specified roles.
func (u *User) HasAllRoles(roles ...Role) bool {
	for _, role := range roles {
		if !u.HasRole(role) {
			return false
		}
	}
	return true
}
