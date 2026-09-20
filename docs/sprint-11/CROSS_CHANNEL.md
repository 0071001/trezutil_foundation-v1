# Sprint 11 — Cross-Channel Proof

## Overview

This document demonstrates that the same business capabilities are executed through WhatsApp, REST, and Web using the same application/business layer.

---

## Architecture

```
WhatsApp                          REST API                         Web
  ↓                                 ↓                                ↓
Conversation                  HTTP Handler                     UI Page
  ↓                                 ↓                                ↓
WhatsApp Service              API Handler                    API Client
  ↓                                 ↓                                ↓
CommerceActionRuntime        CommerceActionRuntime          CommerceActionRuntime
  ↓                                 ↓                                ↓
SearchUseCases               SearchUseCases                 SearchUseCases
ListingUseCases              ListingUseCases                ListingUseCases
OrderUseCases                OrderUseCases                  OrderUseCases
```

**One business capability. Multiple interfaces. Zero duplication.**

---

## Capability: Search Listings

### WhatsApp

```
User: "find laptops under 500"
↓
WhatsApp Service.ProcessWebhook
  ↓ ResolveTenant(WABA ID → Org)
  ↓ ResolveUser(Phone → User)
  ↓ CommerceAction(SEARCH_LISTINGS, actor, conv)
  ↓ CommerceActionRuntime.Execute
    ↓ SearchHandler
      ↓ SearchUseCases.ExecuteSearch(actor, query)
```

### REST API

```
POST /v1/search
{
  "text": "find laptops under 500",
  "user_id": "user-123"
}
↓
SearchHandler.SearchH
  ↓ Build DiscoveryQuery from request
  ↓ SearchUseCases.ExecuteSearch(actorID, query)
```

### Web

```
User types in search box → Debounce → 
fetch('/v1/search', { method: 'POST', body: { text: 'laptops', user_id: 'user-123' } })
↓
SearchHandler.SearchH (same REST endpoint)
  ↓ SearchUseCases.ExecuteSearch(actorID, query)
```

**Same handler. Same use case. Same search engine.**

---

## Capability: View Listing

### WhatsApp

```
User: "show me listing lst-1"
↓
CommerceAction(GET_LISTING, actor, conv)
  ↓ GetListingHandler
    ↓ ListingUseCases.GetListing(actorID, "lst-1")
```

### REST API

```
GET /v1/listings/lst-1?user_id=user-123
↓
ListingHandler.GetListingH
  ↓ ListingUseCases.GetListing(actorID, "lst-1")
```

### Web

```
fetch('/v1/listings/lst-1?user_id=user-123')
  ↓ ListingHandler.GetListingH (same REST endpoint)
```

---

## Capability: Create Order

### WhatsApp

```
User: "buy this"
↓
CommerceAction(CREATE_ORDER, actor, conv, {listing_id: "lst-1"})
  ↓ CreateOrderHandler
    ↓ OrderUseCases.CreateOrder(actorID, "lst-1", 1)
```

### REST API

```
POST /v1/orders
{
  "listing_id": "lst-1",
  "quantity": 1,
  "user_id": "user-123"
}
↓
OrderHandler.CreateOrderH
  ↓ OrderUseCases.CreateOrder(actorID, "lst-1", 1)
```

### Web

```
fetch('/v1/orders', {
  method: 'POST',
  body: JSON.stringify({ listing_id: 'lst-1', quantity: 1 }),
  headers: { 'user_id': 'user-123' }
})
  ↓ OrderHandler.CreateOrderH (same REST endpoint)
```

---

## Capability: Get Order

### WhatsApp

```
User: "what's my order ord-1"
↓
CommerceAction(GET_ORDER, actor, conv, {order_id: "ord-1"})
  ↓ GetOrderHandler
    ↓ OrderUseCases.GetOrder(actorID, "ord-1")
```

### REST API

```
GET /v1/orders/ord-1?user_id=user-123
↓
OrderHandler.GetOrderH
  ↓ OrderUseCases.GetOrder(actorID, "ord-1")
```

---

## Capability: Cancel Order

### WhatsApp

```
User: "cancel my order ord-1"
↓
CommerceAction(CANCEL_ORDER, actor, conv, {order_id: "ord-1"})
  ↓ CancelOrderHandler
    ↓ OrderUseCases.CancelOrder(actorID, "ord-1")
```

### REST API

```
POST /v1/orders/ord-1/cancel?user_id=user-123
↓
OrderHandler.CancelOrderH
  ↓ OrderUseCases.CancelOrder(actorID, "ord-1")
```

---

## Shared Layer Summary

| Component | WhatsApp | REST | Web |
|-----------|----------|------|-----|
| CommerceActionRuntime | ✅ | ✅ | ✅ |
| SearchUseCases | ✅ | ✅ | ✅ |
| ListingUseCases | ✅ | ✅ | ✅ |
| OrderUseCases | ✅ | ✅ | ✅ |
| UsageService | ✅ | ✅ | ✅ |
| PostgreSQL repos | ✅ | ✅ | ✅ |
| Idempotency | ✅ | ✅ | ✅ |

**Zero business logic duplication across channels.**
