package application

import (
	"context"
	"errors"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// InvitationTTL is how long an invitation stays open.
const InvitationTTL = 14 * 24 * time.Hour

// TeamUseCases manages who works in a business: invitations by phone number and roles. Owners and admins of
// the business manage the team; only an owner gives or takes the admin role; owners are never changed here.
type TeamUseCases struct {
	team  domain.TeamRepository
	orgs  domain.OrganizationRepository
	audit domain.AuditLogger
	now   func() time.Time
}

// NewTeamUseCases creates TeamUseCases.
func NewTeamUseCases(team domain.TeamRepository, orgs domain.OrganizationRepository, audit domain.AuditLogger) *TeamUseCases {
	return &TeamUseCases{team: team, orgs: orgs, audit: audit, now: time.Now}
}

// WithClock sets the clock (tests).
func (u *TeamUseCases) WithClock(now func() time.Time) *TeamUseCases {
	u.now = now
	return u
}

// managerRole returns the actor's role if it may manage the team (owner or admin).
func (u *TeamUseCases) managerRole(ctx context.Context, organizationID, actorID string) (domain.OrganizationRole, error) {
	role, err := u.orgs.GetMemberRole(ctx, organizationID, actorID)
	if err != nil || (role != domain.OrganizationRoleOwner && role != domain.OrganizationRoleAdmin) {
		return "", domain.ErrInsufficientPermissions
	}
	return role, nil
}

func (u *TeamUseCases) record(ctx context.Context, actorID, action, organizationID string, meta map[string]interface{}) {
	if u.audit != nil {
		u.audit.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: action, TargetType: "organization", TargetID: organizationID, Metadata: meta})
	}
}

// Invite invites a phone number with a role. The answer is the same whether or not the number has an account.
func (u *TeamUseCases) Invite(ctx context.Context, actorID, organizationID, phone string, role domain.OrganizationRole) (string, error) {
	actorRole, err := u.managerRole(ctx, organizationID, actorID)
	if err != nil {
		return "", err
	}
	if !domain.InvitableRoles[role] || (role == domain.OrganizationRoleAdmin && actorRole != domain.OrganizationRoleOwner) {
		return "", domain.ErrInvalidRole
	}
	normalized, err := domain.NormalizePhone(phone)
	if err != nil {
		return "", domain.ErrInvalidPhone
	}
	now := u.now()
	id, err := u.team.Invite(ctx, organizationID, normalized, role, actorID, now.Unix(), now.Add(InvitationTTL).Unix())
	if err != nil {
		return "", err
	}
	u.record(ctx, actorID, "organization.member_invited", organizationID, map[string]interface{}{"invitation_id": id, "role": string(role)})
	return id, nil
}

// Invitations lists the open invitations (owners and admins).
func (u *TeamUseCases) Invitations(ctx context.Context, actorID, organizationID string) ([]domain.TeamInvitation, error) {
	if _, err := u.managerRole(ctx, organizationID, actorID); err != nil {
		return nil, err
	}
	return u.team.Invitations(ctx, organizationID, u.now().Unix())
}

// RevokeInvitation closes an open invitation.
func (u *TeamUseCases) RevokeInvitation(ctx context.Context, actorID, organizationID, invitationID string) error {
	if _, err := u.managerRole(ctx, organizationID, actorID); err != nil {
		return err
	}
	if err := u.team.Revoke(ctx, organizationID, invitationID, u.now().Unix()); err != nil {
		return err
	}
	u.record(ctx, actorID, "organization.invitation_revoked", organizationID, map[string]interface{}{"invitation_id": invitationID})
	return nil
}

// ChangeRole gives a member another role. Owners are not changed; only an owner gives or takes admin; nobody
// changes their own role.
func (u *TeamUseCases) ChangeRole(ctx context.Context, actorID, organizationID, userID string, role domain.OrganizationRole) error {
	actorRole, err := u.managerRole(ctx, organizationID, actorID)
	if err != nil {
		return err
	}
	if userID == actorID {
		return domain.ErrOwnerProtected
	}
	current, err := u.orgs.GetMemberRole(ctx, organizationID, userID)
	if err != nil {
		return domain.ErrNotMember
	}
	if current == domain.OrganizationRoleOwner {
		return domain.ErrOwnerProtected
	}
	if !domain.InvitableRoles[role] {
		return domain.ErrInvalidRole
	}
	if (role == domain.OrganizationRoleAdmin || current == domain.OrganizationRoleAdmin) && actorRole != domain.OrganizationRoleOwner {
		return domain.ErrInvalidRole
	}
	if current == role {
		return nil
	}
	if err := u.team.SetMemberRole(ctx, organizationID, userID, current, role); err != nil {
		return err
	}
	u.record(ctx, actorID, "organization.member_role_changed", organizationID, map[string]interface{}{"user_id": userID, "from": string(current), "to": string(role)})
	return nil
}

// AcceptInvitations is called when a phone number signs in: its open invitations become memberships. The
// caller never fails the sign-in on its error; it logs it.
func (u *TeamUseCases) AcceptInvitations(ctx context.Context, userID, phone string) error {
	normalized, err := domain.NormalizePhone(phone)
	if err != nil {
		return nil
	}
	joined, err := u.team.AcceptFor(ctx, userID, normalized, u.now().Unix())
	if err != nil {
		return err
	}
	for _, org := range joined {
		u.record(ctx, userID, "organization.invitation_accepted", org, nil)
	}
	return nil
}

// IsTeamRefusal reports whether err is one of the refusals the team routes answer with a 4xx.
func IsTeamRefusal(err error) bool {
	return errors.Is(err, domain.ErrInvalidRole) || errors.Is(err, domain.ErrOwnerProtected) || errors.Is(err, domain.ErrNotMember) ||
		errors.Is(err, domain.ErrInvitationNotFound) || errors.Is(err, domain.ErrInvalidPhone) || errors.Is(err, domain.ErrAccountState)
}
