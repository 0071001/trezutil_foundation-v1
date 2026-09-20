# Trezutil — Sprint 4 Search & Discovery Architecture Report

## 1. Architecture Decision

**Dedicated `search-service` within the Commerce Core.**

Search & Discovery is a read-model service that derives a search projection from authoritative PostgreSQL sources via domain events. It does not own transactional state.

The service exposes REST APIs for external clients and gRPC for internal consumers (Conversation Engine, AI layer).

## 2. Search Service Boundary

**Service:** `search-service`

**Responsibilities:**
* Accept and parse `DiscoveryQuery` from REST and gRPC
* Orchestrate PostgreSQL (exact filters, tenant validation) and OpenSearch (full-text, geo, ranking)
* Maintain search index synchronization via domain events
* Enforce visibility rules and tenant isolation
* Return `SearchResult` pages

**Not responsible for:**
* WhatsApp adapters
* Conversation state
* AI / LLM orchestration
* Order management
* Payments
* Logistics

## 3. DiscoveryQuery Model

A canonical, channel-independent query model with the following fields:

| Field | Type | Purpose |
|-------|------|---------|
| `text` | `string` | Full-text search |
| `category` | `CategoryFilter` | Category filter with optional children inclusion |
| `attributes` | `AttributeFilter[]` | Typed attribute filters with operators |
| `price_range` | `PriceRange` | Min/max price bounds |
| `currency` | `string` | Currency filter |
| `availability` | `string` | Availability status filter |
| `location` | `SearchLocation` | Origin for geo queries |
| `radius_meters` | `number` | Search radius |
| `seller_id` | `string` | Seller filter |
| `store_id` | `string` | Store filter |
| `delivery` | `DeliveryRequirements` | Delivery constraints (reserved) |
| `listing_type` | `string` | Listing type filter |
| `sort` | `Sort` | Sort field and direction |
| `pagination` | `Pagination` | Cursor or offset pagination |

**Channel mapping:**
* REST query parameters / JSON body → `DiscoveryQuery`
* gRPC message → `DiscoveryQuery`
* Conversational AI output → `DiscoveryQuery`

All three map to the same canonical model.

## 4. SearchResult Model

A projection, not the authoritative domain entity.

```text
SearchResult
├── listing_id
├── title
├── price_amount
├── price_currency
├── availability_status
├── distance_meters
├── score
├── matched_attributes
├── location (city, neighborhood)
├── seller (seller_id, display_name, trust_level)
├── store (store_id, name)
├── primary_media_url
└── published_at
```

`SearchResultPage` wraps results with pagination metadata and optional `query_interpretation` for AI-injected context.

## 5. Search Index Model

### Document Structure

`SearchListingDocument` is a flattened, denormalized projection built from:

| Source Table | Document Fields |
|--------------|-----------------|
| `listings` | listing_id, organization_id, seller_id, store_id, title, description, category_id, listing_type, pricing_model, price_amount, price_currency, status, published_at, updated_at |
| `categories` | category_path (hierarchical) |
| `listing_availability` | availability_status, quantity |
| `listing_locations` + `locations` | location (geo_point), location_ids |
| `listing_attributes` + `attribute_definitions` | attributes (searchable only) |
| `listing_media` | media_count, primary_media_url |
| `seller_profiles` | seller_display_name, seller_trust_level |
| `stores` | store_name, store_status |

### Source of Truth

PostgreSQL remains authoritative. The search index is a derived read model.

### Index Synchronization

Event-driven, asynchronous, eventually consistent.

**Events that affect the index:**
* `ListingCreated` — wait for `ListingPublished`
* `ListingUpdated` — reindex if published
* `ListingPublished` — index or make visible
* `ListingPaused` — remove from public search
* `ListingSuspended` — remove from all search
* `ListingSold` / `ListingClosed` / `ListingExpired` — remove from public search
* `ListingAvailabilityChanged` — update availability fields
* `ListingLocationUpdated` — update geo fields
* `CategoryUpdated` — reindex affected listings
* `AttributeDefinitionUpdated` — reindex affected listings

**Guarantees:**
* At-least-once delivery
* Idempotent by `listing_id`
* Ordered per listing
* Dead-letter queue for failures

## 6. Geo Architecture

**Decision: Hybrid PostgreSQL + OpenSearch with denormalization.**

* PostgreSQL `locations` is the source of truth for coordinates and hierarchy.
* OpenSearch contains a denormalized copy for fast geo queries.
* `UserLocation` is NEVER exposed to Search.
* `SearchLocation` is client-provided per query.
* `ListingLocation` is the only source of position for listings in the search index.
* `DeliveryArea` is reserved for future logistics integration.

### Geo Query Flow

