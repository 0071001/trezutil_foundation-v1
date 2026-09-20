# Search Index Model

## 1. Purpose

This document defines the conceptual search document structure and the synchronization strategy between PostgreSQL and the search index.

## 2. Search Document

A search document represents a single searchable listing projection.

```text
SearchListingDocument
├── listing_id: string
├── organization_id: string
├── seller_id: string
├── store_id: string
├── title: string
├── description: string
├── category_id: string
├── category_path: string[]
├── listing_type: string
├── pricing_model: string
├── price_amount: number
├── price_currency: string
├── availability_status: string
├── quantity: number | null
├── location: GeoPoint
├── location_ids: string[]
├── attributes: AttributeProjection[]
├── media_count: number
├── primary_media_url: string | null
├── seller_display_name: string
├── seller_trust_level: number
├── store_name: string
├── store_status: string
├── status: string
├── published_at: timestamp
├── updated_at: timestamp
├── freshness_score: number
└── metadata: map<string, any>
```

### Field Semantics

| Field | Source | Purpose |
|-------|--------|---------|
| `listing_id` | listings.id | Document identity |
| `organization_id` | listings.organization_id | Tenant isolation in index |
| `seller_id` | listings.seller_id | Seller filtering |
| `store_id` | listings.store_id | Store filtering |
| `title` | listings.title | Full-text search |
| `description` | listings.description | Full-text search |
| `category_id` | listings.category_id | Exact filter |
| `category_path` | categories (joined) | Hierarchical category search |
| `listing_type` | listings.listing_type | Type filter |
| `pricing_model` | listings.pricing_model | Pricing behavior |
| `price_amount` | listings.price_amount | Range filter, sort, ranking |
| `price_currency` | listings.price_currency | Currency filter |
| `availability_status` | listing_availability.status | Availability filter |
| `quantity` | listing_availability.quantity | Availability signal |
| `location` | listing_locations → locations | Geo search |
| `location_ids` | listing_locations.location_id | Multi-location support |
| `attributes` | listing_attributes + attribute_definitions | Typed attribute search |
| `media_count` | listing_media (count) | Richness signal |
| `primary_media_url` | listing_media (primary) | Result preview |
| `seller_display_name` | seller_profiles.display_name | Seller filter, result display |
| `seller_trust_level` | seller_profiles.trust_level | Ranking signal |
| `store_name` | stores.name | Store filter, result display |
| `store_status` | stores.status | Store visibility |
| `status` | listings.status | Visibility filtering |
| `published_at` | listings.published_at | Freshness, sort |
| `updated_at` | listings.updated_at | Freshness, reindex trigger |
| `freshness_score` | computed | Ranking signal |
| `metadata` | listings.metadata | Extensible payload |

### AttributeProjection

```text
AttributeProjection
├── key: string
├── name: string
├── type: string
├── value_text: string | null
├── value_number: number | null
├── value_boolean: boolean | null
├── value_date: timestamp | null
├── value_select: string | null
└── searchable: boolean
```

Only searchable attributes are included in the search projection.

## 3. Source of Truth

PostgreSQL remains the authoritative source.

The search document is a **denormalized, flattened projection** built from:

| Source Table | Document Fields |
|--------------|-----------------|
| listings | listing_id, organization_id, seller_id, store_id, title, description, category_id, listing_type, pricing_model, price_amount, price_currency, status, published_at, updated_at, metadata |
| categories | category_path |
| listing_availability | availability_status, quantity |
| listing_locations + locations | location, location_ids |
| listing_attributes + attribute_definitions | attributes |
| listing_media | media_count, primary_media_url |
| seller_profiles | seller_display_name, seller_trust_level |
| stores | store_name, store_status |

## 4. Index Synchronization Strategy

Synchronization is asynchronous, event-driven, and eventually consistent.

### Events That Affect the Index

| Event | Action |
|-------|--------|
| `ListingCreated` | Index new document |
| `ListingUpdated` | Reindex document |
| `ListingPublished` | Make document visible in public search |
| `ListingPaused` | Remove from public search |
| `ListingSuspended` | Remove from all search |
| `ListingSold` | Remove from public search |
| `ListingClosed` | Remove from public search |
| `ListingExpired` | Remove from public search |
| `ListingAvailabilityChanged` | Update availability fields |
| `ListingLocationUpdated` | Update geo fields |
| `CategoryUpdated` | Reindex all listings in category |
| `AttributeDefinitionUpdated` | Reindex listings with that attribute |

### Processing Guarantees

* Events are processed at-least-once.
* Index updates are idempotent by `listing_id`.
* Events are processed in order per listing.
* Failed events are retried with backoff and sent to a dead-letter queue after exhaustion.

### Idempotency

Each indexing operation uses `listing_id` as the document identity. Re-processing the same event produces the same index state.

### Rebuild Strategy

* Full reindex is triggered by reindexing all `PUBLISHED` listings from PostgreSQL.
* Rebuild uses batched reads with resume capability.
* Index versioning allows zero-downtime index swaps.

## 5. Index Versioning

The search index supports versioned aliases:

```text
listings_v1 → listings_current
listings_v2 → rebuild in progress
```

A reindex writes to a new versioned index. When complete, the alias is switched atomically.

## 6. Document Size Budget

A single search document should not exceed 50 KB.

Large attribute values or metadata should be truncated or stored out-of-band.

## 7. Reserved / Future Fields

The following fields are reserved for future use:

* `delivery_areas`
* `promotion_flags`
* `personalization_signals`
* `analytics_signals`
