package domain

import (
	"fmt"
)

// ResourceType represents the type of inventory resource.
type ResourceType string

const (
	ResourceTypeCountable ResourceType = "COUNTABLE"
	ResourceTypeCapacity  ResourceType = "CAPACITY"
	ResourceTypeTimeSlot  ResourceType = "TIME_SLOT"
	ResourceTypeUnit      ResourceType = "UNIT"
	ResourceTypeSeat      ResourceType = "SEAT"
	ResourceTypeRoom      ResourceType = "ROOM"
	ResourceTypeVehicle   ResourceType = "VEHICLE"
	ResourceTypeAsset     ResourceType = "ASSET"
)

// IsValidResourceType checks if a resource type is valid.
func IsValidResourceType(rt ResourceType) bool {
	switch rt {
	case ResourceTypeCountable, ResourceTypeCapacity, ResourceTypeTimeSlot,
		ResourceTypeUnit, ResourceTypeSeat, ResourceTypeRoom,
		ResourceTypeVehicle, ResourceTypeAsset:
		return true
	default:
		return false
	}
}

// InventoryResource represents a quantifiable resource attached to a listing.
type InventoryResource struct {
	ID           string
	TenantID     string
	ListingID    string
	ResourceType ResourceType
	Total        int64
	Reserved     int64
	Consumed     int64
	Version      int64
	CreatedAt    int64
	UpdatedAt    int64
}

// NewInventoryResource creates a new InventoryResource.
func NewInventoryResource(tenantID, listingID string, resourceType ResourceType, total int64, now int64) *InventoryResource {
	if !IsValidResourceType(resourceType) {
		resourceType = ResourceTypeCountable
	}
	if total < 0 {
		total = 0
	}
	return &InventoryResource{
		ID:           NewID(),
		TenantID:     tenantID,
		ListingID:    listingID,
		ResourceType: resourceType,
		Total:        total,
		Reserved:     0,
		Consumed:     0,
		Version:      1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

// Available returns the currently available quantity.
func (r *InventoryResource) Available() int64 {
	avail := r.Total - r.Reserved - r.Consumed
	if avail < 0 {
		return 0
	}
	return avail
}

// Reserve reserves quantity from available inventory.
// Preconditions: quantity > 0, Available() >= quantity
func (r *InventoryResource) Reserve(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if r.Available() < quantity {
		return ErrInsufficientInventory
	}
	r.Reserved += quantity
	r.Version++
	r.UpdatedAt = now
	return nil
}

// Release releases previously reserved quantity back to available.
// Preconditions: quantity > 0, Reserved >= quantity
func (r *InventoryResource) Release(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if r.Reserved < quantity {
		return ErrInvalidQuantity
	}
	r.Reserved -= quantity
	r.Version++
	r.UpdatedAt = now
	return nil
}

// Consume consumes quantity, moving from reserved to consumed (if from reservation)
// or directly incrementing consumed (if direct consumption).
// Preconditions: quantity > 0, and either Reserved >= quantity or Available() >= quantity
func (r *InventoryResource) Consume(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if r.Reserved >= quantity {
		r.Reserved -= quantity
		r.Consumed += quantity
	} else if r.Available() >= quantity {
		r.Consumed += quantity
	} else {
		return ErrInsufficientInventory
	}
	r.Version++
	r.UpdatedAt = now
	return nil
}

// Restore restores previously consumed quantity back to available.
// Preconditions: quantity > 0, Consumed >= quantity
func (r *InventoryResource) Restore(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if r.Consumed < quantity {
		return ErrInvalidQuantity
	}
	r.Consumed -= quantity
	r.Version++
	r.UpdatedAt = now
	return nil
}

// Adjust atomically adjusts total, reserved, and consumed counters.
// Preconditions: Resulting values must satisfy all invariants.
func (r *InventoryResource) Adjust(deltaTotal, deltaReserved, deltaConsumed int64, now int64) error {
	newTotal := r.Total + deltaTotal
	newReserved := r.Reserved + deltaReserved
	newConsumed := r.Consumed + deltaConsumed

	if newTotal < 0 {
		return ErrInvalidAdjustment
	}
	if newReserved < 0 {
		return ErrInvalidAdjustment
	}
	if newConsumed < 0 {
		return ErrInvalidAdjustment
	}
	if newReserved+newConsumed > newTotal {
		return ErrInvalidAdjustment
	}

	r.Total = newTotal
	r.Reserved = newReserved
	r.Consumed = newConsumed
	r.Version++
	r.UpdatedAt = now
	return nil
}

// ValidateInvariants checks all invariants.
func (r *InventoryResource) ValidateInvariants() error {
	if r.Total < 0 {
		return fmt.Errorf("total cannot be negative: %d", r.Total)
	}
	if r.Reserved < 0 {
		return fmt.Errorf("reserved cannot be negative: %d", r.Reserved)
	}
	if r.Consumed < 0 {
		return fmt.Errorf("consumed cannot be negative: %d", r.Consumed)
	}
	if r.Reserved+r.Consumed > r.Total {
		return fmt.Errorf("reserved + consumed (%d) exceeds total (%d)", r.Reserved+r.Consumed, r.Total)
	}
	if r.Version < 1 {
		return fmt.Errorf("version must be >= 1: %d", r.Version)
	}
	return nil
}
