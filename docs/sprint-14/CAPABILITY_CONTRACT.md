# Sprint 14 — Capability Contract

## Overview

Provider integration is capability-driven. Providers expose one or more capabilities. The orchestrator selects a provider based on the required capability.

## Capability Types

| Capability           | Description                                  | Domain Adapter Interface       |
|----------------------|----------------------------------------------|--------------------------------|
| `AVAILABILITY`       | Check if a listing/resource is available     | `AvailabilityProviderAdapter`  |
| `RESERVATION`        | Create/cancel reservations externally        | `ReservationProviderAdapter`   |
| `BOOKING`            | Create external bookings                     | `BookingProviderAdapter` (ext.)|
| `CATALOG`            | Sync/supply external catalog data            | Future                        |
| `DELIVERY`           | Initiate delivery shipments                  | `DeliveryProviderAdapter` (ext.)|
| `TRACKING`           | Track shipments/parcels                      | `TrackingProviderAdapter`      |
| `LOCATION`           | Resolve/geocode locations                    | Future                         |
| `NOTIFICATION`       | Send notifications via external channel      | Future                         |
| `IDENTITY_VERIFICATION` | Verify user identity externally          | Future                         |
| `PAYMENT`            | Process payments externally                  | Future (not in Sprint 14)       |
| `FULFILLMENT`        | External fulfillment operations              | Future                         |

## Adapter Interfaces

### ProviderAdapter (base)

```go
type ProviderAdapter interface {
    ProviderID() string
    Capabilities() []Capability
    Health(ctx context.Context) ProviderHealthStatus
}
```

### AvailabilityProviderAdapter

```go
type AvailabilityProviderAdapter interface {
    ProviderAdapter
    CheckAvailability(ctx context.Context, query AvailabilityQuery) (AvailabilityResult, error)
}
```

Implements `domain.AvailabilityProvider` semantics. External providers adapt into this interface, which the orchestrator wraps via `OrchestratedAvailabilityProvider`.

### ReservationProviderAdapter

```go
type ReservationProviderAdapter interface {
    ProviderAdapter
    CreateExternalReservation(ctx context.Context, req ProviderReservationRequest) (ProviderResult, error)
    CancelExternalReservation(ctx context.Context, externalRef string) (ProviderResult, error)
}
```

### TrackingProviderAdapter

```go
type TrackingProviderAdapter interface {
    ProviderAdapter
    Track(ctx context.Context, trackingRef string) (ProviderResult, error)
}
```

## ProviderResult

```type
ProviderResult
    Success: bool
    Data: map[string]interface{}
    ExternalReference: string
    ErrorCode: ProviderErrorCode
    ErrorMessage: string
    ProviderDiagnostics: map[string]interface{}
```

## Design Rules

1. **Do not duplicate existing interfaces.** `domain.AvailabilityProvider` remains the source of truth. Provider adapters adapt external providers into this interface.
2. **Capabilities are verbs, not provider names.** Never hardcode "Booking.com" or "Amadeus" into capability constants.
3. **A provider may expose multiple capabilities.** E.g., Provider A: AVAILABILITY + RESERVATION + TRACKING.
4. **Adapters are transport-agnostic.** HTTP, gRPC, file-based, or database-backed — the adapter interface does not constrain the transport.
