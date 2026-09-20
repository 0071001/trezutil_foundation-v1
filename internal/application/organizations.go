package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// OrganizationUseCases handles organization-related operations.
type OrganizationUseCases struct {
	orgRepo     domain.OrganizationRepository
	userRepo    domain.UserRepository
	auditLogger domain.AuditLogger
}

// NewOrganizationUseCases creates a new OrganizationUseCases.
func NewOrganizationUseCases(orgRepo domain.OrganizationRepository, userRepo domain.UserRepository, auditLogger domain.AuditLogger) *OrganizationUseCases {
	return &OrganizationUseCases{
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

// CreateOrganization creates a new organization.
func (u *OrganizationUseCases) CreateOrganization(ctx context.Context, actorID string, org *domain.Organization) error {
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	if !actor.HasRole(domain.RoleAdmin) && !actor.HasRole(domain.RoleOperator) {
		return domain.ErrInsufficientPermissions
	}

	org.CreatedAt = time.Now().Unix()
	org.UpdatedAt = time.Now().Unix()

	err = u.orgRepo.Create(ctx, org)
	if err != nil {
		return fmt.Errorf("failed to create organization: %w", err)
	}

	member := domain.NewOrganizationMember(actorID, org.ID, domain.OrganizationRoleOwner)
	err = u.orgRepo.AddMember(ctx, member)
	if err != nil {
		return fmt.Errorf("failed to add creator as owner: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "organization.created",
			TargetType: "organization",
			TargetID:   org.ID,
		})
	}

	return nil
}

// GetOrganization retrieves an organization by ID.
func (u *OrganizationUseCases) GetOrganization(ctx context.Context, organizationID string) (*domain.Organization, error) {
	return u.orgRepo.FindByID(ctx, organizationID)
}

// UpdateOrganization updates an organization.
func (u *OrganizationUseCases) UpdateOrganization(ctx context.Context, actorID, organizationID string, updates map[string]interface{}) error {
	org, err := u.orgRepo.FindByID(ctx, organizationID)
	if err != nil {
		return err
	}

	memberRole, err := u.orgRepo.GetMemberRole(ctx, organizationID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin {
		return domain.ErrInsufficientPermissions
	}

	if legalName, ok := updates["legal_name"].(string); ok {
		org.LegalName = legalName
	}
	if displayName, ok := updates["display_name"].(string); ok {
		org.DisplayName = displayName
	}
	if description, ok := updates["description"].(*string); ok {
		org.Description = description
	}
	if phone, ok := updates["phone"].(string); ok {
		org.Phone = phone
	}
	if email, ok := updates["email"].(*string); ok {
		org.Email = email
	}
	if status, ok := updates["status"].(domain.OrganizationStatus); ok {
		org.Status = status
	}
	if address, ok := updates["address"].(*string); ok {
		org.Address = address
	}

	org.UpdatedAt = time.Now().Unix()

	err = u.orgRepo.Update(ctx, org)
	if err != nil {
		return fmt.Errorf("failed to update organization: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "organization.updated",
			TargetType: "organization",
			TargetID:   organizationID,
			Metadata:   updates,
		})
	}

	return nil
}

// AddMember adds a member to an organization.
func (u *OrganizationUseCases) AddMember(ctx context.Context, actorID, organizationID, userID string, role domain.OrganizationRole) error {
	_, err := u.orgRepo.FindByID(ctx, organizationID)
	if err != nil {
		return err
	}

	memberRole, err := u.orgRepo.GetMemberRole(ctx, organizationID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin {
		return domain.ErrInsufficientPermissions
	}

	_, err = u.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	isMember, err := u.orgRepo.HasMember(ctx, organizationID, userID)
	if err != nil {
		return err
	}
	if isMember {
		return domain.ErrDuplicateMembership
	}

	member := domain.NewOrganizationMember(userID, organizationID, role)
	err = u.orgRepo.AddMember(ctx, member)
	if err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "organization.member_added",
			TargetType: "organization",
			TargetID:   organizationID,
			Metadata: map[string]interface{}{
				"user_id": userID,
				"role":    role,
			},
		})
	}

	return nil
}

// RemoveMember removes a member from an organization.
func (u *OrganizationUseCases) RemoveMember(ctx context.Context, actorID, organizationID, userID string) error {
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	if actor.ID == userID {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, organizationID, userID)
		if err == nil && memberRole == domain.OrganizationRoleOwner {
			return fmt.Errorf("owner cannot remove themselves; transfer ownership first")
		}
	}

	_, err = u.orgRepo.FindByID(ctx, organizationID)
	if err != nil {
		return err
	}

	memberRole, err := u.orgRepo.GetMemberRole(ctx, organizationID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin {
		return domain.ErrInsufficientPermissions
	}

	err = u.orgRepo.RemoveMember(ctx, organizationID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "organization.member_removed",
			TargetType: "organization",
			TargetID:   organizationID,
			Metadata: map[string]interface{}{
				"user_id": userID,
			},
		})
	}

	return nil
}

// GetMembers retrieves all members of an organization.
func (u *OrganizationUseCases) GetMembers(ctx context.Context, organizationID string) ([]domain.OrganizationMember, error) {
	return u.orgRepo.GetMembers(ctx, organizationID)
}
