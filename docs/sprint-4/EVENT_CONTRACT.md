# Search Event Contract

## 1. Purpose

This document defines the domain events that the Search & Discovery Engine consumes to maintain its search projection.

Events are produced by the Commerce Core (Catalog, Listing, Category, Seller, Store modules) and consumed by the Search Indexer.

## 2. Event Format

```text
event_id
event_name
aggregate_id
aggregate_type
organization_id
seller_id
store_id
listing_id
version
payload
occurred_at
produced_by
```

All search-relevant events follow this envelope.

## 3. Events

### ListingCreated

```json
{
  "event_name": "ListingCreated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "organization_id": "org-123",
    "seller_id": "seller-456",
    "store_id": "store-789",
    "category_id": "cat-123",
    "listing_type": "physical_product",
    "pricing_model": "fixed",
    "price_amount": 100.00,
    "price_currency": "USD",
    "status": "draft",
    "title": "iPhone 15",
    "description": "Brand new...",
    "metadata": {}
  }
}
```

Action: Do not index yet. Wait for `ListingPublished`.

### ListingUpdated

```json
{
  "event_name": "ListingUpdated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "changes": {
      "title": "iPhone 15 256GB",
      "price_amount": 899.99
    }
  }
}
```

Action: If listing is published, reindex.

### ListingPublished

```json
{
  "event_name": "ListingPublished",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "published_at": "2026-09-16T00:00:00Z"
  }
}
```

Action: Index or make visible in search.

### ListingPaused

```json
{
  "event_name": "ListingPaused",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123"
  }
}
```

Action: Remove from public search. Keep in internal index.

### ListingSuspended

```json
{
  "event_name": "ListingSuspended",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "reason": "policy_violation"
  }
}
```

Action: Remove from all search indices.

### ListingSold

```json
{
  "event_name": "ListingSold",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "sold_at": "2026-09-16T00:00:00Z"
  }
}
```

Action: Remove from public search.

### ListingClosed

```json
{
  "event_name": "ListingClosed",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "closed_at": "2026-09-16T00:00:00Z"
  }
}
```

Action: Remove from public search.

### ListingExpired

```json
{
  "event_name": "ListingExpired",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "expired_at": "2026-09-16T00:00:00Z"
  }
}
```

Action: Remove from public search.

### ListingAvailabilityChanged

```json
{
  "event_name": "ListingAvailabilityChanged",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "availability_status": "out_of_stock",
    "quantity": 0
  }
}
```

Action: Update availability fields in search document.

### ListingLocationUpdated

```json
{
  "event_name": "ListingLocationUpdated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "locations": [
      {
        "location_id": "loc-123",
        "label": "Store",
        "is_primary": true
      }
    ]
  }
}
```

Action: Reindex geo fields.

### CategoryUpdated

```json
{
  "event_name": "CategoryUpdated",
  "aggregate_type": "Category",
  "payload": {
    "category_id": "cat-123",
    "changes": {
      "name": "Electronics",
      "slug": "electronics"
    }
  }
}
```

Action: Reindex all listings in this category and its children.

### AttributeDefinitionUpdated

```json
{
  "event_name": "AttributeDefinitionUpdated",
  "aggregate_type": "AttributeDefinition",
  "payload": {
    "attribute_definition_id": "attr-123",
    "category_id": "cat-123",
    "changes": {
      "searchable": true
    }
  }
}
```

Action: Reindex all listings using this attribute definition.

## 4. Event Versioning

All events carry a `version` field.

Initial version for all events is `1`.

When payload changes:
* Increment `version`
* Maintain backward-compatible consumers
* Support multiple event versions during transition

## 5. Idempotency

Each `event_id` is globally unique.

Consumers must deduplicate by `event_id` before processing.

## 6. Ordering

Events are ordered by `occurred_at`.

For a given `listing_id`, events must be processed in order.

## 7. Dead-Letter Strategy

Events that fail processing after retries are moved to a dead-letter topic/queue.

Dead-letter events are:
* Monitored for alerting
* Replayed manually after root cause is fixed
* Never silently dropped

## 8. Non-Events

The following changes do NOT generate search events:

* UserLocation changes
* Store metadata changes (unless store name/status affects search)
* Seller profile changes (unless display name/trust level affects search)
* Organization changes

These are handled via periodic reindex or specific events when their search impact is significant.
