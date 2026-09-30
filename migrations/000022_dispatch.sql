-- 000022_dispatch.sql
-- Dispatch core shared by food delivery and ride-hailing.
-- See docs/architecture/DISPATCH_FOOD_AND_RIDES.md.
--
-- Contract:
--   * An agent is a user acting as a COURIER or a DRIVER. Supply is native for
--     now; matching reads agents through a port, so external supply can be
--     added later without changing jobs.
--   * A job is one unit of work (FOOD_DELIVERY linked to an order, or a RIDE).
--     organization_id is the store's organization for food and NULL for rides.
--   * Assignment safety is enforced here, not only in code: an agent can hold
--     at most one active job, and a job has at most one pending offer.
--   * proof_code is the delivery/trip code the customer reads to the agent.
--     It is shown to the customer only, never to agents; attempts are limited.

-- +goose Up
CREATE TABLE IF NOT EXISTS dispatch_agents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        VARCHAR(16) NOT NULL,
    vehicle     VARCHAR(16) NOT NULL,
    status      VARCHAR(16) NOT NULL DEFAULT 'OFFLINE',
    lat         DOUBLE PRECISION,
    lng         DOUBLE PRECISION,
    position_at TIMESTAMP WITH TIME ZONE,
    version     BIGINT NOT NULL DEFAULT 1,
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, kind),
    CHECK (kind IN ('COURIER', 'DRIVER')),
    CHECK (vehicle IN ('BICYCLE', 'MOTORBIKE', 'CAR')),
    CHECK (status IN ('OFFLINE', 'ONLINE', 'BUSY')),
    CHECK (lat IS NULL OR lat BETWEEN -90 AND 90),
    CHECK (lng IS NULL OR lng BETWEEN -180 AND 180),
    CHECK (status <> 'ONLINE' OR (lat IS NOT NULL AND lng IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS idx_dispatch_agents_online ON dispatch_agents(kind, status) WHERE status = 'ONLINE';

CREATE TABLE IF NOT EXISTS dispatch_jobs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id     UUID REFERENCES organizations(id) ON DELETE CASCADE,
    kind                VARCHAR(16) NOT NULL,
    order_id            UUID REFERENCES orders(id) ON DELETE CASCADE,
    customer_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    pickup_lat          DOUBLE PRECISION NOT NULL,
    pickup_lng          DOUBLE PRECISION NOT NULL,
    pickup_address      VARCHAR(300) NOT NULL DEFAULT '',
    dropoff_lat         DOUBLE PRECISION NOT NULL,
    dropoff_lng         DOUBLE PRECISION NOT NULL,
    dropoff_address     VARCHAR(300) NOT NULL DEFAULT '',
    status              VARCHAR(16) NOT NULL DEFAULT 'REQUESTED',
    agent_id            UUID REFERENCES dispatch_agents(id),
    fee_minor           BIGINT NOT NULL DEFAULT 0,
    currency            CHAR(3) NOT NULL,
    cash_due_minor      BIGINT NOT NULL DEFAULT 0,
    cash_collected_minor BIGINT,
    proof_code          VARCHAR(8) NOT NULL,
    proof_attempts      INTEGER NOT NULL DEFAULT 0,
    cancel_reason       VARCHAR(200),
    version             BIGINT NOT NULL DEFAULT 1,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (kind IN ('FOOD_DELIVERY', 'RIDE')),
    CHECK (status IN ('REQUESTED', 'OFFERED', 'ASSIGNED', 'AT_PICKUP', 'IN_TRANSIT', 'COMPLETED', 'CANCELLED', 'NO_AGENT')),
    CHECK (fee_minor >= 0 AND cash_due_minor >= 0),
    CHECK (kind <> 'FOOD_DELIVERY' OR (order_id IS NOT NULL AND organization_id IS NOT NULL)),
    CHECK (status NOT IN ('ASSIGNED', 'AT_PICKUP', 'IN_TRANSIT') OR agent_id IS NOT NULL)
);
-- One delivery job per order.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_dispatch_jobs_order ON dispatch_jobs(order_id) WHERE order_id IS NOT NULL;
-- An agent holds at most one active job.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_dispatch_jobs_agent_active ON dispatch_jobs(agent_id)
    WHERE status IN ('ASSIGNED', 'AT_PICKUP', 'IN_TRANSIT');
CREATE INDEX IF NOT EXISTS idx_dispatch_jobs_customer ON dispatch_jobs(customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dispatch_jobs_open ON dispatch_jobs(status) WHERE status IN ('REQUESTED', 'OFFERED');

CREATE TABLE IF NOT EXISTS dispatch_offers (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id     UUID NOT NULL REFERENCES dispatch_jobs(id) ON DELETE CASCADE,
    agent_id   UUID NOT NULL REFERENCES dispatch_agents(id) ON DELETE CASCADE,
    status     VARCHAR(16) NOT NULL DEFAULT 'PENDING',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (job_id, agent_id),
    CHECK (status IN ('PENDING', 'ACCEPTED', 'DECLINED', 'EXPIRED', 'WITHDRAWN'))
);
-- A job has at most one pending offer at a time.
CREATE UNIQUE INDEX IF NOT EXISTS uniq_dispatch_offers_pending ON dispatch_offers(job_id) WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_dispatch_offers_agent ON dispatch_offers(agent_id, status);

CREATE TABLE IF NOT EXISTS dispatch_positions (
    id       BIGSERIAL PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES dispatch_agents(id) ON DELETE CASCADE,
    lat      DOUBLE PRECISION NOT NULL,
    lng      DOUBLE PRECISION NOT NULL,
    at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (lat BETWEEN -90 AND 90 AND lng BETWEEN -180 AND 180)
);
CREATE INDEX IF NOT EXISTS idx_dispatch_positions_agent ON dispatch_positions(agent_id, at DESC);

-- +goose Down
DROP TABLE IF EXISTS dispatch_positions;
DROP TABLE IF EXISTS dispatch_offers;
DROP TABLE IF EXISTS dispatch_jobs;
DROP TABLE IF EXISTS dispatch_agents;
