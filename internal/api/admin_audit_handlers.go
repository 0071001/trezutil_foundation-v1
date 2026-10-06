package api

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// AuditLister reads the durable audit trail (domain.AuditStore).
type AuditLister interface {
	List(ctx context.Context, filter domain.AuditFilter, limit, offset int) ([]domain.AuditEvent, int, error)
}

// WithAudit attaches the audit trail that GET /api/v1/admin/audit reads.
func (h *AdminHandler) WithAudit(store AuditLister) *AdminHandler {
	h.audit = store
	return h
}

// auditToken is what an action name, a target type or an id may look like in a filter.
var auditToken = regexp.MustCompile(`^[A-Za-z0-9._:@-]{1,64}$`)

// AuditH handles GET /api/v1/admin/audit?action=&actor_id=&target_type=&target_id=&from=&to=&page=&page_size=
// (admin or operator). action matches the start of the name ("order." lists every order event); from and to
// are RFC 3339 times or plain dates (to includes the whole day). Newest first.
func (h *AdminHandler) AuditH() http.HandlerFunc {
	return h.require(func(w http.ResponseWriter, r *http.Request) {
		if h.audit == nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "The audit trail is not available"}}, http.StatusServiceUnavailable)
			return
		}
		q := r.URL.Query()
		filter := domain.AuditFilter{Action: q.Get("action"), ActorID: q.Get("actor_id"), TargetType: q.Get("target_type"), TargetID: q.Get("target_id")}
		for name, value := range map[string]string{"action": filter.Action, "actor_id": filter.ActorID, "target_type": filter.TargetType, "target_id": filter.TargetID} {
			if value != "" && !auditToken.MatchString(value) {
				writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_FILTER", Message: "The filter " + name + " is not valid"}}, http.StatusBadRequest)
				return
			}
		}
		var err error
		if filter.From, err = parseAuditTime(q.Get("from"), false); err == nil {
			filter.To, err = parseAuditTime(q.Get("to"), true)
		}
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_FILTER", Message: "Dates must be like 2026-10-05 or 2026-10-05T14:30:00Z"}}, http.StatusBadRequest)
			return
		}
		pageSize := atoiClamp(q.Get("page_size"), 50, 1, 200)
		page := atoiClamp(q.Get("page"), 1, 1, 100000)

		events, total, err := h.audit.List(r.Context(), filter, pageSize, (page-1)*pageSize)
		if err != nil {
			writeJSON(w, ErrorResponse{Error: Error{Code: "ADMIN_QUERY_FAILED", Message: "The audit trail could not be loaded"}}, http.StatusInternalServerError)
			return
		}
		items := make([]map[string]interface{}, 0, len(events))
		for _, e := range events {
			items = append(items, map[string]interface{}{
				"id": e.ID, "at": isoTime(e.OccurredAt), "actor_id": e.ActorID, "actor_name": e.ActorName,
				"action": e.Action, "target_type": e.TargetType, "target_id": e.TargetID, "request_id": e.RequestID, "metadata": e.Metadata,
			})
		}
		writeJSON(w, map[string]interface{}{"items": items, "total": total, "page": page, "page_size": pageSize}, http.StatusOK)
	}, domain.RoleAdmin, domain.RoleOperator)
}

// parseAuditTime reads an RFC 3339 time or a plain date; "" is 0 (no bound). A plain date as an upper bound
// means the end of that day.
func parseAuditTime(s string, upper bool) (int64, error) {
	if s == "" {
		return 0, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return 0, err
	}
	if upper {
		t = t.Add(24*time.Hour - time.Second)
	}
	return t.Unix(), nil
}
