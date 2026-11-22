-- Personal credit space: the organisation of type 'individual' that holds a person's own credits.
--
-- Every credit table (credit_accounts, usage_ledger, credit_grants, credit_purchases, subscriptions) belongs to an
-- organisation (tenant_id NOT NULL REFERENCES organizations). A person who buys for themself has none, so their
-- credits had nowhere to live: the runtime fell back to the user id as tenant, which no organisation has, and any
-- metered action of a personal account would have failed. Rather than a second billing model, each person gets one
-- 'individual' organisation, created the first time their credits are needed, and the existing credit system applies
-- to it unchanged. It is not a membership: it never appears among the organisations a person works for.
--
-- Contract: one personal space per user and one user per personal space (both keys unique); both go with the user.

-- +goose Up
CREATE TABLE IF NOT EXISTS personal_spaces (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL UNIQUE REFERENCES organizations(id) ON DELETE CASCADE,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS personal_spaces;
