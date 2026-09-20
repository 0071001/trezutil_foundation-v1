# Sprint 11 — Rental Scenario

## Overview

Demonstrates the rental commerce flow: Search → View → Availability request.

---

## Scenario Flow

### Step 1: Search for a Rental

```
User: "find apartments for rent"
```

**WhatsApp:**
```
↓ CommerceAction(SEARCH_LISTINGS)
↓ SearchUseCases.ExecuteSearch(actor, query{text: "apartments for rent"})
Response:
- Apartment A - $1500/mo
- Apartment B - $2000/mo
```

**REST:**
```
POST /v1/search
{ "text": "apartments for rent", "user_id": "user-123" }
```

---

### Step 2: View Rental Details

```
User: "Show me Apartment A"
```

**WhatsApp:**
```
↓ CommerceAction(GET_LISTING)
↓ GetListingHandler → ListingUseCases.GetListing
Response:
- Title: Apartment A
- Price: $1500/month
- Location: Downtown
- Status: Published
- Features: 2 beds, 1 bath, 800 sqft
```

---

### Step 3: Request Availability

```
User: "Is Apartment A available in January?"
```

**WhatsApp:**
```
↓ Message in Conversation
↓ CommerceAction generated for Ask intent
↓ (Seller confirms availability)
```

---

### Step 4: Availability Request via API

```
GET /v1/listings/rental-a?user_id=user-123
```

Response includes:
```json
{
  "id": "rental-a",
  "title": "Apartment A",
  "price_amount": 1500,
  "price_currency": "USD",
  "availability": {
    "status": "available",
    "next_available": "2026-01-01"
  }
}
```

**Note:** This is a listing attribute, not a reservation.

---

### Step 5: No Reservation Yet

```
User: "I want to rent Apartment A in January"
```

**Response:**
```
Reservation system coming in Sprint 12.
Availability will be managed through the Reservation Engine.
```

**Status:** Reservation is correctly identified as Sprint 12 capability.

---

## Key Points

- Rentals use the same Listing domain
- `ListingType: "real_estate"` or custom rental type
- Search works the same way for rentals as products
- Availability is a listing attribute, not a reservation
- The Reservation Engine boundary is clearly marked
- No fake bookings are created
