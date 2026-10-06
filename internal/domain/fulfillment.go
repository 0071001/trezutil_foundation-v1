package domain

import (
	"fmt"
)

// FulfillmentType represents the type of fulfillment.
type FulfillmentType string

const (
	FulfillmentTypeDelivery          FulfillmentType = "DELIVERY"
	FulfillmentTypePickup            FulfillmentType = "PICKUP"
	FulfillmentTypeAppointment       FulfillmentType = "APPOINTMENT"
	FulfillmentTypeDigital           FulfillmentType = "DIGITAL"
	FulfillmentTypeEventEntry        FulfillmentType = "EVENT_ENTRY"
	FulfillmentTypeTravelTicket      FulfillmentType = "TRAVEL_TICKET"
	FulfillmentTypeServiceCompletion FulfillmentType = "SERVICE_COMPLETION"
	FulfillmentTypePropertyVisit     FulfillmentType = "PROPERTY_VISIT"
	FulfillmentTypeNone              FulfillmentType = "NONE"
)

// IsValidFulfillmentType checks if a fulfillment type is valid.
func IsValidFulfillmentType(ft FulfillmentType) bool {
	switch ft {
	case FulfillmentTypeDelivery, FulfillmentTypePickup, FulfillmentTypeAppointment,
		FulfillmentTypeDigital, FulfillmentTypeEventEntry, FulfillmentTypeTravelTicket,
		FulfillmentTypeServiceCompletion, FulfillmentTypePropertyVisit, FulfillmentTypeNone:
		return true
	default:
		return false
	}
}

// RequiresTracking returns true if this fulfillment type typically uses tracking.
func (ft FulfillmentType) RequiresTracking() bool {
	return ft == FulfillmentTypeDelivery || ft == FulfillmentTypeTravelTicket
}

// FulfillmentStatus represents the status of a fulfillment.
type FulfillmentStatus string

const (
	FulfillmentStatusPending    FulfillmentStatus = "PENDING"
	FulfillmentStatusAccepted   FulfillmentStatus = "ACCEPTED"
	FulfillmentStatusInProgress FulfillmentStatus = "IN_PROGRESS"
	FulfillmentStatusReady      FulfillmentStatus = "READY"
	FulfillmentStatusCompleted  FulfillmentStatus = "COMPLETED"
	FulfillmentStatusCancelled  FulfillmentStatus = "CANCELLED"
	FulfillmentStatusFailed     FulfillmentStatus = "FAILED"
)

