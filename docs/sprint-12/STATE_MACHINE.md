# Sprint 12 — Reservation State Machine

## States

```go
type ReservationStatus string

const (
    ReservationStatusPending   ReservationStatus = "pending"
    ReservationStatusConfirmed ReservationStatus = "confirmed"
    ReservationStatusCompleted ReservationStatus = "completed"
    ReservationStatusCancelled ReservationStatus = "cancelled"
)
```

## State Diagram

```
                    ┌─────────────┐
                    │   PENDING   │
                    └──────┬──────┘
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
       ┌───────────┐ ┌──────────┐ ┌──────────┐
       │ CONFIRMED │ │ CANCELLED│ │  (auto)  │
       └─────┬─────┘ └──────────┘ └──────────┘
             │
      ┌──────┴──────┐
      ▼             ▼
┌──────────┐  ┌──────────┐
│COMPLETED │  │CANCELLED │
└──────────┘  └──────────┘
```

## Transitions

| Current State | Next State | Method | Conditions |
|---------------|------------|--------|------------|
| PENDING | CONFIRMED | `Confirm(now)` | Always allowed |
| PENDING | CANCELLED | `Cancel(now)` | Always allowed |
| CONFIRMED | COMPLETED | `Complete(now)` | Always allowed |
| CONFIRMED | CANCELLED | `Cancel(now)` | Always allowed |
| COMPLETED | — | — | Terminal — no transitions |
| CANCELLED | — | — | Terminal — no transitions |

## Implementation

All transitions are controlled by domain methods on the `Reservation` aggregate:

```go
func (r *Reservation) Confirm(now int64) error
func (r *Reservation) Complete(now int64) error
func (r *Reservation) Cancel(now int64) error
func (r *Reservation) ValidateTransition(newStatus ReservationStatus) error
func (r *Reservation) IsTerminal() bool
```

**Never** set `reservation.Status = ...` directly from handlers or use cases.

## Confirmation Policy

The `ConfirmationPolicy` determines initial state after creation:

```go
type ConfirmationPolicy struct {
    RequiresConfirmation bool // If true, stays PENDING until explicit confirm
    AutoConfirm          bool // If true, auto-confirms immediately
}
```

| Policy | Initial State | Use Case |
|--------|---------------|----------|
| `RequiresConfirmation=true, AutoConfirm=false` | PENDING | Payment required later |
| `RequiresConfirmation=false, AutoConfirm=true` | CONFIRMED | Free booking, instant |
| `RequiresConfirmation=true, AutoConfirm=true` | CONFIRMED | Default for most cases |

## Usage in CreateReservation

```go
now := time.Now().Unix()
reservation := domain.NewReservation(tenantID, actorID, listingID, startAt, endAt, quantity, partySize, now)

if !confirmationPolicy.RequiresConfirmation || confirmationPolicy.AutoConfirm {
    if err := reservation.Confirm(now); err != nil {
        return nil, fmt.Errorf("failed to confirm reservation: %w", err)
    }
}
```

## Authorization by State

| Action | PENDING | CONFIRMED | COMPLETED | CANCELLED |
|--------|---------|-----------|-----------|-----------|
| Customer: View | ✅ | ✅ | ✅ | ✅ |
| Customer: Cancel | ✅ | ✅ | ❌ | ❌ |
| Seller: View | ✅ | ✅ | ✅ | ✅ |
| Seller: Cancel | ✅ | ✅ | ❌ | ❌ |
| Admin: View | ✅ | ✅ | ✅ | ✅ |
| Admin: Cancel | ✅ | ✅ | ✅* | ✅* |
| System: Complete | ❌ | ✅ | — | — |

*Admin can force-cancel completed with audit trail (future enhancement)

## Concurrency Control

- **Optimistic Locking**: `Version` field incremented on every state change
- **Database Exclusion Constraint**: Prevents overlapping reservations for same listing
- **Idempotency**: Prevents duplicate creation via `Idempotency-Key`

## Status

**DONE** — Implemented in `internal/domain/reservation.go`

## Files

- `internal/domain/reservation.go` — State machine implementation