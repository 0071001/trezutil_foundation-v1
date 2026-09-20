# Sprint 12 — Concurrency / Double Booking Model

## Problem Statement

Prevent two customers from successfully reserving the same resource for overlapping periods.

```
Customer A: Checks room → Available
Customer B: Checks room → Available
Customer A: Books room
Customer B: Books room  ← MUST FAIL
```

## Solution: PostgreSQL Exclusion Constraint

### Migration
```sql
CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE reservations
ADD CONSTRAINT no_overlapping_reservations
EXCLUDE USING GIST (
    listing_id WITH =,
    tsrange(start_at, end_at) WITH &&
) WHERE (status <> 'cancelled');
```

### How It Works

| Component | Purpose |
|-----------|---------|
| `btree_gist` | Enables GIST index on UUID + range types |
| `listing_id WITH =` | Exact match on listing |
| `tsrange(start_at, end_at) WITH &&` | Temporal overlap operator |
| `WHERE (status <> 'cancelled')` | Ignore cancelled reservations |

### Guarantees

| Scenario | Guarantee |
|----------|-----------|
| Same listing, overlapping dates | **Prevented at DB level** |
| Same listing, adjacent dates (end=start) | **Allowed** (no overlap) |
| Different listings | **Allowed** (different listing_id) |
| Cancelled reservations | **Ignored** (filtered by WHERE) |
| Concurrent INSERTs | **Serialized by DB** — one wins, one gets 23P01 error |

### Error Handling

PostgreSQL error code `23P01` (exclusion_constraint_violation) is caught and mapped:

```go
if err := u.reservationRepo.Create(ctx, reservation); err != nil {
    if isExclusionConstraintError(err) {
        return nil, domain.ErrReservationConflict
    }
    return nil, fmt.Errorf("failed to create reservation: %w", err)
}
```

## Alternative Strategies Considered

| Strategy | Pros | Cons | Decision |
|----------|------|------|----------|
| SELECT then INSERT | Simple | Race condition | ❌ Rejected |
| Advisory Locks | Flexible | App-level, not DB-enforced | ❌ Rejected |
| Optimistic Versioning | Works for updates | Doesn't prevent INSERT conflicts | ✅ Used for updates |
| **Exclusion Constraint** | **DB-enforced, no race** | **Requires PostgreSQL 9.2+** | ✅ **Chosen** |

## Concurrency Test

```go
func TestConcurrentReservation(t *testing.T) {
    // Setup: 1 room, 2 customers, same dates
    listingID := createListing(t, 1) // capacity 1
    
    var wg sync.WaitGroup
    results := make(chan error, 2)
    
    for i := 0; i < 2; i++ {
        wg.Add(1)
        go func(customerID string) {
            defer wg.Done()
            _, err := useCases.CreateReservation(ctx, customerID, listingID, start, end, 1, 0, "")
            results <- err
        }(customers[i])
    }
    
    wg.Wait()
    close(results)
    
    var success, conflict int
    for err := range results {
        if err == nil {
            success++
        } else if errors.Is(err, domain.ErrReservationConflict) {
            conflict++
        }
    }
    
    assert.Equal(t, 1, success)   // Exactly one succeeds
    assert.Equal(t, 1, conflict)  // One gets conflict
}
```

## Limitations

1. **Only works for internal reservations** — External provider availability not protected
2. **Single resource per reservation** — Multi-resource bookings need saga pattern (future)
3. **No distributed lock** — Not needed for single-region PostgreSQL

## Status

**DONE** — Implemented in:
- `migrations/000009_reservations_schema.sql` — Exclusion constraint
- `infrastructure/postgres/reservations.go` — Error mapping
- Tests: Required in TEST_STRATEGY.md

## Files

- `migrations/000009_reservations_schema.sql` — Database constraint
- `infrastructure/postgres/reservations.go` — Error handling