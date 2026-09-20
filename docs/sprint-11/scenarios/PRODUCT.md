# Sprint 11 — Product Scenario

## Overview

Demonstrates the complete product flow: Search → View → Ask → Compare if supported → Create Order → Get Order → Cancel if permitted.

---

## Scenario Flow

### Step 1: Search

```
User: "find laptops"
```

**WhatsApp:**
```
↓ CommerceAction(SEARCH_LISTINGS)
↓ SearchHandler → SearchUseCases.ExecuteSearch
Response: 3 listings found
- Laptop A - $999
- Laptop B - $1299
- Laptop C - $799
```

**REST:**
```
POST /v1/search
{ "text": "find laptops", "user_id": "user-123" }
Response: { items: [...], total: 3, has_next: false }
```

**Web:**
```
Search page → type "find laptops" → results appear
```

---

### Step 2: View

```
User: "show me Laptop A"
```

**WhatsApp:**
```
↓ CommerceAction(GET_LISTING, {listing_id: "lst-a"})
↓ GetListingHandler → ListingUseCases.GetListing
Response:
- Title: Laptop A
- Price: $999
- Seller: TechStore
- Status: Published
- Description: High-performance laptop
```

**REST:**
```
GET /v1/listings/lst-a?user_id=user-123
Response: Listing details
```

**Web:**
```
Click listing → Listing Detail page
```

---

### Step 3: Ask

```
User: "Is Laptop A still available?"
```

**WhatsApp:**
```
↓ Message stored in Conversation
↓ CommerceAction generated for Ask intent
↓ (Answer depends on seller response)
```

**REST:** No direct REST endpoint for Ask (conversation-driven).

**Web:** Conversation UI (placeholder this sprint).

---

### Step 4: Compare (NOT IMPLEMENTED)

```
User: "Compare Laptop A and Laptop B"
```

**Status:** NOT_IMPLEMENTED

**Reason:** Compare handler and API endpoint not yet implemented.
Expected in future sprint when generic comparison domain is defined.

---

### Step 5: Create Order

```
User: "I want to buy Laptop A"
```

**WhatsApp:**
```
↓ CommerceAction(CREATE_ORDER, {listing_id: "lst-a"})
↓ CreateOrderHandler → OrderUseCases.CreateOrder
Response:
- order_id: ord-1
- status: draft
- total: $999
- currency: USD
```

**REST:**
```
POST /v1/orders
{ "listing_id": "lst-a", "quantity": 1, "user_id": "user-123" }
Response: { order_id: "ord-1", status: "draft", total: 999 }
```

**Web:**
```
Click "Order" → Confirm → Order created
```

---

### Step 6: Get Order

```
User: "What's my order status?"
```

**WhatsApp:**
```
↓ CommerceAction(GET_ORDER, {order_id: "ord-1"})
↓ GetOrderHandler → OrderUseCases.GetOrder
Response:
- order_id: ord-1
- status: draft
- items: Laptop A x1
- total: $999
```

**REST:**
```
GET /v1/orders/ord-1?user_id=user-123
Response: Order details
```

**Web:**
```
Orders page → Click order → Details
```

---

### Step 7: Cancel (if permitted)

```
User: "Cancel my order ord-1"
```

**WhatsApp:**
```
↓ CommerceAction(CANCEL_ORDER, {order_id: "ord-1"})
↓ CancelOrderHandler → OrderUseCases.CancelOrder
Response: { order_id: "ord-1", status: "cancelled" }
```

**REST:**
```
POST /v1/orders/ord-1/cancel?user_id=user-123
Response: { order_id: "ord-1", status: "cancelled" }
```

**Web:**
```
Orders page → Click order → Cancel → Confirmed
```

---

### Step 8: Cannot Cancel (if not permitted)

```
User: "Cancel my order ord-2" (status: confirmed)
```

**Response:**
```
Error: INVALID_TRANSITION - Order cannot be cancelled in current state
```

---

## Summary

| Step | Action | Status |
|------|--------|--------|
| 1 | Search | ✅ IMPLEMENTED |
| 2 | View | ✅ IMPLEMENTED |
| 3 | Ask | ✅ PARTIAL (WhatsApp only) |
| 4 | Compare | ❌ NOT_IMPLEMENTED |
| 5 | Create Order | ✅ IMPLEMENTED |
| 6 | Get Order | ✅ IMPLEMENTED |
| 7 | Cancel | ✅ IMPLEMENTED |
| 8 | Cancel blocked | ✅ ENFORCED |
