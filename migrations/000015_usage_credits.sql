-- 000015_usage_credits.sql
-- Usage and Credits system for Sprint 18.1
-- Credit accounts, usage policies, ledger, and reservations

-- +goose Up

CREATE TABLE IF NOT EXISTS credit_accounts (
    account_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type    VARCHAR(32) NOT NULL,
    owner_id      VARCHAR(255) NOT NULL,
    tenant_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id       VARCHAR(100),
    period_id     VARCHAR(100) NOT NULL,
    granted       BIGINT NOT NULL DEFAULT 0,
    reserved      BIGINT NOT NULL DEFAULT 0,
    consumed      BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (owner_type, owner_id, tenant_id, period_id)
);

CREATE TABLE IF NOT EXISTS usage_policies (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    action_type   VARCHAR(64) NOT NULL,
    channel       VARCHAR(32) NOT NULL DEFAULT 'other',
    actor_type    VARCHAR(32) NOT NULL DEFAULT 'customer',
    cost          BIGINT NOT NULL DEFAULT 0,
    active        BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS usage_ledger (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES credit_accounts(account_id) ON DELETE CASCADE,
    action_type     VARCHAR(64),
    actor_id        VARCHAR(255) NOT NULL,
    tenant_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    transaction_type VARCHAR(32) NOT NULL,
    amount          BIGINT NOT NULL,
    balance_before  BIGINT NOT NULL,
    balance_after   BIGINT NOT NULL,
    period_id       VARCHAR(100) NOT NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    metadata        JSONB
);

CREATE TABLE IF NOT EXISTS usage_reservations (
    reservation_id  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES credit_accounts(account_id) ON DELETE CASCADE,
    action_type     VARCHAR(64) NOT NULL,
    amount          BIGINT NOT NULL,
    status          VARCHAR(32) NOT NULL DEFAULT 'active',
    reserved_at     TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    consumed_at     TIMESTAMP WITH TIME ZONE,
    released_at     TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_credit_accounts_owner ON credit_accounts(owner_type, owner_id, tenant_id);
CREATE INDEX IF NOT EXISTS idx_credit_accounts_tenant ON credit_accounts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_usage_policies_action ON usage_policies(action_type, channel, actor_type, active);
CREATE INDEX IF NOT EXISTS idx_usage_ledger_account ON usage_ledger(account_id);
CREATE INDEX IF NOT EXISTS idx_usage_ledger_tenant ON usage_ledger(tenant_id);
CREATE INDEX IF NOT EXISTS idx_usage_ledger_created ON usage_ledger(created_at);
CREATE INDEX IF NOT EXISTS idx_usage_reservations_account ON usage_reservations(account_id);
CREATE INDEX IF NOT EXISTS idx_usage_reservations_status ON usage_reservations(status);

-- +goose Down

DROP INDEX IF EXISTS idx_usage_reservations_status;
DROP INDEX IF EXISTS idx_usage_reservations_account;
DROP INDEX IF EXISTS idx_usage_ledger_created;
DROP INDEX IF EXISTS idx_usage_ledger_tenant;
DROP INDEX IF EXISTS idx_usage_ledger_account;
DROP INDEX IF EXISTS idx_usage_policies_action;
DROP INDEX IF EXISTS idx_credit_accounts_tenant;
DROP INDEX IF EXISTS idx_credit_accounts_owner;

DROP TABLE IF EXISTS usage_reservations;
DROP TABLE IF EXISTS usage_ledger;
DROP TABLE IF EXISTS usage_policies;
DROP TABLE IF EXISTS credit_accounts;