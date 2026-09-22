-- +goose Up
-- SQL in section 'Up' is executed when migration is applied

-- Inventory Resources table
CREATE TABLE IF NOT EXISTS inventory_resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    listing_id UUID NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    resource_type VARCHAR(50) NOT NULL,
    total BIGINT NOT NULL DEFAULT 0,
    reserved BIGINT NOT NULL DEFAULT 0,
    consumed BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (resource_type IN ('COUNTABLE','CAPACITY','TIME_SLOT','UNIT','SEAT','ROOM','VEHICLE','ASSET')),
    CHECK (total >= 0),
    CHECK (reserved >= 0),
    CHECK (consumed >= 0),
    CHECK (reserved + consumed <= total),
    CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_inventory_resources_tenant_id ON inventory_resources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_inventory_resources_listing_id ON inventory_resources(listing_id);
CREATE INDEX IF NOT EXISTS idx_inventory_resources_resource_type ON inventory_resources(resource_type);
CREATE UNIQUE INDEX IF NOT EXISTS idx_inventory_resources_listing_type ON inventory_resources(listing_id, resource_type);

-- Capacity Units table (time-based capacity slices)
CREATE TABLE IF NOT EXISTS capacity_units (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inventory_id UUID NOT NULL REFERENCES inventory_resources(id) ON DELETE CASCADE,
    start_at TIMESTAMP WITH TIME ZONE NOT NULL,
    end_at TIMESTAMP WITH TIME ZONE NOT NULL,
    capacity BIGINT NOT NULL,
    reserved BIGINT NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (capacity > 0),
    CHECK (reserved >= 0),
    CHECK (reserved <= capacity),
    CHECK (end_at > start_at),
    CHECK (version >= 1)
);

CREATE INDEX IF NOT EXISTS idx_capacity_units_inventory_id ON capacity_units(inventory_id);
CREATE INDEX IF NOT EXISTS idx_capacity_units_time_range ON capacity_units(start_at, end_at);

