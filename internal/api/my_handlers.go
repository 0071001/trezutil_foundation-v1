package api

import (
	"context"
	"net/http"
	"strconv"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// CustomerOrderLister lists the authenticated customer's own orders.
type CustomerOrderLister interface {
	FindByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]*domain.Order, error)
}

// CustomerReservationLister lists the authenticated customer's own reservations.
type CustomerReservationLister interface {
	FindByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]*domain.Reservation, error)
}

func pageParams(r *http.Request) (limit, offset int) {
	limit, offset = 20, 0
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	return limit, offset
}

// ListMyOrdersH handles GET /api/v1/orders (the caller's own orders only).
func ListMyOrdersH(orders CustomerOrderLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		limit, offset := pageParams(r)
		items, err := orders.FindByCustomerID(r.Context(), userID, limit, offset)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ORDER_LIST_FAILED", Message: "Could not load orders"}}, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []*domain.Order{}
		}
		writeJSON(w, map[string]interface{}{"items": items, "limit": limit, "offset": offset}, http.StatusOK)
	}
}

// ListMyReservationsH handles GET /api/v1/reservations (the caller's own reservations only).
func ListMyReservationsH(reservations CustomerReservationLister) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		limit, offset := pageParams(r)
		items, err := reservations.FindByCustomerID(r.Context(), userID, limit, offset)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "RESERVATION_LIST_FAILED", Message: "Could not load reservations"}}, http.StatusInternalServerError)
			return
		}
		if items == nil {
			items = []*domain.Reservation{}
		}
		writeJSON(w, map[string]interface{}{"items": items, "limit": limit, "offset": offset}, http.StatusOK)
	}
}
