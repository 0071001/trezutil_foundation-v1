# Trezutil

**Trezutil = Très utile**

Trezutil is a conversation-first **Commerce OS** for the DRC and progressively Africa.

## Core idea

WhatsApp is the primary interface for customers and sellers, while the backend remains a channel-independent commerce platform.

Customers can search, compare, buy, reserve, pay and track. Sellers can create listings, manage availability, process orders, run marketing and request reports through WhatsApp. A web console is reserved for administration and advanced operations.

## Architecture principles

1. API and domain contracts before business code.
2. Go modular backend with clean boundaries.
3. PostgreSQL is the transactional source of truth.
4. Redis is used for hot state/cache/rate limits.
5. OpenSearch is a derived search index.
6. Kafka carries domain/integration events.
7. S3/MinIO stores media.
8. n8n is an automation/integration engine, not the commerce core.
9. AI is a tool-using orchestrator behind a provider-neutral gateway.
10. Payment and channel providers are adapters.
11. Sensitive actions are authorized, audited and idempotent.
12. Start modular; extract services only when justified.

## Repository map

- `cmd/` application entrypoints
- `internal/` core application packages
- `packages/` reusable shared packages
- `api/openapi/` external REST contracts
- `api/proto/` internal gRPC contracts
- `docs/` product, architecture, domain, API, event and security specifications
- `deployments/` local/container orchestration
- `infrastructure/` service-specific local infrastructure configuration
- `migrations/` database migrations
- `test/` integration, E2E and fixtures
- `web/` customer-facing SPA (static HTML/CSS/JS, no build step)
- `.github/workflows/` CI

## Local development

Requirements: Docker, Docker Compose, Go 1.23+.

```bash
make up
make test
make lint
```

See `docs/PRODUCT_SPEC.md` and `docs/ARCHITECTURE.md` before implementing new business logic.

## Build rule

Do not add a production business feature unless its domain invariants, API contract and event impacts are documented.
