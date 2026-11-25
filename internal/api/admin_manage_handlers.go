package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/adminmanage"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// AdminManageHandler serves what the Control Center creates, changes and removes (adminmanage.Service). Every write
// is for administrators; the service checks the role and writes the audit trail.
//
//	POST   /api/v1/admin/plans                        create a plan
//	PATCH  /api/v1/admin/plans/{id}                   change a plan
//	DELETE /api/v1/admin/plans/{id}                   delete (never used) or archive (used): {"archived": bool}
//	GET    /api/v1/admin/subscriptions                list the platform's subscriptions (admin, operator)
//	POST   /api/v1/admin/subscriptions                {"user_id","plan_id"}: subscribe or move to another plan
//	POST   /api/v1/admin/subscriptions/{id}/cancel    end a subscription
//	POST   /api/v1/admin/credits/grants               {"user_id","amount","reason"}: manual credits (Idempotency-Key)
//	POST   /api/v1/admin/usage-policies               price a metered action
//	PATCH  /api/v1/admin/usage-policies/{id}
//	DELETE /api/v1/admin/usage-policies/{id}
//	GET    /api/v1/admin/usage-policies               the pricing rules, with their ids (admin, operator)
//	GET    /api/v1/admin/usage-policies/actions       the actions a rule can price
//	PATCH  /api/v1/admin/categories/{id}              rename / move
//	DELETE /api/v1/admin/categories/{id}              only when empty
//	POST   /api/v1/admin/users/{user_id}/roles        {"role"}: grant a staff role
//	DELETE /api/v1/admin/users/{user_id}/roles/{role} revoke a staff role
type AdminManageHandler struct {
	svc    *adminmanage.Service
	logger zerolog.Logger
}

// NewAdminManageHandler builds the handler.
func NewAdminManageHandler(svc *adminmanage.Service, logger zerolog.Logger) *AdminManageHandler {
	return &AdminManageHandler{svc: svc, logger: logger}
}

// Register mounts the routes.
func (h *AdminManageHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/admin/plans", h.createPlan)
	mux.HandleFunc("PATCH /api/v1/admin/plans/{id}", h.updatePlan)
	mux.HandleFunc("DELETE /api/v1/admin/plans/{id}", h.deletePlan)
	mux.HandleFunc("GET /api/v1/admin/subscriptions", h.subscriptions)
	mux.HandleFunc("POST /api/v1/admin/subscriptions", h.subscribe)
	mux.HandleFunc("POST /api/v1/admin/subscriptions/{id}/cancel", h.cancel)
	mux.HandleFunc("POST /api/v1/admin/credits/grants", h.grant)
	mux.HandleFunc("GET /api/v1/admin/usage-policies/actions", h.actions)
	mux.HandleFunc("GET /api/v1/admin/usage-policies", h.policies)
	mux.HandleFunc("POST /api/v1/admin/usage-policies", h.createPolicy)
	mux.HandleFunc("PATCH /api/v1/admin/usage-policies/{id}", h.updatePolicy)
	mux.HandleFunc("DELETE /api/v1/admin/usage-policies/{id}", h.deletePolicy)
	mux.HandleFunc("PATCH /api/v1/admin/categories/{id}", h.updateCategory)
	mux.HandleFunc("DELETE /api/v1/admin/categories/{id}", h.deleteCategory)
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/roles", h.grantRole)
	mux.HandleFunc("DELETE /api/v1/admin/users/{user_id}/roles/{role}", h.revokeRole)
}

// RunRenewals grants the monthly included credits of every active subscription (the monthly job).
func (h *AdminManageHandler) RunRenewals(ctx context.Context) {
	n, err := h.svc.RenewAll(ctx)
	if err != nil {
		if !errors.Is(err, adminmanage.ErrNoPlatformOrg) {
			h.logger.Warn().Err(err).Msg("subscription renewal failed")
		}
		return
	}
	if n > 0 {
		h.logger.Info().Int("subscriptions", n).Msg("subscription credits checked for this month")
	}
}

func (h *AdminManageHandler) caller(w http.ResponseWriter, r *http.Request) (string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	return auth.RequireUser(w, r)
}

