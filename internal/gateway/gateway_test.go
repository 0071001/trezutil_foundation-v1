package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/api"
)

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("bad url %q: %v", s, err)
	}
	return u
}

func testGateway(t *testing.T, upstreams map[string]*Upstream, origins []string, webRoot string) (*Gateway, *httptest.Server) {
	t.Helper()
	cfg := Config{
		WebRoot:         webRoot,
		AllowedOrigins:  origins,
		Upstreams:       upstreams,
		UpstreamTimeout: 10 * time.Second,
		RequestTimeout:  10 * time.Second,
	}
	g := New(cfg, zerolog.Nop())
	srv := httptest.NewServer(g.Handler())
	t.Cleanup(srv.Close)
	return g, srv
}

func TestRouteMappingPassthrough(t *testing.T) {
	var gotPath, gotMethod string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(conv.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, conv.URL)},
	}, nil, t.TempDir())

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/conversations/message",
		strings.NewReader(`{"message_text":"hi","actor":{"id":"a","type":"guest"}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if gotPath != "/api/v1/conversations/message" {
		t.Fatalf("path: want /api/v1/conversations/message, got %s", gotPath)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method: want POST, got %s", gotMethod)
	}
}

func TestRouteMappingSearchStrip(t *testing.T) {
	var gotPath, gotRawQuery string
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRawQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s"))
	}))
	t.Cleanup(search.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"search": {Name: "search", URL: mustURL(t, search.URL), StripAPI: true},
	}, nil, t.TempDir())

	resp, err := http.Get(srv.URL + "/api/v1/search?q=hello&user_id=anon")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if gotPath != "/v1/search" {
		t.Fatalf("path: want /v1/search, got %s", gotPath)
	}
	if gotRawQuery != "q=hello&user_id=anon" {
		t.Fatalf("query not forwarded: want q=hello&user_id=anon, got %s", gotRawQuery)
	}
}

func TestRouteMappingVerticalsStrip(t *testing.T) {
	var gotPath string
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	t.Cleanup(search.Close)
	_, srv := testGateway(t, map[string]*Upstream{
		"search": {Name: "search", URL: mustURL(t, search.URL), StripAPI: true},
	}, nil, t.TempDir())
	resp, err := http.Get(srv.URL + "/api/v1/verticals")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotPath != "/v1/verticals" {
		t.Fatalf("path: want /v1/verticals, got %s", gotPath)
	}
}

func TestRouteMappingListingsStrip(t *testing.T) {
	var gotPath string
	search := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(search.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"search": {Name: "search", URL: mustURL(t, search.URL), StripAPI: true},
	}, nil, t.TempDir())

	resp, err := http.Get(srv.URL + "/api/v1/listings/abc-123?user_id=anon")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if gotPath != "/v1/listings/abc-123" {
		t.Fatalf("path: want /v1/listings/abc-123, got %s", gotPath)
	}
}

func TestUnknownRoute404(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	resp, err := http.Get(srv.URL + "/api/v1/unknown-route")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: want 404, got %d", resp.StatusCode)
	}
	var e api.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Error.Code != "NOT_FOUND" {
		t.Fatalf("code: want NOT_FOUND, got %s", e.Error.Code)
	}
	id := resp.Header.Get(RequestIDHeader)
	if id == "" {
		t.Fatal("expect X-Request-ID in response")
	}
}

func TestUpstreamUnavailable502(t *testing.T) {
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	closed.Close()

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, closed.URL)},
	}, nil, t.TempDir())

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/conversations/message",
		strings.NewReader(`{"message_text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status: want 502, got %d", resp.StatusCode)
	}
	var e api.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Error.Code != "UPSTREAM_UNAVAILABLE" {
		t.Fatalf("code: want UPSTREAM_UNAVAILABLE, got %s", e.Error.Code)
	}
}

func TestUpstreamTimeout504(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(slow.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {
			Name:    "conversation",
			URL:     mustURL(t, slow.URL),
			Timeout: 100 * time.Millisecond,
		},
	}, nil, t.TempDir())

	resp, err := http.Get(srv.URL + "/api/v1/conversations/x")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusGatewayTimeout {
		t.Fatalf("status: want 504, got %d", resp.StatusCode)
	}
	var e api.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&e); err != nil {
		t.Fatal(err)
	}
	if e.Error.Code != "GATEWAY_TIMEOUT" {
		t.Fatalf("code: want GATEWAY_TIMEOUT, got %s", e.Error.Code)
	}
}

