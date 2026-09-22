# Sprint 14 — Provider Architecture

## Overview

This document describes the architecture for the Provider Orchestration Layer introduced in Sprint 14.

## Goals

- Allow Trezutil to integrate external providers without coupling Commerce Core to provider-specific implementations.
- Support any vertical (hotels, travel, delivery, ticketing, payment, identity, mapping, etc.) through generic capability contracts.
- Ensure provider-specific concerns remain outside the Commerce Core.
- Provide a testable, dependency-injected architecture.

## Architecture

```
Web / Mobile / WhatsApp / Partners
                ↓
          REST API / Webhooks
                ↓
       Commerce Application Layer
                ↓
      Domain / Commerce Core
                ↓
   Internal Provider Orchestration
                ↓
 External Providers / Integrations
```

### Key Principles

1. **No provider-specific imports in Commerce Core.** The core domain and application layers depend only on generic abstractions (`domain.Capability`, `domain.ProviderAdapter`, `domain.AvailabilityProvider`).

2. **Adapters implement domain contracts.** External provider implementations adapt into existing domain interfaces. For example, an external hotel availability provider implements `domain.AvailabilityProviderAdapter`, which is registered as a `domain.ProviderAdapter`.

3. **Orchestration is explicit.** The `ProviderOrchestrator` selects a provider, calls the adapter, normalizes results, handles retries, and records execution metadata.

4. **Dependency injection only.** No global mutable state. The registry, orchestrator, and repositories are passed explicitly.

## Components

### Provider (domain)

`domain.IntegrationProvider` represents an external integration:

| Field             | Type                 | Description                           |
|-------------------|----------------------|---------------------------------------|
| `ID`              | string               | Unique identifier                     |
| `OrganizationID`  | string               | Tenant scope                          |
| `Code`            | string               | Human-friendly code                   |
| `Name`            | string               | Display name                          |
| `Type`            | string               | Provider category (e.g., "hotel")     |
| `Status`          | `ProviderStatus`     | ACTIVE, INACTIVE, SUSPENDED, DEGRADED |
| `Capabilities`    | `[]Capability`       | Capabilities exposed by this provider |
| `Configuration`   | `ProviderConfig`     | Endpoints and settings (no secrets)   |
| `SecretRef`       | string               | Reference to secret store entry       |
| `Priority`        | int                  | Selection priority (higher wins)      |
| `HealthStatus`    | `ProviderHealthStatus` | UNKNOWN, HEALTHY, DEGRADED, UNAVAILABLE |
| `CreatedAt`       | int64                | Unix timestamp                        |
| `UpdatedAt`       | int64                | Unix timestamp                        |
| `Version`         | int64                | Optimistic locking version            |

### Capability (domain)

Capabilities are generic operation types:

```
AVAILABILITY
RESERVATION
BOOKING
CATALOG
DELIVERY
TRACKING
LOCATION
NOTIFICATION
IDENTITY_VERIFICATION
PAYMENT
FULFILLMENT
```

A provider may expose multiple capabilities. Capabilities never reference provider names — they are generic verbs.

### Provider Adapter (domain)

Provider adapters implement one or more adapter interfaces:

```go
type ProviderAdapter interface {
    ProviderID() string
    Capabilities() []Capability
    Health(ctx context.Context) ProviderHealthStatus
}

type AvailabilityProviderAdapter interface {
    ProviderAdapter
    CheckAvailability(ctx context.Context, query AvailabilityQuery) (AvailabilityResult, error)
}
```

External providers adapt into these interfaces. The existing `domain.AvailabilityProvider` interface remains the source of truth for the application layer. The orchestrator's `OrchestratedAvailabilityProvider` implements `AvailabilityProvider` and delegates to the selected adapter.

### Provider Registry (application/provider)

The `ProviderRegistry` is an in-memory, dependency-injected registry:

- `Register(provider, adapter)` — registers a provider with its adapter
- `Unregister(orgID, providerID)` — removes a provider
- `FindByID(ctx, orgID, providerID)` — looks up a provider
- `FindByCapability(ctx, orgID, capability)` — finds usable providers supporting a capability, ordered by priority
- `FindAll(ctx, orgID, offset, limit)` — lists all providers for a tenant

The registry filters out non-usable providers (inactive, suspended, health=unavailable) and enforces tenant isolation. An in-memory implementation is provided; the abstraction supports future PostgreSQL backing via `domain.ProviderRepository`.

### Provider Orchestrator (application/provider)

The `ProviderOrchestrator` orchestrates provider calls:

1. Receives a capability, operation, tenant context, and a `perform` callback
2. Queries the registry for eligible providers
3. Selects providers by priority
4. Executes the `perform` callback with timeout and context
5. Classifies errors as RETRYABLE, NON_RETRYABLE, or UNKNOWN
6. Retries with exponential backoff (up to `MaxAttempts`)
7. Logs execution metadata (provider, capability, duration, success/failure)
8. Returns a normalized `ExecuteResult`

Retry policy defaults: 3 attempts, 100ms base delay, 10s max delay. The policy is injectable.

### Provider Repository (infrastructure/postgres)

PostgreSQL implementation following existing project conventions:

- `ProviderRepository` — CRUD for providers, queries by organization and capability
- `ProviderExecutionRepository` — records provider execution metadata
- Uses `*sql.DB` + `context.Context` pattern consistent with all other repos
- Uses `gen_random_uuid()` for IDs
- Uses `pq.Array` for capabilities array
- Uses `CHECK` constraints for validation
- Version column for optimistic locking

### Provider Use Cases (application/provider)

`ProviderUseCases` handles provider lifecycle:

- `CreateProvider` — validates, checks authorization, creates provider as INACTIVE
- `GetProvider` — tenant-isolated lookup
- `ListProviders` — paginated tenant-scoped listing
- `EnableProvider` / `DisableProvider` — lifecycle transitions
- `GetProviderCapabilities` — returns provider's capability list
