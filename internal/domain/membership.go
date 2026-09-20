package domain

import "time"

// OrganizationRole represents a role within an organization.
type OrganizationRole string

const (
	OrganizationRoleOwner      OrganizationRole = "owner"
	OrganizationRoleAdmin      OrganizationRole = "admin"
	OrganizationRoleManager    OrganizationRole = "manager"
	OrganizationRoleStaff      OrganizationRole = "staff"
	OrganizationRoleAccountant OrganizationRole = "accountant"
	OrganizationRoleSales      OrganizationRole = "sales"
	OrganizationRoleOperations OrganizationRole = "operations"
)

// MembershipStatus represents the status of a membership.
type MembershipStatus string

const (
	MembershipStatusActive    MembershipStatus = "active"
	MembershipStatusSuspended MembershipStatus = "suspended"
	MembershipStatusInvited   MembershipStatus = "invited"
)

// OrganizationMember represents a user's membership in an organization.
type OrganizationMember struct {
	UserID         string
	OrganizationID string
	Role           OrganizationRole
	Status         MembershipStatus
	JoinedAt       int64
}

// NewOrganizationMember creates a new OrganizationMember.
func NewOrganizationMember(userID, organizationID string, role OrganizationRole) *OrganizationMember {
	return &OrganizationMember{
		UserID:         userID,
		OrganizationID: organizationID,
		Role:           role,
		Status:         MembershipStatusActive,
		JoinedAt:       time.Now().Unix(),
	}
}