func TestCorrelationIDPropagation(t *testing.T) {
	var gotReqID string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReqID = r.Header.Get(RequestIDHeader)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(conv.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, conv.URL)},
	}, nil, t.TempDir())

	// client-supplied id is preserved and forwarded
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/conversations/x", nil)
	req.Header.Set(RequestIDHeader, "client-id-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if gotReqID != "client-id-123" {
		t.Fatalf("upstream request_id: want client-id-123, got %s", gotReqID)
	}
	if resp.Header.Get(RequestIDHeader) != "client-id-123" {
		t.Fatal("response did not echo X-Request-ID")
	}

	// when client omits it, gateway generates one
	gotReqID = ""
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/conversations/x", nil)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if gotReqID == "" {
		t.Fatal("expected generated request_id forwarded to upstream")
	}
	if resp2.Header.Get(RequestIDHeader) == "" {
		t.Fatal("response missing generated X-Request-ID")
	}
}

func TestAuthorizationHeaderForward(t *testing.T) {
	var gotAuth string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(conv.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, conv.URL)},
	}, nil, t.TempDir())

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/conversations/message",
		strings.NewReader(`{"message_text":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token-xyz")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if gotAuth != "Bearer test-token-xyz" {
		t.Fatalf("upstream Authorization: want Bearer test-token-xyz, got %q", gotAuth)
	}
}

func TestBodyForwarding(t *testing.T) {
	var gotBody string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"response":{"text":"hi back"}}`))
	}))
	t.Cleanup(conv.Close)

	_, srv := testGateway(t, map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, conv.URL)},
	}, nil, t.TempDir())

	body := `{"message_text":"hello body"}`
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/conversations/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: want 200, got %d", resp.StatusCode)
	}
	if gotBody != body {
		t.Fatalf("upstream body:\nwant %s\ngot  %s", body, gotBody)
	}
	out, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(out), "hi back") {
		t.Fatalf("response body not forwarded: %s", out)
	}
}

func TestCORS(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, []string{"http://localhost:9000"}, t.TempDir())

	// ALLOWED origin → preflight 204 + ACAO echo + credentials
	req, _ := http.NewRequest(http.MethodOptions, srv.URL+"/api/v1/conversations/message", nil)
	req.Header.Set("Origin", "http://localhost:9000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("preflight status: want 204, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:9000" {
		t.Fatalf("ACAO: want allowed origin, got %q", got)
	}
	if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatal("credentials flag not set")
	}

	// DISALLOWED origin → no ACAO header
	req2, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/v1/conversations/message", nil)
	req2.Header.Set("Origin", "http://evil.example")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if got := resp2.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no ACAO for disallowed origin, got %q", got)
	}
}

func TestUpstreamConfigMissing(t *testing.T) {
	// search upstream misconfigured/missing -> explicit 502, not 404
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	resp, err := http.Get(srv.URL + "/api/v1/search?q=test")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status: want 502, got %d", resp.StatusCode)
	}
}

func TestSPAStaticFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/index.html", []byte("TREZUTIL SPA"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, srv := testGateway(t, map[string]*Upstream{}, nil, dir)

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "TREZUTIL SPA" {
		t.Fatalf("GET / body: want SPA, got %q", string(b))
	}

	// unknown client route falls back to index (SPA)
	resp2, err := http.Get(srv.URL + "/nonexistent-route")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /nonexistent-route: want 200 fallback, got %d", resp2.StatusCode)
	}
}

func TestHealthz(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var h healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		t.Fatal(err)
	}
	if h.Status != "ok" {
		t.Fatalf("status: want ok, got %q", h.Status)
	}
}

func TestReadyzUpAndDown(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()

	upstreams := map[string]*Upstream{
		"conversation": {Name: "conversation", URL: mustURL(t, slow.URL)},
		"payment":      {Name: "payment", URL: mustURL(t, "http://127.0.0.1:1")},
	}
	_, srv := testGateway(t, upstreams, nil, t.TempDir())
	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status: want 503 (one upstream down), got %d", resp.StatusCode)
	}
	var rdy readyResponse
	if err := json.NewDecoder(resp.Body).Decode(&rdy); err != nil {
		t.Fatal(err)
	}
	if rdy.Status != "degraded" {
		t.Fatalf("status field: want degraded, got %q", rdy.Status)
	}
}

