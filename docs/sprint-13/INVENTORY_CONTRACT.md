# Sprint 13 — Inventory Contract

## Inventory Resource Aggregate

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `tenant_id` | UUID | Yes | Organization (tenant) |
| `listing_id` | UUID | Yes | Referenced listing |
| `resource_type` | string | Yes | `COUNTABLE`, `CAPACITY`, `TIME_SLOT`, `UNIT`, `SEAT`, `ROOM`, `VEHICLE`, `ASSET` |
| `total` | int64 | Yes | Total quantity/capacity |
| `reserved` | int64 | Yes | Currently reserved (holds) |
| `consumed` | int64 | Yes | Consumed/fulfilled |
| `version` | int64 | Yes | Optimistic lock |
| `created_at` | timestamp | Yes | Creation time |
| `updated_at` | timestamp | Yes | Last update time |

### Derived Properties

| Property | Formula | Description |
|----------|---------|-------------|
| `available` | `total - reserved - consumed` | Currently available for new reservations/orders |

### Invariants (Database-Enforced)

```sql
CHECK (total >= 0)
CHECK (reserved >= 0)
CHECK (consumed >= 0)
CHECK (reserved + consumed <= total)
CHECK (version >= 1)
```

---

## Domain Methods

### NewInventoryResource

```go
func NewInventoryResource(
    tenantID, listingID string,
    resourceType ResourceType,
    total int64,
    now int64,
) *InventoryResource
```

Creates a new inventory resource with `reserved=0`, `consumed=0`, `version=1`.

### Reserve

```go
func (r *InventoryResource) Reserve(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `r.Available() >= quantity`
- `r.Version` matches current

**Effects:**
- `r.Reserved += quantity`
- `r.Version++`
- `r.UpdatedAt = now`

**Errors:**
- `ErrInsufficientInventory` if `Available() < quantity`
- `ErrOptimisticLock` if version mismatch

### Release

```go
func (r *InventoryResource) Release(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `r.Reserved >= quantity`

**Effects:**
- `r.Reserved -= quantity`
- `r.Version++`
- `r.UpdatedAt = now`

**Errors:**
- `ErrInvalidQuantity` if `quantity > Reserved`

### Consume

```go
func (r *InventoryResource) Consume(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `r.Reserved >= quantity` (if from reservation)
- OR `r.Available() >= quantity` (direct consumption)

**Effects:**
- If from reservation: `r.Reserved -= quantity; r.Consumed += quantity`
- If direct: `r.Consumed += quantity`
- `r.Version++`
- `r.UpdatedAt = now`

### Restore

```go
func (r *InventoryResource) Restore(quantity int64, now int64) error
```

**Preconditions:**
- `quantity > 0`
- `r.Consumed >= quantity`

**Effects:**
- `r.Consumed -= quantity`
- `r.Version++`
- `r.UpdatedAt = now`

### Adjust

```go
func (r *InventoryResource) Adjust(
    deltaTotal int64,
    deltaReserved int64,
    deltaConsumed int64,
    now int64,
) error
```

**Preconditions:**
- Called by authorized seller/admin only
- Resulting `total >= 0`, `reserved >= 0`, `consumed >= 0`
- Resulting `reserved + consumed <= total`

**Effects:**
- Atomically adjusts all three counters
- `r.Version++`
- `r.UpdatedAt = now`

---

## Capacity Unit Aggregate (Time-Based)

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `inventory_id` | UUID | Yes | Parent InventoryResource |
| `start_at` | timestamp | Yes | Interval start |
| `end_at` | timestamp | Yes | Interval end |
| `capacity` | int64 | Yes | Total capacity in interval |
| `reserved` | int64 | Yes | Currently reserved |
| `version` | int64 | Yes | Optimistic lock |
| `created_at` | timestamp | Yes | |
| `updated_at` | timestamp | Yes | |

### Invariants

```sql
CHECK (capacity > 0)
CHECK (reserved >= 0)
CHECK (reserved <= capacity)
CHECK (end_at > start_at)
CHECK (version >= 1)
```

---

## Reservation Hold Integration

### Reservation → Inventory Link

When a reservation is created against an inventory-controlled listing:
1. `InventoryResource.Reserve(quantity)` called
2. Reservation created with `PENDING`/`CONFIRMED` status
3. On cancellation: `InventoryResource.Release(quantity)`

### Order → Inventory Link

When an order transitions `PAID` → `CONFIRMED`:
1. If reservation exists: `InventoryResource.Consume(quantity)` (moves Reserved → Consumed)
2. If no reservation: `InventoryResource.Consume(quantity)` (direct consumption)

---

## Errors

| Error | Code | Description |
|-------|------|-------------|
| `ErrInventoryNotFound` | `INVENTORY_NOT_FOUND` | Inventory resource not found |
| `ErrInsufficientInventory` | `INSUFFICIENT_INVENTORY` | `Available() < requested` |
| `ErrInvalidQuantity` | `INVALID_QUANTITY` | Quantity <= 0 or exceeds Reserved |
| `ErrOptimisticLock` | `OPTIMISTIC_LOCK` | Version mismatch |
| `ErrInsufficientPermissions` | `FORBIDDEN` | Actor not authorized |
| `ErrTenantMismatch` | `TENANT_MISMATCH` | Cross-tenant access attempt |

---

## Tenant Isolation

All queries automatically scoped by `tenant_id`:
- `FindByID(ctx, tenantID, id)`
- `FindByListingID(ctx, tenantID, listingID)`
- `Create(ctx, tenantID, resource)`
- `Update(ctx, tenantID, resource)`

Actor identity resolved server-side from JWT/session.