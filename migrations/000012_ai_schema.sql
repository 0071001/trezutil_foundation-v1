-- Migration 000012: AI Provider Schema
-- Supports AI Gateway, Model Router, and AI Orchestrator (Sprint 15)

CREATE TABLE IF NOT EXISTS ai_providers (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code        VARCHAR(100) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    type        VARCHAR(100) NOT NULL,
    status      VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    config      JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE ai_providers ADD CONSTRAINT ai_provider_status_check
    CHECK (status IN ('ACTIVE', 'INACTIVE'));

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_providers_code ON ai_providers(code);

CREATE TABLE IF NOT EXISTS ai_models (
    model_id             VARCHAR(255) NOT NULL,
    provider_id          UUID NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
    name                 VARCHAR(255) NOT NULL,
    capabilities         TEXT[] NOT NULL,
    context_window       INTEGER NOT NULL DEFAULT 8192,
    supports_tools       BOOLEAN NOT NULL DEFAULT false,
    supports_structured  BOOLEAN NOT NULL DEFAULT false,
    status               VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    priority             INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (model_id, provider_id)
);

ALTER TABLE ai_models ADD CONSTRAINT ai_model_status_check
    CHECK (status IN ('ACTIVE', 'INACTIVE'));

CREATE INDEX IF NOT EXISTS idx_ai_models_provider ON ai_models(provider_id);
CREATE INDEX IF NOT EXISTS idx_ai_models_status ON ai_models(status);

CREATE TABLE IF NOT EXISTS ai_execution_records (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    conversation_id UUID,
    request_id      VARCHAR(255) NOT NULL,
    trace_id        VARCHAR(255),
    task_type       VARCHAR(50) NOT NULL,
    provider_id     VARCHAR(255) NOT NULL,
    model_id        VARCHAR(255) NOT NULL,
    tokens_used     INTEGER NOT NULL DEFAULT 0,
    latency_ms      BIGINT NOT NULL DEFAULT 0,
    success         BOOLEAN NOT NULL,
    error_code      VARCHAR(50),
    error_message   TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ai_exec_org ON ai_execution_records(organization_id);
CREATE INDEX IF NOT EXISTS idx_ai_exec_request ON ai_execution_records(request_id);
CREATE INDEX IF NOT EXISTS idx_ai_exec_conversation ON ai_execution_records(conversation_id);

CREATE TABLE IF NOT EXISTS ai_tenant_policies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL,
    allowed_models  TEXT[],
    allowed_providers TEXT[],
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_tenant_policies_org ON ai_tenant_policies(organization_id);
