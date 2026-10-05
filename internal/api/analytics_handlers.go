package api

import (
	"net/http"
	"regexp"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// AnalyticsHandler serves the figures behind the dashboards:
//
//	GET /api/v1/admin/analytics?days=30                      administrators and operators: the whole platform
//	GET /api/v1/sellers/analytics?organization_id=…&days=30   owners, admins and managers of that business
//	                                                          (and platform administrators and operators)
//
// The answers are computed from the platform's tables (see domain.Analytics);
// nothing is estimated. Who may read what is decided here, on every request,
// from the database, never from the token alone.
type AnalyticsHandler struct {
	repo  domain.AnalyticsRepository
	users domain.UserRepository
	orgs  domain.OrganizationRepository
	now   func() time.Time
}

// NewAnalyticsHandler creates an AnalyticsHandler.
func NewAnalyticsHandler(repo domain.AnalyticsRepository, users domain.UserRepository, orgs domain.OrganizationRepository) *AnalyticsHandler {
	return &AnalyticsHandler{repo: repo, users: users, orgs: orgs, now: time.Now}
}

// WithClock sets the clock the windows are counted from (tests use a fixed one).
func (h *AnalyticsHandler) WithClock(now func() time.Time) *AnalyticsHandler {
	h.now = now
	return h
}

var orgIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// analyticsDays reads ?days=, defaulting to 30 and clamping to the supported range.
func analyticsDays(r *http.Request) int {
	return atoiClamp(r.URL.Query().Get("days"), 30, domain.MinAnalyticsDays, domain.MaxAnalyticsDays)
}

func analyticsJSON(a *domain.Analytics) map[string]interface{} {
	out := map[string]interface{}{
		"generated_at": isoTime(a.GeneratedAt), "timezone": domain.AnalyticsTimeZone, "days": a.Days, "from": a.From, "to": a.To,
		"revenue_currencies": a.RevenueCurrencies(),
		"current":            a.Current, "previous": a.Previous,
		"orders": a.Orders, "revenue": a.Revenue, "reservations": a.Reservations, "new_listings": a.Listings,
		"orders_by_status": a.OrdersByStatus, "reservations_by_status": a.ReservationsByStatus,
		"listings_by_status": a.ListingsByStatus, "published_listings_by_vertical": a.ListingsByVertical,
	}
	// Platform-only and organization-only parts are left out where they do not apply.
	if a.Users != nil {
		out["new_users"], out["conversations"], out["users_by_role"] = a.Users, a.Conversations, a.UsersByRole
		out["top_organizations"] = orEmpty(a.TopOrganizations)
	} else {
		out["top_listings"] = orEmpty(a.TopListings)
	}
	return out
}

func orEmpty(v []domain.RankedEntry) []domain.RankedEntry {
	if v == nil {
		return []domain.RankedEntry{}
	}
	return v
}

// AdminH handles GET /api/v1/admin/analytics.
func (h *AnalyticsHandler) AdminH() http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		a, err := h.repo.Platform(r.Context(), h.now().Unix(), analyticsDays(r))
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ANALYTICS_FAILED", Message: "The analytics could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, analyticsJSON(a), http.StatusOK)
	}, domain.RoleAdmin, domain.RoleOperator)
}

// canReadOrganizationAnalytics: platform staff, or an owner, admin or manager of the business.
func (h *AnalyticsHandler) canReadOrganizationAnalytics(r *http.Request, userID, organizationID string) bool {
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil || user.Status != domain.UserStatusActive {
		return false
	}
	if user.HasAnyRole(domain.RoleAdmin, domain.RoleOperator) {
		return true
	}
	role, err := h.orgs.GetMemberRole(r.Context(), organizationID, userID)
	if err != nil {
		return false
	}
	switch role {
	case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager:
		return true
	}
	return false
}

// SellerH handles GET /api/v1/sellers/analytics?organization_id=.
func (h *AnalyticsHandler) SellerH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		organizationID := r.URL.Query().Get("organization_id")
		if !orgIDPattern.MatchString(organizationID) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "organization_id is required"}}, http.StatusBadRequest)
			return
		}
		if !h.canReadOrganizationAnalytics(r, userID, organizationID) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only owners, admins and managers of this business can read its analytics"}}, http.StatusForbidden)
			return
		}
		a, err := h.repo.Organization(r.Context(), organizationID, h.now().Unix(), analyticsDays(r))
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ANALYTICS_FAILED", Message: "The analytics could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, analyticsJSON(a), http.StatusOK)
	}
}
