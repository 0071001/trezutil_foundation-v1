package common

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/trezutil/trezutil/packages/observability"
)

// NewServer returns an http.Server with production timeouts and a body limit.
func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           LimitBody(DefaultMaxBodyBytes, handler),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// Run serves until SIGINT/SIGTERM, then drains in-flight requests.
func Run(srv *http.Server, logger zerolog.Logger, name string) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info().Str("service", name).Str("addr", srv.Addr).Msg("service starting")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-quit:
	}

	logger.Info().Str("service", name).Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

// HealthHandler reports liveness.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// ReadyHandler reports readiness, including database reachability.
func ReadyHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, code := "ready", http.StatusOK
		if db != nil {
			if err := db.PingContext(ctx); err != nil {
				status, code = "database_unavailable", http.StatusServiceUnavailable
				observability.ReadinessFailures.Inc(status)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}

// Logger returns the standard JSON service logger. It is also installed as
// the process-wide zerolog logger, so shared helpers (observability) write
// the same structured lines with the same service field.
func Logger(service string) zerolog.Logger {
	level, err := zerolog.ParseLevel(Getenv("LOG_LEVEL", "info"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	logger := zerolog.New(os.Stdout).Level(level).With().Timestamp().Str("service", service).Logger()
	log.Logger = logger
	return logger
}

// AccessLog is Observe without a service label.
func AccessLog(logger zerolog.Logger, next http.Handler) http.Handler {
	return Observe("service", logger, next)
}

// Observe is the service's outermost middleware. For every request it:
//   - correlates: keeps a well-formed X-Request-ID (or creates one), returns it
//     in the response and puts it in the request context for logs;
//   - measures: request count, errors, latency and error codes by normalized route;
//   - logs one structured line without query strings or bodies (level follows
//     the status: 5xx error, 4xx warn), including the API error code;
//   - recovers from a panic: logged with the request id, counted, answered 500.
//
// Health, readiness and metrics probes are counted but logged at debug level.
func Observe(service string, logger zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := cleanRequestID(r.Header.Get("X-Request-ID"))
		if id == "" {
			id = newRequestID()
		}
		r.Header.Set("X-Request-ID", id)
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(observability.ContextWithRequestID(r.Context(), id))

		sw := &statusWriter{ResponseWriter: w}
		func() {
			defer func() {
				if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
					logger.Error().Str("event", "panic").Interface("panic", rec).Str("request_id", id).
						Str("method", r.Method).Str("path", r.URL.Path).Bytes("stack", debug.Stack()).Msg("request panicked")
					if !sw.wrote {
						sw.Header().Set("Content-Type", "application/json")
						sw.WriteHeader(http.StatusInternalServerError)
						_, _ = sw.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"Internal error"}}`))
					}
				} else if rec == http.ErrAbortHandler {
					panic(rec)
				}
			}()
			next.ServeHTTP(sw, r)
		}()
		if !sw.wrote {
			sw.status = http.StatusOK
		}

		elapsed := time.Since(start)
		route := observability.NormalizeRoute(r.URL.Path)
		if sw.status == http.StatusNotFound && sw.errCode == "" {
			route = "unmatched" // the mux found no route: never label by an attacker-chosen path
		}
		status := strconv.Itoa(sw.status)
		observability.HTTPRequests.Inc(service, r.Method, route, status)
		observability.HTTPDuration.Observe(elapsed.Seconds(), service, route)
		if sw.status >= 400 {
			class := "4xx"
			if sw.status >= 500 {
				class = "5xx"
			}
			observability.HTTPErrors.Inc(service, route, class)
			if sw.errCode != "" {
				observability.APIErrors.Inc(service, sw.errCode)
			}
		}
		webhookFailed := sw.status >= 400 && strings.HasPrefix(sw.errCode, "WEBHOOK_")
		if webhookFailed {
			observability.WebhookFailures.Inc(service, sw.errCode)
		}

		var event *zerolog.Event
		switch {
		case sw.status >= 500 || webhookFailed:
			event = logger.Error()
		case sw.status >= 400:
			event = logger.Warn()
		case isProbe(r.URL.Path):
			event = logger.Debug()
		default:
			event = logger.Info()
		}
		event.Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", sw.status).
			Dur("duration", elapsed).
			Str("request_id", id)
		if sw.errCode != "" {
			event = event.Str("error_code", sw.errCode)
		}
		if sw.detail != "" {
			event = event.Str("error_detail", sw.detail)
		}
		if webhookFailed {
			event = event.Str("event", "webhook_failed")
		}
		event.Msg("request")
	})
}

func isProbe(path string) bool {
	return path == "/healthz" || path == "/health" || path == "/readyz" || path == "/metrics"
}

// cleanRequestID keeps a caller-supplied id only when it is short and made of
// safe characters (it is echoed in headers and logs).
func cleanRequestID(id string) string {
	if id == "" || len(id) > 64 {
		return ""
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return ""
		}
	}
	return id
}

func newRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// statusWriter records the status and the API error code a handler reports in
// the internal X-Error-Code header; that header is removed before the response
// is sent, so it never reaches clients.
type statusWriter struct {
	http.ResponseWriter
	status  int
	wrote   bool
	errCode string
	detail  string
}

// ErrorCodeHeader is set by handlers that write an API error; the Observe
// middleware consumes and removes it.
const ErrorCodeHeader = "X-Error-Code"

// ErrorDetailHeader carries the raw error text of a 5xx from the handler to the
// middleware's log line; it is removed before the response is sent.
const ErrorDetailHeader = "X-Error-Detail"

func (w *statusWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.status = code
	w.errCode = w.Header().Get(ErrorCodeHeader)
	w.Header().Del(ErrorCodeHeader)
	w.detail = sanitizeDetail(w.Header().Get(ErrorDetailHeader))
	w.Header().Del(ErrorDetailHeader)
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach Flush and friends.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// sanitizeDetail keeps a log-safe, bounded single line.
func sanitizeDetail(d string) string {
	d = strings.Map(func(r rune) rune {
		if r < 0x20 {
			return ' '
		}
		return r
	}, d)
	if len(d) > 500 {
		d = d[:500]
	}
	return d
}
