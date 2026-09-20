package observability

import (
	"context"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger initializes the zerolog logger with the given log level.
func InitLogger(level string) {
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, "request_id", requestID)
}

// RequestIDFromContext extracts the request ID from the context.
func RequestIDFromContext(ctx context.Context) string {
	if rid, ok := ctx.Value("request_id").(string); ok {
		return rid
	}
	return ""
}

// Logger returns the global zerolog logger instance.
func Logger() *zerolog.Logger {
	return &log.Logger
}
