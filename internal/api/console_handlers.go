package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

/*
 * The seller workspace's read models and team management, under /api/v1/sellers/*:
 *
 *	GET    /sellers/finance?organization_id=&days=&status=&page=     payments and per-currency totals      manager
 *	GET    /sellers/customers?organization_id=&q=&page=              who bought and booked, no contact     manager
 *	GET    /sellers/fulfilments?organization_id=&status=&page=       deliveries, pickups, appointments     manager
 *	GET    /sellers/capacity?organization_id=&from=&days=            stock and capacity windows            manager
 *	GET    /sellers/usage?organization_id=                           credits, ledger, plan                 manager
 *	GET    /sellers/integrations?organization_id=                    providers and channels, no secrets    manager
 *	GET    /sellers/team?organization_id=                            members (masked phones), invitations  member
 *	POST   /sellers/team/invitations?organization_id=  {phone, role}                                       owner, admin
 *	DELETE /sellers/team/invitations/{id}?organization_id=                                                 owner, admin
 *	PATCH  /sellers/team/members/{user_id}?organization_id=  {role}                                        owner, admin
 *	DELETE /sellers/team/members/{user_id}?organization_id=                                                owner, admin
 *
 * "manager" means an owner, admin or manager of the business, or a platform administrator or operator; roles are
 * read from the database on every request.
 */

// TeamManager is what the team routes need (application.TeamUseCases).
type TeamManager interface {
	Invite(ctx context.Context, actorID, organizationID, phone string, role domain.OrganizationRole) (string, error)
	Invitations(ctx context.Context, actorID, organizationID string) ([]domain.TeamInvitation, error)
	RevokeInvitation(ctx context.Context, actorID, organizationID, invitationID string) error
	ChangeRole(ctx context.Context, actorID, organizationID, userID string, role domain.OrganizationRole) error
}

// MemberRemover removes a member (application.OrganizationUseCases).
type MemberRemover interface {
	RemoveMember(ctx context.Context, actorID, organizationID, userID string) error
}

// ConsoleHandler serves the seller workspace's read models and team routes.
type ConsoleHandler struct {
	reader  domain.ConsoleReader
	users   domain.UserRepository
	orgs    domain.OrganizationRepository
	team    TeamManager
	members MemberRemover
	now     func() time.Time
}

// NewConsoleHandler creates a ConsoleHandler.
func NewConsoleHandler(reader domain.ConsoleReader, users domain.UserRepository, orgs domain.OrganizationRepository) *ConsoleHandler {
	return &ConsoleHandler{reader: reader, users: users, orgs: orgs, now: time.Now}
}

// WithTeam attaches team management.
func (h *ConsoleHandler) WithTeam(team TeamManager, members MemberRemover) *ConsoleHandler {
	h.team, h.members = team, members
	return h
}

// WithClock sets the clock (tests).
func (h *ConsoleHandler) WithClock(now func() time.Time) *ConsoleHandler {
	h.now = now
	return h
}

type orgAccess struct {
	actorID string
	orgID   string
	role    domain.OrganizationRole // "" for platform staff who are not members
	staff   bool
}

func (a orgAccess) manages() bool {
	return a.staff || a.role == domain.OrganizationRoleOwner || a.role == domain.OrganizationRoleAdmin || a.role == domain.OrganizationRoleManager
}

// access checks the caller against ?organization_id=; it answers 400/401/403 itself.
func (h *ConsoleHandler) access(w http.ResponseWriter, r *http.Request, needManager bool) (orgAccess, bool) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return orgAccess{}, false
	}
	orgID := r.URL.Query().Get("organization_id")
	if !orgIDPattern.MatchString(orgID) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "organization_id is required"}}, http.StatusBadRequest)
		return orgAccess{}, false
	}
	a := orgAccess{actorID: userID, orgID: orgID}
	user, err := h.users.FindByID(r.Context(), userID)
	if err != nil || user.Status != domain.UserStatusActive {
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Access denied"}}, http.StatusForbidden)
		return orgAccess{}, false
	}
	a.staff = user.HasAnyRole(domain.RoleAdmin, domain.RoleOperator)
	if role, err := h.orgs.GetMemberRole(r.Context(), orgID, userID); err == nil {
		a.role = role
	}
	if (needManager && !a.manages()) || (!needManager && a.role == "" && !a.staff) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only owners, admins and managers of this business can see this"}}, http.StatusForbidden)
		return orgAccess{}, false
	}
	return a, true
}

