package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// SellerUseCases handles seller-related operations.
type SellerUseCases struct {
	sellerRepo  domain.SellerRepository
	orgRepo     domain.OrganizationRepository
	userRepo    domain.UserRepository
	auditLogger domain.AuditLogger
}

// NewSellerUseCases creates a new SellerUseCases.
func NewSellerUseCases(sellerRepo domain.SellerRepository, orgRepo domain.OrganizationRepository, userRepo domain.UserRepository, auditLogger domain.AuditLogger) *SellerUseCases {
	return &SellerUseCases{
		sellerRepo:  sellerRepo,
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

// CreateSellerProfile creates a new seller profile.
func (u *SellerUseCases) CreateSellerProfile(ctx context.Context, actorID string, seller *domain.SellerProfile) error {
	// Get actor for permission check
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	// Check if actor has seller role
	if !actor.HasRole(domain.RoleSeller) && !actor.HasRole(domain.RoleAdmin) && !actor.HasRole(domain.RoleOperator) {
		return domain.ErrInsufficientPermissions
	}

	// Validate organization if provided
	if seller.OrganizationID != nil && *seller.OrganizationID != "" {
		org, err := u.orgRepo.FindByID(ctx, *seller.OrganizationID)
		if err != nil {
			return err
		}
		// Check if actor is a member of the organization
		isMember, err := u.orgRepo.HasMember(ctx, org.ID, actorID)
		if err != nil {
			return err
		}
		if !isMember {
			return domain.ErrInsufficientPermissions
		}
	}

	// Server-controlled identity: a seller profile always belongs to the
	// caller; clients cannot attach it to another user.
	seller.ID = domain.NewID()
	seller.UserID = &actorID
	// Trust signals are earned through verification, never self-declared.
	seller.VerificationStatus = domain.VerificationStatusUnverified
	seller.TrustLevel = 0
	if seller.Status == "" {
		seller.Status = domain.SellerStatusActive
	}
	seller.CreatedAt = time.Now().Unix()
	seller.UpdatedAt = time.Now().Unix()

	// Create seller profile
	err = u.sellerRepo.Create(ctx, seller)
	if err != nil {
		return fmt.Errorf("failed to create seller profile: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "seller.created",
			TargetType: "seller",
			TargetID:   seller.ID,
		})
	}

	return nil
}

// GetSellerProfile retrieves a seller profile by ID.
func (u *SellerUseCases) GetSellerProfile(ctx context.Context, sellerID string) (*domain.SellerProfile, error) {
	seller, err := u.sellerRepo.FindByID(ctx, sellerID)
	if err != nil {
		return nil, err
	}
	return seller, nil
}

// UpdateSellerProfile updates a seller profile.
func (u *SellerUseCases) UpdateSellerProfile(ctx context.Context, actorID, sellerID string, updates map[string]interface{}) error {
	// Get actor for permission check
	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return fmt.Errorf("failed to find actor: %w", err)
	}

	// Get seller
	seller, err := u.sellerRepo.FindByID(ctx, sellerID)
	if err != nil {
		return err
	}

	// Check if actor owns this seller or has admin/operator role
	if seller.UserID != nil && *seller.UserID == actorID {
		// Owner can update
	} else if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		// Admin/operator can update
	} else if seller.OrganizationID != nil && *seller.OrganizationID != "" {
		// Check if actor is a member of the organization with appropriate role
		memberRole, err := u.orgRepo.GetMemberRole(ctx, *seller.OrganizationID, actorID)
		if err != nil {
			return domain.ErrInsufficientPermissions
		}
		if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin && memberRole != domain.OrganizationRoleManager {
			return domain.ErrInsufficientPermissions
		}
	} else {
		return domain.ErrInsufficientPermissions
	}

	// Apply updates
	if displayName, ok := updates["display_name"].(string); ok {
		seller.DisplayName = displayName
	}
	if bio, ok := updates["bio"].(*string); ok {
		seller.Bio = bio
	}
	if phone, ok := updates["phone"].(string); ok {
		seller.Phone = phone
	}
	if email, ok := updates["email"].(*string); ok {
		seller.Email = email
	}
	if status, ok := updates["status"].(domain.SellerStatus); ok {
		seller.Status = status
	}

	seller.UpdatedAt = time.Now().Unix()

	err = u.sellerRepo.Update(ctx, seller)
	if err != nil {
		return fmt.Errorf("failed to update seller profile: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "seller.updated",
			TargetType: "seller",
			TargetID:   sellerID,
			Metadata:   updates,
		})
	}

	return nil
}
