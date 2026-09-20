# Sprint 8 — Order Workflows

## Architecture

```text
Conversation
    ↓
CommerceAction
    ↓
CommerceActionRuntime.Execute
    ↓
Handler (CreateOrder/GetOrder/CancelOrder)
    ↓
OrderUseCases
    ↓
domain.Order
    ↓
OrderRepository
    ↓
Persistence (BLOCKED)
```

## Create Order Workflow

```text
1. User: "Buy this listing"
2. Intent: CREATE_ORDER
3. CommerceAction: listing_id="listing-1", quantity=1
4. Runtime → CreateOrderHandler
5. Handler validates: listing_id present, quantity > 0
6. Handler → OrderUseCases.CreateOrder(actorID="user-1", listingID="listing-1", quantity=1)
7. OrderUseCases:
   a. Validate actor is authenticated
   b. Find listing by ID
   c. Resolve tenant from listing.OrganizationID
   d. Resolve actor from userRepo
   e. Check authorization (customer, org admin, platform admin)
   f. Build OrderItem from listing + quantity
   g. Create PricingSnapshot with frozen price
   h. Call domain.NewOrder(tenantID, sellerID, customerID, storeID, items, currency, now)
   i. Persist via OrderRepository.Create()
   j. Audit log
8. Return CommerceActionResult with order ID, status, total
```

## Get Order Workflow

```text
1. User: "Show me my order"
2. Intent: GET_ORDER
3. CommerceAction: order_id="order-1"
4. Runtime → GetOrderHandler
5. Handler validates: order_id present
6. Handler → OrderUseCases.GetOrder(actorID="user-1", orderID="order-1")
7. OrderUseCases:
   a. Validate actor is authenticated
   b. Find order by ID (via OrderRepository)
   c. Check authorization (customer/seller/org admin/platform admin)
8. Return order details
```

## Cancel Order Workflow

```text
1. User: "Cancel my order"
2. Intent: CANCEL_ORDER
3. CommerceAction: order_id="order-1"
4. Runtime → CancelOrderHandler
5. Handler validates: order_id present
6. Handler → OrderUseCases.CancelOrder(actorID="user-1", orderID="order-1")
7. OrderUseCases:
   a. Validate actor is authenticated
   b. Find order by ID
   c. Check authorization
   d. Check state: must be DRAFT or PENDING_PAYMENT
   e. Call order.Cancel(now) — domain state machine
   f. Update via OrderRepository.Update()
   g. Audit log
8. Return CommerceActionResult with cancelled status
```

## Error Handling Workflow

```text
Order not found → NOT_FOUND
Actor cannot access → FORBIDDEN
Invalid state → CONFLICT (domain.ErrOrderCannotBeCancelled)
Missing params → VALIDATION_FAILED
```

## Real Estate Scenario

```text
Customer: "I want to buy this property"
→ GET_LISTING (view details)
→ CREATE_ORDER (create order for listing)
→ GET_ORDER (check order status)
→ CANCEL_ORDER (if needed, from DRAFT/PENDING_PAYMENT)
```
