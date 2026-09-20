# Search & Discovery Architecture

## 1. Architectural Position

Search & Discovery is a **read-model service** within the Commerce Core.

It does not own transactional state. It derives a search projection from the authoritative PostgreSQL sources via domain events.

```
PostgreSQL (source of truth)
    ↓
Domain Events
    ↓
Search Indexer
    ↓
Search Index / Query Layer
    ↓
Search & Discovery Engine
    ↓
SearchResult
```

## 2. Service Boundary

Search & Discovery is implemented as a dedicated internal service: `search-service`.

### Why a dedicated service?

* Search has distinct scaling, availability, and latency requirements from transactional catalog operations.
* Search indexing is asynchronous and eventually consistent; mixing it with transactional handlers adds failure surface.
* Search needs its own query parsing, geo computation, ranking, and caching concerns.
* Future extraction into an independently deployable service is easier if the boundary is explicit from the start.
* Conversation Engine and AI layer depend on Search; Search must not depend on them.

### What Search Service does NOT include

* WhatsApp adapters
* Conversation state
* AI / LLM orchestration
* Order management
* Payments
* Logistics

## 3. High-Level Component Model

```text
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│   REST /     │────▶│  Query       │────▶│  Filter /      │
│   gRPC      │     │  Parser      │     │  Validator     │
└─────────────┘     └──────────────┘     └───────┬────────┘
                                                  │
                                                  ▼
                                         ┌────────────────┐
                                         │  DiscoveryQuery │
                                         │  (canonical)    │
                                         └───────┬────────┘
                                                 │
                              ┌──────────────────┼──────────────────┐
                              ▼                  ▼                  ▼
                    ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
                    │  PostgreSQL   │  │  Search      │  │  Cache       │
                    │  Exact Filters│  │  Index       │  │  Layer       │
                    └──────────────┘  └──────────────┘  └──────────────┘
                              │                  │
                              └──────────────────┘
                                                 ▼
                                         ┌────────────────┐
                                         │  Ranker        │
                                         │  (deterministic│
                                         │   signals)     │
                                         └───────┬────────┘
                                                 │
                                                 ▼
                                         ┌────────────────┐
                                         │  SearchResult  │
                                         │  Page          │
                                         └────────────────┘
```

## 4. Source of Truth vs Search Projection

PostgreSQL is the authoritative source for all business data.

The search index is a **derived projection** optimized for retrieval.

The following data lives in PostgreSQL only:
* transactional listings, categories, attributes, media, availability, locations
* organizations, sellers, stores, users
* orders, payments, conversations
* audit logs

The search index contains a **flattened, denormalized, search-optimized projection** of published listings and related reference data.

## 5. Query Processing Flow

```text
Incoming Request
    ↓
Authentication / Tenant Context
    ↓
Query Parsing (REST params / natural language → DiscoveryQuery)
    ↓
DiscoveryQuery Validation
    ↓
Visibility Filtering (what statuses are visible to this actor)
    ↓
Exact Filters (PostgreSQL: category, price, availability, tenant)
    ↓
Full-Text / Geo / Attribute Filters (Search Index)
    ↓
Candidate Set
    ↓
Ranking (deterministic signals)
    ↓
Pagination / Sorting
    ↓
SearchResult Page
```

## 6. Channel Independence

The Search & Discovery Engine exposes:

* REST API for web/admin clients
* Internal gRPC for Conversation Engine, AI layer, and other services

It does not depend on:
* WhatsApp SDKs
* Telegram adapters
* Any specific UI framework

Natural language input from conversational channels is **translated into a structured `DiscoveryQuery` before reaching Search**. Search never consumes raw LLM output.

## 7. Consistency Model

| Data | Consistency |
|------|-------------|
| PostgreSQL writes | Strongly consistent |
| Search index | Eventually consistent |
| Search results | Eventually consistent |
| Cache | Eventually consistent |

A newly published listing may take a bounded amount of time to appear in search results. This is acceptable and must be documented in API semantics.

## 8. Degradation Strategy

If the search index is unavailable:
* The API must fail gracefully with an explicit error or degraded mode.
* It must NOT fall back to raw PostgreSQL full-table scans in production without an explicit circuit-breaker and timeout budget.
* Conversational channels should receive a graceful "search temporarily unavailable" response.

PostgreSQL remains available for transactional operations regardless of search availability.

## 9. Relationship to Conversation Engine

```
User
    ↓
Conversation Engine
    ↓
AI / Query Understanding
    ↓
DiscoveryQuery (structured)
    ↓
Search & Discovery Engine
    ↓
SearchResult
    ↓
Conversation Engine (formats response for channel)
```

The Conversation Engine owns:
* conversation state
* message formatting
* channel-specific UX
* human handoff

Search & Discovery owns:
* query execution
* candidate generation
* filtering
* ranking
* pagination

## 10. Non-Goals for Sprint 4

* OpenSearch deployment
* OpenSearch client implementation
* Indexer implementation
* Kafka producer/consumer implementation
* AI / LLM query parsing implementation
* WhatsApp integration
* Order engine
* Payment engine
* Logistics

Sprint 4 deliverables are architecture, domain models, contracts, ADRs, and documentation only.
