# Sprint 13 — Test Strategy

## Test Categories

### 1. Domain Unit Tests

**Inventory Resource**
- `NewInventoryResource` creates valid aggregate
- `Reserve` succeeds when available
- `Reserve` fails when insufficient
- `Reserve` fails on negative quantity
- `Release` succeeds when reserved >= quantity
- `Release` fails when reserved < quantity
- `Consume` from reservation transfers Reserved→Consumed
- `Consume` direct increments Consumed
- `Restore` decrements Consumed
- `Adjust` validates invariants
- `Available()` computes correctly

**Capacity Unit**
- `NewCapacityUnit` creates valid aggregate
- `Reserve` succeeds when available
- `Reserve` fails when insufficient
- `Release` succeeds
- `Available()` computes correctly

**Fulfillment**
- `NewFulfillment` creates with PENDING status
- `TransitionTo` validates all valid transitions
- `TransitionTo` rejects invalid transitions
- `TransitionTo` rejects from terminal states
- `Assign` sets assigned_to
- `SetTrackingReference` only for DELIVERY/TRAVEL_TICKET
- `IsTerminal()` returns correct values

---

### 2. Application Use Case Tests

**InventoryUseCases**
- `GetInventory` returns resource
- `GetInventory` enforces tenant isolation
- `GetInventory` enforces seller authorization
- `AdjustInventory` succeeds for seller
- `AdjustInventory` fails for customer
- `AdjustInventory` fails for other seller
- `AdjustInventory` rejects invalid invariants
- `AdjustInventory` idempotent with key

**CapacityUseCases**
- `GetCapacityUnits` returns overlapping units
- `CheckCapacity` returns availability

**FulfillmentUseCases**
- `GetFulfillment` by ID
- `GetFulfillmentByOrder` 
- `GetFulfillmentByReservation`
- `TransitionFulfillment` valid transitions
- `TransitionFulfillment` invalid transitions rejected
- `TransitionFulfillment` enforces authorization
- `AssignFulfillment` only in PENDING
- `UpdateTracking` only for DELIVERY/TRAVEL_TICKET

---

### 3. Repository Integration Tests

**InventoryRepository**
- `Create` persists all fields
- `FindByID` returns resource
- `FindByListingID` returns resource
- `Update` with version check succeeds
- `Update` with stale version fails
- `Reserve` conditional update works
- `Release` conditional update works
- `Consume` conditional update works
- Tenant isolation enforced

**CapacityRepository**
- `Create` persists unit
- `FindByInventoryIDAndRange` returns overlapping
- `Reserve` with row lock works
- `Release` works
- Concurrency: two reserves, one wins

**FulfillmentRepository**
- `Create` persists
- `FindByID` returns
- `FindByOrderID` returns
- `FindByReservationID` returns
- `FindByAssignedTo` filters by status
- `Update` with version check
- `Transition` conditional update

---

### 4. API Handler Tests

**Inventory Handlers**
- `GET /v1/inventory` lists with pagination
- `GET /v1/inventory` filters by listing_id
- `GET /v1/inventory` filters by resource_type
- `GET /v1/inventory/{id}` returns 404 for missing
- `POST /v1/inventory/{id}/adjust` requires idempotency key
- `POST /v1/inventory/{id}/adjust` validates invariants
- `POST /v1/inventory/{id}/adjust` returns 403 for customer
- `POST /v1/inventory/{id}/adjust` returns 403 for other seller

**Capacity Handlers**
- `GET /v1/capacity/{id}` requires time range
- `POST /v1/capacity/check` returns availability

**Fulfillment Handlers**
- `GET /v1/fulfillments/{id}` returns fulfillment
- `GET /v1/orders/{id}/fulfillment` returns fulfillment
- `GET /v1/reservations/{id}/fulfillment` returns fulfillment
- `POST /v1/fulfillments/{id}/transition` validates transitions
- `POST /v1/fulfillments/{id}/transition` requires idempotency
- `POST /v1/fulfillments/{id}/assign` only in PENDING
- `POST /v1/fulfillments/{id}/tracking` only for DELIVERY

---

### 5. Concurrency Tests (Mandatory)

