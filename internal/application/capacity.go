package application

import (
	"context"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// CapacityUseCases handles capacity-related operations.
type CapacityUseCases struct {
	inventoryRepo domain.InventoryRepository
	capacityRepo  domain.CapacityRepository
	listingRepo   domain.ListingRepository
	sellerRepo    domain.SellerRepository
	orgRepo       domain.OrganizationRepository
	userRepo      domain.UserRepository
	auditLogger   domain.AuditLogger
}

func NewCapacityUseCases(
	inventoryRepo domain.InventoryRepository,
	capacityRepo domain.CapacityRepository,
	listingRepo domain.ListingRepository,
	sellerRepo domain.SellerRepository,
	orgRepo domain.OrganizationRepository,
	userRepo domain.UserRepository,
	auditLogger domain.AuditLogger,
) *CapacityUseCases {
	return &CapacityUseCases{
		inventoryRepo: inventoryRepo,
		capacityRepo:  capacityRepo,
		listingRepo:   listingRepo,
		sellerRepo:    sellerRepo,
		orgRepo:       orgRepo,
		userRepo:      userRepo,
		auditLogger:   auditLogger,
	}
}

func (uc *CapacityUseCases) GetCapacityUnits(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error) {
	return uc.capacityRepo.FindByInventoryIDAndRange(ctx, tenantID, inventoryID, startAt, endAt)
}

func (uc *CapacityUseCases) CheckCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int) (domain.CapacityResult, error) {
	inv, err := uc.inventoryRepo.FindByListingID(ctx, tenantID, listingID)
	if err != nil {
		return domain.CapacityResult{}, domain.ErrInventoryNotFound
	}

	units, err := uc.capacityRepo.FindByInventoryIDAndRange(ctx, tenantID, inv.ID, startAt, endAt)
	if err != nil {
		return domain.CapacityResult{}, err
	}

	var totalCapacity, availableCapacity int64
	for _, u := range units {
		totalCapacity += u.Capacity
		if u.Available() < availableCapacity || availableCapacity == 0 {
			availableCapacity = u.Available()
		}
	}

	result := domain.CapacityResult{
		Units:             units,
		TotalCapacity:     totalCapacity,
		AvailableCapacity: availableCapacity,
	}
	if len(units) == 0 || int64(quantity) > availableCapacity {
		result.Available = false
	} else {
		result.Available = true
	}
	return result, nil
}

func (uc *CapacityUseCases) ReserveCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	if quantity <= 0 {
		return domain.ErrInvalidCapacityQuantity
	}

	units, err := uc.capacityRepo.FindByInventoryIDAndRangeForUpdate(ctx, tenantID, listingID, startAt, endAt)
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Available() < quantity {
			return domain.ErrInsufficientCapacity
		}
	}

	now := time.Now().Unix()
	for _, u := range units {
		if err := uc.capacityRepo.Reserve(ctx, u.ID, quantity, u.Version); err != nil {
			return err
		}
		if uc.auditLogger != nil {
			uc.auditLogger.Record(ctx, domain.AuditEntry{
				Action:     "capacity.reserved",
				TargetType: "capacity_unit",
				TargetID:   u.ID,
				Metadata: map[string]interface{}{
					"quantity":       quantity,
					"reservation_id": reservationID,
				},
				OccurredAt: now,
			})
		}
	}
	return nil
}

func (uc *CapacityUseCases) ReleaseCapacity(ctx context.Context, tenantID, listingID string, startAt, endAt int64, quantity int64, reservationID string) error {
	if quantity <= 0 {
		return domain.ErrInvalidCapacityQuantity
	}

	units, err := uc.capacityRepo.FindByInventoryIDAndRangeForUpdate(ctx, tenantID, listingID, startAt, endAt)
	if err != nil {
		return err
	}

	for _, u := range units {
		if u.Reserved < quantity {
			return domain.ErrInvalidCapacityQuantity
		}
	}

	now := time.Now().Unix()
	for _, u := range units {
		if err := uc.capacityRepo.Release(ctx, u.ID, quantity, u.Version); err != nil {
			return err
		}
		if uc.auditLogger != nil {
			uc.auditLogger.Record(ctx, domain.AuditEntry{
				Action:     "capacity.released",
				TargetType: "capacity_unit",
				TargetID:   u.ID,
				Metadata: map[string]interface{}{
					"quantity":       quantity,
					"reservation_id": reservationID,
				},
				OccurredAt: now,
			})
		}
	}
	return nil
}
