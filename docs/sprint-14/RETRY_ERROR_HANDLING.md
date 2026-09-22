# Sprint 14 — Retry & Error Handling

## Overview

The orchestrator applies a retry policy for transient failures and normalizes all external errors to a generic error code space.

## Error Classification

External errors are classified into three buckets:

| Classification   | Meaning                                           |
|------------------|---------------------------------------------------|
| `RETRYABLE`      | Retry after backoff; provider is temporarily unavailable |
| `NON_RETRYABLE`  | Stop retrying; the request itself is invalid or permanently rejected |
| `UNKNOWN`        | Default for unrecognized errors                   |

### Retryable Errors

- `PROVIDER_TIMEOUT`
- `PROVIDER_RATE_LIMITED`
- `PROVIDER_UNAVAILABLE`
- `UNKNOWN_PROVIDER_ERROR` (treated as retryable for resilience)

### Non-Retryable Errors

- `PROVIDER_AUTH_FAILED`
- `INVALID_REQUEST`
- `UNSUPPORTED_CAPABILITY`
- `RESOURCE_NOT_FOUND`
- `CONFLICT`
- `PROVIDER_REJECTED`

## RetryPolicy

```go
type RetryPolicy struct {
    MaxAttempts     int
    BaseDelay       time.Duration
    MaxDelay        time.Duration
    BackoffFactor   float64
}
```

### Defaults

| Field         | Default  |
|---------------|----------|
| `MaxAttempts` | 3        |
| `BaseDelay`   | 100ms    |
| `MaxDelay`    | 10000ms  |
| `BackoffFactor` | 2.0    |

### Backoff Calculation

```
delay = min(BaseDelay * (BackoffFactor ^ attempt), MaxDelay)
```

Examples:
- Attempt 1: 100ms
- Attempt 2: 200ms
- Attempt 3: 400ms

A small random jitter (±10%) is applied to avoid thundering herd.

## Error Normalization

External providers return provider-specific errors. The orchestrator wraps adapter calls and normalizes errors to:

```go
type ProviderErrorCode string

const (
    ProviderUnavailable       ProviderErrorCode = "PROVIDER_UNAVAILABLE"
    ProviderTimeout           ProviderErrorCode = "PROVIDER_TIMEOUT"
    ProviderAuthFailed        ProviderErrorCode = "PROVIDER_AUTH_FAILED"
    ProviderRateLimited       ProviderErrorCode = "PROVIDER_RATE_LIMITED"
    InvalidRequest            ProviderErrorCode = "INVALID_REQUEST"
    UnsupportedCapability     ProviderErrorCode = "UNSUPPORTED_CAPABILITY"
    ResourceNotFound          ProviderErrorCode = "RESOURCE_NOT_FOUND"
    ResourceUnavailable       ProviderErrorCode = "RESOURCE_UNAVAILABLE"
    Conflict                  ProviderErrorCode = "CONFLICT"
    ProviderRejected          ProviderErrorCode = "PROVIDER_REJECTED"
    UnknownProviderError      ProviderErrorCode = "UNKNOWN_PROVIDER_ERROR"
)
```

The raw provider error is logged internally (for debugging) but never exposed through Commerce APIs or external-facing responses.

## Idempotency & Retry Safety

The retry policy is designed for idempotent-safe operations. Clients that provide an `IdempotencyKey` can safely retry without external side effects.

If a client does not provide a key, the orchestrator generates one (`gen_random_uuid()`) to guarantee safe deduplication.