// Finding S-6: the public /readyz used to list every upstream with its internal address.
func TestReadyzPublicAnswerIsTheStatusAlone(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer up.Close()
	upstreams := map[string]*Upstream{"conversation": {Name: "conversation", URL: mustURL(t, up.URL)}}
	_, srv := testGateway(t, upstreams, nil, t.TempDir())
	t.Setenv("METRICS_TOKEN", "operator-secret-token")

	get := func(auth string) (int, string) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/readyz", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}

	for _, auth := range []string{"", "Bearer wrong", "operator-secret-token", "Bearer "} {
		code, body := get(auth)
		if code != http.StatusOK || !strings.Contains(body, `"status":"ready"`) {
			t.Fatalf("auth %q: %d %s", auth, code, body)
		}
		if strings.Contains(body, "http://") || strings.Contains(body, "127.0.0.1") || strings.Contains(body, "conversation") || strings.Contains(body, "upstreams") {
			t.Errorf("auth %q: the public answer discloses the topology: %s", auth, body)
		}
	}
	code, body := get("Bearer operator-secret-token")
	if code != http.StatusOK || !strings.Contains(body, `"name":"conversation"`) || !strings.Contains(body, up.URL) {
		t.Errorf("the operator keeps the detail: %d %s", code, body)
	}

	t.Setenv("METRICS_TOKEN", "")
	if _, body := get("Bearer "); strings.Contains(body, "upstreams") {
		t.Errorf("with no token configured nobody gets the detail: %s", body)
	}
}

