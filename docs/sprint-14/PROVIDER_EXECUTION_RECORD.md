# Sprint 14 — Provider Execution Record

## Overview

Every provider invocation is recorded in the `provider_executions` table for auditing, observability, and idempotency.

## Schema

```sql
CREATE TABLE provider_executions (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id  UUID NOT NULL REFERENCES organizations(id),
    provider_id      UUID NOT NULL REFERENCES providers(id),
    capability       VARCHAR(50) NOT NULL,
    operation        VARCHAR(100) NOT NULL,
    idempotency_key  VARCHAR(255),
    started_at       TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at     TIMESTAMP WITH TIME ZONE,
    duration_ms      BIGINT,
    success          BOOLEAN NOT NULL,
    error_code       VARCHAR(50),
    error_message    TEXT
);
```

## Recording Strategy

The orchestrator creates an `ProviderExecution` record at the start of each `ExecuteCapability` call:

1. **At start** — insert row with `started_at`, `success = false` (pending).
2. **At completion** — update row with `completed_at`, `duration_ms`, `success`, `error_code`, `error_message`.

If the orchestrator cannot write to the execution record table (e.g., DB failure), execution proceeds — execution recording is best-effort and never blocks the actual operation.

## Idempotency

If a client provides an `IdempotencyKey`, the orchestrator checks `provider_executions` for a prior successful execution with the same key. If found, it returns the prior result instead of re-executing.

| Key                 | Behavior                         |
|---------------------|----------------------------------|
| Empty / not provided | Orchestrator generates a UUID    |
| Provided + matches prior successful execution | Returns prior result, skips execution |
| Provided + no match  | Proceeds with normal execution   |

## Fields

| Field             | Description                                 |
|-------------------|---------------------------------------------|
| `capability`      | The capability invoked (e.g., AVAILABILITY) |
| `operation`       | Operation name (e.g., "check_availability") |
| `idempotency_key` | Client-provided or generated UUID           |
| `duration_ms`     | Total time from start to completion         |
| `success`         | Whether the execution succeeded             |
| `error_code`      | Normalized error code or null on success    |
| `error_message`   | Raw error message (internal use)            |

## Observability

The execution record feeds into:

- **Metrics dashboard** — provider success rates, latency percentiles, error code distribution.
- **Alerting** — `success = false` rate exceeds threshold → page on-call.
- **Audit logs** — who invoked what provider, when, and whether it succeeded.

## Retention

Execution records are retained for 90 days. A daily job purges records older than the retention window.
