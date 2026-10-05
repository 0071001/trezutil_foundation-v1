-- 000024_site_content.sql
-- Content of the public site that staff manage from the administration console:
-- the home page's trust strip, partner band and section order.
--
-- Contract:
--   * site_settings holds one JSON document per key ("home"). The application
--     validates it (internal/domain/site.go); the database stores it as is.
--   * version is the optimistic-concurrency token: a save names the version it
--     read, and loses (409) if somebody saved in between.
--   * No figure is ever stored. The trust strip stores which platform metrics to
--     show and how; the numbers are computed from the platform's own tables on
--     every read.
--   * site_assets holds uploaded partner logos (PNG, JPEG or WebP, at most
--     512 KiB). The same bytes are stored once (unique sha256) and served by
--     GET /api/v1/site/assets/{id}.

-- +goose Up
CREATE TABLE IF NOT EXISTS site_settings (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    version    INTEGER NOT NULL DEFAULT 1 CHECK (version >= 1),
    updated_by UUID REFERENCES users(id) ON DELETE SET NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS site_assets (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_type VARCHAR(32) NOT NULL CHECK (content_type IN ('image/png', 'image/jpeg', 'image/webp')),
    size_bytes   INTEGER NOT NULL CHECK (size_bytes BETWEEN 1 AND 524288),
    sha256       CHAR(64) NOT NULL,
    data         BYTEA NOT NULL,
    created_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_site_assets_sha256 UNIQUE (sha256)
);

-- +goose Down
DROP TABLE IF EXISTS site_assets;
DROP TABLE IF EXISTS site_settings;
