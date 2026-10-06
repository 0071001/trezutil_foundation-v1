package api

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// AdminReader supplies the read-only data of the administration console.
type AdminReader interface {
	Overview(ctx context.Context, now int64) (*domain.AdminOverview, error)
	ListUsers(ctx context.Context, query string, limit, offset int) ([]domain.AdminUser, int, error)
}

// AdminHandler serves /api/v1/admin/*. The caller's roles are read from the
// database on every request, never from the token alone, so a revoked role
// stops working at once.
type AdminHandler struct {
	reader AdminReader
	users  domain.UserRepository
	audit  AuditLister
	// moderation suspends and reactivates accounts
	moderation AccountModerator
	// commerce lists orders, bookings and businesses
	commerce AdminCommerceReader
	now      func() time.Time
}

// NewAdminHandler creates an AdminHandler.
func NewAdminHandler(reader AdminReader, users domain.UserRepository) *AdminHandler {
	return &AdminHandler{reader: reader, users: users, now: time.Now}
}

// require wraps a handler so that only a signed-in user holding one of the
// roles may reach it: 401 for anonymous callers, 403 for everyone else.
func (h *AdminHandler) require(next http.HandlerFunc, roles ...domain.Role) http.HandlerFunc {
	return RequireRoles(h.users, next, roles...)
}

// RequireRoles is the guard behind every /api/v1/admin/* route. The caller's
// roles and status are read from the database on every request, never from the
// token alone, so a revoked role or a suspension stops working at once.
func RequireRoles(users domain.UserRepository, next http.HandlerFunc, roles ...domain.Role) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		userID, ok := auth.RequireUser(w, r)
		if !ok {
			return
		}
		user, err := users.FindByID(r.Context(), userID)
		if err != nil || user.Status != domain.UserStatusActive || !user.HasAnyRole(roles...) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "This area is reserved for platform administrators"}}, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// OverviewH handles GET /api/v1/admin/overview (admin or operator).
func (h *AdminHandler) OverviewH() http.HandlerFunc {
	return h.require(func(w http.ResponseWriter, r *http.Request) {
		o, err := h.reader.Overview(r.Context(), h.now().Unix())
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ADMIN_QUERY_FAILED", Message: "The overview could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]interface{}{
			"generated_at": isoTime(o.GeneratedAt),
			"users": map[string]interface{}{
				"total": o.UsersTotal, "active": o.UsersActive, "suspended": o.UsersSuspended,
				"new_last_7_days": o.UsersNew7d, "by_role": o.UsersByRole,
			},
			"organizations":      o.Organizations,
			"listings_by_status": o.ListingsByStatus,
			"orders":             map[string]int{"total": o.OrdersTotal, "last_7_days": o.Orders7d},
			"reservations":       o.ReservationsTotal,
			"conversations":      map[string]int{"last_7_days": o.Conversations7d},
		}, http.StatusOK)
	}, domain.RoleAdmin, domain.RoleOperator)
}

// UsersH handles GET /api/v1/admin/users?q=&page=&page_size= (admin only:
// the list shows phone numbers and email addresses).
func (h *AdminHandler) UsersH() http.HandlerFunc {
	return h.require(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		query := q.Get("q")
		if len(query) > 100 {
			query = query[:100]
		}
		pageSize := atoiClamp(q.Get("page_size"), 25, 1, 100)
		page := atoiClamp(q.Get("page"), 1, 1, 100000)
		users, total, err := h.reader.ListUsers(r.Context(), query, pageSize, (page-1)*pageSize)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ADMIN_QUERY_FAILED", Message: "The users could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		items := make([]map[string]interface{}, 0, len(users))
		for _, u := range users {
			item := map[string]interface{}{
				"id": u.ID, "email": u.Email, "phone": u.Phone, "display_name": u.DisplayName,
				"status": u.Status, "roles": u.Roles, "created_at": isoTime(u.CreatedAt),
			}
			if u.LastLoginAt != 0 {
				item["last_login_at"] = isoTime(u.LastLoginAt)
			}
			items = append(items, item)
		}
		writeJSON(w, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize}, http.StatusOK)
	}, domain.RoleAdmin)
}

func atoiClamp(raw string, def, lo, hi int) int {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
