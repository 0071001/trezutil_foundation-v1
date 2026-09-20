# Sprint 8 — Commerce Actions

## Implemented Actions

| Commerce Action | Status | Handler | Dependency | Tests |
| --------------- | ------ | ------- | ---------- | ----- |
| CREATE_ORDER | IMPLEMENTED | CreateOrderHandler | OrderUseCases | 6 handler + 12 use case + 3 runtime + 3 validation |
| GET_ORDER | IMPLEMENTED | GetOrderHandler | OrderUseCases | 6 handler + 1 runtime + 1 validation |
| CANCEL_ORDER | IMPLEMENTED | CancelOrderHandler | OrderUseCases | 6 handler + 1 runtime + 1 validation |

## Not Implemented — No Persistence Layer

| Commerce Action | Reason |
| --------------- | ------ |
| SEARCH_LISTINGS | Already implemented in Sprint 6 (SearchEnginePort) |
| GET_LISTING | Already implemented in Sprint 6 (ListingUseCases) |
| PUBLISH_LISTING | Already implemented in Sprint 7 (ListingUseCases) |
| PAUSE_LISTING | Already implemented in Sprint 7 (ListingUseCases) |
| CREATE_LISTING | Already implemented in Sprint 7 (ListingUseCases) |
| CHECK_INVENTORY | No inventory abstraction |
| INITIATE_PAYMENT | No payment abstraction |
| GET_PAYMENT_STATUS | No payment abstraction |
| GET_DELIVERY_STATUS | No delivery abstraction |
| COMPARE_LISTINGS | No comparison domain model |
| CREATE_MARKETING_CAMPAIGN | No marketing abstraction |
| GET_SELLER_REPORT | No analytics abstraction |

## OrderRepository Implementation

OrderRepository now has a PostgreSQL implementation at `internal/infrastructure/postgres/orders.go`.

Orders table migration: `migrations/000005_orders_schema.sql`.

All PostgreSQL integration tests pass.

## Action Matrix Update

Total CommerceActionTypes: 15

| Category | Implemented | Not Implemented | Blocked |
|----------|------------|-----------------|---------|
| Read | SEARCH_LISTINGS, GET_LISTING | COMPARE_LISTINGS, CHECK_INVENTORY | GET_PAYMENT_STATUS, GET_DELIVERY_STATUS, GET_SELLER_REPORT |
| Write | PUBLISH_LISTING, PAUSE_LISTING, CREATE_LISTING, CREATE_ORDER, CANCEL_ORDER | CREATE_MARKETING_CAMPAIGN | — |
| Financial | — | — | INITIATE_PAYMENT |

Total: 10 implemented | 5 not implemented | 3 financial/other | 0 blocked by code | 0 blocked by infrastructure
