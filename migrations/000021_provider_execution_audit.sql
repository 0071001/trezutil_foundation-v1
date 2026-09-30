-- 000021_provider_execution_audit.sql
-- Provider Orchestration records every provider attempt in provider_executions
-- (migration 000011) for operational debugging and future reconciliation.
--
-- Contract (one generic table for every provider and capability):
--   request_id   correlates the attempts of one orchestrated call
--   attempt      1-based attempt number within that call
--   retry_class  RETRYABLE | NON_RETRYABLE | UNKNOWN for failed attempts
-- Existing columns keep their meaning: organization, provider, capability,
-- operation, actor (NULL for system calls such as webhooks), idempotency key,
-- status SUCCESS | FAILED | CANCELLED, normalized error_code, timings and the
-- provider's external reference. Provider error messages are not stored: they
-- are logged, and the normalized code is what reconciliation needs.
--
-- Executions are written only by Provider Orchestration; no API exposes or
-- changes them, and every read is scoped to an organization.

-- +goose Up
ALTER TABLE provider_executions ADD COLUMN IF NOT EXISTS request_id VARCHAR(100);
ALTER TABLE provider_executions ADD COLUMN IF NOT EXISTS attempt INTEGER NOT NULL DEFAULT 1;
ALTER TABLE provider_executions ADD COLUMN IF NOT EXISTS retry_class VARCHAR(20);
ALTER TABLE provider_executions DROP CONSTRAINT IF EXISTS provider_executions_attempt_check;
ALTER TABLE provider_executions ADD CONSTRAINT provider_executions_attempt_check CHECK (attempt >= 1);
ALTER TABLE provider_executions DROP CONSTRAINT IF EXISTS provider_executions_retry_class_check;
ALTER TABLE provider_executions ADD CONSTRAINT provider_executions_retry_class_check
    CHECK (retry_class IS NULL OR retry_class IN ('RETRYABLE', 'NON_RETRYABLE', 'UNKNOWN'));
CREATE INDEX IF NOT EXISTS idx_provider_executions_request_id ON provider_executions(request_id) WHERE request_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_provider_executions_org_started ON provider_executions(organization_id, started_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_provider_executions_org_started;
DROP INDEX IF EXISTS idx_provider_executions_request_id;
ALTER TABLE provider_executions DROP CONSTRAINT IF EXISTS provider_executions_retry_class_check;
ALTER TABLE provider_executions DROP CONSTRAINT IF EXISTS provider_executions_attempt_check;
ALTER TABLE provider_executions DROP COLUMN IF EXISTS retry_class;
ALTER TABLE provider_executions DROP COLUMN IF EXISTS attempt;
ALTER TABLE provider_executions DROP COLUMN IF EXISTS request_id;
