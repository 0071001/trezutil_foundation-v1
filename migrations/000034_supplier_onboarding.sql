-- Supplier self-registration: a signed-in person registers a business as a supplier, describes it, sends its
-- documents and submits it for verification. Registering never makes a supplier verified or trusted.
--
-- Three things are kept apart, on purpose:
--   * registered        — the business exists (organizations + seller_profiles), its applicant is its owner, and the
--                         application is a 'draft' the owner may edit;
--   * documents sent    — status 'submitted': complete, waiting for Trezutil staff; nothing is verified yet;
--   * verified          — status 'verified', decided by staff (never by the applicant, never automatically); only
--                         then do organizations.verification_status and seller_profiles.verification_status become
--                         'verified'. Staff may also reject, or ask for changes (the owner edits and submits again).
--
-- The organisation's own fields (names, phone, address…) stay in organizations; this table holds what a supplier
-- adds (legal identifiers, sectors, capacities, served areas) and the review. Documents are stored in the database
-- like site logos (bytes, type checked from the content, SHA-256), readable only by the business's managers and staff.

-- +goose Up
CREATE TABLE IF NOT EXISTS supplier_applications (
    organization_id UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    applicant_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          VARCHAR(24) NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'submitted', 'changes_requested', 'verified', 'rejected')),
    legal_form      VARCHAR(40)  NOT NULL DEFAULT '',
    rccm            VARCHAR(60)  NOT NULL DEFAULT '',
    id_nat          VARCHAR(60)  NOT NULL DEFAULT '',
    nif             VARCHAR(60)  NOT NULL DEFAULT '',
    website         VARCHAR(255) NOT NULL DEFAULT '',
    contact_name    VARCHAR(120) NOT NULL DEFAULT '',
    contact_role    VARCHAR(80)  NOT NULL DEFAULT '',
    sectors         TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality(sectors) <= 10),
    capabilities    VARCHAR(2000) NOT NULL DEFAULT '',
    min_order       VARCHAR(200) NOT NULL DEFAULT '',
    lead_time_days  INTEGER CHECK (lead_time_days IS NULL OR lead_time_days BETWEEN 0 AND 365),
    payment_terms   VARCHAR(300) NOT NULL DEFAULT '',
    service_areas   TEXT[] NOT NULL DEFAULT '{}' CHECK (cardinality(service_areas) <= 30),
    submitted_at    TIMESTAMP WITH TIME ZONE,
    reviewed_at     TIMESTAMP WITH TIME ZONE,
    reviewer_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    review_note     VARCHAR(1000) NOT NULL DEFAULT '',
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_supplier_applications_status ON supplier_applications(status, submitted_at);
CREATE INDEX IF NOT EXISTS idx_supplier_applications_applicant ON supplier_applications(applicant_id);

CREATE TABLE IF NOT EXISTS supplier_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES supplier_applications(organization_id) ON DELETE CASCADE,
    kind            VARCHAR(20) NOT NULL CHECK (kind IN ('rccm', 'id_nat', 'nif', 'identity', 'other')),
    filename        VARCHAR(200) NOT NULL,
    content_type    VARCHAR(40) NOT NULL CHECK (content_type IN ('application/pdf', 'image/jpeg', 'image/png', 'image/webp')),
    size_bytes      INTEGER NOT NULL CHECK (size_bytes BETWEEN 1 AND 5242880),
    sha256          CHAR(64) NOT NULL,
    data            BYTEA NOT NULL,
    status          VARCHAR(12) NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted', 'accepted', 'rejected')),
    uploaded_by     UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, sha256)
);
CREATE INDEX IF NOT EXISTS idx_supplier_documents_org ON supplier_documents(organization_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS supplier_documents;
DROP INDEX IF EXISTS idx_supplier_applications_applicant;
DROP INDEX IF EXISTS idx_supplier_applications_status;
DROP TABLE IF EXISTS supplier_applications;
