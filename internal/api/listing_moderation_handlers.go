package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// ListingModeration is what the listing lifecycle routes beyond publish, pause and resume need
// (application.ListingUseCases).
type ListingModeration interface {
	CloseListing(ctx context.Context, actorID, listingID string) error
	SuspendListing(ctx context.Context, actorID, listingID, reason string) error
	ReinstateListing(ctx context.Context, actorID, listingID, reason string) error
}

// writeListingStepFailure maps the refusals of a listing step to their HTTP answers.
func writeListingStepFailure(w http.ResponseWriter, code string, err error) {
	switch {
	case errors.Is(err, domain.ErrListingNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "LISTING_NOT_FOUND", Message: "Listing not found"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrInvalidStatusTransition):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "The listing is not in a status that allows this step"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrReasonRequired):
		writeJSON(w, ErrorResponse{Error: Error{Code: "REASON_REQUIRED", Message: err.Error()}}, http.StatusBadRequest)
	default:
		writeFailure(w, code, err)
	}
}

// CloseListingH handles POST /v1/listings/close?listing_id= (the seller side, or platform staff).
func CloseListingH(uc ListingModeration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		listingID := r.URL.Query().Get("listing_id")
		if listingID == "" {
			writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "listing_id is required"}}, http.StatusBadRequest)
			return
		}
		if err := uc.CloseListing(r.Context(), actorID, listingID); err != nil {
			writeListingStepFailure(w, "LISTING_CLOSE_FAILED", err)
			return
		}
		writeJSON(w, map[string]string{"status": string(domain.ListingStatusClosed)}, http.StatusOK)
	}
}

// moderationStep handles POST /v1/listings/suspend|reinstate?listing_id= {"reason": "…"} (platform staff).
func moderationStep(code, status string, step func(ctx context.Context, actorID, listingID, reason string) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		listingID := r.URL.Query().Get("listing_id")
		if listingID == "" {
			writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "listing_id is required"}}, http.StatusBadRequest)
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if r.ContentLength != 0 {
			if err := readJSONBody(r, &body); err != nil {
				writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
				return
			}
		}
		if err := step(r.Context(), actorID, listingID, body.Reason); err != nil {
			writeListingStepFailure(w, code, err)
			return
		}
		writeJSON(w, map[string]string{"status": status}, http.StatusOK)
	}
}

// SuspendListingH handles POST /v1/listings/suspend?listing_id= {"reason"} (administrators and operators).
func SuspendListingH(uc ListingModeration) http.HandlerFunc {
	return moderationStep("LISTING_SUSPEND_FAILED", string(domain.ListingStatusSuspended), uc.SuspendListing)
}

// ReinstateListingH handles POST /v1/listings/reinstate?listing_id= {"reason"?} (administrators and operators).
// The listing comes back paused.
func ReinstateListingH(uc ListingModeration) http.HandlerFunc {
	return moderationStep("LISTING_REINSTATE_FAILED", string(domain.ListingStatusPaused), uc.ReinstateListing)
}
