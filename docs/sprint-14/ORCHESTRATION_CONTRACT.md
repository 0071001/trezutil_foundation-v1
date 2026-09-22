# Sprint 14 — Orchestration Contract

## Overview

The `ProviderOrchestrator` is responsible for selecting an appropriate provider, executing the operation, handling failures, and producing normalized results.

## Flow

```
Request
  ↓
Capability
  ↓
Provider Registry (FindByCapability)
  ↓
Eligible Providers (usable, capability match, tenant match)
  ↓
Selection Strategy (priority ordering)
  ↓
Provider Adapter (perform callback)
  ↓
Normalized Result (ExecuteResult)
```

## ExecuteCapability API

```go
func (o *ProviderOrchestrator) ExecuteCapability(
    ctx context.Context,
    orgID, actorID string,
    opts ExecuteOptions,
    perform func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error),
) (*ExecuteResult, error)
```

### ExecuteOptions

| Field           | Type     | Required | Default | Description                        |
|-----------------|----------|----------|---------|------------------------------------|
| `Capability`    | Capability | yes    | —       | The capability to execute          |
| `Operation`     | string   | yes      | —       | Operation name (e.g., "check")     |
| `IdempotencyKey`| string   | no       | —       | Client idempotency key             |
| `Timeout`       | Duration | no       | 30s     | Execution timeout                  |
| `MaxAttempts`   | int      | no       | 3       | Max retry attempts                 |

### ExecuteResult

| Field              | Type             | Description                              |
|--------------------|------------------|------------------------------------------|
| `ProviderID`       | string           | Selected provider ID                     |
| `Success`          | bool             | Whether execution succeeded              |
| `Data`             | map[string]any   | Normalized result data                   |
| `ExternalReference`| string           | Reference from external provider         |
| `ErrorCode`        | ProviderErrorCode| Normalized error code                   |
| `ErrorMessage`     | string           | Error message                            |
| `RetryCount`       | int              | Number of retries attempted              |
| `DurationMs`       | int64            | Execution duration in milliseconds       |

## Provider Selection

1. Registry queries for providers matching the capability, filtered to usable providers (active + health != unavailable).
2. Providers are ordered by priority (descending).
3. Orchestrator iterates through the list, trying each provider in priority order.
4. If a provider fails with a retryable error, the orchestrator retries (with backoff) before moving to the next provider.
5. If a provider fails with a non-retryable error, the orchestrator stops and returns the result.

## Error Normalization

External errors are classified into:

```
PROVIDER_UNAVAILABLE
PROVIDER_TIMEOUT
PROVIDER_AUTH_FAILED
PROVIDER_RATE_LIMITED
INVALID_REQUEST
UNSUPPORTED_CAPABILITY
RESOURCE_NOT_FOUND
RESOURCE_UNAVAILABLE
CONFLICT
PROVIDER_REJECTED
UNKNOWN_PROVIDER_ERROR
```

Raw provider errors are logged internally but not exposed through Commerce APIs.

## Idempotency

The orchestrator preserves client idempotency keys:

```
Client idempotency key
    ↓
Orchestrator (ExecuteOptions.IdempotencyKey)
    ↓
ProviderExecutionContext
    ↓
Provider Execution Record
```

The orchestrator does NOT claim external provider idempotency — it documents that:
- Trezutil guarantees internal idempotent dispatch where possible.
- External provider guarantees depend on provider support.

## Timeout

Every execution has an explicit timeout (default 30s). The orchestrator creates a context with deadline and cancels it after the timeout or when the operation completes.

## Logging

Each execution is logged with:

| Field        | Description                    |
|-------------|--------------------------------|
| provider    | Provider ID                    |
| capability  | Capability type                |
| operation   | Operation name                 |
| tenant      | Organization ID                |
| actor       | Actor ID                       |
| duration    | Execution time                 |
| success     | Boolean                        |
| error       | Error (if failed)              |
