package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func render(r *Registry) string {
	var b strings.Builder
	r.Render(&b)
	return b.String()
}

func TestCounterAndHistogramExposition(t *testing.T) {
	r := &Registry{}
	c := r.NewCounter("t_total", "help", "a", "b")
	c.Inc("x", "1")
	c.Inc("x", "1")
	c.Add(3, "y", `q"uote`)
	h := r.NewHistogram("t_seconds", "lat", []float64{0.1, 1}, "svc")
	h.Observe(0.05, "s")
	h.Observe(0.5, "s")
	h.Observe(7, "s")

	out := render(r)
	for _, want := range []string{
		"# TYPE t_total counter",
		`t_total{a="x",b="1"} 2`,
		`t_total{a="y",b="q\"uote"} 3`,
		"# TYPE t_seconds histogram",
		`t_seconds_bucket{svc="s",le="0.1"} 1`,
		`t_seconds_bucket{svc="s",le="1"} 2`,
		`t_seconds_bucket{svc="s",le="+Inf"} 3`,
		`t_seconds_sum{svc="s"} 7.55`,
		`t_seconds_count{svc="s"} 3`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if c.Value("x", "1") != 2 {
		t.Fatalf("value = %v", c.Value("x", "1"))
	}
}

func TestWrongLabelCountNeverPanics(t *testing.T) {
	r := &Registry{}
	c := r.NewCounter("t_total", "h", "a", "b")
	c.Inc("only-one")
	c.Inc("1", "2", "3")
	_ = render(r)
}

func TestNormalizeRouteBoundsIdentifiers(t *testing.T) {
	cases := map[string]string{
		"/api/v1/orders/8f14e45f-ceea-467a-9575-1b2c3d4e5f60/confirm": "/api/v1/orders/:id/confirm",
		"/orders/12345":                       "/orders/:id",
		"/orders/incoming":                    "/orders/incoming",
		"/hotels/room-types":                  "/hotels/room-types",
		"/":                                   "/",
		"/x/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa": "/x/:id",
	}
	for in, want := range cases {
		if got := NormalizeRoute(in); got != want {
			t.Errorf("NormalizeRoute(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMetricsHandlerAccess(t *testing.T) {
	get := func(h http.Handler, remote, auth string) int {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.RemoteAddr = remote
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	t.Setenv("METRICS_TOKEN", "")
	internal := MetricsHandler()
	if get(internal, "10.0.0.5:1234", "") != 200 || get(internal, "127.0.0.1:1", "") != 200 {
		t.Fatal("private and loopback clients must be served")
	}
	if get(internal, "203.0.113.9:1234", "") != 404 {
		t.Fatal("a public client must not be served")
	}
	if get(PublicMetricsHandler(), "10.0.0.5:1234", "") != 404 {
		t.Fatal("the public handler needs a token")
	}
	t.Setenv("METRICS_TOKEN", "s3cret")
	pub := PublicMetricsHandler()
	if get(pub, "203.0.113.9:1", "Bearer s3cret") != 200 || get(pub, "203.0.113.9:1", "Bearer wrong") != 404 || get(pub, "203.0.113.9:1", "") != 404 {
		t.Fatal("token check failed")
	}
}
