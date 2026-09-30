-- 000019_listing_verticals.sql
-- Every listing belongs to a commerce vertical (HOTEL, RESTAURANT, RETAIL…).
-- See docs/architecture/VERTICAL_COMMERCE_PACKS.md.
--
-- Contract:
--   * listings.vertical holds a vertical code from the vertical catalogue
--     (internal/application/vertical). The database checks only the code's
--     shape, so adding a vertical never needs a migration.
--   * The column is nullable so binaries built before this migration keep
--     inserting listings during a rolling deploy; readers treat NULL as the
--     listing type's default vertical (the same mapping as the backfill below).
--   * The vertical is set when a listing is created and is not updated.
--
-- Backfill (domain.DefaultVerticalForListingType):
--   physical_product, vehicle -> RETAIL   (vehicles are sold today)
--   food                      -> RESTAURANT
--   service                   -> SERVICES
--   real_estate               -> REAL_ESTATE
--   other                     -> OTHER

-- +goose Up
ALTER TABLE listings ADD COLUMN IF NOT EXISTS vertical VARCHAR(32);

UPDATE listings SET vertical = CASE listing_type
    WHEN 'physical_product' THEN 'RETAIL'
    WHEN 'vehicle'          THEN 'RETAIL'
    WHEN 'food'             THEN 'RESTAURANT'
    WHEN 'service'          THEN 'SERVICES'
    WHEN 'real_estate'      THEN 'REAL_ESTATE'
    ELSE 'OTHER'
END
WHERE vertical IS NULL;

ALTER TABLE listings DROP CONSTRAINT IF EXISTS listings_vertical_format;
ALTER TABLE listings ADD CONSTRAINT listings_vertical_format
    CHECK (vertical IS NULL OR vertical ~ '^[A-Z][A-Z0-9_]{1,31}$');

-- An organization's verticals and per-vertical seller views.
CREATE INDEX IF NOT EXISTS idx_listings_org_vertical ON listings(organization_id, vertical);

-- +goose Down
DROP INDEX IF EXISTS idx_listings_org_vertical;
ALTER TABLE listings DROP CONSTRAINT IF EXISTS listings_vertical_format;
ALTER TABLE listings DROP COLUMN IF EXISTS vertical;
