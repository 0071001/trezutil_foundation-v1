# Sprint 12 — API Contract

## Base URL
```
https://api.trezutil.example/v1
```

## Authentication
All reservation endpoints require `Authorization: Bearer <JWT>` header.

## Endpoints

### Check Availability
```
GET /v1/availability
```

**Query Parameters**
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| listing_id | UUID | Yes | — | Listing to check |
| start_at | int64 | Yes | — | Unix timestamp (UTC) |
| end_at | int64 | Yes | — | Unix timestamp (UTC) |
| quantity | int | No | 1 | Units to reserve |
| party_size | int | No | 0 | People count |

**Headers**
- `Authorization: Bearer <token>` — Required
- `user_id` query param — Actor ID (from JWT in production)

**Responses**

**200 OK**
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

**400 Bad Request**
```json
{
  "error": {
    "code": "INVALID_DATES",
    "message": "End must be after start"
  }
}
```

**401 Unauthorized**
```json
{
  "error": {
    "code": "MISSING_USER_ID",
    "message": "User ID required"
  }
}
```

---

### Create Reservation
```
POST /v1/reservations
```

**Headers**
- `Authorization: Bearer <token>` — Required
- `Idempotency-Key` — Optional (recommended for retries)
- `Content-Type: application/json`

**Body**
```json
{
  "listing_id": "uuid",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2,
  "idempotency_key": "optional-client-key"
}
```

**Responses**

**201 Created**
```json
{
  "reservation_id": "uuid",
  "status": "pending",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2,
  "customer_id": "uuid",
  "listing_id": "uuid"
}
```

**403 Forbidden**
```json
{
  "error": {
    "code": "FORBIDDEN",
    "message": "Insufficient permissions"
  }
}
```

**404 Not Found**
```json
{
  "error": {
    "code": "LISTING_NOT_FOUND",
    "message": "Listing not found"
  }
}
```

**409 Conflict**
```json
{
  "error": {
    "code": "UNAVAILABLE",
    "message": "Requested period not available"
  }
}
```
or
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "Reservation conflict: period not available"
  }
}
```

**400 Bad Request**
```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid request format"
  }
}
```

---

### Get Reservation
```
GET /v1/reservations/{reservation_id}
```

**Path Parameters**
- `reservation_id` — UUID (required)

**Headers**
- `Authorization: Bearer <token>` — Required
- `user_id` query param — Actor ID

**Responses**

**200 OK**
```json
{
  "id": "uuid",
  "organization_id": "uuid",
  "customer_id": "uuid",
  "listing_id": "uuid",
  "status": "confirmed",
  "start_at": 1760227200,
  "end_at": 1760486400,
  "quantity": 1,
  "party_size": 2,
  "pricing_snapshot": {
    "items": [...],
    "subtotal": {"amount_minor": 20000, "currency": "USD"},
    "tax": {"amount_minor": 2000, "currency": "USD"},
    "total": {"amount_minor": 22000, "currency": "USD"},
    "currency": "USD",
    "created_at": 1760000000
  },
  "version": 1,
  "created_at": 1760000000,
  "updated_at": 1760000000
}
```

**404 Not Found** / **403 Forbidden** — Same as above

---

### Cancel Reservation
```
POST /v1/reservations/{reservation_id}/cancel
```

**Path Parameters**
- `reservation_id` — UUID (required)

**Headers**
- `Authorization: Bearer <token>` — Required
- `user_id` query param — Actor ID

**Responses**

**200 OK**
```json
{
  "reservation_id": "uuid",
  "status": "cancelled"
}
```

**400 Bad Request**
```json
{
  "error": {
    "code": "INVALID_TRANSITION",
    "message": "Reservation cannot be cancelled in current state"
  }
}
```

**404 Not Found** / **403 Forbidden** — Same as above

---

## Idempotency

All write operations (`POST /v1/reservations`, `POST /v1/reservations/{id}/cancel`) support idempotency via:
- `Idempotency-Key` header (preferred)
- `idempotency_key` in request body (fallback)

Same key + same actor + same tenant = same result (no duplicate reservations).

---

## Error Codes

| Code | HTTP | Description |
|------|------|-------------|
| MISSING_USER_ID | 401 | Actor not authenticated |
| MISSING_LISTING_ID | 400 | Required parameter missing |
| MISSING_DATES | 400 | start_at/end_at required |
| INVALID_DATES | 400 | End before start |
| INVALID_REQUEST | 400 | Malformed JSON |
| LISTING_NOT_FOUND | 404 | Listing doesn't exist |
| FORBIDDEN | 403 | Actor not authorized |
| UNAVAILABLE | 409 | Period not available |
| CONFLICT | 409 | Double-booking detected |
| INVALID_TRANSITION | 400 | Invalid status change |
| RESERVATION_NOT_FOUND | 404 | Reservation doesn't exist |
| RESERVATION_CREATE_FAILED | 500 | Internal error |
| AVAILABILITY_CHECK_FAILED | 500 | Internal error |

---

## Tenant Isolation

All endpoints enforce tenant isolation:
- `OrganizationID` resolved server-side from listing
- Actor must have access to the organization (member, seller, admin)
- Cross-tenant access returns 403/404

---

## Status

**DONE** — Implemented in:
- `internal/api/handlers.go` — ReservationHandler with CreateReservationH, GetReservationH, CancelReservationH, CheckAvailabilityH
- `api/openapi/openapi.yaml` — OpenAPI 3.0 specification