# Sprint 12 — Scenario: Restaurant Table

## Scenario
Customer reserves a restaurant table for a specific time slot.

## Listing Setup
```json
{
  "id": "listing-restaurant-001",
  "type": "food",
  "title": "Main Dining - Table for 4",
  "attributes": {
    "seating_type": "table",
    "capacity": 4,
    "zone": "main_dining"
  },
  "availability": {
    "capacity": 20,
    "model": "time_slot",
    "slot_duration_minutes": 120
  },
  "pricing": {
    "amount_minor": 0,
    "currency": "USD",
    "model": "pay_at_venue"
  }
}
```

## Steps

### 1. Check Availability
**Request**
```
GET /v1/availability?listing_id=listing-restaurant-001&start_at=1760290800&end_at=1760298000&quantity=1&party_size=4
```
(Timestamps: 2026-10-12 19:00 UTC → 2026-10-12 21:00 UTC)

**Expected Response**
```json
{
  "available": true,
  "capacity": 20,
  "remaining": 12,
  "reason": "",
  "start_at": 1760290800,
  "end_at": 1760298000
}
```

### 2. Create Reservation
**Request**
```
POST /v1/reservations
Headers: Authorization: Bearer <token>, Idempotency-Key: rest-res-001
Body:
{
  "listing_id": "listing-restaurant-001",
  "start_at": 1760290800,
  "end_at": 1760298000,
  "quantity": 1,
  "party_size": 4
}
```

**Expected Response (201)**
```json
{
  "reservation_id": "res-uuid-002",
  "status": "confirmed",
  "start_at": 1760290800,
  "end_at": 1760298000,
  "quantity": 1,
  "party_size": 4,
  "customer_id": "user-customer-001",
  "listing_id": "listing-restaurant-001"
}
```

### 3. Get Reservation
**Request**
```
GET /v1/reservations/res-uuid-002?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "id": "res-uuid-002",
  "organization_id": "org-restaurant-001",
  "customer_id": "user-customer-001",
  "listing_id": "listing-restaurant-001",
  "status": "confirmed",
  "start_at": 1760290800,
  "end_at": 1760298000,
  "quantity": 1,
  "party_size": 4,
  "pricing_snapshot": null,
  "version": 1,
  "created_at": 1760000000,
  "updated_at": 1760000000
}
```

### 4. Cancel Reservation
**Request**
```
POST /v1/reservations/res-uuid-002/cancel?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "reservation_id": "res-uuid-002",
  "status": "cancelled"
}
```

## WhatsApp Flow
```
User: "Reserve a table for 4 at 7PM tomorrow"
Bot:  "Checking tables for tomorrow 7PM..."
      "✅ Available! 12 tables for 4 at 7PM."
      "Confirm reservation for 4 people?"
      [Confirm] [Cancel]
User: [Confirm]
Bot:  "✅ Table reserved! ID: res-uuid-002"
      "Date: Oct 12 | Time: 7:00 PM - 9:00 PM"
      "Party: 4 | Pay at restaurant"
```

## Concurrency Test
```
Setup: 5 tables of 4 (capacity 20 covers), 2 customers want 4 covers at 7PM

Customer A: POST /v1/reservations (party_size=4)
Customer B: POST /v1/reservations (party_size=4) — concurrent

Expected:
- One gets 201 Created
- Other gets 409 Conflict or 201 if capacity remains
```