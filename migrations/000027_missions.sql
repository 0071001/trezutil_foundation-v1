-- Organiser: missions and their steps.
--
-- A mission is a goal a person delegates to Trezutil ("organise a birthday for 40 people in Kinshasa") that may
-- need several commerce actions. It is an orchestration layer over the existing Commerce Core, not a second
-- engine: a step only references the commerce action it will run and the result it produced, and nothing here
-- is priced or metered (credits stay per action, in the existing usage policies).
--
-- Contract:
--   * A mission belongs to one user (owner_id); it is deleted with its owner.
--   * mission_type is an open code (like listings.vertical), status is a closed lifecycle.
--   * A retried creation carrying the same idempotency_key returns the first mission of that owner.
--   * A step has a position unique within its mission, assigned by the repository (next free, from 1).
--   * Phase 1 records steps; action_type, action_ref and result_ref stay empty until steps are connected to the
--     Commerce Action Runtime.

-- +goose Up
CREATE TABLE IF NOT EXISTS missions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    mission_type    VARCHAR(32) NOT NULL,
    title           VARCHAR(120) NOT NULL,
    objective       TEXT NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'draft',
    location        VARCHAR(120) NOT NULL DEFAULT '',
    start_date      DATE,
    end_date        DATE,
    participants    INTEGER NOT NULL DEFAULT 0,
    budget_minor    BIGINT NOT NULL DEFAULT 0,
    currency        VARCHAR(3) NOT NULL DEFAULT '',
    preferences     JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key VARCHAR(100),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (mission_type ~ '^[A-Z][A-Z0-9_]{1,31}$'),
    CHECK (status IN ('draft', 'active', 'paused', 'completed', 'cancelled')),
    CHECK (participants >= 0),
    CHECK (budget_minor >= 0),
    CHECK (budget_minor = 0 OR currency ~ '^[A-Z]{3}$'),
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_missions_owner_updated ON missions (owner_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_missions_owner_status ON missions (owner_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_missions_owner_idempotency
    ON missions (owner_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS mission_steps (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mission_id  UUID NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
    step_type   VARCHAR(32) NOT NULL,
    title       VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      VARCHAR(20) NOT NULL DEFAULT 'pending',
    position    INTEGER NOT NULL,
    action_type VARCHAR(64) NOT NULL DEFAULT '',
    action_ref  VARCHAR(64) NOT NULL DEFAULT '',
    result_ref  VARCHAR(64) NOT NULL DEFAULT '',
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (step_type ~ '^[A-Z][A-Z0-9_]{1,31}$'),
    CHECK (status IN ('pending', 'in_progress', 'completed', 'skipped', 'failed')),
    CHECK (position > 0),
    UNIQUE (mission_id, position)
);

-- +goose Down
DROP TABLE IF EXISTS mission_steps;
DROP INDEX IF EXISTS uq_missions_owner_idempotency;
DROP INDEX IF EXISTS idx_missions_owner_status;
DROP INDEX IF EXISTS idx_missions_owner_updated;
DROP TABLE IF EXISTS missions;
