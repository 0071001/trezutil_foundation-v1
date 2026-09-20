# Sprint 8 — Order Contracts

## OrderRepository Interface

```go
type OrderRepository interface {
    Create(ctx context.Context, order *domain.Order) error
    FindByID(ctx context.Context, orderID string) (*domain.Order, error)
    Update(ctx context.Context, order *domain.Order) error
}
```

Tenant isolation: enforced in OrderUseCases (not in repository interface).

## OrderUseCases Interface

```go
type OrderUseCases interface {
    CreateOrder(ctx context.Context, actorID string, listingID string, quantity int) (*domain.Order, error)
    GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error)
    CancelOrder(ctx context.Context, actorID string, orderID string) error
}
```

## Handler Interfaces

```go
type CreateOrderHandler struct { orderUseCases OrderUseCases }
func (h *CreateOrderHandler) Handle(ctx, actor, params) (result, error)
func (h *CreateOrderHandler) ActionType() domain.CommerceActionType // ActionCreateOrder

type GetOrderHandler struct { orderUseCases OrderUseCases }
func (h *GetOrderHandler) Handle(ctx, actor, params) (result, error)
func (h *GetOrderHandler) ActionType() domain.CommerceActionType // ActionGetOrder

type CancelOrderHandler struct { orderUseCases OrderUseCases }
func (h *CancelOrderHandler) Handle(ctx, actor, params) (result, error)
func (h *CancelOrderHandler) ActionType() domain.CommerceActionType // ActionCancelOrder
```

## Order Creation Contract

### Input (CommerceAction Parameters)
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| listing_id | string | Yes | Listing to order |
| quantity | int | Yes | Quantity (must be positive) |

### Output (CommerceActionResult Data)
| Field | Type | Description |
|-------|------|-------------|
| order_id | string | Created order ID |
| status | string | Order status (draft) |
| total | Money | Order total |
| currency | string | Currency code |
| customer_id | string | Authenticated customer ID |

### OrderUseCases.CreateOrder Output
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Generated order ID |
| OrganizationID | string | Tenant from listing |
| CustomerID | string | Authenticated actor |
| SellerID | string | From listing |
| Items | []*OrderItem | Line items with pricing snapshot |
| TotalAmount | Money | Calculated total |
| Currency | string | From listing |
| PricingSnapshot | *PricingSnapshot | Frozen price at creation |
| Status | OrderStatus | Initial: draft |

## Get Order Contract

### Input
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| order_id | string | Yes | Order ID |

### Output
| Field | Type | Description |
|-------|------|-------------|
| ID | string | Order ID |
| Status | OrderStatus | Current status |
| Items | []*OrderItem | Line items |
| TotalAmount | Money | Total |
| PricingSnapshot | *PricingSnapshot | Frozen prices |
| CustomerID | string | Only visible to authorized actors |

## Cancel Order Contract

### Input
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| order_id | string | Yes | Order ID |

### Output
| Field | Type | Description |
|-------|------|-------------|
| order_id | string | Cancelled order ID |
| status | string | "cancelled" |

## Error Semantics

| Error Condition | Status | Error Type |
|----------------|--------|------------|
| Order not found | NOT_FOUND | domain.ErrOrderNotFound |
| Actor cannot access order | FORBIDDEN | domain.ErrInsufficientPermissions |
| Invalid state transition | CONFLICT | domain.ErrOrderCannotBeCancelled |
| Missing listing_id | VALIDATION_FAILED | Error string |
| Missing order_id | NOT_FOUND | domain.ErrOrderNotFound |
| Zero/negative quantity | VALIDATION_FAILED | Error string |
| No actor ID | FORBIDDEN | domain.ErrInsufficientPermissions |
