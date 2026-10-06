package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// fakeConsole answers every read with one row and remembers what it was asked.
type fakeConsole struct {
	domain.ConsoleReader
	org    string
	status string
	calls  int
}

func (f *fakeConsole) note(org string) { f.calls++; f.org = org }
func (f *fakeConsole) Payments(_ context.Context, pf domain.PaymentFilter, _ domain.Page) ([]domain.PaymentRow, int, error) {
	f.note(pf.OrganizationID)
	f.status = pf.Status
	return []domain.PaymentRow{{ID: "p-1", OrderID: "o-1", OrganizationID: "org", OrganizationName: "Gombe", AmountMinor: 100, Currency: "USD", Status: "captured", CreatedAt: 1_790_000_000}}, 1, nil
}
func (f *fakeConsole) MoneyTotals(_ context.Context, org string, _, _ int64) ([]domain.MoneyTotals, error) {
	f.note(org)
	return []domain.MoneyTotals{{Currency: "USD", CapturedMinor: 100, NetMinor: 100}}, nil
}
func (f *fakeConsole) Customers(_ context.Context, org, _ string, _ domain.Page) ([]domain.CustomerRow, int, error) {
	f.note(org)
	return []domain.CustomerRow{{CustomerID: "c-1", DisplayName: "Marie", Orders: 2, Spent: map[string]int64{"USD": 100}, FirstAt: 1, LastAt: 2}}, 1, nil
}
func (f *fakeConsole) Team(_ context.Context, org string) ([]domain.TeamMember, error) {
	f.note(org)
	return []domain.TeamMember{{UserID: "owner", DisplayName: "Owner", MaskedPhone: "+243 ••• ••• 61", Role: "owner", JoinedAt: 1}}, nil
}
func (f *fakeConsole) AdminListings(_ context.Context, lf domain.AdminListingFilter, _ domain.Page) ([]domain.AdminListing, int, error) {
	f.calls++
	f.status = lf.Status
	return []domain.AdminListing{{ID: "l-1", Title: "Wax", Status: "published", Flags: []string{}}}, 1, nil
}
func (f *fakeConsole) Providers(_ context.Context, org string, _ int64) ([]domain.ProviderRow, error) {
	f.note(org)
	return []domain.ProviderRow{{ID: "pr-1", Code: "stripe", Status: "ACTIVE", HealthStatus: "HEALTHY", Capabilities: []string{"PAYMENT"}}}, nil
}
func (f *fakeConsole) ChannelAccounts(_ context.Context, org string) ([]domain.ChannelAccountRow, error) {
	f.note(org)
	return []domain.ChannelAccountRow{}, nil
}

func consoleMux(t *testing.T) (*http.ServeMux, *fakeConsole) {
	t.Helper()
	user := func(id string, roles ...domain.Role) *domain.User {
		return &domain.User{ID: id, Status: domain.UserStatusActive, Roles: roles}
	}
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin": user("admin", domain.RoleAdmin), "operator": user("operator", domain.RoleOperator), "customer": user("customer", domain.RoleCustomer),
		"owner": user("owner", domain.RoleCustomer), "manager": user("manager", domain.RoleCustomer), "staff": user("staff", domain.RoleCustomer),
		"ownerB": user("ownerB", domain.RoleCustomer),
		"gone":   {ID: "gone", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleAdmin}},
	}}
	orgs := fakeAnalyticsOrgs{roles: map[string]domain.OrganizationRole{
		orgA + "|owner": domain.OrganizationRoleOwner, orgA + "|manager": domain.OrganizationRoleManager, orgA + "|staff": domain.OrganizationRoleStaff,
		orgB + "|ownerB": domain.OrganizationRoleOwner, orgA + "|gone": domain.OrganizationRoleOwner,
	}}
	c := &fakeConsole{}
	h := NewConsoleHandler(c, users, orgs)
	a := NewAdminConsoleHandler(c, users)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/sellers/finance", h.FinanceH())
	mux.HandleFunc("GET /api/v1/sellers/customers", h.CustomersH())
	mux.HandleFunc("GET /api/v1/sellers/team", h.TeamH())
	mux.HandleFunc("GET /api/v1/sellers/integrations", h.IntegrationsH())
	mux.HandleFunc("GET /api/v1/admin/listings", a.ListingsH())
	mux.HandleFunc("GET /api/v1/admin/security", a.SecurityH())
	return mux, c
}

func TestSellerConsole_OnlyTheBusinessManagersReadItsMoneyAndCustomers(t *testing.T) {
	mux, c := consoleMux(t)
	for _, path := range []string{"/api/v1/sellers/finance", "/api/v1/sellers/customers", "/api/v1/sellers/integrations"} {
		cases := []struct {
			user string
			org  string
			want int
		}{
			{"", orgA, http.StatusUnauthorized},
			{"customer", orgA, http.StatusForbidden},
			{"staff", orgA, http.StatusForbidden},  // a member below manager
			{"ownerB", orgA, http.StatusForbidden}, // the owner of another business
			{"gone", orgA, http.StatusForbidden},   // a suspended account, whatever its roles
			{"owner", "not-a-uuid", http.StatusBadRequest},
			{"owner", orgA, http.StatusOK},
			{"manager", orgA, http.StatusOK},
			{"operator", orgA, http.StatusOK},
		}
		for _, tc := range cases {
			c.calls = 0
			w := get(mux, path+"?organization_id="+tc.org, tc.user)
			if w.Code != tc.want {
				t.Errorf("%s as %q on %s: %d, want %d (%s)", path, tc.user, tc.org, w.Code, tc.want, w.Body)
			}
			if tc.want != http.StatusOK && c.calls != 0 {
				t.Errorf("%s as %q read the data before refusing", path, tc.user)
			}
			if tc.want == http.StatusOK && c.org != orgA {
				t.Errorf("%s read organization %q, want the one asked", path, c.org)
			}
		}
	}
}

