# Sprint 8 — Authorization Matrix

## Identity Protection

All order operations enforce:
```
Authenticated identity > client-supplied identity > AI-supplied identity
```

The `actorID` parameter in all OrderUseCases comes from `authenticatedActor.ID` (extracted by CommerceActionRuntime). The action's Actor field is never used.

Tested via: TestExecute_CreateOrder_IdentityProtection, TestExecute_CancelOrder_IdentityProtection, TestExecute_GetOrder (through handler).

## Tenant Isolation

Tenant is resolved server-side:
- CreateOrder: from `listing.OrganizationID` (server lookup)
- GetOrder/CancelOrder: through authorization check (actor must have access to the order)

A client cannot access another tenant's order by manipulating order_id.

Tested via: TestGetOrderHandler_Handle_TenantIsolation, TestOrderUseCases_GetOrder_TenantIsolation.

## Authorization Matrix

### CREATE_ORDER
| Actor | Permission | Scope |
|-------|-----------|-------|
| Customer (ActorTypeCustomer) | CAN create | Own orders only |
| Seller (ActorTypeSeller) | CAN create | Orders for own listings |
| Org Admin | CAN create | Org orders |
| Platform Admin | CAN create | All orders |
| System | CAN create | All orders |
| Unauthenticated | FORBIDDEN | — |

### GET_ORDER
| Actor | Permission | Scope |
|-------|-----------|-------|
| Customer | CAN view | Own orders |
| Seller | CAN view | Orders for own listings |
| Org Admin/Manager | CAN view | Org orders |
| Platform Admin | CAN view | All orders |
| Other | FORBIDDEN | — |

### CANCEL_ORDER
| Actor | Permission | State | Scope |
|-------|-----------|-------|-------|
| Customer | CAN cancel | DRAFT, PENDING_PAYMENT | Own orders |
| Seller | CAN cancel | DRAFT, PENDING_PAYMENT | Orders for own listings |
| Org Admin | CAN cancel | DRAFT, PENDING_PAYMENT | Org orders |
| Platform Admin | CAN cancel | DRAFT, PENDING_PAYMENT | All orders |
| Other | FORBIDDEN | — | — |
| Any | CANNOT cancel | CONFIRMED+ | CONFLICT |

## Authorization Implementation

Authorization is enforced at two levels:

1. **OrderUseCases level**: Explicit permission checks using existing Trezutil RBAC
   - Seller ownership: `seller.UserID == actorID`
   - Org membership: `GetMemberRole(tenantID, actorID)` → Owner/Admin/Manager
   - Platform roles: `HasRole(RoleAdmin) || HasRole(RoleOperator)`

2. **CommerceActionRuntime level**: Identity protection, tenant resolution, confirmation gate

Tested via:
- TestOrderUseCases_CreateOrder_Unauthorized
- TestOrderUseCases_GetOrder_TenantIsolation
- TestOrderUseCases_CancelOrder_Unauthorized
