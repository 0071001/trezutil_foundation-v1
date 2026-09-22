# Sprint 14 — Provider Contract

## Provider Entity

| Field             | Type                 | Required | Description                           |
|-------------------|----------------------|----------|---------------------------------------|
| `ID`              | string               | yes      | Server-generated UUID                 |
| `OrganizationID`  | string               | yes      | Tenant scope (server-resolved)        |
| `Code`            | string               | yes      | Unique within tenant (max 100 chars)  |
| `Name`            | string               | yes      | Display name (max 255 chars)          |
| `Type`            | string               | yes      | Provider category (e.g., "hotel")     |
| `Status`          | ProviderStatus       | yes      | ACTIVE, INACTIVE, SUSPENDED, DEGRADED |
| `Capabilities`    | []Capability         | yes      | Non-empty list of capabilities        |
| `Configuration`   | ProviderConfig       | no       | Endpoints and settings (no secrets)   |
| `SecretRef`       | string               | no       | Reference to secret management entry  |
| `Priority`        | int                  | yes      | Default 0; higher wins selection    |
| `HealthStatus`    | ProviderHealthStatus | yes      | Default UNKNOWN                       |
| `Version`         | int64                | yes      | Default 1; incremented on updates   |
| `CreatedAt`       | int64 (Unix)         | yes      | Server-set                              |
| `UpdatedAt`       | int64 (Unix)         | yes      | Server-set, updated on writes          |

## Status Lifecycle

```
New provider
    ↓
INACTIVE (created)
    ↓
ACTIVE (enabled)
    ↕
SUSPENDED / DEGRADED (operational issues)
    ↓
INACTIVE (disabled)
```

- `NewProvider()` creates a provider in `INACTIVE` state.
- `Enable()` transitions `INACTIVE` → `ACTIVE`. Only one direction; calling on `ACTIVE` returns error.
- `Disable()` transitions `ACTIVE`/`DEGRADED` → `INACTIVE`. Calling on already-INACTIVE/SUSPENDED returns error.
- `Suspend()` transitions to `SUSPENDED`.
- `SetHealth()` updates health status independently of lifecycle status.

## Validation Rules

- `Code` must be non-empty and ≤ 100 chars.
- `Name` must be non-empty and ≤ 255 chars.
- `Status` must be one of: `ACTIVE`, `INACTIVE`, `SUSPENDED`, `DEGRADED`.
- `Version` must be ≥ 1.
- A provider must support at least one capability to be usable.
- `SecretRef` must not be returned through any API.

## Invariants

- `IsUsable()` returns `true` only if `Status == ACTIVE` and `HealthStatus != UNAVAILABLE`.
- `SupportsCapability(c)` checks if capability `c` is in the provider's capability list.
- Optimistic locking via `Version` column enforces concurrent update safety.