func consolePage(r *http.Request, def int) (domain.Page, int) {
	q := r.URL.Query()
	size := atoiClamp(q.Get("page_size"), def, 1, 100)
	page := atoiClamp(q.Get("page"), 1, 1, 100000)
	return domain.Page{Limit: size, Offset: (page - 1) * size}, page
}

func paged(items interface{}, total, page int, p domain.Page) map[string]interface{} {
	return map[string]interface{}{"items": items, "total": total, "page": page, "page_size": p.Limit}
}

func queryFailed(w http.ResponseWriter, what string) {
	writeJSON(w, ErrorResponse{Error: Error{Code: "QUERY_FAILED", Message: "The " + what + " could not be loaded"}}, http.StatusInternalServerError)
}

func badFilter(w http.ResponseWriter) {
	writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_FILTER", Message: "Unknown filter value"}}, http.StatusBadRequest)
}

// paymentJSON is one payment as the consoles read it.
func paymentJSON(p domain.PaymentRow, withBusiness bool) map[string]interface{} {
	m := map[string]interface{}{
		"id": p.ID, "order_id": p.OrderID, "amount_minor": p.AmountMinor, "currency": p.Currency, "method": p.Method,
		"provider": p.ProviderCode, "status": p.Status, "refunded_minor": p.RefundedMinor, "failure_reason": p.FailureReason,
		"created_at": isoTime(p.CreatedAt),
	}
	if p.CapturedAt != 0 {
		m["captured_at"] = isoTime(p.CapturedAt)
	}
	if withBusiness {
		m["organization_id"], m["organization_name"] = p.OrganizationID, p.OrganizationName
	}
	return m
}

// moneyWindow reads ?days= (default 30, 1–365) and returns the window ending now.
func (h *ConsoleHandler) moneyWindow(r *http.Request) (int, int64, int64) {
	days := atoiClamp(r.URL.Query().Get("days"), 30, 1, 365)
	to := h.now()
	return days, to.Add(-time.Duration(days) * 24 * time.Hour).Unix(), to.Unix()
}

// FinanceH handles GET /api/v1/sellers/finance.
func (h *ConsoleHandler) FinanceH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		status := r.URL.Query().Get("status")
		if status != "" && !paymentStatuses[status] {
			badFilter(w)
			return
		}
		days, from, to := h.moneyWindow(r)
		totals, err := h.reader.MoneyTotals(r.Context(), a.orgID, from, to)
		if err != nil {
			queryFailed(w, "totals")
			return
		}
		p, page := consolePage(r, 25)
		rows, total, err := h.reader.Payments(r.Context(), domain.PaymentFilter{OrganizationID: a.orgID, Status: status}, p)
		if err != nil {
			queryFailed(w, "payments")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, x := range rows {
			items = append(items, paymentJSON(x, false))
		}
		writeJSON(w, map[string]interface{}{
			"days": days, "from": isoTime(from), "to": isoTime(to), "totals": totals, "payments": paged(items, total, page, p),
		}, http.StatusOK)
	}
}

var paymentStatuses = map[string]bool{"created": true, "pending": true, "authorized": true, "captured": true, "failed": true, "cancelled": true, "expired": true, "refunded": true, "partially_refunded": true}

// CustomersH handles GET /api/v1/sellers/customers.
func (h *ConsoleHandler) CustomersH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len(q) > 100 {
			q = q[:100]
		}
		p, page := consolePage(r, 25)
		rows, total, err := h.reader.Customers(r.Context(), a.orgID, q, p)
		if err != nil {
			queryFailed(w, "customers")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, c := range rows {
			items = append(items, map[string]interface{}{
				"customer_id": c.CustomerID, "display_name": c.DisplayName, "orders": c.Orders, "reservations": c.Reservations,
				"spent": c.Spent, "first_at": isoTime(c.FirstAt), "last_at": isoTime(c.LastAt),
			})
		}
		writeJSON(w, paged(items, total, page, p), http.StatusOK)
	}
}

