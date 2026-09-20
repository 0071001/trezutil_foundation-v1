-- +goose Up
-- SQL in section 'Up' is executed when migration is applied

CREATE TABLE IF NOT EXISTS channel_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(channel, external_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_accounts_organization_id ON channel_accounts(organization_id);
CREATE INDEX IF NOT EXISTS idx_channel_accounts_channel_external ON channel_accounts(channel, external_id);

-- +goose Down
-- SQL in section 'Down' is executed when migration is rolled back

DROP INDEX IF EXISTS idx_channel_accounts_channel_external;
DROP INDEX IF EXISTS idx_channel_accounts_organization_id;
DROP TABLE IF EXISTS channel_accounts;
