# Sprint 8 — Security

## Identity Protection

The authenticated actor from JWT/session is ALWAYS authoritative. AI-supplied actor identity is NEVER trusted.

**Implementation**: `CommerceActionRuntime.Execute()` sets `action.Actor = authenticatedActor` before dispatch.

**Tests**: TestExecute_CreateOrder_IdentityProtection, TestExecute_CancelOrder_IdentityProtection

## Tenant Isolation

Tenant resolved server-side:
- CreateOrder: from listing.OrganizationID (database lookup)
- GetOrder/CancelOrder: through authorization check (order.OrganizationID vs actor's accessible tenants)

**Tests**: TestGetOrderHandler_Handle_TenantIsolation, TestOrderUseCases_GetOrder_TenantIsolation

## Authorization

Three-layer model:
1. **Runtime**: Identity protection + tenant resolution
2. **OrderUseCases**: Seller ownership, org membership, platform roles
3. **Domain**: State machine guards (cancellation from valid states only)

**Tests**: TestOrderUseCases_CreateOrder_Unauthorized, TestOrderUseCases_CancelOrder_Unauthorized, TestOrderUseCases_CancelOrder_InvalidState

## Confirmation

CREATE_ORDER and CANCEL_ORDER require confirmation (WRITE actions per DefaultConfirmationPolicy).

**Tests**: TestExecute_CreateOrder_ConfirmationRequired, TestExecute_CancelOrder_ConfirmationRequired

## State Transition Protection

Cancellation uses the existing `domain.Order.Cancel()` state machine, which enforces:
- DRAFT, PENDING_PAYMENT → Cancelled (allowed)
- Paid, Confirmed, Processing, etc. → Cannot cancel (CONFLICT)
- Terminal states → Cannot cancel (CONFLICT)

**Tests**: TestOrderUseCases_CancelOrder_InvalidState

## Idempotency

CREATE_ORDER is protected by the CommerceActionRuntime's idempotency system. Same actor + same tenant + same action + same key → no duplicate order. In-memory store (documented limitation).

**Tests**: TestExecute_IdempotencyCacheHit (generic runtime test)
