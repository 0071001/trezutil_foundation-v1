-- 000016_payments_schema.sql
-- Payment Core schema for Sprint 19
-- Payments, Refunds, Payment Methods, Payment Ledger, Payment Webhook Events

-- +goose Up

-- Payments: external money-movement orchestration state.
CREATE TABLE IF NOT EXISTS payments (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id              UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    amount_minor          BIGINT NOT NULL,
    currency              CHAR(3) NOT NULL,
    payment_method_type   VARCHAR(32) NOT NULL,
    payment_method_id     VARCHAR(255) NOT NULL,
    provider_id           UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    provider_payment_id   VARCHAR(255),
    status                VARCHAR(32) NOT NULL DEFAULT 'created',
    idempotency_key       VARCHAR(255) NOT NULL,
    authorized_at         TIMESTAMP WITH TIME ZONE,
    captured_at           TIMESTAMP WITH TIME ZONE,
    cancelled_at          TIMESTAMP WITH TIME ZONE,
    failed_at             TIMESTAMP WITH TIME ZONE,
    failure_reason        TEXT,
    refunded_amount_minor BIGINT NOT NULL DEFAULT 0,
    refunded_currency     CHAR(3),
    metadata              JSONB NOT NULL DEFAULT '{}',
    version               BIGINT NOT NULL DEFAULT 1,
    created_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (amount_minor > 0),
    CHECK (LENGTH(currency) = 3),
    CHECK (payment_method_type IN ('MOBILE_MONEY', 'CARD', 'BANK_TRANSFER', 'CASH', 'WALLET', 'OTHER')),
    CHECK (status IN ('created', 'pending', 'authorized', 'captured', 'failed', 'cancelled', 'expired', 'refunded', 'partially_refunded')),
    CHECK (version >= 1),
    CHECK (refunded_amount_minor >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_idempotency_key ON payments(organization_id, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payments_provider_ref ON payments(organization_id, provider_id, provider_payment_id) WHERE provider_payment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_payments_tenant ON payments(organization_id);
CREATE INDEX IF NOT EXISTS idx_payments_order ON payments(organization_id, order_id);
CREATE INDEX IF NOT EXISTS idx_payments_provider ON payments(organization_id, provider_id);
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments(created_at);
CREATE INDEX IF NOT EXISTS idx_payments_provider_payment_id ON payments(organization_id, provider_id, provider_payment_id);

-- Refunds: refund records against captured payments.
CREATE TABLE IF NOT EXISTS refunds (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id       UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    payment_id          UUID NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    amount_minor        BIGINT NOT NULL,
    currency            CHAR(3) NOT NULL,
    provider_refund_id   VARCHAR(255),
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    idempotency_key     VARCHAR(255) NOT NULL,
    requested_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    processed_at        TIMESTAMP WITH TIME ZONE,
    failed_at           TIMESTAMP WITH TIME ZONE,
    failure_reason      TEXT,
    metadata            JSONB NOT NULL DEFAULT '{}',
    version             BIGINT NOT NULL DEFAULT 1,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (amount_minor > 0),
    CHECK (LENGTH(currency) = 3),
    CHECK (status IN ('pending', 'succeeded', 'failed', 'cancelled')),
    CHECK (version >= 1)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refunds_idempotency_key ON refunds(organization_id, idempotency_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_refunds_provider_ref ON refunds(organization_id, provider_refund_id) WHERE provider_refund_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refunds_tenant ON refunds(organization_id);
CREATE INDEX IF NOT EXISTS idx_refunds_payment ON refunds(organization_id, payment_id);
CREATE INDEX IF NOT EXISTS idx_refunds_status ON refunds(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_refunds_created_at ON refunds(created_at);

-- Payment methods: tokenized references only (NO secrets).
CREATE TABLE IF NOT EXISTS payment_methods (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    method_type     VARCHAR(32) NOT NULL,
    token_ref       VARCHAR(255) NOT NULL,
    provider_id     UUID NOT NULL REFERENCES providers(id) ON DELETE RESTRICT,
    details         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (method_type IN ('MOBILE_MONEY', 'CARD', 'BANK_TRANSFER', 'CASH', 'WALLET', 'OTHER'))
);

CREATE INDEX IF NOT EXISTS idx_payment_methods_tenant ON payment_methods(organization_id);
CREATE INDEX IF NOT EXISTS idx_payment_methods_provider ON payment_methods(organization_id, provider_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_methods_token ON payment_methods(organization_id, provider_id, token_ref);

-- Payment ledger: exactly-once accounting postings.
-- Each successful capture posts DEBIT+CREDIT entries; each refund posts a reversing entry.
-- The idempotency_key is unique per tenant, ensuring duplicate posting attempts are no-ops.
CREATE TABLE IF NOT EXISTS payment_ledger (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    payment_id      UUID REFERENCES payments(id) ON DELETE SET NULL,
    refund_id       UUID REFERENCES refunds(id) ON DELETE SET NULL,
    account         VARCHAR(64) NOT NULL,
    entry_type      VARCHAR(16) NOT NULL,
    amount_minor    BIGINT NOT NULL,
    currency        CHAR(3) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending',
    posted_at       TIMESTAMP WITH TIME ZONE,
    failure_reason  TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (amount_minor > 0),
    CHECK (LENGTH(currency) = 3),
    CHECK (entry_type IN ('DEBIT', 'CREDIT')),
    CHECK (status IN ('pending', 'posted', 'failed')),
    CHECK (account IN ('cash', 'revenue', 'liability', 'refund', 'fee'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payment_ledger_idempotency ON payment_ledger(organization_id, idempotency_key);
CREATE INDEX IF NOT EXISTS idx_payment_ledger_tenant ON payment_ledger(organization_id);
CREATE INDEX IF NOT EXISTS idx_payment_ledger_payment ON payment_ledger(organization_id, payment_id);
CREATE INDEX IF NOT EXISTS idx_payment_ledger_status ON payment_ledger(organization_id, status);
CREATE INDEX IF NOT EXISTS idx_payment_ledger_created_at ON payment_ledger(created_at);

-- Payment webhook events: durable idempotency for provider callbacks.
CREATE TABLE IF NOT EXISTS payment_webhook_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_code   VARCHAR(100) NOT NULL,
    provider_msg_id VARCHAR(255) NOT NULL,
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_type      VARCHAR(64) NOT NULL,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    payload         JSONB,
    processed_at    TIMESTAMP WITH TIME ZONE,
    error_message   TEXT,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    UNIQUE(provider_code, provider_msg_id)
);

CREATE INDEX IF NOT EXISTS idx_payment_webhook_provider ON payment_webhook_events(provider_code);
CREATE INDEX IF NOT EXISTS idx_payment_webhook_msg_id ON payment_webhook_events(provider_code, provider_msg_id);
CREATE INDEX IF NOT EXISTS idx_payment_webhook_status ON payment_webhook_events(status);
CREATE INDEX IF NOT EXISTS idx_payment_webhook_created_at ON payment_webhook_events(created_at);

-- +goose Down

DROP INDEX IF EXISTS idx_payment_webhook_created_at;
DROP INDEX IF EXISTS idx_payment_webhook_status;
DROP INDEX IF EXISTS idx_payment_webhook_msg_id;
DROP INDEX IF EXISTS idx_payment_webhook_provider;

DROP TABLE IF EXISTS payment_webhook_events;

DROP INDEX IF EXISTS idx_payment_ledger_created_at;
DROP INDEX IF EXISTS idx_payment_ledger_status;
DROP INDEX IF EXISTS idx_payment_ledger_payment;
DROP INDEX IF EXISTS idx_payment_ledger_tenant;
DROP INDEX IF EXISTS idx_payment_ledger_idempotency;

DROP TABLE IF EXISTS payment_ledger;

DROP INDEX IF EXISTS idx_payment_methods_token;
DROP INDEX IF EXISTS idx_payment_methods_provider;
DROP INDEX IF EXISTS idx_payment_methods_tenant;
DROP TABLE IF EXISTS payment_methods;

DROP INDEX IF EXISTS idx_refunds_created_at;
DROP INDEX IF EXISTS idx_refunds_status;
DROP INDEX IF EXISTS idx_refunds_payment;
DROP INDEX IF EXISTS idx_refunds_tenant;
DROP INDEX IF EXISTS idx_refunds_provider_ref;
DROP INDEX IF EXISTS idx_refunds_idempotency_key;

DROP TABLE IF EXISTS refunds;

DROP INDEX IF EXISTS idx_payments_provider_payment_id;
DROP INDEX IF EXISTS idx_payments_created_at;
DROP INDEX IF EXISTS idx_payments_status;
DROP INDEX IF EXISTS idx_payments_provider;
DROP INDEX IF EXISTS idx_payments_order;
DROP INDEX IF EXISTS idx_payments_tenant;
DROP INDEX IF EXISTS idx_payments_provider_ref;
DROP INDEX IF EXISTS idx_payments_idempotency_key;

DROP TABLE IF EXISTS payments;
