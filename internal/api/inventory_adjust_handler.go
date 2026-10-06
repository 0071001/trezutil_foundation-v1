package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// MaxStockDelta bounds one restock or write-off.
const MaxStockDelta = 100000

// AdjustInventoryH handles POST /api/v1/inventory/adjust?tenant_id=&listing_id= {"delta_total": 5, "reason": "…"}
// (owner, admin or manager of the business, or platform staff). Only the total changes here: what is reserved and
// sold moves with orders and bookings, never by hand. A total cannot go below what is reserved and sold. The
// change is audited with its reason.
func (h *InventoryHandler) AdjustInventoryH(w http.ResponseWriter, r *http.Request) {
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	tenantID, listingID := r.URL.Query().Get("tenant_id"), r.URL.Query().Get("listing_id")
	if tenantID == "" || listingID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "tenant_id and listing_id are required"}}, http.StatusBadRequest)
		return
	}
	var body struct {
		DeltaTotal     int64  `json:"delta_total"`
		Reason         string `json:"reason"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	if err := readJSONBody(r, &body); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if body.DeltaTotal == 0 || body.DeltaTotal > MaxStockDelta || body.DeltaTotal < -MaxStockDelta {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_QUANTITY", Message: "The change must be a whole number other than zero"}}, http.StatusBadRequest)
		return
	}
	if len(reason) < 3 || len(reason) > 200 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "REASON_REQUIRED", Message: "A reason of 3 to 200 characters is required"}}, http.StatusBadRequest)
		return
	}
	inv, err := h.inventoryUseCases.AdjustInventory(r.Context(), actorID, tenantID, listingID, body.DeltaTotal, 0, 0, reason, body.IdempotencyKey)
	switch {
	case err == nil:
		writeJSON(w, inv, http.StatusOK)
	case errors.Is(err, domain.ErrInventoryNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVENTORY_NOT_FOUND", Message: "This listing has no stock record"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidAdjustment):
		writeJSON(w, ErrorResponse{Error: Error{Code: "STOCK_TOO_LOW", Message: "The total cannot go below what is reserved and sold"}}, http.StatusConflict)
	default:
		writeFailure(w, "INVENTORY_ADJUST_FAILED", err)
	}
}
