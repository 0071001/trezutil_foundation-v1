# Search API Contract

## 1. Base

```text
Host: api.trezutil.com
BasePath: /v1
Auth: Bearer JWT (for internal search)
Public: allowed for public search endpoints
```

## 2. Endpoints

### POST /v1/search

Primary search endpoint. Accepts a structured `DiscoveryQuery` in the request body.

```text
Request: DiscoveryQuery
Response: SearchResultPage
Auth: optional (public search if unauthenticated)
```

### GET /v1/search/suggest

Autocomplete / typeahead suggestions.

```text
Request: SuggestRequest
Response: SuggestResponse
Auth: optional
```

## 3. DiscoveryQuery (REST)

```json
{
  "text": "iPhone 15",
  "category": {
    "category_id": "cat-123",
    "include_children": true
  },
  "attributes": [
    {
      "key": "brand",
      "operator": "eq",
      "value": "Apple"
    }
  ],
  "price_range": {
    "min": 100.00,
    "max": 700.00
  },
  "currency": "USD",
  "availability": "in_stock",
  "location": {
    "location_id": "loc-123",
    "latitude": -4.44,
    "longitude": 15.27
  },
  "radius_meters": 5000,
  "seller_id": "seller-123",
  "store_id": "store-123",
  "listing_type": "physical_product",
  "sort": {
    "field": "distance",
    "direction": "asc"
  },
  "pagination": {
    "mode": "cursor",
    "cursor": "opaque-cursor-string",
    "page_size": 20
  }
}
```

## 4. SearchResultPage

```json
{
  "items": [
    {
      "listing_id": "listing-123",
      "title": "iPhone 15 256GB",
      "price_amount": 699.99,
      "price_currency": "USD",
      "availability_status": "in_stock",
      "distance_meters": 1200,
      "score": 0.85,
      "matched_attributes": [
        {
          "key": "brand",
          "value": "Apple"
        }
      ],
      "location": {
        "city": "Kinshasa",
        "neighborhood": "Gombe"
      },
      "seller": {
        "seller_id": "seller-123",
        "display_name": "Tech Store",
        "trust_level": 3
      },
      "store": {
        "store_id": "store-123",
        "name": "Tech Store Kinshasa"
      },
      "primary_media_url": "https://media.example.com/image.jpg",
      "published_at": "2026-09-16T00:00:00Z"
    }
  ],
  "total": 42,
  "has_next": true,
  "next_cursor": "opaque-cursor-string",
  "query_interpretation": {
    "original_text": "iPhone near me",
    "category_inferred": "cat-123",
    "location_inferred": "Gombe"
  }
}
```

### Fields

| Field | Type | Notes |
|-------|------|-------|
| `items` | `SearchResult[]` | Results for current page |
| `total` | `integer` | Total matching results (approximate for search engine) |
| `has_next` | `boolean` | Whether more results exist |
| `next_cursor` | `string \| null` | Opaque cursor for next page |
| `query_interpretation` | `object \| null` | AI-injected interpretation (future) |

## 5. SearchResult

```json
{
  "listing_id": "listing-123",
  "title": "iPhone 15 256GB",
  "price_amount": 699.99,
  "price_currency": "USD",
  "availability_status": "in_stock",
  "distance_meters": 1200,
  "score": 0.85,
  "matched_attributes": [...],
  "location": {...},
  "seller": {...},
  "store": {...},
  "primary_media_url": "...",
  "published_at": "..."
}
```

`SearchResult` is a projection. It is not the authoritative `Listing` domain entity.

## 6. SuggestRequest

```json
{
  "text": "iPh",
  "category_id": "cat-123",
  "limit": 10
}
```

## 7. SuggestResponse

```json
{
  "suggestions": [
    {
      "text": "iPhone 15",
      "category_id": "cat-123",
      "type": "query"
    },
    {
      "text": "iPhone 14",
      "category_id": "cat-123",
      "type": "query"
    }
  ]
}
```

## 8. Errors

Standard error format:

```json
{
  "error": {
    "code": "SEARCH_INVALID_REQUEST",
    "message": "Invalid query parameters",
    "details": []
  }
}
```

| Code | HTTP Status | Meaning |
|------|-------------|---------|
| `SEARCH_INVALID_REQUEST` | 400 | Malformed DiscoveryQuery |
| `SEARCH_UNAVAILABLE` | 503 | Search engine temporarily unavailable |
| `SEARCH_RATE_LIMITED` | 429 | Too many requests |

## 9. Pagination

Cursor-based pagination is the primary mechanism.

Offset pagination is supported for backward compatibility but not recommended for large result sets.

## 10. Sorting

Supported sort fields:

```text
relevance
distance
price_asc
price_desc
newest
oldest
availability
```

Default sort is `relevance` descending.

## 11. Contract Stability

This contract is design-stage. No implementation should be considered stable until:

1. Schema is finalized
2. Authorization rules are defined
3. Error model is agreed
4. Pagination semantics are validated
5. Observability requirements are defined
