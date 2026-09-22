package domain

import (
	"fmt"
)

// CapacityUnit represents a time-bounded capacity slice for a resource.
type CapacityUnit struct {
	ID          string
	InventoryID string
	StartAt     int64
	EndAt       int64
	Capacity    int64
	Reserved    int64
	Version     int64
	CreatedAt   int64
	UpdatedAt   int64
}

// NewCapacityUnit creates a new CapacityUnit.
func NewCapacityUnit(inventoryID string, startAt, endAt int64, capacity int64, now int64) *CapacityUnit {
	if capacity < 1 {
		capacity = 1
	}
	if endAt <= startAt {
		endAt = startAt + 3600 // default 1 hour
	}
	return &CapacityUnit{
		ID:          "",
		InventoryID: inventoryID,
		StartAt:     startAt,
		EndAt:       endAt,
		Capacity:    capacity,
		Reserved:    0,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Available returns available capacity in this unit.
func (c *CapacityUnit) Available() int64 {
	avail := c.Capacity - c.Reserved
	if avail < 0 {
		return 0
	}
	return avail
}

// Overlaps returns true if this unit overlaps with the given time range.
func (c *CapacityUnit) Overlaps(startAt, endAt int64) bool {
	return c.StartAt < endAt && c.EndAt > startAt
}

// Reserve reserves capacity in this unit.
// Preconditions: quantity > 0, Available() >= quantity
func (c *CapacityUnit) Reserve(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidCapacityQuantity
	}
	if c.Available() < quantity {
		return ErrInsufficientCapacity
	}
	c.Reserved += quantity
	c.Version++
	c.UpdatedAt = now
	return nil
}

// Release releases previously reserved capacity.
// Preconditions: quantity > 0, Reserved >= quantity
func (c *CapacityUnit) Release(quantity int64, now int64) error {
	if quantity <= 0 {
		return ErrInvalidCapacityQuantity
	}
	if c.Reserved < quantity {
		return ErrInvalidCapacityQuantity
	}
	c.Reserved -= quantity
	c.Version++
	c.UpdatedAt = now
	return nil
}

// ValidateInvariants checks all invariants.
func (c *CapacityUnit) ValidateInvariants() error {
	if c.Capacity <= 0 {
		return fmt.Errorf("capacity must be positive: %d", c.Capacity)
	}
	if c.Reserved < 0 {
		return fmt.Errorf("reserved cannot be negative: %d", c.Reserved)
	}
	if c.Reserved > c.Capacity {
		return fmt.Errorf("reserved (%d) exceeds capacity (%d)", c.Reserved, c.Capacity)
	}
	if c.EndAt <= c.StartAt {
		return fmt.Errorf("end_at must be after start_at")
	}
	if c.Version < 1 {
		return fmt.Errorf("version must be >= 1: %d", c.Version)
	}
	return nil
}

// CapacityQuery represents a query for capacity availability.
type CapacityQuery struct {
	InventoryID string
	StartAt     int64
	EndAt       int64
	Quantity    int
}

// CapacityResult represents the result of a capacity check.
type CapacityResult struct {
	Available         bool
	TotalCapacity     int64
	AvailableCapacity int64
	Units             []*CapacityUnit
}
