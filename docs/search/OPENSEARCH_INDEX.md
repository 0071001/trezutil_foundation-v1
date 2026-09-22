# OpenSearch Index

## Index Structure

### Default Index

| Property | Value |
|---|---|
| Index name | `trezutil-listings-v1` |
| Alias | `trezutil-listings` |
| Version | `v1` |

### Field Mapping

The complete mapping is defined in `internal/search/infrastructure/opensearch/index.go`
at `IndexManager.buildMapping()`.

#### Core Fields

| Field | Type | Description |
|---|---|---|
| `listing_id` | `keyword` | Unique listing identifier (document ID) |
| `organization_id` | `keyword` | Tenant/organization ID |
| `seller_id` | `keyword` | Seller identifier |
| `store_id` | `keyword` | Store identifier |
| `title` | `text` (with `keyword` sub-field) | Listing title, boosted x3 in search |
| `description` | `text` | Listing description |
| `category_id` | `keyword` | Direct category ID |
| `category_path` | `keyword` (array) | Full category hierarchy |
| `listing_type` | `keyword` | Physical product, real estate, vehicle, etc. |
| `pricing_model` | `keyword` | Fixed, auction, etc. |
| `price_amount` | `scaled_float` (scale=100) | Price as fixed-point |
| `price_currency` | `keyword` | ISO 4217 currency code |
| `availability_status` | `keyword` | In stock, out of stock, etc. |
| `quantity` | `integer` | Available quantity |
| `location` | `geo_point` | Geographic coordinates (lat/lon) |
| `location_ids` | `keyword` (array) | Location IDs for filtering |
| `status` | `keyword` | Published, draft, archived, deleted, hidden |
| `published_at` | `date` | Publication timestamp |
| `updated_at` | `date` | Last update timestamp |

#### Nested: Attributes

| Field | Type | Description |
|---|---|---|
| `attributes.key` | `keyword` | Attribute key |
| `attributes.name` | `text` | Attribute display name |
| `attributes.type` | `keyword` | Attribute value type |
| `attributes.value_text` | `text` | Text value (searchable) |
| `attributes.value_number` | `scaled_float` (scale=100) | Numeric value |
| `attributes.value_boolean` | `boolean` | Boolean value |
| `attributes.value_date` | `date` | Date value |
| `attributes.value_select` | `keyword` | Select value |
| `attributes.searchable` | `boolean` | Whether attribute is searchable |

#### Store/Seller Fields

| Field | Type | Description |
|---|---|---|
| `seller_display_name` | `text` (with `keyword` sub-field) | Seller display name |
| `seller_trust_level` | `byte` | Seller trust level (0-100) |
| `store_name` | `text` (with `keyword` sub-field) | Store name |
| `store_status` | `keyword` | Store status (active, suspended) |
| `media_count` | `integer` | Number of media items |
| `primary_media_url` | `keyword` (not indexed) | URL of primary media |

#### Metadata

| Field | Type | Description |
|---|---|---|
| `metadata` | `object` (disabled) | Arbitrary metadata, not indexed |
| `freshness_score` | `scaled_float` (scale=1000) | Freshness ranking score |

## Search DSL

The query DSL is built by `QueryBuilder.buildSearchQuery()` in
`internal/search/infrastructure/opensearch/query_builder.go`.

### Bool Query Structure

```json
{
  "query": {
    "bool": {
      "must": [
        {
          "multi_match": {
            "query": "search text",
            "fields": ["title^3", "description", "category_path^2", ...],
            "type": "best_fields"
          }
        }
      ],
      "filter": [
        {"term": {"status": "published"}},
        {"term": {"category_id": "cat-1"}},
        {"range": {"price_amount": {"gte": 100, "lte": 500}}},
        {"term": {"price_currency": "USD"}},
        {"term": {"listing_type": "physical_product"}},
        {"nested": {...}},
        {"geo_distance": {...}}
      ],
      "should": [
        {"geo_distance": {..., "boost": 1.2}},
        {"term": {"category_path": {"value": "cat-1", "boost": 1.1}}}
      ],
      "minimum_should_match": 0
    }
  },
  "sort": [...],
  "size": 20,
  "from": 0,
  "track_total_hits": true
}
```

### Refresh Semantics

- `IndexDocument`: Uses `refresh=wait_for` — document is immediately searchable
- `DeleteDocument`: Uses `refresh=wait_for` — deletion is immediately visible
- `BulkIndexDocuments`: Uses `refresh=wait_for` — all documents immediately searchable

## Index Lifecycle

### Bootstrap

1. `EnsureIndex(ctx)` checks if the alias exists via `HEAD /{alias}`
2. If not found, creates the index with mapping via `PUT /{index}`
3. Associates the alias with the index via `PUT /{index}/_alias/{alias}`

### Versioning

For zero-downtime reindexing:
1. `CreateVersionedIndex(ctx, "v2")` creates `trezutil-listings-v2`
2. Index documents into the new version
3. `SwitchAlias(ctx, oldIndex, newIndex)` atomically switches the alias

### Cleanup

Test indices are cleaned up automatically. For manual cleanup:

```bash
# Delete a specific index
curl -X DELETE http://localhost:9200/trezutil-listings-v1

# Delete by pattern (use with caution)
curl -X DELETE "http://localhost:9200/trezutil-integration-*"
```
