-- 000020_hotel_pack.sql
-- Native Hotel Commerce Pack on the Commerce Core.
-- See docs/architecture/HOTEL_COMMERCE_PACK.md.
--
-- Contract:
--   * A room type is a listing of vertical HOTEL. hotel_room_types holds only
--     what the core cannot: occupancy, bed configuration and amenities. Rooms
--     are the listing's ROOM inventory resource; nightly availability is its
--     capacity units, one per UTC night.
--   * Multi-unit booking: a reservation that draws from capacity units is
--     "capacity_managed" and may overlap other reservations of the same
--     listing (the capacity units, locked and counted in one transaction,
--     prevent overbooking). Every other reservation keeps the exclusion
--     constraint (a single bookable unit can never be double-booked).
--   * One resource's capacity windows never overlap, so a period's availability
--     is well defined (the tightest window).
--
-- Backward compatibility: capacity_managed defaults to false, so existing rows
-- and binaries built before this migration keep the exclusive behaviour.

-- +goose Up
CREATE TABLE IF NOT EXISTS hotel_room_types (
    listing_id        UUID PRIMARY KEY REFERENCES listings(id) ON DELETE CASCADE,
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    max_occupancy     INTEGER NOT NULL,
    bed_configuration VARCHAR(120),
    amenities         TEXT[] NOT NULL DEFAULT '{}',
    version           BIGINT NOT NULL DEFAULT 1,
    created_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (max_occupancy BETWEEN 1 AND 20),
    CHECK (cardinality(amenities) <= 30),
    CHECK (version >= 1)
);
CREATE INDEX IF NOT EXISTS idx_hotel_room_types_org ON hotel_room_types(organization_id);

ALTER TABLE reservations ADD COLUMN IF NOT EXISTS capacity_managed BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE reservations DROP CONSTRAINT IF EXISTS no_overlapping_reservations;
ALTER TABLE reservations
ADD CONSTRAINT no_overlapping_reservations
EXCLUDE USING GIST (
    listing_id WITH =,
    tstzrange(start_at, end_at) WITH &&
) WHERE (status <> 'cancelled' AND NOT capacity_managed);

ALTER TABLE capacity_units DROP CONSTRAINT IF EXISTS capacity_units_no_overlap;
ALTER TABLE capacity_units
ADD CONSTRAINT capacity_units_no_overlap
EXCLUDE USING GIST (
    inventory_id WITH =,
    tstzrange(start_at, end_at) WITH &&
);

-- +goose Down
ALTER TABLE capacity_units DROP CONSTRAINT IF EXISTS capacity_units_no_overlap;
ALTER TABLE reservations DROP CONSTRAINT IF EXISTS no_overlapping_reservations;
ALTER TABLE reservations
ADD CONSTRAINT no_overlapping_reservations
EXCLUDE USING GIST (
    listing_id WITH =,
    tstzrange(start_at, end_at) WITH &&
) WHERE (status <> 'cancelled');
ALTER TABLE reservations DROP COLUMN IF EXISTS capacity_managed;
DROP TABLE IF EXISTS hotel_room_types;
