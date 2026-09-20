# Sprint 5C — Authorization Matrix

## Actor Types

| Actor | Description | Source |
|-------|-------------|--------|
| CUSTOMER | Purchaser/buyer | Authenticated JWT |
| SELLER | Seller profile owner | Authenticated JWT |
| STORE_OPERATOR | Store staff member | Authenticated JWT |
| ORGANIZATION_ADMIN | Org-level admin | Authenticated JWT |
| PLATFORM_OPERATOR | Platform-level operator | Authenticated JWT |
| SYSTEM | Service-to-service | Internal |

## Authorization Rules

All authorization is server-side. Client-supplied ownership fields are NEVER trusted.

```text
actor_id, organization_id, seller_id, store_id, role from client/AI
→ IGNORED for authorization decisions
→ Authenticated identity from JWT/session is used instead
```

## Matrix

| Action | Customer | Seller | Store Operator | Org Admin | Platform Operator | System |
|--------|----------|--------|----------------|-----------|-------------------|--------|
| SEARCH_LISTINGS | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GET_LISTING | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| COMPARE_LISTINGS | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| GET_ORDER (own) | ✅ | — | — | — | ✅ | ✅ |
| GET_ORDER (org) | — | ✅ | ✅ | ✅ | ✅ | ✅ |
| CREATE_LISTING | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| UPDATE_LISTING | ❌ | Own | ✅ | ✅ | ✅ | ✅ |
| PUBLISH_LISTING | ❌ | Own | ✅ | ✅ | ✅ | ✅ |
| PAUSE_LISTING | ❌ | Own | ✅ | ✅ | ✅ | ✅ |
| CREATE_ORDER | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ |
| GET_ORDER | Own | Own | Store | Org | ✅ | ✅ |
| CANCEL_ORDER | Own | Policy | Policy | Policy | ✅ | ✅ |
| INITIATE_PAYMENT | Own | Policy | Policy | Policy | ✅ | ✅ |
| CHECK_INVENTORY | ❌ | Own | ✅ | ✅ | ✅ | ✅ |
| GET_SELLER_REPORT | ❌ | Own | ✅ | ✅ | ✅ | ✅ |
| CREATE_CAMPAIGN | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |

Legend:
- ✅ = Allowed for all actors of this type
- Own = Allowed only for the owner of the resource
- Policy = Determined by business rules (documented per action)
- ❌ = Not allowed

## Tenancy Rules

Every action is tenant-scoped:
1. Organization context is resolved from the authenticated user's membership
2. Resource access is validated against the user's organization
3. Cross-tenant access is NEVER permitted for regular actors
4. Platform operators can span tenants but are still audited

## Negative Authorization Tests

These scenarios MUST be rejected:

```text
Customer attempting to publish a listing → UNAUTHORIZED
Seller attempting to access another seller's listing → FORBIDDEN (cross-tenant)
User attempting to access another user's order → FORBIDDEN
AI-generated actor identity conflicting with authenticated identity → IGNORED (auth wins)
Cross-tenant action attempt → FORBIDDEN
Unauthenticated action attempt → UNAUTHORIZED
```
