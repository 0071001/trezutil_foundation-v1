package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type fakeAdminUsers struct {
	domain.UserRepository
	byID map[string]*domain.User
}

func (f fakeAdminUsers) FindByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := f.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

type fakeAdminReader struct {
	lastQuery          string
	lastLimit, lastOff int
	fail               bool
}

func (f *fakeAdminReader) Overview(_ context.Context, now int64) (*domain.AdminOverview, error) {
	if f.fail {
		return nil, errors.New("db down: secret detail")
	}
	return &domain.AdminOverview{GeneratedAt: now, UsersTotal: 3, UsersActive: 2, UsersSuspended: 1, UsersByRole: map[string]int{"admin": 1}, ListingsByStatus: map[string]int{"published": 5}, OrdersTotal: 4}, nil
}

func (f *fakeAdminReader) ListUsers(_ context.Context, q string, limit, offset int) ([]domain.AdminUser, int, error) {
	f.lastQuery, f.lastLimit, f.lastOff = q, limit, offset
	return []domain.AdminUser{{ID: "u1", Email: "a@b.cd", DisplayName: "A", Status: "active", Roles: []string{"admin"}, CreatedAt: 1_700_000_000}}, 41, nil
}

func adminMux(t *testing.T) (*http.ServeMux, *fakeAdminReader) {
	t.Helper()
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin":    {ID: "admin", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleAdmin}},
		"operator": {ID: "operator", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}},
		"customer": {ID: "customer", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleCustomer}},
		"gone":     {ID: "gone", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleAdmin}},
	}}
	reader := &fakeAdminReader{}
	h := NewAdminHandler(reader, users)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/overview", h.OverviewH())
	mux.HandleFunc("GET /api/v1/admin/users", h.UsersH())
	return mux, reader
}

func asUser(req *http.Request, id string) *http.Request {
	if id == "" {
		return req
	}
	return req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: id}))
}

func TestAdminAreaIsClosedToEveryoneButStaff(t *testing.T) {
	mux, _ := adminMux(t)
	cases := []struct {
		path, user string
		want       int
	}{
		{"/api/v1/admin/overview", "", http.StatusUnauthorized},
		{"/api/v1/admin/users", "", http.StatusUnauthorized},
		{"/api/v1/admin/overview", "customer", http.StatusForbidden},
		{"/api/v1/admin/users", "customer", http.StatusForbidden},
		{"/api/v1/admin/overview", "unknown", http.StatusForbidden},
		{"/api/v1/admin/overview", "gone", http.StatusForbidden}, // a suspended admin is shut out at once
		{"/api/v1/admin/overview", "admin", http.StatusOK},
		{"/api/v1/admin/overview", "operator", http.StatusOK},
		{"/api/v1/admin/users", "operator", http.StatusForbidden}, // the user list shows personal data: admin only
		{"/api/v1/admin/users", "admin", http.StatusOK},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, asUser(httptest.NewRequest(http.MethodGet, c.path+"?user_id=admin", nil), c.user))
		if rec.Code != c.want {
			t.Errorf("%s as %q: got %d, want %d", c.path, c.user, rec.Code, c.want)
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s as %q: admin responses must not be cached", c.path, c.user)
		}
	}
}

