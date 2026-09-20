# Sprint 11 — Action Matrix

## Overview

For every `CommerceActionType`, this matrix shows implementation status across all channels and layers.

---

## Matrix

| Action Type | Domain | Application | API | Web | WhatsApp | Tests | Status |
|---|---|---|---|---|---|---|---|
| `SEARCH_LISTINGS` | Discovery | ✅ `SearchHandler` → `SearchUseCases.ExecuteSearch` | ✅ `POST /v1/search` | ✅ Search page | ✅ Via `SearchHandler` | ✅ Unit | **IMPLEMENTED** |
| `GET_LISTING` | Catalog | ✅ `GetListingHandler` → `ListingUseCases.GetListing` | ✅ `GET /v1/listings/{id}` | ✅ Listing Detail | ✅ Via runtime | ✅ Unit | **IMPLEMENTED** |
| `COMPARE_LISTINGS` | Catalog | ❌ No handler | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `SUGGEST` | Catalog | ✅ `SuggestHandler` → `SearchUseCases.Suggest` | ✅ `GET /v1/search/suggest` | ✅ Search input | ✅ Via `SuggestHandler` | ✅ Unit | **IMPLEMENTED** |
| `CREATE_LISTING` | Catalog | ✅ `CreateListingHandler` → `ListingUseCases.CreateListing` | ✅ `POST /v1/listings` | ✅ Seller Create | ✅ Via `CreateListingHandler` | ✅ Unit | **IMPLEMENTED** |
| `UPDATE_LISTING` | Catalog | ❌ Not yet | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `PUBLISH_LISTING` | Catalog | ✅ `PublishListingHandler` → `ListingUseCases.PublishListing` | ✅ `POST /v1/listings/{id}/publish` | ✅ Seller Dashboard | ✅ Via `PublishListingHandler` | ✅ Unit | **IMPLEMENTED** |
| `PAUSE_LISTING` | Catalog | ✅ `PauseListingHandler` → `ListingUseCases.PauseListing` | ✅ `POST /v1/listings/{id}/pause` | ✅ Seller Dashboard | ✅ Via `PauseListingHandler` | ✅ Unit | **IMPLEMENTED** |
| `RESUME_LISTING` | Catalog | ✅ `ResumeListingHandler` → `ListingUseCases.ResumeListing` | ✅ `POST /v1/listings/{id}/resume` | ✅ Seller Dashboard | ✅ Via `ResumeListingHandler` | ✅ Unit | **IMPLEMENTED** |
| `CREATE_ORDER` | Order | ✅ `CreateOrderHandler` → `OrderUseCases.CreateOrder` | ✅ `POST /v1/orders` | ✅ Order flow | ✅ Via `CreateOrderHandler` | ✅ Unit | **IMPLEMENTED** |
| `GET_ORDER` | Order | ✅ `GetOrderHandler` → `OrderUseCases.GetOrder` | ✅ `GET /v1/orders/{id}` | ✅ Orders | ✅ Via `GetOrderHandler` | ✅ Unit | **IMPLEMENTED** |
| `CANCEL_ORDER` | Order | ✅ `CancelOrderHandler` → `OrderUseCases.CancelOrder` | ✅ `POST /v1/orders/{id}/cancel` | ✅ Orders | ✅ Via `CancelOrderHandler` | ✅ Unit | **IMPLEMENTED** |
| `CHECK_INVENTORY` | Inventory | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `INITIATE_PAYMENT` | Payment | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `GET_PAYMENT_STATUS` | Payment | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `GET_DELIVERY_STATUS` | Delivery | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `CREATE_CAMPAIGN` | Marketing | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |
| `GET_SELLER_REPORT` | Marketing | ❌ No implementation | ❌ No endpoint | ❌ No UI | ❌ No handler | ❌ | **NOT_IMPLEMENTED** |

---

## Summary

| Status | Count | Action Types |
|--------|-------|-------------|
| **IMPLEMENTED** | 11 | SEARCH_LISTINGS, GET_LISTING, SUGGEST, CREATE_LISTING, PUBLISH_LISTING, PAUSE_LISTING, RESUME_LISTING, CREATE_ORDER, GET_ORDER, CANCEL_ORDER |
| **NOT_IMPLEMENTED** | 7 | COMPARE_LISTINGS, UPDATE_LISTING, CHECK_INVENTORY, INITIATE_PAYMENT, GET_PAYMENT_STATUS, GET_DELIVERY_STATUS, CREATE_CAMPAIGN |

---

## Layer Coverage

| Layer | Implemented | Not Implemented |
|-------|------------|-----------------|
| **Domain** | All 17 action types defined in `intent.go` | — |
| **Application (Handlers)** | 10 handlers in `commerce/` | 7 action types without handlers |
| **Commerce Runtime** | All handlers registered via `CommerceActionRuntime` | — |
| **REST API** | 6 endpoints in `internal/api/` | 11 action types without API endpoints |
| **WhatsApp** | 10 action types via service | 7 action types without WhatsApp handlers |
| **Web** | Search, Listing, Orders, Seller | Compare, Ask, Payment, Delivery |
