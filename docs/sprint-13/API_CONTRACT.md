# Sprint 13 — API Contract

## Base URL

```
/v1
```

## Authentication

All endpoints require `Authorization: Bearer <JWT>` header.
Actor identity and tenant resolved server-side from token.

---

## Inventory Endpoints

### List Inventory Resources

```
GET /v1/inventory
```

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `listing_id` | UUID | No | Filter by listing |
| `resource_type` | string | No | Filter by type |
| `offset` | int | No | Pagination offset (default 0) |
| `limit` | int | No | Page size (default 20, max 100) |

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "tenant_id": "uuid",
      "listing_id": "uuid",
      "resource_type": "COUNTABLE",
      "total": 100,
      "reserved": 10,
      "consumed": 20,
      "available": 70,
      "version": 5,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-20T14:30:00Z"
    }
  ],
  "pagination": {
    "offset": 0,
    "limit": 20,
    "total": 45
  }
}
```

**Errors:**
- `400` — Invalid query parameters
- `401` — Unauthorized
- `403` — Forbidden (not seller/admin for this tenant)

---

### Get Inventory Resource

```
GET /v1/inventory/{id}
```

**Response 200:** Inventory resource object (same as list item)

**Errors:**
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Not found

---

### Adjust Inventory

```
POST /v1/inventory/{id}/adjust
```

**Request Body:**
```json
{
  "delta_total": 10,
  "delta_reserved": 0,
  "delta_consumed": 0,
  "reason": "RESTOCK",
  "idempotency_key": "adj-2024-001"
}
```

**Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `delta_total` | int64 | No | Change to total (can be negative) |
| `delta_reserved` | int64 | No | Change to reserved |
| `delta_consumed` | int64 | No | Change to consumed |
| `reason` | string | Yes | `RECEIVE`, `DAMAGE`, `CORRECTION`, `RESTOCK`, `COUNT_ADJUSTMENT` |
| `idempotency_key` | string | Yes | Idempotency key |

**Response 200:** Updated inventory resource

**Errors:**
- `400` — Invalid adjustment (would violate invariants)
- `401` — Unauthorized
- `403` — Forbidden (seller/admin only)
- `404` — Not found
- `409` — Idempotency conflict / optimistic lock

---

## Capacity Endpoints

### Get Capacity Units

```
GET /v1/capacity/{inventory_id}
```

**Query Parameters:**
| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `start_at` | timestamp | Yes | Range start (RFC3339) |
| `end_at` | timestamp | Yes | Range end (RFC3339) |

**Response 200:**
```json
{
  "data": [
    {
      "id": "uuid",
      "inventory_id": "uuid",
      "start_at": "2024-06-15T10:00:00Z",
      "end_at": "2024-06-15T11:00:00Z",
      "capacity": 10,
      "reserved": 3,
      "available": 7,
      "version": 2,
      "created_at": "2024-01-15T10:00:00Z",
      "updated_at": "2024-01-20T14:30:00Z"
    }
  ]
}
```

**Errors:**
- `400` — Missing/invalid time range
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Inventory not found

---

### Check Capacity Availability

```
POST /v1/capacity/check
```

**Request Body:**
```json
{
  "inventory_id": "uuid",
  "start_at": "2024-06-15T10:00:00Z",
  "end_at": "2024-06-15T11:00:00Z",
  "quantity": 2
}
```

**Response 200:**
```json
{
  "available": true,
  "total_capacity": 10,
  "available_capacity": 7,
  "reason": ""
}
```

**Errors:**
- `400` — Invalid request
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Inventory not found

---

## Fulfillment Endpoints

### Get Fulfillment

```
GET /v1/fulfillments/{id}
```

**Response 200:**
```json
{
  "id": "uuid",
  "tenant_id": "uuid",
  "order_id": "uuid",
  "reservation_id": null,
  "type": "DELIVERY",
  "status": "IN_PROGRESS",
  "reference": "FUL-2024-001",
  "tracking_ref": "TRK-12345",
  "assigned_to": "uuid",
  "metadata": {
    "address": { "street": "...", "city": "..." }
  },
  "created_at": "2024-01-15T10:00:00Z",
  "updated_at": "2024-01-15T14:30:00Z"
}
```

**Errors:**
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Not found

---

### Get Fulfillment by Order

```
GET /v1/orders/{order_id}/fulfillment
```

**Response 200:** Fulfillment object (same as above)

**Errors:**
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Order or fulfillment not found

---

### Get Fulfillment by Reservation

```
GET /v1/reservations/{reservation_id}/fulfillment
```

**Response 200:** Fulfillment object

**Errors:**
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Reservation or fulfillment not found

---

### Transition Fulfillment Status

```
POST /v1/fulfillments/{id}/transition
```

**Request Body:**
```json
{
  "target_status": "IN_PROGRESS",
  "idempotency_key": "trans-2024-001"
}
```

**Valid target_status values:** `ACCEPTED`, `IN_PROGRESS`, `READY`, `COMPLETED`, `CANCELLED`, `FAILED`

**Response 200:** Updated fulfillment object

**Errors:**
- `400` — Invalid transition / missing idempotency key
- `401` — Unauthorized
- `403` — Forbidden (not assigned actor or admin)
- `404` — Not found
- `409` — Invalid transition / optimistic lock / idempotency conflict

---

### Assign Fulfillment

```
POST /v1/fulfillments/{id}/assign
```

**Request Body:**
```json
{
  "actor_id": "uuid",
  "idempotency_key": "assign-2024-001"
}
```

**Response 200:** Updated fulfillment object

**Errors:**
- `400` — Invalid request / not in PENDING status
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Not found
- `409` — Idempotency conflict

---

### Update Tracking Reference

```
POST /v1/fulfillments/{id}/tracking
```

**Request Body:**
```json
{
  "tracking_ref": "TRK-12345",
  "idempotency_key": "trk-2024-001"
}
```

**Response 200:** Updated fulfillment object

**Errors:**
- `400` — Invalid type for tracking / missing idempotency key
- `401` — Unauthorized
- `403` — Forbidden
- `404` — Not found
- `409` — Idempotency conflict

---

## Error Response Format

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": {}
  }
}
```

## Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `INVALID_PARAMETER` | 400 | Invalid request parameter |
| `MISSING_IDEMPOTENCY_KEY` | 400 | Idempotency key required for mutation |
| `IDEMPOTENCY_CONFLICT` | 409 | Duplicate idempotency key |
| `UNAUTHORIZED` | 401 | Missing/invalid auth |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `OPTIMISTIC_LOCK` | 409 | Version conflict |
| `INSUFFICIENT_INVENTORY` | 409 | Not enough inventory available |
| `INSUFFICIENT_CAPACITY` | 409 | Not enough capacity available |
| `INVALID_TRANSITION` | 400 | Invalid state transition |
| `INTERNAL_ERROR` | 500 | Server error |

---

## Idempotency

All mutation endpoints require `idempotency_key` header or body field.
- Keys scoped per tenant
- Results cached for 24 hours
- Repeat request with same key returns cached result

---

## Authorization Matrix

| Endpoint | Customer | Seller | Org Admin | Platform |
|----------|----------|--------|-----------|----------|
| `GET /inventory` | ❌ | ✅ (own) | ✅ | ✅ |
| `GET /inventory/{id}` | ❌ | ✅ (own) | ✅ | ✅ |
| `POST /inventory/{id}/adjust` | ❌ | ✅ (own) | ✅ | ✅ |
| `GET /capacity/{id}` | ❌ | ✅ (own) | ✅ | ✅ |
| `POST /capacity/check` | ✅ | ✅ | ✅ | ✅ |
| `GET /fulfillments/{id}` | ✅ (own) | ✅ (own) | ✅ | ✅ |
| `GET /orders/{id}/fulfillment` | ✅ (own) | ✅ (own) | ✅ | ✅ |
| `POST /fulfillments/{id}/transition` | ❌ | ✅ (assigned) | ✅ | ✅ |
| `POST /fulfillments/{id}/assign` | ❌ | ✅ (admin) | ✅ | ✅ |
| `POST /fulfillments/{id}/tracking` | ❌ | ✅ (assigned) | ✅ | ✅ |

---

## OpenAPI Update

Add to `api/openapi/openapi.yaml`:
- `/inventory` paths
- `/capacity` paths
- `/fulfillments` paths
- `/orders/{id}/fulfillment`
- `/reservations/{id}/fulfillment`
- Schema definitions for all request/response bodies