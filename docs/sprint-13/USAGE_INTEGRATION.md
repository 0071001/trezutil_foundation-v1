# Sprint 13 — Usage Integration

## Overview

Inventory, Capacity, and Fulfillment operations integrate with the existing Usage & Credits system (Sprint 9). Operations that consume seller resources or platform capacity are metered.

---

## Metered Operations

### Inventory Operations

| Operation | Action Type | Metered | Cost (Credits) | Notes |
|-----------|-------------|---------|----------------|-------|
| `AdjustInventory` | `ADJUST_INVENTORY` | ✅ Yes | 1 | Seller/admin operation |
| `BulkAdjustInventory` | `ADJUST_INVENTORY` | ✅ Yes | 5 | Batch operations |
| `GetInventory` | `CHECK_INVENTORY` | ❌ No | 0 | Read-only |
| `CheckCapacity` | `CHECK_INVENTORY` | ❌ No | 0 | Read-only |

### Capacity Operations

| Operation | Action Type | Metered | Cost (Credits) | Notes |
|-----------|-------------|---------|----------------|-------|
| `ReserveCapacity` | `CREATE_RESERVATION` | ✅ Yes | 1 | Part of reservation |
| `ReleaseCapacity` | `CANCEL_RESERVATION` | ❌ No | 0 | Cleanup |

### Fulfillment Operations

| Operation | Action Type | Metered | Cost (Credits) | Notes |
|-----------|-------------|---------|----------------|-------|
| `GetFulfillmentStatus` | `GET_FULFILLMENT_STATUS` | ❌ No | 0 | Read-only |
| `TransitionFulfillment` | `TRANSITION_FULFILLMENT` | ✅ Yes | 1 | Seller operation |
| `AssignFulfillment` | `TRANSITION_FULFILLMENT` | ✅ Yes | 1 | Seller operation |
| `UpdateTracking` | `TRANSITION_FULFILLMENT` | ✅ Yes | 1 | Seller operation |

---

## Usage Policy Configuration

### Default Policies

```sql
INSERT INTO usage_policies (action_type, channel, actor_type, cost, is_metered, period)
VALUES
    ('ADJUST_INVENTORY', 'OTHER', 'SELLER', 1, true, 'MONTHLY'),
    ('ADJUST_INVENTORY', 'OTHER', 'ORGANIZATION_ADMIN', 1, true, 'MONTHLY'),
    ('TRANSITION_FULFILLMENT', 'OTHER', 'SELLER', 1, true, 'MONTHLY'),
    ('TRANSITION_FULFILLMENT', 'OTHER', 'STORE_OPERATOR', 1, true, 'MONTHLY'),
    ('CHECK_INVENTORY', 'OTHER', 'CUSTOMER', 0, false, 'MONTHLY'),
    ('CHECK_INVENTORY', 'OTHER', 'SELLER', 0, false, 'MONTHLY'),
    ('GET_FULFILLMENT_STATUS', 'OTHER', 'CUSTOMER', 0, false, 'MONTHLY');
```

### Actor-Type Mapping

| Commerce Actor | Usage Actor Type |
|----------------|------------------|
| `CUSTOMER` | `CUSTOMER` |
| `SELLER` | `SELLER` |
| `STORE_OPERATOR` | `STORE_OPERATOR` |
| `ORGANIZATION_ADMIN` | `ORGANIZATION_ADMIN` |
| `ORGANIZATION_OWNER` | `ORGANIZATION_ADMIN` |
| `ORGANIZATION_MANAGER` | `ORGANIZATION_ADMIN` |
| `PLATFORM_OPERATOR` | `PLATFORM_OPERATOR` |
| `PLATFORM_ADMIN` | `PLATFORM_ADMIN` |
| `SYSTEM` | `SYSTEM` |

---

## Integration Points

### CommerceActionRuntime

The `CommerceActionRuntime` (Sprint 6/10) automatically:
1. Resolves `UsagePolicy` for `actionType` + `channel` + `actorType`
2. Checks credit balance via `UsageService.Check()`
3. Reserves credits via `UsageService.Reserve()`
4. Consumes on success / releases on failure

### Handler Registration

```go
// In handler registry setup
runtime.RegisterHandler(NewAdjustInventoryHandler(inventoryUseCases))
runtime.RegisterHandler(NewTransitionFulfillmentHandler(fulfillmentUseCases))
```

### Idempotency + Usage

Idempotency key prevents double-charging:
```
Same actor + same action + same idempotency key
→ Returns cached result
→ No duplicate usage consumption
```

---

## Seller Credit Management

### Credit Granting

Platform admins can grant credits to sellers:

```go
func (uc *AdminUseCases) GrantSellerCredits(ctx context.Context, sellerID string, amount int64, reason string) error {
    account, err := uc.creditAccountRepo.FindByOwner(ctx, domain.ActorTypeSeller, sellerID, tenantID)
    if err != nil {
        return err
    }
    return uc.usageService.Grant(ctx, account.ID, amount, reason)
}
```

### Usage Reporting

Sellers can view their usage:

```go
GET /v1/usage/balance
GET /v1/usage/history?period=2024-01
```

---

## Free Tier Considerations

| Tier | Monthly Inventory Adjustments | Monthly Fulfillment Transitions |
|------|-------------------------------|----------------------------------|
| Free | 10 | 20 |
| Starter | 100 | 200 |
| Professional | 1000 | 2000 |
| Enterprise | Unlimited | Unlimited |

Configured via `UsagePolicy` per plan.

---

## Audit Integration

Usage events automatically recorded:

```go
type UsageEvent struct {
    EventID       string
    AccountID     string
    ActionType    CommerceActionType
    Channel       Channel
    ActorType     ActorType
    Cost          int64
    ReservedAt    int64
    ConsumedAt    *int64
    ReleasedAt    *int64
    IdempotencyKey string
    Metadata      JSONB
}
```

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Insufficient credits | `INSUFFICIENT_CREDITS` (402) |
| Usage service unavailable | `TEMPORARY_FAILURE` (503) - retryable |
| Policy not found | Treat as unmetered (0 cost) |
| Idempotency hit | Return cached result, no usage charge |