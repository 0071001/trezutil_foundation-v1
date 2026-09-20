# Sprint 11 — Search Contract

## Overview

This document defines the search architecture, query model, and result contracts for the Trezutil Commerce OS.

## Design Principles

1. **Channel-neutral** — Same query/result for REST, WhatsApp, Mobile, Partner API
2. **Engine-agnostic** — `SearchEnginePort` interface abstracts OpenSearch, PostgreSQL, in-memory
3. **Visibility-first** — Authorization resolved before query execution
4. **Tenant-isolated** — Results scoped to actor's tenant/organization
5. **Extensible** — `DiscoveryQuery` evolves without breaking existing callers

---

## Discovery Query

The canonical search request. Independent of transport, engine, or AI provider.

```go
type DiscoveryQuery struct {
    Text         string                // Free-text query
    Category     *CategoryFilter       // Category + children inclusion
    Attributes   []AttributeFilter     // Typed attribute filters
    PriceRange   *PriceRange           // Min/max price
    Currency     string                // ISO 4217 for price filtering
    Availability *SearchAvailabilityStatus  // Filter by stock status
    Location     *SearchLocation       // Origin for geo search
    RadiusMeters *float64              // Search radius
    SellerID     string                // Filter by seller
    StoreID      string                // Filter by store
    Delivery     *DeliveryRequirements // Delivery constraints
    ListingType  *SearchListingType    // physical_product, service, etc.
    Sort         *SearchSort           // Field + direction
    Pagination   PaginationCursor      // Cursor-based pagination
}
```

### Filter Details

| Filter | Type | Description |
|--------|------|-------------|
| `Category` | `CategoryFilter` | Category ID + `IncludeChildren` |
| `Attributes` | `[]AttributeFilter` | Key + Operator (`eq`, `neq`, `gt`, `gte`, `lt`, `lte`, `in`, `contains`, `search`) + Value |
| `PriceRange` | `PriceRange` | `Min`/`Max` in minor units |
| `Availability` | `SearchAvailabilityStatus` | `in_stock`, `out_of_stock`, `pre_order`, `reserved`, `available`, `unavailable` |
| `Location` | `SearchLocation` | `LocationID` or `Latitude`/`Longitude` + `Address` |
| `RadiusMeters` | float64 | Geo radius from location |
| `Sort` | `SearchSort` | Field: `relevance`, `distance`, `price_asc`, `price_desc`, `newest`, `oldest`, `availability` |

### Pagination

```go
type PaginationCursor struct {
    Mode     PaginationMode  // "cursor" or "offset"
    Offset   *int
    Cursor   string          // Base64-encoded
    PageSize int             // Default 10, max 100
}
```

---

## Search Result

Channel-neutral projection — NOT the full Listing entity.

```go
type SearchResult struct {
    ListingID          string
    Title              string
    PriceAmount        float64
    PriceCurrency      string
    AvailabilityStatus string
    DistanceMeters     *float64
    Score              float64
    MatchedAttributes  []MatchedAttribute
    Location           *SearchResultLocation
    Seller             *SearchResultSeller
    Store              *SearchResultStore
    PrimaryMediaURL    *string
    PublishedAt        int64
}
```

### Enrichment Objects

```go
type MatchedAttribute struct {
    Key   string
    Value interface{}
}

type SearchResultLocation struct {
    City         *string
    Neighborhood *string
}

type SearchResultSeller struct {
    SellerID    string
    DisplayName string
    TrustLevel  int
}

type SearchResultStore struct {
    StoreID string
    Name    string
}
```

### Result Page

```go
type SearchResultPage struct {
    Items               []SearchResult
    Total               int
    HasNext             bool
    NextCursor          *string
    QueryInterpretation *QueryInterpretation  // Future: AI-injected
}
```

---

## Search Engine Port

```go
type SearchEnginePort interface {
    ExecuteSearch(ctx context.Context, query DiscoveryQuery) (SearchResultPage, error)
}
```

### Implementations

| Engine | Status | Use Case |
|--------|--------|----------|
| `InMemorySearchEngine` | ✅ Testing | Unit tests, local dev |
| `OpenSearchEngine` | 🔄 Infra ready, service unavailable | Production |
| PostgreSQL Full-Text | ❌ Not implemented | Fallback |

**Current:** `InMemorySearchEngine` stub in `internal/search/application/search_service.go`

---

## Search Use Cases

```go
type SearchUseCases interface {
    ExecuteSearch(ctx context.Context, actorID string, query DiscoveryQuery) (SearchResultPage, error)
    Suggest(ctx context.Context, actorID string, req SuggestRequest) (SuggestResponse, error)
}
```

### ExecuteSearch Flow

```
1. ResolveVisibility(actorID) → SearchVisibility
2. AuthorizeSearch(actorID, visibility)
3. Inject visibility constraints into query:
   - query.SellerID = actorID
   - if visibility.OrganizationID != nil: query.StoreID = *visibility.OrganizationID
4. ExecuteSearch via SearchEnginePort
5. Return SearchResultPage
```

