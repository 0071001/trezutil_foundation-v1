# Sprint 14 — Security & Secrets

## Overview

Provider secrets (API keys, credentials, tokens) are never stored in the database or codebase. They are managed through a secret management system.

## Secret Storage

- `Provider.SecretRef` contains only a **reference key** (name/identifier) for the secret.
- Actual secret material is stored in the secret management system (Vault, AWS Secrets Manager, etc.).
- The `providers.secret_ref` column stores the reference string; the actual secret is resolved at runtime by the adapter.

## API Security

### Secrets Prevention

The `ProviderResponse` struct explicitly excludes `SecretRef` from JSON serialization:

```go
type ProviderResponse struct {
    ...
    SecretRef      string `json:"-"`
}
```

Test `TestSecretsNotReturned` verifies this — the response body never contains `secretRef`.

### Response Field Whitelisting

All API responses use a dedicated response struct (`ProviderResponse`) with explicit field lists. No entity is serialized directly.

### Cross-Tenant Access

- Provider IDs are scoped to `OrganizationID`.
- Any operation targeting a provider outside the authenticated tenant's scope returns 404.
- The PostgreSQL repository enforces this at the SQL level (`WHERE organization_id = $1`).

## RBAC

| Action          | Platform Role       | Organization Role        |
|-----------------|---------------------|--------------------------|
| Create          | Admin, Super Admin  | Owner, Admin, Manager   |
| List            | Admin, Super Admin  | Owner, Admin, Manager   |
| Get             | Admin, Super Admin  | Owner, Admin, Manager   |
| Enable/Disable  | Admin, Super Admin  | Owner, Admin           |
| Capabilities    | Admin, Super Admin  | Owner, Admin, Manager   |

## Error Exposure

External provider errors are normalized to generic codes:

```
PROVIDER_UNAVAILABLE, PROVIDER_TIMEOUT, PROVIDER_AUTH_FAILED, etc.
```

The raw provider error message and diagnostics are logged internally but never returned to external callers.

## Audit Logging

All provider operations are logged:

- Who performed the action (actor ID)
- What was done (capability, operation)
- When it started/completed
- Duration
- Whether it succeeded