```
SearchLocation + Radius
    ↓
Search Service
    ├── PostgreSQL : tenant, category, price, availability filters
    └── OpenSearch  : geo_filter + full-text + ranking
         ↓
    SearchResult (with distance_meters)
```

### Distance Ranking

Distance is a ranking signal, not a dominant filter. A listing at maximum radius can still win if much more relevant.

## 7. Filtering

**Filtering (Filtering)** eliminates candidates that do not match exact criteria.

**Ranking (Ranking)** orders remaining candidates by relevance and signals.

This distinction is explicit.

### Supported Filters

| Filter | Type | Source |
|--------|------|--------|
| Category | exact + hierarchy | PostgreSQL + OpenSearch |
| Listing Type | enum | PostgreSQL |
| Price Range | range | PostgreSQL |
| Currency | exact | PostgreSQL |
| Availability | enum | PostgreSQL |
| Attributes | typed operators | OpenSearch |
| Location + Radius | geo | OpenSearch |
| Seller | exact | PostgreSQL |
| Store | exact | PostgreSQL |
| Status | exact | PostgreSQL |

## 8. Ranking

Multi-phase scoring:

1. **Filtering** — exact criteria elimination
2. **Relevance** — BM25 full-text + category/attribute matching
3. **Ranking Signals** — distance, price, freshness, trust, availability, media richness

### Signals

| Signal | Weight | Notes |
|--------|--------|-------|
| Text relevance | base | BM25 from OpenSearch |
| Category match | +10% | Exact category match |
| Attribute match | +5% per match | Searchable attributes |
| Distance penalty | up to -30% | Linear decay within radius |
| Price competitiveness | configurable | Relative to category median |
| Freshness | time decay | Logarithmic decay from published_at |
| Seller trust | +2% per level | `trust_level` field |
| Availability | +5% | In-stock bonus |
| Media richness | +1% per item | Up to a cap |

### Tie Breaking

1. `published_at` descending
2. `listing_id` ascending

## 9. Visibility & Lifecycle

| Status | Public Search | Internal Search |
|--------|---------------|-----------------|
| `draft` | Excluded | Seller/org only |
| `pending_review` | Excluded | Seller/org only |
| `published` | Included | Included |
| `paused` | Excluded | Seller/org only |
| `suspended` | Excluded | Platform admin only |
| `sold` | Excluded | Seller/org only |
| `closed` | Excluded | Seller/org only |
| `expired` | Excluded | Seller/org only |

Public search always filters by `status = PUBLISHED`.

Internal search includes `organization_id` in the filter and validates actor membership in the application layer.

## 10. Multi-Tenant Security

Search must never expose cross-tenant data.

**Public search:** No tenant context. Only published listings visible. No `organization_id` in results.

**Internal search:** Authenticated. `organization_id` is always included in the query filter. Application layer verifies:
* Seller owner → own listings only
* Store staff → own store listings only
* Org admin/owner → all org listings
* Platform admin/operator → all listings

**UserLocation** is never indexed or exposed through search.

## 11. API Contracts

### POST /v1/search

Primary search endpoint. Accepts `DiscoveryQuery` in request body.

### GET /v1/search/suggest

Autocomplete endpoint. Accepts `SuggestRequest` with text and optional category.

### Public vs Authenticated

* Unauthenticated: public search only (`PUBLISHED` listings)
* Authenticated: internal search with tenant context

See `API_CONTRACT.md` for full schema.

## 12. Internal Contracts

Search Service exposes:

* **REST** — for web/admin clients
* **gRPC** — for internal service-to-service communication

```protobuf
service SearchService {
  rpc Search(DiscoveryQuery) returns (SearchResultPage);
  rpc Suggest(SuggestRequest) returns (SuggestResponse);
}
```

`DiscoveryQuery` and `SearchResult` are defined as protobuf messages matching the domain model.

No generated code is created in this sprint.

## 13. Event Contracts

See `EVENT_CONTRACT.md` for full event definitions.

Key events consumed by Search:

| Event | Action |
|-------|--------|
| `ListingCreated` | Do not index yet |
| `ListingUpdated` | Reindex if published |
| `ListingPublished` | Index or make visible |
| `ListingPaused` | Remove from public search |
| `ListingSuspended` | Remove from all search |
| `ListingSold` | Remove from public search |
| `ListingClosed` | Remove from public search |
| `ListingExpired` | Remove from public search |
| `ListingAvailabilityChanged` | Update availability fields |
| `ListingLocationUpdated` | Update geo fields |
| `CategoryUpdated` | Reindex affected listings |
| `AttributeDefinitionUpdated` | Reindex affected listings |

All events carry:
* `event_id` (globally unique)
* `event_name`
* `aggregate_id`
* `version`
* `occurred_at`
* `payload`