-- Fulfillments table
CREATE TABLE IF NOT EXISTS fulfillments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    reservation_id UUID REFERENCES reservations(id) ON DELETE SET NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    reference VARCHAR(100) NOT NULL,
    tracking_ref VARCHAR(200),
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (type IN ('DELIVERY','PICKUP','APPOINTMENT','DIGITAL','EVENT_ENTRY','TRAVEL_TICKET','SERVICE_COMPLETION','PROPERTY_VISIT','NONE')),
    CHECK (status IN ('PENDING','ACCEPTED','IN_PROGRESS','READY','COMPLETED','CANCELLED','FAILED')),
    CHECK (version >= 1),
    CHECK (
        (order_id IS NOT NULL AND reservation_id IS NULL) OR
        (order_id IS NULL AND reservation_id IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_fulfillments_tenant_id ON fulfillments(tenant_id);
CREATE INDEX IF NOT EXISTS idx_fulfillments_order_id ON fulfillments(order_id);
CREATE INDEX IF NOT EXISTS idx_fulfillments_reservation_id ON fulfillments(reservation_id);
CREATE INDEX IF NOT EXISTS idx_fulfillments_status ON fulfillments(status);
CREATE INDEX IF NOT EXISTS idx_fulfillments_assigned_to ON fulfillments(assigned_to);
CREATE UNIQUE INDEX IF NOT EXISTS idx_fulfillments_reference ON fulfillments(reference);

-- Inventory audit log for tracking all inventory changes
CREATE TABLE IF NOT EXISTS inventory_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    inventory_id UUID NOT NULL REFERENCES inventory_resources(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    delta_total BIGINT NOT NULL DEFAULT 0,
    delta_reserved BIGINT NOT NULL DEFAULT 0,
    delta_consumed BIGINT NOT NULL DEFAULT 0,
    previous_total BIGINT NOT NULL,
    previous_reserved BIGINT NOT NULL,
    previous_consumed BIGINT NOT NULL,
    reason VARCHAR(50),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (action IN ('RESERVE','RELEASE','CONSUME','RESTORE','ADJUST','RECEIVE','DAMAGE','CORRECTION','RESTOCK','COUNT_ADJUSTMENT'))
);

CREATE INDEX IF NOT EXISTS idx_inventory_audit_inventory_id ON inventory_audit(inventory_id);
CREATE INDEX IF NOT EXISTS idx_inventory_audit_actor_id ON inventory_audit(actor_id);
CREATE INDEX IF NOT EXISTS idx_inventory_audit_created_at ON inventory_audit(created_at);

-- Capacity audit log
CREATE TABLE IF NOT EXISTS capacity_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    capacity_unit_id UUID NOT NULL REFERENCES capacity_units(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    quantity BIGINT NOT NULL,
    previous_reserved BIGINT NOT NULL,
    reservation_id UUID REFERENCES reservations(id) ON DELETE SET NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (action IN ('RESERVE','RELEASE')),
    CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS idx_capacity_audit_capacity_unit_id ON capacity_audit(capacity_unit_id);
CREATE INDEX IF NOT EXISTS idx_capacity_audit_reservation_id ON capacity_audit(reservation_id);

-- Fulfillment audit log
CREATE TABLE IF NOT EXISTS fulfillment_audit (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    fulfillment_id UUID NOT NULL REFERENCES fulfillments(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(50) NOT NULL,
    previous_status VARCHAR(50),
    new_status VARCHAR(50),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CHECK (action IN ('CREATED','ASSIGNED','TRANSITIONED','TRACKING_UPDATED','CANCELLED','FAILED'))
);

CREATE INDEX IF NOT EXISTS idx_fulfillment_audit_fulfillment_id ON fulfillment_audit(fulfillment_id);
CREATE INDEX IF NOT EXISTS idx_fulfillment_audit_actor_id ON fulfillment_audit(actor_id);

-- +goose Down
-- SQL in section 'Down' is executed when migration is rolled back

DROP INDEX IF EXISTS idx_fulfillment_audit_actor_id;
DROP INDEX IF EXISTS idx_fulfillment_audit_fulfillment_id;
DROP TABLE IF EXISTS fulfillment_audit;

DROP INDEX IF EXISTS idx_capacity_audit_reservation_id;
DROP INDEX IF EXISTS idx_capacity_audit_capacity_unit_id;
DROP TABLE IF EXISTS capacity_audit;

DROP INDEX IF EXISTS idx_inventory_audit_created_at;
DROP INDEX IF EXISTS idx_inventory_audit_actor_id;
DROP INDEX IF EXISTS idx_inventory_audit_inventory_id;
DROP TABLE IF EXISTS inventory_audit;

DROP INDEX IF EXISTS idx_fulfillments_reference;
DROP INDEX IF EXISTS idx_fulfillments_assigned_to;
DROP INDEX IF EXISTS idx_fulfillments_status;
DROP INDEX IF EXISTS idx_fulfillments_reservation_id;
DROP INDEX IF EXISTS idx_fulfillments_order_id;
DROP INDEX IF EXISTS idx_fulfillments_tenant_id;
ALTER TABLE fulfillments DROP CONSTRAINT IF EXISTS fulfillments_order_id_fkey;
ALTER TABLE fulfillments DROP CONSTRAINT IF EXISTS fulfillments_reservation_id_fkey;
DROP TABLE IF EXISTS fulfillments;

DROP INDEX IF EXISTS idx_capacity_units_time_range;
DROP INDEX IF EXISTS idx_capacity_units_inventory_id;
ALTER TABLE capacity_units DROP CONSTRAINT IF EXISTS capacity_units_inventory_id_fkey;
DROP TABLE IF EXISTS capacity_units;

DROP INDEX IF EXISTS idx_inventory_resources_resource_type;
DROP INDEX IF EXISTS idx_inventory_resources_listing_type;
DROP INDEX IF EXISTS idx_inventory_resources_listing_id;
DROP INDEX IF EXISTS idx_inventory_resources_tenant_id;
ALTER TABLE inventory_resources DROP CONSTRAINT IF EXISTS inventory_resources_listing_id_fkey;
ALTER TABLE inventory_resources DROP CONSTRAINT IF EXISTS inventory_resources_tenant_id_fkey;
DROP TABLE IF EXISTS inventory_resources;