var fulfilmentFilters = map[string]bool{"open": true, "PENDING": true, "ACCEPTED": true, "IN_PROGRESS": true, "READY": true, "COMPLETED": true, "CANCELLED": true, "FAILED": true}

// FulfilmentsH handles GET /api/v1/sellers/fulfilments.
func (h *ConsoleHandler) FulfilmentsH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		status := r.URL.Query().Get("status")
		if status != "" && !fulfilmentFilters[status] {
			badFilter(w)
			return
		}
		p, page := consolePage(r, 25)
		rows, total, err := h.reader.Fulfillments(r.Context(), a.orgID, status, p)
		if err != nil {
			queryFailed(w, "fulfilments")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, f := range rows {
			items = append(items, map[string]interface{}{
				"id": f.ID, "reference": f.Reference, "type": f.Type, "status": f.Status, "order_id": f.OrderID, "reservation_id": f.ReservationID,
				"title": f.Title, "order_status": f.OrderStatus, "customer_name": f.CustomerName, "tracking_ref": f.TrackingRef,
				"assigned_to": f.AssignedTo, "assigned_name": f.AssignedName, "created_at": isoTime(f.CreatedAt), "updated_at": isoTime(f.UpdatedAt),
			})
		}
		writeJSON(w, paged(items, total, page, p), http.StatusOK)
	}
}

// CapacityH handles GET /api/v1/sellers/capacity?from=YYYY-MM-DD&days=14 (Africa/Kinshasa days, at most 62).
func (h *ConsoleHandler) CapacityH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		loc, err := time.LoadLocation(domain.AnalyticsTimeZone)
		if err != nil {
			loc = time.FixedZone("WAT", 3600)
		}
		days := atoiClamp(r.URL.Query().Get("days"), 14, 1, 62)
		now := h.now().In(loc)
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		if raw := r.URL.Query().Get("from"); raw != "" {
			d, err := time.ParseInLocation("2006-01-02", raw, loc)
			if err != nil {
				writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_FILTER", Message: "from must be a date (2026-10-05)"}}, http.StatusBadRequest)
				return
			}
			start = d
		}
		end := start.AddDate(0, 0, days)
		rows, err := h.reader.Capacity(r.Context(), a.orgID, start.Unix(), end.Unix())
		if err != nil {
			queryFailed(w, "availability")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, c := range rows {
			windows := make([]map[string]interface{}, 0, len(c.Windows))
			for _, x := range c.Windows {
				windows = append(windows, map[string]interface{}{"id": x.ID, "start_at": isoTime(x.StartAt), "end_at": isoTime(x.EndAt), "capacity": x.Capacity, "reserved": x.Reserved})
			}
			items = append(items, map[string]interface{}{
				"inventory_id": c.InventoryID, "listing_id": c.ListingID, "listing_title": c.ListingTitle, "listing_type": c.ListingType,
				"resource_type": c.ResourceType, "total": c.Total, "reserved": c.Reserved, "consumed": c.Consumed, "windows": windows,
			})
		}
		writeJSON(w, map[string]interface{}{"from": start.Format("2006-01-02"), "days": days, "timezone": domain.AnalyticsTimeZone, "items": items}, http.StatusOK)
	}
}

func subscriptionJSON(s *domain.SubscriptionRow) map[string]interface{} {
	if s == nil {
		return nil
	}
	return map[string]interface{}{
		"id": s.ID, "organization_id": s.OrganizationID, "organization_name": s.OrganizationName, "status": s.Status, "period_id": s.PeriodID,
		"cancel_at_period_end": s.CancelAtPeriodEnd, "started_at": isoTime(s.StartedAt),
		"plan": map[string]interface{}{"id": s.PlanID, "name": s.PlanName, "slug": s.PlanSlug, "price_minor": s.PriceMinor, "currency": s.Currency, "billing_cycle": s.BillingCycle, "quota": s.QuotaAmount},
	}
}

