# Sprint 13 — Capability Matrix

## Capability Coverage

| Capability | Domain | Application | API | Web | WhatsApp | PostgreSQL | Tests |
|------------|--------|-------------|-----|-----|----------|------------|-------|
| Check Inventory | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Adjust Inventory | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Reserve Capacity | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Release Capacity | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Consume Inventory | ✅ | ✅ | ✅ (via Order) | ❌ | ❌ | ✅ | ✅ |
| Restore Inventory | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
| Fulfillment Status | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Fulfillment Transition | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Assign Fulfillment | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| Update Tracking | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |

---

## Detailed Capability Breakdown

### Check Inventory
- **Domain**: `InventoryResource.Available()`, `CapacityUnit.Available()`
- **Application**: `InventoryUseCases.GetInventory()`, `CapacityUseCases.CheckCapacity()`
- **API**: `GET /v1/inventory`, `GET /v1/inventory/{id}`, `POST /v1/capacity/check`
- **Web**: `AvailabilityIndicator` on listing detail, Inventory Dashboard
- **WhatsApp**: `CHECK_INVENTORY` / `CHECK_AVAILABILITY` intents
- **PostgreSQL**: `inventory_resources`, `capacity_units` tables
- **Tests**: Domain, Application, Repository, API, Concurrency

### Adjust Inventory
- **Domain**: `InventoryResource.Adjust()`, `Reserve()`, `Release()`, `Consume()`, `Restore()`
- **Application**: `InventoryUseCases.AdjustInventory()`
- **API**: `POST /v1/inventory/{id}/adjust`
- **Web**: `AdjustInventoryModal` in Inventory Dashboard
- **WhatsApp**: Not supported (seller operation)
- **PostgreSQL**: Conditional UPDATE with version check
- **Tests**: Domain, Application, Repository, API, Authorization

### Reserve Capacity
- **Domain**: `CapacityUnit.Reserve()`
- **Application**: `CapacityUseCases.ReserveCapacity()` (internal, via Reservation)
- **API**: `POST /v1/capacity/{id}/reserve` (internal)
- **Web**: `CapacityCalendar` shows reserved slots
- **WhatsApp**: Via `CREATE_RESERVATION` → capacity hold
- **PostgreSQL**: Row locks + conditional UPDATE on `capacity_units`
- **Tests**: Domain, Application, Repository, Concurrency (mandatory)

### Release Capacity
- **Domain**: `CapacityUnit.Release()`
- **Application**: `CapacityUseCases.ReleaseCapacity()` (internal, via Reservation Cancel)
- **API**: `POST /v1/capacity/{id}/release` (internal)
- **Web**: Capacity Calendar updates on cancel
- **WhatsApp**: Via `CANCEL_RESERVATION` → capacity release
- **PostgreSQL**: Conditional UPDATE
- **Tests**: Domain, Application, Repository, Cross-module

### Consume Inventory
- **Domain**: `InventoryResource.Consume()`
- **Application**: `OrderUseCases.Confirm()` → consumes inventory
- **API**: Via `POST /v1/orders/{id}/confirm` (Order handler)
- **Web**: Order detail shows fulfilled items
- **WhatsApp**: Via order confirmation
- **PostgreSQL**: Atomic transaction (Order + Inventory)
- **Tests**: Domain, Application, Cross-module (Order→Inventory)

### Restore Inventory
- **Domain**: `InventoryResource.Restore()`
- **Application**: `InventoryUseCases.RestoreInventory()` (admin/refund)
- **API**: `POST /v1/inventory/{id}/adjust` with negative delta_consumed
- **Web**: Adjust modal supports restore
- **WhatsApp**: Not supported
- **PostgreSQL**: Conditional UPDATE
- **Tests**: Domain, Application, Repository

### Fulfillment Status
- **Domain**: `Fulfillment` aggregate with status
- **Application**: `FulfillmentUseCases.GetFulfillment()`, `GetByOrder()`, `GetByReservation()`
- **API**: `GET /v1/fulfillments/{id}`, `GET /v1/orders/{id}/fulfillment`, `GET /v1/reservations/{id}/fulfillment`
- **Web**: `FulfillmentTimeline` on Order/Reservation detail, Fulfillment Dashboard
- **WhatsApp**: `GET_FULFILLMENT_STATUS` intent
- **PostgreSQL**: `fulfillments` table
- **Tests**: Domain, Application, Repository, API, Web

### Fulfillment Transition
- **Domain**: `Fulfillment.TransitionTo()`, valid transitions only
- **Application**: `FulfillmentUseCases.TransitionFulfillment()`
- **API**: `POST /v1/fulfillments/{id}/transition`
- **Web**: `FulfillmentDetailModal` with transition buttons
- **WhatsApp**: Not supported (seller operation)
- **PostgreSQL**: Conditional UPDATE with version + status CHECK
- **Tests**: Domain, Application, Repository, API, Concurrency, Authorization

### Assign Fulfillment
- **Domain**: `Fulfillment.Assign()`
- **Application**: `FulfillmentUseCases.AssignFulfillment()`
- **API**: `POST /v1/fulfillments/{id}/assign`
- **Web**: Fulfillment Dashboard assign action
- **WhatsApp**: Not supported
- **PostgreSQL**: Conditional UPDATE (only from PENDING)
- **Tests**: Domain, Application, Repository, API, Authorization

### Update Tracking
- **Domain**: `Fulfillment.SetTrackingReference()`
- **Application**: `FulfillmentUseCases.UpdateTracking()`
- **API**: `POST /v1/fulfillments/{id}/tracking`
- **Web**: Fulfillment Detail modal tracking field
- **WhatsApp**: Not supported
- **PostgreSQL**: Conditional UPDATE (only DELIVERY/TRAVEL_TICKET)
- **Tests**: Domain, Application, Repository, API, Authorization

---

## Channel Coverage Summary

| Channel | Read Operations | Write Operations |
|---------|----------------|------------------|
| REST API | All | All (with auth) |
| Web | All customer + seller views | Seller: Adjust, Transition, Assign, Tracking |
| WhatsApp | Check Inventory, Fulfillment Status | None (read-only) |

---

## Authorization Coverage

| Capability | Customer | Seller | Store Op | Org Admin | Platform |
|------------|----------|--------|----------|-----------|----------|
| Check Inventory | ✅ (public) | ✅ | ✅ | ✅ | ✅ |
| Adjust Inventory | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Reserve Capacity | Via Reservation | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Release Capacity | Via Cancel | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Consume Inventory | Via Order | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Fulfillment Status | ✅ (own) | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Fulfillment Transition | ❌ | ✅ (assigned) | ✅ (assigned) | ✅ | ✅ |
| Assign Fulfillment | ❌ | ✅ (own) | ✅ (own store) | ✅ | ✅ |
| Update Tracking | ❌ | ✅ (assigned) | ✅ (assigned) | ✅ | ✅ |

---

## Data Integrity Guarantees

| Guarantee | Mechanism | Tested |
|-----------|-----------|--------|
| No negative available | `CHECK (reserved + consumed <= total)` | ✅ |
| No negative reserved | `CHECK (reserved >= 0)` | ✅ |
| No negative consumed | `CHECK (consumed >= 0)` | ✅ |
| No double-consume | Conditional UPDATE + version | ✅ (concurrency test) |
| No overbooking capacity | Row locks + conditional UPDATE | ✅ (concurrency test) |
| Valid fulfillment transitions | CHECK constraint + domain validation | ✅ |
| Tenant isolation | tenant_id FK + query scoping | ✅ |
| Optimistic locking | version column on all aggregates | ✅ |