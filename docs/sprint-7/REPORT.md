# Sprint 7 — Commerce Core Actions — Report

## Status

`GO`

## Repository Audit

### What Already Existed

- `domain.Listing` aggregate with Publish(), Pause(), Resume(), ValidateTransition()
- `domain.Order` aggregate with full 12-status state machine
- `domain.CommerceAction`, `CommerceActionResult`, `CommerceActor` (Sprint 5C)
- `domain.DiscoveryQuery`, `SearchEnginePort` (Sprint 4)
- `application.ListingUseCases` with 5 operations (Create, Get, Publish, Pause, Resume) — each with built-in auth, tenant, and state machine validation
- `application.SearchUseCases` interface with ExecuteSearch, Suggest
- `internal/api/handlers.go` with ListingUseCases interface
- All repository interfaces (ListingRepository, OrganizationRepository, SellerRepository, etc.)

### What Was Reused

- Search handler → SearchUseCases.ExecuteSearch (visibility + auth + search)
- Get listing handler → ListingUseCases.GetListing (auth + tenant + listing)
- Publish handler → ListingUseCases.PublishListing (auth + state machine + persistence)
- Pause handler → ListingUseCases.PauseListing (auth + state machine + persistence)
- Create listing handler → ListingUseCases.CreateListing (auth + validation + full creation pipeline)
- CommerceActionRuntime → identity protection, tenant resolution, confirmation, idempotency, dispatch
- CommerceActionResult → structured error semantics

## Implemented Actions

| Commerce Action | Status | Handler | Dependency |
| --------------- | ------ | ------- | ---------- |
| SEARCH_LISTINGS | IMPLEMENTED | SearchHandler | SearchUseCases |
| GET_LISTING | IMPLEMENTED | GetListingHandler | ListingUseCases |
| PUBLISH_LISTING | IMPLEMENTED | PublishListingHandler | ListingUseCases |
| PAUSE_LISTING | IMPLEMENTED | PauseListingHandler | ListingUseCases |
| CREATE_LISTING | IMPLEMENTED | CreateListingHandler | ListingUseCases |
| COMPARE_LISTINGS | NOT IMPLEMENTED | — | No comparison domain model |
| CHECK_INVENTORY | NOT IMPLEMENTED | — | No inventory abstraction |
| CREATE_ORDER | BLOCKED | — | No OrderRepository/OrderUseCases |
| GET_ORDER | BLOCKED | — | No OrderRepository |
| CANCEL_ORDER | BLOCKED | — | No OrderRepository (Order.Cancel() exists) |
| INITIATE_PAYMENT | NOT IMPLEMENTED | — | No payment abstraction |
| GET_PAYMENT_STATUS | NOT IMPLEMENTED | — | No payment abstraction |
| GET_DELIVERY_STATUS | NOT IMPLEMENTED | — | No delivery abstraction |
| CREATE_MARKETING_CAMPAIGN | NOT IMPLEMENTED | — | No marketing abstraction |
| GET_SELLER_REPORT | NOT IMPLEMENTED | — | No analytics abstraction |

## Key Finding: Order Aggregate Gap

The `domain.Order` aggregate is complete with a 12-status state machine, Money value object, PricingSnapshot, and full transition methods (Submit, ConfirmPayment, Confirm, Process, Cancel, Refund, etc.). However, **no OrderRepository interface or OrderUseCases exist**. This prevents CREATE_ORDER, GET_ORDER, and CANCEL_ORDER from being implemented despite having valid domain logic ready to execute.

## Security

- **Identity protection**: ACTIVE — `action.Actor = authenticatedActor` enforced at runtime entry for all 5 actions
- **Tenant isolation**: ACTIVE — tenant from actor.OrgID, never from client/AI
- **Authorization**: Dual-layer — runtime (identity, tenant, validation, confirmation) + ListingUseCases (seller owner, org admin/manager, platform admin/operator)
- **Confirmation**: ACTIVE — PUBLISH_LISTING, PAUSE_LISTING, CREATE_LISTING require explicit confirmation
- **Idempotency**: ACTIVE — in-memory deduplication for state-changing operations

## Tests

```text
gofmt:          PASS (no files need formatting)
go vet ./...:   PASS
go build ./...: PASS
go test ./...:  PASS (except infrastructure-blocked)

commerce package: 87 tests PASS
existing tests:   PASS (no regressions)
```

Test breakdown:
- Runtime orchestration: 28
- Search handler: 5
- Get listing handler: 7
- Publish listing handler: 5
- Pause listing handler: 5
- Create listing handler: 6
- Handler registry: 7
- Idempotency: 6
- Confirmation: 6
- Validation: 12
- Handler tests: 1

## Infrastructure

| Service | Status | Impact |
|---------|--------|--------|
| PostgreSQL | UNAVAILABLE | EDB installer requires elevation; extracted PG binaries crash on client connection (0xC0000142). Blocks integration tests and order/repository persistence. |
| OpenSearch | UNAVAILABLE | Was running earlier (port 9200), now stopped. Search handler tests use SearchUseCases interface (doesn't require direct OpenSearch connection in unit tests). Integration test (TestOpenSearchAdapter_ExecuteSearch) is pre-existing and infrastructure-blocked. |
| Kafka/Zookeeper | UNAVAILABLE | Batch launchers exist at C:\Trezutil\infra\scripts\. Not required for commerce core actions. |
