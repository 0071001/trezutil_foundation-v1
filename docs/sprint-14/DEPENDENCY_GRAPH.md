# Sprint 14 — Dependency Graph

## Overview

The Provider Orchestration Layer has minimal dependencies. This document outlines the dependency graph.

## Packages

```
internal/domain/provider.go
├── no external deps (stdlib only)
│
internal/domain/provider_adapter.go
├── no external deps (stdlib, domain)
│
internal/domain/interfaces.go (updated)
├── no external deps (stdlib, domain)
│
internal/application/provider/registry.go
├── internal/domain
│
internal/application/provider/orchestrator.go
├── internal/domain
│
internal/application/provider/availability_adapter.go
├── internal/domain
│
internal/application/provider/use_cases.go
├── internal/domain
├── internal/application/provider (registry, orchestrator)
│
infrastructure/postgres/provider.go
├── internal/domain
├── stdlib (database/sql, context, time)
├── third-party: lib/pq (for pq.Array)
│
infrastructure/postgres/provider_execution.go
├── internal/domain
├── third-party: lib/pq
│
internal/api/provider_handlers.go
├── internal/application/provider
├── internal/api (existing)
│
internal/api/handlers.go (updated)
├── internal/application/provider
│
internal/api/web.go (updated)
├── internal/api
│
internal/api/provider_handlers_test.go
├── internal/api (existing test infra)
├── internal/application/provider
│
internal/application/provider/*_test.go
├── internal/domain
├── internal/application/provider (mocks)
```

## Layering

```
[ external providers ]
         ↑ (adapters implement domain interfaces)
[ infrastructure/postgres ]
         ↑ (repositories implement domain interfaces)
[ internal/application/provider ]
         ↑ (use cases use registry + orchestrator + repositories)
[ internal/domain ]
         ↑ (domain entity, interfaces, adapter contracts)
[ internal/api ]
         ↑ (handlers call use cases)
```

## Key Constraints

1. No import cycles.
2. `internal/domain` has no knowledge of `internal/application` or `infrastructure`.
3. `internal/application/provider` depends only on `internal/domain` — no infrastructure imports.
4. `infrastructure/postgres/provider.go` imports `internal/domain` and `lib/pq` only.
5. `internal/api` imports both `internal/application` and `internal/domain`.

## Third-Party Dependencies

| Package         | Used By              | Purpose                    |
|-----------------|----------------------|----------------------------|
| `lib/pq`        | infrastructure/postgres | PostgreSQL driver, `pq.Array` |
| `lib/pq/uuid`   | (not used)           | `gen_random_uuid()` handles ID generation in DB |

No new third-party packages were introduced for the application or domain layers.
