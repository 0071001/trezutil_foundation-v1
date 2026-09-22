# Sprint 14 — Database Schema

## Migration: 000011_providers_schema

Creates the `providers` and `provider_executions` tables for the Provider Orchestration Layer.

## providers Table

```sql
CREATE TABLE providers (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organizations(id),
    code             VARCHAR(100) NOT NULL,
    name             VARCHAR(255) NOT NULL,
    type             VARCHAR(100) NOT NULL,
    status           VARCHAR(20) NOT NULL DEFAULT 'INACTIVE',
    capabilities     TEXT[] NOT NULL,
    config           JSONB NOT NULL DEFAULT '{}',
    secret_ref       VARCHAR(255),
    priority         INTEGER NOT NULL DEFAULT 0,
    health_status    VARCHAR(20) NOT NULL DEFAULT 'UNKNOWN',
    version          BIGINT NOT NULL DEFAULT 1,
    created_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
```

### Constraints

```sql
ALTER TABLE providers ADD CONSTRAINT provider_status_check
    CHECK (status IN ('ACTIVE', 'INACTIVE', 'SUSPENDED', 'DEGRADED'));

ALTER TABLE providers ADD CONSTRAINT provider_health_check
    CHECK (health_status IN ('UNKNOWN', 'HEALTHY', 'DEGRADED', 'UNAVAILABLE'));
```

### Indexes

```sql
CREATE INDEX idx_providers_org ON providers(organization_id);
CREATE INDEX idx_providers_code ON providers(organization_id, code);
CREATE INDEX idx_providers_status ON providers(organization_id, status);
```

### Unique Constraints

```sql
ALTER TABLE providers ADD CONSTRAINT provider_org_code_unique
    UNIQUE (organization_id, code);
```

## provider_executions Table

```sql
CREATE TABLE provider_executions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organizations(id),
    provider_id      UUID NOT NULL REFERENCES providers(id),
    capability       VARCHAR(50) NOT NULL,
    operation        VARCHAR(100) NOT NULL,
    idempotency_key  VARCHAR(255),
    started_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMP WITH TIME ZONE,
    duration_ms      BIGINT,
    success          BOOLEAN NOT NULL,
    error_code       VARCHAR(50),
    error_message    TEXT
);
```

### Indexes

```sql
CREATE INDEX idx_exec_provider ON provider_executions(provider_id);
CREATE INDEX idx_exec_org ON provider_executions(organization_id);
CREATE INDEX idx_exec_idempotency ON provider_executions(idempotency_key);
```

## Notes

- Uses `gen_random_uuid()` for ID generation (consistent with project convention).
- Uses version-based optimistic locking (`version` column) for concurrent update safety.
- `config` is stored as JSONB; secrets are never stored in `config` or anywhere in this table.
- `secret_ref` references an external secret by name only — the actual secret material is stored in the secret management system.
- `capabilities` uses PostgreSQL native array type with `pq.Array` for Go interaction.
- The migration is idempotent; existing tables are dropped first if needed.
