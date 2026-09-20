# Sprint 11 — Security Matrix

## Overview

Security guarantees for the Trezutil Commerce & Booking OS.

---

## Authentication

### Mechanism

- OTP-based authentication via phone number
- JWT tokens issued after OTP verification
- All API endpoints require `user_id` query parameter
- `user_id` validated against `UserRepository`

### Token Service

```go
type TokenService interface {
    IssueAccessToken(userID string, roles []Role) (string, error)
    IssueRefreshToken(userID string) (string, error)
    ValidateAccessToken(token string) (string, error)
    RotateRefreshToken(oldToken string) (string, error)
}
```

---

## Authorization

### Role-Based Access Control

| Role | Scope |
|------|-------|
| Customer | Browse, search, order, ask |
| Seller | Manage listings, view orders, usage |
| Organization Admin | Manage organization, members, settings |
| Store Operator | Manage store-specific operations |
| Platform Operator | Full system access |

### Authorization Flow

```
Request → Extract user_id → Load User + Roles → Check permissions → Execute
```

### Commerce Action Authorization

```go
// ActionExecutor checks authorization before execution
err := e.authResolver.AuthorizeAction(ctx, authenticatedActor, action)
if err == domain.ErrInsufficientPermissions {
    return ACTION_UNAUTHORIZED
}
```

---

## Tenant Isolation

### Rule

Every query, action, and response is scoped to the actor's tenant (organization).

### Enforcement Points

| Layer | Enforcement |
|-------|-------------|
| **API** | `user_id` → User → Organization membership |
| **Application** | Actor.OrgID used in all queries |
| **Domain** | CommerceAction.TenantID set server-side |
| **Persistence** | PostgreSQL queries filter by `organization_id` |

### WhatsApp Tenant Resolution

```
WABA ID → ChannelAccountRepository.FindByChannelExternalID → OrganizationID
```

**Never** accept tenant from webhook payload.

---

## Seller Ownership

### Listings

- A seller can only create, update, and manage listings belonging to their organization
- `Listing.OrganizationID` and `Listing.SellerID` enforce ownership
- API validates actor has seller role in the target organization

### Orders

- Customer can only view/cancel orders they created
- Order.CustomerID matches authenticated actor ID
- Order.OrganizationID matches tenant context

---

## Listing Visibility

### Search Visibility

```
SearchUseCases.ExecuteSearch → VisibilityResolver.ResolveVisibility(actorID)
```

| Mode | Description |
|------|-------------|
| **Public** | Actor sees all published listings |
| **Internal** | Actor sees listings within their organization |

### Get Listing Visibility

- `ListingUseCases.GetListing` checks actor has access to the listing's organization
- Unpublished listings only visible to owner/seller

---

## Order Ownership

### Rules

| Operation | Ownership Check |
|-----------|----------------|
| Get Order | Order.CustomerID == actorID OR actor is seller of Order.SellerID |
| Cancel Order | Order.CustomerID == actorID AND status allows cancellation |
| Create Order | Listing belongs to actor's organization, actor is customer |

---

## Identity Protection

### AI Identity Protection

```go
// In CommerceActionRuntime.Execute:
action.Actor = authenticatedActor  // Always override AI-supplied actor
action.TenantID = authenticatedActor.OrgID
```

### WhatsApp Identity Protection

```
WhatsApp Phone → UserRepository.FindByPhone → User.ID → CommerceActor.ID
```

WhatsApp phone numbers are **never** used as actors directly.

---

## Idempotency

### Webhook Idempotency

- Durable via `WebhookEventStore` (atomic claim pattern)
- Prevents duplicate commerce action execution
- Prevents double usage credit consumption

### API Idempotency

- `Idempotency-Key` header on mutating operations (e.g., `POST /v1/orders`)
- Prevents duplicate order creation on retries

---

## Usage Enforcement

### Credit System

```
Action Executed → UsageService.ResolvePolicy → Check credits → Reserve → Execute → Consume
```

### Policy Resolution

| Action Type | Credits | Channel |
|-------------|---------|---------|
| SEARCH_LISTINGS | Configurable per action | WhatsApp, REST, Web |
| CREATE_ORDER | Higher cost (financial) | WhatsApp, REST, Web |
| CREATE_LISTING | Write operation cost | WhatsApp, REST, Web |

---

## Pricing Integrity

### Money Handling

- All monetary values use `Money` struct with `AmountMinor` (integer) and `Currency`
- No floating-point arithmetic on prices
- `PricingSnapshot` frozen at order creation time
- Order totals recalculated from item prices, never trusted from client

---

## CORS / Configuration

### Security Headers

All HTTP responses include:
- `Content-Type: application/json`
- Appropriate status codes per error type
- No raw error details exposed to clients

### Error Handling

Backend errors are transformed into structured `ErrorResponse` objects:
- No database error messages exposed
- No stack traces in responses
- Generic messages for internal errors

---

## Testing Security Properties

```go
// Tenant isolation
GetListing(actor1, listing_from_other_org) → ErrInsufficientPermissions
GetListing(actor2, listing_from_own_org) → OK

// Identity protection
ExecuteAction(actor=AI_supplied) → actor = authenticatedActor

// Idempotency
CreateOrder(key="abc") → 201
CreateOrder(key="abc") → Same result, no duplicate

// Ownership
CancelOrder(other_user_order) → ErrInsufficientPermissions
```
