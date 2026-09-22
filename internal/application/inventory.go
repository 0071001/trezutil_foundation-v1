package application

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// InventoryUseCases handles inventory-related operations.
type InventoryUseCases struct {
	inventoryRepo domain.InventoryRepository
	listingRepo   domain.ListingRepository
	sellerRepo    domain.SellerRepository
	orgRepo       domain.OrganizationRepository
	userRepo      domain.UserRepository
	auditLogger   domain.AuditLogger
}

func NewInventoryUseCases(
	inventoryRepo domain.InventoryRepository,
	listingRepo domain.ListingRepository,
	sellerRepo domain.SellerRepository,
	orgRepo domain.OrganizationRepository,
	userRepo domain.UserRepository,
	auditLogger domain.AuditLogger,
) *InventoryUseCases {
	return &InventoryUseCases{
		inventoryRepo: inventoryRepo,
		listingRepo:   listingRepo,
		sellerRepo:    sellerRepo,
		orgRepo:       orgRepo,
		userRepo:      userRepo,
		auditLogger:   auditLogger,
	}
}

func (uc *InventoryUseCases) GetInventory(ctx context.Context, tenantID, listingID string) (*domain.InventoryResource, error) {
	return uc.inventoryRepo.FindByListingID(ctx, tenantID, listingID)
}

func (uc *InventoryUseCases) GetInventoryByID(ctx context.Context, tenantID, id string) (*domain.InventoryResource, error) {
	return uc.inventoryRepo.FindByID(ctx, tenantID, id)
}

func (uc *InventoryUseCases) ListInventory(ctx context.Context, tenantID string, offset, limit int) ([]*domain.InventoryResource, error) {
	return uc.inventoryRepo.FindByTenantID(ctx, tenantID, offset, limit)
}

func (uc *InventoryUseCases) AdjustInventory(ctx context.Context, actorID, tenantID, listingID string, deltaTotal, deltaReserved, deltaConsumed int64, reason string, idempotencyKey string) (*domain.InventoryResource, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if listingID == "" {
		return nil, fmt.Errorf("listing_id is required")
	}

	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !uc.canAdjustInventory(ctx, actor, tenantID, listingID) {
		return nil, domain.ErrInsufficientPermissions
	}

	inv, err := uc.inventoryRepo.FindByListingID(ctx, tenantID, listingID)
	if err != nil {
		return nil, domain.ErrInventoryNotFound
	}

	now := time.Now().Unix()
	if err := inv.Adjust(deltaTotal, deltaReserved, deltaConsumed, now); err != nil {
		return nil, err
	}

	if err := uc.inventoryRepo.Update(ctx, inv); err != nil {
		return nil, err
	}

	if uc.auditLogger != nil {
		uc.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "inventory.adjusted",
			TargetType: "inventory_resource",
			TargetID:   inv.ID,
			Metadata: map[string]interface{}{
				"delta_total":    deltaTotal,
				"delta_reserved": deltaReserved,
				"delta_consumed": deltaConsumed,
				"reason":         reason,
				"listing_id":     listingID,
			},
		})
	}

	return inv, nil
}

func (uc *InventoryUseCases) canAdjustInventory(ctx context.Context, actor *domain.User, tenantID, listingID string) bool {
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}
	if tenantID != "" {
		role, err := uc.orgRepo.GetMemberRole(ctx, tenantID, actor.ID)
		if err == nil {
			switch role {
			case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager:
				return true
			}
		}
	}
	if listingID != "" {
		seller, err := uc.sellerRepo.FindByUserID(ctx, actor.ID)
		if err == nil && seller != nil {
			listing, err := uc.listingRepo.FindByID(ctx, listingID)
			if err == nil && listing.SellerID == seller.ID {
				return true
			}
		}
	}
	return false
}
