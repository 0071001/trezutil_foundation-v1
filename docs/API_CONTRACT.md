# Trezutil API Contract

**Status:** Version 1.0 — Contract baseline

## 1. Contract-first rule

No significant business implementation should be merged without a corresponding API or internal application contract.

External APIs use REST/OpenAPI. Internal high-throughput or strongly typed service boundaries may use gRPC/Protobuf.

## 2. Base conventions

Base path: `/v1`

Content type: `application/json`

IDs: UUID strings

Timestamps: RFC 3339 UTC

Money:
```json
{ "amount_minor": 60000, "currency": "USD" }
```

Pagination:
```json
{ "page_size": 25, "page_token": "..." }
```

Every error uses a stable machine-readable code:
```json
{
  "error": {
    "code": "LISTING_NOT_FOUND",
    "message": "Listing not found",
    "details": {},
    "request_id": "..."
  }
}
```

## 3. Authentication

External privileged APIs use bearer authentication. Channel webhooks use provider signature verification plus replay/idempotency controls.

Roles and authorization are enforced server-side.

## 4. Idempotency

State-changing endpoints exposed to unreliable clients or external callers SHOULD support:

`Idempotency-Key: <unique-key>`

The server must persist the result for the required replay window.

## 5. Initial resource groups

### Identity
- `POST /v1/auth/verify-phone`
- `GET /v1/me`
- `GET /v1/me/memberships`

### Sellers
- `POST /v1/sellers`
- `GET /v1/sellers/{seller_id}`
- `PATCH /v1/sellers/{seller_id}`
- `POST /v1/sellers/{seller_id}/verification`

### Listings
- `POST /v1/listings`
- `GET /v1/listings/{listing_id}`
- `PATCH /v1/listings/{listing_id}`
- `POST /v1/listings/{listing_id}/publish`
- `POST /v1/listings/{listing_id}/unpublish`

### Search
- `POST /v1/search/listings`

Natural-language requests may arrive as text, but the resulting structured query is validated before execution.

### Conversations
- `POST /v1/webhooks/whatsapp`
- `GET /v1/conversations/{conversation_id}`
- `POST /v1/conversations/{conversation_id}/handoff`

### Conversations (Sprint 18 — Conversational Commerce)
- `POST /api/v1/conversations/message` — process an inbound message
- `POST /api/v1/conversations/{conversation_id}/confirm` — confirm a pending action
- `POST /api/v1/conversations/{conversation_id}/reject` — reject a pending action
- `POST /api/v1/conversations/{conversation_id}/token` — generate a session token

Conversation endpoints accept a channel-neutral message payload and return a
channel-neutral `ConversationResponse` containing text, cards, action prompts,
clarification (missing parameters), or confirmation state.

Idempotency: each inbound message carries a `MessageIdempotencyKey`. Duplicate
delivery produces the same logical response without duplicate side effects.

### Orders
- `POST /v1/orders`
- `GET /v1/orders/{order_id}`
- `POST /v1/orders/{order_id}/cancel`

### Payments
- `POST /v1/payment-intents`
- `GET /v1/payment-intents/{payment_intent_id}`
- `POST /v1/webhooks/payments/{provider}`

### Logistics
- `POST /v1/deliveries`
- `GET /v1/deliveries/{delivery_id}`
- `POST /v1/webhooks/logistics/{provider}`

### Marketing / distribution
- `POST /v1/campaigns`
- `POST /v1/listings/{listing_id}/distributions`
- `GET /v1/listings/{listing_id}/distributions`

### Admin
- seller verification
- listing moderation
- dispute management
- audit search
- platform configuration

These operations MUST be protected by admin permissions and auditable.

## 6. API behaviour for WhatsApp

WhatsApp-specific payloads are normalized into a channel-neutral command/message model before entering application services.

Example:
```text
WhatsApp webhook
  -> verify signature
  -> normalize message
  -> resolve conversation
  -> infer intent
  -> authorize tool/action
  -> execute domain command
  -> persist result
  -> emit events
  -> render response for WhatsApp
```

## 7. AI tool contracts

AI may call only explicit application tools, such as:

- `search_listings`
- `get_listing`
- `create_listing_draft`
- `update_listing`
- `publish_listing`
- `create_order`
- `get_order_status`
- `initiate_payment`
- `schedule_visit`
- `generate_seller_report`
- `create_campaign`
- `request_human_handoff`

Each tool must enforce authentication, authorization, validation, idempotency and audit requirements.

## 8. Compatibility policy

Breaking API changes require a new major API version or explicit compatibility strategy. Additive changes are preferred.
