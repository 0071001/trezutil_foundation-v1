-- +goose Up
-- SQL in section 'Up' is executed when migration is applied

CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL,
    type VARCHAR(20) NOT NULL,
    sender_id VARCHAR(255) NOT NULL,
    recipient_id VARCHAR(255),
    content JSONB NOT NULL DEFAULT '{}',
    external_ref VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(conversation_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_external_ref ON messages(conversation_id, external_ref);
CREATE INDEX IF NOT EXISTS idx_messages_idempotency ON messages(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);

-- +goose Down
-- SQL in section 'Down' is executed when migration is rolled back

DROP INDEX IF EXISTS idx_messages_created_at;
DROP INDEX IF EXISTS idx_messages_idempotency;
DROP INDEX IF EXISTS idx_messages_external_ref;
DROP INDEX IF EXISTS idx_messages_conversation;
DROP TABLE IF EXISTS messages;
