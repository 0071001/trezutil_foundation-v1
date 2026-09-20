# Trezutil Events

**Status:** Version 1.0 — Event contract baseline

## 1. Event purpose

Events represent facts that have occurred. They are not remote procedure calls.

Examples:

- `ListingCreated`
- `ListingPublished`
- `OrderCreated`
- `PaymentSucceeded`

Consumers must not infer undocumented business state from event names alone; the payload schema and version define the contract.

## 2. Envelope

Every event uses a common envelope:

```json
{
  "event_id": "uuid",
  "event_type": "ListingPublished",
  "event_version": 1,
  "occurred_at": "2026-01-01T00:00:00Z",
  "producer": "catalog",
  "aggregate_type": "listing",
  "aggregate_id": "uuid",
  "tenant_id": "uuid-or-null",
  "correlation_id": "uuid",
  "causation_id": "uuid-or-null",
  "data": {}
}
```

## 3. Initial event catalogue

### Identity / seller
- `UserCreated.v1`
- `SellerCreated.v1`
- `SellerVerificationSubmitted.v1`
- `SellerVerified.v1`
- `SellerSuspended.v1`

### Listings
- `ListingCreated.v1`
- `ListingUpdated.v1`
- `ListingPublished.v1`
- `ListingUnpublished.v1`
- `ListingArchived.v1`
- `ListingAvailabilityChanged.v1`
- `ListingDistributionUpdated.v1`

### Orders
- `OrderCreated.v1`
- `OrderConfirmed.v1`
- `OrderCancelled.v1`
- `OrderCompleted.v1`
- `OrderDisputed.v1`

### Payments
- `PaymentIntentCreated.v1`
- `PaymentSucceeded.v1`
- `PaymentFailed.v1`
- `PaymentRefunded.v1`
- `PaymentReconciled.v1`

### Logistics
- `DeliveryCreated.v1`
- `DeliveryAssigned.v1`
- `DeliveryPickedUp.v1`
- `DeliveryDelivered.v1`
- `DeliveryFailed.v1`

### Trust / engagement
- `ReviewCreated.v1`
- `ComplaintCreated.v1`
- `RiskSignalDetected.v1`
- `TrustScoreChanged.v1`

### Marketing
- `CampaignCreated.v1`
- `CampaignPublished.v1`
- `CampaignPerformanceUpdated.v1`

## 4. Event guarantees

- Producer writes MUST use an outbox pattern when event publication is coupled to a database transaction.
- Consumers MUST be idempotent.
- Event processing is at-least-once by default.
- Consumers MUST tolerate duplicate delivery.
- Ordering is guaranteed only where explicitly defined by the partition key.

## 5. Kafka topic strategy

Initial logical topics:

- `trezutil.domain`
- `trezutil.integration`
- `trezutil.notifications`
- `trezutil.analytics`
- `trezutil.dead-letter`

Production topic topology may later be split by domain for throughput/ownership.

## 6. Event evolution

Never silently change the meaning of a released event version. Create `v2` for incompatible schema or semantic changes. Producers may emit multiple compatible versions during migration windows.

## 7. Example

`ListingPublished.v1` data:
```json
{
  "listing_id": "uuid",
  "seller_id": "uuid",
  "listing_type": "property",
  "published_at": "2026-01-01T00:00:00Z"
}
```
