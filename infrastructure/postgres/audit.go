package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// maxAuditMetadataBytes bounds what one event may carry; larger metadata is replaced by a marker, not stored.
const maxAuditMetadataBytes = 8 << 10

// AuditRepository is the durable audit trail (table audit_events, append-only: see migration 000025).
type AuditRepository struct {
	db *sql.DB
}

// NewAuditRepository creates an AuditRepository.
func NewAuditRepository(db *sql.DB) *AuditRepository { return &AuditRepository{db: db} }

// Append stores one entry.
func (r *AuditRepository) Append(ctx context.Context, entry domain.AuditEntry) error {
	if strings.TrimSpace(entry.Action) == "" {
		return fmt.Errorf("audit entry without an action")
	}
	meta := []byte("{}")
	if len(entry.Metadata) > 0 {
		raw, err := json.Marshal(entry.Metadata)
		switch {
		case err != nil:
			meta = []byte(`{"unserializable":true}`)
		case len(raw) > maxAuditMetadataBytes:
			meta = []byte(`{"truncated":true}`)
		default:
			meta = raw
		}
	}
	occurred := time.Now()
	if entry.OccurredAt != 0 {
		occurred = toTime(entry.OccurredAt)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (occurred_at, actor_id, action, target_type, target_id, request_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)`,
		occurred, entry.ActorID, entry.Action, entry.TargetType, entry.TargetID, entry.RequestID, string(meta))
	if err != nil {
		return fmt.Errorf("append audit event: %w", err)
	}
	return nil
}

// auditWhere builds the WHERE clause and arguments of a filter.
func auditWhere(f domain.AuditFilter) (string, []interface{}) {
	var conds []string
	var args []interface{}
	add := func(cond string, arg interface{}) {
		args = append(args, arg)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Action != "" {
		add(`a.action LIKE $%d ESCAPE '\'`, escapeLikePrefix(f.Action)+"%")
	}
	if f.ActorID != "" {
		add(`a.actor_id = $%d`, f.ActorID)
	}
	if f.TargetType != "" {
		add(`a.target_type = $%d`, f.TargetType)
	}
	if f.TargetID != "" {
		add(`a.target_id = $%d`, f.TargetID)
	}
	if f.From != 0 {
		add(`a.occurred_at >= $%d`, toTime(f.From))
	}
	if f.To != 0 {
		add(`a.occurred_at <= $%d`, toTime(f.To))
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func escapeLikePrefix(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(s))
}

// List returns one page of events, newest first, and the total that matches.
func (r *AuditRepository) List(ctx context.Context, f domain.AuditFilter, limit, offset int) ([]domain.AuditEvent, int, error) {
	where, args := auditWhere(f)

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM audit_events a`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit events: %w", err)
	}

	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, `
		SELECT a.id, a.occurred_at, a.actor_id, COALESCE(u.display_name, ''), a.action, a.target_type, a.target_id, a.request_id, a.metadata
		FROM audit_events a
		LEFT JOIN users u ON u.id::text = a.actor_id`+where+fmt.Sprintf(`
		ORDER BY a.occurred_at DESC, a.id DESC
		LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit events: %w", err)
	}
	defer rows.Close()

	events := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		var at time.Time
		var meta []byte
		if err := rows.Scan(&e.ID, &at, &e.ActorID, &e.ActorName, &e.Action, &e.TargetType, &e.TargetID, &e.RequestID, &meta); err != nil {
			return nil, 0, err
		}
		e.OccurredAt = toUnix(at)
		e.Metadata = map[string]interface{}{}
		if len(meta) > 0 {
			_ = json.Unmarshal(meta, &e.Metadata)
		}
		events = append(events, e)
	}
	return events, total, rows.Err()
}

var _ domain.AuditStore = (*AuditRepository)(nil)
