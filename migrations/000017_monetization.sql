-- 000017_monetization.sql
-- Sprint 20: Monetization, Plans & Credit Entitlements
-- Plan catalog, Subscriptions, BillingPeriods, CreditGrants, CreditPurchases.
-- Extends existing 000015 usage/credits schema (reused with fixes).

-- +goose Up

CREATE TABLE IF NOT EXISTS plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(100) NOT NULL,
    price_minor     BIGINT NOT NULL DEFAULT 0,            -- minor currency units; 0 == free tier
    currency        CHAR(3) NOT NULL DEFAULT 'USD',
    billing_cycle   VARCHAR(16) NOT NULL DEFAULT 'monthly', -- monthly | yearly
    quota_amount    BIGINT NOT NULL DEFAULT 0,            -- free-tier credit allocation
    credit_pack     BIGINT NOT NULL DEFAULT 0,            -- paid purchase pack size
    status          VARCHAR(16) NOT NULL DEFAULT 'active', -- active | archived
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, slug)
);

CREATE TABLE IF NOT EXISTS subscriptions (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    plan_id             UUID NOT NULL REFERENCES plans(id),
    owner_type          VARCHAR(32) NOT NULL,
    owner_id            VARCHAR(255) NOT NULL,
    status              VARCHAR(16) NOT NULL DEFAULT 'active', -- active | pending | canceled | past_due
    current_period_id   VARCHAR(100) NOT NULL,
    cancel_at_period_end BOOLEAN NOT NULL DEFAULT false,
    started_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, owner_type, owner_id)
);

CREATE TABLE IF NOT EXISTS billing_periods (
    id              VARCHAR(100) NOT NULL,                 -- e.g. "2026-01"
    tenant_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    plan_id         UUID REFERENCES plans(id),
    starts_at       TIMESTAMP WITH TIME ZONE NOT NULL,
    ends_at         TIMESTAMP WITH TIME ZONE NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'open',   -- open | closed
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id),
    UNIQUE (tenant_id, subscription_id, id)
);

CREATE TABLE IF NOT EXISTS credit_grants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id      UUID NOT NULL REFERENCES credit_accounts(account_id) ON DELETE CASCADE,
    idempotency_key VARCHAR(255) NOT NULL UNIQUE,
    source          VARCHAR(16) NOT NULL,                  -- allocation | purchase | manual
    amount          BIGINT NOT NULL,
    tenant_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    period_id       VARCHAR(100),
    payment_id      UUID REFERENCES payments(id) ON DELETE SET NULL, -- set only for purchase grants
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (amount > 0)
);

CREATE TABLE IF NOT EXISTS credit_purchases (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    owner_type      VARCHAR(32) NOT NULL,
    owner_id        VARCHAR(255) NOT NULL,
    plan_id         UUID NOT NULL REFERENCES plans(id),    -- credit-pack plan
    amount_credits  BIGINT NOT NULL,
    payment_id      UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending', -- pending | captured | failed | canceled
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (payment_id)
);

-- Idempotency safeguard for reservations (Sprint 20 F2/F3 fix).
ALTER TABLE usage_reservations
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255),
    ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE UNIQUE INDEX IF NOT EXISTS uq_usage_reservations_idempotency
    ON usage_reservations (idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- Scope reservations by tenant for audit.
CREATE INDEX IF NOT EXISTS idx_usage_reservations_tenant ON usage_reservations (tenant_id);

-- Index for per-period credit-account lookups (BillingPeriod rollup).
CREATE INDEX IF NOT EXISTS idx_credit_accounts_period ON credit_accounts (tenant_id, period_id);

-- +goose Down

DROP INDEX IF EXISTS idx_credit_accounts_period;
DROP INDEX IF EXISTS idx_usage_reservations_tenant;
DROP INDEX IF EXISTS uq_usage_reservations_idempotency;

ALTER TABLE usage_reservations
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS tenant_id;

DROP TABLE IF EXISTS credit_purchases;
DROP TABLE IF EXISTS credit_grants;
DROP TABLE IF EXISTS billing_periods;
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
