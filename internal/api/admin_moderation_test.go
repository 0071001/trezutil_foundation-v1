package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeModerator struct {
	err                    error
	calls                  []string
	lastReason, lastTarget string
}

func (f *fakeModerator) Suspend(_ context.Context, actor, target, reason string) error {
	f.calls, f.lastReason, f.lastTarget = append(f.calls, "suspend:"+actor+":"+target), reason, target
	return f.err
}
func (f *fakeModerator) Reactivate(_ context.Context, actor, target, reason string) error {
	f.calls, f.lastReason, f.lastTarget = append(f.calls, "reactivate:"+actor+":"+target), reason, target
	return f.err
}

type fakeAudit struct {
	filter           domain.AuditFilter
	limit, offset    int
	events           []domain.AuditEvent
	total            int
	fail             bool
	listed           int
	lastRequestedFor string
}

func (f *fakeAudit) List(_ context.Context, filter domain.AuditFilter, limit, offset int) ([]domain.AuditEvent, int, error) {
	f.listed++
	f.filter, f.limit, f.offset = filter, limit, offset
	if f.fail {
		return nil, 0, fmt.Errorf("db down: secret detail")
	}
	return f.events, f.total, nil
}

func moderationMux(t *testing.T, mod *fakeModerator, trail *fakeAudit) *http.ServeMux {
	t.Helper()
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin":    {ID: "admin", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleAdmin}},
		"operator": {ID: "operator", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}},
		"customer": {ID: "customer", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleCustomer}},
	}}
	h := NewAdminHandler(&fakeAdminReader{}, users).WithModeration(mod).WithAudit(trail)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/suspend", h.SuspendUserH())
	mux.HandleFunc("POST /api/v1/admin/users/{user_id}/reactivate", h.ReactivateUserH())
	mux.HandleFunc("GET /api/v1/admin/audit", h.AuditH())
	return mux
}

func post(mux http.Handler, path, user, body string) *httptest.ResponseRecorder {
	req := asUser(httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)), user)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return body.Error.Code
}

func TestModerationRoutes_AreForAdministratorsOnly(t *testing.T) {
	mod := &fakeModerator{}
	mux := moderationMux(t, mod, &fakeAudit{})
	for _, c := range []struct {
		user string
		want int
	}{{"", http.StatusUnauthorized}, {"customer", http.StatusForbidden}, {"operator", http.StatusForbidden}} {
		for _, action := range []string{"suspend", "reactivate"} {
			w := post(mux, "/api/v1/admin/users/customer/"+action, c.user, `{"reason":"because"}`)
			if w.Code != c.want {
				t.Errorf("%q %s: status %d, want %d", c.user, action, w.Code, c.want)
			}
		}
	}
	if len(mod.calls) != 0 {
		t.Fatalf("a refused caller reached the use case: %v", mod.calls)
	}
}

func TestModerationRoutes_PassTheActorTheTargetAndTheReason(t *testing.T) {
	mod := &fakeModerator{}
	mux := moderationMux(t, mod, &fakeAudit{})
	w := post(mux, "/api/v1/admin/users/customer/suspend", "admin", `{"reason":"repeated chargebacks"}`)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"suspended"`) {
		t.Fatalf("suspend: %d %s", w.Code, w.Body.String())
	}
	if mod.calls[0] != "suspend:admin:customer" || mod.lastReason != "repeated chargebacks" {
		t.Fatalf("calls %v, reason %q", mod.calls, mod.lastReason)
	}
	w = post(mux, "/api/v1/admin/users/customer/reactivate", "admin", ``)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"status":"active"`) {
		t.Fatalf("reactivate without a body: %d %s", w.Code, w.Body.String())
	}
	if w := post(mux, "/api/v1/admin/users/customer/suspend", "admin", `{not json`); w.Code != http.StatusBadRequest {
		t.Fatalf("broken JSON: %d", w.Code)
	}
}

