# Sprint 14 — Backlog & Priorities

## Sprint Scope

Sprint 14 delivers the Provider Orchestration Layer: a generic abstraction for integrating external providers across any vertical (hotels, travel, delivery, ticketing, payment, identity, mapping).

## Priority Order

1. **DONE** — Provider data model and domain entity (`domain.IntegrationProvider`)
2. **DONE** — Provider adapter interfaces (`ProviderAdapter`, `AvailabilityProviderAdapter`, etc.)
3. **DONE** — Provider registry (in-memory, tenant-isolated, priority-ordered)
4. **DONE** — Provider orchestrator (retry, timeout, idempotency, execution logging)
5. **DONE** — Provider repository interface + PostgreSQL implementation
6. **DONE** — Provider execution repository interface + PostgreSQL implementation
7. **DONE** — Provider use cases (CRUD, enable/disable, capabilities list)
8. **DONE** — API handlers + routes
9. **DONE** — Migration (`000011_providers_schema.sql`)
10. **DONE** — Unit tests (domain, registry, orchestrator, use cases, API handlers)
11. **DONE** — Documentation (Sprint 14 docs A–O)
12. **DONE** — OpenAPI spec
13. **BLOCKED** — PostgreSQL integration tests (no local PostgreSQL instance)

## Future Work (Post-Sprint 14)

### High Priority
- Transaction-aware repositories — refactor all repos to accept `*sql.Tx` via DBTX pattern (required for atomic ConfirmOrder + ProviderExecution write)
- Health check monitoring — external component that periodically polls provider health

### Medium Priority
- Expand adapter interfaces:
  - `BookingProviderAdapter` (create external bookings)
  - `DeliveryProviderAdapter` (initiate delivery shipments)
  - `NotificationProviderAdapter`
- Distributed registry backend (Redis or PostgreSQL-backed for multi-instance deployments)
- Provider-specific error mapping (adapter converts external error → `ProviderErrorCode`)

### Low Priority
- Provider config validation schemas (JSON Schema per provider type)
- Provider metrics aggregation dashboard
- CLI command for provider setup and testing
