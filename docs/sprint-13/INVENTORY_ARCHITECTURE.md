# Sprint 13 — Inventory Architecture

## Overview

This document describes the generic Inventory, Capacity & Fulfillment architecture for the Trezutil Commerce & Booking OS. The design maintains strict separation between concepts and enforces database-level invariants for concurrency safety.

---

## Core Separation Principle

```text
Listing
  = what is offered (marketing, pricing, attributes)

Inventory
  = what quantity/resource exists (stock, units, countable resources)

Capacity
  = how much can be used simultaneously (seats, slots, rooms, vehicles)

Availability
  = whether the offer/resource is usable under requested conditions
      (derived from Inventory + Capacity + Reservations + Business rules)

Order
  = commercial purchase (payment, fulfillment)

Reservation
  = temporary or committed booking (time-bound capacity hold)

Fulfillment
  = execution/completion of the transaction
```

---

## Inventory Model

### Resource Types

| Type | Description | Example |
|------|-------------|---------|
| `COUNTABLE` | Discrete countable units | Physical products, tickets |
| `CAPACITY` | Simultaneous usage capacity | Restaurant seats, hotel rooms |
| `TIME_SLOT` | Time-based capacity units | Appointment slots, rental hours |
| `UNIT` | Generic unit | Custom units |
| `SEAT` | Seat-based capacity | Events, transportation |
| `ROOM` | Room-based capacity | Hotels, venues |
| `VEHICLE` | Vehicle-based capacity | Car rentals, fleet |
| `ASSET` | Asset-based capacity | Equipment rental |

### Inventory Resource

```go
type InventoryResource struct {
    ID            string              // UUID
    TenantID      string              // Organization ID (tenant)
    ListingID     string              // References Listing
    ResourceType  ResourceType        // COUNTABLE, CAPACITY, TIME_SLOT, etc.
    Total         int64               // Total quantity/capacity
    Reserved      int64               // Currently reserved
    Consumed      int64               // Consumed/fulfilled
    Version       int64               // Optimistic locking
    CreatedAt     int64
    UpdatedAt     int64
}
```

### Derived Fields (Computed)

```go
Available() int64 = Total - Reserved - Consumed
```

**Invariant**: `Available() >= 0` enforced at database level.

---

## Capacity Model

### Capacity Unit

For time-based resources, capacity is expressed per time interval:

```go
type CapacityUnit struct {
    ID              string
    InventoryID     string            // Parent InventoryResource
    StartAt         int64             // Interval start (unix timestamp)
    EndAt           int64             // Interval end (unix timestamp)
    Capacity        int64             // Total capacity in this interval
    Reserved        int64             // Reserved in this interval
    Version         int64
    CreatedAt       int64
    UpdatedAt       int64
}
```

### Derived Availability

```go
AvailableInInterval(startAt, endAt int64) int64
```

---

## Reservation ↔ Inventory/Capacity Integration

### Reservation Hold

When a reservation is created:
1. Check `AvailabilityProvider` (includes capacity check)
2. If available, create `Reservation` with status `PENDING`/`CONFIRMED`
3. Atomically increment `InventoryResource.Reserved` (or `CapacityUnit.Reserved`)
4. On cancellation/release: decrement `Reserved`

### Double-Booking Prevention

- **Inventory**: Database `CHECK (reserved + consumed <= total)`
- **Capacity**: Exclusion constraint on `CapacityUnit` (similar to reservations)

---

## Order ↔ Inventory Integration

### Consumption Point

Inventory is consumed at **Order Confirmation** (`PAID` → `CONFIRMED` transition):
- Atomic transaction: Order status update + Inventory decrement
- `Reserved` → `Consumed` transfer (if reservation existed)
- Direct `Consumed` increment (if no reservation)

### Rollback

If order is cancelled before confirmation:
- Release reservation hold (decrement `Reserved`)
- No `Consumed` change

---

## Fulfillment Model

### Fulfillment Entity

```go
type Fulfillment struct {
    ID                string
    TenantID          string
    OrderID           *string          // Optional: order-based
    ReservationID     *string          // Optional: reservation-based
    Type              FulfillmentType  // DELIVERY, PICKUP, APPOINTMENT, etc.
    Status            FulfillmentStatus
    Reference         string           // Internal reference
    TrackingRef       *string          // External tracking
    AssignedTo        *string          // Actor assigned
    CreatedAt         int64
    UpdatedAt         int64
}
```

### State Machine

```
PENDING → ACCEPTED → IN_PROGRESS → READY → COMPLETED
                ↓
             CANCELLED
                ↓
             FAILED
```

---

## Database Enforcement Strategy

| Invariant | Mechanism |
|-----------|-----------|
| `available >= 0` | `CHECK (reserved + consumed <= total)` |
| `reserved >= 0` | `CHECK (reserved >= 0)` |
| `consumed >= 0` | `CHECK (consumed >= 0)` |
| No overlapping capacity holds | Exclusion constraint on `capacity_units` |
| Optimistic locking | `version` column on all mutable tables |
| Tenant isolation | `tenant_id` FK + row-level policies (future) |

---

## Concurrency Model

### Inventory Operations

| Operation | Lock Strategy |
|-----------|---------------|
| `RESERVE` | `SELECT FOR UPDATE` on InventoryResource + conditional UPDATE |
| `RELEASE` | `SELECT FOR UPDATE` + conditional UPDATE |
| `CONSUME` | `SELECT FOR UPDATE` + conditional UPDATE (atomic with Order) |
| `RESTORE` | `SELECT FOR UPDATE` + conditional UPDATE |
| `ADJUST` | `SELECT FOR UPDATE` + conditional UPDATE (admin only) |

### Capacity Operations

| Operation | Lock Strategy |
|-----------|---------------|
| Reserve slot | Exclusion constraint on `capacity_units` (like reservations) |
| Release slot | DELETE from hold table + decrement counter |

---

## API Surface

### Inventory

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/inventory` | List inventory resources (seller/admin) |
| GET | `/v1/inventory/{id}` | Get inventory resource |
| POST | `/v1/inventory/{id}/adjust` | Adjust inventory (admin/seller) |

### Capacity

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/capacity/{inventoryId}` | Get capacity units for date range |
| POST | `/v1/capacity/{inventoryId}/reserve` | Reserve capacity slot |

### Fulfillment

| Method | Path | Description |
|--------|------|-------------|
| GET | `/v1/fulfillments/{id}` | Get fulfillment status |
| POST | `/v1/fulfillments/{id}/transition` | Transition fulfillment state |

---

## Commerce Actions

New action types (if not existing):

- `CHECK_INVENTORY` (already in `intent.go`)
- `ADJUST_INVENTORY` (new)
- `GET_FULFILLMENT_STATUS` (already in `intent.go`)
- `TRANSITION_FULFILLMENT` (new)

---

## Usage & Credits Integration

Metered operations:
- `ADJUST_INVENTORY` — seller operations
- Advanced inventory queries — optional
- Fulfillment tracking — optional

---

## Cross-Channel Consistency

| Channel | Implementation |
|---------|----------------|
| REST API | Direct handlers |
| Web | React components calling API |
| WhatsApp | `CHECK_INVENTORY` → `CommerceActionRuntime` → Response |

All channels share the same application use cases.