func headersOf(t *testing.T, srv *httptest.Server, forwardedProto string) http.Header {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/healthz", nil)
	if forwardedProto != "" {
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.Header
}

func TestSecurityHeaders_MicrophoneIsAllowedToThisSiteForVoiceInput(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	pp := headersOf(t, srv, "").Get("Permissions-Policy")
	if !strings.Contains(pp, "microphone=(self)") || strings.Contains(pp, "microphone=()") {
		t.Errorf("voice dictation needs the microphone: %q", pp)
	}
	if !strings.Contains(pp, "camera=()") {
		t.Errorf("the camera is not used and stays off: %q", pp)
	}
}

func TestContentSecurityPolicy_ReportsOnlyByDefault(t *testing.T) {
	t.Setenv("CSP_MODE", "")
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	h := headersOf(t, srv, "")
	if h.Get("Content-Security-Policy") != "" {
		t.Error("by default the policy only reports, it does not block")
	}
	policy := h.Get("Content-Security-Policy-Report-Only")
	for _, must := range []string{"default-src 'self'", "script-src 'self' https://js.stripe.com", "object-src 'none'", "frame-ancestors 'none'", "base-uri 'self'", "report-uri /csp-report"} {
		if !strings.Contains(policy, must) {
			t.Errorf("the policy lacks %q: %s", must, policy)
		}
	}
	if strings.Contains(policy, "'unsafe-eval'") || regexpScriptUnsafe(policy) {
		t.Errorf("no inline or eval script may be allowed: %s", policy)
	}
}

// regexpScriptUnsafe reports whether script-src allows inline script.
func regexpScriptUnsafe(policy string) bool {
	for _, d := range strings.Split(policy, ";") {
		d = strings.TrimSpace(d)
		if strings.HasPrefix(d, "script-src") && strings.Contains(d, "'unsafe-inline'") {
			return true
		}
	}
	return false
}

func TestContentSecurityPolicy_ModesAndHTTPSUpgrade(t *testing.T) {
	t.Setenv("CSP_MODE", "enforce")
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	plain := headersOf(t, srv, "")
	if plain.Get("Content-Security-Policy") == "" || plain.Get("Content-Security-Policy-Report-Only") != "" {
		t.Errorf("enforce mode sends the blocking header only: %v", plain)
	}
	if strings.Contains(plain.Get("Content-Security-Policy"), "upgrade-insecure-requests") {
		t.Error("over plain http (local staging) nothing is upgraded")
	}
	if !strings.Contains(headersOf(t, srv, "https").Get("Content-Security-Policy"), "upgrade-insecure-requests") {
		t.Error("over https, insecure subresources are upgraded")
	}

	t.Setenv("CSP_MODE", "off")
	_, off := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	h := headersOf(t, off, "")
	if h.Get("Content-Security-Policy") != "" || h.Get("Content-Security-Policy-Report-Only") != "" {
		t.Error("off sends no policy")
	}
}

func TestCSPReport_IsLoggedBoundedAndRateLimited(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	post := func(body string) int {
		resp, err := http.Post(srv.URL+"/csp-report", "application/csp-report", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	report := `{"csp-report":{"blocked-uri":"https://evil.example/x.js","violated-directive":"script-src","document-uri":"https://trezutil.example/","disposition":"report"}}`
	if code := post(report); code != http.StatusNoContent {
		t.Errorf("a report is acknowledged with 204, got %d", code)
	}
	if code := post("not json"); code != http.StatusNoContent {
		t.Errorf("garbage is acknowledged too (a browser retries nothing): %d", code)
	}
	if code := post(strings.Repeat("x", 9<<10)); code != http.StatusRequestEntityTooLarge {
		t.Errorf("an oversized body is refused: %d", code)
	}
	resp, _ := http.Get(srv.URL + "/csp-report")
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("only POST: %d", resp.StatusCode)
	}

	var l reportLimiter
	now := time.Now()
	allowed := 0
	for i := 0; i < 100; i++ {
		if l.allow(now) {
			allowed++
		}
	}
	if allowed != cspReportsPerMinute {
		t.Errorf("a flood of reports logs %d lines, want %d", allowed, cspReportsPerMinute)
	}
	if !l.allow(now.Add(61 * time.Second)) {
		t.Error("the limit resets after a minute")
	}
}

func TestStaticSPAServesRealWeb(t *testing.T) {
	// Use the actual web/index.html if present; otherwise skip.
	if _, err := os.Stat("web/index.html"); err != nil {
		t.Skip("web/index.html not found")
	}
	_, srv := testGateway(t, map[string]*Upstream{}, []string{"http://localhost:8080"}, "web")
	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /: want 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "TREZUTIL") {
		t.Fatalf("expected home page to contain TREZUTIL, got: %q", string(b))
	}
}

func TestRouteMappingFinalizationUpstreams(t *testing.T) {
	hits := map[string]string{}
	mk := func(name string) *Upstream {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits[name] = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(s.Close)
		return &Upstream{Name: name, URL: mustURL(t, s.URL)}
	}
	_, srv := testGateway(t, map[string]*Upstream{
		"identity":     mk("identity"),
		"commerce":     mk("commerce"),
		"conversation": mk("conversation"),
		"payment":      mk("payment"),
	}, nil, t.TempDir())

	cases := map[string]string{
		"/api/v1/auth/request-otp":             "identity",
		"/api/v1/auth/staff/login":             "identity",
		"/api/v1/admin/overview":               "identity",
		"/api/v1/admin/users":                  "identity",
		"/api/v1/orders/o-1":                   "commerce",
		"/api/v1/reservations":                 "commerce",
		"/api/v1/availability":                 "commerce",
		"/api/v1/hotels/property":              "commerce",
		"/api/v1/webhooks/whatsapp":            "conversation",
		"/api/v1/webhooks/payments/mobile-pay": "payment",
	}
	for path, want := range cases {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if hits[want] != path {
			t.Errorf("%s: expected upstream %s to receive the path verbatim, got %q", path, want, hits[want])
		}
	}
}

func TestOversizedBodyRejected(t *testing.T) {
	_, srv := testGateway(t, map[string]*Upstream{}, nil, t.TempDir())
	body := strings.NewReader(strings.Repeat("a", 2<<20))
	resp, err := http.Post(srv.URL+"/api/v1/conversations/message", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestStaticHandlerNeverEscapesWebRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "index.html"), []byte("spa"), 0o644); err != nil {
		t.Fatal(err)
	}
	g, _ := testGateway(t, map[string]*Upstream{}, nil, root)
	for _, p := range []string{"/../secret", `/..\..\secret`, "/a/../../secret"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.URL.Path = p
		rec := httptest.NewRecorder()
		g.staticHandler(rec, req)
		if body := rec.Body.String(); rec.Code == http.StatusOK && body != "spa" {
			t.Errorf("%s: served unexpected content %q", p, body)
		}
	}
}
