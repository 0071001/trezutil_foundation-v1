package domain

import (
	"context"
	"errors"
)

// AvailabilityQuery represents a request to check if a listing is available.
type AvailabilityQuery struct {
	// TenantID is the listing's organization, resolved server-side from the
	// listing (never from the client).
	TenantID  string
	ListingID string
	StartAt   int64
	EndAt     int64
	Quantity  int
	PartySize int
	Context   map[string]interface{}
}

// AvailabilityResult represents the result of an availability check.
type AvailabilityResult struct {
	Available bool   `json:"available"`
	Capacity  int    `json:"capacity"`
	Remaining int    `json:"remaining"`
	Reason    string `json:"reason"`
	StartAt   int64  `json:"start_at"`
	EndAt     int64  `json:"end_at"`
	// State is set by external providers and always by the orchestrated
	// provider: availability is only AVAILABLE when the provider says so.
	State AvailabilityState `json:"state,omitempty"`
}

// AvailabilityState is an explicit availability answer.
type AvailabilityState string

const (
	AvailabilityAvailable   AvailabilityState = "AVAILABLE"
	AvailabilityUnavailable AvailabilityState = "UNAVAILABLE"
	// AvailabilityUnknown: the provider could not establish availability
	// (error, timeout, missing or contradictory answer). Never bookable.
	AvailabilityUnknown AvailabilityState = "UNKNOWN"
)

// ErrAvailabilityUnknown is returned with an UNKNOWN result.
var ErrAvailabilityUnknown = errors.New("availability unknown")

// AvailabilityProvider defines the port for checking listing availability.
// Implementations may use internal data, seller schedules, inventory, or external adapters.
type AvailabilityProvider interface {
	CheckAvailability(ctx context.Context, query AvailabilityQuery) (AvailabilityResult, error)
}

// ReservationExistsError is returned when a conflict is detected.
type ReservationExistsError struct {
	ReservationID string
	Message       string
}

func (e *ReservationExistsError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "reservation conflict detected"
}
