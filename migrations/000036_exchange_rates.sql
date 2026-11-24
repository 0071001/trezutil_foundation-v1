-- The CDF / USD exchange rate, entered by a Trezutil administrator (never fetched or guessed).
--
-- Each change is a new row, so the history of who set which rate and when is kept; the current rate is the latest row.
-- The rate is only used to show an indicative conversion ("≈") beside a price and to read a budget given in francs:
-- an order is always priced and paid in the currency the supplier set.

-- +goose Up
CREATE TABLE IF NOT EXISTS exchange_rates (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    base       TEXT NOT NULL CHECK (base = 'USD'),
    quote      TEXT NOT NULL CHECK (quote = 'CDF'),
    rate       NUMERIC(12, 2) NOT NULL CHECK (rate > 0),
    note       TEXT NOT NULL DEFAULT '',
    set_by     UUID NOT NULL REFERENCES users (id),
    set_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_exchange_rates_pair_set_at ON exchange_rates (base, quote, set_at DESC);

-- +goose Down
DROP TABLE IF EXISTS exchange_rates;
