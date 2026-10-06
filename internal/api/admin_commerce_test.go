package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeCommerce struct {
	filter        domain.AdminListFilter
	limit, offset int
	fail          bool
	asked         int
}

func (f *fakeCommerce) note(filter domain.AdminListFilter, limit, offset int) error {
	f.asked++
	f.filter, f.limit, f.offset = filter, limit, offset
	if f.fail {
		return fmt.Errorf("db down: secret detail")
	}
	return nil
}
func (f *fakeCommerce) ListOrders(_ context.Context, filter domain.AdminListFilter, limit, offset int) ([]domain.AdminOrder, int, error) {
	if err := f.note(filter, limit, offset); err != nil {
		return nil, 0, err
	}
	return []domain.AdminOrder{{ID: "o-1", Status: "paid", TotalMinor: 2500, Currency: "USD", Title: "Wax fabric", ItemCount: 2,
		OrganizationID: "org-1", OrganizationName: "Gombe Stays", CustomerID: "c-1", CustomerName: "Marie", CreatedAt: 1_790_000_000, UpdatedAt: 1_790_000_100}}, 41, nil
}
func (f *fakeCommerce) ListReservations(_ context.Context, filter domain.AdminListFilter, limit, offset int) ([]domain.AdminReservation, int, error) {
	if err := f.note(filter, limit, offset); err != nil {
		return nil, 0, err
	}
	return []domain.AdminReservation{{ID: "r-1", Status: "confirmed", Title: "Double room", ListingID: "l-1", OrganizationID: "org-1", OrganizationName: "Gombe Stays",
		CustomerID: "c-1", CustomerName: "Marie", StartAt: 1_790_000_000, EndAt: 1_790_086_400, Quantity: 1, PartySize: 2, CreatedAt: 1_789_000_000}}, 3, nil
}
func (f *fakeCommerce) ListOrganizations(_ context.Context, filter domain.AdminListFilter, limit, offset int) ([]domain.AdminOrganization, int, error) {
	if err := f.note(filter, limit, offset); err != nil {
		return nil, 0, err
	}
	return []domain.AdminOrganization{{ID: "org-1", DisplayName: "Gombe Stays", Type: "business", Status: "active", VerificationStatus: "verified", City: "Kinshasa", Country: "CD",
		Members: 2, Stores: 1, PublishedListings: 7, Orders: 12, CreatedAt: 1_780_000_000}}, 1, nil
}

func commerceMux(t *testing.T, c *fakeCommerce) *http.ServeMux {
	t.Helper()
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin":    {ID: "admin", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleAdmin}},
		"operator": {ID: "operator", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}},
		"customer": {ID: "customer", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleCustomer}},
		"gone":     {ID: "gone", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleAdmin}},
	}}
	h := NewAdminHandler(&fakeAdminReader{}, users).WithCommerce(c)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/orders", h.OrdersH())
	mux.HandleFunc("GET /api/v1/admin/reservations", h.ReservationsH())
	mux.HandleFunc("GET /api/v1/admin/organizations", h.OrganizationsH())
	return mux
}

func get(mux http.Handler, path, user string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, asUser(httptest.NewRequest(http.MethodGet, path, nil), user))
	return w
}

var commercePaths = []string{"/api/v1/admin/orders", "/api/v1/admin/reservations", "/api/v1/admin/organizations"}

func TestAdminCommerceLists_AreForStaffWhoMayRunThePlatform(t *testing.T) {
	c := &fakeCommerce{}
	mux := commerceMux(t, c)
	cases := []struct {
		user string
		want int
	}{{"", http.StatusUnauthorized}, {"customer", http.StatusForbidden}, {"gone", http.StatusForbidden}, {"operator", http.StatusOK}, {"admin", http.StatusOK}}
	for _, path := range commercePaths {
		for _, cs := range cases {
			if w := get(mux, path, cs.user); w.Code != cs.want {
				t.Errorf("%s as %q: %d, want %d", path, cs.user, w.Code, cs.want)
			}
		}
	}
	// only the callers who got through reached the database: 2 per path
	if c.asked != 2*len(commercePaths) {
		t.Fatalf("database reads = %d, want %d", c.asked, 2*len(commercePaths))
	}
}

func TestAdminCommerceLists_PassTheSearchTheStatusAndThePage(t *testing.T) {
	c := &fakeCommerce{}
	mux := commerceMux(t, c)
	w := get(mux, "/api/v1/admin/orders?q=%20marie%20&status=paid&page=3&page_size=10", "operator")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if c.filter.Query != "marie" || c.filter.Status != "paid" || c.limit != 10 || c.offset != 20 {
		t.Fatalf("filter %+v limit %d offset %d", c.filter, c.limit, c.offset)
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
	if body.Total != 41 || body.Page != 3 || body.PageSize != 10 || len(body.Items) != 1 {
		t.Fatalf("body %+v", body)
	}
	o := body.Items[0]
	if o["organization_name"] != "Gombe Stays" || o["customer_name"] != "Marie" || o["total_minor"].(float64) != 2500 || o["item_count"].(float64) != 2 || o["title"] != "Wax fabric" {
		t.Fatalf("order %+v", o)
	}
	// defaults and bounds
	get(mux, "/api/v1/admin/reservations?page_size=100000&page=0", "admin")
	if c.limit != 100 || c.offset != 0 {
		t.Fatalf("limit %d offset %d", c.limit, c.offset)
	}
	long := strings.Repeat("x", 300)
	get(mux, "/api/v1/admin/organizations?q="+long, "admin")
	if len(c.filter.Query) != 100 {
		t.Fatalf("query length %d", len(c.filter.Query))
	}
}

func TestAdminCommerceLists_RefuseAStatusTheyDoNotKnow(t *testing.T) {
	c := &fakeCommerce{}
	mux := commerceMux(t, c)
	for path, status := range map[string]string{
		"/api/v1/admin/orders":        "'; drop table orders; --",
		"/api/v1/admin/reservations":  "paid", // an order status, not a booking status
		"/api/v1/admin/organizations": "verified",
	} {
		w := get(mux, path+"?status="+url.QueryEscape(status), "admin")
		if w.Code != http.StatusBadRequest || errorCode(t, w) != "INVALID_FILTER" {
			t.Errorf("%s status=%s: %d %s", path, status, w.Code, w.Body.String())
		}
	}
	if c.asked != 0 {
		t.Fatalf("an unknown status reached the database (%d reads)", c.asked)
	}
}

func TestAdminCommerceLists_ShowNoContactData(t *testing.T) {
	mux := commerceMux(t, &fakeCommerce{})
	for _, path := range commercePaths {
		body := strings.ToLower(get(mux, path, "operator").Body.String())
		for _, leak := range []string{"phone", "email", "address"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s exposes %q: %s", path, leak, body)
			}
		}
	}
}

func TestAdminCommerceLists_HideDatabaseFailures(t *testing.T) {
	mux := commerceMux(t, &fakeCommerce{fail: true})
	for _, path := range commercePaths {
		w := get(mux, path, "admin")
		if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "secret detail") {
			t.Errorf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}
