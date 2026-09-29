package common

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
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
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
	}
}

// Logger returns the standard JSON service logger.
func Logger(service string) zerolog.Logger {
	level, err := zerolog.ParseLevel(Getenv("LOG_LEVEL", "info"))
	if err != nil {
		level = zerolog.InfoLevel
	}
	return zerolog.New(os.Stdout).Level(level).With().Timestamp().Str("service", service).Logger()
}

// AccessLog logs one line per request without query strings (which may carry
// identifiers) or bodies.
func AccessLog(logger zerolog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", sw.status).
			Dur("duration", time.Since(start)).
			Str("request_id", r.Header.Get("X-Request-ID")).
			Msg("request")
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
