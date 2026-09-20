# Sprint 12 — Scenario: Event Ticket

## Scenario
Customer purchases tickets for an event.

## Listing Setup
```json
{
  "id": "listing-event-001",
  "type": "event",
  "title": "Tech Conference 2026 - General Admission",
  "attributes": {
    "event_type": "conference",
    "venue": "Convention Center",
    "date": "2026-11-15",
    "ticket_tier": "general_admission"
  },
  "availability": {
    "capacity": 500,
    "model": "quantity"
  },
  "pricing": {
    "amount_minor": 29900,
    "currency": "USD",
    "model": "per_ticket"
  }
}
```

## Steps

### 1. Check Availability
**Request**
```
GET /v1/availability?listing_id=listing-event-001&start_at=1763241600&end_at=1763328000&quantity=2&party_size=2
```
(Timestamps: Event day 2026-11-15 00:00 → 2026-11-16 00:00)

**Expected Response**
```json
{
  "available": true,
  "capacity": 500,
  "remaining": 347,
  "reason": "",
  "start_at": 1763241600,
  "end_at": 1763328000
}
```

### 2. Create Reservation
**Request**
```
POST /v1/reservations
Headers: Authorization: Bearer <token>, Idempotency-Key: evt-res-001
Body:
{
  "listing_id": "listing-event-001",
  "start_at": 1763241600,
  "end_at": 1763328000,
  "quantity": 2,
  "party_size": 2
}
```

**Expected Response (201)**
```json
{
  "reservation_id": "res-uuid-004",
  "status": "confirmed",
  "start_at": 1763241600,
  "end_at": 1763328000,
  "quantity": 2,
  "party_size": 2,
  "customer_id": "user-customer-001",
  "listing_id": "listing-event-001"
}
```

### 3. Get Reservation
**Request**
```
GET /v1/reservations/res-uuid-004?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "id": "res-uuid-004",
  "organization_id": "org-events-001",
  "customer_id": "user-customer-001",
  "listing_id": "listing-event-001",
  "status": "confirmed",
  "start_at": 1763241600,
  "end_at": 1763328000,
  "quantity": 2,
  "party_size": 2,
  "pricing_snapshot": {
    "items": [{ "listing_id": "listing-event-001", "title": "Tech Conference 2026 - General Admission", "quantity": 2, "unit_price": {"amount_minor": 29900, "currency": "USD"}, "total_price": {"amount_minor": 59800, "currency": "USD"} }],
    "subtotal": {"amount_minor": 59800, "currency": "USD"},
    "tax": {"amount_minor": 5980, "currency": "USD"},
    "total": {"amount_minor": 65780, "currency": "USD"},
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
POST /v1/reservations/res-uuid-004/cancel?user_id=user-customer-001
```

**Expected Response (200)**
```json
{
  "reservation_id": "res-uuid-004",
  "status": "cancelled"
}
```

## WhatsApp Flow
```
User: "Get 2 tickets for Tech Conference"
Bot:  "Checking General Admission tickets..."
      "✅ 347 tickets remaining."
      "Reserve 2 tickets for $657.80 total (incl. tax)?"
      [Confirm] [Cancel]
User: [Confirm]
Bot:  "✅ Tickets reserved! ID: res-uuid-004"
      "Event: Nov 15, 2026 | Convention Center"
      "Tickets: 2 × General Admission"
      "Total: $657.80"
```

## Concurrency Test
```
Setup: 500 tickets, 2 customers want 2 each

Customer A: POST /v1/reservations (quantity=2)
Customer B: POST /v1/reservations (quantity=2) — concurrent

Expected:
- Both succeed (capacity 500 >> 4)
- Remaining: 496
```