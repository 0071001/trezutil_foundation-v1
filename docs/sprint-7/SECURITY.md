# Sprint 7 — Security

## Identity Protection

The authenticated actor from JWT/session is ALWAYS authoritative. AI-supplied actor identity is NEVER trusted.

**Implementation**: `action.Actor = authenticatedActor` at runtime entry point.

**Tests**:
- TestExecute_IdentityProtection
- TestExecute_PublishListing_IdentityProtection
- TestExecute_PauseListing_IdentityProtection
- TestExecute_CreateListing_IdentityProtection

## Tenant Isolation

Tenant resolved server-side from authenticated actor's OrgID. Never from client/AI input.

**Tests**:
- TestExecute_TenantResolution
- Handler-level tenant isolation tests (SEARCH_LISTINGS, GET_LISTING)

## Authorization

Dual-layer authorization:

1. **Runtime**: Identity protection, tenant resolution, parameter validation, confirmation gate
2. **ListingUseCases**: Seller ownership check, org admin/manager check, platform admin/operator check

Unauthorized access returns `domain.ErrInsufficientPermissions` → `FORBIDDEN` in CommerceActionResult.

**Tests**:
- PublishListing: TestPublishListingHandler_Handle_Forbidden
- PauseListing: TestPauseListingHandler_Handle_Forbidden
- CreateListing: TestCreateListingHandler_Handle_ServiceError

## Confirmation

Write actions require explicit user confirmation before execution:
- PUBLISH_LISTING → REQUIRES_CONFIRMATION
- PAUSE_LISTING → REQUIRES_CONFIRMATION
- CREATE_LISTING → REQUIRES_CONFIRMATION

Confirmed actions proceed to execution.

**Tests**:
- Confirmation tests in confirmation_test.go
- Runtime confirmation tests in runtime_test.go

## Idempotency

In-memory store prevents duplicate execution for state-changing operations with same idempotency key. Not distributed — documented limitation.

**Tests**:
- TestExecute_IdempotencyCacheHit
- Idempotency store tests

## Validation

Parameters validated before execution:
- SEARCH_LISTINGS: text required
- GET_LISTING: listing_id required
- PUBLISH_LISTING: listing_id required
- PAUSE_LISTING: listing_id required
- CREATE_LISTING: listing object required

**Tests**:
- validation_test.go (12 tests)
