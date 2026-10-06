package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

/*
 * The administration console's remaining modules, under /api/v1/admin/* (administrators and operators unless
 * stated):
 *
 *	GET  /admin/listings?q=&status=&vertical=&flag=&page=     listings across businesses, with quality flags
 *	GET  /admin/payments?q=&status=&days=&page=               payments across businesses and per-currency totals
 *	GET  /admin/operations?days=                              failed payments and webhooks, unconfirmed paid orders
 *	GET  /admin/providers?days=                               every provider connection and its executions
 *	POST /admin/providers/{id}/enable|disable  {reason}       (administrators only, audited)
 *	GET  /admin/channels                                      inbound channel health; which senders are configured
 *	GET  /admin/security                                      staff accounts' sign-in state, recent sensitive events
 *	GET  /admin/ai?days=                                      the assistant's activity
 *	GET  /admin/monetization?days=                            plans, subscriptions, credits
 *	GET  /admin/health                                        each service's readiness, the database, providers
 *
 * Every answer leaves out secrets, provider configuration and customers' contact data.
 */

// HealthTarget is a service whose readiness the console reports.
type HealthTarget struct {
	Name string
	URL  string // its /readyz (or /healthz) on the internal network
}

// ConfiguredFlags says which external integrations have their settings present (never their values).
type ConfiguredFlags map[string]bool

// AdminConsoleHandler serves the administration modules above.
type AdminConsoleHandler struct {
	reader     domain.ConsoleReader
	users      domain.UserRepository
	audit      AuditLister
	logger     domain.AuditLogger
	db         *sql.DB
	targets    []HealthTarget
	configured ConfiguredFlags
	client     *http.Client
	now        func() time.Time
}

// NewAdminConsoleHandler creates an AdminConsoleHandler.
func NewAdminConsoleHandler(reader domain.ConsoleReader, users domain.UserRepository) *AdminConsoleHandler {
	return &AdminConsoleHandler{reader: reader, users: users, client: &http.Client{Timeout: 3 * time.Second}, now: time.Now}
}

// WithAudit attaches the audit trail (reading recent sensitive events, recording provider changes).
func (h *AdminConsoleHandler) WithAudit(list AuditLister, logger domain.AuditLogger) *AdminConsoleHandler {
	h.audit, h.logger = list, logger
	return h
}

// WithHealth sets what the health report checks.
func (h *AdminConsoleHandler) WithHealth(db *sql.DB, targets []HealthTarget, configured ConfiguredFlags) *AdminConsoleHandler {
	h.db, h.targets, h.configured = db, targets, configured
	return h
}

// WithClock sets the clock (tests).
func (h *AdminConsoleHandler) WithClock(now func() time.Time) *AdminConsoleHandler {
	h.now = now
	return h
}

func (h *AdminConsoleHandler) staff(next http.HandlerFunc) http.HandlerFunc {
	return RequireRoles(h.users, next, domain.RoleAdmin, domain.RoleOperator)
}

func trimQuery(r *http.Request) string {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 100 {
		q = q[:100]
	}
	return q
}

var listingVerticalPattern = func(v string) bool {
	if len(v) == 0 || len(v) > 40 {
		return false
	}
	for _, c := range v {
		if !(c >= 'A' && c <= 'Z') && c != '_' {
			return false
		}
	}
	return true
}

// ListingsH handles GET /api/v1/admin/listings.
func (h *AdminConsoleHandler) ListingsH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f := domain.AdminListingFilter{Query: trimQuery(r), Status: q.Get("status"), Vertical: q.Get("vertical"), Flag: q.Get("flag")}
		if (f.Status != "" && !adminListingStatus[f.Status]) || (f.Vertical != "" && !listingVerticalPattern(f.Vertical)) || (f.Flag != "" && f.Flag != "no_image" && f.Flag != "no_price") {
			badFilter(w)
			return
		}
		p, page := consolePage(r, 25)
		rows, total, err := h.reader.AdminListings(r.Context(), f, p)
		if err != nil {
			queryFailed(w, "listings")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, l := range rows {
			items = append(items, map[string]interface{}{
				"id": l.ID, "title": l.Title, "status": l.Status, "vertical": l.Vertical, "listing_type": l.ListingType,
				"price_amount": l.PriceAmount, "currency": l.Currency, "organization_id": l.OrganizationID, "organization_name": l.OrganizationName,
				"category": l.CategoryName, "images": l.Images, "flags": l.Flags, "created_at": isoTime(l.CreatedAt), "updated_at": isoTime(l.UpdatedAt),
			})
		}
		writeJSON(w, paged(items, total, page, p), http.StatusOK)
	})
}

var adminListingStatus = map[string]bool{"draft": true, "pending_review": true, "published": true, "paused": true, "sold": true, "closed": true, "expired": true, "suspended": true}

