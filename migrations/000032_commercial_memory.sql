-- Commercial memory: what a person chose to let Trezutil remember (buying preferences, brands, usual budgets,
-- constraints, favourite suppliers, recurring projects), kept on the server so every device and the assistant see it.
--
-- Contract:
--   * Nothing is remembered without the person adding it (POST /api/v1/memory); they can change, delete, or switch
--     the whole memory off (commercial_memory_settings.enabled = false: kept, but never read by the assistant).
--   * kind is one of six values (CHECK); text is at most 400 characters; payment data, codes and secrets are refused
--     by the application before they reach this table.
--   * It is personal: it goes with the user (ON DELETE CASCADE).

-- +goose Up
CREATE TABLE IF NOT EXISTS commercial_memory_items (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       VARCHAR(20) NOT NULL CHECK (kind IN ('preference', 'brand', 'budget', 'constraint', 'supplier', 'project')),
    text       VARCHAR(400) NOT NULL CHECK (char_length(btrim(text)) > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_commercial_memory_items_user ON commercial_memory_items(user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS commercial_memory_settings (
    user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled    BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS commercial_memory_settings;
DROP INDEX IF EXISTS idx_commercial_memory_items_user;
DROP TABLE IF EXISTS commercial_memory_items;
