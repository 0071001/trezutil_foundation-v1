# Sprint 8 — Order Architecture

## Architecture

```text
Commerce Action
       ↓
Commerce Handler (thin)
       ↓
OrderUseCases (authorization, validation, orchestration)
       ↓
domain.Order (state machine authoritative)
       ↓
OrderRepository (persistence interface)
       ↓
Persistence (PostgreSQL - CERTIFIED)
```

## Key Design Decisions

### Repository Pattern
- OrderRepository follows ListingRepository pattern: `Create`, `FindByID`, `Update`
- FindByID takes `(ctx, orderID)` - tenant isolation enforced in OrderUseCases
- Consistent with existing repository interfaces in `domain/interfaces.go`

### Use Case Pattern
- OrderUseCases follows ListingUseCases pattern:
  - Same authorization pattern (seller owner, org admin, platform admin)
  - Same audit logging pattern
  - Same error handling pattern
  - Actor passed as `actorID string` (from authenticated JWT, not AI)

### Handler Pattern
- Handlers remain thin - no business logic
- Validate action-specific parameters only
- Delegate to OrderUseCases
- Translate domain results to CommerceActionResult

### Pricing Snapshot
- Price frozen at order creation time
- `int64(listing.PriceAmount * 100)` converts dollars to cents
- `domain.NewPricingSnapshot` preserves items, discount, tax, total
- After creation, listing price changes don't affect existing orders

### Tenant Resolution
- CreateOrder: tenant from listing.OrganizationID
- GetOrder/CancelOrder: tenant resolved through authorization check (actor → order access)

### Concurrency
- No optimistic locking/versioning in current implementation (documented limitation)
- Future sprints should add version check on Update

## Order State Machine

```text
DRAFT → PENDING_PAYMENT → CONFIRMED → PROCESSING → READY_FOR_FULFILLMENT → FULFILLED → COMPLETED
  ↓           ↓
CANCELLED    CANCELLED
  ↓
REFUNDED → DISPUTED
```

Cancellation allowed only from DRAFT or PENDING_PAYMENT states.
