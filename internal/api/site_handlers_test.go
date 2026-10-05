package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeSiteRepo struct {
	content    *domain.SiteContent
	stats      domain.SiteStats
	statsCalls int
	failStats  bool
	savedBy    string
	assets     map[string]fakeAsset
	saveAssets int
}

type fakeAsset struct {
	contentType string
	data        []byte
}

func newFakeSiteRepo() *fakeSiteRepo {
	def := domain.DefaultSiteContent()
	return &fakeSiteRepo{
		content: &def,
		stats: domain.SiteStats{
			domain.SiteMetricLiveListings: 197, domain.SiteMetricVerifiedSellers: 2, domain.SiteMetricDealsLive: 7,
			domain.SiteMetricCitiesCovered: 1, domain.SiteMetricBookingsConfirmed: 0, domain.SiteMetricOrdersCompleted: 0, domain.SiteMetricVerticalsLive: 3,
			domain.SiteMetricRegisteredUsers: 12, domain.SiteMetricMerchants: 20, domain.SiteMetricTransactions: 5,
			domain.SiteMetricConversations: 340, domain.SiteMetricConnections: 41,
		},
		assets: map[string]fakeAsset{},
	}
}

func (f *fakeSiteRepo) Get(context.Context) (*domain.SiteContent, error) {
	c := *f.content
	return &c, nil
}

func (f *fakeSiteRepo) Save(_ context.Context, c *domain.SiteContent, expected int, actor string) (*domain.SiteContent, error) {
	if f.content.Version != expected {
		return nil, domain.ErrSiteContentConflict
	}
	saved := *c
	saved.Version, saved.Stored, saved.UpdatedAt, saved.UpdatedBy = expected+1, true, 1_700_000_000, actor
	f.content, f.savedBy = &saved, actor
	return &saved, nil
}

func (f *fakeSiteRepo) Stats(context.Context) (domain.SiteStats, error) {
	f.statsCalls++
	if f.failStats {
		return nil, errors.New("db down: secret detail")
	}
	out := domain.SiteStats{}
	for k, v := range f.stats {
		out[k] = v
	}
	return out, nil
}

func (f *fakeSiteRepo) SaveAsset(_ context.Context, contentType string, data []byte, _ string) (string, error) {
	f.saveAssets++
	id := "5e11e700-0000-4000-8000-0000000000a1"
	f.assets[id] = fakeAsset{contentType, data}
	return id, nil
}

func (f *fakeSiteRepo) Asset(_ context.Context, id string) (string, []byte, error) {
	if a, ok := f.assets[id]; ok {
		return a.contentType, a.data, nil
	}
	return "", nil, domain.ErrSiteAssetNotFound
}

func siteMux(t *testing.T) (*http.ServeMux, *fakeSiteRepo, *SiteHandler) {
	t.Helper()
	users := fakeAdminUsers{byID: map[string]*domain.User{
		"admin":    {ID: "admin", DisplayName: "Guilbert", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleAdmin}},
		"operator": {ID: "operator", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}},
		"customer": {ID: "customer", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleCustomer}},
		"gone":     {ID: "gone", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleAdmin}},
	}}
	repo := newFakeSiteRepo()
	h := NewSiteHandler(repo, users, zerolog.Nop())
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/site/content", h.PublicContentH())
	mux.HandleFunc("GET /api/v1/site/assets/{id}", h.AssetH())
	mux.HandleFunc("GET /api/v1/admin/site/content", h.AdminContentH())
	mux.HandleFunc("PUT /api/v1/admin/site/content", h.SaveContentH())
	mux.HandleFunc("POST /api/v1/admin/site/assets", h.UploadAssetH())
	return mux, repo, h
}

func do(mux http.Handler, req *http.Request, user string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, asUser(req, user))
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, rec.Body.String())
	}
	return out
}

