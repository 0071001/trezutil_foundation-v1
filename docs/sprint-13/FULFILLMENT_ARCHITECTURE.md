# Sprint 13 — Fulfillment Architecture

## Overview

Fulfillment represents the **execution and completion** of a commercial transaction. It is the bridge between the commercial world (orders, reservations) and the physical/digital delivery world.

**Key principle**: Not all orders/reservations require fulfillment. Fulfillment is optional and only created when needed.

---

## Fulfillment Types

| Type | Description | Examples |
|------|-------------|----------|
| `DELIVERY` | Physical goods shipped to customer | E-commerce, food delivery |
| `PICKUP` | Customer collects from location | Retail click-and-collect, restaurant takeout |
| `APPOINTMENT` | Service rendered at scheduled time | Consultations, beauty, medical |
| `DIGITAL` | Digital goods delivered electronically | Software licenses, e-books, tickets |
| `EVENT_ENTRY` | Entry to an event/venue | Concerts, sports, conferences |
| `TRAVEL_TICKET` | Travel document | Flight boarding pass, train ticket |
| `SERVICE_COMPLETION` | Service marked as done | Repair, cleaning, freelance work |
| `PROPERTY_VISIT` | Property viewing/inspection | Real estate, rental viewings |
| `NONE` | No fulfillment required | Pure digital services, subscriptions |

---

## Fulfillment Aggregate

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `tenant_id` | UUID | Yes | Organization (tenant) |
| `order_id` | UUID | No* | Linked order (if order-based) |
| `reservation_id` | UUID | No* | Linked reservation (if reservation-based) |
| `type` | string | Yes | `DELIVERY`, `PICKUP`, `APPOINTMENT`, etc. |
| `status` | string | Yes | `PENDING`, `ACCEPTED`, `IN_PROGRESS`, `READY`, `COMPLETED`, `CANCELLED`, `FAILED` |
| `reference` | string | Yes | Internal reference number |
| `tracking_ref` | string | No | External tracking (carrier, etc.) |
| `assigned_to` | UUID | No | Actor assigned to fulfill |
| `metadata` | JSONB | No | Flexible type-specific data |
| `created_at` | timestamp | Yes | |
| `updated_at` | timestamp | Yes | |

*Exactly one of `order_id` or `reservation_id` must be set.

### Derived

```go
IsTerminal() bool = status in {COMPLETED, CANCELLED, FAILED}
```

---

## State Machine

```
PENDING
  ↓ (assigned/accepted)
ACCEPTED
  ↓ (work started)
IN_PROGRESS
  ↓ (ready for customer)
READY
  ↓ (customer receives/completes)
COMPLETED

Any state → CANCELLED (explicit cancellation)
Any state → FAILED (unrecoverable error)
```

### Valid Transitions

| From | To | Trigger |
|------|-----|---------|
| `PENDING` | `ACCEPTED` | Assigned to fulfiller |
| `PENDING` | `CANCELLED` | Cancelled before start |
| `ACCEPTED` | `IN_PROGRESS` | Work begun |
| `ACCEPTED` | `CANCELLED` | Cancelled after acceptance |
| `IN_PROGRESS` | `READY` | Ready for handoff |
| `IN_PROGRESS` | `FAILED` | Unrecoverable issue |
| `READY` | `COMPLETED` | Customer confirms receipt |
| `READY` | `CANCELLED` | Customer cancels at pickup |
| `READY` | `IN_PROGRESS` | Re-opened (e.g., issue) |

---

## Order → Fulfillment Integration

### Creation Triggers

Fulfillment created when:
1. Order reaches `CONFIRMED` or `READY_FOR_FULFILLMENT` status
2. Seller/admin explicitly creates fulfillment
3. Automatic based on listing `FulfillmentType` metadata

### Automatic Creation Logic

```go
// In OrderUseCases.Confirm() or ReadyForFulfillment()
if listing.FulfillmentType != "NONE" {
    fulfillment := NewFulfillment(
        order.TenantID,
        order.ID,
        nil,
        listing.FulfillmentType,
        now,
    )
    fulfillmentRepo.Create(ctx, fulfillment)
}
```

---

## Reservation → Fulfillment Integration

### Creation Triggers

Fulfillment created when:
1. Reservation reaches `CONFIRMED` status (for appointment/event types)
2. Explicit creation by seller
3. Check-in time approaches

### Types Mapped

| Reservation Type | Fulfillment Type |
|------------------|------------------|
| Hotel booking | `TRAVEL_TICKET` / `SERVICE_COMPLETION` |
| Restaurant reservation | `SERVICE_COMPLETION` |
| Appointment | `APPOINTMENT` |
| Event ticket | `EVENT_ENTRY` |
| Vehicle rental | `PICKUP` / `SERVICE_COMPLETION` |
| Property visit | `PROPERTY_VISIT` |

---

## Fulfillment Provider Port (Future)

For Sprint 13, **no external providers are integrated**. The architecture supports future provider abstraction:

```go
type FulfillmentProvider interface {
    Create(ctx context.Context, fulfillment *Fulfillment) (externalRef string, error)
    UpdateStatus(ctx context.Context, fulfillment *Fulfillment) error
    GetTracking(ctx context.Context, trackingRef string) (TrackingInfo, error)
    Cancel(ctx context.Context, fulfillment *Fulfillment) error
}
```

---

## Database Schema

### fulfillments table

```sql
CREATE TABLE fulfillments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    reservation_id UUID REFERENCES reservations(id) ON DELETE SET NULL,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
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

CREATE INDEX idx_fulfillments_tenant_id ON fulfillments(tenant_id);
CREATE INDEX idx_fulfillments_order_id ON fulfillments(order_id);
CREATE INDEX idx_fulfillments_reservation_id ON fulfillments(reservation_id);
CREATE INDEX idx_fulfillments_status ON fulfillments(status);
CREATE INDEX idx_fulfillments_assigned_to ON fulfillments(assigned_to);
CREATE UNIQUE INDEX idx_fulfillments_reference ON fulfillments(reference);
```

---

## API Surface

### GET /v1/fulfillments/{id}

**Response:**
```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "order_id": "uuid|null",
  "reservation_id": "uuid|null",
  "type": "DELIVERY",
  "status": "IN_PROGRESS",
  "reference": "FUL-2024-001",
  "tracking_ref": "TRK-12345",
  "assigned_to": "uuid|null",
  "metadata": {},
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T14:30:00Z"
}
```

### POST /v1/fulfillments/{id}/transition

**Request:**
```json
{
  "target_status": "IN_PROGRESS",
  "actor_id": "uuid"
}
```

**Response:** Updated fulfillment object.

---

## Commerce Actions

| Action | Description |
|--------|-------------|
| `GET_FULFILLMENT_STATUS` | Query fulfillment status (existing) |
| `TRANSITION_FULFILLMENT` | Advance fulfillment state (new) |

---

## Authorization

| Actor | Permissions |
|-------|-------------|
| Customer | Read own fulfillment (via order/reservation) |
| Seller/Store Operator | Read/write fulfillments for their listings |
| Organization Admin | Full access to org fulfillments |
| Platform Operator | Full access |

---

## Web UI

### Customer View
- Order detail page → Fulfillment status card
- Reservation detail page → Fulfillment status card
- Visual states: `PENDING` (gray), `IN_PROGRESS` (blue), `READY` (mint), `COMPLETED` (green), `CANCELLED` (red)

### Seller View
- Fulfillment list with filters
- Assigned fulfillments dashboard
- Transition buttons per valid state change