# Sprint 14 — Provider Integration Guide

## Overview

This guide explains how to add a new external provider to the orchestration layer.

## Prerequisites

- The provider is accessible via HTTP/gRPC or another transport.
- The provider has been registered in `domain.IntegrationProvider` with its `Code`.
- The provider's secrets are managed via `SecretRef`.

## Step 1: Implement the Adapter

Create a new file implementing one or more adapter interfaces:

```go
// Example: hotel_booking_adapter.go
type HotelBookingAvailabilityAdapter struct {
    endpoint string
    apiKey   string // resolved from secret management
}

func (a *HotelBookingAvailabilityAdapter) ProviderID() string { return "hotel-booking" }
func (a *HotelBookingAvailabilityAdapter) Capabilities() []domain.Capability {
    return []domain.Capability{domain.CapabilityAvailability}
}
func (a *HotelBookingAvailabilityAdapter) Health(ctx context.Context) domain.ProviderHealthStatus {
    // ping the provider
    return domain.ProviderHealthy
}
func (a *HotelBookingAvailabilityAdapter) CheckAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
    // make HTTP/gRPC call
    // normalize response to domain.AvailabilityResult
    return result, nil
}
```

The adapter must implement the appropriate `ProviderAdapter` interface (or embed one).

## Step 2: Register the Provider

Register the adapter in the registry during provider setup:

```go
provider := &domain.IntegrationProvider{
    ID:              "uuid",
    OrganizationID:  orgID,
    Code:            "hotel-booking",
    Name:            "Hotel Booking Provider",
    Type:            "hotel",
    Status:          domain.ProviderActive,
    Capabilities:    []domain.Capability{domain.CapabilityAvailability},
    HealthStatus:    domain.ProviderHealthy,
    Priority:        10,
    Configuration:   domain.ProviderConfig{...},
    SecretRef:       "hotel-booking-api-key",
}

registry.Register(provider, &HotelBookingAvailabilityAdapter{...})
```

## Step 3: Wire into Use Cases

The orchestrator automatically discovers registered adapters. When `AvailabilityUseCase` is called, it:

1. Calls `OrchestratedAvailabilityProvider.CheckAvailability`
2. The orchestrator queries the registry for AVAILABILITY providers
3. Selects the highest-priority usable provider
4. Delegates to the adapter
5. Normalizes the result

## Step 4: Test

### Unit Tests

Use the fake provider adapter in tests:

```go
type FakeAvailabilityAdapter struct {
    result domain.AvailabilityResult
    err    error
}
```

### Integration Tests

Add integration tests that exercise the real adapter against a mock HTTP server (using `httptest`).

## Design Constraints

1. **No provider-specific code in Commerce Core.** If a provider returns data that maps to a domain concept, normalize it in the adapter.
2. **Errors must be classified.** Return `domain.ProviderErrorCode` values, not raw exceptions.
3. **Respect timeouts.** Use the context's deadline; do not make calls longer than the configured timeout.
4. **No secrets in config.** All credentials come from the `SecretRef`, resolved at runtime by the adapter.