**Inventory Double-Consume Prevention**
```
Given: inventory.total = 1, reserved = 0, consumed = 0
When: Two concurrent Consume(1) operations
Then: One succeeds, one fails with INSUFFICIENT_INVENTORY
```

**Inventory Reserve + Consume**
```
Given: inventory.total = 5, reserved = 0
When: Concurrent Reserve(3) + Consume(3)
Then: Both succeed, final: reserved=3, consumed=3, available=-1? NO!
      Reserve(3) → reserved=3
      Consume(3) from reservation → reserved=0, consumed=3
      Final available = 2 ✓
```

**Capacity Overbooking Prevention**
```
Given: capacity.capacity = 3, reserved = 0
When: Two concurrent Reserve(2) operations
Then: One succeeds (reserved=2), one fails INSUFFICIENT_CAPACITY
```

**Fulfillment Double-Transition**
```
Given: fulfillment.status = PENDING
When: Two concurrent TransitionTo(ACCEPTED)
Then: One succeeds, one fails OPTIMISTIC_LOCK
```

---

### 6. Cross-Module Integration Tests

**Order → Inventory**
- Order Confirm consumes inventory
- Order Confirm rolls back on inventory failure
- Order Cancel before confirm does not consume
- Direct order (no reservation) consumes directly

**Reservation → Capacity**
- Reservation Create reserves capacity
- Reservation Cancel releases capacity
- Reservation Modify re-reserves (NOT IMPLEMENTED)

**Order → Fulfillment**
- Order ReadyForFulfillment creates fulfillment
- Fulfillment Complete → Order Complete (when all done)

**Reservation → Fulfillment**
- Reservation Confirm creates fulfillment for appointment types

---

### 7. Database Constraint Tests

- `CHECK (reserved + consumed <= total)` prevents negative available
- `CHECK (reserved >= 0)` prevents negative reserved
- `CHECK (consumed >= 0)` prevents negative consumed
- `CHECK (version >= 1)` prevents version corruption
- Fulfillment status CHECK constraint
- Fulfillment type CHECK constraint
- Fulfillment order_id XOR reservation_id CHECK

---

### 8. WhatsApp Integration Tests

- `CHECK_AVAILABILITY` intent → AvailabilityUseCases
- `CHECK_INVENTORY` intent → InventoryUseCases
- `GET_FULFILLMENT_STATUS` intent → FulfillmentUseCases
- Response formatting for each state
- Tenant isolation in WhatsApp flow

---

### 9. Web Component Tests

- `AvailabilityIndicator` renders correct states
- `FulfillmentTimeline` shows correct progress
- `InventoryTable` shows adjust action for sellers
- `AdjustInventoryModal` validates input
- `FulfillmentDetailModal` shows valid transitions

---

## Test Files to Create

| Layer | Files |
|-------|-------|
| Domain | `inventory_test.go`, `capacity_test.go`, `fulfillment_test.go`, `inventory_fulfillment_capacity_test.go` |
| Application | `inventory_use_cases_test.go`, `capacity_use_cases_test.go`, `fulfillment_use_cases_test.go`, `reservation_use_cases_test.go`, `orders_test.go` |
| Repository | `inventory_repo_test.go`, `capacity_repo_test.go`, `fulfillment_repo_test.go`, `inventory_capacity_test.go` |
| API | `inventory_handlers_test.go`, `capacity_handlers_test.go`, `fulfillment_handlers_test.go`, `handlers_test.go` |
| Concurrency | `inventory_concurrency_test.go`, `capacity_concurrency_test.go` |
| Cross-module | `order_inventory_test.go`, `reservation_capacity_test.go`, `fulfillment_order_test.go` |

---

## Coverage Targets

| Layer | Target |
|-------|--------|
| Domain | 95% |
| Application | 90% |
| Repository | 85% |
| API | 80% |
| Overall | 85% |

---

## Test Execution

```bash
# Unit tests
go test -buildvcs=false ./internal/domain/... -v
go test -buildvcs=false ./internal/application/... -v

# Integration tests (require Postgres)
go test -buildvcs=false ./infrastructure/postgres/... -v

# API tests
go test -buildvcs=false ./internal/api/... -v

# Concurrency tests (may need -race)
go test -buildvcs=false -race ./internal/... -run Concurrency

# Full suite
go test -buildvcs=false ./...
```