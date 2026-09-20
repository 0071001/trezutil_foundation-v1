# Sprint 12 — Availability Contract

## AvailabilityQuery

```go
type AvailabilityQuery struct {
    ListingID  string                 // Required: UUID of the listing
    StartAt    int64                  // Required: Unix timestamp (UTC)
    EndAt      int64                  // Required: Unix timestamp (UTC)
    Quantity   int                    // Optional: Default 1, units to reserve
    PartySize  int                    // Optional: Default 0, people count
    Context    map[string]interface{} // Optional: Extensible context
}
```

### Validation Rules
- `ListingID` must be non-empty
- `StartAt` and `EndAt` must be > 0
- `EndAt > StartAt`
- `Quantity > 0` (defaults to 1)
- `PartySize >= 0` (defaults to 0)

## AvailabilityResult

```go
type AvailabilityResult struct {
    Available  bool   // True if the requested period/quantity is available
    Capacity   int    // Total capacity for the resource in the period
    Remaining  int    // Remaining capacity after existing reservations
    Reason     string // Human-readable reason if unavailable
    StartAt    int64  // Echo of query start
    EndAt      int64  // Echo of query end
}
```

### Semantics
- `Available = true` means the request CAN be reserved (subject to race conditions)
- `Available = false` means the request CANNOT be reserved (definitive)
- `Reason` populated only when `Available = false`
- `Capacity` = maximum theoretical capacity (e.g., total rooms, seats, tickets)
- `Remaining` = `Capacity - sum(existing confirmed reservations)` for the period

## AvailabilityProvider (Port)

```go
type AvailabilityProvider interface {
    CheckAvailability(ctx context.Context, query AvailabilityQuery) (AvailabilityResult, error)
}
```

### Implementations

| Implementation | Source | Use Case |
|----------------|--------|----------|
| `InternalAvailabilityProvider` | ReservationRepository + ListingAvailability | Default: derived from existing reservations + listing capacity |
| `SellerScheduleProvider` | Seller-entered availability | Restaurant tables, appointment slots |
| `InventoryProvider` | Stock/inventory system | Quantity-based (tickets, rentals) |
| `ExternalProvider` | Future: Booking.com, Amadeus, etc. | External inventory |

### Default Implementation Logic
```go
func (p *InternalAvailabilityProvider) CheckAvailability(ctx, query) (AvailabilityResult, error) {
    // 1. Get listing availability record (ListingAvailability)
    // 2. Query existing non-cancelled reservations for period
    // 3. Calculate remaining = capacity - reserved
    // 4. Return Available = (remaining >= query.Quantity)
}
```

## Availability Models Supported

The generic engine supports these models via the Provider abstraction:

### 1. Quantity-Based (Tickets, Rentals)
```
Capacity: 100 tickets
Existing: 30 reserved
Query:    5 tickets
Result:   Available=true, Capacity=100, Remaining=70
```

### 2. Time-Slot Based (Appointments, Restaurant)
```
Capacity: 1 slot at 14:00-15:00
Existing: 0 reservations
Query:    1 slot at 14:00-15:00
Result:   Available=true, Capacity=1, Remaining=1
```

### 3. Date-Range Based (Hotel Rooms, Vehicle Rentals)
```
Capacity: 10 rooms
Existing: 3 rooms reserved for 2026-10-12 to 2026-10-15
Query:    1 room for 2026-10-12 to 2026-10-15
Result:   Available=true, Capacity=10, Remaining=7
```

### 4. Capacity-Based (Restaurant Tables)
```
Capacity: 20 covers (4 tables × 5 seats)
Existing: 8 covers reserved for 19:00-21:00
Query:    6 covers for 19:00-21:00
Result:   Available=true, Capacity=20, Remaining=12
```

## Error Handling

```go
type ReservationExistsError struct {
    ReservationID string
    Message       string
}

func (e *ReservationExistsError) Error() string
```

Returned when a conflict is detected during reservation creation (not during availability check).

## Status

**DONE** — Contract implemented in `internal/domain/availability.go`

## Files

- `internal/domain/availability.go` — AvailabilityQuery, AvailabilityResult, AvailabilityProvider, ReservationExistsError
- `internal/domain/listing_availability.go` — ListingAvailability (per-listing availability record)