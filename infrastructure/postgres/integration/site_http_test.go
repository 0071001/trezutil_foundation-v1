package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// The whole path, with no fakes: HTTP handler -> real repository -> real
// PostgreSQL. The roles and memberships are real rows, so these tests also prove
// that who may read or change what is decided from the database.

type httpFixture struct {
	*analyticsFixture
	mux *http.ServeMux
}

const (
	hAdmin    = "5e11e700-0000-4000-8000-00000000ff01"
	hOperator = "5e11e700-0000-4000-8000-00000000ff02"
	hSeller   = "5e11e700-0000-4000-8000-00000000ff03" // owner of business A
	hOther    = "5e11e700-0000-4000-8000-00000000ff04" // owner of business B
	hStaff    = "5e11e700-0000-4000-8000-00000000ff05" // staff member of business A
)

func newHTTPFixture(t *testing.T) *httpFixture {
	t.Helper()
	f := newAnalyticsFixture(t)
	ctx := context.Background()
	for i, id := range []string{hAdmin, hOperator, hSeller, hOther, hStaff} {
		sellerUser(t, f.db, ctx, id, "+24381000020"+string(rune('1'+i)))
	}
	exec(t, f.db, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'admin'`, hAdmin)
	exec(t, f.db, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'operator'`, hOperator)
	orgs := postgres.NewOrganizationRepository(f.db.DB)
	for _, m := range []*domain.OrganizationMember{
		domain.NewOrganizationMember(hSeller, aOrgA, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(hOther, aOrgB, domain.OrganizationRoleOwner),
		domain.NewOrganizationMember(hStaff, aOrgA, domain.OrganizationRoleStaff),
	} {
		if err := orgs.AddMember(ctx, m); err != nil {
			t.Fatal(err)
		}
	}
	users := postgres.NewUserRepository(f.db.DB)
	site := api.NewSiteHandler(postgres.NewSiteContentRepository(f.db.DB), users, zerolog.Nop())
	analytics := api.NewAnalyticsHandler(postgres.NewAnalyticsRepository(f.db.DB), users, orgs).WithClock(func() time.Time { return time.Unix(f.now, 0) })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/site/content", site.PublicContentH())
	mux.HandleFunc("GET /api/v1/site/assets/{id}", site.AssetH())
	mux.HandleFunc("GET /api/v1/admin/site/content", site.AdminContentH())
	mux.HandleFunc("PUT /api/v1/admin/site/content", site.SaveContentH())
	mux.HandleFunc("POST /api/v1/admin/site/assets", site.UploadAssetH())
	mux.HandleFunc("GET /api/v1/admin/analytics", analytics.AdminH())
	mux.HandleFunc("GET /api/v1/sellers/analytics", analytics.SellerH())
	return &httpFixture{analyticsFixture: f, mux: mux}
}

func (f *httpFixture) do(method, path, user string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not JSON (%d): %s", rec.Code, rec.Body.String())
	}
	return out
}

