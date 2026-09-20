# Sprint 6 — Commerce Actions Catalog

## Action Types

| Action Type | Category | Confirmation | Handler Status |
|-------------|----------|-------------|---------------|
| SEARCH_LISTINGS | READ | No | ✅ IMPLEMENTED |
| GET_LISTING | READ | No | ✅ IMPLEMENTED |
| GET_ORDER | READ | No | ✅ IMPLEMENTED (placeholder) |
| COMPARE_LISTINGS | READ | No | ❌ NOT IMPLEMENTED |
| CHECK_INVENTORY | READ | No | ❌ NOT IMPLEMENTED |
| GET_PAYMENT_STATUS | READ | No | ❌ NOT IMPLEMENTED |
| GET_DELIVERY_STATUS | READ | No | ❌ NOT IMPLEMENTED |
| GET_SELLER_REPORT | READ | No | ❌ NOT IMPLEMENTED |
| CREATE_LISTING | WRITE | Yes | ❌ NOT IMPLEMENTED |
| UPDATE_LISTING | WRITE | Yes | ❌ NOT IMPLEMENTED |
| PUBLISH_LISTING | WRITE | Yes | ❌ NOT IMPLEMENTED |
| PAUSE_LISTING | WRITE | Yes | ❌ NOT IMPLEMENTED |
| CREATE_ORDER | WRITE | Yes | ❌ NOT IMPLEMENTED |
| CANCEL_ORDER | WRITE | Yes | ❌ NOT IMPLEMENTED |
| CREATE_MARKETING_CAMPAIGN | WRITE | Yes | ❌ NOT IMPLEMENTED |
| INITIATE_PAYMENT | FINANCIAL | Yes | ❌ NOT IMPLEMENTED |

## Implemented Handlers

### SEARCH_LISTINGS

Delegates to `domain.SearchEnginePort.ExecuteSearch`. Parameters:
- `text` (string, required) — search query text
- `seller_id` (string, optional) — filter by seller
- `store_id` (string, optional) — filter by store
- `page` (string, optional) — pagination cursor
- `page_size` (int, optional) — results per page

Returns: `{items, total, has_next, next_cursor}`

### GET_LISTING

Delegates to `domain.ListingRepository.FindByID`. Parameters:
- `listing_id` (string, required)

Returns: `domain.Listing` entity

### GET_ORDER

Placeholder handler. Parameters:
- `order_id` (string, required)

Returns: `{order_id, status, message}` (requires persistence for full implementation)

## Handler Interface

```go
type Handler interface {
    Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error)
    ActionType() domain.CommerceActionType
}
```

## Parameter Validation Rules

| Action | Required Parameters |
|--------|-------------------|
| SEARCH_LISTINGS | text |
| GET_LISTING | listing_id |
| GET_ORDER | order_id |
| CREATE_ORDER | listing_id, quantity |
| CANCEL_ORDER | order_id |
| PUBLISH_LISTING | listing_id |
| PAUSE_LISTING | listing_id |
| RESUME_LISTING | listing_id |
