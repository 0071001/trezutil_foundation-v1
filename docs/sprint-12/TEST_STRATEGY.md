# Sprint 12 — Test Strategy

## Test Categories

### 1. Unit Tests (Domain)

**File**: `internal/domain/reservation_test.go`, `internal/domain/availability_test.go`

| Test | Description |
|------|-------------|
| `TestNewReservation` | Creates reservation with correct defaults |
| `TestReservation_Confirm` | PENDING → CONFIRMED |
| `TestReservation_Complete` | CONFIRMED → COMPLETED |
| `TestReservation_Cancel` | PENDING/CONFIRMED → CANCELLED |
| `TestReservation_InvalidTransitions` | COMPLETED/CANCELLED blocked |
| `TestReservation_ValidateTransition` | Validates without applying |
| `TestReservation_Overlaps` | Temporal overlap detection |
| `TestAvailabilityResult_IsAvailable` | Result interpretation |

### 2. Unit Tests (Application - Use Cases)

**File**: `internal/application/reservations/reservation_use_cases_test.go`

| Test | Description |
|------|-------------|
| `TestCreateReservation_Success` | Valid input → confirmed reservation |
| `TestCreateReservation_InvalidActor` | Empty actorID → ErrInsufficientPermissions |
| `TestCreateReservation_InvalidListing` | Empty listingID → error |
| `TestCreateReservation_InvalidDates` | EndAt <= StartAt → error |
| `TestCreateReservation_ListingNotFound` | Unknown listing → ErrListingNotFound |
| `TestCreateReservation_Unauthorized` | Actor without permission → ErrInsufficientPermissions |
| `TestCreateReservation_Unavailable` | Availability false → ErrAvailabilityUnavailable |
| `TestCreateReservation_Idempotency` | Same key → returns existing |
| `TestGetReservation_Success` | Owner can retrieve |
| `TestGetReservation_Forbidden` | Non-owner → ErrInsufficientPermissions |
| `TestGetReservation_NotFound` | Unknown ID → ErrReservationNotFound |
| `TestCancelReservation_Success` | Owner cancels PENDING → CANCELLED |
| `TestCancelReservation_Forbidden` | Non-owner → ErrInsufficientPermissions |
| `TestCancelReservation_Completed` | COMPLETED → ErrReservationCannotBeCancelled |
| `TestCancelReservation_AlreadyCancelled` | CANCELLED → ErrReservationCannotBeCancelled |

### 3. Unit Tests (Application - Handlers)

**File**: `internal/application/commerce/reservation_handlers_test.go`

| Test | Description |
|------|-------------|
| `TestCreateReservationHandler_Handle_Success` | Valid params → reservation data |
| `TestCreateReservationHandler_Handle_MissingListing` | No listing_id → error |
| `TestCreateReservationHandler_Handle_InvalidDates` | Bad timestamps → error |
| `TestGetReservationHandler_Handle_Success` | Valid ID → reservation data |
| `TestCancelReservationHandler_Handle_Success` | Valid ID → cancelled status |
| `TestCheckAvailabilityHandler_Handle_Success` | Valid query → availability result |

### 4. Unit Tests (API Handlers)

**File**: `internal/api/handlers_test.go` (new tests)

| Test | Description |
|------|-------------|
| `TestReservationHandler_CreateReservationH_Success` | 201 with reservation data |
| `TestReservationHandler_CreateReservationH_Unauthorized` | 401 no user_id |
| `TestReservationHandler_CreateReservationH_BadRequest` | 400 invalid JSON |
| `TestReservationHandler_CreateReservationH_NotFound` | 404 listing not found |
| `TestReservationHandler_CreateReservationH_Forbidden` | 403 no permission |
| `TestReservationHandler_CreateReservationH_Conflict` | 409 unavailable/conflict |
| `TestReservationHandler_CreateReservationH_Idempotency` | Same key → same reservation |
| `TestReservationHandler_GetReservationH_Success` | 200 with reservation |
| `TestReservationHandler_GetReservationH_NotFound` | 404 |
| `TestReservationHandler_GetReservationH_Forbidden` | 403 |
| `TestReservationHandler_CancelReservationH_Success` | 200 cancelled |
| `TestReservationHandler_CancelReservationH_InvalidTransition` | 400 completed |
| `TestReservationHandler_CheckAvailabilityH_Success` | 200 available result |
| `TestReservationHandler_CheckAvailabilityH_BadRequest` | 400 missing params |

### 5. Integration Tests (PostgreSQL)

**File**: `infrastructure/postgres/integration/reservations_test.go`

| Test | Description |
|------|-------------|
| `TestReservationRepository_CRUD` | Create, FindByID, Update |
| `TestReservationRepository_FindByResourceAndPeriod` | Queries overlapping reservations |
| `TestReservationRepository_ExclusionConstraint` | Double booking prevented |
| `TestReservationRepository_TenantIsolation` | Org A cannot see Org B reservations |

### 6. Concurrency Tests

**File**: `internal/application/reservations/concurrency_test.go`

| Test | Description |
|------|-------------|
| `TestConcurrentCreateReservation_SingleResource` | 2 customers, 1 room, same dates → 1 success, 1 conflict |
| `TestConcurrentCreateReservation_DifferentResources` | 2 customers, 2 rooms, same dates → 2 success |
| `TestConcurrentCreateReservation_AdjacentDates` | End=Start allowed → 2 success |

### 7. WhatsApp Tests

**File**: `internal/application/whatsapp/service_test.go` (new tests)

| Test | Description |
|------|-------------|
| `TestService_ResolveActionType_CheckAvailability` | "check availability" → ActionCheckAvailability |
| `TestService_ResolveActionType_CreateReservation` | "reserve room" → ActionCreateReservation |
| `TestService_ResolveActionType_CancelReservation` | "cancel booking" → ActionCancelReservation |
| `TestService_ProcessWebhook_CreateReservation` | End-to-end via runtime |

### 8. OpenAPI Tests

**File**: `api/openapi/openapi_test.go` (if applicable)

| Test | Description |
|------|-------------|
| `TestOpenAPI_ReservationPaths` | All reservation paths exist |
| `TestOpenAPI_ReservationSchemas` | All schemas valid |

## Test Execution

```bash
# All tests
go test -buildvcs=false ./internal/... ./infrastructure/postgres/integration/...

# Unit only
go test -buildvcs=false ./internal/domain/... ./internal/application/... ./internal/api/...

# Integration (requires PostgreSQL)
go test -buildvcs=false ./infrastructure/postgres/integration/... -v

# Concurrency
go test -buildvcs=false ./internal/application/reservations/... -run TestConcurrent -v -count=5
```

## Coverage Targets

| Package | Target |
|---------|--------|
| `internal/domain` | 90% |
| `internal/application/reservations` | 80% |
| `internal/application/availability` | 80% |
| `internal/application/commerce` (handlers) | 80% |
| `internal/api` | 80% |
| `infrastructure/postgres` | 70% (integration) |

## Test Data

Use existing test fixtures from `infrastructure/postgres/integration/test_setup.go`:
- `createTestOrg`
- `createTestUser`
- `createTestSeller`
- `createTestListing` (with availability)

## Status

**PARTIAL** — Unit tests for domain exist; application/integration tests need to be written

## Files to Create

- `internal/domain/reservation_test.go` (extend existing)
- `internal/application/reservations/reservation_use_cases_test.go`
- `internal/application/commerce/reservation_handlers_test.go`
- `infrastructure/postgres/integration/reservations_test.go`
- `internal/application/reservations/concurrency_test.go`
- Update `internal/api/handlers_test.go`
- Update `internal/application/whatsapp/service_test.go`