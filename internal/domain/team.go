package domain

import (
	"context"
	"fmt"
)

// Team management of a business: invitations by phone number and role changes.
var (
	// ErrInvalidRole: the role is unknown, or cannot be given this way (owner is never given by invitation or by a
	// role change; only an owner gives or takes the admin role).
	ErrInvalidRole = fmt.Errorf("this role cannot be given here")
	// ErrOwnerProtected: an owner's membership, and one's own, are not changed or removed from the workspace.
	ErrOwnerProtected = fmt.Errorf("an owner, or your own membership, cannot be changed here")
	// ErrNotMember: the person is not a member of the business.
	ErrNotMember = fmt.Errorf("not a member of this business")
	// ErrInvitationNotFound: no open invitation with that id in the business.
	ErrInvitationNotFound = fmt.Errorf("invitation not found")
)

// InvitableRoles are the roles an invitation or a role change may give.
var InvitableRoles = map[OrganizationRole]bool{
	OrganizationRoleAdmin: true, OrganizationRoleManager: true, OrganizationRoleStaff: true,
	OrganizationRoleAccountant: true, OrganizationRoleSales: true, OrganizationRoleOperations: true,
}

// TeamRepository stores invitations and changes roles.
type TeamRepository interface {
	// Invite records an open invitation (or replaces the role and expiry of the open one for that phone).
	Invite(ctx context.Context, organizationID, phone string, role OrganizationRole, invitedBy string, now, expiresAt int64) (string, error)
	// Invitations lists the open, unexpired invitations of a business.
	Invitations(ctx context.Context, organizationID string, now int64) ([]TeamInvitation, error)
	// Revoke closes an open invitation of the business; ErrInvitationNotFound when there is none.
	Revoke(ctx context.Context, organizationID, invitationID string, now int64) error
	// AcceptFor turns every open invitation for the phone into a membership of userID (skipping businesses the
	// user already belongs to) and returns the businesses joined.
	AcceptFor(ctx context.Context, userID, phone string, now int64) ([]string, error)
	// SetMemberRole changes a member's role only from the role the caller saw (ErrAccountState otherwise).
	SetMemberRole(ctx context.Context, organizationID, userID string, from, to OrganizationRole) error
}
