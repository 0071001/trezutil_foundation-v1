# OpenSearch Architecture

## Overview

Trezutil uses OpenSearch as the **search projection/index**, providing full-text
search, filtering, sorting, and pagination for listing discovery.

**PostgreSQL = authoritative commerce data**
**OpenSearch = search projection/index**

OpenSearch is NOT the authoritative database. It is a read-only projection
populated by event processing from PostgreSQL. Writes go to PostgreSQL; read
search queries go to OpenSearch.

## Architecture Diagram

```
PostgreSQL (authoritative)
    │
    ▼
Event Processor (events from Kafka or DB changes)
    │
    ▼
SearchListingDocumentMapper → SearchListingDocument
    │
    ▼
OpenSearchAdapter (OpenSearchAdapter)
    ├── IndexManager     — index lifecycle (create, alias, versioning)
    ├── DocumentStore    — document CRUD (index, delete, bulk)
    ├── ExecutionEngine  — search execution (query DSL → results)
    ├── HealthChecker    — health/ready checks
    ├── QueryBuilder     — DiscoveryQuery → OpenSearch DSL
    ├── QueryValidator   — input validation (page size, radius, etc.)
    └── ResultMapper     — OpenSearch hits → SearchResult
    │
    ▼
REST /v1/search
    │
    ▼
Web / WhatsApp / AI Specialists
```

## Components

### OpenSearchAdapter

Implements both `domain.SearchEnginePort` (ExecuteSearch) and
`domain.SearchIndexPort` (IndexListingDocument, DeleteListingDocument).
Single entry point for all search operations.

### IndexManager

Manages index lifecycle:
- `EnsureIndex`: Creates index with mapping if not exists, ensures alias
- `CreateVersionedIndex`: Creates a versioned index (e.g., `trezutil-listings-v2`)
- `SwitchAlias`: Atomically swaps alias between indices (zero-downtime reindexing)
- `GetIndexForAlias`: Returns the concrete index behind an alias

### DocumentStore

Handles document operations:
- `IndexDocument`: Indexes a single document with `refresh=wait_for`
- `DeleteDocument`: Deletes a document by ID with `refresh=wait_for`
- `BulkIndexDocuments`: Bulk indexes multiple documents with `refresh=wait_for`

### ExecutionEngine (execution.go)

Translates `domain.DiscoveryQuery` to OpenSearch DSL and parses results:
- `ExecuteSearch`: Full search execution with cursor pagination
- `Suggest`: Term suggestion queries
- `buildSearchQuery`: Query DSL builder
- `mapHitToSearchResult`: Maps OpenSearch hits to `domain.SearchResult`

### QueryBuilder (query_builder.go)

Builds OpenSearch Query DSL from `DiscoveryQuery`:
- Full-text search via `multi_match` on title, description, category path, etc.
- Filters: status=published, category, listing_type, price range, currency,
  availability, seller, store, attributes (nested), geo distance
- Ranking signals: geo-distance boost, category match boost
- Sorting: score, distance, price asc/desc, newest/oldest

### QueryValidator (query_validator.go)

Validates query parameters before sending to OpenSearch:
- Page size: 1–100
- Text length: ≤ 500 characters
- Radius: 1–50,000 meters
- Sort fields: relevance, distance, price asc/desc, newest, oldest
- Listing types: physical_product, real_estate, vehicle, food, service, other
- Currency: 3 uppercase letters (ISO 4217)
- Maximum 20 filters, 10 attribute filters, 3 sort clauses

### ResultMapper (result_mapper.go)

Maps OpenSearch search hits to domain `SearchResult` objects, including:
- Basic fields: listing_id, title, price, currency, availability
- Nested objects: seller (with display name and trust level), store
- Location: city, neighborhood
- Distance: from geo-distance sort

## Data Flow

1. **Indexing**: PostgreSQL → Event Processor → SearchListingDocument →
   OpenSearchAdapter.IndexListingDocument → OpenSearch

2. **Search**: REST /v1/search → SearchUseCase → DiscoveryQuery →
   OpenSearchAdapter.ExecuteSearch → OpenSearch → SearchResultPage →
   REST response

## Configuration

See `internal/search/infrastructure/opensearch/config.go`.

Configuration is loaded from environment variables in the application layer.
The OpenSearch adapter receives a `Config` struct with URL, credentials,
index name, alias, timeout, and TLS settings.

## Connection

The adapter uses the official `opensearch-go` client library (v2).
Connection is established lazily — no health check on construction.
Health is checked via the `Ping` endpoint and `Health()` method.
