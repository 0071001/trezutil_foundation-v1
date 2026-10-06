package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// AccountModerator suspends and reactivates accounts (application/identity.Moderation).
type AccountModerator interface {
	Suspend(ctx context.Context, actorID, targetID, reason string) error
	Reactivate(ctx context.Context, actorID, targetID, reason string) error
}

// WithModeration attaches the account moderation use cases.
func (h *AdminHandler) WithModeration(m AccountModerator) *AdminHandler {
	h.moderation = m
	return h
}

// SuspendUserH handles POST /api/v1/admin/users/{user_id}/suspend with {"reason": "…"} (administrators only).
func (h *AdminHandler) SuspendUserH() http.HandlerFunc { return h.moderate(true) }

// ReactivateUserH handles POST /api/v1/admin/users/{user_id}/reactivate with an optional {"reason": "…"}
// (administrators only).
func (h *AdminHandler) ReactivateUserH() http.HandlerFunc { return h.moderate(false) }

func (h *AdminHandler) moderate(suspend bool) http.HandlerFunc {
	return h.require(func(w http.ResponseWriter, r *http.Request) {
		if h.moderation == nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Account moderation is not available"}}, http.StatusServiceUnavailable)
			return
		}
		var body struct {
			Reason string `json:"reason"`
		}
		if raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<10)); err == nil && len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "The request body is not valid JSON"}}, http.StatusBadRequest)
				return
			}
		}
		actor, target := auth.UserID(r.Context()), r.PathValue("user_id")
		var err error
		if suspend {
			err = h.moderation.Suspend(r.Context(), actor, target, body.Reason)
		} else {
			err = h.moderation.Reactivate(r.Context(), actor, target, body.Reason)
		}
		switch {
		case err == nil:
			status := "active"
			if suspend {
				status = "suspended"
			}
			writeJSON(w, map[string]string{"user_id": target, "status": status}, http.StatusOK)
		case errors.Is(err, domain.ErrUserNotFound):
			writeJSON(w, ErrorResponse{Error: Error{Code: "USER_NOT_FOUND", Message: "No such account"}}, http.StatusNotFound)
		case errors.Is(err, domain.ErrReasonRequired):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REASON", Message: "Give a reason of 3 to 300 characters"}}, http.StatusUnprocessableEntity)
		case errors.Is(err, domain.ErrCannotModerateSelf):
			writeJSON(w, ErrorResponse{Error: Error{Code: "CANNOT_MODERATE_SELF", Message: err.Error()}}, http.StatusConflict)
		case errors.Is(err, domain.ErrCannotModerateStaff):
			writeJSON(w, ErrorResponse{Error: Error{Code: "STAFF_PROTECTED", Message: err.Error()}}, http.StatusConflict)
		case errors.Is(err, domain.ErrAccountState):
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_STATE", Message: err.Error()}}, http.StatusConflict)
		default:
			writeFailure(w, "MODERATION_FAILED", err)
		}
	}, domain.RoleAdmin)
}
