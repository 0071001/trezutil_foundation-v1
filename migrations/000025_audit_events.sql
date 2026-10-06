-- The durable audit trail: who did what, to which resource, and when.
--
-- Until now an audit entry was one structured log line, which the log platform keeps or loses.
-- This table is the queryable record the administration console reads (/admin/audit).
--
-- Contract:
--   * Append-only. The application can add events and read them; a trigger refuses any UPDATE or DELETE,
--     so a bug or a compromised service account cannot rewrite history. (A database superuser still can:
--     this is not a tamper-proof ledger, and the log line written beside every event remains.)
--   * actor_id and target_id are plain text, not foreign keys: an event must outlive the account or the
--     record it names. actor_id is empty for the system itself.
--   * metadata holds small facts (previous and new status, counts). Never secrets, never message text.
--   * Written best-effort by the services (a failure to write is logged, and never blocks the action).

-- +goose Up
CREATE TABLE IF NOT EXISTS audit_events (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    actor_id    TEXT NOT NULL DEFAULT '',
    action      TEXT NOT NULL CHECK (action <> ''),
    target_type TEXT NOT NULL DEFAULT '',
    target_id   TEXT NOT NULL DEFAULT '',
    request_id  TEXT NOT NULL DEFAULT '',
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_audit_events_occurred_at ON audit_events (occurred_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events (actor_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_target ON audit_events (target_type, target_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_action ON audit_events (action text_pattern_ops, occurred_at DESC);

CREATE OR REPLACE FUNCTION audit_events_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'audit_events is append-only (% refused)', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
CREATE TRIGGER trg_audit_events_append_only
    BEFORE UPDATE OR DELETE ON audit_events
    FOR EACH ROW EXECUTE FUNCTION audit_events_append_only();

-- +goose Down
DROP TRIGGER IF EXISTS trg_audit_events_append_only ON audit_events;
DROP FUNCTION IF EXISTS audit_events_append_only();
DROP TABLE IF EXISTS audit_events;
