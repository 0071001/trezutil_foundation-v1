# Sprint 11 — Bookable Offer Scenario

## Overview

Demonstrates the bookable offer commerce flow: Search → View → Availability → Reservation in future sprint.

---

## Scenario Flow

### Step 1: Search for a Bookable Offer

```
User: "find hotel rooms in Kinshasa"
```

**WhatsApp:**
```
↓ CommerceAction(SEARCH_LISTINGS)
↓ SearchUseCases.ExecuteSearch(actor, query{text: "hotel rooms in Kinshasa"})
Response:
- Hotel A - $100/night
- Hotel B - $150/night
```

**REST:**
```
POST /v1/search
{ "text": "hotel rooms in Kinshasa", "user_id": "user-123" }
```

---

### Step 2: View Bookable Offer Details

```
User: "Show me Hotel A"
```

**WhatsApp:**
```
↓ CommerceAction(GET_LISTING)
↓ GetListingHandler → ListingUseCases.GetListing
Response:
- Title: Hotel A
- Price: $100/night
- Type: Bookable Offer
- Amenities: WiFi, Pool, Breakfast
- Status: Published
```

---

### Step 3: Check Availability

```
User: "Is Hotel A available March 10-12?"
```

**WhatsApp:**
```
↓ Message in Conversation
↓ CommerceAction generated for Ask intent
↓ Seller/System responds with availability
```

**REST:**
```
GET /v1/listings/hotel-a?user_id=user-123
```

Response includes availability status (provided by seller).

---

### Step 4: Request Reservation

```
User: "Book Hotel A for March 10-12"
```

**Status:** NEXT SPRINT

The Reservation Engine is planned for Sprint 12.

In Sprint 11, we can:
- ✅ View availability (as listing attribute)
- ✅ Create an order (draft) for the listing
- ❌ Create a reservation (Sprint 12)

---

### Step 5: Order as Proxy (Current Sprint)

```
User: "I'd like to book Hotel A"
```

**WhatsApp:**
```
↓ CommerceAction(CREATE_ORDER, {listing_id: "hotel-a"})
↓ CreateOrderHandler → OrderUseCases.CreateOrder
Response:
- order_id: ord-3
- status: draft
- total: $200 (2 nights × $100)
```

**Note:** This is an order, not a reservation.
Actual reservation management (date selection, room blocking, etc.) 
belongs to Sprint 12.

---

## Implementation Status

| Feature | Sprint 11 | Sprint 12 |
|---------|-----------|-----------|
| Search bookable offers | ✅ IMPLEMENTED | — |
| View bookable offers | ✅ IMPLEMENTED | — |
| Check availability | ✅ IMPLEMENTED | — |
| Create reservation | ❌ NOT YET | ✅ PLANNED |
| Availability query API | ❌ NOT YET | ✅ PLANNED |
| Room blocking | ❌ NOT YET | ✅ PLANNED |
| Booking confirmation | ❌ NOT YET | ✅ PLANNED |

---

## Key Points

- Bookable offers use the same Listing domain
- `ListingType` distinguishes bookable offers
- Search and View work identically
- Availability is checked via listing attributes (seller-provided)
- Reservation is the next sprint's capability
- Order is a valid proxy for "I want this" in Sprint 11
- No fake bookings or reservations are created
