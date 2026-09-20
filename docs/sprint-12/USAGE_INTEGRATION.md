# Sprint 12 — Usage Integration

## Overview

Reservation actions integrate with the existing Trezutil Usage & Credits Engine (Sprint 11). Actions are metered according to `UsagePolicy` resolved by `UsageService`.

## Flow

```
CreateReservation Request
       ↓
CommerceActionRuntime.Execute
       ↓
1. Validate action structure
2. Validate parameters
3. Check idempotency
4. Check confirmation policy
5. CHECK USAGE POLICY
       ↓
   UsageService.ResolvePolicy(actionType, channel, actorType)
       ↓
   if policy.IsMetered():
       UsageService.Check(actorID, policy.Cost)
       UsageService.Reserve(actorID, actionType, policy.Cost)
       ↓
6. Dispatch to CreateReservationHandler
       ↓
7. ReservationUseCases.CreateReservation
       ↓
8. On SUCCESS: UsageService.Consume(reservationID)
   On FAILURE:  UsageService.Release(reservationID)
```

## Action Types & Metering

| Action Type | Category | Metered | Cost | Notes |
|-------------|----------|---------|------|-------|
| `CHECK_AVAILABILITY` | READ | ❌ No | 0 | Free — read operation |
| `CREATE_RESERVATION` | WRITE | ✅ Yes | Configurable | Consumes credit on success |
| `GET_RESERVATION` | READ | ❌ No | 0 | Free — read operation |
| `CANCEL_RESERVATION` | WRITE | ❌ No | 0 | Free — but could be metered |
| `MODIFY_RESERVATION` | WRITE | ✅ Yes | Configurable | Not implemented |

## UsagePolicy Resolution

```go
policy, err := usageService.ResolvePolicy(ctx, action.ActionType, domain.ChannelOther, actor.Type)
if err == nil && policy.IsMetered() {
    // Check and reserve credits
}
```

- **Channel**: `domain.ChannelWeb`, `domain.ChannelWhatsApp`, `domain.ChannelOther`
- **ActorType**: `domain.ActorTypeCustomer`, `domain.ActorTypeSeller`, `domain.ActorTypeSystem`

## Credit Reservation Lifecycle

```
Reserve → [Handler Execute] → Consume (success) / Release (failure)
```

- Reservation held in memory during handler execution (`CommerceActionRuntime.reservations` map)
- Automatic cleanup on panic/error via deferred release

## Configuration

Default policies (configured in Sprint 11):

```go
// Example: CREATE_RESERVATION costs 1 credit for customers on Web
UsagePolicy{
    ActionType: "CREATE_RESERVATION",
    Channel:    "web",
    ActorType:  "customer",
    Cost:       1,
    Metered:    true,
}

// Free for sellers managing their own listings
UsagePolicy{
    ActionType: "CREATE_RESERVATION",
    Channel:    "web",
    ActorType:  "seller",
    Cost:       0,
    Metered:    false,
}
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Insufficient credits | Returns `ActionResultFailed` with `INSUFFICIENT_CREDITS` |
| UsageService unavailable | Logs warning, allows action (fail-open) |
| Reserve fails | Returns `ActionResultFailed` with `USAGE_RESERVE_FAILED` |
| Consume fails | Logs warning, non-blocking |

## Idempotency & Usage

Same idempotency key = same result = same credit consumption (only once).

```
Request A (idem-key-1) → Reserve 1 credit → Create Reservation → Consume 1 credit
Retry Request A (idem-key-1) → Returns cached result → NO additional credit
```

## Status

**DONE** — Integrated via existing `CommerceActionRuntime` in `internal/application/commerce/runtime.go`

## Files

- `internal/application/commerce/runtime.go` — Usage integration in Execute()
- `internal/application/commerce/reservation_handlers.go` — Handlers registered with runtime
- `internal/application/whatsapp/service.go` — WhatsApp handlers registered