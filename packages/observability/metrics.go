package observability

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// A deliberately small Prometheus-compatible registry (counters and
// histograms in the text exposition format). It has no dependencies and no
// global state beyond Default; every label set is a bounded value (service,
// normalized route, status class, error code), never an identifier.

// Registry holds metric families and renders them.
type Registry struct {
	mu       sync.Mutex
	families []family
}

type family interface {
	write(w io.Writer)
}

// Default is the process-wide registry served by MetricsHandler.
var Default = &Registry{}

func (r *Registry) add(f family) {
	r.mu.Lock()
	r.families = append(r.families, f)
	r.mu.Unlock()
}

// Render writes every family in registration order.
func (r *Registry) Render(w io.Writer) {
	r.mu.Lock()
	fams := append([]family(nil), r.families...)
	r.mu.Unlock()
	for _, f := range fams {
		f.write(w)
	}
}

// Counter is a monotonically increasing value per label set.
type Counter struct {
	name, help string
	labels     []string
	mu         sync.Mutex
	vals       map[string]float64
}

// NewCounter registers a counter with the given label names.
func (r *Registry) NewCounter(name, help string, labels ...string) *Counter {
	c := &Counter{name: name, help: help, labels: labels, vals: map[string]float64{}}
	r.add(c)
	return c
}

// Inc adds one to the series identified by the label values.
func (c *Counter) Inc(values ...string) { c.Add(1, values...) }

// Add adds n to the series identified by the label values.
func (c *Counter) Add(n float64, values ...string) {
	key := seriesKey(len(c.labels), values)
	c.mu.Lock()
	c.vals[key] += n
	c.mu.Unlock()
}

// Value returns the current value of a series (0 when it does not exist).
func (c *Counter) Value(values ...string) float64 {
	key := seriesKey(len(c.labels), values)
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.vals[key]
}

func (c *Counter) write(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", c.name, c.help, c.name)
	c.mu.Lock()
	keys := sortedKeys(c.vals)
	for _, k := range keys {
		fmt.Fprintf(w, "%s%s %s\n", c.name, renderLabels(c.labels, k), formatFloat(c.vals[k]))
	}
	c.mu.Unlock()
}

// Histogram observes values into fixed cumulative buckets per label set.
type Histogram struct {
	name, help string
	labels     []string
	buckets    []float64
	mu         sync.Mutex
	series     map[string]*histSeries
}

type histSeries struct {
	counts []uint64 // per bucket (non-cumulative); the last slot is +Inf
	sum    float64
	count  uint64
}

// NewHistogram registers a histogram; buckets are upper bounds in ascending order.
func (r *Registry) NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	h := &Histogram{name: name, help: help, labels: labels, buckets: buckets, series: map[string]*histSeries{}}
	r.add(h)
	return h
}

// Observe records one value.
func (h *Histogram) Observe(v float64, values ...string) {
	key := seriesKey(len(h.labels), values)
	h.mu.Lock()
	s := h.series[key]
	if s == nil {
		s = &histSeries{counts: make([]uint64, len(h.buckets)+1)}
		h.series[key] = s
	}
	i := sort.SearchFloat64s(h.buckets, v)
	s.counts[i]++
	s.sum += v
	s.count++
	h.mu.Unlock()
}

func (h *Histogram) write(w io.Writer) {
	fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", h.name, h.help, h.name)
	h.mu.Lock()
	defer h.mu.Unlock()
	keys := make([]string, 0, len(h.series))
	for k := range h.series {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		s := h.series[k]
		base := labelPairs(h.labels, k)
		var cum uint64
		for i, ub := range h.buckets {
			cum += s.counts[i]
			fmt.Fprintf(w, "%s_bucket{%s} %d\n", h.name, joinLabels(base, `le="`+formatFloat(ub)+`"`), cum)
		}
		cum += s.counts[len(h.buckets)]
		fmt.Fprintf(w, "%s_bucket{%s} %d\n", h.name, joinLabels(base, `le="+Inf"`), cum)
		fmt.Fprintf(w, "%s_sum%s %s\n", h.name, wrapLabels(base), formatFloat(s.sum))
		fmt.Fprintf(w, "%s_count%s %d\n", h.name, wrapLabels(base), s.count)
	}
}

