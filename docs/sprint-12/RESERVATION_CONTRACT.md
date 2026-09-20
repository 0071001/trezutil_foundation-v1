# Sprint 12 — Reservation Domain Contract

## Reservation Aggregate

### Identity
- **ID**: UUID (generated server-side)
- **OrganizationID**: UUID — Tenant/organization that owns the listing
- **CustomerID**: UUID — User who made the reservation
- **ListingID**: UUID — Reference to the Listing being reserved

### Temporal
- **StartAt**: Unix timestamp (UTC) — Check-in / Start time
- **EndAt**: Unix timestamp (UTC) — Check-out / End time
- **Constraint**: `EndAt > StartAt` enforced at domain and database level

### Quantity
- **Quantity**: Integer (> 0) — Number of units reserved (e.g., rooms, tickets)
- **PartySize**: Integer (≥ 0) — Number of people (e.g., restaurant covers, appointment attendees)

### Status
```
PENDING → CONFIRMED → COMPLETED
    ↓           ↓
  CANCELLED   CANCELLED
```

- **PENDING**: Initial state, awaiting confirmation (or auto-confirmed)
- **CONFIRMED**: Reservation is active and held
- **COMPLETED**: Service consumed / stay finished
- **CANCELLED**: Reservation cancelled (terminal)

### Pricing Snapshot (Optional)
```go
type PricingSnapshot struct {
    Items      []OrderItem
    Subtotal   Money
    Tax        Money
    Total      Money
    Currency   string
    CreatedAt  int64
}
```
Captured at reservation time if listing has pricing. Not a payment record.

### Versioning
- **Version**: Integer (≥ 1) — Optimistic locking for concurrent modifications
- **CreatedAt**: Unix timestamp
- **UpdatedAt**: Unix timestamp

## Domain Methods

```go
func NewReservation(organizationID, customerID, listingID string, startAt, endAt int64, quantity, partySize int, now int64) *Reservation

func (r *Reservation) Confirm(now int64) error
func (r *Reservation) Complete(now int64) error
func (r *Reservation) Cancel(now int64) error
func (r *Reservation) ValidateTransition(newStatus ReservationStatus) error
func (r *Reservation) IsTerminal() bool
func (r *Reservation) Overlaps(startAt, endAt int64) bool
```

## Valid Transitions

| From | To | Allowed |
|------|-----|---------|
| PENDING | CONFIRMED | ✅ |
| PENDING | CANCELLED | ✅ |
| CONFIRMED | COMPLETED | ✅ |
| CONFIRMED | CANCELLED | ✅ |
| COMPLETED | * | ❌ (terminal) |
| CANCELLED | * | ❌ (terminal) |

## Validation Rules

1. `StartAt` and `EndAt` required, `EndAt > StartAt`
2. `Quantity > 0`
3. `PartySize >= 0`
4. `Status` must be one of the defined constants
5. `Version >= 1`
6. OrganizationID, CustomerID, ListingID must be non-empty UUIDs

## Status

**DONE** — Contract implemented in `internal/domain/reservation.go`

## Files

- `internal/domain/reservation.go` — Reservation aggregate
- `internal/domain/availability.go` — AvailabilityQuery, AvailabilityResult, AvailabilityProvider