func TestSellerConsole_EveryMemberSeesTheTeamButNotTheInvitations(t *testing.T) {
	mux, _ := consoleMux(t)
	if w := get(mux, "/api/v1/sellers/team?organization_id="+orgA, "staff"); w.Code != http.StatusOK || strings.Contains(w.Body.String(), `"invitations"`) ||
		!strings.Contains(w.Body.String(), `"can_manage":false`) {
		t.Fatalf("staff member: %d %s", w.Code, w.Body)
	}
	if w := get(mux, "/api/v1/sellers/team?organization_id="+orgA, "ownerB"); w.Code != http.StatusForbidden {
		t.Fatalf("outsider: %d", w.Code)
	}
}

func TestSellerConsole_UnknownFilterIsRefused(t *testing.T) {
	mux, c := consoleMux(t)
	w := get(mux, "/api/v1/sellers/finance?organization_id="+orgA+"&status=captured%27%3B--", "owner")
	if w.Code != http.StatusBadRequest || c.calls != 0 {
		t.Fatalf("bad status: %d calls=%d", w.Code, c.calls)
	}
}

func TestSellerConsole_CustomersCarryNoContactData(t *testing.T) {
	mux, _ := consoleMux(t)
	w := get(mux, "/api/v1/sellers/customers?organization_id="+orgA, "owner")
	for _, field := range []string{`"phone"`, `"email"`} {
		if strings.Contains(w.Body.String(), field) {
			t.Fatalf("customers expose %s: %s", field, w.Body)
		}
	}
}

func TestAdminConsole_ListingsAreForStaffAndSecurityForAdministrators(t *testing.T) {
	mux, c := consoleMux(t)
	for _, tc := range []struct {
		path, user string
		want       int
	}{
		{"/api/v1/admin/listings", "", http.StatusUnauthorized},
		{"/api/v1/admin/listings", "owner", http.StatusForbidden},
		{"/api/v1/admin/listings", "gone", http.StatusForbidden},
		{"/api/v1/admin/listings", "operator", http.StatusOK},
		{"/api/v1/admin/listings?status=deleted", "admin", http.StatusBadRequest},
		{"/api/v1/admin/listings?vertical=retail%3Bdrop", "admin", http.StatusBadRequest},
		{"/api/v1/admin/security", "operator", http.StatusForbidden}, // staff email addresses: administrators only
	} {
		c.calls = 0
		if w := get(mux, tc.path, tc.user); w.Code != tc.want {
			t.Errorf("%s as %q: %d, want %d", tc.path, tc.user, w.Code, tc.want)
		}
	}
}

func TestProfileUpdates_OnlyCheckedFieldsReachTheUseCase(t *testing.T) {
	for _, tc := range []struct {
		body    map[string]interface{}
		allowed map[string]profileField
		bad     string
	}{
		{map[string]interface{}{"status": "active"}, organizationFields, "status"},
		{map[string]interface{}{"verification_status": "verified"}, organizationFields, "verification_status"},
		{map[string]interface{}{"display_name": "  "}, organizationFields, "display_name"},
		{map[string]interface{}{"email": "not an email"}, storeFields, "email"},
		{map[string]interface{}{"phone": "call me"}, sellerFields, "phone"},
		{map[string]interface{}{"display_name": 12}, sellerFields, "display_name"},
		{map[string]interface{}{"name": strings.Repeat("x", 121)}, storeFields, "name"},
		{map[string]interface{}{}, storeFields, ""},
	} {
		if _, bad := sanitizeProfileUpdates(tc.body, tc.allowed); bad == nil || bad.field != tc.bad {
			t.Errorf("%v accepted or wrong field: %+v", tc.body, bad)
		}
	}
	clean, bad := sanitizeProfileUpdates(map[string]interface{}{"display_name": " Gombe Stays ", "email": "hello@gombe.cd", "city": ""}, organizationFields)
	if bad != nil {
		t.Fatalf("valid edit refused: %+v", bad)
	}
	if clean["display_name"] != "Gombe Stays" {
		t.Errorf("name not trimmed: %q", clean["display_name"])
	}
	if p, ok := clean["email"].(*string); !ok || *p != "hello@gombe.cd" {
		t.Errorf("optional field not passed as *string: %#v", clean["email"])
	}
	if p, ok := clean["city"].(*string); !ok || *p != "" {
		t.Errorf("an emptied optional field must clear it: %#v", clean["city"])
	}
}

func TestParseHealthTargets_SkipsWhatIsNotAURL(t *testing.T) {
	got := ParseHealthTargets("search=http://search:8081/readyz, bad, x=ftp://y, pay=https://p/readyz")
	if len(got) != 2 || got[0].Name != "search" || got[1].Name != "pay" {
		t.Fatalf("targets = %+v", got)
	}
}

var _ = httptest.NewRecorder
