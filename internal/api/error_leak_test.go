package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/common"
)

// Internal error text that must never reach a client: database detail, hosts,
// file paths, credentials and wrapped provider errors.
var internalSecrets = []string{
	"pq: password authentication failed for user \"trezutil\"",
	"dial tcp 10.0.3.7:5432: connect: connection refused",
	"/app/infrastructure/postgres/orders.go:76",
	"sk_live_SECRET",
	"stripe-provider",
}

func leakyError() error {
	return fmt.Errorf("failed to create order: %w", errors.New(strings.Join(internalSecrets, " | ")))
}

// failingOrders fails every creation with an internal error.
type failingOrders struct {
	cartOrders
	single error
}

func (f *failingOrders) CreateOrder(context.Context, string, string, int) (*domain.Order, error) {
	return nil, f.single
}

func (f *failingOrders) CreateOrderIdempotent(context.Context, string, string, int, string) (*domain.Order, error) {
	return nil, f.single
}

// serve runs a handler behind the service middleware (as in production) and
// returns the client-visible response and the server log.
func serve(h http.HandlerFunc, req *http.Request) (*httptest.ResponseRecorder, string) {
	var logs bytes.Buffer
	rec := httptest.NewRecorder()
	common.Observe("test-service", zerolog.New(&logs), h).ServeHTTP(rec, req)
	return rec, logs.String()
}

func assertNoLeak(t *testing.T, name string, rec *httptest.ResponseRecorder) {
	t.Helper()
	wire := rec.Body.String()
	for k, v := range rec.Header() {
		wire += "\n" + k + ": " + strings.Join(v, ",")
	}
	for _, secret := range internalSecrets {
		if strings.Contains(wire, secret) {
			t.Errorf("%s: the response leaked %q:\n%s", name, secret, wire)
		}
	}
	for _, h := range []string{common.ErrorDetailHeader, common.ErrorCodeHeader} {
		if rec.Header().Get(h) != "" {
			t.Errorf("%s: internal header %s reached the client", name, h)
		}
	}
}

func TestOrderCreateInternalErrorsNeverLeak(t *testing.T) {
	cases := map[string]struct {
		body string
		uc   OrderUseCases
	}{
		"single item":            {`{"listing_id":"l1","quantity":1}`, &failingOrders{single: leakyError()}},
		"single item, keyed":     {`{"listing_id":"l1","quantity":1}`, &failingOrders{single: leakyError()}},
		"cart":                   {`{"items":[{"listing_id":"a","quantity":1}]}`, &failingOrders{cartOrders: cartOrders{err: leakyError()}}},
		"single item, wrapped 5": {`{"listing_id":"l1"}`, &failingOrders{single: fmt.Errorf("layer: %w", leakyError())}},
	}
	for name, c := range cases {
		req := asQueryUser(httptest.NewRequest(http.MethodPost, "/v1/orders?user_id=u1", strings.NewReader(c.body)))
		if strings.Contains(name, "keyed") {
			req.Header.Set("Idempotency-Key", "k-1")
		}
		req.Header.Set("X-Request-ID", "req-leak-1")
		rec, logs := serve(NewOrderHandler(c.uc).CreateOrderH, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status %d, want 500: %s", name, rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), `"ORDER_CREATE_FAILED"`) || !strings.Contains(rec.Body.String(), InternalErrorMessage) {
			t.Errorf("%s: not the stable generic body: %s", name, rec.Body)
		}
		assertNoLeak(t, name, rec)
		if rec.Header().Get("X-Request-ID") != "req-leak-1" {
			t.Errorf("%s: request id must be returned so support can find the log line", name)
		}
		// The detail is kept server-side, with the correlation and error code.
		for _, want := range []string{`"level":"error"`, `"request_id":"req-leak-1"`, `"error_code":"ORDER_CREATE_FAILED"`, `"service":"test-service"`, "password authentication failed"} {
			if want == `"service":"test-service"` {
				continue // the service field is added by common.Logger, not by the test logger
			}
			if !strings.Contains(logs, want) {
				t.Errorf("%s: server log lacks %s:\n%s", name, want, logs)
			}
		}
	}
}

func TestOrderCreateUnavailableDependencyIs503(t *testing.T) {
	err := fmt.Errorf("orders: %w", context.DeadlineExceeded)
	req := asQueryUser(httptest.NewRequest(http.MethodPost, "/v1/orders?user_id=u1", strings.NewReader(`{"listing_id":"l1"}`)))
	rec, _ := serve(NewOrderHandler(&failingOrders{single: err}).CreateOrderH, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Fatalf("got %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "deadline") {
		t.Fatal("the cause must not be echoed")
	}
}

func TestKnownOrderErrorsKeepTheirPublicCodes(t *testing.T) {
	for err, want := range map[error]int{
		domain.ErrOrderNotFound:                            http.StatusNotFound,
		fmt.Errorf("wrapped: %w", domain.ErrOrderNotFound): http.StatusNotFound,
		domain.ErrInsufficientPermissions:                  http.StatusForbidden,
		domain.ErrListingUnavailable:                       http.StatusConflict,
		domain.ErrIdempotencyConflict:                      http.StatusConflict,
	} {
		req := asQueryUser(httptest.NewRequest(http.MethodPost, "/v1/orders?user_id=u1", strings.NewReader(`{"listing_id":"l1"}`)))
		rec, _ := serve(NewOrderHandler(&failingOrders{single: err}).CreateOrderH, req)
		if rec.Code != want {
			t.Errorf("%v: status %d, want %d", err, rec.Code, want)
		}
	}
}

// Any handler that reports a 5xx through writeJSON is covered, whatever the
// endpoint: the guarantee lives in writeJSON, not in each call site.
func TestWriteJSONNeverSendsInternalDetailOn5xx(t *testing.T) {
	for _, status := range []int{500, 502, 503} {
		rec, logs := serve(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_CAPTURE_FAILED", Message: leakyError().Error()}}, status)
		}, httptest.NewRequest(http.MethodGet, "/x", nil))
		assertNoLeak(t, fmt.Sprint(status), rec)
		if !strings.Contains(rec.Body.String(), InternalErrorMessage) || !strings.Contains(logs, "error_detail") {
			t.Errorf("%d: body %s logs %s", status, rec.Body, logs)
		}
	}
	// A deliberate 4xx message is unchanged.
	rec, _ := serve(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MIXED_STORES", Message: "an order can only contain items from one store"}}, 400)
	}, httptest.NewRequest(http.MethodGet, "/x", nil))
	if !strings.Contains(rec.Body.String(), "one store") {
		t.Fatalf("4xx message was altered: %s", rec.Body)
	}
}

func TestWriteFailureMapsKnownOutcomesInsteadOfA500(t *testing.T) {
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrInsufficientPermissions, http.StatusForbidden, "FORBIDDEN"},
		{fmt.Errorf("wrapped: %w", domain.ErrInsufficientPermissions), http.StatusForbidden, "FORBIDDEN"},
		{context.DeadlineExceeded, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE"},
		{leakyError(), http.StatusInternalServerError, "LISTING_PUBLISH_FAILED"},
	}
	for _, c := range cases {
		rec, _ := serve(func(w http.ResponseWriter, r *http.Request) { writeFailure(w, "LISTING_PUBLISH_FAILED", c.err) }, httptest.NewRequest(http.MethodPost, "/x", nil))
		if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.code) {
			t.Errorf("%v: got %d %s, want %d %s", c.err, rec.Code, rec.Body, c.status, c.code)
		}
		assertNoLeak(t, c.err.Error(), rec)
	}
}
