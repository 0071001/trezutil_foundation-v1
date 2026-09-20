# Sprint 7 — Commerce Core Actions — Actions

## Implemented Actions

| Commerce Action | Status | Handler | Dependency | Confirmation |
| --------------- | ------ | ------- | ---------- | ------------ |
| SEARCH_LISTINGS | IMPLEMENTED | SearchHandler | SearchUseCases | No |
| GET_LISTING | IMPLEMENTED | GetListingHandler | ListingUseCases | No |
| PUBLISH_LISTING | IMPLEMENTED | PublishListingHandler | ListingUseCases | Yes |
| PAUSE_LISTING | IMPLEMENTED | PauseListingHandler | ListingUseCases | Yes |
| CREATE_LISTING | IMPLEMENTED | CreateListingHandler | ListingUseCases | Yes |

## Not Implemented — No Domain Support

| Commerce Action | Reason |
| --------------- | ------ |
| COMPARE_LISTINGS | No comparison domain model or query infrastructure exists |
| CHECK_INVENTORY | No inventory service or abstraction in the codebase |
| CREATE_ORDER | Order aggregate exists but no OrderRepository or OrderUseCases interface |
| GET_ORDER | No OrderRepository interface |
| CANCEL_ORDER | Order.Cancel() state machine exists but no persistence layer |
| INITIATE_PAYMENT | No payment provider abstraction |
| GET_PAYMENT_STATUS | No payment provider abstraction |
| GET_DELIVERY_STATUS | No delivery tracking abstraction |
| CREATE_MARKETING_CAMPAIGN | No marketing/campaign domain model |
| GET_SELLER_REPORT | No analytics/report domain model |
