# Sprint 12 — WhatsApp Flow

## Overview

All reservation capabilities are accessible through the existing WhatsApp architecture without adding reservation-specific logic to the WhatsApp adapter.

## Flow

```
Customer (WhatsApp)
       │
       ▼
┌────────────────────────────────────────┐
│  WhatsApp Webhook                      │
│  POST /webhooks/whatsapp               │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  WhatsApp Service                      │
│  ProcessWebhook()                      │
│  - Verify signature                    │
│  - Resolve tenant (WABA ID)            │
│  - Find/create user by phone           │
│  - Deduplicate via idempotency key     │
│  - Load/create Conversation            │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  Intent Resolution                     │
│  resolveActionType(text)               │
│  - "reserve" / "book" / "check"        │
│  - Returns CommerceActionType          │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  CommerceActionRuntime.Execute()       │
│  - Identity protection                 │
│  - Tenant resolution (server-side)     │
│  - Validation                          │
│  - Idempotency                         │
│  - Confirmation                        │
│  - Usage check/reserve                 │
│  - Handler dispatch                    │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  Reservation Handlers                  │
│  - CheckAvailabilityHandler            │
│  - CreateReservationHandler            │
│  - GetReservationHandler               │
│  - CancelReservationHandler            │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  Reservation Use Cases                 │
│  - AvailabilityUseCases.CheckAvailability│
│  - ReservationUseCases.CreateReservation│
│  - ReservationUseCases.GetReservation   │
│  - ReservationUseCases.CancelReservation│
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  Domain + PostgreSQL                   │
│  - AvailabilityProvider                │
│  - Reservation Aggregate               │
│  - ReservationRepository               │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│  Response Conversion                   │
│  convertExecuteResult()                │
│  - Maps ActionResult to ConversationResponse│
│  - Handles confirmation prompts        │
└────────────────┬───────────────────────┘
                 │
                 ▼
       Customer (WhatsApp)
```

## Intent Mapping

```go
func (s *Service) resolveActionType(text string) domain.CommerceActionType {
    lower := strings.ToLower(text)
    
    // Availability check
    if strings.Contains(lower, "available") ||
       strings.Contains(lower, "check") ||
       strings.Contains(lower, "disponible") {
        return domain.ActionCheckAvailability
    }
    
    // Create reservation
    if strings.Contains(lower, "reserve") ||
       strings.Contains(lower, "book") ||
       strings.Contains(lower, "reserver") ||
       strings.Contains(lower, "booker") {
        return domain.ActionCreateReservation
    }
    
    // Get reservation
    if strings.Contains(lower, "my reservation") ||
       strings.Contains(lower, "ma réservation") ||
       strings.Contains(lower, "view booking") {
        return domain.ActionGetReservation
    }
    
    // Cancel reservation
    if strings.Contains(lower, "cancel") ||
       strings.Contains(lower, "annuler") {
        return domain.ActionCancelReservation
    }
    
    return ""
}
```

## Example Conversations

### Check Availability
```
Customer: "Is the suite available Oct 15-18?"
Bot:      "Checking availability..."
          "✅ Available! 3 suites remaining for Oct 15-18."
          "Want to reserve?"
```

### Create Reservation
```
Customer: "Reserve the suite for Oct 15-18 for 2 people"
Bot:      "Creating reservation..."
          "✅ Reservation confirmed! ID: res-abc123"
          "Check-in: Oct 15, 3PM | Check-out: Oct 18, 11AM"
          "Party: 2 guests"
```

### Cancel Reservation
```
Customer: "Cancel my reservation res-abc123"
Bot:      "Cancelling reservation..."
          "✅ Reservation cancelled"
```

### Confirmation Flow (if RequiresConfirmation)
```
Customer: "Book the table for 7PM"
Bot:      "This will reserve a table for 4 at 7PM. Confirm?"
          [Confirm] [Cancel]
Customer: [taps Confirm]
Bot:      "✅ Reservation confirmed! ID: res-xyz789"
```

## Data Extraction

Parameters extracted from message `Content.Data` (structured data from WhatsApp interactive messages):

```json
{
  "listing_id": "uuid",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2
}
```

For natural language, future AI orchestrator will extract entities.

## Error Handling

| Error | WhatsApp Response |
|-------|-------------------|
| `MISSING_USER_ID` | "Please start a conversation first" |
| `FORBIDDEN` | "You don't have permission" |
| `UNAVAILABLE` | "Sorry, not available for those dates" |
| `CONFLICT` | "Just booked by someone else. Try other dates?" |
| `INSUFFICIENT_CREDITS` | "Insufficient credits. Top up to continue." |

## Registration

Handlers registered in `NewService()`:

```go
if reservationUseCases != nil {
    runtime.RegisterHandler(commerce.NewCreateReservationHandler(reservationUseCases))
    runtime.RegisterHandler(commerce.NewGetReservationHandler(reservationUseCases))
    runtime.RegisterHandler(commerce.NewCancelReservationHandler(reservationUseCases))
}
if availabilityUseCases != nil {
    runtime.RegisterHandler(commerce.NewCheckAvailabilityHandler(availabilityUseCases))
}
```

## Status

**DONE** — Implemented in `internal/application/whatsapp/service.go`

## Files

- `internal/application/whatsapp/service.go` — Intent resolution, handler registration, response conversion
- `internal/application/commerce/reservation_handlers.go` — WhatsApp-compatible handlers