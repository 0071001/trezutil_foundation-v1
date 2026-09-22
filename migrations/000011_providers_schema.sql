-- +goose Up
CREATE TABLE IF NOT EXISTS providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    code VARCHAR(100) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'INACTIVE',
    capabilities TEXT[] NOT NULL DEFAULT '{}',
    configuration TEXT DEFAULT '',
    secret_ref VARCHAR(255),
    priority INTEGER NOT NULL DEFAULT 0,
    health_status VARCHAR(50) NOT NULL DEFAULT 'UNKNOWN',
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (status IN ('ACTIVE','INACTIVE','SUSPENDED','DEGRADED')),
    CHECK (health_status IN ('UNKNOWN','HEALTHY','DEGRADED','UNAVAILABLE')),
    CHECK (priority >= 0),
    CHECK (version >= 1),
    CHECK (LENGTH(code) > 0),
    CHECK (LENGTH(name) > 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_providers_org_code ON providers(organization_id, code);
CREATE INDEX IF NOT EXISTS idx_providers_org ON providers(organization_id);
CREATE INDEX IF NOT EXISTS idx_providers_status ON providers(status);
CREATE INDEX IF NOT EXISTS idx_providers_health ON providers(health_status);
CREATE INDEX IF NOT EXISTS idx_providers_priority ON providers(priority);
CREATE INDEX IF NOT EXISTS idx_providers_capabilities ON providers USING GIN (capabilities);

CREATE TABLE IF NOT EXISTS provider_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    capability VARCHAR(50) NOT NULL,
    operation VARCHAR(100) NOT NULL,
    actor_id UUID REFERENCES users(id) ON DELETE SET NULL,
    idempotency_key VARCHAR(255),
    status VARCHAR(50) NOT NULL,
    error_code VARCHAR(50),
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms BIGINT,
    external_reference VARCHAR(255),

    CHECK (status IN ('SUCCESS','FAILED','CANCELLED')),
    CHECK (duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_provider_executions_provider_id ON provider_executions(provider_id);
CREATE INDEX IF NOT EXISTS idx_provider_executions_org ON provider_executions(organization_id);
CREATE INDEX IF NOT EXISTS idx_provider_executions_capability ON provider_executions(capability);
CREATE INDEX IF NOT EXISTS idx_provider_executions_idempotency_key ON provider_executions(idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_provider_executions_status ON provider_executions(status);

-- +goose Down
DROP INDEX IF EXISTS idx_provider_executions_status;
DROP INDEX IF EXISTS idx_provider_executions_idempotency_key;
DROP INDEX IF EXISTS idx_provider_executions_capability;
DROP INDEX IF EXISTS idx_provider_executions_org;
DROP INDEX IF EXISTS idx_provider_executions_provider_id;
DROP TABLE IF EXISTS provider_executions;

DROP INDEX IF EXISTS idx_providers_capabilities;
DROP INDEX IF EXISTS idx_providers_priority;
DROP INDEX IF EXISTS idx_providers_health;
DROP INDEX IF EXISTS idx_providers_status;
DROP INDEX IF EXISTS idx_providers_org;
DROP INDEX IF EXISTS idx_providers_org_code;
DROP TABLE IF EXISTS providers;
