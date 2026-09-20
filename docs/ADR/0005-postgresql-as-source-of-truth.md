# ADR 0005 — PostgreSQL as Transactional Source of Truth

## Decision
PostgreSQL is the authoritative transactional store for Trezutil business state.

## Rationale
It provides strong transactional semantics, constraints, indexes, JSON support where useful, mature tooling and a clear source-of-truth model.

OpenSearch, Redis and analytics stores are derived/secondary systems.
