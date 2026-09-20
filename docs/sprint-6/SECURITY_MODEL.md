# Sprint 6 — Security Model

## Identity Protection

The Commerce Action Runtime enforces AI identity protection:

```go
// In CommerceActionRuntime.Execute():
action.Actor = authenticatedActor  // ALWAYS override, never trust action.Actor
```

The `action.Actor` field (which may have been set by AI) is always replaced with the authenticated actor from JWT/session before any operation.

### Tests

- `TestExecute_IdentityProtection` — verifies actor is replaced
- `TestExecute_MissingActorID` — validates actor identity at runtime

## Tenant Isolation

Tenant resolution is server-side only:

```go
action.TenantID = authenticatedActor.OrgID
if action.TenantID == "" {
    action.TenantID = authenticatedActor.ID
}
```

Never trusts `TenantID` from client/AI input. Always derived from authenticated actor.

### Tests

- `TestExecute_TenantResolution` — verifies tenant is set from actor

## Authorization

### Per-Action Authorization

The `DefaultConfirmationPolicy` enforces confirmation-based authorization:
- **READ actions**: No confirmation required (public data access)
- **WRITE actions**: Confirmation required (listing modifications)
- **FINANCIAL actions**: Confirmation required (payments)

### Actor Roles

| Actor Type | Scope |
|-----------|-------|
| Customer | Own orders, own data |
| Seller | Own listings, own organization |
| Store Operator | Store-level operations |
| Organization Admin | Organization-wide management |
| Platform Operator | Full system access |
| System | Service-to-service |

### Tests

- `TestExecute_MissingActorType` — validates actor type present
- Confirmation tests verify unauthorized actions require explicit approval

## Confirmation Policy

Actions requiring confirmation before execution:
- CREATE_LISTING, UPDATE_LISTING, PUBLISH_LISTING, PAUSE_LISTING
- CREATE_ORDER, CANCEL_ORDER, CREATE_MARKETING_CAMPAIGN
- INITIATE_PAYMENT

### Tests

- `TestDefaultConfirmationPolicy_*` — 5 confirmation policy tests
- `TestExecute_ConfirmationRequired` — runtime confirmation gate
- `TestExecute_ConfirmedProceeds` — confirmed actions execute successfully

## Idempotency

Prevents duplicate execution of actions with idempotency keys:
- Check cache before execution
- Store result after execution
- Thread-safe in-memory storage

### Tests

- `TestExecute_IdempotencyCacheHit` — duplicate key returns cached result
- `TestInMemoryIdempotencyStore_*` — 6 store tests

## Security Validation Summary

| Control | Status | Tests |
|---------|--------|-------|
| AI identity protection | ACTIVE | TestExecute_IdentityProtection |
| Tenant isolation | ACTIVE | TestExecute_TenantResolution |
| Actor validation | ACTIVE | TestExecute_MissingActor* |
| Parameter validation | ACTIVE | TestExecute_InvalidParameter, validation_test.go |
| Confirmation gate | ACTIVE | Confirmation tests |
| Idempotency dedup | ACTIVE | Idempotency tests |
