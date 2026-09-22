# Sprint 13 — Concurrency Model

## Overview

This document describes the concurrency control mechanisms for Inventory, Capacity, and Fulfillment operations. The goal is to prevent:

1. **Negative inventory** — `available < 0`
2. **Over-reservation** — `reserved > total` or `reserved > capacity`
3. **Double-consumption** — consuming same inventory twice
4. **Lost updates** — concurrent modifications overwriting each other

---

## Inventory Concurrency

### Problem

```
Stock = 10

Customer A → order 3
Customer B → order 8

A checks: available = 10 >= 3 ✓
B checks: available = 10 >= 8 ✓

A reserves: reserved = 3
B reserves: reserved = 11  ← VIOLATION (reserved > total)
```

### Solution: Conditional UPDATE with Version Check

```sql
-- Reserve inventory
UPDATE inventory_resources
SET reserved = reserved + $1,
    version = version + 1,
    updated_at = NOW()
WHERE id = $2
  AND version = $3
  AND (reserved + $1 + consumed) <= total
  AND $1 > 0;

-- Check rows affected
-- 0 rows = conflict (version mismatch OR invariant violation)
```

### Go Implementation Pattern

```go
func (r *InventoryRepository) Reserve(ctx context.Context, id string, quantity int64, expectedVersion int64) error {
    query := `
        UPDATE inventory_resources
        SET reserved = reserved + $1,
            version = version + 1,
            updated_at = NOW()
        WHERE id = $2
          AND version = $3
          AND (reserved + $1 + consumed) <= total
          AND $1 > 0
    `
    result, err := r.db.ExecContext(ctx, query, quantity, id, expectedVersion)
    if err != nil {
        return err
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return ErrOptimisticLock // or ErrInsufficientInventory
    }
    return nil
}
```

### Retry Logic

```go
func (uc *InventoryUseCases) ReserveWithRetry(ctx context.Context, id string, quantity int64, maxRetries int) error {
    for attempt := 0; attempt < maxRetries; attempt++ {
        inv, err := r.repo.FindByID(ctx, tenantID, id)
        if err != nil {
            return err
        }
        if inv.Available() < quantity {
            return ErrInsufficientInventory
        }
        err = r.repo.Reserve(ctx, id, quantity, inv.Version)
        if err == nil {
            return nil
        }
        if errors.Is(err, ErrOptimisticLock) {
            continue // retry with fresh version
        }
        return err
    }
    return ErrConcurrencyMaxRetries
}
```

---

## Capacity Concurrency

### Problem

```
Capacity = 3 slots (10:00-11:00)

Customer A → reserve 2
Customer B → reserve 2

Both check: available = 3 >= 2 ✓
Both reserve → reserved = 4  ← VIOLATION
```

### Solution: Row Locks + Conditional UPDATE

```go
func (r *CapacityRepository) Reserve(ctx context.Context, inventoryID string, startAt, endAt int64, quantity int64) error {
    // 1. Lock overlapping capacity units
    query := `
        SELECT id, version FROM capacity_units
        WHERE inventory_id = $1
          AND start_at < $3
          AND end_at > $2
        FOR UPDATE
    `
    rows, err := r.db.QueryContext(ctx, query, inventoryID, startAt, endAt)
    if err != nil {
        return err
    }
    defer rows.Close()

    // 2. Check each unit has capacity
    for rows.Next() {
        var unitID string
        var version int64
        rows.Scan(&unitID, &version)
        
        // Check available
        checkQuery := `SELECT capacity - reserved FROM capacity_units WHERE id = $1`
        var available int64
        r.db.QueryRowContext(ctx, checkQuery, unitID).Scan(&available)
        if available < quantity {
            return ErrInsufficientCapacity
        }
    }

    // 3. Reserve in each unit (all or nothing)
    tx, err := r.db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    for rows.Next() { // re-iterate or store IDs
        _, err := tx.ExecContext(ctx, `
            UPDATE capacity_units
            SET reserved = reserved + $1,
                version = version + 1,
                updated_at = NOW()
            WHERE id = $2 AND version = $3
        `, quantity, unitID, version)
        if err != nil {
            return err
        }
    }

    return tx.Commit()
}
```

### Alternative: Exclusion Constraint

For simpler capacity models, use PostgreSQL exclusion constraint:

```sql
CREATE TABLE capacity_holds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    capacity_unit_id UUID NOT NULL REFERENCES capacity_units(id),
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE capacity_holds
ADD CONSTRAINT no_overbooking
EXCLUDE USING GIST (
    capacity_unit_id WITH =,
    tsrange(
        (SELECT start_at FROM capacity_units WHERE id = capacity_unit_id),
        (SELECT end_at FROM capacity_units WHERE id = capacity_unit_id)
    ) WITH &&
);
-- Note: This requires the capacity_units table to have start_at/end_at
-- and the sum of quantities per unit must not exceed capacity.
-- A trigger or application check is still needed for the capacity limit.
```

