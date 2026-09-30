package common

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/packages/observability"
)

func observed(h http.HandlerFunc) (http.Handler, *bytes.Buffer) {
	var buf bytes.Buffer
	return Observe("obs-test", zerolog.New(&buf), h), &buf
}

func TestObserveCorrelatesAndCounts(t *testing.T) {
	h, logs := observed(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(ErrorCodeHeader, "LISTING_UNAVAILABLE")
		w.WriteHeader(http.StatusConflict)
	})
	before := observability.HTTPRequests.Value("obs-test", "POST", "/orders", "409")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/orders?secret=1", nil)
	req.Header.Set("X-Request-ID", "req-123")
	h.ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-ID") != "req-123" {
		t.Fatalf("request id not echoed: %q", rec.Header().Get("X-Request-ID"))
	}
	if rec.Header().Get(ErrorCodeHeader) != "" {
		t.Fatal("the internal error-code header leaked to the client")
	}
	if observability.HTTPRequests.Value("obs-test", "POST", "/orders", "409") != before+1 {
		t.Fatal("request not counted")
	}
	if observability.HTTPErrors.Value("obs-test", "/orders", "4xx") < 1 || observability.APIErrors.Value("obs-test", "LISTING_UNAVAILABLE") < 1 {
		t.Fatal("error not counted by class and code")
	}
	line := logs.String()
	for _, want := range []string{`"level":"warn"`, `"request_id":"req-123"`, `"error_code":"LISTING_UNAVAILABLE"`, `"status":409`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line missing %s: %s", want, line)
		}
	}
	if strings.Contains(line, "secret") {
		t.Fatal("query string must not be logged")
	}
}

func TestObserveGeneratesAndSanitizesRequestIDs(t *testing.T) {
	h, _ := observed(func(w http.ResponseWriter, r *http.Request) {
		if observability.RequestIDFromContext(r.Context()) != r.Header.Get("X-Request-ID") {
			t.Error("request id not in the context")
		}
	})
	for _, in := range []string{"", "bad id with spaces", "x\r\nInjected: 1", strings.Repeat("a", 65)} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/x", nil)
		if in != "" {
			req.Header.Set("X-Request-ID", strings.ReplaceAll(in, "\r\n", " "))
		}
		h.ServeHTTP(rec, req)
		id := rec.Header().Get("X-Request-ID")
		if id == "" || id == in || len(id) > 64 || strings.ContainsAny(id, " \r\n") {
			t.Fatalf("input %q -> %q", in, id)
		}
	}
}

func TestObserveRecoversFromPanic(t *testing.T) {
	h, logs := observed(func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/boom", nil))
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), "INTERNAL_ERROR") {
		t.Fatalf("got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(logs.String(), `"event":"panic"`) || !strings.Contains(logs.String(), `"level":"error"`) {
		t.Fatalf("panic not logged: %s", logs.String())
	}
}

func TestObserveCountsWebhookFailuresAndUnmatchedRoutes(t *testing.T) {
	h, logs := observed(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webhooks/x" {
			w.Header().Set(ErrorCodeHeader, "WEBHOOK_SIGNATURE_INVALID")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	})
	before := observability.WebhookFailures.Value("obs-test", "WEBHOOK_SIGNATURE_INVALID")
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/webhooks/x", nil))
	if observability.WebhookFailures.Value("obs-test", "WEBHOOK_SIGNATURE_INVALID") != before+1 {
		t.Fatal("webhook failure not counted")
	}
	if !strings.Contains(logs.String(), `"event":"webhook_failed"`) || !strings.Contains(logs.String(), `"level":"error"`) {
		t.Fatalf("webhook failure not logged as an error: %s", logs.String())
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/random/probe/path", nil))
	if observability.HTTPRequests.Value("obs-test", "GET", "unmatched", "404") != 1 {
		t.Fatal("an unmatched path must be labelled \"unmatched\"")
	}
}
