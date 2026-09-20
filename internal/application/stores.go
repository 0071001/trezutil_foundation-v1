package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// StoreUseCases handles store-related operations.
type StoreUseCases struct {
	storeRepo   domain.StoreRepository
	orgRepo     domain.OrganizationRepository
	userRepo    domain.UserRepository
	auditLogger domain.AuditLogger
}

// NewStoreUseCases creates a new StoreUseCases.
func NewStoreUseCases(storeRepo domain.StoreRepository, orgRepo domain.OrganizationRepository, userRepo domain.UserRepository, auditLogger domain.AuditLogger) *StoreUseCases {
	return &StoreUseCases{
		storeRepo:   storeRepo,
		orgRepo:     orgRepo,
		userRepo:    userRepo,
		auditLogger: auditLogger,
	}
}

// CreateStore creates a new store.
func (u *StoreUseCases) CreateStore(ctx context.Context, actorID string, store *domain.Store) error {
	// Get organization
	org, err := u.orgRepo.FindByID(ctx, store.OrganizationID)
	if err != nil {
		return err
	}

	// Check if actor is a member with appropriate role
	memberRole, err := u.orgRepo.GetMemberRole(ctx, org.ID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	// Only owner, admin, and manager can create stores
	if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin && memberRole != domain.OrganizationRoleManager {
		return domain.ErrInsufficientPermissions
	}

	// Set timestamps
	store.CreatedAt = time.Now().Unix()
	store.UpdatedAt = time.Now().Unix()

	// Create store
	err = u.storeRepo.Create(ctx, store)
	if err != nil {
		return fmt.Errorf("failed to create store: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "store.created",
			TargetType: "store",
			TargetID:   store.ID,
		})
	}

	return nil
}

// GetStore retrieves a store by ID.
func (u *StoreUseCases) GetStore(ctx context.Context, storeID string) (*domain.Store, error) {
	store, err := u.storeRepo.FindByID(ctx, storeID)
	if err != nil {
		return nil, err
	}
	return store, nil
}

// UpdateStore updates a store.
func (u *StoreUseCases) UpdateStore(ctx context.Context, actorID, storeID string, updates map[string]interface{}) error {
	// Get store
	store, err := u.storeRepo.FindByID(ctx, storeID)
	if err != nil {
		return err
	}

	// Get organization
	org, err := u.orgRepo.FindByID(ctx, store.OrganizationID)
	if err != nil {
		return err
	}

	// Check if actor is a member with appropriate role
	memberRole, err := u.orgRepo.GetMemberRole(ctx, org.ID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	// Only owner, admin, and manager can update stores
	if memberRole != domain.OrganizationRoleOwner && memberRole != domain.OrganizationRoleAdmin && memberRole != domain.OrganizationRoleManager {
		return domain.ErrInsufficientPermissions
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		store.Name = name
	}
	if description, ok := updates["description"].(*string); ok {
		store.Description = description
	}
	if phone, ok := updates["phone"].(string); ok {
		store.Phone = phone
	}
	if email, ok := updates["email"].(*string); ok {
		store.Email = email
	}
	if status, ok := updates["status"].(domain.StoreStatus); ok {
		store.Status = status
	}
	if address, ok := updates["address"].(*string); ok {
		store.Address = address
	}

	store.UpdatedAt = time.Now().Unix()

	err = u.storeRepo.Update(ctx, store)
	if err != nil {
		return fmt.Errorf("failed to update store: %w", err)
	}

	// Audit log
	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "store.updated",
			TargetType: "store",
			TargetID:   storeID,
			Metadata:   updates,
		})
	}

	return nil
}