func (h *AdminManageHandler) fail(w http.ResponseWriter, err error) {
	var inv *adminmanage.InvalidError
	switch {
	case errors.As(err, &inv):
		writeJSON(w, map[string]interface{}{"error": map[string]string{"code": "INVALID_FIELD", "message": inv.Error(), "field": inv.Field}}, http.StatusBadRequest)
	case errors.Is(err, adminmanage.ErrForbidden):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only an administrator can make this change"}}, http.StatusForbidden)
	case errors.Is(err, adminmanage.ErrNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Not found"}}, http.StatusNotFound)
	case errors.Is(err, adminmanage.ErrInUse):
		writeJSON(w, ErrorResponse{Error: Error{Code: "IN_USE", Message: "Still in use"}}, http.StatusConflict)
	case errors.Is(err, adminmanage.ErrConflict):
		writeJSON(w, ErrorResponse{Error: Error{Code: "CONFLICT", Message: "This conflicts with an existing entry"}}, http.StatusConflict)
	case errors.Is(err, adminmanage.ErrLastAdmin):
		writeJSON(w, ErrorResponse{Error: Error{Code: "LAST_ADMIN", Message: "The platform must keep at least one administrator"}}, http.StatusConflict)
	case errors.Is(err, adminmanage.ErrSelfDemote):
		writeJSON(w, ErrorResponse{Error: Error{Code: "SELF_DEMOTE", Message: "You cannot remove your own administrator role"}}, http.StatusConflict)
	case errors.Is(err, adminmanage.ErrNoPlatformOrg):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NO_PLATFORM_ORGANIZATION", Message: "The Trezutil platform organisation does not exist yet"}}, http.StatusConflict)
	default:
		h.logger.Error().Err(err).Msg("Control Center change failed")
		writeJSON(w, ErrorResponse{Error: Error{Code: "ADMIN_CHANGE_FAILED", Message: "The change could not be made"}}, http.StatusInternalServerError)
	}
}

func (h *AdminManageHandler) body(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := readJSONBody(r, v); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return false
	}
	return true
}

func (h *AdminManageHandler) createPlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var p adminmanage.Plan
	if !ok || !h.body(w, r, &p) {
		return
	}
	out, err := h.svc.CreatePlan(r.Context(), actor, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, out, http.StatusCreated)
}

func (h *AdminManageHandler) updatePlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var p adminmanage.Plan
	if !ok || !h.body(w, r, &p) {
		return
	}
	p.ID = r.PathValue("id")
	out, err := h.svc.UpdatePlan(r.Context(), actor, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, out, http.StatusOK)
}

func (h *AdminManageHandler) deletePlan(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	archived, err := h.svc.DeletePlan(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]bool{"archived": archived}, http.StatusOK)
}

func (h *AdminManageHandler) subscriptions(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Subscriptions(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"items": list}, http.StatusOK)
}

func (h *AdminManageHandler) subscribe(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var body struct {
		UserID string `json:"user_id"`
		PlanID string `json:"plan_id"`
	}
	if !ok || !h.body(w, r, &body) {
		return
	}
	sub, err := h.svc.Subscribe(r.Context(), actor, body.UserID, body.PlanID)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, sub, http.StatusOK)
}

func (h *AdminManageHandler) cancel(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	sub, err := h.svc.Cancel(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, sub, http.StatusOK)
}

func (h *AdminManageHandler) grant(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var body struct {
		UserID string `json:"user_id"`
		Amount int64  `json:"amount"`
		Reason string `json:"reason"`
	}
	if !ok || !h.body(w, r, &body) {
		return
	}
	if err := h.svc.GrantCredits(r.Context(), actor, body.UserID, body.Amount, body.Reason, r.Header.Get("Idempotency-Key")); err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "granted"}, http.StatusOK)
}

func (h *AdminManageHandler) actions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.caller(w, r); !ok {
		return
	}
	writeJSON(w, map[string]interface{}{"items": adminmanage.BillableActions}, http.StatusOK)
}

func (h *AdminManageHandler) policies(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	list, err := h.svc.Policies(r.Context(), actor)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]interface{}{"items": list}, http.StatusOK)
}

func (h *AdminManageHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var p adminmanage.Policy
	if !ok || !h.body(w, r, &p) {
		return
	}
	out, err := h.svc.CreatePolicy(r.Context(), actor, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, out, http.StatusCreated)
}

func (h *AdminManageHandler) updatePolicy(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var p adminmanage.Policy
	if !ok || !h.body(w, r, &p) {
		return
	}
	p.ID = r.PathValue("id")
	out, err := h.svc.UpdatePolicy(r.Context(), actor, p)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, out, http.StatusOK)
}

func (h *AdminManageHandler) deletePolicy(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePolicy(r.Context(), actor, r.PathValue("id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminManageHandler) updateCategory(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var c adminmanage.Category
	if !ok || !h.body(w, r, &c) {
		return
	}
	c.ID = r.PathValue("id")
	out, err := h.svc.UpdateCategory(r.Context(), actor, c)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, out, http.StatusOK)
}

func (h *AdminManageHandler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), actor, r.PathValue("id")); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *AdminManageHandler) grantRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	var body struct {
		Role string `json:"role"`
	}
	if !ok || !h.body(w, r, &body) {
		return
	}
	if err := h.svc.GrantRole(r.Context(), actor, r.PathValue("user_id"), domain.Role(strings.TrimSpace(body.Role))); err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "granted"}, http.StatusOK)
}

func (h *AdminManageHandler) revokeRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := h.caller(w, r)
	if !ok {
		return
	}
	if err := h.svc.RevokeRole(r.Context(), actor, r.PathValue("user_id"), domain.Role(r.PathValue("role"))); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
