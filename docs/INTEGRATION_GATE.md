# Integration Gate

## Purpose

This document defines the rules for declaring a feature production-ready in the Trezutil project.

## Severity Levels

```text
UNIT TEST PASS
```

```text
INTEGRATION TEST PASS
```

```text
PRODUCTION CERTIFIED
```

A feature at `UNIT TEST PASS` level may have passing unit tests and mocks but has **not** been validated against real dependencies.

A feature at `INTEGRATION TEST PASS` level has been validated against real PostgreSQL, OpenSearch, Kafka, and/or Redis infrastructure.

A feature at `PRODUCTION CERTIFIED` level has passed the integration gate and all production-readiness checks.

**No feature may be labeled `PRODUCTION CERTIFIED` without `INTEGRATION TEST PASS`.**

**No feature may be labeled `INTEGRATION TEST PASS` without real dependency validation.**

## Gate Requirements

The following requirements must be satisfied for a feature to be declared `PRODUCTION CERTIFIED`:

1. PostgreSQL integration tests pass.
2. Redis integration tests pass where applicable.
3. Kafka/event integration tests pass where applicable.
4. OpenSearch integration tests pass where applicable.
5. Migration tests pass.
6. Authorization/tenant-isolation integration tests pass.
7. Idempotency tests pass.
8. Failure/retry behavior is validated.
9. Observability checks pass.
10. Security checks pass.

## Feature-Level Gate

Each feature must document its gate status:

| Gate Level | Criteria |
|---|---|
| UNIT TEST PASS | Unit tests pass with mocks/fakes; no real dependency validation |
| INTEGRATION TEST PASS | All gate requirements 1-10 satisfied against real infrastructure |
| PRODUCTION CERTIFIED | Integration gate satisfied; reviewed and approved by engineering lead |

## Distinction Rules

### Mock test ≠ PostgreSQL integration test

Unit tests using mocks or in-memory fakes are NOT integration tests. They must not be reported as integration test results.

### Code complete ≠ Integration certified

A component can be `IMPLEMENTATION COMPLETE` while still being `INTEGRATION NOT CERTIFIED`.

### Implementation complete

```text
All unit tests pass.
Code compiles.
Code passes vet and fmt.
Architecture is sound.
Domain contracts are defined.
But integration has not been validated.
```

### Integration not certified

```text
Unit tests pass.
One or more integration dependencies have not been validated.
Production certification is NOT permitted.
```

### Production certified

```text
Unit tests pass.
All integration dependencies validated.
Integration gate fully satisfied.
Reviewed and approved.
```

## Status Tracking

Each feature must declare:

```text
Implementation Status: COMPLETE | IN_PROGRESS | NOT_STARTED
Integration Status: CERTIFIED | BLOCKED | NOT_CERTIFIED
Production Status: READY | NOT_READY | BLOCKED
```

## Current Project Status

| Feature | Implementation | Integration | Production |
|---|---|---|---|
| Search & Discovery | COMPLETE | BLOCKED (PostgreSQL) | NOT_READY |
| Identity / Auth | COMPLETE | CERTIFIED (PostgreSQL) | READY |
| Organization / Seller | COMPLETE | CERTIFIED (PostgreSQL) | READY |
| Catalog / Listing | COMPLETE | CERTIFIED (PostgreSQL) | READY |
| Order Foundation | IN_PROGRESS | CERTIFIED (PostgreSQL) | NOT_READY |
| Conversation | NOT_STARTED | NOT_CERTIFIED | NOT_READY |
| Channel Gateway | NOT_STARTED | NOT_CERTIFIED | NOT_READY |
| AI Commerce Agent | NOT_STARTED | NOT_CERTIFIED | NOT_READY |

## PostgreSQL Status

PostgreSQL is operational via the `postgresql-x64-18` Windows service (port 5432). The `openpg` user with CREATEDB privilege is configured. All integration tests pass.

Previously blocked on Windows due to PG 16 binaries crashing with `STATUS_DLL_INIT_FAILED` (0xC0000142). PG 18 service runs correctly.
