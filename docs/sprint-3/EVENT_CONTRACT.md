# Event Contract — Listing Events

## Principe

Les événements métier sont produits par l’application lors de changements d’état du `Listing`.

Ils sont utilisés pour :

* l’indexation OpenSearch
* les notifications
* l’audit
* les integrations futures

## Format

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

## Events

### ListingCreated

```json
{
  "event_name": "ListingCreated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "store_id": "store-456",
    "seller_id": "seller-789",
    "organization_id": "org-123",
    "listing_type": "PHYSICAL_PRODUCT",
    "category_id": "category-456",
    "status": "DRAFT"
  }
}
```

### ListingUpdated

```json
{
  "event_name": "ListingUpdated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "changes": {
      "title": "New title",
      "price_amount": 899.99
    }
  }
}
```

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

### ListingPaused

```json
{
  "event_name": "ListingPaused",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "reason": "out_of_stock"
  }
}
```

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

### ListingAvailabilityChanged

```json
{
  "event_name": "ListingAvailabilityChanged",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "availability_status": "OUT_OF_STOCK",
    "quantity": 0
  }
}
```

### ListingLocationUpdated

```json
{
  "event_name": "ListingLocationUpdated",
  "aggregate_type": "Listing",
  "payload": {
    "listing_id": "listing-123",
    "locations": [
      { "location_id": "loc-1", "label": "Store", "is_primary": true }
    ]
  }
}
```

## Producers

* `ListingService` / `ListingUseCase` produit tous les événements.
* Aucun autre service ne produit ces événements.

## Consumers

* `DiscoveryIndexer` : met à jour OpenSearch.
* `NotificationService` : notifie le seller.
* `AuditService` : enregistre l’audit.
* `AnalyticsService` : agrège les métriques.

## Ordering

* Les événements sont ordonnés par `occurred_at`.
* Le `version` permet le replay et la cohérence.

## Idempotence

* Chaque `event_id` est unique.
* Les consumers doivent gérer les doublons.

## Non-objectifs Sprint 3

* implémenter Kafka
* implémenter les consumers
* implémenter le producer
