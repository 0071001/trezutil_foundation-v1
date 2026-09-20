# Sprint 4 — Search & Discovery Architecture

## Documents

| Document | Purpose |
|----------|---------|
| `SEARCH_ARCHITECTURE.md` | Architecture overview, service boundaries, technology decisions |
| `DISCOVERY_QUERY.md` | DiscoveryQuery domain model |
| `SEARCH_INDEX.md` | Search index conceptual model and synchronization |
| `SEARCH_SECURITY.md` | Security model, visibility rules, multi-tenant isolation |
| `API_CONTRACT.md` | Search/discovery API contracts |
| `EVENT_CONTRACT.md` | Event contracts for search indexing |
| `ADR/0009-search-service-boundary.md` | Search service boundary decision |
| `ADR/0010-search-engine-selection.md` | PostgreSQL-only vs OpenSearch vs hybrid |
| `ADR/0011-geo-search-architecture.md` | Geo search architecture decision |
| `ADR/0012-ranking-architecture.md` | Ranking architecture decision |

## Status

```text
DESIGN COMPLETE — NO IMPLEMENTATION
```

This sprint is strictly design. No search engine, no OpenSearch client, no AI, no WhatsApp integration has been created.

## Usage

1. Read `SEARCH_ARCHITECTURE.md` for the overall architecture.
2. Read `DISCOVERY_QUERY.md` for the query model.
3. Read `SEARCH_INDEX.md` for the index model and synchronization strategy.
4. Read `SEARCH_SECURITY.md` for visibility and multi-tenant rules.
5. Read `API_CONTRACT.md` for the REST contract.
6. Read `EVENT_CONTRACT.md` for the event contract.
7. Read ADRs for the architectural decisions.
