package gateway

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/packages/common"
	"github.com/trezutil/trezutil/packages/observability"
)

const RequestIDHeader = "X-Request-ID"

type ctxKey int

const requestIDKey ctxKey = 0

// New constructs a Gateway from Config.
func New(cfg Config, logger zerolog.Logger) *Gateway {
	if cfg.UpstreamTimeout <= 0 {
		cfg.UpstreamTimeout = 30 * time.Second
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 60 * time.Second
	}
	t := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: cfg.UpstreamTimeout,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
	return &Gateway{config: cfg, logger: logger, transport: t}
}

type Gateway struct {
	config     Config
	logger     zerolog.Logger
	transport  *http.Transport
	ready      atomic.Bool
	cspLimiter reportLimiter
}

// Handler returns the root http.Handler for the gateway.
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", g.handleHealthz)
	mux.HandleFunc("/readyz", g.handleReadyz)
	mux.HandleFunc("/csp-report", g.handleCSPReport)
	// Internet-facing: served only with METRICS_TOKEN set and presented.
	mux.Handle("/metrics", observability.PublicMetricsHandler())
	mux.Handle("/api/v1/", http.HandlerFunc(g.apiRouter))
	mux.Handle("/", http.HandlerFunc(g.staticHandler))
	return g.requestID(g.logging(securityHeaders(g.cors(mux))))
}

type apiRoute struct {
	prefix   string
	upstream string
}

func (g *Gateway) routes() []apiRoute {
	return []apiRoute{
		{"/api/v1/auth", "identity"},
		{"/api/v1/admin", "identity"},
		{"/api/v1/organizations", "identity"},
		{"/api/v1/sellers", "identity"},
		{"/api/v1/stores", "identity"},
		{"/api/v1/site", "identity"},
		{"/api/v1/categories", "search"},
		{"/api/v1/verticals", "search"},
		{"/api/v1/conversations", "conversation"},
		{"/api/v1/packs", "conversation"},
		{"/api/v1/missions", "conversation"},
		{"/api/v1/webhooks/whatsapp", "conversation"},
		{"/api/v1/payments", "payment"},
		{"/api/v1/webhooks/payments", "payment"},
		{"/api/v1/search", "search"},
		{"/api/v1/listings", "search"},
		{"/api/v1/orders", "commerce"},
		{"/api/v1/reservations", "commerce"},
		{"/api/v1/availability", "commerce"},
		{"/api/v1/inventory", "commerce"},
		{"/api/v1/hotels", "commerce"},
		{"/api/v1/fulfillments", "commerce"},
	}
}

// apiRouter dispatches /api/v1/* to configured upstreams, rewriting prefixes
// where internal services use a /v1/* contract.
func (g *Gateway) apiRouter(w http.ResponseWriter, r *http.Request) {
	maxBody := g.config.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = 1 << 20
	}
	if r.ContentLength > maxBody {
		g.writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "request body too large", r)
		return
	}
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	}
	for _, rt := range g.routes() {
		if strings.HasPrefix(r.URL.Path, rt.prefix) {
			up := g.config.Upstreams[rt.upstream]
			if up == nil || up.URL == nil {
				g.writeError(w, http.StatusBadGateway, "UPSTREAM_NOT_CONFIGURED", "upstream service not configured", r)
				return
			}
			to := up.Timeout
			if to <= 0 {
				to = g.config.UpstreamTimeout
			}
			ctx, cancel := context.WithTimeout(r.Context(), to)
			defer cancel()
			g.serveProxy(up, r.WithContext(ctx), w)
			return
		}
	}
	g.writeError(w, http.StatusNotFound, "NOT_FOUND", "route not found", r)
}

func (g *Gateway) serveProxy(up *Upstream, r *http.Request, w http.ResponseWriter) {
	rp := httputil.NewSingleHostReverseProxy(up.URL)
	rp.Transport = g.transport
	rp.ErrorHandler = g.proxyError
	rp.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set(RequestIDHeader, r.Header.Get(RequestIDHeader))
		return nil
	}
	orig := rp.Director
	rp.Director = func(req *http.Request) {
		orig(req)
		if up.StripAPI && strings.HasPrefix(req.URL.Path, "/api") {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
			if req.URL.RawPath != "" {
				req.URL.RawPath = strings.TrimPrefix(req.URL.RawPath, "/api")
			}
		}
		if _, ok := req.Header["X-Forwarded-For"]; !ok {
			req.Header.Set("X-Forwarded-For", clientIP(r))
		}
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Forwarded-Host", r.Host)
		// Authorization and other client headers are forwarded by default;
		// the gateway never synthesizes tenant identity.
	}
	rp.ServeHTTP(w, r)
}