func put(body interface{}) *http.Request {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/site/content", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPublicSiteContent_ShowsOnlyWhatIsEnabledAndReal(t *testing.T) {
	mux, repo, _ := siteMux(t)
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), "") // anonymous visitors read it
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "public") {
		t.Errorf("the public content is cacheable, got %q", cc)
	}
	body := decode(t, rec)
	trust := body["trust"].([]interface{})
	metrics := map[string]float64{}
	for _, it := range trust {
		m := it.(map[string]interface{})
		metrics[m["metric"].(string)] = m["value"].(float64)
	}
	// The defaults show what Trezutil enables: conversations, connections between customers and businesses, what they
	// did together, and the businesses that are here. Users and partners are available to staff but off.
	want := map[string]float64{"conversations": 340, "connections": 41, "transactions": 5, "merchants": 20}
	if len(metrics) != len(want) {
		t.Fatalf("shown metrics %v, want %v", metrics, want)
	}
	for k, v := range want {
		if metrics[k] != v {
			t.Errorf("%s = %v, want %v (the figure comes from the database)", k, metrics[k], v)
		}
	}
	if _, shown := metrics["live_listings"]; shown {
		t.Error("a metric that is not enabled must not be shown")
	}
	var order []string
	for _, it := range trust {
		order = append(order, it.(map[string]interface{})["metric"].(string))
	}
	if strings.Join(order, ",") != "conversations,connections,transactions,merchants" {
		t.Errorf("the strip reads conversations, connections, transactions, merchants: %v", order)
	}
	if got := len(body["partners"].([]interface{})); got != len(repo.content.Partners) {
		t.Errorf("partners shown %d, want all %d visible", got, len(repo.content.Partners))
	}
	if got := len(body["sections"].([]interface{})); got != len(domain.HomeSections) {
		t.Errorf("sections %d", got)
	}
}

func TestPublicSiteContent_HidesWhatTheEditorHidesAndHonoursMinimums(t *testing.T) {
	mux, repo, _ := siteMux(t)
	repo.content.Trust = []domain.TrustItem{
		{Metric: domain.SiteMetricLiveListings, Enabled: true, MinValue: 500},                        // 197 < 500: not yet
		{Metric: domain.SiteMetricVerifiedSellers, Enabled: true, MinValue: 2, LabelFR: "boutiques"}, // 2 >= 2
		{Metric: domain.SiteMetricOrdersCompleted, Enabled: true, MinValue: 0},                       // shown even at zero: the editor chose that
		{Metric: domain.SiteMetricDealsLive, Enabled: false, MinValue: 1},
	}
	repo.content.Partners = []domain.Partner{
		{ID: "a", Name: "A", Kind: domain.PartnerBank, LogoURL: "/partners/a.png", Visible: true},
		{ID: "b", Name: "B", Kind: domain.PartnerBank, LogoURL: "/partners/b.png", Visible: false},
	}
	body := decode(t, do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), ""))
	var shown []string
	for _, it := range body["trust"].([]interface{}) {
		shown = append(shown, it.(map[string]interface{})["metric"].(string))
	}
	if strings.Join(shown, ",") != "verified_sellers,orders_completed" {
		t.Errorf("trust shown = %v", shown)
	}
	first := body["trust"].([]interface{})[0].(map[string]interface{})
	if first["label_fr"] != "boutiques" {
		t.Errorf("the editor's wording is passed through: %v", first)
	}
	partners := body["partners"].([]interface{})
	if len(partners) != 1 || partners[0].(map[string]interface{})["id"] != "a" {
		t.Errorf("only visible partners are public: %v", partners)
	}
}

