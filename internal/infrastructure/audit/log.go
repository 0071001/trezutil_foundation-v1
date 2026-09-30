// Package audit provides the domain.AuditLogger used by the services: every
// critical commerce action (order created or confirmed, reservation made or
// cancelled, listing published, inventory or capacity changed…) becomes one
// structured log line with the actor, the action, the target and the request id.
//
// This is a log trail, not a tamper-proof store: retention and search are the
// log platform's. A durable audit table can implement the same interface later.
package audit

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

// Logger writes audit entries as JSON log lines (event="audit").
type Logger struct {
	logger zerolog.Logger
}

// NewLogger returns an audit logger writing to the given service logger.
func NewLogger(logger zerolog.Logger) *Logger { return &Logger{logger: logger} }

// Record implements domain.AuditLogger. It never fails: an audit line must not
// break the action it describes.
func (l *Logger) Record(ctx context.Context, entry domain.AuditEntry) error {
	requestID := entry.RequestID
	if requestID == "" {
		requestID = observability.RequestIDFromContext(ctx)
	}
	event := l.logger.Info().
		Str("event", "audit").
		Str("action", entry.Action).
		Str("actor_id", entry.ActorID).
		Str("target_type", entry.TargetType).
		Str("target_id", entry.TargetID).
		Str("request_id", requestID)
	if entry.OccurredAt != 0 {
		event = event.Time("occurred_at", time.Unix(entry.OccurredAt, 0).UTC())
	}
	if len(entry.Metadata) > 0 {
		event = event.Fields(map[string]interface{}{"metadata": entry.Metadata})
	}
	event.Msg("audit")
	return nil
}

var _ domain.AuditLogger = (*Logger)(nil)