func (g *Gateway) proxyError(w http.ResponseWriter, r *http.Request, err error) {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		g.writeError(w, http.StatusGatewayTimeout, "GATEWAY_TIMEOUT", "upstream request timed out", r)
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		g.writeError(w, http.StatusGatewayTimeout, "GATEWAY_TIMEOUT", "upstream request timed out", r)
		return
	}
	g.writeError(w, http.StatusBadGateway, "UPSTREAM_UNAVAILABLE", "upstream service unavailable", r)
}

// -- endpoints -----------------------------------------------------------

type healthResponse struct {
	Status string `json:"status"`
}

func (g *Gateway) handleHealthz(w http.ResponseWriter, r *http.Request) {
	g.writeJSON(w, r, healthResponse{Status: "ok"}, http.StatusOK)
}

type upstreamStatus struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Healthy bool   `json:"healthy"`
}

type readyResponse struct {
	Status string `json:"status"`
	// Upstreams (names and internal addresses) are shown only to a caller that presents METRICS_TOKEN:
	// the public answer is the status alone, so the topology is not disclosed (finding S-6).
	Upstreams []upstreamStatus `json:"upstreams,omitempty"`
}

// operatorAllowed reports whether the request carries the operator token (METRICS_TOKEN as a bearer token).
func operatorAllowed(r *http.Request) bool {
	token := os.Getenv("METRICS_TOKEN")
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) == 1
}

func (g *Gateway) handleReadyz(w http.ResponseWriter, r *http.Request) {
	statuses := make([]upstreamStatus, 0, len(g.config.Upstreams))
	allUp := true
	for _, up := range g.config.Upstreams {
		st := upstreamStatus{Name: up.Name}
		if up == nil || up.URL == nil || up.URL.Host == "" {
			st.Healthy = false
			st.URL = ""
		} else {
			st.URL = up.URL.String()
			st.Healthy = ping(up.URL.Host, 2*time.Second)
		}
		if !st.Healthy {
			allUp = false
		}
		statuses = append(statuses, st)
	}
	statusCode := http.StatusOK
	gwStatus := "ready"
	if !allUp {
		statusCode = http.StatusServiceUnavailable
		gwStatus = "degraded"
	}
	if !operatorAllowed(r) {
		statuses = nil
	}
	g.writeJSON(w, r, readyResponse{Status: gwStatus, Upstreams: statuses}, statusCode)
}

// -- static SPA ----------------------------------------------------------

func (g *Gateway) staticHandler(w http.ResponseWriter, r *http.Request) {
	root, err := filepath.Abs(g.config.WebRoot)
	if err != nil {
		http.Error(w, "web root not configured", http.StatusInternalServerError)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p != "" && p != "/" {
		full := filepath.Join(root, filepath.FromSlash(filepath.Clean("/"+p)))
		// Never serve anything outside the web root (defense in depth; also
		// covers "\" separators on Windows).
		if rel, err := filepath.Rel(root, full); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			g.writeError(w, http.StatusBadRequest, "INVALID_PATH", "invalid path", r)
			return
		}
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			http.ServeFile(w, r, full)
			return
		}
	}
	http.ServeFile(w, r, filepath.Join(root, "index.html"))
}

// -- helpers -------------------------------------------------------------

func (g *Gateway) writeJSON(w http.ResponseWriter, r *http.Request, v interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set(RequestIDHeader, requestIDFromContext(r.Context()))
	if status != 0 {
		w.WriteHeader(status)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(v)
}

func (g *Gateway) writeError(w http.ResponseWriter, status int, code, message string, r *http.Request) {
	g.writeJSON(w, r, api.ErrorResponse{Error: api.Error{
		Code:      code,
		Message:   message,
		RequestID: requestIDFromContext(r.Context()),
	}}, status)
}

func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

func (g *Gateway) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(RequestIDHeader, id)
		r.Header.Set(RequestIDHeader, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (g *Gateway) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(ww, r)
		elapsed := time.Since(start)
		route := g.gatewayRoute(r.URL.Path)
		observability.HTTPRequests.Inc("api-gateway", r.Method, route, strconv.Itoa(ww.status))
		observability.HTTPDuration.Observe(elapsed.Seconds(), "api-gateway", route)
		event := g.logger.Info()
		if ww.status >= 400 {
			class := "4xx"
			event = g.logger.Warn()
			if ww.status >= 500 {
				class = "5xx"
				event = g.logger.Error()
			}
			observability.HTTPErrors.Inc("api-gateway", route, class)
		}
		event.Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.status).
			Dur("duration", elapsed).
			Str("request_id", requestIDFromContext(r.Context())).
			Str("remote", clientIP(r)).
			Msg("gateway request")
	})
}

