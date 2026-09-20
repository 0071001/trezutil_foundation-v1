# Sprint 12 — Scenario: Service Appointment

## Scenario
Customer books a service appointment (e.g., salon, mechanic, consultant).

## Listing Setup
```json
{
  "id": "listing-service-001",
  "type": "service",
  "title": "Haircut & Styling",
  "attributes": {
    "service_type": "haircut",
    "duration_minutes": 45,
    "staff": "any"
  },
  "availability": {
    "capacity": 1,
    "model": "time_slot",
    "slot_duration_minutes": 45,
    "slots_per_day": ["09:00", "09:45", "10:30", "11:15", "14:00", "14:45", "15:30"]
  },
  "pricing": {
    "amount_minor": 4500,
    "currency": "USD",
    "model": "fixed"
  }
}
```

## Steps

### 1. Check Availability
**Request**
```
GET /v1/availability?listing_id=listing-service-001&start_at=1760269200&end_at=1760271900&quantity=1&party_size=1
```
(Timestamps: 2026-10-12 10:00 UTC → 2026-10-12 10:45 UTC)

**Expected Response**
```json
{
  "available": true,
  "capacity": 1,
  "remaining": 1,
  "reason": "",
  "start_at": 1760269200,
  "end_at": 1760271900
}
```

### 2. Create Reservation
**Request**
```
POST /v1/reservations
Headers: Authorization: Bearer <token>, Idempotency-Key: svc-res-001
Body:
{
  "listing_id": "listing-service-001",
  "start_at": 1760269200,
  "end_at": 1760271900,
  "quantity": 1,
  "party_size": 1
}
```

**Expected Response (201)**
```json
{
  "reservation_id": "res-uuid-003",
  "status": "confirmed",
  "start_at": 1760269200,
  "end_at": 1760271900,
  "quantity": 1,
  "party_size": 1,
  "customer_id": "user-customer-001",
  "listing_id": "listing-service-001"
}
```

### 3. Get Reservation
**Request**
```
GET /v1/reservations/res-uuid-003?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "id": "res-uuid-003",
  "organization_id": "org-salon-001",
  "customer_id": "user-customer-001",
  "listing_id": "listing-service-001",
  "status": "confirmed",
  "start_at": 1760269200,
  "end_at": 1760271900,
  "quantity": 1,
  "party_size": 1,
  "pricing_snapshot": {
    "items": [{ "listing_id": "listing-service-001", "title": "Haircut & Styling", "quantity": 1, "unit_price": {"amount_minor": 4500, "currency": "USD"}, "total_price": {"amount_minor": 4500, "currency": "USD"} }],
    "subtotal": {"amount_minor": 4500, "currency": "USD"},
    "tax": {"amount_minor": 450, "currency": "USD"},
    "total": {"amount_minor": 4950, "currency": "USD"},
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
POST /v1/reservations/res-uuid-003/cancel?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "reservation_id": "res-uuid-003",
  "status": "cancelled"
}
```

## WhatsApp Flow
```
User: "Book a haircut for tomorrow 10AM"
Bot:  "Checking stylist availability for tomorrow 10AM..."
      "✅ Available! 10:00 AM slot open."
      "Confirm haircut appointment for 1 person?"
      [Confirm] [Cancel]
User: [Confirm]
Bot:  "✅ Appointment booked! ID: res-uuid-003"
      "Date: Oct 12 | Time: 10:00 AM - 10:45 AM"
      "Service: Haircut & Styling | Price: $49.50 (incl. tax)"
```

## Concurrency Test
```
Setup: 1 stylist, 1 slot at 10AM, 2 customers

Customer A: POST /v1/reservations (10:00-10:45)
Customer B: POST /v1/reservations (10:00-10:45) — concurrent

Expected:
- One gets 201 Created
- Other gets 409 Conflict
```