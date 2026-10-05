package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeAnalyticsRepo struct {
	platformCalls int
	orgCalls      []string
	lastDays      int
	fail          bool
}

func sample(platform bool) *domain.Analytics {
	a := &domain.Analytics{
		Days: 3, From: "2026-10-02", To: "2026-10-04",
		Current:        domain.PeriodTotals{Orders: 3, Revenue: map[string]int64{"USD": 5000, "CDF": 100}},
		Previous:       domain.PeriodTotals{Orders: 1, Revenue: map[string]int64{"USD": 1000}},
		Orders:         []domain.DayCount{{Date: "2026-10-02", Count: 1}, {Date: "2026-10-03", Count: 0}, {Date: "2026-10-04", Count: 2}},
		Revenue:        []domain.DayRevenue{{Date: "2026-10-02", Amounts: map[string]int64{}}},
		OrdersByStatus: map[string]int{"paid": 3}, ReservationsByStatus: map[string]int{}, ListingsByStatus: map[string]int{"published": 5},
		ListingsByVertical: map[string]int{"RETAIL": 5},
	}
	if platform {
		a.Users, a.Conversations = []domain.DayCount{{Date: "2026-10-04", Count: 1}}, []domain.DayCount{}
		a.UsersByRole = map[string]int{"customer": 4}
		a.TopOrganizations = []domain.RankedEntry{{ID: "o1", Name: "Gombe Stays", Orders: 3}}
	} else {
		a.TopListings = []domain.RankedEntry{{ID: "l1", Name: "Café", Orders: 2, Units: 4}}
	}
	return a
}

func (f *fakeAnalyticsRepo) Platform(_ context.Context, _ int64, days int) (*domain.Analytics, error) {
	f.platformCalls++
	f.lastDays = days
	if f.fail {
		return nil, errors.New("db down: secret detail")
	}
	return sample(true), nil
}

func (f *fakeAnalyticsRepo) Organization(_ context.Context, org string, _ int64, days int) (*domain.Analytics, error) {
	f.orgCalls = append(f.orgCalls, org)
	f.lastDays = days
	if f.fail {
		return nil, errors.New("db down: secret detail")
	}
	return sample(false), nil
}

type fakeAnalyticsOrgs struct {
	domain.OrganizationRepository
	roles map[string]domain.OrganizationRole // "org|user" -> role
}

func (f fakeAnalyticsOrgs) GetMemberRole(_ context.Context, org, user string) (domain.OrganizationRole, error) {
	if r, ok := f.roles[org+"|"+user]; ok {
		return r, nil
	}
	return "", domain.ErrInsufficientPermissions
}

const (
	orgA = "5e11e700-0000-4000-8000-00000000001a"
	orgB = "5e11e700-0000-4000-8000-00000000001b"
)

func analyticsMux(t *testing.T) (*http.ServeMux, *fakeAnalyticsRepo) {
	t.Helper()
	user := func(id string, roles ...domain.Role) *domain.User {
		return &domain.User{ID: id, Status: domain.UserStatusActive, Roles: roles}
	}
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin": user("admin", domain.RoleAdmin), "operator": user("operator", domain.RoleOperator),
		"customer": user("customer", domain.RoleCustomer),
		"owner":    user("owner", domain.RoleCustomer), "orgadmin": user("orgadmin", domain.RoleCustomer), "manager": user("manager", domain.RoleCustomer),
		"staff": user("staff", domain.RoleCustomer), "accountant": user("accountant", domain.RoleCustomer), "sales": user("sales", domain.RoleCustomer),
		"ownerB": user("ownerB", domain.RoleCustomer),
		"gone":   {ID: "gone", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleCustomer}},
	}}
	orgs := fakeAnalyticsOrgs{roles: map[string]domain.OrganizationRole{
		orgA + "|owner": domain.OrganizationRoleOwner, orgA + "|orgadmin": domain.OrganizationRoleAdmin, orgA + "|manager": domain.OrganizationRoleManager,
		orgA + "|staff": domain.OrganizationRoleStaff, orgA + "|accountant": domain.OrganizationRoleAccountant, orgA + "|sales": domain.OrganizationRoleSales,
		orgA + "|gone":   domain.OrganizationRoleOwner,
		orgB + "|ownerB": domain.OrganizationRoleOwner,
	}}
	repo := &fakeAnalyticsRepo{}
	h := NewAnalyticsHandler(repo, users, orgs)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/analytics", h.AdminH())
	mux.HandleFunc("GET /api/v1/sellers/analytics", h.SellerH())
	return mux, repo
}

func TestAdminAnalytics_IsClosedToEveryoneButPlatformStaff(t *testing.T) {
	mux, repo := analyticsMux(t)
	cases := []struct {
		user string
		want int
	}{
		{"", http.StatusUnauthorized}, {"customer", http.StatusForbidden}, {"owner", http.StatusForbidden}, // a business owner is not platform staff
		{"gone", http.StatusForbidden}, {"unknown", http.StatusForbidden},
		{"admin", http.StatusOK}, {"operator", http.StatusOK},
	}
	for _, c := range cases {
		rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics", nil), c.user)
		if rec.Code != c.want {
			t.Errorf("as %q: %d, want %d", c.user, rec.Code, c.want)
		}
	}
	if repo.platformCalls != 2 {
		t.Errorf("the repository must only be reached by staff: %d calls", repo.platformCalls)
	}
}

