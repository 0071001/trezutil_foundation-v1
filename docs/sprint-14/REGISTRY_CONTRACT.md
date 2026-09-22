# Sprint 14 — Registry Contract

## Overview

The `ProviderRegistry` is an in-memory, dependency-injected registry for provider adapters. It supports tenant-isolated registration and lookup.

## API

```go
type ProviderRegistry struct {
    mu        sync.RWMutex
    providers map[string]*RegistryEntry // orgID:providerID → entry
}
```

### Register

```go
func (r *ProviderRegistry) Register(provider *domain.IntegrationProvider, adapter domain.ProviderAdapter)
```

- Stores the provider and adapter.
- If a provider with the same `OrganizationID:ProviderID` already exists, it is replaced.

### Unregister

```go
func (r *ProviderRegistry) Unregister(organizationID, providerID string)
```

- Removes the provider from the registry.
- No-op if not found.

### FindByID

```go
func (r *ProviderRegistry) FindByID(ctx context.Context, organizationID, providerID string) (*domain.IntegrationProvider, domain.ProviderAdapter, bool)
```

- Returns provider, adapter, and whether it was found.
- Always returns false for a non-existent provider.

### FindByCapability

```go
func (r *ProviderRegistry) FindByCapability(ctx context.Context, organizationID string, capability domain.Capability) ([]*domain.IntegrationProvider, []domain.ProviderAdapter, error)
```

- Returns all usable providers supporting the given capability.
- Filters to `IsUsable()` providers (active + health != unavailable).
- Ordered by `Priority` (descending).
- Tenant isolation: only returns providers for `organizationID`.

### FindAll

```go
func (r *ProviderRegistry) FindAll(ctx context.Context, organizationID string, offset, limit int) ([]*domain.IntegrationProvider, error)
```

- Paginated listing of all providers for the tenant.

## RegistryEntry

```go
type RegistryEntry struct {
    Provider *domain.IntegrationProvider
    Adapter  domain.ProviderAdapter
}
```

## Concurrency

The registry uses a `sync.RWMutex`:
- Read operations (`FindByID`, `FindByCapability`, `FindAll`) acquire a read lock.
- Write operations (`Register`, `Unregister`) acquire a write lock.

## Design Notes

- The in-memory registry is suitable for the default orchestrator deployment.
- A PostgreSQL-backed registry can be provided via `domain.ProviderRepository` for distributed scenarios.
- The registry does not perform health checks — it trusts the `HealthStatus` set on the provider entity.
- Provider health is expected to be updated by an external monitoring component.
