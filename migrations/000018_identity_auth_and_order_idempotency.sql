-- +goose Up
-- MVP finalization: phone-OTP authentication, one account per phone number,
-- idempotent order creation, and incremental search projection sync.

-- One user per phone number. OTP login and WhatsApp sender resolution both
-- find-or-create users by phone, so duplicates would split a person's
-- orders, reservations and credits across accounts.
CREATE UNIQUE INDEX IF NOT EXISTS uq_users_phone ON users (phone);

-- Platform roles referenced by user_roles. AssignRole resolves roles by name,
-- so they must exist before the first user signs up.
INSERT INTO roles (name, description) VALUES
    ('customer', 'Buyer / guest who completed sign-in'),
    ('seller', 'Seller operating listings'),
    ('staff', 'Organization staff'),
    ('admin', 'Platform administrator'),
    ('operator', 'Platform operator'),
    ('moderator', 'Content moderator'),
    ('delivery_partner', 'Delivery partner')
ON CONFLICT (name) DO NOTHING;

-- One-time passcode challenges. Only a salted hash of the code is stored.
CREATE TABLE IF NOT EXISTS auth_otp_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone VARCHAR(20) NOT NULL,
    code_hash VARCHAR(128) NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    consumed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (attempts >= 0),
    CHECK (max_attempts > 0)
);

CREATE INDEX IF NOT EXISTS idx_auth_otp_challenges_phone_created ON auth_otp_challenges (phone, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_auth_otp_challenges_expires_at ON auth_otp_challenges (expires_at);

-- Idempotent order creation: a retried checkout with the same key returns the
-- original order instead of creating a duplicate.
ALTER TABLE orders ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_customer_idempotency
    ON orders (customer_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

-- At most one live payment per order: a second concurrent checkout with a
-- different idempotency key must not be able to charge the customer twice.
CREATE UNIQUE INDEX IF NOT EXISTS uq_payments_live_per_order
    ON payments (order_id)
    WHERE status NOT IN ('failed', 'cancelled', 'expired');

-- Marketplace-wide and guest conversations. A web visitor browsing the whole
-- marketplace talks to Trezutil before choosing a seller (no organization),
-- and may not be signed in yet (no customer). Tenant-scoped conversations
-- (e.g. WhatsApp business numbers) still carry organization_id; ownership of
-- a conversation is enforced by the conversation processor.
ALTER TABLE conversations ALTER COLUMN organization_id DROP NOT NULL;
ALTER TABLE conversations ALTER COLUMN customer_id DROP NOT NULL;

-- Keyset scan used by the search projection sync loop.
CREATE INDEX IF NOT EXISTS idx_listings_updated_at_id ON listings (updated_at, id);

-- +goose Down

DROP INDEX IF EXISTS idx_listings_updated_at_id;
DROP INDEX IF EXISTS uq_payments_live_per_order;
DROP INDEX IF EXISTS uq_orders_customer_idempotency;
ALTER TABLE orders DROP COLUMN IF EXISTS idempotency_key;
DROP INDEX IF EXISTS idx_auth_otp_challenges_expires_at;
DROP INDEX IF EXISTS idx_auth_otp_challenges_phone_created;
DROP TABLE IF EXISTS auth_otp_challenges;
DROP INDEX IF EXISTS uq_users_phone;
