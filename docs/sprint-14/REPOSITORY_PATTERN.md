# Sprint 14 — Repository Pattern Guide

## Provider Repository Interface

The provider repository follows the same pattern as all repositories.

```go
type ProviderRepository interface {
    Create(ctx context.Context, orgID string, provider *domain.IntegrationProvider) error
    GetByID(ctx context.Context, orgID, id string) (*domain.IntegrationProvider, error)
    List(ctx context.Context, orgID string, offset, limit int) ([]*domain.IntegrationProvider, int, error)
    Update(ctx context.Context, provider *domain.IntegrationProvider) error
    Delete(ctx context.Context, orgID, id string) error
    FindByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error)
}
```

## Conventions

### Context & Organization

All methods accept `context.Context` as the first parameter. The `organizationID` is explicitly passed (not read from context) to match the pattern in other repositories.

### Optimistic Locking

Updates require a `Version` match:

```sql
UPDATE providers
SET ..., version = version + 1
WHERE id = $1 AND organization_id = $2 AND version = $3
```

If no rows are affected, the operation returns a conflict error — the caller must re-fetch and retry.

### Soft Deletes

Providers are never physically deleted. Instead, `Delete` sets `status = 'SUSPENDED'` (or `INACTIVE`), which makes the provider non-usable without losing audit history.

### Tenant Isolation

Every query is scoped to `organization_id`. Cross-tenant reads return no rows (never an error).

## PostgreSQL Implementation (infrastructure/postgres/provider.go)

Key patterns:

- Uses `*sql.DB` — consistent with all other PostgreSQL repos.
- Capabilities stored as PostgreSQL `TEXT[]` using `pq.Array`.
- Configuration stored as `JSONB`.
- `gen_random_uuid()` for ID generation.
- `CHECK` constraints for `status` and `health_status`.
- `WHERE organization_id = $N` on every query.

## ProviderExecutionRepository

```go
type ProviderExecutionRepository interface {
    Create(ctx context.Context, exec *ProviderExecution) error
    FindByIdempotency(ctx context.Context, orgID, key string) (*ProviderExecution, error)
}
```

Records are immutable after initial creation — `completed_at` and `success` are set at write time. No updates.
