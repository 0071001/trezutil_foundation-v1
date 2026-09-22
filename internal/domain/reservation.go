package domain

import (
	"fmt"
)

// ReservationStatus represents the lifecycle state of a reservation.
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "pending"
	ReservationStatusConfirmed ReservationStatus = "confirmed"
	ReservationStatusCompleted ReservationStatus = "completed"
	ReservationStatusCancelled ReservationStatus = "cancelled"
)

// Reservation represents a booking against a listing's availability.
type Reservation struct {
	ID              string
	OrganizationID  string
	CustomerID      string
	ListingID       string
	Status          ReservationStatus
	StartAt         int64
	EndAt           int64
	Quantity        int
	PartySize       int
	PricingSnapshot *PricingSnapshot
	IdempotencyKey  string
	CreatedAt       int64
	UpdatedAt       int64
	Version         int64
}

// NewReservation creates a new Reservation in PENDING status.
func NewReservation(organizationID, customerID, listingID string, startAt, endAt int64, quantity, partySize int, now int64) *Reservation {
	return &Reservation{
		ID:             "",
		OrganizationID: organizationID,
		CustomerID:     customerID,
		ListingID:      listingID,
		Status:         ReservationStatusPending,
		StartAt:        startAt,
		EndAt:          endAt,
		Quantity:       quantity,
		PartySize:      partySize,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
	}
}

// Confirm transitions the reservation from PENDING to CONFIRMED.
// Valid from: PENDING only.
func (r *Reservation) Confirm(now int64) error {
	if r.Status != ReservationStatusPending {
		return fmt.Errorf("cannot confirm reservation from status %s: only pending reservations can be confirmed", r.Status)
	}
	r.Status = ReservationStatusConfirmed
	r.UpdatedAt = now
	r.Version++
	return nil
}

// Complete transitions the reservation from CONFIRMED to COMPLETED.
// Valid from: CONFIRMED only.
func (r *Reservation) Complete(now int64) error {
	if r.Status != ReservationStatusConfirmed {
		return fmt.Errorf("cannot complete reservation from status %s: only confirmed reservations can be completed", r.Status)
	}
	r.Status = ReservationStatusCompleted
	r.UpdatedAt = now
	r.Version++
	return nil
}

// Cancel transitions the reservation to CANCELLED.
// Valid from: PENDING, CONFIRMED.
func (r *Reservation) Cancel(now int64) error {
	switch r.Status {
	case ReservationStatusPending, ReservationStatusConfirmed:
		r.Status = ReservationStatusCancelled
	case ReservationStatusCompleted:
		return fmt.Errorf("cannot cancel reservation in status %s: completed reservations cannot be cancelled", r.Status)
	case ReservationStatusCancelled:
		return fmt.Errorf("cannot cancel reservation in terminal status %s", r.Status)
	default:
		return fmt.Errorf("cannot cancel reservation from status %s: invalid source state", r.Status)
	}
	r.UpdatedAt = now
	r.Version++
	return nil
}

// ValidateTransition validates a status transition without applying it.
func (r *Reservation) ValidateTransition(newStatus ReservationStatus) error {
	current := r.Status

	if current == newStatus {
		return nil
	}

	switch newStatus {
	case ReservationStatusConfirmed:
		if current != ReservationStatusPending {
			return fmt.Errorf("only pending reservations can be confirmed")
		}
	case ReservationStatusCompleted:
		if current != ReservationStatusConfirmed {
			return fmt.Errorf("only confirmed reservations can be completed")
		}
	case ReservationStatusCancelled:
		if current != ReservationStatusPending && current != ReservationStatusConfirmed {
			return fmt.Errorf("only pending or confirmed reservations can be cancelled")
		}
	default:
		return fmt.Errorf("unknown target status: %s", newStatus)
	}

	return nil
}

// IsTerminal returns true if the reservation is in a terminal state.
func (r *Reservation) IsTerminal() bool {
	return r.Status == ReservationStatusCancelled || r.Status == ReservationStatusCompleted
}

// Overlaps returns true if this reservation's period overlaps with the given range.
func (r *Reservation) Overlaps(startAt, endAt int64) bool {
	return r.StartAt < endAt && r.EndAt > startAt
}
