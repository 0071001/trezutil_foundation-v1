-- Team invitations by phone number.
--
-- An owner or admin of a business invites somebody by phone number and role. The invitation becomes a
-- membership the next time that number signs in with a one-time code, whether or not it already had an
-- account, so inviting a number never tells the business whether it belongs to somebody on Trezutil.
--
-- Contract:
--   * phone is stored normalised (E.164), as users.phone is.
--   * One open invitation per business and phone number (a second one replaces the role).
--   * An invitation is open while accepted_at and revoked_at are empty and expires_at is in the future.
--   * The owner role cannot be given by invitation.

-- +goose Up
CREATE TABLE IF NOT EXISTS organization_invitations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    phone           VARCHAR(32) NOT NULL,
    membership_role VARCHAR(50) NOT NULL,
    invited_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMP WITH TIME ZONE NOT NULL,
    accepted_at     TIMESTAMP WITH TIME ZONE,
    accepted_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    revoked_at      TIMESTAMP WITH TIME ZONE,

    CHECK (membership_role IN ('admin', 'manager', 'staff', 'accountant', 'sales', 'operations')),
    CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_org_invitations_open
    ON organization_invitations (organization_id, phone)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_org_invitations_phone ON organization_invitations (phone) WHERE accepted_at IS NULL AND revoked_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_org_invitations_phone;
DROP INDEX IF EXISTS uq_org_invitations_open;
DROP TABLE IF EXISTS organization_invitations;