func TestModerationRoutes_NameWhyAStepWasRefused(t *testing.T) {
	cases := []struct {
		err  error
		code int
		name string
	}{
		{domain.ErrUserNotFound, http.StatusNotFound, "USER_NOT_FOUND"},
		{domain.ErrReasonRequired, http.StatusUnprocessableEntity, "INVALID_REASON"},
		{domain.ErrCannotModerateSelf, http.StatusConflict, "CANNOT_MODERATE_SELF"},
		{domain.ErrCannotModerateStaff, http.StatusConflict, "STAFF_PROTECTED"},
		{domain.ErrAccountState, http.StatusConflict, "INVALID_STATE"},
		{fmt.Errorf("db down"), http.StatusInternalServerError, "MODERATION_FAILED"},
	}
	for _, c := range cases {
		mux := moderationMux(t, &fakeModerator{err: c.err}, &fakeAudit{})
		w := post(mux, "/api/v1/admin/users/customer/suspend", "admin", `{"reason":"because"}`)
		if w.Code != c.code || errorCode(t, w) != c.name {
			t.Errorf("%v: %d %s, want %d %s", c.err, w.Code, w.Body.String(), c.code, c.name)
		}
		if strings.Contains(w.Body.String(), "db down") {
			t.Errorf("internal detail leaked: %s", w.Body.String())
		}
	}
}

func TestAuditRoute_ListsForStaffAndPassesTheFilters(t *testing.T) {
	trail := &fakeAudit{total: 120, events: []domain.AuditEvent{{
		ID: 7, OccurredAt: 1_790_000_000, ActorID: "admin", ActorName: "Guilbert", Action: "user.suspended",
		TargetType: "user", TargetID: "customer", RequestID: "req-1", Metadata: map[string]interface{}{"reason": "chargebacks"},
	}}}
	mux := moderationMux(t, &fakeModerator{}, trail)

	for _, user := range []string{"", "customer"} {
		req := asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil), user)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
			t.Errorf("%q: status %d", user, w.Code)
		}
	}
	if trail.listed != 0 {
		t.Fatal("a refused caller read the audit trail")
	}

	for _, user := range []string{"admin", "operator"} {
		req := asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?action=user.&target_type=user&target_id=customer&actor_id=admin&from=2026-10-01&to=2026-10-05&page=3&page_size=20", nil), user)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", user, w.Code, w.Body.String())
		}
		var body struct {
			Items    []map[string]interface{} `json:"items"`
			Total    int                      `json:"total"`
			Page     int                      `json:"page"`
			PageSize int                      `json:"page_size"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Total != 120 || body.Page != 3 || body.PageSize != 20 || len(body.Items) != 1 {
			t.Fatalf("body = %+v", body)
		}
		item := body.Items[0]
		if item["action"] != "user.suspended" || item["actor_name"] != "Guilbert" || item["request_id"] != "req-1" || item["at"] == "" {
			t.Fatalf("item = %+v", item)
		}
		if item["metadata"].(map[string]interface{})["reason"] != "chargebacks" {
			t.Fatalf("metadata = %+v", item["metadata"])
		}
	}
	f := trail.filter
	if f.Action != "user." || f.TargetType != "user" || f.TargetID != "customer" || f.ActorID != "admin" || trail.limit != 20 || trail.offset != 40 {
		t.Fatalf("filter = %+v limit %d offset %d", f, trail.limit, trail.offset)
	}
	// "to" as a plain date includes that whole day
	wantFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC).Unix()
	wantTo := time.Date(2026, 10, 5, 23, 59, 59, 0, time.UTC).Unix()
	if f.From != wantFrom || f.To != wantTo {
		t.Fatalf("range = %d..%d, want %d..%d", f.From, f.To, wantFrom, wantTo)
	}
}

func TestAuditRoute_RefusesBadFiltersAndHidesFailures(t *testing.T) {
	trail := &fakeAudit{}
	mux := moderationMux(t, &fakeModerator{}, trail)
	for _, q := range []string{"action=%27%3B%20drop%20table", "from=yesterday", "to=2026-13-45", "actor_id=" + strings.Repeat("a", 65)} {
		req := asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?"+q, nil), "admin")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest || errorCode(t, w) != "INVALID_FILTER" {
			t.Errorf("%s: %d %s", q, w.Code, w.Body.String())
		}
	}
	if trail.listed != 0 {
		t.Fatal("an invalid filter reached the store")
	}

	trail.fail = true
	req := asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil), "admin")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "secret detail") {
		t.Fatalf("failure: %d %s", w.Code, w.Body.String())
	}
}