func TestPublicSiteContent_ReusesFiguresForAMinuteThenRefreshes(t *testing.T) {
	mux, repo, h := siteMux(t)
	now := time.Unix(1_700_000_000, 0)
	h.now = func() time.Time { return now }
	get := func() int {
		return do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), "").Code
	}
	for i := 0; i < 5; i++ {
		if get() != http.StatusOK {
			t.Fatal("expected 200")
		}
	}
	if repo.statsCalls != 1 {
		t.Fatalf("five visits within a minute must compute the figures once, got %d", repo.statsCalls)
	}
	now = now.Add(statsTTL + time.Second)
	repo.stats[domain.SiteMetricConversations] = 250
	body := decode(t, do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), ""))
	if repo.statsCalls != 2 {
		t.Fatalf("after the TTL the figures are recomputed, calls = %d", repo.statsCalls)
	}
	if v := body["trust"].([]interface{})[0].(map[string]interface{})["value"]; v != float64(250) {
		t.Errorf("fresh figure expected, got %v", v)
	}

	// A refresh that fails keeps serving the last good figures instead of breaking the home page.
	now = now.Add(statsTTL + time.Second)
	repo.failStats = true
	if get() != http.StatusOK {
		t.Error("a failed refresh must fall back to the last good figures")
	}
}

func TestPublicSiteContent_PartnersFigureFollowsTheBandAndNeverTheCachedFigures(t *testing.T) {
	mux, repo, _ := siteMux(t)
	for i := range repo.content.Trust { // the figure is off by default: staff switch it on
		if repo.content.Trust[i].Metric == domain.SiteMetricPartners {
			repo.content.Trust[i].Enabled = true
		}
	}
	repo.content.Partners = []domain.Partner{
		{ID: "a", Name: "A", Kind: domain.PartnerBank, LogoURL: "/partners/a.png", Visible: true},
		{ID: "b", Name: "B", Kind: domain.PartnerBank, LogoURL: "/partners/b.png", Visible: true},
		{ID: "c", Name: "C", Kind: domain.PartnerBank, LogoURL: "/partners/c.png", Visible: false},
	}
	partners := func() (float64, bool) {
		for _, it := range decode(t, do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), ""))["trust"].([]interface{}) {
			if m := it.(map[string]interface{}); m["metric"] == "partners" {
				return m["value"].(float64), true
			}
		}
		return 0, false
	}
	if n, ok := partners(); !ok || n != 2 {
		t.Fatalf("two partners are visible, the figure says %v (shown: %v)", n, ok)
	}
	repo.content.Partners[2].Visible = true // the editor publishes the third one
	if n, _ := partners(); n != 3 {
		t.Errorf("the figure follows the band at once, got %v", n)
	}
	for i := range repo.content.Partners {
		repo.content.Partners[i].Visible = false
	}
	if _, ok := partners(); ok {
		t.Error("with no partner shown, the figure is zero and stays under its minimum of 1: it must not be advertised")
	}
	if _, kept := repo.stats[domain.SiteMetricPartners]; kept {
		t.Error("the partners figure is counted from the content, it is never stored with the platform figures")
	}
}

func TestPublicSiteContent_FailureIsGenericAndWithoutCachedFigures(t *testing.T) {
	mux, repo, _ := siteMux(t)
	repo.failStats = true
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/content", nil), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "secret") || strings.Contains(rec.Body.String(), "db down") {
		t.Errorf("internal detail leaked to the client: %s", rec.Body.String())
	}
}

