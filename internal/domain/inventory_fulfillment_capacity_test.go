package domain

import (
	"testing"
)

func TestInventoryResource_Reserve(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)

	// Reserve available quantity
	err := inv.Reserve(5, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Reserved != 5 {
		t.Errorf("expected reserved=5, got %d", inv.Reserved)
	}
	if inv.Available() != 5 {
		t.Errorf("expected available=5, got %d", inv.Available())
	}

	// Try to reserve more than available
	err = inv.Reserve(10, 1002)
	if err != ErrInsufficientInventory {
		t.Errorf("expected ErrInsufficientInventory, got %v", err)
	}

	// Try invalid quantity
	err = inv.Reserve(-1, 1003)
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestInventoryResource_Release(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)
	inv.Reserve(5, 1001)

	// Release reserved quantity
	err := inv.Release(3, 1002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Reserved != 2 {
		t.Errorf("expected reserved=2, got %d", inv.Reserved)
	}

	// Try to release more than reserved
	err = inv.Release(10, 1003)
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestInventoryResource_Consume(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)

	// Consume from available (no reservation)
	err := inv.Consume(3, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Consumed != 3 {
		t.Errorf("expected consumed=3, got %d", inv.Consumed)
	}
	if inv.Available() != 7 {
		t.Errorf("expected available=7, got %d", inv.Available())
	}

	// Reserve then consume from reserved
	inv2 := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)
	inv2.Reserve(5, 1001)

	err = inv2.Consume(3, 1002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv2.Reserved != 2 {
		t.Errorf("expected reserved=2, got %d", inv2.Reserved)
	}
	if inv2.Consumed != 3 {
		t.Errorf("expected consumed=3, got %d", inv2.Consumed)
	}

	// Try to consume more than available
	inv3 := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 2, 1000)
	err = inv3.Consume(5, 1001)
	if err != ErrInsufficientInventory {
		t.Errorf("expected ErrInsufficientInventory, got %v", err)
	}
}

func TestInventoryResource_Restore(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)
	inv.Consume(5, 1001)

	// Restore consumed quantity
	err := inv.Restore(3, 1002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Consumed != 2 {
		t.Errorf("expected consumed=2, got %d", inv.Consumed)
	}
	if inv.Available() != 8 {
		t.Errorf("expected available=8, got %d", inv.Available())
	}

	// Try to restore more than consumed
	err = inv.Restore(10, 1003)
	if err != ErrInvalidQuantity {
		t.Errorf("expected ErrInvalidQuantity, got %v", err)
	}
}

func TestInventoryResource_Adjust(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)

	// Valid adjustment
	err := inv.Adjust(5, 0, 0, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inv.Total != 15 {
		t.Errorf("expected total=15, got %d", inv.Total)
	}

	// Invalid: would make reserved + consumed > total
	// Total is 15, Reserved is 5, Consumed is 0
	inv.Reserve(5, 1002)
	err = inv.Adjust(-12, 0, 0, 1003)
	if err != ErrInvalidAdjustment {
		t.Errorf("expected ErrInvalidAdjustment, got %v", err)
	}

	// Invalid: negative total
	err = inv.Adjust(-20, 0, 0, 1004)
	if err != ErrInvalidAdjustment {
		t.Errorf("expected ErrInvalidAdjustment, got %v", err)
	}

	// Invalid: negative reserved
	err = inv.Adjust(0, -10, 0, 1005)
	if err != ErrInvalidAdjustment {
		t.Errorf("expected ErrInvalidAdjustment, got %v", err)
	}
}

func TestInventoryResource_ValidateInvariants(t *testing.T) {
	// Valid
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)
	if err := inv.ValidateInvariants(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Invalid: negative total
	inv.Total = -1
	if err := inv.ValidateInvariants(); err == nil {
		t.Error("expected error for negative total")
	}

	// Invalid: reserved + consumed > total
	inv.Total = 10
	inv.Reserved = 5
	inv.Consumed = 10
	if err := inv.ValidateInvariants(); err == nil {
		t.Error("expected error for reserved+consumed > total")
	}
}

func TestInventoryResource_Available(t *testing.T) {
	inv := NewInventoryResource("tenant-1", "listing-1", ResourceTypeCountable, 10, 1000)
	if inv.Available() != 10 {
		t.Errorf("expected available=10, got %d", inv.Available())
	}

	inv.Reserve(5, 1001)
	if inv.Available() != 5 {
		t.Errorf("expected available=5, got %d", inv.Available())
	}

	// Consume 3 from reserved -> Reserved=2, Consumed=3, Available=10-2-3=5
	inv.Consume(3, 1002)
	if inv.Available() != 5 {
		t.Errorf("expected available=5 after consume from reserved, got %d", inv.Available())
	}

	// Consume 2 more from reserved -> Reserved=0, Consumed=5, Available=10-0-5=5
	inv.Consume(2, 1003)
	if inv.Available() != 5 {
		t.Errorf("expected available=5, got %d", inv.Available())
	}
}

func TestCapacityUnit_Reserve(t *testing.T) {
	unit := NewCapacityUnit("inv-1", 1000, 2000, 10, 1000)

	err := unit.Reserve(5, 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unit.Reserved != 5 {
		t.Errorf("expected reserved=5, got %d", unit.Reserved)
	}
	if unit.Available() != 5 {
		t.Errorf("expected available=5, got %d", unit.Available())
	}

	// Try to reserve more than capacity
	err = unit.Reserve(10, 1002)
	if err != ErrInsufficientCapacity {
		t.Errorf("expected ErrInsufficientCapacity, got %v", err)
	}

	// Try invalid quantity
	err = unit.Reserve(-1, 1003)
	if err != ErrInvalidCapacityQuantity {
		t.Errorf("expected ErrInvalidCapacityQuantity, got %v", err)
	}
}

func TestCapacityUnit_Release(t *testing.T) {
	unit := NewCapacityUnit("inv-1", 1000, 2000, 10, 1000)
	unit.Reserve(5, 1001)

	err := unit.Release(3, 1002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if unit.Reserved != 2 {
		t.Errorf("expected reserved=2, got %d", unit.Reserved)
	}

	// Try to release more than reserved
	err = unit.Release(10, 1003)
	if err != ErrInvalidCapacityQuantity {
		t.Errorf("expected ErrInvalidCapacityQuantity, got %v", err)
	}
}

func TestCapacityUnit_ValidateInvariants(t *testing.T) {
	// Valid
	unit := NewCapacityUnit("inv-1", 1000, 2000, 10, 1000)
	if err := unit.ValidateInvariants(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Invalid: reserved > capacity
	unit.Reserved = 15
	if err := unit.ValidateInvariants(); err == nil {
		t.Error("expected error for reserved > capacity")
	}

	// Invalid: capacity <= 0
	unit2 := NewCapacityUnit("inv-2", 1000, 2000, -1, 1000)
	if unit2.Capacity != 1 {
		t.Errorf("expected capacity=1 for negative input, got %d", unit2.Capacity)
	}
}

func TestFulfillment_TransitionTo(t *testing.T) {
	orderID := "order-1234"
	f := NewFulfillment("tenant-1", &orderID, nil, FulfillmentTypeDelivery, 1000)

	// PENDING -> ACCEPTED (valid)
	err := f.TransitionTo(FulfillmentStatusAccepted, "actor-1", 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.Status != FulfillmentStatusAccepted {
		t.Errorf("expected ACCEPTED, got %s", f.Status)
	}
	if f.AssignedTo == nil || *f.AssignedTo != "actor-1" {
		t.Error("expected auto-assignment on ACCEPTED")
	}

	// ACCEPTED -> IN_PROGRESS (valid)
	err = f.TransitionTo(FulfillmentStatusInProgress, "actor-1", 1002)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// IN_PROGRESS -> READY (valid)
	err = f.TransitionTo(FulfillmentStatusReady, "actor-1", 1003)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// READY -> COMPLETED (valid)
	err = f.TransitionTo(FulfillmentStatusCompleted, "actor-1", 1004)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// COMPLETED -> IN_PROGRESS (invalid - terminal)
	err = f.TransitionTo(FulfillmentStatusInProgress, "actor-1", 1005)
	if err != ErrInvalidFulfillmentTransition {
		t.Errorf("expected ErrInvalidFulfillmentTransition, got %v", err)
	}
}

func TestFulfillment_TransitionFromPending(t *testing.T) {
	orderID := "order-1234"
	f := NewFulfillment("tenant-1", &orderID, nil, FulfillmentTypeDelivery, 1000)

	// PENDING -> ACCEPTED (valid)
	err := f.TransitionTo(FulfillmentStatusAccepted, "actor-1", 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Already accepted, try to go PENDING -> ACCEPTED again (invalid)
	orderID2 := "order-5678"
	f2 := NewFulfillment("tenant-1", &orderID2, nil, FulfillmentTypeDelivery, 1000)
	f2.Status = FulfillmentStatusAccepted
	err = f2.TransitionTo(FulfillmentStatusPending, "actor-1", 1002)
	// ACCEPTED -> PENDING is not in ValidTransitions, so should fail
	if err == nil {
		t.Error("expected error for ACCEPTED -> PENDING transition")
	}
}

func TestFulfillment_Assign(t *testing.T) {
	orderID := "order-1234"
	f := NewFulfillment("tenant-1", &orderID, nil, FulfillmentTypeDelivery, 1000)

	// Can assign when PENDING
	err := f.Assign("actor-1", 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.AssignedTo == nil || *f.AssignedTo != "actor-1" {
		t.Errorf("expected assigned_to=actor-1, got nil")
	}

	// Cannot assign when not PENDING
	f.Status = FulfillmentStatusAccepted
	err = f.Assign("actor-2", 1002)
	if err != ErrInvalidFulfillmentTransition {
		t.Errorf("expected ErrInvalidFulfillmentTransition, got %v", err)
	}
}

func TestFulfillment_SetTrackingReference(t *testing.T) {
	// Delivery type allows tracking
	orderID1 := "order-aaaa"
	f := NewFulfillment("tenant-1", &orderID1, nil, FulfillmentTypeDelivery, 1000)
	f.Status = FulfillmentStatusAccepted

	err := f.SetTrackingReference("tracking-123", 1001)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.TrackingRef == nil || *f.TrackingRef != "tracking-123" {
		t.Errorf("expected tracking_ref=tracking-123, got nil")
	}

	// Digital type does not allow tracking
	orderID2 := "order-bbbb"
	f2 := NewFulfillment("tenant-1", &orderID2, nil, FulfillmentTypeDigital, 1000)
	f2.Status = FulfillmentStatusAccepted
	err = f2.SetTrackingReference("tracking-456", 1001)
	if err != ErrTrackingNotSupported {
		t.Errorf("expected ErrTrackingNotSupported, got %v", err)
	}

	// Cannot set tracking when PENDING
	orderID3 := "order-cccc"
	f3 := NewFulfillment("tenant-1", &orderID3, nil, FulfillmentTypeDelivery, 1000)
	err = f3.SetTrackingReference("tracking-789", 1001)
	if err != ErrInvalidFulfillmentTransition {
		t.Errorf("expected ErrInvalidFulfillmentTransition, got %v", err)
	}

	// Cannot set tracking when COMPLETED
	orderID4 := "order-dddd"
	f4 := NewFulfillment("tenant-1", &orderID4, nil, FulfillmentTypeDelivery, 1000)
	f4.Status = FulfillmentStatusCompleted
	err = f4.SetTrackingReference("tracking-000", 1001)
	if err != ErrInvalidFulfillmentTransition {
		t.Errorf("expected ErrInvalidFulfillmentTransition, got %v", err)
	}
}

func TestFulfillment_ValidateInvariants(t *testing.T) {
	// Valid
	orderID1 := "order-aaaa"
	f := NewFulfillment("tenant-1", &orderID1, nil, FulfillmentTypeDelivery, 1000)
	if err := f.ValidateInvariants(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Invalid: both order_id and reservation_id set
	orderID2 := "order-bbbb"
	resID2 := "res-bbbb"
	f2 := NewFulfillment("tenant-1", &orderID2, &resID2, FulfillmentTypeDelivery, 1000)
	if err := f2.ValidateInvariants(); err == nil {
		t.Error("expected error for both order_id and reservation_id set")
	}

	// Invalid: neither set
	f3 := NewFulfillment("tenant-1", nil, nil, FulfillmentTypeDelivery, 1000)
	if err := f3.ValidateInvariants(); err == nil {
		t.Error("expected error for neither order_id nor reservation_id set")
	}
}

func TestFulfillment_AvailableTransitions(t *testing.T) {
	// PENDING can go to ACCEPTED or CANCELLED
	tests := []struct {
		from      FulfillmentStatus
		validTo   []FulfillmentStatus
		invalidTo []FulfillmentStatus
	}{
		{
			from:      FulfillmentStatusPending,
			validTo:   []FulfillmentStatus{FulfillmentStatusAccepted, FulfillmentStatusCancelled},
			invalidTo: []FulfillmentStatus{FulfillmentStatusInProgress, FulfillmentStatusCompleted, FulfillmentStatusFailed},
		},
		{
			from:      FulfillmentStatusAccepted,
			validTo:   []FulfillmentStatus{FulfillmentStatusInProgress, FulfillmentStatusCancelled},
			invalidTo: []FulfillmentStatus{FulfillmentStatusReady, FulfillmentStatusCompleted},
		},
		{
			from:      FulfillmentStatusInProgress,
			validTo:   []FulfillmentStatus{FulfillmentStatusReady, FulfillmentStatusFailed},
			invalidTo: []FulfillmentStatus{FulfillmentStatusAccepted, FulfillmentStatusCompleted},
		},
		{
			from:      FulfillmentStatusReady,
			validTo:   []FulfillmentStatus{FulfillmentStatusCompleted, FulfillmentStatusCancelled, FulfillmentStatusInProgress},
			invalidTo: []FulfillmentStatus{FulfillmentStatusAccepted, FulfillmentStatusFailed},
		},
	}

	for _, tt := range tests {
		for _, target := range tt.validTo {
			if !tt.from.CanTransitionTo(target) {
				t.Errorf("expected %s -> %s to be valid", tt.from, target)
			}
		}
		for _, target := range tt.invalidTo {
			if tt.from.CanTransitionTo(target) {
				t.Errorf("expected %s -> %s to be invalid", tt.from, target)
			}
		}
	}
}

func TestCapacityUnit_Overlaps(t *testing.T) {
	unit := NewCapacityUnit("inv-1", 1000, 2000, 10, 1000)

	// Overlapping ranges
	if !unit.Overlaps(1500, 2500) {
		t.Error("expected overlap for (1500, 2500)")
	}
	if !unit.Overlaps(1500, 1700) {
		t.Error("expected overlap for (1500, 1700)")
	}
	if !unit.Overlaps(500, 2500) {
		t.Error("expected overlap for (500, 2500)")
	}

	// Non-overlapping ranges
	if unit.Overlaps(0, 1000) {
		t.Error("did not expect overlap for (0, 1000)")
	}
	if unit.Overlaps(2000, 3000) {
		t.Error("did not expect overlap for (2000, 3000)")
	}
	if unit.Overlaps(3000, 4000) {
		t.Error("did not expect overlap for (3000, 4000)")
	}
}
