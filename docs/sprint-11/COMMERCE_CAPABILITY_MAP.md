# Sprint 11 — Commerce Capability Map

## Overview

This document maps every commerce capability available in Trezutil, across all channels.

---

## Capability Matrix

| Capability | Domain | Application Use Case | Commerce Action | REST API | Web | WhatsApp | Persistence | Usage Policy | Status | Tests |
|---|---|---|---|---|---|---|---|---|---|---|
| **Search Listings** | Catalog | `SearchUseCases.ExecuteSearch` | `SEARCH_LISTINGS` | `POST /v1/search` | ✅ Search page | ✅ Via `SearchHandler` | OpenSearch (in-memory stub) | Per-search credits | IMPLEMENTED | ✅ Unit |
| **Suggest** | Catalog | `SearchUseCases.Suggest` | `SUGGEST` | `GET /v1/search/suggest` | ✅ Search input | ✅ Via `SuggestHandler` | In-memory | Per-suggest credits | IMPLEMENTED | ✅ Unit |
| **View Listing** | Catalog | `ListingUseCases.GetListing` | `GET_LISTING` | `GET /v1/listings/{id}` | ✅ Listing Detail | ✅ Via `GET_LISTING` action | PostgreSQL | Read credits | IMPLEMENTED | ✅ Unit |
| **Create Listing** | Catalog | `ListingUseCases.CreateListing` | `CREATE_LISTING` | `POST /v1/listings` | ✅ Create Listing | ✅ Via `CreateListingHandler` | PostgreSQL | Write credits | IMPLEMENTED | ✅ Unit |
| **Publish Listing** | Catalog | `ListingUseCases.PublishListing` | `PUBLISH_LISTING` | `POST /v1/listings/{id}/publish` | ✅ Seller Dashboard | ✅ Via `PublishListingHandler` | PostgreSQL | Write credits | IMPLEMENTED | ✅ Unit |
| **Pause Listing** | Catalog | `ListingUseCases.PauseListing` | `PAUSE_LISTING` | `POST /v1/listings/{id}/pause` | ✅ Seller Dashboard | ✅ Via `PauseListingHandler` | PostgreSQL | Write credits | IMPLEMENTED | ✅ Unit |
| **Resume Listing** | Catalog | `ListingUseCases.ResumeListing` | `RESUME_LISTING` | `POST /v1/listings/{id}/resume` | ✅ Seller Dashboard | ✅ Via `ResumeListingHandler` | PostgreSQL | Write credits | IMPLEMENTED | ✅ Unit |
| **Compare Listings** | Catalog | N/A | `COMPARE_LISTINGS` | N/A | ❌ Not yet | ❌ Not yet | N/A | N/A | NOT_IMPLEMENTED | ❌ |
| **Check Availability** | Catalog | N/A (boundary) | N/A | N/A | ❌ Not yet | ❌ Not yet | N/A | N/A | PARTIAL | ❌ |
| **Create Order** | Order | `OrderUseCases.CreateOrder` | `CREATE_ORDER` | `POST /v1/orders` | ✅ Order | ✅ Via `CreateOrderHandler` | PostgreSQL | Financial credits | IMPLEMENTED | ✅ Unit |
| **Get Order** | Order | `OrderUseCases.GetOrder` | `GET_ORDER` | `GET /v1/orders/{id}` | ✅ Orders | ✅ Via `GetOrderHandler` | PostgreSQL | Read credits | IMPLEMENTED | ✅ Unit |
| **Cancel Order** | Order | `OrderUseCases.CancelOrder` | `CANCEL_ORDER` | `POST /v1/orders/{id}/cancel` | ✅ Orders | ✅ Via `CancelOrderHandler` | PostgreSQL | Financial credits | IMPLEMENTED | ✅ Unit |
| **Ask / Conversation** | Conversation | Conversation infrastructure | N/A | N/A | ❌ Not yet | ✅ WhatsApp | PostgreSQL | Per-message credits | PARTIAL | ✅ Unit |

---

## Category Definitions

| Category | Capabilities |
|---|---|
| **DISCOVER** | Search Listings, Suggest |
| **VIEW** | View Listing |
| **ASK** | Conversation (WhatsApp only this sprint) |
| **COMPARE** | Compare Listings (NOT_IMPLEMENTED) |
| **ACT** | Create Listing, Publish, Pause, Resume, Create Order, Cancel Order |
| **TRACK** | Get Order |
| **BOUNDARY** | Check Availability (generic boundary only) |

---

## Cross-Channel Mapping

| Capability | WhatsApp | REST | Web |
|---|---|---|---|
| Search | ✅ `SEARCH_LISTINGS` action | ✅ `POST /v1/search` | ✅ Search page |
| View Listing | ✅ `GET_LISTING` action | ✅ `GET /v1/listings/{id}` | ✅ Listing Detail |
| Create Listing | ✅ `CREATE_LISTING` action | ✅ `POST /v1/listings` | ✅ Seller Create |
| Create Order | ✅ `CREATE_ORDER` action | ✅ `POST /v1/orders` | ✅ Order flow |
| Get Order | ✅ `GET_ORDER` action | ✅ `GET /v1/orders/{id}` | ✅ Orders page |
| Cancel Order | ✅ `CANCEL_ORDER` action | ✅ `POST /v1/orders/{id}/cancel` | ✅ Orders page |

---

## Persistence

| Storage | Purpose |
|---|---|
| PostgreSQL | Listings, Orders, Conversations, Messages, Channel Accounts, Usage Ledger |
| OpenSearch | Search indexing (in-memory stub, OpenSearch unavailable) |
| In-memory | Suggestion cache, Idempotency cache (non-durable) |