// PaymentsH handles GET /api/v1/admin/payments.
func (h *AdminConsoleHandler) PaymentsH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		if status != "" && !paymentStatuses[status] {
			badFilter(w)
			return
		}
		days := atoiClamp(r.URL.Query().Get("days"), 30, 1, 365)
		now := h.now()
		from := now.Add(-time.Duration(days) * 24 * time.Hour).Unix()
		totals, err := h.reader.MoneyTotals(r.Context(), "", from, now.Unix())
		if err != nil {
			queryFailed(w, "totals")
			return
		}
		p, page := consolePage(r, 25)
		rows, total, err := h.reader.Payments(r.Context(), domain.PaymentFilter{Status: status, Query: trimQuery(r)}, p)
		if err != nil {
			queryFailed(w, "payments")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, x := range rows {
			items = append(items, paymentJSON(x, true))
		}
		writeJSON(w, map[string]interface{}{"days": days, "from": isoTime(from), "to": isoTime(now.Unix()), "totals": totals, "payments": paged(items, total, page, p)}, http.StatusOK)
	})
}

// StuckOrderAfter is how long a paid order may wait for its seller's confirmation before operations see it.
const StuckOrderAfter = 48 * time.Hour

// OperationsH handles GET /api/v1/admin/operations?days=7.
func (h *AdminConsoleHandler) OperationsH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		days := atoiClamp(r.URL.Query().Get("days"), 7, 1, 90)
		now := h.now()
		ex, err := h.reader.Exceptions(r.Context(), now.Add(-time.Duration(days)*24*time.Hour).Unix(), now.Add(-StuckOrderAfter).Unix(), 25)
		if err != nil {
			queryFailed(w, "operations")
			return
		}
		recent := make([]map[string]interface{}, 0, len(ex.Recent))
		for _, x := range ex.Recent {
			recent = append(recent, map[string]interface{}{"kind": x.Kind, "id": x.ID, "reference": x.Reference, "status": x.Status, "detail": x.Detail, "business": x.Business, "at": isoTime(x.At)})
		}
		sort.SliceStable(recent, func(i, j int) bool { return recent[i]["at"].(string) > recent[j]["at"].(string) })
		writeJSON(w, map[string]interface{}{"days": days, "stuck_after_hours": int(StuckOrderAfter.Hours()), "counts": ex.Counts, "recent": recent}, http.StatusOK)
	})
}

// ProvidersH handles GET /api/v1/admin/providers?days=7.
func (h *AdminConsoleHandler) ProvidersH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		days := atoiClamp(r.URL.Query().Get("days"), 7, 1, 90)
		rows, err := h.reader.Providers(r.Context(), "", h.now().Add(-time.Duration(days)*24*time.Hour).Unix())
		if err != nil {
			queryFailed(w, "providers")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, p := range rows {
			items = append(items, providerJSON(p, true))
		}
		writeJSON(w, map[string]interface{}{"days": days, "items": items}, http.StatusOK)
	})
}

// providerSwitch handles POST /api/v1/admin/providers/{id}/enable|disable {reason} (administrators only).
func (h *AdminConsoleHandler) providerSwitch(from, to, action string, reasonRequired bool) http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Reason string `json:"reason"`
		}
		if r.ContentLength != 0 {
			if err := readJSONBody(r, &body); err != nil {
				writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
				return
			}
		}
		reason := strings.TrimSpace(body.Reason)
		if (reasonRequired && len(reason) < 3) || len(reason) > domain.MaxModerationReason {
			writeJSON(w, ErrorResponse{Error: Error{Code: "REASON_REQUIRED", Message: domain.ErrReasonRequired.Error()}}, http.StatusBadRequest)
			return
		}
		id := r.PathValue("id")
		if !orgIDPattern.MatchString(id) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Provider not found"}}, http.StatusNotFound)
			return
		}
		org, err := h.reader.SetProviderStatus(r.Context(), id, from, to)
		if errors.Is(err, domain.ErrAccountState) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_TRANSITION", Message: "The provider is not in a status that allows this step"}}, http.StatusConflict)
			return
		}
		if err != nil {
			writeFailure(w, "PROVIDER_UPDATE_FAILED", err)
			return
		}
		if h.logger != nil {
			meta := map[string]interface{}{"organization_id": org, "from": from, "to": to}
			if reason != "" {
				meta["reason"] = reason
			}
			h.logger.Record(r.Context(), domain.AuditEntry{ActorID: auth.UserID(r.Context()), Action: action, TargetType: "provider", TargetID: id, Metadata: meta})
		}
		writeJSON(w, map[string]string{"status": to}, http.StatusOK)
	}, domain.RoleAdmin)
}