// UsageH handles GET /api/v1/sellers/usage.
func (h *ConsoleHandler) UsageH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		u, err := h.reader.Usage(r.Context(), a.orgID)
		if err != nil {
			queryFailed(w, "usage")
			return
		}
		accounts := make([]map[string]interface{}, 0, len(u.Accounts))
		for _, x := range u.Accounts {
			accounts = append(accounts, map[string]interface{}{
				"account_id": x.AccountID, "owner_type": x.OwnerType, "period_id": x.PeriodID, "granted": x.Granted, "reserved": x.Reserved,
				"consumed": x.Consumed, "balance": x.Granted - x.Reserved - x.Consumed, "updated_at": isoTime(x.UpdatedAt),
			})
		}
		ledger := make([]map[string]interface{}, 0, len(u.Ledger))
		for _, e := range u.Ledger {
			ledger = append(ledger, map[string]interface{}{
				"id": e.ID, "action_type": e.ActionType, "kind": e.TransactionType, "amount": e.Amount, "balance_after": e.BalanceAfter,
				"period_id": e.PeriodID, "created_at": isoTime(e.CreatedAt),
			})
		}
		writeJSON(w, map[string]interface{}{"accounts": accounts, "ledger": ledger, "subscription": subscriptionJSON(u.Subscription), "policies": u.Policies}, http.StatusOK)
	}
}

func providerJSON(p domain.ProviderRow, withBusiness bool) map[string]interface{} {
	m := map[string]interface{}{
		"id": p.ID, "code": p.Code, "name": p.Name, "type": p.Type, "status": p.Status, "health_status": p.HealthStatus,
		"capabilities": p.Capabilities, "priority": p.Priority, "updated_at": isoTime(p.UpdatedAt),
		"executions": map[string]interface{}{"attempts": p.Attempts, "failures": p.Failures, "p50_ms": p.P50Ms, "p95_ms": p.P95Ms, "last_error": p.LastError},
	}
	if p.LastFailure != 0 {
		m["executions"].(map[string]interface{})["last_failure_at"] = isoTime(p.LastFailure)
	}
	if withBusiness {
		m["organization_id"], m["organization_name"] = p.OrganizationID, p.OrganizationName
	}
	return m
}

// IntegrationsH handles GET /api/v1/sellers/integrations: the business's providers (7 days of executions) and
// linked channels. No configuration, no secret, no full external id.
func (h *ConsoleHandler) IntegrationsH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, true)
		if !ok {
			return
		}
		providers, err := h.reader.Providers(r.Context(), a.orgID, h.now().Add(-7*24*time.Hour).Unix())
		if err != nil {
			queryFailed(w, "providers")
			return
		}
		channels, err := h.reader.ChannelAccounts(r.Context(), a.orgID)
		if err != nil {
			queryFailed(w, "channels")
			return
		}
		items := make([]map[string]interface{}, 0, len(providers))
		for _, p := range providers {
			items = append(items, providerJSON(p, false))
		}
		writeJSON(w, map[string]interface{}{"providers": items, "channels": channels}, http.StatusOK)
	}
}

// TeamH handles GET /api/v1/sellers/team: every member sees the team; owners and admins also see the open
// invitations.
func (h *ConsoleHandler) TeamH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := h.access(w, r, false)
		if !ok {
			return
		}
		members, err := h.reader.Team(r.Context(), a.orgID)
		if err != nil {
			queryFailed(w, "team")
			return
		}
		items := make([]map[string]interface{}, 0, len(members))
		for _, m := range members {
			items = append(items, map[string]interface{}{
				"user_id": m.UserID, "display_name": m.DisplayName, "phone": m.MaskedPhone, "role": m.Role, "status": m.Status,
				"joined_at": isoTime(m.JoinedAt), "you": m.UserID == a.actorID,
			})
		}
		canManage := a.role == domain.OrganizationRoleOwner || a.role == domain.OrganizationRoleAdmin
		out := map[string]interface{}{"members": items, "can_manage": canManage, "your_role": string(a.role)}
		if canManage && h.team != nil {
			inv, err := h.team.Invitations(r.Context(), a.actorID, a.orgID)
			if err != nil {
				queryFailed(w, "invitations")
				return
			}
			list := make([]map[string]interface{}, 0, len(inv))
			for _, x := range inv {
				list = append(list, map[string]interface{}{
					"id": x.ID, "phone": x.MaskedPhone, "role": x.Role, "invited_by": x.InvitedName, "created_at": isoTime(x.CreatedAt), "expires_at": isoTime(x.ExpiresAt),
				})
			}
			out["invitations"] = list
		}
		writeJSON(w, out, http.StatusOK)
	}
}

func writeTeamFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInsufficientPermissions):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Only owners and admins of this business manage its team"}}, http.StatusForbidden)
	case errors.Is(err, domain.ErrInvitationNotFound), errors.Is(err, domain.ErrNotMember):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: err.Error()}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrAccountState):
		writeJSON(w, ErrorResponse{Error: Error{Code: "MEMBER_CHANGED", Message: "This member was changed in the meantime. Reload and try again."}}, http.StatusConflict)
	case application.IsTeamRefusal(err):
		writeJSON(w, ErrorResponse{Error: Error{Code: "REFUSED", Message: err.Error()}}, http.StatusUnprocessableEntity)
	default:
		writeFailure(w, "TEAM_UPDATE_FAILED", err)
	}
}

// teamWrite authenticates and reads ?organization_id= for the team changes (the use cases decide who may).
func (h *ConsoleHandler) teamWrite(w http.ResponseWriter, r *http.Request) (string, string, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if h.team == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Team management is not available"}}, http.StatusServiceUnavailable)
		return "", "", false
	}
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return "", "", false
	}
	orgID := r.URL.Query().Get("organization_id")
	if !orgIDPattern.MatchString(orgID) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "organization_id is required"}}, http.StatusBadRequest)
		return "", "", false
	}
	return userID, orgID, true
}

// InviteH handles POST /api/v1/sellers/team/invitations {phone, role}. The answer never says whether the
// number has an account.
func (h *ConsoleHandler) InviteH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := h.teamWrite(w, r)
		if !ok {
			return
		}
		var body struct {
			Phone string `json:"phone"`
			Role  string `json:"role"`
		}
		if err := readJSONBody(r, &body); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
			return
		}
		id, err := h.team.Invite(r.Context(), actorID, orgID, body.Phone, domain.OrganizationRole(body.Role))
		if err != nil {
			writeTeamFailure(w, err)
			return
		}
		writeJSON(w, map[string]string{"id": id, "status": "invited"}, http.StatusCreated)
	}
}

// RevokeInvitationH handles DELETE /api/v1/sellers/team/invitations/{id}.
func (h *ConsoleHandler) RevokeInvitationH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := h.teamWrite(w, r)
		if !ok {
			return
		}
		if err := h.team.RevokeInvitation(r.Context(), actorID, orgID, r.PathValue("id")); err != nil {
			writeTeamFailure(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ChangeRoleH handles PATCH /api/v1/sellers/team/members/{user_id} {role}.
func (h *ConsoleHandler) ChangeRoleH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := h.teamWrite(w, r)
		if !ok {
			return
		}
		var body struct {
			Role string `json:"role"`
		}
		if err := readJSONBody(r, &body); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
			return
		}
		if err := h.team.ChangeRole(r.Context(), actorID, orgID, r.PathValue("user_id"), domain.OrganizationRole(body.Role)); err != nil {
			writeTeamFailure(w, err)
			return
		}
		writeJSON(w, map[string]string{"status": "updated"}, http.StatusOK)
	}
}

// RemoveMemberH handles DELETE /api/v1/sellers/team/members/{user_id}. Owners are never removed.
func (h *ConsoleHandler) RemoveMemberH() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, orgID, ok := h.teamWrite(w, r)
		if !ok {
			return
		}
		target := r.PathValue("user_id")
		if target == actorID {
			writeTeamFailure(w, domain.ErrOwnerProtected)
			return
		}
		if err := h.members.RemoveMember(r.Context(), actorID, orgID, target); err != nil {
			writeTeamFailure(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
