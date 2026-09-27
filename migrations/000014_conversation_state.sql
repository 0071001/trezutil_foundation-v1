-- 000014_conversation_state.sql
-- Conversational Commerce state persistence for Sprint 18
-- Adds conversation context, confirmation state, result items, and pending parameters

-- +goose Up

CREATE TABLE IF NOT EXISTS conversation_contexts (
    conversation_id UUID PRIMARY KEY REFERENCES conversations(id) ON DELETE CASCADE,
    context         JSONB NOT NULL DEFAULT '{}',
    language        VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conversation_result_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    ref_type        VARCHAR(32) NOT NULL,        -- "listing", "order", "reservation"
    entity_id       VARCHAR(255) NOT NULL,       -- canonical entity ID
    position        INTEGER NOT NULL,            -- 0-based ordinal position in result set
    title           TEXT NOT NULL,
    subtitle        TEXT,
    data            JSONB,                       -- full entity snapshot
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS conversation_confirmations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    action_id       VARCHAR(255) NOT NULL,
    action_type     VARCHAR(64) NOT NULL,
    state           VARCHAR(20) NOT NULL DEFAULT 'pending',
    summary         TEXT NOT NULL,
    details         JSONB,                       -- confirmation details (redacted)
    confirmation_token VARCHAR(255) NOT NULL,
    expires_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_conversation_contexts_conversation_id ON conversation_contexts(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_result_items_conv ON conversation_result_items(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_result_items_ref_type ON conversation_result_items(ref_type);
CREATE INDEX IF NOT EXISTS idx_conversation_confirmations_conv ON conversation_confirmations(conversation_id);
CREATE INDEX IF NOT EXISTS idx_conversation_confirmations_action_id ON conversation_confirmations(action_id);
CREATE INDEX IF NOT EXISTS idx_conversation_confirmations_token ON conversation_confirmations(confirmation_token);
CREATE INDEX IF NOT EXISTS idx_conversation_confirmations_expires ON conversation_confirmations(expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_conversation_confirmations_expires;
DROP INDEX IF EXISTS idx_conversation_confirmations_token;
DROP INDEX IF EXISTS idx_conversation_confirmations_action_id;
DROP INDEX IF EXISTS idx_conversation_confirmations_conv;
DROP INDEX IF EXISTS idx_conversation_result_items_ref_type;
DROP INDEX IF EXISTS idx_conversation_result_items_conv;
DROP INDEX IF EXISTS idx_conversation_contexts_conversation_id;

DROP TABLE IF EXISTS conversation_confirmations;
DROP TABLE IF EXISTS conversation_result_items;
DROP TABLE IF EXISTS conversation_contexts;