// securityHeaders adds the baseline browser protections to every response.
// HSTS is sent only when the request arrived over TLS (directly or through the
// TLS-terminating proxy's X-Forwarded-Proto), so plain-HTTP staging is not pinned.
// A Content-Security-Policy is deliberately left to the reverse proxy: it must
// be tuned to the payment provider's script and frame origins.
func securityHeaders(next http.Handler) http.Handler {
	cspName, cspValue := contentSecurityPolicy(os.Getenv("CSP_MODE"))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		// The microphone is allowed to this site only: voice dictation needs it. The camera is not used.
		h.Set("Permissions-Policy", "camera=(), microphone=(self), geolocation=(self)")
		secure := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		if secure {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if cspName != "" {
			v := cspValue
			if secure {
				v += "; upgrade-insecure-requests"
			}
			h.Set(cspName, v)
		}
		next.ServeHTTP(w, r)
	})
}

// contentSecurityPolicy returns the header to send for CSP_MODE: "enforce", "report-only" (the default:
// violations are reported to /csp-report and logged, nothing is blocked) or "off".
//
// The web app is a single-page app served from this origin. The wide allowances are the ones it needs:
// Stripe.js and its payment frame, Google Fonts, and listing photos from any https address (sellers paste
// links, there is no media upload yet). No inline script is allowed.
func contentSecurityPolicy(mode string) (header, value string) {
	policy := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' https://js.stripe.com",
		"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
		"font-src 'self' https://fonts.gstatic.com",
		"img-src 'self' data: blob: https:",
		"media-src 'self' blob:",
		"connect-src 'self' https://*.stripe.com https://*.stripe.network",
		"frame-src https://*.stripe.com https://*.stripe.network",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"report-uri /csp-report",
	}, "; ")
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off":
		return "", ""
	case "enforce":
		return "Content-Security-Policy", policy
	default:
		return "Content-Security-Policy-Report-Only", policy
	}
}

// handleCSPReport logs what a browser says the policy blocked. The body is small and untrusted: it is
// bounded, parsed for a few known fields only, truncated, and the logging is rate-limited so a flood of
// reports cannot flood the logs.
func (g *Gateway) handleCSPReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<10+1))
	if err != nil || len(body) > 8<<10 {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}
	if g.cspLimiter.allow(time.Now()) {
		var report struct {
			Body struct {
				BlockedURI         string `json:"blocked-uri"`
				ViolatedDirective  string `json:"violated-directive"`
				EffectiveDirective string `json:"effective-directive"`
				DocumentURI        string `json:"document-uri"`
				Disposition        string `json:"disposition"`
			} `json:"csp-report"`
		}
		if json.Unmarshal(body, &report) == nil && report.Body.ViolatedDirective != "" {
			clip := func(s string) string {
				if len(s) > 200 {
					return s[:200]
				}
				return s
			}
			g.logger.Warn().Str("event", "csp_violation").Str("directive", clip(report.Body.ViolatedDirective)).Str("blocked", clip(report.Body.BlockedURI)).
				Str("document", clip(report.Body.DocumentURI)).Str("disposition", clip(report.Body.Disposition)).Msg("content security policy report")
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// reportLimiter allows a few reports per minute in all.
type reportLimiter struct {
	mu     sync.Mutex
	window time.Time
	count  int
}

const cspReportsPerMinute = 30

func (l *reportLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.window) >= time.Minute {
		l.window, l.count = now, 0
	}
	l.count++
	return l.count <= cspReportsPerMinute
}

// gatewayRoute is the bounded metric label of a gateway request: the API
// resource (/api/v1/orders), a probe, or "static" for the web app.
func (g *Gateway) gatewayRoute(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/"):
		for _, rt := range g.routes() {
			if path == rt.prefix || strings.HasPrefix(path, rt.prefix+"/") {
				return rt.prefix
			}
		}
		return "unmatched" // never label by an attacker-chosen path
	case path == "/healthz" || path == "/readyz" || path == "/metrics" || path == "/csp-report":
		return path
	default:
		return "static"
	}
}

func (g *Gateway) cors(next http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range g.config.AllowedOrigins {
		if strings.TrimSpace(o) == "*" {
			allowed["*"] = true
		} else {
			allowed[strings.TrimSpace(o)] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowAll := allowed["*"]
		if allowAll {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if origin != "" && allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		w.Header().Add("Vary", "Origin")
		if r.Method == http.MethodOptions && (allowAll || allowed[origin]) {
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,Idempotency-Key,X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Only the entries appended by our own proxies are trustworthy (see common.ClientIP).
		parts := strings.Split(xff, ",")
		i := len(parts) - common.TrustedProxyHops()
		if i < 0 {
			i = 0
		}
		if i >= len(parts) {
			i = len(parts) - 1
		}
		return strings.TrimSpace(parts[i])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		return r.RemoteAddr
	}
	return host
}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return hexEncode(b)
}

func hexEncode(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexd[v>>4]
		out[i*2+1] = hexd[v&0x0f]
	}
	return string(out)
}

func ping(addr string, timeout time.Duration) bool {
	d := &net.Dialer{Timeout: timeout}
	c, err := d.Dial("tcp", addr)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
