-- Negotiation, phase 1 (docs/architecture/NEGOTIATION_ENGINE.md): a buyer asks one or more suppliers for their best
-- price on published offers, within a mandate the buyer sets; suppliers answer in their portal (and are told on
-- WhatsApp when an approved template is configured); Trezutil may counter on the buyer's behalf, inside the mandate
-- only; the buyer alone accepts an offer, which creates an order at the agreed price.
--
-- What the code guarantees (negotiation.Guard), not the model:
--   * the buyer's ceiling (max_unit_minor) never leaves the buyer's side: it is in no message and no supplier view;
--   * a counter-offer is between the buyer's target and ceiling, below the supplier's current offer, within the
--     number of rounds and before the deadline;
--   * no supplier answer ever creates an order or a payment.
--
-- Every change of state is written in negotiation_events (append-only) in the same transaction.

-- +goose Up
CREATE TABLE IF NOT EXISTS negotiations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    buyer_id          UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id   VARCHAR(80) NOT NULL DEFAULT '',
    title             VARCHAR(200) NOT NULL,
    quantity          INTEGER NOT NULL CHECK (quantity BETWEEN 1 AND 100000),
    currency          VARCHAR(3) NOT NULL,
    status            VARCHAR(20) NOT NULL DEFAULT 'soliciting'
                      CHECK (status IN ('soliciting', 'negotiating', 'awaiting_decision', 'accepted', 'rejected', 'expired', 'cancelled')),
    deadline          TIMESTAMPTZ NOT NULL,
    accepted_offer_id UUID,
    order_id          UUID,
    version           INTEGER NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_negotiations_buyer ON negotiations(buyer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_negotiations_open_deadline ON negotiations(deadline)
    WHERE status IN ('soliciting', 'negotiating', 'awaiting_decision');

-- The mandate: what the buyer allows Trezutil to do. A change is a new version; the latest one applies.
CREATE TABLE IF NOT EXISTS negotiation_mandates (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    negotiation_id    UUID NOT NULL REFERENCES negotiations(id) ON DELETE CASCADE,
    version           INTEGER NOT NULL CHECK (version >= 1),
    target_unit_minor BIGINT NOT NULL CHECK (target_unit_minor > 0),
    max_unit_minor    BIGINT NOT NULL CHECK (max_unit_minor >= target_unit_minor),
    max_rounds        INTEGER NOT NULL CHECK (max_rounds BETWEEN 1 AND 5),
    auto_counter      BOOLEAN NOT NULL DEFAULT TRUE,
    granted_by        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (negotiation_id, version)
);

-- One supplier offer (listing) asked in a negotiation.
CREATE TABLE IF NOT EXISTS negotiation_participants (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    negotiation_id   UUID NOT NULL REFERENCES negotiations(id) ON DELETE CASCADE,
    organization_id  UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    listing_id       UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    listing_title    VARCHAR(255) NOT NULL,
    list_unit_minor  BIGINT NOT NULL CHECK (list_unit_minor >= 0),
    status           VARCHAR(16) NOT NULL DEFAULT 'invited'
                     CHECK (status IN ('invited', 'responded', 'countered', 'declined', 'accepted', 'rejected', 'expired')),
    round            INTEGER NOT NULL DEFAULT 1 CHECK (round >= 1),
    reminded_at      TIMESTAMPTZ,
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (negotiation_id, listing_id)
);
CREATE INDEX IF NOT EXISTS idx_negotiation_participants_org ON negotiation_participants(organization_id, last_activity_at DESC);

-- Offers and counter-offers, kept as history: a new one supersedes the author's previous open one.
CREATE TABLE IF NOT EXISTS negotiation_offers (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    negotiation_id  UUID NOT NULL REFERENCES negotiations(id) ON DELETE CASCADE,
    participant_id  UUID NOT NULL REFERENCES negotiation_participants(id) ON DELETE CASCADE,
    round           INTEGER NOT NULL CHECK (round >= 1),
    author          VARCHAR(8) NOT NULL CHECK (author IN ('supplier', 'buyer')),
    source          VARCHAR(12) NOT NULL CHECK (source IN ('portal', 'buyer', 'strategy')),
    unit_minor      BIGINT NOT NULL CHECK (unit_minor > 0),
    quantity        INTEGER NOT NULL CHECK (quantity >= 1),
    delivery_days   INTEGER CHECK (delivery_days IS NULL OR delivery_days BETWEEN 0 AND 365),
    valid_until     TIMESTAMPTZ,
    note            VARCHAR(1000) NOT NULL DEFAULT '',
    status          VARCHAR(12) NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'superseded', 'accepted', 'declined')),
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_negotiation_offers_participant ON negotiation_offers(participant_id, created_at);

-- Messages to and from suppliers. Outgoing ones are an outbox: sent exactly once per idempotency key.
CREATE TABLE IF NOT EXISTS negotiation_messages (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    negotiation_id  UUID NOT NULL REFERENCES negotiations(id) ON DELETE CASCADE,
    participant_id  UUID NOT NULL REFERENCES negotiation_participants(id) ON DELETE CASCADE,
    channel         VARCHAR(10) NOT NULL CHECK (channel IN ('portal', 'whatsapp')),
    direction       VARCHAR(3) NOT NULL CHECK (direction IN ('out', 'in')),
    body            TEXT NOT NULL,
    recipient       VARCHAR(40) NOT NULL DEFAULT '',
    external_id     VARCHAR(120) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(160) NOT NULL UNIQUE,
    state           VARCHAR(10) NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'sent', 'delivered', 'failed', 'skipped')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_error      VARCHAR(500) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at         TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_negotiation_messages_outbox ON negotiation_messages(created_at) WHERE state = 'pending';
CREATE UNIQUE INDEX IF NOT EXISTS idx_negotiation_messages_external ON negotiation_messages(channel, external_id) WHERE external_id <> '';

-- The journal: who did what, when (append-only).
CREATE TABLE IF NOT EXISTS negotiation_events (
    id              BIGSERIAL PRIMARY KEY,
    negotiation_id  UUID NOT NULL REFERENCES negotiations(id) ON DELETE CASCADE,
    participant_id  UUID REFERENCES negotiation_participants(id) ON DELETE CASCADE,
    actor           VARCHAR(8) NOT NULL CHECK (actor IN ('buyer', 'supplier', 'system')),
    actor_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    kind            VARCHAR(40) NOT NULL,
    data            JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_negotiation_events_negotiation ON negotiation_events(negotiation_id, id);

-- +goose Down
DROP TABLE IF EXISTS negotiation_events;
DROP TABLE IF EXISTS negotiation_messages;
DROP TABLE IF EXISTS negotiation_offers;
DROP TABLE IF EXISTS negotiation_participants;
DROP TABLE IF EXISTS negotiation_mandates;
DROP TABLE IF EXISTS negotiations;
