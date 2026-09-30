package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/trezutil/trezutil/internal/application/hotel"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// HotelUseCases is the Hotel Commerce Pack (internal/application/hotel).
type HotelUseCases interface {
	CreateRoomType(ctx context.Context, actorID string, in hotel.NewRoomType) (*hotel.RoomTypeView, error)
	UpdateRoomType(ctx context.Context, actorID, listingID string, p hotel.RoomTypePatch) (*hotel.RoomTypeView, error)
	SetUnits(ctx context.Context, actorID, listingID string, units int) (*hotel.RoomTypeView, error)
	SetNightlyCapacity(ctx context.Context, actorID, listingID, from, to string, rooms int) ([]domain.NightCapacity, error)
	NightlyCapacity(ctx context.Context, actorID, listingID, from, to string) ([]domain.NightCapacity, error)
	RoomTypes(ctx context.Context, actorID, organizationID string) ([]hotel.RoomTypeView, error)
	Property(ctx context.Context, storeID string) (*hotel.PropertyView, error)
	Availability(ctx context.Context, storeID string, stay domain.StayDates, guests, rooms int) (*hotel.StayQuote, error)
	CheckoutStay(ctx context.Context, actorID, reservationID string) (*domain.Order, error)
}

// HotelHandler serves /api/v1/hotels/* and stay checkout. Authorization is
// enforced by the use cases (organization membership and role); property and
// availability are public, like search.
type HotelHandler struct{ uc HotelUseCases }

// NewHotelHandler creates a HotelHandler.
func NewHotelHandler(uc HotelUseCases) *HotelHandler { return &HotelHandler{uc: uc} }

func hotelError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientPermissions):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Insufficient permissions"}}, http.StatusForbidden)
	case errors.Is(err, hotel.ErrNotRoomType):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ROOM_TYPE_NOT_FOUND", Message: "Room type not found"}}, http.StatusNotFound)
	case errors.Is(err, hotel.ErrNoHotel):
		writeJSON(w, ErrorResponse{Error: Error{Code: "HOTEL_NOT_FOUND", Message: "No hotel rooms are published at this property"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrReservationNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "RESERVATION_NOT_FOUND", Message: "Reservation not found"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidParameter):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: err.Error()}}, http.StatusBadRequest)
	case errors.Is(err, domain.ErrCapacityConflict):
		writeJSON(w, ErrorResponse{Error: Error{Code: "CAPACITY_CONFLICT", Message: err.Error()}}, http.StatusConflict)
	case errors.Is(err, hotel.ErrStayNotPayable):
		writeJSON(w, ErrorResponse{Error: Error{Code: "STAY_NOT_PAYABLE", Message: err.Error()}}, http.StatusConflict)
	case errors.Is(err, domain.ErrListingUnavailable):
		writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_UNAVAILABLE", Message: "This room type is not available"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrOptimisticLock), errors.Is(err, domain.ErrIdempotencyConflict):
		writeJSON(w, ErrorResponse{Error: Error{Code: "CONFLICT", Message: "Changed concurrently, please retry"}}, http.StatusConflict)
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: "HOTEL_REQUEST_FAILED", Message: "The hotel request could not be completed"}}, http.StatusInternalServerError)
	}
}

func signedIn(w http.ResponseWriter, r *http.Request) (string, bool) {
	actor := auth.UserID(r.Context())
	if actor == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "UNAUTHENTICATED", Message: "Sign in required"}}, http.StatusUnauthorized)
		return "", false
	}
	return actor, true
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: msg}}, http.StatusBadRequest)
}

// CreateRoomTypeH handles POST /api/v1/hotels/room-types.
func (h *HotelHandler) CreateRoomTypeH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	var in hotel.NewRoomType
	if err := readJSONBody(r, &in); err != nil {
		badRequest(w, "Invalid request format")
		return
	}
	rt, err := h.uc.CreateRoomType(r.Context(), actor, in)
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, rt, http.StatusCreated)
}