### Visibility Modes

| Mode | Description |
|------|-------------|
| `public` | Public listings only |
| `internal` | Actor's organization listings + public |

---

## Suggest / Autocomplete

```go
type SuggestRequest struct {
    Text       string
    CategoryID *string
    Limit      int  // default 10, max 50
}

type SuggestResponse struct {
    Suggestions []Suggestion
}

type Suggestion struct {
    Text       string
    CategoryID *string
    Type       string  // "category", "listing", "attribute"
}
```

---

## REST API

### POST /v1/search

```json
{
  "text": "iphone 15",
  "category_id": "cat-electronics",
  "page_size": 20,
  "cursor": "eyJvZmZzZXQiOjIwfQ==",
  "sort_field": "price_asc",
  "sort_direction": "asc"
}
```

### GET /v1/search/suggest

```
GET /v1/search/suggest?text=iph&category_id=cat-electronics&limit=10
```

---

## WhatsApp Integration

Same `SearchUseCases.ExecuteSearch` called via `SearchHandler` in Commerce Runtime.

```go
// In SearchHandler.Handle
query := domain.DiscoveryQuery{
    Text:          parameters["text"].(string),
    SellerID:      parameters["seller_id"].(string),
    Pagination:    domain.PaginationCursor{Cursor: parameters["page"].(string)},
}
result, _ := h.searchUseCases.ExecuteSearch(ctx, actor.ID, query)
```

Response formatted via `ConversationResponse` → WhatsApp Presenter.

---

## Visibility & Authorization

### VisibilityResolver

```go
type VisibilityResolver interface {
    ResolveVisibility(ctx context.Context, actorID string) (SearchVisibility, error)
}
```

Returns:
- `Mode`: `public` or `internal`
- `OrganizationID`: Actor's org (if internal)

### AuthorizationResolver

```go
type AuthorizationResolver interface {
    AuthorizeSearch(ctx context.Context, actorID string, visibility SearchVisibility) error
}
```

### Injection

Visibility constraints injected into query before engine execution:
- `query.SellerID = actorID`
- `query.StoreID = visibility.OrganizationID` (if internal mode)

---

## Indexing

### SearchIndexPort

```go
type SearchIndexPort interface {
    IndexListingDocument(ctx context.Context, document interface{}) error
    DeleteListingDocument(ctx context.Context, listingID string) error
}
```

### Document Projection

`ListingProjection` — denormalized for search:

```go
type ListingProjection struct {
    ListingID          string
    OrganizationID     string
    SellerID           string
    StoreID            string
    Title              string
    Description        string
    CategoryID         string
    CategoryPath       []string
    ListingType        string
    PricingModel       string
    PriceAmount        float64
    PriceCurrency      string
    AvailabilityStatus string
    Quantity           *int
    Location           *GeoPoint
    LocationIDs        []string
    Attributes         []AttributeProjection
    MediaCount         int
    PrimaryMediaURL    *string
    SellerDisplayName  string
    SellerTrustLevel   int
    StoreName          string
    StoreStatus        string
    Status             string
    PublishedAt        int64
    UpdatedAt          int64
    FreshnessScore     float64
    Metadata           map[string]interface{}
}
```

---

## OpenSearch Implementation

### Architecture

```
SearchService
    ↓
OpenSearchAdapter (implements SearchEnginePort)
    ↓
OpenSearch Client (REST/HTTP)
    ↓
OpenSearch Cluster
```

### Query Builder

Translates `DiscoveryQuery` → OpenSearch DSL:
- `multi_match` for text
- `term`/`terms` for exact filters
- `range` for price, distance
- `geo_distance` for location
- `nested` for attributes

### Current Blockers

| Blocker | Impact |
|---------|--------|
| OpenSearch service unavailable | All integration tests fail |
| No local OpenSearch in CI | Cannot validate DSL generation |

### Workaround

`InMemorySearchEngine` stub used for unit tests and local development.

---

## Testing

| Test Type | Coverage |
|-----------|----------|
| Unit (`SearchService`) | ✅ `search_service_test.go` |
| Unit (`InMemorySearchEngine`) | ✅ |
| Unit (`OpenSearch` query builder) | ✅ `execution_test.go`, `config_test.go` |
| Integration (`OpenSearch` adapter) | ⚠️ Blocked (service unavailable) |
| REST API (`SearchHandler`) | ✅ `search_handlers_test.go` |

---

## Future: AI Query Interpretation

```go
type QueryInterpretation struct {
    OriginalText     string
    CategoryInferred *string
    LocationInferred *string
}
```

- Injected into `SearchResultPage.QueryInterpretation`
- Generated by future AI Orchestrator
- Optional — not required for basic search

---

## OpenSearch Dependency

| Environment | Status |
|-------------|--------|
| Local dev | Use `InMemorySearchEngine` |
| CI | Use `InMemorySearchEngine` |
| Staging | Requires OpenSearch cluster |
| Production | Requires OpenSearch cluster |

**Do not redesign search solely because OpenSearch is unavailable.**