// EnableProviderH turns an inactive provider on.
func (h *AdminConsoleHandler) EnableProviderH() http.HandlerFunc {
	return h.providerSwitch("INACTIVE", "ACTIVE", "provider.enabled", false)
}

// DisableProviderH turns an active provider off; a reason is required. Payment services refresh their
// provider list periodically (PAYMENT_PROVIDER_REFRESH_SECONDS).
func (h *AdminConsoleHandler) DisableProviderH() http.HandlerFunc {
	return h.providerSwitch("ACTIVE", "INACTIVE", "provider.disabled", true)
}

// ChannelsH handles GET /api/v1/admin/channels.
func (h *AdminConsoleHandler) ChannelsH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		rows, err := h.reader.ChannelHealth(r.Context(), h.now().Unix())
		if err != nil {
			queryFailed(w, "channels")
			return
		}
		items := make([]map[string]interface{}, 0, len(rows))
		for _, c := range rows {
			m := map[string]interface{}{
				"channel": c.Channel, "events_24h": c.Events24h, "failed_24h": c.Failed24h, "pending": c.Pending,
				"last_error": c.LastError, "accounts": c.Accounts, "conversations_7d": c.Conversations7d,
			}
			if c.LastEventAt != 0 {
				m["last_event_at"] = isoTime(c.LastEventAt)
			}
			if c.LastFailureAt != 0 {
				m["last_failure_at"] = isoTime(c.LastFailureAt)
			}
			items = append(items, m)
		}
		writeJSON(w, map[string]interface{}{"items": items, "configured": h.configuredFlags()}, http.StatusOK)
	})
}

func (h *AdminConsoleHandler) configuredFlags() ConfiguredFlags {
	if h.configured == nil {
		return ConfiguredFlags{}
	}
	return h.configured
}

// sensitiveActions are the audit events the security view shows.
var sensitiveActions = []string{"user.", "provider.", "listing.suspended", "listing.reinstated", "organization.member"}

// SecurityH handles GET /api/v1/admin/security (administrators only: the list shows staff email addresses).
func (h *AdminConsoleHandler) SecurityH() http.HandlerFunc {
	return RequireRoles(h.users, func(w http.ResponseWriter, r *http.Request) {
		staff, err := h.reader.StaffAccounts(r.Context())
		if err != nil {
			queryFailed(w, "staff accounts")
			return
		}
		now := h.now().Unix()
		accounts := make([]map[string]interface{}, 0, len(staff))
		for _, s := range staff {
			m := map[string]interface{}{
				"user_id": s.UserID, "display_name": s.DisplayName, "email": s.Email, "roles": s.Roles, "status": s.Status,
				"failed_attempts": s.FailedAttempts, "locked": s.LockedUntil > now,
			}
			if s.LastLoginAt != 0 {
				m["last_login_at"] = isoTime(s.LastLoginAt)
			}
			if s.LockedUntil > now {
				m["locked_until"] = isoTime(s.LockedUntil)
			}
			if s.PasswordSetAt != 0 {
				m["password_set_at"] = isoTime(s.PasswordSetAt)
			}
			accounts = append(accounts, m)
		}
		events := []map[string]interface{}{}
		if h.audit != nil {
			since := h.now().Add(-30 * 24 * time.Hour).Unix()
			for _, prefix := range sensitiveActions {
				rows, _, err := h.audit.List(r.Context(), domain.AuditFilter{Action: prefix, From: since}, 20, 0)
				if err != nil {
					queryFailed(w, "events")
					return
				}
				for _, e := range rows {
					events = append(events, map[string]interface{}{"id": e.ID, "at": isoTime(e.OccurredAt), "action": e.Action, "actor_id": e.ActorID, "actor_name": e.ActorName, "target_type": e.TargetType, "target_id": e.TargetID})
				}
			}
			sort.SliceStable(events, func(i, j int) bool { return events[i]["at"].(string) > events[j]["at"].(string) })
			if len(events) > 40 {
				events = events[:40]
			}
		}
		writeJSON(w, map[string]interface{}{"staff": accounts, "events": events}, http.StatusOK)
	}, domain.RoleAdmin)
}

// AIH handles GET /api/v1/admin/ai?days=30.
func (h *AdminConsoleHandler) AIH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		days := atoiClamp(r.URL.Query().Get("days"), 30, 1, 365)
		s, err := h.reader.ConversationSummary(r.Context(), h.now().Unix(), days)
		if err != nil {
			queryFailed(w, "assistant activity")
			return
		}
		writeJSON(w, map[string]interface{}{
			"days": s.Days, "timezone": domain.AnalyticsTimeZone, "conversations": s.Conversations, "active_customers": s.ActiveCustomers,
			"by_channel": s.ByChannel, "by_state": s.ByState, "messages_in": s.MessagesIn, "messages_out": s.MessagesOut,
			"confirmations": s.Confirmations, "results_by_type": s.ResultsByType, "per_day": s.PerDay, "ai_execution_records": s.AIExecutions,
			"configured": h.configuredFlags(),
		}, http.StatusOK)
	})
}