---

## Fulfillment Concurrency

### Problem

Multiple actors trying to transition the same fulfillment:

```
Fulfillment: PENDING

Actor A → Transition to ACCEPTED
Actor B → Transition to ACCEPTED

Both succeed → double assignment
```

### Solution: Optimistic Locking + Valid Transition Check

```go
func (r *FulfillmentRepository) Transition(ctx context.Context, id string, targetStatus FulfillmentStatus, expectedVersion int64) error {
    query := `
        UPDATE fulfillments
        SET status = $1,
            version = version + 1,
            updated_at = NOW()
        WHERE id = $2
          AND version = $3
          AND status IN ('PENDING', 'ACCEPTED', 'IN_PROGRESS', 'READY')
          AND $1 IN ('ACCEPTED', 'IN_PROGRESS', 'READY', 'COMPLETED', 'CANCELLED', 'FAILED')
    `
    result, err := r.db.ExecContext(ctx, query, targetStatus, id, expectedVersion)
    if err != nil {
        return err
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        // Could be version mismatch OR invalid transition
        return ErrOptimisticLock // or check current state for better error
    }
    return nil
}
```

---

## Order ↔ Inventory Atomicity

### Critical Section

When order transitions `PAID` → `CONFIRMED`, inventory must be consumed atomically:

```go
func (uc *OrderUseCases) Confirm(ctx context.Context, orderID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        // 1. Lock order
        order, err := uc.orderRepo.FindByIDForUpdate(tx, orderID)
        if err != nil {
            return err
        }
        if order.Status != OrderStatusPaid {
            return ErrInvalidTransition
        }

        // 2. Consume inventory for each item
        for _, item := range order.Items {
            listing, err := uc.listingRepo.FindByID(tx, item.ListingID)
            if err != nil {
                return err
            }
            inv, err := uc.inventoryRepo.FindByListingIDForUpdate(tx, listing.ID)
            if err != nil {
                return err
            }
            if err := inv.Consume(item.Quantity); err != nil {
                return err
            }
            if err := uc.inventoryRepo.Update(tx, inv); err != nil {
                return err
            }
        }

        // 3. Confirm order
        order.Confirm(now)
        return uc.orderRepo.Update(tx, order)
    })
}
```

---

## Reservation ↔ Capacity Atomicity

Similar pattern: lock both reservation and capacity units in same transaction.

---

## Deadlock Prevention

### Lock Ordering Rules

Always acquire locks in consistent order:

1. **Parent before child**: `inventory_resources` before `capacity_units`
2. **By ID**: Sort IDs before locking multiple rows
3. **Short transactions**: Minimize time holding locks

### Example: Consistent Lock Order

```go
func lockResources(ctx context.Context, tx *sql.Tx, inventoryIDs []string) error {
    // Sort IDs to prevent deadlock
    sort.Strings(inventoryIDs)
    
    for _, id := range inventoryIDs {
        _, err := tx.ExecContext(ctx, `
            SELECT 1 FROM inventory_resources WHERE id = $1 FOR UPDATE
        `, id)
        if err != nil {
            return err
        }
    }
    return nil
}
```

---

## Database Guarantees Summary

| Invariant | Mechanism |
|-----------|-----------|
| `total >= 0` | `CHECK (total >= 0)` |
| `reserved >= 0` | `CHECK (reserved >= 0)` |
| `consumed >= 0` | `CHECK (consumed >= 0)` |
| `reserved + consumed <= total` | `CHECK (reserved + consumed <= total)` |
| No double-consumption | Conditional UPDATE + version |
| No lost updates | Optimistic locking (version column) |
| Capacity not exceeded | Row locks + conditional UPDATE |
| Fulfillment valid transitions | CHECK constraint + conditional UPDATE |

---

## Retry Configuration

| Operation | Max Retries | Backoff |
|-----------|-------------|---------|
| Inventory reserve/release/consume | 3 | 10ms, 50ms, 100ms |
| Capacity reserve/release | 3 | 10ms, 50ms, 100ms |
| Fulfillment transition | 3 | 10ms, 50ms, 100ms |
| Order + inventory atomic | 1 (longer tx) | N/A |

---

## Monitoring

Track these metrics:
- `inventory.optimistic_lock_conflicts` — version conflicts
- `inventory.insufficient_errors` — business logic rejections
- `capacity.optimistic_lock_conflicts` — version conflicts
- `fulfillment.transition_conflicts` — version conflicts
- `tx.duration` — transaction latency