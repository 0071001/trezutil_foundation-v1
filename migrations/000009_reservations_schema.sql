-- +goose Up
-- SQL in section 'Up' is executed when migration is applied

CREATE TABLE IF NOT EXISTS reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    start_at TIMESTAMP WITH TIME ZONE NOT NULL,
    end_at TIMESTAMP WITH TIME ZONE NOT NULL,
    quantity INTEGER NOT NULL DEFAULT 1,
    party_size INTEGER NOT NULL DEFAULT 1,
     pricing_snapshot JSONB,
    idempotency_key VARCHAR(255) UNIQUE,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending', 'confirmed', 'completed', 'cancelled')),
    CHECK (quantity > 0),
    CHECK (party_size >= 0),
    CHECK (end_at > start_at),
    CHECK (version >= 1)
);

-- Unique constraint to prevent double booking for the same listing and overlapping period
-- This uses a GiST index with exclusion constraint for temporal overlap prevention
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- Exclusion constraint to prevent overlapping reservations for the same listing
-- Only applies to non-cancelled reservations
ALTER TABLE reservations
ADD CONSTRAINT no_overlapping_reservations
EXCLUDE USING GIST (
    listing_id WITH =,
    tstzrange(start_at, end_at) WITH &&
) WHERE (status <> 'cancelled');

CREATE INDEX IF NOT EXISTS idx_reservations_organization_id ON reservations(organization_id);
CREATE INDEX IF NOT EXISTS idx_reservations_customer_id ON reservations(customer_id);
CREATE INDEX IF NOT EXISTS idx_reservations_listing_id ON reservations(listing_id);
CREATE INDEX IF NOT EXISTS idx_reservations_status ON reservations(status);
CREATE INDEX IF NOT EXISTS idx_reservations_start_at ON reservations(start_at);
CREATE INDEX IF NOT EXISTS idx_reservations_end_at ON reservations(end_at);
CREATE INDEX IF NOT EXISTS idx_reservations_created_at ON reservations(created_at);
CREATE INDEX IF NOT EXISTS idx_reservations_updated_at ON reservations(updated_at);
CREATE INDEX IF NOT EXISTS idx_reservations_idempotency_key ON reservations(idempotency_key);

-- +goose Down
-- SQL in section 'Down' is executed when migration is rolled back

DROP INDEX IF EXISTS idx_reservations_updated_at;
DROP INDEX IF EXISTS idx_reservations_idempotency_key;
DROP INDEX IF EXISTS idx_reservations_created_at;
DROP INDEX IF EXISTS idx_reservations_end_at;
DROP INDEX IF EXISTS idx_reservations_start_at;
DROP INDEX IF EXISTS idx_reservations_status;
DROP INDEX IF EXISTS idx_reservations_listing_id;
DROP INDEX IF EXISTS idx_reservations_customer_id;
DROP INDEX IF EXISTS idx_reservations_organization_id;

ALTER TABLE reservations DROP CONSTRAINT IF EXISTS no_overlapping_reservations;

DROP TABLE IF EXISTS reservations;