func TestAdminSiteRoutesAreAdminOnly(t *testing.T) {
	mux, _, _ := siteMux(t)
	cases := []struct {
		method, path, user string
		want               int
	}{
		{http.MethodGet, "/api/v1/admin/site/content", "", http.StatusUnauthorized},
		{http.MethodGet, "/api/v1/admin/site/content", "customer", http.StatusForbidden},
		{http.MethodGet, "/api/v1/admin/site/content", "operator", http.StatusForbidden},
		{http.MethodGet, "/api/v1/admin/site/content", "gone", http.StatusForbidden}, // a suspended admin is shut out at once
		{http.MethodGet, "/api/v1/admin/site/content", "unknown", http.StatusForbidden},
		{http.MethodGet, "/api/v1/admin/site/content", "admin", http.StatusOK},
		{http.MethodPut, "/api/v1/admin/site/content", "", http.StatusUnauthorized},
		{http.MethodPut, "/api/v1/admin/site/content", "customer", http.StatusForbidden},
		{http.MethodPut, "/api/v1/admin/site/content", "operator", http.StatusForbidden},
		{http.MethodPost, "/api/v1/admin/site/assets", "", http.StatusUnauthorized},
		{http.MethodPost, "/api/v1/admin/site/assets", "customer", http.StatusForbidden},
		{http.MethodPost, "/api/v1/admin/site/assets", "operator", http.StatusForbidden},
	}
	for _, c := range cases {
		rec := do(mux, httptest.NewRequest(c.method, c.path, strings.NewReader(`{}`)), c.user)
		if rec.Code != c.want {
			t.Errorf("%s %s as %q = %d, want %d", c.method, c.path, c.user, rec.Code, c.want)
		}
	}
}

func TestAdminSiteContent_ShowsTheEditableDocumentAndTheLiveFigures(t *testing.T) {
	mux, _, _ := siteMux(t)
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/admin/site/content", nil), "admin")
	body := decode(t, rec)
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("the editor must never be cached, got %q", rec.Header().Get("Cache-Control"))
	}
	if body["stored"] != false || body["version"] != float64(0) {
		t.Errorf("nothing was saved yet: stored=%v version=%v", body["stored"], body["version"])
	}
	stats := body["stats"].(map[string]interface{})
	if stats["live_listings"] != float64(197) || stats["bookings_confirmed"] != float64(0) {
		t.Errorf("the editor sees the real figure behind every metric, including zeros: %v", stats)
	}
	content := body["content"].(map[string]interface{})
	if len(content["trust"].([]interface{})) != len(domain.SiteMetrics) || len(content["partners"].([]interface{})) == 0 {
		t.Errorf("the whole document is returned: %v", content)
	}
}

func validSave(version int) map[string]interface{} {
	def := domain.DefaultSiteContent()
	def.Partners = def.Partners[:2]
	return map[string]interface{}{"expected_version": version, "trust": def.Trust, "partners": def.Partners, "sections": def.Sections}
}

func TestSaveSiteContent_SavesWithTheVersionItRead(t *testing.T) {
	mux, repo, _ := siteMux(t)
	rec := do(mux, put(validSave(0)), "admin")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	if body["version"] != float64(1) || body["stored"] != true {
		t.Errorf("version/stored after the first save: %v / %v", body["version"], body["stored"])
	}
	if repo.savedBy != "admin" {
		t.Errorf("the change is attributed to the signed-in administrator, got %q", repo.savedBy)
	}
	if body["updated_by_name"] != "Guilbert" {
		t.Errorf("the editor shows who saved last: %v", body["updated_by_name"])
	}
	if got := len(repo.content.Partners); got != 2 {
		t.Errorf("saved partners = %d", got)
	}
	// A second save must name version 1.
	if rec := do(mux, put(validSave(1)), "admin"); rec.Code != http.StatusOK {
		t.Errorf("second save: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSaveSiteContent_RefusesToOverwriteSomeoneElsesChanges(t *testing.T) {
	mux, repo, _ := siteMux(t)
	if rec := do(mux, put(validSave(0)), "admin"); rec.Code != http.StatusOK {
		t.Fatalf("first save: %d", rec.Code)
	}
	before := repo.content.Version
	rec := do(mux, put(validSave(0)), "admin") // a stale page still names version 0
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409", rec.Code)
	}
	if decode(t, rec)["error"].(map[string]interface{})["code"] != "SITE_CONTENT_CONFLICT" {
		t.Errorf("conflict code: %s", rec.Body.String())
	}
	if repo.content.Version != before {
		t.Error("a refused save must not change anything")
	}
}

func TestSaveSiteContent_RejectsInvalidInputWithAnActionableMessage(t *testing.T) {
	mux, repo, _ := siteMux(t)
	cases := map[string]func(m map[string]interface{}){
		"remote logo": func(m map[string]interface{}) {
			m["partners"] = []domain.Partner{{ID: "x", Name: "X", Kind: domain.PartnerBank, LogoURL: "https://evil.example/pixel.png", Visible: true}}
		},
		"markup in a name": func(m map[string]interface{}) {
			m["partners"] = []domain.Partner{{ID: "x", Name: "<img src=x onerror=alert(1)>", Kind: domain.PartnerBank, LogoURL: "/partners/a.png", Visible: true}}
		},
		"unknown metric": func(m map[string]interface{}) {
			m["trust"] = []domain.TrustItem{{Metric: "visitors_online", Enabled: true}}
		},
	}
	for name, mutate := range cases {
		body := validSave(0)
		mutate(body)
		rec := do(mux, put(body), "admin")
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want 422", name, rec.Code)
			continue
		}
		msg := decode(t, rec)["error"].(map[string]interface{})["message"].(string)
		if !strings.Contains(msg, "[") && !strings.Contains(msg, "metric") {
			t.Errorf("%s: the message should name the field, got %q", name, msg)
		}
	}
	if repo.content.Stored {
		t.Error("an invalid save must not store anything")
	}
}