func TestAdminAnalytics_ShapeAndWindow(t *testing.T) {
	mux, repo := analyticsMux(t)
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/admin/analytics?days=90", nil), "admin")
	if rec.Code != http.StatusOK || repo.lastDays != 90 {
		t.Fatalf("status %d, days %d", rec.Code, repo.lastDays)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("analytics are never cached: %q", rec.Header().Get("Cache-Control"))
	}
	body := decode(t, rec)
	for _, key := range []string{"current", "previous", "orders", "revenue", "reservations", "new_listings", "new_users", "conversations", "orders_by_status", "users_by_role", "top_organizations", "revenue_currencies", "timezone"} {
		if _, ok := body[key]; !ok {
			t.Errorf("the platform answer lacks %q", key)
		}
	}
	if _, ok := body["top_listings"]; ok {
		t.Error("top listings belong to the seller answer")
	}
	cur := body["revenue_currencies"].([]interface{})
	if len(cur) != 2 || cur[0] != "USD" || cur[1] != "CDF" {
		t.Errorf("currencies are listed USD first, then alphabetically: %v", cur)
	}
	if body["timezone"] != "Africa/Kinshasa" {
		t.Errorf("timezone %v", body["timezone"])
	}
}

func TestAnalyticsDays_AreClampedNotTrusted(t *testing.T) {
	mux, repo := analyticsMux(t)
	cases := map[string]int{"": 30, "7": 7, "0": domain.MinAnalyticsDays, "-5": domain.MinAnalyticsDays, "999999": domain.MaxAnalyticsDays, "abc": 30, "30; DROP TABLE orders": 30}
	for raw, want := range cases {
		path := "/api/v1/admin/analytics"
		if raw != "" {
			path += "?days=" + strings.ReplaceAll(raw, " ", "%20")
		}
		do(mux, httptest.NewRequest(http.MethodGet, path, nil), "admin")
		if repo.lastDays != want {
			t.Errorf("days=%q reached the repository as %d, want %d", raw, repo.lastDays, want)
		}
	}
}

func TestSellerAnalytics_OnlyOwnersAdminsAndManagersOfThatBusiness(t *testing.T) {
	mux, repo := analyticsMux(t)
	get := func(user, org string) int {
		return do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/sellers/analytics?organization_id="+org, nil), user).Code
	}
	allowed := []string{"owner", "orgadmin", "manager", "admin", "operator"}
	for _, u := range allowed {
		if code := get(u, orgA); code != http.StatusOK {
			t.Errorf("%s should read business A's analytics, got %d", u, code)
		}
	}
	denied := []string{"staff", "accountant", "sales", "customer", "ownerB", "gone", "unknown"}
	for _, u := range denied {
		if code := get(u, orgA); code != http.StatusForbidden {
			t.Errorf("%s must not read business A's analytics, got %d", u, code)
		}
	}
	if code := get("", orgA); code != http.StatusUnauthorized {
		t.Errorf("anonymous = %d", code)
	}
	// Business B's owner reads B and only B: the question names the business, the server checks the membership.
	if code := get("ownerB", orgB); code != http.StatusOK {
		t.Errorf("owner of B reading B = %d", code)
	}
	if code := get("owner", orgB); code != http.StatusForbidden {
		t.Errorf("owner of A reading B = %d, want 403", code)
	}
	for _, org := range repo.orgCalls {
		if org != orgA && org != orgB {
			t.Errorf("the repository was asked for an unexpected business %q", org)
		}
	}
	if len(repo.orgCalls) != len(allowed)+1 {
		t.Errorf("only permitted reads may reach the repository: %d calls", len(repo.orgCalls))
	}
}

func TestSellerAnalytics_RequiresAWellFormedBusinessId(t *testing.T) {
	mux, repo := analyticsMux(t)
	for _, org := range []string{"", "not-a-uuid", "1 OR 1=1", "../admin", orgA + "'"} {
		rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/sellers/analytics?organization_id="+strings.ReplaceAll(org, " ", "%20"), nil), "owner")
		if rec.Code != http.StatusBadRequest {
			t.Errorf("organization_id=%q = %d, want 400", org, rec.Code)
		}
	}
	if len(repo.orgCalls) != 0 {
		t.Error("a malformed id must never reach the repository")
	}
}

func TestSellerAnalytics_ShapeIsScopedToTheBusiness(t *testing.T) {
	mux, _ := analyticsMux(t)
	body := decode(t, do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/sellers/analytics?organization_id="+orgA, nil), "owner"))
	if _, ok := body["top_listings"]; !ok {
		t.Error("the seller answer lists the business's best sellers")
	}
	for _, key := range []string{"new_users", "conversations", "users_by_role", "top_organizations"} {
		if _, ok := body[key]; ok {
			t.Errorf("%q is platform-wide and must not be sent to a seller", key)
		}
	}
}

func TestAnalytics_FailureIsGeneric(t *testing.T) {
	mux, repo := analyticsMux(t)
	repo.fail = true
	for _, path := range []string{"/api/v1/admin/analytics", "/api/v1/sellers/analytics?organization_id=" + orgA} {
		rec := do(mux, httptest.NewRequest(http.MethodGet, path, nil), "admin")
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("%s: %d", path, rec.Code)
		}
		if strings.Contains(rec.Body.String(), "secret") {
			t.Errorf("%s leaked internal detail: %s", path, rec.Body.String())
		}
	}
}
