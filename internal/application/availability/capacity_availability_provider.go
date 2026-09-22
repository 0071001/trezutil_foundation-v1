package availability

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type CapacityAvailabilityProvider struct {
	inventoryRepo domain.InventoryRepository
	capacityRepo  domain.CapacityRepository
}

func NewCapacityAvailabilityProvider(
	inventoryRepo domain.InventoryRepository,
	capacityRepo domain.CapacityRepository,
) *CapacityAvailabilityProvider {
	return &CapacityAvailabilityProvider{
		inventoryRepo: inventoryRepo,
		capacityRepo:  capacityRepo,
	}
}

func (p *CapacityAvailabilityProvider) CheckAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if query.ListingID == "" {
		return domain.AvailabilityResult{}, fmt.Errorf("listing_id is required")
	}
	if query.StartAt == 0 || query.EndAt == 0 {
		return domain.AvailabilityResult{}, fmt.Errorf("start_at and end_at are required")
	}
	if query.EndAt <= query.StartAt {
		return domain.AvailabilityResult{}, fmt.Errorf("end_at must be after start_at")
	}

	inventory, err := p.inventoryRepo.FindByListingID(ctx, "", query.ListingID)
	if err != nil {
		return domain.AvailabilityResult{}, fmt.Errorf("inventory lookup failed: %w", err)
	}
	if inventory == nil {
		return domain.AvailabilityResult{
			Available: false,
			Capacity:  0,
			Remaining: 0,
			Reason:    "no inventory configured for listing",
		}, nil
	}

	quantity := int64(query.Quantity)
	if quantity == 0 {
		quantity = 1
	}

	switch inventory.ResourceType {
	case domain.ResourceTypeCountable:
		available := inventory.Available()
		return domain.AvailabilityResult{
			Available: available >= quantity,
			Capacity:  int(available),
			Remaining: int(available),
			StartAt:   query.StartAt,
			EndAt:     query.EndAt,
			Reason:    map[bool]string{true: "", false: "insufficient inventory"}[available >= quantity],
		}, nil

	case domain.ResourceTypeCapacity, domain.ResourceTypeTimeSlot,
		domain.ResourceTypeUnit, domain.ResourceTypeSeat,
		domain.ResourceTypeRoom, domain.ResourceTypeVehicle,
		domain.ResourceTypeAsset:
		units, err := p.capacityRepo.FindByInventoryIDAndRange(ctx, "", inventory.ID, query.StartAt, query.EndAt)
		if err != nil {
			return domain.AvailabilityResult{}, fmt.Errorf("capacity lookup failed: %w", err)
		}

		var totalCapacity, availableCapacity int64
		allOverlap := true
		for _, unit := range units {
			if !unit.Overlaps(query.StartAt, query.EndAt) {
				allOverlap = false
				continue
			}
			totalCapacity += unit.Capacity
			availableCapacity += unit.Available()
		}

		if !allOverlap || len(units) == 0 {
			return domain.AvailabilityResult{
				Available: false,
				Capacity:  0,
				Remaining: 0,
				Reason:    "no capacity units configured for the requested period",
				StartAt:   query.StartAt,
				EndAt:     query.EndAt,
			}, nil
		}

		return domain.AvailabilityResult{
			Available: availableCapacity >= quantity,
			Capacity:  int(totalCapacity),
			Remaining: int(availableCapacity),
			StartAt:   query.StartAt,
			EndAt:     query.EndAt,
			Reason:    map[bool]string{true: "", false: "insufficient capacity"}[availableCapacity >= quantity],
		}, nil

	default:
		return domain.AvailabilityResult{}, fmt.Errorf("unsupported resource type: %s", inventory.ResourceType)
	}
}
