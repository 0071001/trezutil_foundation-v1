# Sprint 14 — Migration Notes

## Overview

Sprint 14 introduces the Provider Orchestration Layer. This document covers migration considerations for existing code and services.

## For Commerce Core Maintainers

### What Changed

- No changes to existing domain entities or use cases.
- New `provider` package added under `internal/application/provider/`.
- New `provider` directory created under `infrastructure/postgres/`.
- New API handlers added — no existing routes modified.
- New migration `000011_providers_schema.sql` added — runs only on databases that don't already have the tables.

### What Did NOT Change

- `domain.AvailabilityProvider` interface is unchanged. The `OrchestratedAvailabilityProvider` implements this interface and delegates to adapters.
- Existing availability, reservation, and booking use cases remain untouched.
- Existing API routes are unchanged.
- No provider-specific imports added to any core package.

## For Service Owners

Services that need to expose external provider availability should:

1. Implement `domain.AvailabilityProviderAdapter` (or the relevant adapter interface).
2. Register the adapter with the `ProviderRegistry`.
3. The orchestrator handles selection, retries, and normalization automatically.

## For API Consumers

New endpoints are available at:

```
POST /api/v1/providers           — Create provider
GET /api/v1/providers            — List providers
GET /api/v1/providers/{id}       — Get provider
POST /api/v1/providers/{id}/enable   — Enable provider
POST /api/v1/providers/{id}/disable  — Disable provider
GET /api/v1/providers/{id}/capabilities — Get capabilities
```

Providers are created as `INACTIVE` and must be explicitly enabled.

## For Testing

Run the new tests alongside existing tests:

```bash
go test ./internal/application/provider/... -v
go test ./internal/api/... -run TestProvider -v
```

## Rollback

If you need to roll back Sprint 14 features:

1. Remove API routes (see `internal/api/web.go`).
2. Remove handler registrations (see `internal/api/handlers.go`).
3. Delete migration `000011_providers_schema.sql`.
4. The new tables (`providers`, `provider_executions`) can be dropped — they contain no data referenced by existing features.

## Compatibility

- Database schema is additive — existing tables are not modified.
- API endpoints are additive — existing routes are not changed.
- Domain models are additive — existing entities are not modified.
- All new code is in new packages — no import cycles risk.