func TestSiteContent_EndToEndOverHTTP(t *testing.T) {
	f := newHTTPFixture(t)
	defer f.db.Close()

	// The public strip is the platform's own count, from the first request.
	pub := decodeBody(t, f.do(http.MethodGet, "/api/v1/site/content", "", nil, ""))
	shown := map[string]float64{}
	for _, it := range pub["trust"].([]interface{}) {
		m := it.(map[string]interface{})
		shown[m["metric"].(string)] = m["value"].(float64)
	}
	// The defaults show conversations, connections, transactions and merchants. Nobody has talked to Trezutil in this
	// fixture, so conversations stays under its minimum of 1 and is not advertised.
	if shown["connections"] != 3 || shown["transactions"] != 3 || shown["merchants"] != 2 {
		t.Fatalf("public figures = %v", shown)
	}
	for _, off := range []string{"conversations", "registered_users", "partners", "live_listings", "verified_sellers", "deals_live", "cities_covered", "bookings_confirmed", "orders_completed", "verticals_live"} {
		if _, ok := shown[off]; ok {
			t.Errorf("%s is under its minimum or off by default: it must not be public", off)
		}
	}

	// Who may edit.
	for _, who := range []string{"", hSeller, hOperator, hStaff} {
		if rec := f.do(http.MethodGet, "/api/v1/admin/site/content", who, nil, ""); rec.Code == http.StatusOK {
			t.Errorf("%q must not read the editor", who)
		}
		if rec := f.do(http.MethodPut, "/api/v1/admin/site/content", who, []byte(`{"expected_version":0}`), "application/json"); rec.Code == http.StatusOK {
			t.Errorf("%q must not publish", who)
		}
	}

	// An administrator reads the editor (version 0, defaults), changes it and publishes.
	view := decodeBody(t, f.do(http.MethodGet, "/api/v1/admin/site/content", hAdmin, nil, ""))
	if view["version"] != float64(0) || view["stored"] != false {
		t.Fatalf("fresh editor = %v / %v", view["version"], view["stored"])
	}
	content := view["content"].(map[string]interface{})
	trust := content["trust"].([]interface{})
	for _, it := range trust {
		m := it.(map[string]interface{})
		switch m["metric"] {
		case "live_listings":
			m["min_value"] = 500 // 4 listings: not worth showing yet
		case "orders_completed":
			m["enabled"] = true
		}
	}
	partners := content["partners"].([]interface{})
	content["partners"] = partners[:2]
	save, _ := json.Marshal(map[string]interface{}{"expected_version": 0, "trust": trust, "partners": content["partners"], "sections": content["sections"]})
	rec := f.do(http.MethodPut, "/api/v1/admin/site/content", hAdmin, save, "application/json")
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", rec.Code, rec.Body.String())
	}
	if saved := decodeBody(t, rec); saved["version"] != float64(1) || saved["updated_by_name"] == nil {
		t.Errorf("after publish: %v", saved)
	}

	// The public site follows at once: one metric held back by its minimum, one newly on, two partners.
	pub = decodeBody(t, f.do(http.MethodGet, "/api/v1/site/content", "", nil, ""))
	shown = map[string]float64{}
	for _, it := range pub["trust"].([]interface{}) {
		m := it.(map[string]interface{})
		shown[m["metric"].(string)] = m["value"].(float64)
	}
	if _, ok := shown["live_listings"]; ok {
		t.Error("live_listings (4) is below its minimum of 500 and must be hidden")
	}
	if shown["orders_completed"] != 1 {
		t.Errorf("orders_completed should now show its real figure, got %v", shown["orders_completed"])
	}
	if got := len(pub["partners"].([]interface{})); got != 2 {
		t.Errorf("public partners = %d", got)
	}

	// A stale editor (still on version 0) cannot overwrite it.
	if rec := f.do(http.MethodPut, "/api/v1/admin/site/content", hAdmin, save, "application/json"); rec.Code != http.StatusConflict {
		t.Errorf("stale publish = %d, want 409", rec.Code)
	}
	// Nor can anyone put a remote logo on the site.
	bad, _ := json.Marshal(map[string]interface{}{"expected_version": 1, "trust": trust, "sections": content["sections"],
		"partners": []map[string]interface{}{{"id": "x", "name": "X", "kind": "bank", "logo_url": "https://evil.example/p.png", "visible": true}}})
	if rec := f.do(http.MethodPut, "/api/v1/admin/site/content", hAdmin, bad, "application/json"); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("remote logo = %d, want 422", rec.Code)
	}
}