## 14. Pagination

**Recommendation: Cursor-based pagination as primary mechanism.**

Reasons:
* Stable ordering even as listings change
* Better performance for large offsets
* Consistent with search engine capabilities
* Suitable for conversational interfaces (WhatsApp)

Offset pagination is supported for backward compatibility but not recommended.

## 15. Reindexing

Full reindex strategy:

1. **Trigger:** manual, scheduled, or after index corruption
2. **Source:** PostgreSQL (authoritative)
3. **Batching:** configurable batch size (recommended 1000 listings)
4. **Resume:** checkpoint by `listing_id` range
5. **Idempotency:** write-by-`listing_id` ensures safe retries
6. **Versioning:** write to new index version, then atomically switch alias
7. **Verification:** document count and sample validation before cutover
8. **Rollback:** revert alias to previous version

## 16. Failure Handling

| Scenario | Behavior |
|----------|----------|
| Search engine unavailable | Return `503 SEARCH_UNAVAILABLE` |
| Index outdated | Results may be stale; document eventual consistency |
| Indexing event delayed | Listing appears in search with bounded delay |
| Listing deleted | Removed from index via event |
| Listing paused | Removed from public search via event |
| PostgreSQL available, search unavailable | Transactional operations unaffected; search returns 503 |
| Search available, event processing delayed | Index temporarily stale; eventual consistency |

**Strongly consistent:** PostgreSQL reads, authorization checks, visibility filters.

**Eventually consistent:** Search index, search results.

## 17. Observability

### Metrics

```
search_requests_total
search_latency_seconds
search_failures_total
search_zero_result_queries_total
index_lag_seconds
indexing_failures_total
reindex_duration_seconds
```

### Logs

* `actor_id` (if authenticated)
* `organization_id` (if applicable)
* `query_hash` (not raw query text)
* `result_count`
* `latency`
* `timestamp`

### Traces

* Correlation ID on every request
* Trace spans for: query parsing, PostgreSQL filters, OpenSearch query, ranking, response assembly

## 18. Security Threat Model

| Threat | Control |
|--------|---------|
| Cross-tenant search leakage | Always filter by `organization_id` in internal search; public search filters by `status = PUBLISHED` |
| Private listing exposure | Public search excludes non-published statuses |
| Location privacy | `UserLocation` never indexed or exposed |
| Seller data leakage | Search results expose only display name and trust level, not contact info |
| Malicious query parameters | Query validation, operator whitelist, coordinate bounds checks |
| Search abuse | Rate limiting per IP and per user |
| Enumeration | Pagination cursors are opaque; no sequential IDs exposed |
| DoS through expensive queries | Timeout budgets, query complexity limits, circuit breakers |

## 19. ADRs Created

| ADR | Title |
|-----|-------|
| ADR 0009 | Search Service Boundary |
| ADR 0010 | Search Engine Selection (Hybrid PostgreSQL + OpenSearch) |
| ADR 0011 | Geo Search Architecture |
| ADR 0012 | Ranking Architecture |

## 20. Documentation Created

```
docs/sprint-4/README.md
docs/sprint-4/SEARCH_ARCHITECTURE.md
docs/sprint-4/DISCOVERY_QUERY.md
docs/sprint-4/SEARCH_INDEX.md
docs/sprint-4/SEARCH_SECURITY.md
docs/sprint-4/API_CONTRACT.md
docs/sprint-4/EVENT_CONTRACT.md
docs/sprint-4/ADR/0009-search-service-boundary.md
docs/sprint-4/ADR/0010-search-engine-selection.md
docs/sprint-4/ADR/0011-geo-search-architecture.md
docs/sprint-4/ADR/0012-ranking-architecture.md
```

## 21. Open Questions

1. OpenSearch deployment topology (cluster size, shard strategy, RDC-specific considerations)
2. Exact BM25 configuration and language analyzers for French, English, Swahili, Lingala
3. Final ranking weights — to be tuned with real data and business rules
4. Geocoding strategy for free-text addresses in `SearchLocation`
5. Caching strategy for popular queries
6. Conversation Engine ↔ Search Service protocol details
7. AI Query Understanding module boundaries (outside this sprint)

## 22. Implementation Status

```text
NO SEARCH IMPLEMENTATION HAS BEEN CREATED.
NO OPENSEARCH/ELASTICSEARCH IMPLEMENTATION HAS BEEN CREATED.
NO AI IMPLEMENTATION HAS BEEN CREATED.
NO WHATSAPP IMPLEMENTATION HAS BEEN CREATED.
```

This sprint is strictly architecture, domain model, contracts, ADRs, and documentation. No production code has been written.
