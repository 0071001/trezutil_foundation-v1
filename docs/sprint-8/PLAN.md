# Sprint 8 — Order & Commerce Transaction Core — Plan

## Phase 1: Domain Foundation
1. **OrderRepository interface** - Added to `domain/interfaces.go` with Create, FindByID, Update
2. **Order errors** - Added `ErrOrderNotFound`, `ErrOrderCannotBeCancelled` to `domain/interfaces.go`

## Phase 2: Application Layer
3. **OrderUseCases** - Created `internal/application/orders.go` with CreateOrder, GetOrder, CancelOrder
4. Authorization: seller owner, org admin/manager, platform admin/operator (consistent with ListingUseCases)

## Phase 3: Commerce Handlers
5. **CreateOrderHandler** - Thin handler validating params, calling OrderUseCases.CreateOrder
6. **GetOrderHandler** - Updated from placeholder to use OrderUseCases.GetOrder
7. **CancelOrderHandler** - New handler using OrderUseCases.CancelOrder

## Phase 4: Tests
8. **orders_test.go** - 12 OrderUseCases tests (success, missing, invalid, unauthorized, tenant isolation, state transitions)
9. **Handler tests** - 22 handler tests across 3 handlers (success, error, identity protection, tenant isolation)
10. **Runtime tests** - 9 new runtime tests (identity protection, confirmation, validation for CREATE_ORDER and CANCEL_ORDER)
11. **Validation tests** - 5 new tests for CREATE_ORDER and CANCEL_ORDER parameters
