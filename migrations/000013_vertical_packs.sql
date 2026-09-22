-- 000013_vertical_packs.sql
-- Vertical Pack Architecture for Sprint 17
-- Adds pack definitions, organization pack activation, and policies

CREATE TABLE IF NOT EXISTS vertical_packs (
    id              VARCHAR(64) PRIMARY KEY,
    code            VARCHAR(32) NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    description     TEXT,
    version_major   INTEGER NOT NULL DEFAULT 1,
    version_minor   INTEGER NOT NULL DEFAULT 0,
    version_patch   INTEGER NOT NULL DEFAULT 0,
    status          VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    capabilities    TEXT NOT NULL,          -- JSON array of capability strings
    supported_actions TEXT NOT NULL,      -- JSON array of action type strings
    workflow_json   JSONB,                -- pack workflow definition
    ui_metadata     JSONB,                -- UI routing, icon, color, order
    ai_profile      VARCHAR(32) NOT NULL, -- AI specialist code
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL
);

CREATE TABLE IF NOT EXISTS organization_packs (
    organization_id VARCHAR(64) NOT NULL,
    pack_code       VARCHAR(32) NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    activated_at    BIGINT NOT NULL,
    config_overrides JSONB,               -- JSON map of override key->value
    PRIMARY KEY (organization_id, pack_code)
);

CREATE TABLE IF NOT EXISTS pack_policies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pack_code       VARCHAR(32) NOT NULL REFERENCES vertical_packs(code) ON DELETE CASCADE,
    policy_key      VARCHAR(128) NOT NULL,
    policy_value    TEXT NOT NULL,
    created_at      BIGINT NOT NULL,
    updated_at      BIGINT NOT NULL,
    UNIQUE (pack_code, policy_key)
);

CREATE INDEX IF NOT EXISTS idx_organization_packs_org ON organization_packs(organization_id);
CREATE INDEX IF NOT EXISTS idx_organization_packs_pack ON organization_packs(pack_code);
CREATE INDEX IF NOT EXISTS idx_organization_packs_enabled ON organization_packs(enabled);
CREATE INDEX IF NOT EXISTS idx_pack_policies_pack ON pack_policies(pack_code);

INSERT INTO vertical_packs (
    id, code, name, description,
    version_major, version_minor, version_patch,
    status, capabilities, supported_actions, workflow_json, ui_metadata, ai_profile,
    created_at, updated_at
) VALUES
(
    'ECOMMERCE', 'ECOMMERCE', 'E-Commerce Pack', 'Vertical pack for e-commerce marketplace operations',
    1, 0, 0, 'ACTIVE',
    '["CATALOG","SEARCH","ORDERING","INVENTORY","FULFILLMENT","CUSTOMER_SUPPORT","MERCHANT_OPERATIONS"]',
    '["ActionSearchListings","ActionGetListing","ActionCompareListings","ActionSuggest","ActionCreateListing","ActionPublishListing","ActionPauseListing","ActionResumeListing","ActionCreateOrder","ActionGetOrder","ActionCancelOrder","ActionGetFulfillmentStatus","ActionInitiatePayment","ActionGetPaymentStatus","ActionGetDeliveryStatus","ActionAssignFulfillment","ActionTransitionFulfillment","ActionUpdateFulfillmentTracking"]',
    '{"name":"ecommerce_checkout","entryStep":"listing","steps":[{"name":"listing","action":"ActionSearchListings","nextSteps":["details","order"]},{"name":"details","action":"ActionGetListing","nextSteps":["checkout"]},{"name":"checkout","action":"ActionCreateOrder","nextSteps":["payment"]},{"name":"payment","action":"ActionInitiatePayment","nextSteps":["fulfillment"]},{"name":"fulfillment","action":"ActionAssignFulfillment","nextSteps":["tracking"]},{"name":"tracking","action":"ActionUpdateFulfillmentTracking","nextSteps":[]}]}',
    '{"routePrefix":"/commerce","icon":"shopping-cart","color":"#2563eb","order":1}',
    'COMMERCE_AI',
    1726979200, 1726979200
),
(
    'BOOKING', 'BOOKING', 'Booking & Hospitality Pack', 'Vertical pack for booking and reservation operations',
    1, 0, 0, 'ACTIVE',
    '["CATALOG","SEARCH","AVAILABILITY","RESERVATION","FULFILLMENT","CUSTOMER_SUPPORT"]',
    '["ActionSearchListings","ActionGetListing","ActionSuggest","ActionCreateReservation","ActionCheckAvailability","ActionGetReservation","ActionCancelReservation","ActionModifyReservation","ActionGetPaymentStatus","ActionGetDeliveryStatus"]',
    '{"name":"booking_flow","entryStep":"search","steps":[{"name":"search","action":"ActionSearchListings","nextSteps":["details"]},{"name":"details","action":"ActionGetListing","nextSteps":["availability"]},{"name":"availability","action":"ActionCheckAvailability","nextSteps":["reservation"]},{"name":"reservation","action":"ActionCreateReservation","nextSteps":["confirmation"]},{"name":"confirmation","action":"ActionGetReservation","nextSteps":["reminder"]},{"name":"reminder","nextSteps":[]}]}',
    '{"routePrefix":"/booking","icon":"calendar-check","color":"#059669","order":2}',
    'BOOKING_AI',
    1726979200, 1726979200
),
(
    'SERVICES', 'SERVICES', 'Services Pack', 'Vertical pack for service provider operations',
    1, 0, 0, 'ACTIVE',
    '["CATALOG","SEARCH","AVAILABILITY","SCHEDULING","CAPACITY","CUSTOMER_SUPPORT","MERCHANT_OPERATIONS"]',
    '["ActionSearchListings","ActionGetListing","ActionSuggest","ActionCreateReservation","ActionCheckAvailability","ActionGetReservation","ActionCancelReservation","ActionModifyReservation","ActionCheckInventory","ActionAdjustInventory","ActionGetFulfillmentStatus","ActionGetPaymentStatus"]',
    '{"name":"service_flow","entryStep":"inquiry","steps":[{"name":"inquiry","action":"ActionSearchListings","nextSteps":["availability"]},{"name":"availability","action":"ActionCheckAvailability","nextSteps":["booking"]},{"name":"booking","action":"ActionCreateReservation","nextSteps":["confirmation"]},{"name":"confirmation","action":"ActionGetReservation","nextSteps":["fulfillment"]},{"name":"fulfillment","action":"ActionAssignFulfillment","nextSteps":[]}]}',
    '{"routePrefix":"/services","icon":"wrench","color":"#7c3aed","order":3}',
    'COMMERCE_AI',
    1726979200, 1726979200
);

INSERT INTO pack_policies (pack_code, policy_key, policy_value, created_at, updated_at) VALUES
('ECOMMERCE', 'require_payment_before_fulfillment', 'true', 1726979200, 1726979200),
('ECOMMERCE', 'allow_partial_reservations', 'false', 1726979200, 1726979200),
('ECOMMERCE', 'inventory_decrement_on', 'order_created', 1726979200, 1726979200),
('BOOKING', 'require_payment_before_reservation', 'true', 1726979200, 1726979200),
('BOOKING', 'allow_overbooking', 'false', 1726979200, 1726979200),
('BOOKING', 'min_advance_booking_hours', '1', 1726979200, 1726979200),
('SERVICES', 'require_payment_before_reservation', 'true', 1726979200, 1726979200),
('SERVICES', 'allow_scheduling_into_future_days', '90', 1726979200, 1726979200),
('SERVICES', 'require_provider_assignment', 'true', 1726979200, 1726979200);
