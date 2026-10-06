// Package audit provides the domain.AuditLogger used by the services: every
// critical commerce action (order created or confirmed, reservation made or
// cancelled, listing published, inventory or capacity changed…) becomes one
// structured log line with the actor, the action, the target and the request id.
//
// Every entry is also written to the durable audit table when a store is attached
// (WithStore): that is what the administration console lists. The table is
// append-only but is not a tamper-proof ledger; the log line is written regardless.
package audit

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

// storeTimeout bounds one write to the audit table, so a slow database cannot hold a request.
const storeTimeout = 3 * time.Second

// Logger writes audit entries as JSON log lines (event="audit"), and to the durable store when one is attached.
type Logger struct {
	logger zerolog.Logger
	store  domain.AuditStore
}

// NewLogger returns an audit logger writing to the given service logger.
func NewLogger(logger zerolog.Logger) *Logger { return &Logger{logger: logger} }

// WithStore also records every entry in the durable audit store.
func (l *Logger) WithStore(store domain.AuditStore) *Logger {
	l.store = store
	return l
}

// Record implements domain.AuditLogger. It never fails: an audit line must not
// break the action it describes. A store that cannot be written is logged
// (event="audit_store_failed") and the log line remains the record.
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

	if l.store != nil {
		entry.RequestID = requestID
		// the request may already be over (a cancelled client must not lose the record): detach from its cancellation
		wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), storeTimeout)
		defer cancel()
		if err := l.store.Append(wctx, entry); err != nil {
			l.logger.Warn().Err(err).Str("event", "audit_store_failed").Str("action", entry.Action).Str("request_id", requestID).Msg("audit event not stored")
		}
	}
	return nil
}

var _ domain.AuditLogger = (*Logger)(nil)
