package availability

import (
	"context"
	"errors"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type CapacityAvailabilityProvider struct {
	reservations  domain.ReservationRepository
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

// WithReservations enables availability for listings without configured
// inventory: such a listing is a single bookable unit, available when no
// active reservation overlaps the requested period (the same invariant the
// reservations exclusion constraint enforces in PostgreSQL).
func (p *CapacityAvailabilityProvider) WithReservations(reservations domain.ReservationRepository) *CapacityAvailabilityProvider {
	p.reservations = reservations
	return p
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

	if query.TenantID == "" {
		return domain.AvailabilityResult{}, fmt.Errorf("tenant is required for availability lookup")
	}

	inventory, err := p.inventoryRepo.FindByListingID(ctx, query.TenantID, query.ListingID)
	if err != nil && !errors.Is(err, domain.ErrInventoryNotFound) {
		return domain.AvailabilityResult{}, fmt.Errorf("inventory lookup failed: %w", err)
	}
	if inventory == nil {
		return p.singleUnitAvailability(ctx, query)
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
		units, err := p.capacityRepo.FindByInventoryIDAndRange(ctx, query.TenantID, inventory.ID, query.StartAt, query.EndAt)
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

func (p *CapacityAvailabilityProvider) singleUnitAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	if p.reservations == nil {
		return domain.AvailabilityResult{
			Available: false,
			Reason:    "no inventory configured for listing",
			StartAt:   query.StartAt,
			EndAt:     query.EndAt,
		}, nil
	}
	overlapping, err := p.reservations.FindByResourceAndPeriod(ctx, query.ListingID, query.StartAt, query.EndAt)
	if err != nil {
		return domain.AvailabilityResult{}, fmt.Errorf("reservation lookup failed: %w", err)
	}
	available := len(overlapping) == 0
	remaining := 0
	if available {
		remaining = 1
	}
	return domain.AvailabilityResult{
		Available: available,
		Capacity:  1,
		Remaining: remaining,
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
		Reason:    map[bool]string{true: "", false: "already booked for the requested period"}[available],
	}, nil
}
