-- Organiser phase 2: the plan of a mission and its versions.
--
-- A planner turns a mission into a plan: ordered steps that are INTENTIONS to act (a step type and the commerce
-- action it will run). Nothing is searched, reserved, ordered or paid, and no credit is used: steps stay pending.
--
-- Contract:
--   * A mission has at most one CURRENT plan; a replacement is the next version and the old one becomes
--     'superseded' and stays (history). A plan is never edited in place.
--   * fingerprint identifies a plan's content (planner, templates version, mission type, steps): planning the
--     same mission again with the same planner finds the same fingerprint and stores nothing new.
--   * A retried request carrying the same idempotency_key returns the plan it stored first.
--   * mission_steps.plan_version is the plan a step belongs to; 0 is a step added by hand (phase 1). Positions are
--     unique within a mission and plan version.
--   * action_support says whether the Commerce OS can run the step's action today: 'supported' (a handler is
--     registered in the Commerce Action Runtime) or 'not_yet_supported'; empty for a step added by hand.

-- +goose Up
CREATE TABLE IF NOT EXISTS mission_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mission_id      UUID NOT NULL REFERENCES missions(id) ON DELETE CASCADE,
    version         INTEGER NOT NULL,
    mission_type    VARCHAR(32) NOT NULL,
    planner         VARCHAR(40) NOT NULL,
    planner_version VARCHAR(20) NOT NULL,
    status          VARCHAR(12) NOT NULL DEFAULT 'current',
    fingerprint     VARCHAR(64) NOT NULL,
    idempotency_key VARCHAR(100),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    superseded_at   TIMESTAMP WITH TIME ZONE,

    CHECK (version > 0),
    CHECK (mission_type ~ '^[A-Z][A-Z0-9_]{1,31}$'),
    CHECK (status IN ('current', 'superseded')),
    CHECK ((status = 'superseded') = (superseded_at IS NOT NULL)),
    UNIQUE (mission_id, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_mission_plans_current ON mission_plans (mission_id) WHERE status = 'current';
CREATE UNIQUE INDEX IF NOT EXISTS uq_mission_plans_idempotency
    ON mission_plans (mission_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

ALTER TABLE mission_steps ADD COLUMN IF NOT EXISTS plan_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mission_steps ADD COLUMN IF NOT EXISTS action_support VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE mission_steps ADD CONSTRAINT mission_steps_plan_version_check CHECK (plan_version >= 0);
ALTER TABLE mission_steps ADD CONSTRAINT mission_steps_action_support_check CHECK (action_support IN ('', 'supported', 'not_yet_supported'));
ALTER TABLE mission_steps DROP CONSTRAINT IF EXISTS mission_steps_mission_id_position_key;
ALTER TABLE mission_steps ADD CONSTRAINT uq_mission_steps_plan_position UNIQUE (mission_id, plan_version, position);

-- +goose Down
DELETE FROM mission_steps WHERE plan_version > 0;
ALTER TABLE mission_steps DROP CONSTRAINT IF EXISTS uq_mission_steps_plan_position;
ALTER TABLE mission_steps ADD CONSTRAINT mission_steps_mission_id_position_key UNIQUE (mission_id, position);
ALTER TABLE mission_steps DROP CONSTRAINT IF EXISTS mission_steps_action_support_check;
ALTER TABLE mission_steps DROP CONSTRAINT IF EXISTS mission_steps_plan_version_check;
ALTER TABLE mission_steps DROP COLUMN IF EXISTS action_support;
ALTER TABLE mission_steps DROP COLUMN IF EXISTS plan_version;
DROP INDEX IF EXISTS uq_mission_plans_idempotency;
DROP INDEX IF EXISTS uq_mission_plans_current;
DROP TABLE IF EXISTS mission_plans;
