-- Organiser phase 4: the execution graph of a mission, and the lock that lets one orchestration at a time run it.
--
-- A step of a plan may depend on other steps of the SAME plan: it cannot run until each has completed. READY and
-- BLOCKED are not stored: they are derived from the step statuses and these dependencies, so the five step statuses
-- (pending, in_progress, completed, skipped, failed) stay the only source of truth. Nothing here is a credit, a
-- reservation or an action: orchestration only asks the step executor to run what is ready, and the Commerce Action
-- Runtime stays the only place where actions run and credits are handled.
--
-- Contract:
--   * mission_step_dependencies(step_id, depends_on_step_id) says depends_on must be completed before step may run.
--     The two composite foreign keys tie both steps to the same mission AND the same plan version, so a dependency on a
--     step of another mission, of another plan version, or on a step that does not exist cannot be stored; a step added by
--     hand (plan_version 0) has no dependencies.
--   * A step cannot depend on itself (CHECK) and the same dependency cannot be stored twice (primary key).
--   * Cycles cannot be excluded by a constraint: the domain rejects them when a plan is built and again before every run
--     (domain.ValidateGraph), and a mission whose stored graph has a cycle is refused, never run.
--   * Dependencies go with their steps (ON DELETE CASCADE), which go with the mission.
--   * mission_run_locks holds one row per mission being run: the holder's token and a lease. Taking it is a single atomic
--     statement; a lease that ran out (the process died) can be taken over. Step claims (mission_step_executions) remain
--     the second line of defence: even without this lock one step cannot be run twice at once.

-- +goose Up
ALTER TABLE mission_steps ADD CONSTRAINT uq_mission_steps_identity UNIQUE (id, mission_id, plan_version);

CREATE TABLE IF NOT EXISTS mission_step_dependencies (
    mission_id         UUID NOT NULL,
    plan_version       INTEGER NOT NULL,
    step_id            UUID NOT NULL,
    depends_on_step_id UUID NOT NULL,
    created_at         TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    PRIMARY KEY (step_id, depends_on_step_id),
    CHECK (step_id <> depends_on_step_id),
    CHECK (plan_version > 0),
    FOREIGN KEY (step_id, mission_id, plan_version)
        REFERENCES mission_steps (id, mission_id, plan_version) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_step_id, mission_id, plan_version)
        REFERENCES mission_steps (id, mission_id, plan_version) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_mission_step_dependencies_plan ON mission_step_dependencies (mission_id, plan_version);
CREATE INDEX IF NOT EXISTS idx_mission_step_dependencies_target ON mission_step_dependencies (depends_on_step_id);

CREATE TABLE IF NOT EXISTS mission_run_locks (
    mission_id  UUID PRIMARY KEY REFERENCES missions(id) ON DELETE CASCADE,
    token       UUID NOT NULL,
    acquired_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at  TIMESTAMP WITH TIME ZONE NOT NULL,

    CHECK (expires_at > acquired_at)
);

-- +goose Down
DROP TABLE IF EXISTS mission_run_locks;
DROP INDEX IF EXISTS idx_mission_step_dependencies_target;
DROP INDEX IF EXISTS idx_mission_step_dependencies_plan;
DROP TABLE IF EXISTS mission_step_dependencies;
ALTER TABLE mission_steps DROP CONSTRAINT IF EXISTS uq_mission_steps_identity;
