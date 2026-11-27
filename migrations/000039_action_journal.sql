-- Durable idempotency of commerce actions and recovery of credit reservations (remediation phase 1).
--
-- commerce_action_journal replaces the in-memory store: an action claimed or completed stays known after a restart.
-- usage_reservations.action_state says what is known of the action an active reservation pays for, so a reservation
-- left behind is charged, released or kept accordingly. Nothing existing is changed or deleted.
--
-- Down removes the journal (completed results are lost: a repeated request would run again) and the column.

-- +goose Up
CREATE TABLE IF NOT EXISTS commerce_action_journal (
    action_key   CHAR(64) PRIMARY KEY,                 -- SHA-256 of tenant, actor, action type and client key
    fingerprint  VARCHAR(64) NOT NULL,                 -- of the request: the same key cannot stand for another
    status       VARCHAR(16) NOT NULL CHECK (status IN ('in_progress', 'completed')),
    result       JSONB,
    claimed_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    CHECK (status <> 'completed' OR result IS NOT NULL)
);
CREATE INDEX IF NOT EXISTS idx_commerce_action_journal_completed ON commerce_action_journal (completed_at) WHERE status = 'completed';

ALTER TABLE usage_reservations ADD COLUMN IF NOT EXISTS action_state VARCHAR(16)
    CHECK (action_state IN ('running', 'succeeded', 'failed'));

-- +goose Down
ALTER TABLE usage_reservations DROP COLUMN IF EXISTS action_state;
DROP INDEX IF EXISTS idx_commerce_action_journal_completed;
DROP TABLE IF EXISTS commerce_action_journal;
