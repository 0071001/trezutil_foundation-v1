package commerce

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type CreateReservationHandler struct {
	reservationUseCases ReservationUseCases
}

func NewCreateReservationHandler(reservationUseCases ReservationUseCases) *CreateReservationHandler {
	return &CreateReservationHandler{reservationUseCases: reservationUseCases}
}

func (h *CreateReservationHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrListingNotFound
	}

	startAtRaw, ok := parameters["start_at"]
	if !ok {
		return nil, fmt.Errorf("start_at is required")
	}
	startAt, err := parseInt64(startAtRaw)
	if err != nil {
		return nil, fmt.Errorf("start_at must be a valid integer timestamp: %w", err)
	}

	endAtRaw, ok := parameters["end_at"]
	if !ok {
		return nil, fmt.Errorf("end_at is required")
	}
	endAt, err := parseInt64(endAtRaw)
	if err != nil {
		return nil, fmt.Errorf("end_at must be a valid integer timestamp: %w", err)
	}

	quantity := 1
	if v, ok := intParam(parameters, "quantity"); ok && v > 0 {
		quantity = v
	}

	partySize := 0
	if v, ok := intParam(parameters, "party_size"); ok && v >= 0 {
		partySize = v
	}

	// The server's effect key comes first: a parameter written by a model or a client never replaces it.
	idempotencyKey := domain.EffectKey(ctx)
	if v, ok := parameters["idempotency_key"].(string); ok && idempotencyKey == "" {
		idempotencyKey = v
	}

	reservation, err := h.reservationUseCases.CreateReservation(ctx, actor.ID, listingID, startAt, endAt, quantity, partySize, idempotencyKey)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"reservation_id": reservation.ID,
		"status":         string(reservation.Status),
		"start_at":       reservation.StartAt,
		"end_at":         reservation.EndAt,
		"quantity":       reservation.Quantity,
		"party_size":     reservation.PartySize,
		"customer_id":    reservation.CustomerID,
		"listing_id":     reservation.ListingID,
	}, nil
}

func (h *CreateReservationHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCreateReservation
}

type GetReservationHandler struct {
	reservationUseCases ReservationUseCases
}

func NewGetReservationHandler(reservationUseCases ReservationUseCases) *GetReservationHandler {
	return &GetReservationHandler{reservationUseCases: reservationUseCases}
}

func (h *GetReservationHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	reservationID, ok := parameters["reservation_id"].(string)
	if !ok || reservationID == "" {
		return nil, domain.ErrReservationNotFound
	}

	reservation, err := h.reservationUseCases.GetReservation(ctx, actor.ID, reservationID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"reservation_id": reservation.ID,
		"status":         string(reservation.Status),
		"start_at":       reservation.StartAt,
		"end_at":         reservation.EndAt,
		"quantity":       reservation.Quantity,
		"party_size":     reservation.PartySize,
		"customer_id":    reservation.CustomerID,
		"listing_id":     reservation.ListingID,
		"created_at":     reservation.CreatedAt,
		"updated_at":     reservation.UpdatedAt,
	}, nil
}

func (h *GetReservationHandler) ActionType() domain.CommerceActionType {
	return domain.ActionGetReservation
}

type CancelReservationHandler struct {
	reservationUseCases ReservationUseCases
}

func NewCancelReservationHandler(reservationUseCases ReservationUseCases) *CancelReservationHandler {
	return &CancelReservationHandler{reservationUseCases: reservationUseCases}
}

func (h *CancelReservationHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	reservationID, ok := parameters["reservation_id"].(string)
	if !ok || reservationID == "" {
		return nil, domain.ErrReservationNotFound
	}

	err := h.reservationUseCases.CancelReservation(ctx, actor.ID, reservationID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"reservation_id": reservationID,
		"status":         "cancelled",
	}, nil
}

func (h *CancelReservationHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCancelReservation
}

type CheckAvailabilityHandler struct {
	availabilityUseCases AvailabilityUseCases
}

func NewCheckAvailabilityHandler(availabilityUseCases AvailabilityUseCases) *CheckAvailabilityHandler {
	return &CheckAvailabilityHandler{availabilityUseCases: availabilityUseCases}
}

func (h *CheckAvailabilityHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, fmt.Errorf("listing_id is required")
	}

	startAtRaw, ok := parameters["start_at"]
	if !ok {
		return nil, fmt.Errorf("start_at is required")
	}
	startAt, err := parseInt64(startAtRaw)
	if err != nil {
		return nil, fmt.Errorf("start_at must be a valid integer timestamp: %w", err)
	}

	endAtRaw, ok := parameters["end_at"]
	if !ok {
		return nil, fmt.Errorf("end_at is required")
	}
	endAt, err := parseInt64(endAtRaw)
	if err != nil {
		return nil, fmt.Errorf("end_at must be a valid integer timestamp: %w", err)
	}

	quantity := 1
	if v, ok := intParam(parameters, "quantity"); ok && v > 0 {
		quantity = v
	}

	partySize := 0
	if v, ok := intParam(parameters, "party_size"); ok && v >= 0 {
		partySize = v
	}

	result, err := h.availabilityUseCases.CheckAvailability(ctx, actor.ID, domain.AvailabilityQuery{
		ListingID: listingID,
		StartAt:   startAt,
		EndAt:     endAt,
		Quantity:  quantity,
		PartySize: partySize,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"available": result.Available,
		"capacity":  result.Capacity,
		"remaining": result.Remaining,
		"reason":    result.Reason,
		"start_at":  result.StartAt,
		"end_at":    result.EndAt,
	}, nil
}

func (h *CheckAvailabilityHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCheckAvailability
}

func parseInt64(v interface{}) (int64, error) {
	switch val := v.(type) {
	case int:
		return int64(val), nil
	case int64:
		return val, nil
	case float64:
		return int64(val), nil
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(val), 10, 64); err == nil {
			return n, nil
		}
		// Human dates from conversations: "2026-10-01", "2026-10-01 14:00", RFC 3339.
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02T15:04", "2006-01-02"} {
			if t, err := time.Parse(layout, strings.TrimSpace(val)); err == nil {
				return t.Unix(), nil
			}
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(val), 64)
		if err != nil {
			return 0, err
		}
		return int64(f), nil
	default:
		return 0, fmt.Errorf("cannot parse %T as int64", v)
	}
}

// intParam reads an integer parameter regardless of how it was encoded:
// Go int (internal callers), float64 (JSON) or string (AI / conversation
// entities). It returns ok=false when the parameter is absent or not numeric.
func intParam(parameters map[string]interface{}, key string) (int, bool) {
	v, present := parameters[key]
	if !present || v == nil {
		return 0, false
	}
	n, err := parseInt64(v)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

func parseTime(v interface{}) (time.Time, error) {
	ts, err := parseInt64(v)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(ts, 0), nil
}