func TestSaveSiteContent_RejectsMalformedRequests(t *testing.T) {
	mux, _, _ := siteMux(t)
	cases := map[string]string{
		"not json":          `not json`,
		"unknown field":     `{"expected_version":0,"trust":[],"partners":[],"sections":[],"figures":{"live_listings":99999}}`, // a client can never supply a figure
		"negative version":  `{"expected_version":-1,"trust":[],"partners":[],"sections":[]}`,
		"trailing garbage":  `{"expected_version":0} {"x":1}`,
		"wrong type":        `{"expected_version":"zero"}`,
		"oversized payload": `{"expected_version":0,"partners":[` + strings.Repeat(`{"id":"a","name":"`+strings.Repeat("x", 100)+`"},`, 2000) + `{}]}`,
	}
	for name, raw := range cases {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/site/content", strings.NewReader(raw))
		rec := do(mux, req, "admin")
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("%s: status %d, want a 4xx refusal", name, rec.Code)
		}
	}
}

func TestSaveSiteContent_FillsInWhatAnOlderEditorDoesNotKnow(t *testing.T) {
	mux, repo, _ := siteMux(t)
	// An editor from before sections/metrics were added sends fewer of them.
	req := put(map[string]interface{}{
		"expected_version": 0,
		"trust":            []domain.TrustItem{{Metric: domain.SiteMetricLiveListings, Enabled: true, MinValue: 1}},
		"partners":         []domain.Partner{},
		"sections":         []domain.SectionSetting{{Key: domain.SectionNearby, Visible: false}, {Key: "deals", Visible: true}}, // "deals" no longer exists: it is dropped, not an error
	})
	if rec := do(mux, req, "admin"); rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(repo.content.Sections) != len(domain.HomeSections) || len(repo.content.Trust) != len(domain.SiteMetrics) {
		t.Errorf("missing sections/metrics should be completed: %d sections, %d metrics", len(repo.content.Sections), len(repo.content.Trust))
	}
	if repo.content.Sections[0].Key != domain.SectionNearby || repo.content.Sections[0].Visible {
		t.Error("what the editor did send must be kept as sent")
	}
}

// --- uploads -------------------------------------------------------------

func pngBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, 0, color.RGBA{R: 255, A: 255})
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func upload(t *testing.T, field, filename string, data []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(data)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/site/assets", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestUploadSiteAsset_AcceptsARealImageAndDecidesTheTypeFromTheBytes(t *testing.T) {
	mux, repo, _ := siteMux(t)
	rec := do(mux, upload(t, "file", "logo.txt", pngBytes(64, 32)), "admin") // a misleading name changes nothing
	if rec.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	body := decode(t, rec)
	id, _ := body["id"].(string)
	if body["url"] != "/api/v1/site/assets/"+id || body["content_type"] != "image/png" {
		t.Errorf("response: %v", body)
	}
	// The returned url is exactly what a partner's logo_url accepts.
	c := validContent()
	c.Partners[0].LogoURL = body["url"].(string)
	if err := c.Validate(); err != nil {
		t.Errorf("the uploaded url must be accepted as a logo: %v", err)
	}
	if repo.saveAssets != 1 {
		t.Errorf("saved %d times", repo.saveAssets)
	}
}

func validContent() domain.SiteContent {
	c := domain.DefaultSiteContent()
	c.Partners = []domain.Partner{{ID: "x", Name: "X", Kind: domain.PartnerBank, LogoURL: "/partners/a.png", Visible: true}}
	return c
}

func TestUploadSiteAsset_RefusesWhatIsNotAPlainRasterLogo(t *testing.T) {
	mux, repo, _ := siteMux(t)
	webp := append([]byte("RIFF\x24\x00\x00\x00WEBPVP8 "), make([]byte, 32)...)
	cases := []struct {
		name  string
		field string
		data  []byte
		want  int
	}{
		{"svg (can carry scripts)", "file", []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(1)</script></svg>`), http.StatusUnsupportedMediaType},
		{"html", "file", []byte(`<html><script>alert(1)</script></html>`), http.StatusUnsupportedMediaType},
		{"gif renamed .png", "file", []byte("GIF89a\x01\x00\x01\x00\x80\x00\x00\x00\x00\x00\xff\xff\xff!\xf9\x04\x01\x00\x00\x00\x00,\x00\x00\x00\x00\x01\x00\x01\x00\x00\x02\x02D\x01\x00;"), http.StatusUnsupportedMediaType},
		{"empty", "file", nil, http.StatusBadRequest},
		{"png header, damaged body", "file", append(pngBytes(32, 32)[:40], make([]byte, 20)...), http.StatusUnprocessableEntity},
		{"too small", "file", pngBytes(8, 8), http.StatusUnprocessableEntity},
		{"too many pixels", "file", pngBytes(2100, 20), http.StatusUnprocessableEntity},
		{"wrong form field", "image", pngBytes(32, 32), http.StatusBadRequest},
		{"too large", "file", append(pngBytes(32, 32), make([]byte, domain.SiteAssetMaxBytes)...), http.StatusRequestEntityTooLarge},
	}
	for _, c := range cases {
		rec := do(mux, upload(t, c.field, "logo.png", c.data), "admin")
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body.String())
		}
	}
	if repo.saveAssets != 0 {
		t.Errorf("a refused upload must store nothing, stored %d", repo.saveAssets)
	}
	if rec := do(mux, upload(t, "file", "logo.webp", webp), "admin"); rec.Code != http.StatusCreated {
		t.Errorf("a WebP logo is accepted: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSiteAsset_IsServedSoItCanNeverBecomeAPage(t *testing.T) {
	mux, repo, _ := siteMux(t)
	repo.assets["5e11e700-0000-4000-8000-0000000000a1"] = fakeAsset{"image/png", pngBytes(32, 32)}
	rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/assets/5e11e700-0000-4000-8000-0000000000a1", nil), "") // public
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	h := rec.Header()
	if h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("type headers: %v", h)
	}
	if !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") || !strings.Contains(h.Get("Cache-Control"), "immutable") {
		t.Errorf("security/cache headers: %v", h)
	}
	for _, id := range []string{"5e11e700-0000-4000-8000-0000000000ff", "not-an-id"} {
		if rec := do(mux, httptest.NewRequest(http.MethodGet, "/api/v1/site/assets/"+id, nil), ""); rec.Code != http.StatusNotFound {
			t.Errorf("unknown asset %q = %d, want 404", id, rec.Code)
		}
	}
}