// ListRoomTypesH handles GET /api/v1/hotels/room-types?organization_id=.
func (h *HotelHandler) ListRoomTypesH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	items, err := h.uc.RoomTypes(r.Context(), actor, r.URL.Query().Get("organization_id"))
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

// UpdateRoomTypeH handles POST /api/v1/hotels/room-types/{listing_id}.
func (h *HotelHandler) UpdateRoomTypeH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	var p hotel.RoomTypePatch
	if err := readJSONBody(r, &p); err != nil {
		badRequest(w, "Invalid request format")
		return
	}
	rt, err := h.uc.UpdateRoomType(r.Context(), actor, r.PathValue("listing_id"), p)
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, rt, http.StatusOK)
}

// SetUnitsH handles POST /api/v1/hotels/room-types/{listing_id}/units with {"units": 10}.
func (h *HotelHandler) SetUnitsH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	var body struct {
		Units int `json:"units"`
	}
	if err := readJSONBody(r, &body); err != nil {
		badRequest(w, "Invalid request format")
		return
	}
	rt, err := h.uc.SetUnits(r.Context(), actor, r.PathValue("listing_id"), body.Units)
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, rt, http.StatusOK)
}

// SetCapacityH handles POST /api/v1/hotels/room-types/{listing_id}/capacity
// with {"from":"2026-10-10","to":"2026-10-20","rooms":8}.
func (h *HotelHandler) SetCapacityH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	var body struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Rooms *int   `json:"rooms"`
	}
	if err := readJSONBody(r, &body); err != nil || body.Rooms == nil {
		badRequest(w, "from, to and rooms are required")
		return
	}
	nights, err := h.uc.SetNightlyCapacity(r.Context(), actor, r.PathValue("listing_id"), body.From, body.To, *body.Rooms)
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"nights": nights}, http.StatusOK)
}

// CapacityH handles GET /api/v1/hotels/room-types/{listing_id}/capacity?from=&to=.
func (h *HotelHandler) CapacityH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	nights, err := h.uc.NightlyCapacity(r.Context(), actor, r.PathValue("listing_id"), q.Get("from"), q.Get("to"))
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"nights": nights}, http.StatusOK)
}

// PropertyH handles GET /api/v1/hotels/property?store_id= (public).
func (h *HotelHandler) PropertyH(w http.ResponseWriter, r *http.Request) {
	p, err := h.uc.Property(r.Context(), r.URL.Query().Get("store_id"))
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, p, http.StatusOK)
}

// AvailabilityH handles GET /api/v1/hotels/availability?store_id=&check_in=&check_out=&guests=&rooms= (public).
func (h *HotelHandler) AvailabilityH(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	stay, err := domain.ParseStay(q.Get("check_in"), q.Get("check_out"))
	if err != nil {
		hotelError(w, err)
		return
	}
	guests, err := strconv.Atoi(q.Get("guests"))
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: "guests must be a whole number"}}, http.StatusBadRequest)
		return
	}
	rooms := 0
	if v := q.Get("rooms"); v != "" {
		if rooms, err = strconv.Atoi(v); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PARAMETER", Message: "rooms must be a whole number"}}, http.StatusBadRequest)
			return
		}
	}
	quote, err := h.uc.Availability(r.Context(), q.Get("store_id"), stay, guests, rooms)
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, quote, http.StatusOK)
}

// CheckoutStayH handles POST /api/v1/reservations/{reservation_id}/checkout:
// the order that pays a hotel stay (created once; pay it with POST /payments).
func (h *HotelHandler) CheckoutStayH(w http.ResponseWriter, r *http.Request) {
	actor, ok := signedIn(w, r)
	if !ok {
		return
	}
	order, err := h.uc.CheckoutStay(r.Context(), actor, r.PathValue("reservation_id"))
	if err != nil {
		hotelError(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{
		"order_id": order.ID, "status": string(order.Status), "total": order.TotalAmount, "currency": order.Currency,
		"organization_id": order.OrganizationID, "reservation_id": r.PathValue("reservation_id"),
	}, http.StatusCreated)
}
