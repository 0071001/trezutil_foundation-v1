# Sprint 11 — Service Scenario

## Overview

Demonstrates the service commerce flow: Search Service → View → Ask → Availability boundary.

---

## Scenario Flow

### Step 1: Search for a Service

```
User: "find a plumber near me"
```

**WhatsApp:**
```
↓ CommerceAction(SEARCH_LISTINGS)
↓ SearchUseCases.ExecuteSearch(actor, query{text: "find a plumber"})
Response:
- Service A - Plumber - $50/hr
- Service B - Plumber - $70/hr
```

**REST:**
```
POST /v1/search
{ "text": "find a plumber", "user_id": "user-123" }
```

---

### Step 2: View Service Details

```
User: "tell me about Service A"
```

**WhatsApp:**
```
↓ CommerceAction(GET_LISTING, {listing_id: "svc-a"})
↓ GetListingHandler → ListingUseCases.GetListing
Response:
- Title: Service A - Plumbing
- Description: Professional plumbing services
- Price: $50/hr
- Seller: Handyman Co
- Category: Service
```

---

### Step 3: Ask about Availability

```
User: "Are you available this Saturday?"
```

**WhatsApp:**
```
↓ Message in Conversation
↓ (Seller responds with availability)
```

---

### Step 4: Availability Boundary

```
User: "Book Service A for Saturday"
```

**Response:**
```
Availability check required.
Reservation engine coming in Sprint 12.
```

**Status:** This is the boundary. The Reservation Engine belongs to Sprint 12.

We do NOT fake reservations in Sprint 11.

---

### Step 5: Place Order (without reservation)

```
User: "I'd like to order Service A"
```

**WhatsApp:**
```
↓ CommerceAction(CREATE_ORDER)
↓ CreateOrderHandler → OrderUseCases.CreateOrder
Response:
- order_id: ord-2
- status: draft
- total: calculated from listing price
```

Note: Order is created as a draft. Actual service scheduling requires Sprint 12.

---

## Key Points

- Services use the same Listing domain as products
- `ListingType: "service"` distinguishes service listings
- Search works the same way for services and products
- Availability reservation is the boundary — not implemented in Sprint 11
- Orders can be created for services without reservation
- No fake reservations are created
