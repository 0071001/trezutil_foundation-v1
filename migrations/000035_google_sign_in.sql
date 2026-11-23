-- Sign-in with Google, as a second way into the same accounts (the phone one-time code stays).
--
-- external_identities (migration 000002) links an account to an identity at a provider: provider 'google' and the
-- Google account's stable subject ("sub" of its ID token; never the email, which can change). It existed unused and
-- unconstrained. These indexes make the link unambiguous:
--   * one Google account opens one Trezutil account (unique provider + subject);
--   * one Trezutil account has at most one Google account linked (unique user + provider).
-- metadata keeps the email and name Google reported at the last sign-in (for display only, never for matching).

-- +goose Up
CREATE UNIQUE INDEX IF NOT EXISTS uq_external_identities_provider_subject ON external_identities (provider, provider_subject);
CREATE UNIQUE INDEX IF NOT EXISTS uq_external_identities_user_provider ON external_identities (user_id, provider);

-- +goose Down
DROP INDEX IF EXISTS uq_external_identities_user_provider;
DROP INDEX IF EXISTS uq_external_identities_provider_subject;
