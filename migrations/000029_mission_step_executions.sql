-- Organiser phase 3: executing a mission step, one step at a time.
--
-- A step is run on request through the existing Commerce Action Runtime, which stays the only place where commerce
-- actions run and credits are checked, reserved, consumed or released. This table is the durable record of each
-- attempt: what was asked, what the runtime answered and how it ended. It holds a small summary and references to
-- commerce entities (for a search: the ids of the listings found), never a copy of commerce data.
--
-- Contract:
--   * An attempt belongs to a step (and its mission) and goes with them: deleting the step, its mission or the
--     owner deletes the attempts.
--   * attempt counts from 1 per step. A retry is the next attempt; a failed attempt is never edited back.
--   * One attempt at most is in progress per step (the step row is locked when an attempt starts), and the same
--     step with the same idempotency_key is the same attempt: a repeated request returns it and runs nothing.
--   * error_code is a stable classification (a runtime error code, or one of the mission layer's own), never a
--     free-text runtime message. runtime_status is the runtime's answer, kept for diagnosis.
--   * Nothing here is a credit: credit accounts, reservations and the ledger are the runtime's and are untouched.

-- +goose Up
CREATE TABLE IF NOT EXISTS mission_step_executions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    step_id          UUID NOT NULL REFERENCES mission_steps(id) ON DELETE CASCADE,
    mission_id       UUID NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
    attempt          INTEGER NOT NULL,
    status           VARCHAR(12) NOT NULL DEFAULT 'in_progress',
    action_type      VARCHAR(64) NOT NULL,
    idempotency_key  VARCHAR(100),
    result_reference VARCHAR(64) NOT NULL DEFAULT '',
    result_payload   JSONB NOT NULL DEFAULT '{}'::jsonb,
    runtime_status   VARCHAR(32) NOT NULL DEFAULT '',
    error_code       VARCHAR(64) NOT NULL DEFAULT '',
    started_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMP WITH TIME ZONE,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (attempt > 0),
    CHECK (status IN ('in_progress', 'completed', 'failed')),
    CHECK ((status = 'in_progress') = (completed_at IS NULL)),
    CHECK (status <> 'failed' OR error_code <> ''),
    UNIQUE (step_id, attempt)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mission_step_executions_key
    ON mission_step_executions (step_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_mission_step_executions_running
    ON mission_step_executions (step_id) WHERE status = 'in_progress';
CREATE INDEX IF NOT EXISTS idx_mission_step_executions_mission ON mission_step_executions (mission_id, step_id, attempt DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_mission_step_executions_mission;
DROP INDEX IF EXISTS uq_mission_step_executions_running;
DROP INDEX IF EXISTS uq_mission_step_executions_key;
DROP TABLE IF EXISTS mission_step_executions;