func TestSiteAssets_UploadAndPublicRead(t *testing.T) {
	f := newHTTPFixture(t)
	defer f.db.Close()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		img.Set(x, 5, color.RGBA{B: 200, A: 255})
	}
	var logo bytes.Buffer
	_ = png.Encode(&logo, img)

	var form bytes.Buffer
	mw := multipart.NewWriter(&form)
	part, _ := mw.CreateFormFile("file", "logo.png")
	_, _ = part.Write(logo.Bytes())
	_ = mw.Close()

	if rec := f.do(http.MethodPost, "/api/v1/admin/site/assets", hSeller, form.Bytes(), mw.FormDataContentType()); rec.Code != http.StatusForbidden {
		t.Errorf("a seller must not upload, got %d", rec.Code)
	}
	rec := f.do(http.MethodPost, "/api/v1/admin/site/assets", hAdmin, form.Bytes(), mw.FormDataContentType())
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	url := decodeBody(t, rec)["url"].(string)

	// Anyone can read it, byte for byte, with headers that keep it from ever being a page.
	got := f.do(http.MethodGet, url, "", nil, "")
	if got.Code != http.StatusOK || !bytes.Equal(got.Body.Bytes(), logo.Bytes()) {
		t.Fatalf("read back: %d, %d bytes (want %d)", got.Code, got.Body.Len(), logo.Len())
	}
	if got.Header().Get("Content-Type") != "image/png" || got.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("headers: %v", got.Header())
	}

	// The returned address is accepted as a partner's logo, and a second upload of the same bytes reuses it.
	again := decodeBody(t, f.do(http.MethodPost, "/api/v1/admin/site/assets", hAdmin, form.Bytes(), mw.FormDataContentType()))
	if again["url"] != url {
		t.Errorf("the same file must keep the same address: %v vs %s", again["url"], url)
	}
	body, _ := json.Marshal(map[string]interface{}{
		"expected_version": 0,
		"trust":            domain.DefaultSiteContent().Trust,
		"sections":         domain.DefaultSiteContent().Sections,
		"partners":         []domain.Partner{{ID: "mine", Name: "Mine", Kind: domain.PartnerOther, LogoURL: url, Visible: true}},
	})
	if rec := f.do(http.MethodPut, "/api/v1/admin/site/content", hAdmin, body, "application/json"); rec.Code != http.StatusOK {
		t.Errorf("an uploaded logo must be accepted: %d %s", rec.Code, rec.Body.String())
	}
	if rec := f.do(http.MethodGet, "/api/v1/site/assets/"+strings.Repeat("0", 8)+"-0000-4000-8000-000000000000", "", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("unknown logo = %d", rec.Code)
	}
}

func TestAnalytics_EndToEndOverHTTP(t *testing.T) {
	f := newHTTPFixture(t)
	defer f.db.Close()
	// The handlers read the fixture's clock (2026-10-04 13:00 in Kinshasa), so a 3-day window is exactly the one the repository tests check.
	window := "&days=3"

	// Platform analytics: platform staff only.
	for who, want := range map[string]int{"": 401, hSeller: 403, hStaff: 403, hOther: 403, hAdmin: 200, hOperator: 200} {
		if rec := f.do(http.MethodGet, "/api/v1/admin/analytics?x=1"+window, who, nil, ""); rec.Code != want {
			t.Errorf("admin analytics as %q = %d, want %d", who, rec.Code, want)
		}
	}
	// Business analytics: owner of that business (and platform staff), nobody else, whatever organization_id is asked for.
	get := func(who, org string) *httptest.ResponseRecorder {
		return f.do(http.MethodGet, "/api/v1/sellers/analytics?organization_id="+org+window, who, nil, "")
	}
	for who, want := range map[string]int{hSeller: 200, hAdmin: 200, hOperator: 200, hStaff: 403, hOther: 403, "": 401} {
		if rec := get(who, aOrgA); rec.Code != want {
			t.Errorf("business A analytics as %q = %d, want %d", who, rec.Code, want)
		}
	}
	if rec := get(hSeller, aOrgB); rec.Code != http.StatusForbidden {
		t.Errorf("the owner of A reading B = %d, want 403", rec.Code)
	}
	if rec := get(hSeller, "not-an-id"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed id = %d, want 400", rec.Code)
	}

	// What the owner of A sees is A's: no other business's orders, no platform-wide lists.
	body := decodeBody(t, get(hSeller, aOrgA))
	if _, ok := body["top_organizations"]; ok {
		t.Error("a seller must not receive the platform's business ranking")
	}
	if _, ok := body["new_users"]; ok {
		t.Error("a seller must not receive platform sign-ups")
	}
	current := body["current"].(map[string]interface{})
	if current["orders"] != float64(4) || current["customers"] != float64(3) {
		t.Errorf("A's figures over the whole fixture = %v", current)
	}
	revenue := current["revenue"].(map[string]interface{})
	if revenue["USD"] != float64(3500) || revenue["CDF"] != float64(50000) {
		t.Errorf("A's revenue = %v (B's 1234 must not be in it)", revenue)
	}
	if body["revenue_currencies"].([]interface{})[0] != "USD" {
		t.Errorf("currencies = %v", body["revenue_currencies"])
	}
}