// IsValidFulfillmentStatus checks if a fulfillment status is valid.
func IsValidFulfillmentStatus(fs FulfillmentStatus) bool {
	switch fs {
	case FulfillmentStatusPending, FulfillmentStatusAccepted, FulfillmentStatusInProgress,
		FulfillmentStatusReady, FulfillmentStatusCompleted, FulfillmentStatusCancelled,
		FulfillmentStatusFailed:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the status is terminal.
func (fs FulfillmentStatus) IsTerminal() bool {
	return fs == FulfillmentStatusCompleted || fs == FulfillmentStatusCancelled || fs == FulfillmentStatusFailed
}

// ValidTransitions returns the valid target statuses from the current status.
func (fs FulfillmentStatus) ValidTransitions() []FulfillmentStatus {
	switch fs {
	case FulfillmentStatusPending:
		return []FulfillmentStatus{FulfillmentStatusAccepted, FulfillmentStatusCancelled}
	case FulfillmentStatusAccepted:
		return []FulfillmentStatus{FulfillmentStatusInProgress, FulfillmentStatusCancelled}
	case FulfillmentStatusInProgress:
		return []FulfillmentStatus{FulfillmentStatusReady, FulfillmentStatusFailed}
	case FulfillmentStatusReady:
		return []FulfillmentStatus{FulfillmentStatusCompleted, FulfillmentStatusCancelled, FulfillmentStatusInProgress}
	case FulfillmentStatusCompleted, FulfillmentStatusCancelled, FulfillmentStatusFailed:
		return []FulfillmentStatus{}
	default:
		return []FulfillmentStatus{}
	}
}

// CanTransitionTo checks if transition to target is valid.
func (fs FulfillmentStatus) CanTransitionTo(target FulfillmentStatus) bool {
	for _, t := range fs.ValidTransitions() {
		if t == target {
			return true
		}
	}
	return false
}

// Fulfillment represents the execution/completion of a transaction.
type Fulfillment struct {
	ID            string
	TenantID      string
	OrderID       *string
	ReservationID *string
	Type          FulfillmentType
	Status        FulfillmentStatus
	Reference     string
	TrackingRef   *string
	AssignedTo    *string
	Metadata      map[string]interface{}
	Version       int64
	CreatedAt     int64
	UpdatedAt     int64
}

// referenceStem is the first eight characters of an id (all of it when it is shorter), for readable references.
func referenceStem(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// NewFulfillment creates a new Fulfillment.
func NewFulfillment(tenantID string, orderID, reservationID *string, ftype FulfillmentType, now int64) *Fulfillment {
	if !IsValidFulfillmentType(ftype) {
		ftype = FulfillmentTypeNone
	}

	var ref string
	if orderID != nil {
		ref = "FUL-" + referenceStem(*orderID) + "-" + fmt.Sprintf("%d", now%1000000)
	} else if reservationID != nil {
		ref = "FUL-" + referenceStem(*reservationID) + "-" + fmt.Sprintf("%d", now%1000000)
	} else {
		ref = "FUL-" + fmt.Sprintf("%d", now)
	}

	return &Fulfillment{
		ID:            NewID(),
		TenantID:      tenantID,
		OrderID:       orderID,
		ReservationID: reservationID,
		Type:          ftype,
		Status:        FulfillmentStatusPending,
		Reference:     ref,
		TrackingRef:   nil,
		AssignedTo:    nil,
		Metadata:      make(map[string]interface{}),
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// TransitionTo transitions the fulfillment to a new status.
// Preconditions: Valid transition, actor authorized, version matches.
func (f *Fulfillment) TransitionTo(target FulfillmentStatus, actorID string, now int64) error {
	if !f.Status.CanTransitionTo(target) {
		return ErrInvalidFulfillmentTransition
	}
	if f.Status.IsTerminal() {
		return ErrInvalidFulfillmentTransition
	}

	f.Status = target
	f.Version++
	f.UpdatedAt = now

	// Auto-assign on first acceptance
	if target == FulfillmentStatusAccepted && f.AssignedTo == nil && actorID != "" {
		f.AssignedTo = &actorID
	}

	return nil
}

// Assign assigns the fulfillment to an actor.
// Preconditions: Status == PENDING, actor authorized.
func (f *Fulfillment) Assign(actorID string, now int64) error {
	if f.Status != FulfillmentStatusPending {
		return ErrInvalidFulfillmentTransition
	}
	f.AssignedTo = &actorID
	f.UpdatedAt = now
	return nil
}

// SetTrackingReference sets the external tracking reference.
// Preconditions: Type requires tracking, status allows it.
func (f *Fulfillment) SetTrackingReference(trackingRef string, now int64) error {
	if !f.Type.RequiresTracking() {
		return ErrTrackingNotSupported
	}
	if f.Status == FulfillmentStatusPending || f.Status.IsTerminal() {
		return ErrInvalidFulfillmentTransition
	}
	f.TrackingRef = &trackingRef
	f.UpdatedAt = now
	return nil
}

// ValidateInvariants checks all invariants.
func (f *Fulfillment) ValidateInvariants() error {
	if !IsValidFulfillmentType(f.Type) {
		return fmt.Errorf("invalid fulfillment type: %s", f.Type)
	}
	if !IsValidFulfillmentStatus(f.Status) {
		return fmt.Errorf("invalid fulfillment status: %s", f.Status)
	}
	if (f.OrderID == nil && f.ReservationID == nil) || (f.OrderID != nil && f.ReservationID != nil) {
		return fmt.Errorf("exactly one of order_id or reservation_id must be set")
	}
	if f.Version < 1 {
		return fmt.Errorf("version must be >= 1: %d", f.Version)
	}
	if f.TrackingRef != nil && !f.Type.RequiresTracking() {
		return fmt.Errorf("tracking reference not supported for type %s", f.Type)
	}
	return nil
}