const sep = "\xff"

// seriesKey joins label values; a wrong number of values is padded or
// truncated rather than panicking, so a metrics bug can never break a request.
func seriesKey(n int, values []string) string {
	vs := make([]string, n)
	copy(vs, values)
	return strings.Join(vs, sep)
}

func labelPairs(names []string, key string) string {
	if len(names) == 0 {
		return ""
	}
	vals := strings.Split(key, sep)
	parts := make([]string, 0, len(names))
	for i, n := range names {
		v := ""
		if i < len(vals) {
			v = vals[i]
		}
		parts = append(parts, n+`="`+escapeLabel(v)+`"`)
	}
	return strings.Join(parts, ",")
}

func renderLabels(names []string, key string) string { return wrapLabels(labelPairs(names, key)) }

func wrapLabels(pairs string) string {
	if pairs == "" {
		return ""
	}
	return "{" + pairs + "}"
}

func joinLabels(a, b string) string {
	if a == "" {
		return b
	}
	return a + "," + b
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return strings.ReplaceAll(v, "\n", `\n`)
}

func formatFloat(f float64) string { return strconv.FormatFloat(f, 'g', -1, 64) }

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// NewCounter registers a counter on the Default registry.
func NewCounter(name, help string, labels ...string) *Counter {
	return Default.NewCounter(name, help, labels...)
}

// The application metrics. Label values are bounded: service names, HTTP
// methods, normalized routes, status classes, error codes, provider ids.
var (
	// HTTPRequests counts every request by service, method, route and status.
	HTTPRequests = NewCounter("trezutil_http_requests_total", "HTTP requests handled.", "service", "method", "route", "status")
	// HTTPErrors counts failed requests: class 4xx (rejected) or 5xx (server failure).
	HTTPErrors = NewCounter("trezutil_http_errors_total", "HTTP responses with status 400 or above.", "service", "route", "class")
	// HTTPDuration is the request latency in seconds.
	HTTPDuration = Default.NewHistogram("trezutil_http_request_duration_seconds", "HTTP request latency.",
		[]float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}, "service", "route")
	// APIErrors counts error responses by their API error code.
	APIErrors = NewCounter("trezutil_api_errors_total", "Error responses by API error code.", "service", "code")

	// PaymentFailures counts payments that failed (declined, provider error).
	PaymentFailures = NewCounter("trezutil_payment_failures_total", "Payments that failed.", "stage")
	// ReservationFailures counts reservation requests that were refused or failed.
	ReservationFailures = NewCounter("trezutil_reservation_failures_total", "Reservation requests that failed.", "reason")
	// ProviderFailures counts failed provider attempts.
	ProviderFailures = NewCounter("trezutil_provider_failures_total", "Failed provider attempts.", "provider", "capability", "error_code")
	// WebhookFailures counts webhooks that were rejected or could not be processed.
	WebhookFailures = NewCounter("trezutil_webhook_failures_total", "Webhook deliveries that failed.", "service", "code")
)

// MetricsHandler serves the Default registry. It is meant for the internal
// network: with METRICS_TOKEN set it requires that bearer token, and without
// one it answers only loopback and private-network clients.
func MetricsHandler() http.Handler { return metricsHandler(false) }

// PublicMetricsHandler is for a service that faces the internet (the gateway):
// it answers only with METRICS_TOKEN set and the matching bearer token.
func PublicMetricsHandler() http.Handler { return metricsHandler(true) }

func metricsHandler(tokenRequired bool) http.Handler {
	token := os.Getenv("METRICS_TOKEN")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if (tokenRequired && token == "") || !metricsAllowed(r, token) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		Default.Render(w)
	})
}

func metricsAllowed(r *http.Request, token string) bool {
	if token != "" {
		return r.Header.Get("Authorization") == "Bearer "+token
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate())
}
