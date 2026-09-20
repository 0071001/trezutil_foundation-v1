# Sprint 7 — Action Matrix

## Identity Protection

All handlers receive the authenticated actor via the runtime. The `action.Actor` field (potentially set by AI) is always overridden:

```
action.Actor = authenticatedActor
```

Tested via: TestExecute_*_IdentityProtection for all implemented actions.

## Tenant Isolation

Tenant is resolved server-side from `authenticatedActor.OrgID`, falling back to `authenticatedActor.ID`:

```
action.TenantID = authenticatedActor.OrgID
if action.TenantID == "" {
    action.TenantID = authenticatedActor.ID
}
```

Tested via: TestExecute_TenantResolution, handler-level tenant isolation tests.

## Authorization Flow

```
Authenticated Actor → CommerceActionRuntime.Execute
    → Parameter Validation
    → Confirmation Gate (WRITE/FINANCIAL require confirmation)
    → Handler
        → Handler validates action-specific inputs
        → Handler calls ListingUseCases (which has built-in auth)
        → ListingUseCases checks: seller owner, org admin/manager, platform admin/operator
        → Returns FORBIDDEN if unauthorized
```

Authorization is enforced at TWO levels:
1. Runtime level: identity protection, tenant resolution, parameter validation, confirmation
2. Handler level: ListingUseCases internal authorization (seller ownership, org membership, platform roles)

## Confirmation Policy

| Action | Requires Confirmation | Reason |
| ------ | --------------------- | ------ |
| SEARCH_LISTINGS | No | Read action |
| GET_LISTING | No | Read action |
| PUBLISH_LISTING | Yes | Write action |
| PAUSE_LISTING | Yes | Write action |
| CREATE_LISTING | Yes | Write action |

## Idempotency

State-changing operations (PUBLISH_LISTING, PAUSE_LISTING, CREATE_LISTING) use idempotency keys:
- Same actor + same tenant + same action + same key → no duplicate side effects
- In-memory store (not distributed — documented limitation)
- Result cached after first successful execution

## Error Semantics

| Error Source | CommerceActionResult Status |
| ------------ | --------------------------- |
| domain.ErrListingNotFound | NOT_FOUND |
| domain.ErrInsufficientPermissions | FORBIDDEN |
| Validation failure | VALIDATION_FAILED |
| Missing action handler | NOT_IMPLEMENTED |
| Handler execution error | FAILED |
| Action needs confirmation | REQUIRES_CONFIRMATION |
