# Sprint 8 — Test Strategy

## Test Coverage

### OrderUseCases Tests (12 tests)
| Test | Scenario |
|------|----------|
| TestOrderUseCases_CreateOrder_Success | Valid order creation with pricing snapshot |
| TestOrderUseCases_CreateOrder_MissingListing | Listing not found → ErrOrderNotFound |
| TestOrderUseCases_CreateOrder_InvalidQuantity | Zero and negative quantity |
| TestOrderUseCases_CreateOrder_EmptyActor | Missing actor |
| TestOrderUseCases_CreateOrder_Unauthorized | Non-authorized actor |
| TestOrderUseCases_GetOrder_Success | Valid retrieval |
| TestOrderUseCases_GetOrder_NotFound | Order not found |
| TestOrderUseCases_GetOrder_EmptyActor | Missing actor |
| TestOrderUseCases_CancelOrder_Success | Valid cancellation, state transition |
| TestOrderUseCases_CancelOrder_NotFound | Order not found |
| TestOrderUseCases_CancelOrder_InvalidState | CONFLICT for paid orders |
| TestOrderUseCases_CancelOrder_EmptyActor | Missing actor |
| TestOrderUseCases_CancelOrder_Unauthorized | Unauthorized actor |
| TestOrderUseCases_GetOrder_TenantIsolation | Tenant A cannot see tenant B's order |

### Handler Tests (22 tests)
- CreateOrderHandler: 6 tests (action type, success, missing listing, service error, identity protection, tenant isolation)
- GetOrderHandler: 6 tests (action type, success, not found, empty ID, service error, tenant isolation)
- CancelOrderHandler: 6 tests (action type, success, not found, empty ID, cannot cancel, identity protection)

### Runtime Tests (9 tests)
- CreateOrder identity protection, confirmation required, missing param
- CancelOrder identity protection, confirmation required, missing param

### Validation Tests (5 tests)
- CreateOrder missing params, zero quantity, valid
- CancelOrder missing params, valid

### Regression Tests
All existing tests in `internal/application/commerce/...` continue passing
All existing tests in `internal/application/...` continue passing
All existing tests in `internal/domain/...` continue passing

## Validation Commands

```text
gofmt: PASS
go vet ./...: PASS
go build ./...: PASS
go test ./...: PASS (except infrastructure-blocked)
```

## Test Double Strategy

All tests use inline fake implementations:
- `fakeOrderRepo` - OrderRepository
- `fakeListingRepoForOrder` - ListingRepository
- `fakeSellerRepoForOrder` - SellerRepository
- `fakeOrgRepoForOrder` - OrganizationRepository
- `fakeUserRepoForOrder` - UserRepository
- `fakeAuditLoggerForOrder` - AuditLogger

No external dependencies required for unit tests.
