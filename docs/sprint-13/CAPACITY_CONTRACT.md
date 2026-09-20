# Sprint 13 — Capacity Contract

## Overview

Capacity represents **simultaneous usage limits** for time-based resources. Unlike inventory (which tracks total stock), capacity tracks how many units can be active at the same time.

Examples:
- Restaurant: 50 seats (capacity = 50 concurrent diners)
- Hotel: 100 rooms (capacity = 100 concurrent guests)
- Event: 500 tickets (capacity = 500 concurrent attendees)
- Appointment: 3 slots per hour (capacity = 3 concurrent appointments)
- Vehicle rental: 20 vehicles (capacity = 20 concurrent rentals)

---

## Capacity Unit Aggregate

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `inventory_id` | UUID | Yes | Parent InventoryResource |
| `start_at` | timestamp with TZ | Yes | Interval start (inclusive) |
| `end_at` | timestamp with TZ | Yes | Interval end (exclusive) |
| `capacity` | int64 | Yes | Total capacity in this interval |
| `reserved` | int64 | Yes | Currently reserved in interval |
| `version` | int64 | Yes | Optimistic lock |
| `created_at` | timestamp | Yes | |
| `updated_at` | timestamp | Yes | |

### Derived

```go
Available() int64 = Capacity - Reserved
```

### Invariants (Database-Enforced)

```sql
CHECK (capacity > 0)
CHECK (reserved >= 0)
CHECK (reserved <= capacity)
CHECK (end_at > start_at)
CHECK (version >= 1)
```

---

## Domain Methods

### NewCapacityUnit

```go
func NewCapacityUnit(
    inventoryID string,
    startAt, endAt int64,
    capacity int64,
    now int64,
) *CapacityUnit
```

### Reserve

```go
func (c *CapacityUnit) Reserve(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `c.Available() >= quantity`

**Effects:**
- `c.Reserved += quantity`
- `c.Version++`
- `c.UpdatedAt = now`

### Release

```go
func (c *CapacityUnit) Release(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `c.Reserved >= quantity`

**Effects:**
- `c.Reserved -= quantity`
- `c.Version++`
- `c.UpdatedAt = now`

---

## Capacity Query

### Availability Query

```go
type CapacityQuery struct {
    InventoryID string
    StartAt     int64
    EndAt       int64
    Quantity    int
}
```

### Availability Result

```go
type CapacityResult struct {
    Available     bool
    TotalCapacity int64
    AvailableCapacity int64
    Units         []CapacityUnit // Units overlapping the query range
}
```

### Logic

A query for `[StartAt, EndAt)` overlaps with capacity units where:
```
unit.start_at < query.end_at AND unit.end_at > query.start_at
```

Available capacity = min of overlapping units' available capacity.

---

## Reservation ↔ Capacity Integration

### Reservation Creates Capacity Hold

When a time-based reservation is created:
1. Query `CapacityUnit`s overlapping the reservation period
2. For each overlapping unit: `Reserve(quantity)`
3. Create `Reservation` record
4. All in single transaction

### Cancellation Releases Capacity Hold

When reservation is cancelled:
1. Find overlapping `CapacityUnit`s
2. For each: `Release(quantity)`
3. Update `Reservation` status
4. All in single transaction

---

## Concurrency Model

### Exclusion Constraint (Primary)

PostgreSQL exclusion constraint prevents overlapping holds exceeding capacity:

```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE capacity_units
ADD CONSTRAINT no_overbooking
EXCLUDE USING GIST (
    inventory_id WITH =,
    tsrange(start_at, end_at) WITH &&
) WHERE (reserved >= capacity);  -- Prevents reserved > capacity
```

**Note**: This is a simplified model. Actual implementation uses a `capacity_holds` table similar to reservations.

### Alternative: Hold Table (More Flexible)

```sql
CREATE TABLE capacity_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capacity_unit_id UUID NOT NULL REFERENCES capacity_units(id),
    reservation_id UUID NOT NULL REFERENCES reservations(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (quantity > 0)
);

-- Exclusion constraint on holds
ALTER TABLE capacity_holds
ADD CONSTRAINT no_overbooking_holds
EXCLUDE USING GIST (
    capacity_unit_id WITH =,
    tsrange(
        (SELECT start_at FROM capacity_units WHERE id = capacity_unit_id),
        (SELECT end_at FROM capacity_units WHERE id = capacity_unit_id)
    ) WITH &&
) WHERE (false); -- Enforced via trigger or application logic
```

### Recommended Approach: Application-Level + Row Locks

For Sprint 13, use:
1. `SELECT FOR UPDATE` on overlapping `CapacityUnit` rows
2. Check `Available() >= quantity`
3. `UPDATE ... SET reserved = reserved + quantity WHERE version = $1`
4. Retry on version conflict

This avoids complex exclusion constraints while maintaining safety.

---

## Errors

| Error | Code | Description |
|-------|------|-------------|
| `ErrCapacityNotFound` | `CAPACITY_NOT_FOUND` | Capacity unit not found |
| `ErrInsufficientCapacity` | `INSUFFICIENT_CAPACITY` | `Available() < requested` |
| `ErrInvalidCapacityQuantity` | `INVALID_QUANTITY` | Quantity <= 0 or exceeds Reserved |
| `ErrOptimisticLock` | `OPTIMISTIC_LOCK` | Version mismatch |

---

## Tenant Isolation

All capacity operations scoped by `tenant_id` via `inventory_id` FK.

---

## Scheduling Algorithms (Out of Scope for Sprint 13)

The following are explicitly NOT implemented in Sprint 13:
- Complex scheduling (recurring, patterns, exceptions)
- Resource allocation optimization
- Waitlist management
- Dynamic pricing based on capacity utilization

These belong to future sprints or vertical-specific extensions.