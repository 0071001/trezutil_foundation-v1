# Sprint 12 — Scenario: Hotel Room

## Scenario
Customer books a hotel room for a date range.

## Listing Setup
```json
{
  "id": "listing-hotel-001",
  "type": "accommodation",
  "title": "Deluxe Suite",
  "attributes": {
    "room_type": "suite",
    "max_occupancy": 4,
    "bed_type": "king"
  },
  "availability": {
    "capacity": 10,
    "model": "date_range"
  },
  "pricing": {
    "amount_minor": 25000,
    "currency": "USD",
    "model": "per_night"
  }
}
```

## Steps

### 1. Check Availability
**Request**
```
GET /v1/availability?listing_id=listing-hotel-001&start_at=1760227200&end_at=1760486400&quantity=1&party_size=2
```
(Timestamps: 2026-10-12 15:00 UTC → 2026-10-15 11:00 UTC)

**Expected Response**
```json
{
  "available": true,
  "capacity": 10,
  "remaining": 7,
  "reason": "",
  "start_at": 1760227200,
  "end_at": 1760486400
}
```

### 2. Create Reservation
**Request**
```
POST /v1/reservations
Headers: Authorization: Bearer <token>, Idempotency-Key: hotel-res-001
Body:
{
  "listing_id": "listing-hotel-001",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2
}
```

**Expected Response (201)**
```json
{
  "reservation_id": "res-uuid-001",
  "status": "confirmed",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2,
  "customer_id": "user-customer-001",
  "listing_id": "listing-hotel-001"
}
```

### 3. Get Reservation
**Request**
```
GET /v1/reservations/res-uuid-001?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "id": "res-uuid-001",
  "organization_id": "org-hotel-001",
  "customer_id": "user-customer-001",
  "listing_id": "listing-hotel-001",
  "status": "confirmed",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2,
  "pricing_snapshot": {
    "items": [{ "listing_id": "listing-hotel-001", "title": "Deluxe Suite", "quantity": 1, "unit_price": {"amount_minor": 25000, "currency": "USD"}, "total_price": {"amount_minor": 75000, "currency": "USD"} }],
    "subtotal": {"amount_minor": 75000, "currency": "USD"},
    "tax": {"amount_minor": 7500, "currency": "USD"},
    "total": {"amount_minor": 82500, "currency": "USD"},
    "currency": "USD",
    "created_at": 1760000000
  },
  "version": 1,
  "created_at": 1760000000,
  "updated_at": 1760000000
}
```

### 4. Cancel Reservation
**Request**
```
POST /v1/reservations/res-uuid-001/cancel?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "reservation_id": "res-uuid-001",
  "status": "cancelled"
}
```

### 5. Verify Cancellation
**Request**
```
GET /v1/reservations/res-uuid-001?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "status": "cancelled",
  ...
}
```

## WhatsApp Flow
```
User: "I want to book the Deluxe Suite for Oct 12-15 for 2 people"
Bot:  "Checking availability..."
      "✅ Available! 7 suites left for Oct 12-15."
      "Confirm reservation for 2 guests?"
      [Confirm] [Cancel]
User: [Confirm]
Bot:  "✅ Reservation confirmed! ID: res-uuid-001"
      "Check-in: Oct 12, 3PM | Check-out: Oct 15, 11AM"
      "Guests: 2 | Total: $825 (incl. tax)"
```

## Concurrency Test
```
Setup: 1 suite (capacity 1), 2 customers, same dates

Customer A: POST /v1/reservations (idem-key-a)
Customer B: POST /v1/reservations (idem-key-b) — concurrent

Expected:
- One gets 201 Created
- Other gets 409 Conflict { "code": "CONFLICT" }
```