func TestAdminOverviewShape(t *testing.T) {
	mux, _ := adminMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil), "admin"))
	var got struct {
		Users struct {
			Total  int            `json:"total"`
			ByRole map[string]int `json:"by_role"`
		} `json:"users"`
		Orders           map[string]int `json:"orders"`
		ListingsByStatus map[string]int `json:"listings_by_status"`
		GeneratedAt      string         `json:"generated_at"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Users.Total != 3 || got.Users.ByRole["admin"] != 1 || got.Orders["total"] != 4 || got.ListingsByStatus["published"] != 5 || got.GeneratedAt == "" {
		t.Fatalf("unexpected overview: %s", rec.Body.String())
	}
}

func TestAdminUsersPaging(t *testing.T) {
	mux, reader := adminMux(t)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?q=gmail&page=3&page_size=10", nil), "admin"))
	if reader.lastQuery != "gmail" || reader.lastLimit != 10 || reader.lastOff != 20 {
		t.Fatalf("query=%q limit=%d offset=%d", reader.lastQuery, reader.lastLimit, reader.lastOff)
	}
	if !strings.Contains(rec.Body.String(), `"total":41`) || !strings.Contains(rec.Body.String(), `"a@b.cd"`) {
		t.Fatalf("body: %s", rec.Body.String())
	}
	// Absurd paging is clamped, never passed to the database as is.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=-4&page_size=100000", nil), "admin"))
	if reader.lastLimit != 100 || reader.lastOff != 0 {
		t.Fatalf("limit=%d offset=%d", reader.lastLimit, reader.lastOff)
	}
}

func TestAdminErrorsDoNotLeakDetail(t *testing.T) {
	mux, reader := adminMux(t)
	reader.fail = true
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, asUser(httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil), "admin"))
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret detail") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
}

// ---- staff sign-in endpoint -------------------------------------------------

type fakeStaffAuth struct {
	IdentityUseCases
	calls int
	err   error
}

func (f *fakeStaffAuth) StaffLogin(_ context.Context, email, password string) (*identity.Session, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &identity.Session{User: &domain.User{ID: "admin", Roles: []domain.Role{domain.RoleAdmin}, DisplayName: "Admin"}, AccessToken: "a", RefreshToken: "r", ExpiresAt: 1_800_000_000}, nil
}

func staffLogin(h *IdentityHandler, body string, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/staff/login", bytes.NewReader([]byte(body)))
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	h.StaffLoginH(rec, req)
	return rec
}

func TestStaffLoginEndpoint(t *testing.T) {
	svc := &fakeStaffAuth{}
	h := NewIdentityHandler(svc)

	rec := staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.0.0.1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"access_token":"a"`) || !strings.Contains(rec.Body.String(), `"admin"`) {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("a sign-in response must not be cached")
	}
	for _, bad := range []string{``, `{}`, `{"email":"a@b.cd"}`, `{"password":"x"}`, `not json`} {
		if rec := staffLogin(h, bad, "10.0.0.2"); rec.Code != http.StatusBadRequest {
			t.Errorf("body %q: got %d, want 400", bad, rec.Code)
		}
	}

	svc.err = domain.ErrInvalidCredentials
	rec = staffLogin(h, `{"email":"a@b.cd","password":"wrong"}`, "10.0.0.3")
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "INVALID_CREDENTIALS") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	svc.err = errors.New("pq: connection refused")
	rec = staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.0.0.4")
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "pq:") {
		t.Fatalf("internal errors must not leak: %d %s", rec.Code, rec.Body.String())
	}
}

func TestStaffLoginIsRateLimitedPerIP(t *testing.T) {
	svc := &fakeStaffAuth{err: domain.ErrInvalidCredentials}
	h := NewIdentityHandler(svc).WithStaffLoginLimit(3)
	for i := 0; i < 3; i++ {
		if rec := staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.9.9.9"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: %d", i, rec.Code)
		}
	}
	if rec := staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.9.9.9"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the fourth attempt from the same address must be refused, got %d", rec.Code)
	}
	if svc.calls != 3 {
		t.Fatalf("a refused attempt must not reach the service: %d calls", svc.calls)
	}
	if rec := staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.9.9.10"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("another address is not limited: %d", rec.Code)
	}
}

func TestStaffLoginIsUnavailableWithoutSupport(t *testing.T) {
	h := NewIdentityHandler(struct{ IdentityUseCases }{})
	if rec := staffLogin(h, `{"email":"a@b.cd","password":"x"}`, "10.0.0.1"); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d", rec.Code)
	}
}