// MonetizationH handles GET /api/v1/admin/monetization?days=30.
func (h *AdminConsoleHandler) MonetizationH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		days := atoiClamp(r.URL.Query().Get("days"), 30, 1, 365)
		m, err := h.reader.Monetization(r.Context(), h.now().Add(-time.Duration(days)*24*time.Hour).Unix())
		if err != nil {
			queryFailed(w, "monetization")
			return
		}
		plans := make([]map[string]interface{}, 0, len(m.Plans))
		for _, p := range m.Plans {
			plans = append(plans, map[string]interface{}{
				"id": p.ID, "organization_id": p.OrganizationID, "name": p.Name, "slug": p.Slug, "price_minor": p.PriceMinor, "currency": p.Currency,
				"billing_cycle": p.BillingCycle, "quota": p.QuotaAmount, "credit_pack": p.CreditPack, "status": p.Status, "subscribers": p.Subscribers,
			})
		}
		subs := make([]map[string]interface{}, 0, len(m.Subscriptions))
		for i := range m.Subscriptions {
			subs = append(subs, subscriptionJSON(&m.Subscriptions[i]))
		}
		writeJSON(w, map[string]interface{}{
			"days": days, "plans": plans, "subscriptions_by_status": m.SubscriptionsByStatus, "subscriptions": subs,
			"credits":        map[string]interface{}{"accounts": m.Accounts, "granted": m.Granted, "consumed": m.Consumed, "reserved": m.Reserved},
			"ledger_by_type": m.LedgerByType, "policies": m.Policies,
		}, http.StatusOK)
	})
}

// HealthH handles GET /api/v1/admin/health: each service's readiness probed now (in parallel, 3 s each), the
// database, and the providers' recorded health. Internal addresses are not returned.
func (h *AdminConsoleHandler) HealthH() http.HandlerFunc {
	return h.staff(func(w http.ResponseWriter, r *http.Request) {
		type component struct {
			Name      string `json:"name"`
			Kind      string `json:"kind"`
			Status    string `json:"status"` // ok | degraded | down | unknown
			LatencyMs int64  `json:"latency_ms"`
			Detail    string `json:"detail,omitempty"`
		}
		out := make([]component, len(h.targets))
		var wg sync.WaitGroup
		for i, t := range h.targets {
			wg.Add(1)
			go func(i int, t HealthTarget) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
				defer cancel()
				c := component{Name: t.Name, Kind: "service"}
				start := time.Now()
				req, _ := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
				resp, err := h.client.Do(req)
				c.LatencyMs = time.Since(start).Milliseconds()
				switch {
				case err != nil:
					c.Status, c.Detail = "down", "unreachable"
				case resp.StatusCode == http.StatusOK:
					c.Status = "ok"
				default:
					c.Status, c.Detail = "degraded", http.StatusText(resp.StatusCode)
				}
				if resp != nil {
					resp.Body.Close()
				}
				out[i] = c
			}(i, t)
		}
		wg.Wait()
		if h.db != nil {
			c := component{Name: "postgresql", Kind: "database"}
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			start := time.Now()
			err := h.db.PingContext(ctx)
			cancel()
			c.LatencyMs = time.Since(start).Milliseconds()
			if err != nil {
				c.Status, c.Detail = "down", "unreachable"
			} else {
				c.Status = "ok"
			}
			out = append(out, c)
		}
		providers := map[string]int{}
		if rows, err := h.reader.Providers(r.Context(), "", h.now().Add(-24*time.Hour).Unix()); err == nil {
			for _, p := range rows {
				if p.Status == "ACTIVE" {
					providers[strings.ToLower(p.HealthStatus)]++
				}
			}
		}
		overall := "ok"
		for _, c := range out {
			if c.Status == "down" {
				overall = "down"
				break
			}
			if c.Status != "ok" {
				overall = "degraded"
			}
		}
		writeJSON(w, map[string]interface{}{"status": overall, "checked_at": isoTime(h.now().Unix()), "components": out, "providers_by_health": providers, "configured": h.configuredFlags()}, http.StatusOK)
	})
}

// ParseHealthTargets reads "name=url,name=url"; malformed entries are skipped.
func ParseHealthTargets(raw string) []HealthTarget {
	out := []HealthTarget{}
	for _, part := range strings.Split(raw, ",") {
		name, url, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" || !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
			continue
		}
		out = append(out, HealthTarget{Name: name, URL: url})
	}
	return out
}
