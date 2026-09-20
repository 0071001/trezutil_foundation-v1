package domain

import "context"

// AvailabilityQuery represents a request to check if a listing is available.
type AvailabilityQuery struct {
	ListingID string
	StartAt   int64
	EndAt     int64
	Quantity  int
	PartySize int
	Context   map[string]interface{}
}

// AvailabilityResult represents the result of an availability check.
type AvailabilityResult struct {
	Available bool
	Capacity  int
	Remaining int
	Reason    string
	StartAt   int64
	EndAt     int64
}

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
