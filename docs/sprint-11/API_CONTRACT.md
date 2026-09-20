# Sprint 11 — API Contracts

## Overview

This document defines the REST API contracts for Sprint 11 commerce capabilities.

All endpoints require authentication via `user_id` query parameter (validated against `UserRepository`).

---

## Base URL

```
/v1
```

---

## Authentication

| Header / Parameter | Description |
|--------------------|-------------|
| `user_id` (query) | Required. The authenticated user's ID. |

### Tenant Resolution

Tenant is resolved server-side from:
- User's organization membership
- Seller profile ownership
- Channel account configuration (WhatsApp)

**Never** accepted from request body or query parameters.

---

## Error Format

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable message",
    "details": {}
  }
}
```

### Common Error Codes

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `MISSING_USER_ID` | 401 | `user_id` query parameter missing |
| `INVALID_REQUEST` | 400 | Malformed JSON or invalid parameters |
| `UNAUTHORIZED` | 401 | User not found or invalid credentials |
| `FORBIDDEN` | 403 | Insufficient permissions for resource |
| `NOT_FOUND` | 404 | Resource does not exist |
| `CONFLICT` | 409 | Resource conflict (e.g., duplicate) |
| `INVALID_TRANSITION` | 400 | Invalid state transition |
| `SEARCH_FAILED` | 500 | Search engine error |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

---

## Search

### POST /v1/search

Search for listings across categories.

#### Request

```json
{
  "text": "string",
  "category_id": "string (optional)",
  "page_size": "integer (optional, default 10, max 100)",
  "cursor": "string (optional)",
  "sort_field": "relevance|distance|price_asc|price_desc|newest|oldest|availability (optional)",
  "sort_direction": "asc|desc (optional, default desc)"
}
```

#### Response

```json
{
  "items": [
    {
      "listing_id": "string",
      "title": "string",
      "price_amount": "float",
      "price_currency": "string",
      "score": "float"
    }
  ],
  "total": "integer",
  "has_next": "boolean",
  "next_cursor": "string (optional)"
}
```

---

### GET /v1/search/suggest

Autocomplete suggestions for search queries.

#### Request

```
GET /v1/search/suggest?text=string&category_id=string&limit=10
```

#### Response

```json
{
  "suggestions": [
    {
      "text": "string",
      "category_id": "string (optional)",
      "type": "string"
    }
  ]
}
```

---

## Listings

### GET /v1/listings/{listing_id}

Retrieve a listing by ID with visibility resolution.

#### Response

```json
{
  "id": "string",
  "organization_id": "string",
  "seller_id": "string",
  "category_id": "string",
  "title": "string",
  "description": "string",
  "listing_type": "physical_product|real_estate|vehicle|food|service|other",
  "pricing_model": "fixed|negotiable|auction",
  "price_amount": "float",
  "price_currency": "string",
  "status": "draft|pending_review|published|paused|sold|closed|expired|suspended"
}
```

---

### POST /v1/listings

Create a new listing.

#### Request

```json
{
  "listing": {
    "title": "string",
    "listing_type": "physical_product",
    "pricing_model": "fixed",
    "price_amount": 99.99,
    "price_currency": "USD",
    "category_id": "cat-1"
  },
  "attributes": [],
  "media": [],
  "availability": {},
  "locations": []
}
```

---

### POST /v1/listings/{listing_id}/publish

Publish a listing.

#### Response

```json
{ "status": "published" }
```

---

### POST /v1/listings/{listing_id}/pause

Pause a listing.

#### Response

```json
{ "status": "paused" }
```

---

### POST /v1/listings/{listing_id}/resume

Resume a paused listing.

#### Response

```json
{ "status": "published" }
```

---

## Orders

### POST /v1/orders

Create an order from a listing.

#### Request

```json
{
  "listing_id": "string",
  "quantity": 1
}
```

#### Idempotency

Use `Idempotency-Key` header for idempotent order creation.

#### Response

```json
{
  "order_id": "string",
  "status": "draft",
  "total": 9999,
  "currency": "USD",
  "customer_id": "string"
}
```

---

### GET /v1/orders/{order_id}

Retrieve an order by ID.

#### Response

```json
{
  "id": "string",
  "organization_id": "string",
  "seller_id": "string",
  "customer_id": "string",
  "items": [
    {
      "id": "string",
      "listing_id": "string",
      "title": "string",
      "quantity": 1,
      "unit_price": { "amount_minor": 9999, "currency": "USD" },
      "total_price": { "amount_minor": 9999, "currency": "USD" }
    }
  ],
  "total_amount": { "amount_minor": 9999, "currency": "USD" },
  "status": "draft|pending_payment|paid|confirmed|processing|completed|cancelled",
  "created_at": 1700000000
}
```

---

### POST /v1/orders/{order_id}/cancel

Cancel an order.

#### Response

```json
{ "order_id": "string", "status": "cancelled" }
```

#### Error Codes

| Code | Condition |
|------|-----------|
| `ORDER_NOT_FOUND` | Order does not exist |
| `INVALID_TRANSITION` | Order cannot be cancelled in current state |
| `FORBIDDEN` | User is not the order owner |

---

## Listings API Summary

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/v1/listings/{listing_id}` | View listing details |
| `POST` | `/v1/listings` | Create listing |
| `POST` | `/v1/listings/{listing_id}/publish` | Publish listing |
| `POST` | `/v1/listings/{listing_id}/pause` | Pause listing |
| `POST` | `/v1/listings/{listing_id}/resume` | Resume listing |

---

## Orders API Summary

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/orders` | Create order |
| `GET` | `/v1/orders/{order_id}` | Get order |
| `POST` | `/v1/orders/{order_id}/cancel` | Cancel order |

---

## Search API Summary

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/search` | Search listings |
| `GET` | `/v1/search/suggest` | Search suggestions |

---

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `WHATSAPP_WEBHOOK_SECRET` | Yes | HMAC secret for webhook verification |
| `WHATSAPP_APP_SECRET` | No | Fallback for webhook secret |

---

## Testing

```bash
C:\Go\bin\go.exe test -buildvcs=false ./internal/api/...
C:\Go\bin\go.exe test -buildvcs=false ./internal/application/commerce/...
C:\Go\bin\go.exe test -buildvcs=false ./internal/application/conversation/...
```
