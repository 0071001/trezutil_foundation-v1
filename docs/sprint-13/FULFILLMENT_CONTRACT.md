# Sprint 13 — Fulfillment Contract

## Fulfillment Aggregate

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | UUID | Yes | Primary key |
| `tenant_id` | UUID | Yes | Organization (tenant) |
| `order_id` | UUID | No* | Linked order |
| `reservation_id` | UUID | No* | Linked reservation |
| `type` | string | Yes | `DELIVERY`, `PICKUP`, `APPOINTMENT`, `DIGITAL`, `EVENT_ENTRY`, `TRAVEL_TICKET`, `SERVICE_COMPLETION`, `PROPERTY_VISIT`, `NONE` |
| `status` | string | Yes | `PENDING`, `ACCEPTED`, `IN_PROGRESS`, `READY`, `COMPLETED`, `CANCELLED`, `FAILED` |
| `reference` | string | Yes | Human-readable reference (e.g., `FUL-2024-001`) |
| `tracking_ref` | string | No | External tracking number |
| `assigned_to` | UUID | No | Actor assigned to fulfill |
| `metadata` | JSONB | No | Type-specific flexible data |
| `version` | int64 | Yes | Optimistic lock |
| `created_at` | timestamp | Yes | |
| `updated_at` | timestamp | Yes | |

*Exactly one of `order_id` or `reservation_id` must be non-null.

### Invariants

```sql
CHECK (type IN ('DELIVERY','PICKUP','APPOINTMENT','DIGITAL','EVENT_ENTRY','TRAVEL_TICKET','SERVICE_COMPLETION','PROPERTY_VISIT','NONE'))
CHECK (status IN ('PENDING','ACCEPTED','IN_PROGRESS','READY','COMPLETED','CANCELLED','FAILED'))
CHECK (version >= 1)
CHECK (
    (order_id IS NOT NULL AND reservation_id IS NULL) OR
    (order_id IS NULL AND reservation_id IS NOT NULL)
)
```

---

## Domain Methods

### NewFulfillment

```go
func NewFulfillment(
    tenantID string,
    orderID *string,
    reservationID *string,
    ftype FulfillmentType,
    now int64,
) *Fulfillment
```

Creates fulfillment with `status=PENDING`, auto-generated reference.

### TransitionTo

```go
func (f *Fulfillment) TransitionTo(target FulfillmentStatus, actorID string, now int64) error
```

**Valid transitions:**

| Current | Allowed Targets |
|---------|-----------------|
| `PENDING` | `ACCEPTED`, `CANCELLED` |
| `ACCEPTED` | `IN_PROGRESS`, `CANCELLED` |
| `IN_PROGRESS` | `READY`, `FAILED` |
| `READY` | `COMPLETED`, `CANCELLED`, `IN_PROGRESS` |
| `COMPLETED` | *(none - terminal)* |
| `CANCELLED` | *(none - terminal)* |
| `FAILED` | *(none - terminal)* |

**Preconditions:**
- Actor authorized for this fulfillment
- `f.Version` matches current

**Effects:**
- `f.Status = target`
- `f.Version++`
- `f.UpdatedAt = now`
- If `target == ACCEPTED`: `f.AssignedTo = actorID`

### Assign

```go
func (f *Fulfillment) Assign(actorID string, now int64) error
```

**Preconditions:**
- `f.Status == PENDING`
- Actor has fulfillment permissions for tenant

**Effects:**
- `f.AssignedTo = actorID`
- `f.UpdatedAt = now`

### SetTrackingReference

```go
func (f *Fulfillment) SetTrackingReference(trackingRef string, now int64) error
```

**Preconditions:**
- `f.Type == DELIVERY` or `f.Type == TRAVEL_TICKET`
- `f.Status` in `{ACCEPTED, IN_PROGRESS, READY}`

**Effects:**
- `f.TrackingRef = trackingRef`
- `f.UpdatedAt = now`

---

## Status Values

```go
type FulfillmentStatus string

const (
    FulfillmentStatusPending     FulfillmentStatus = "PENDING"
    FulfillmentStatusAccepted    FulfillmentStatus = "ACCEPTED"
    FulfillmentStatusInProgress  FulfillmentStatus = "IN_PROGRESS"
    FulfillmentStatusReady       FulfillmentStatus = "READY"
    FulfillmentStatusCompleted   FulfillmentStatus = "COMPLETED"
    FulfillmentStatusCancelled   FulfillmentStatus = "CANCELLED"
    FulfillmentStatusFailed      FulfillmentStatus = "FAILED"
)
```

### Terminal States

`COMPLETED`, `CANCELLED`, `FAILED` — no further transitions allowed.

---

## Type Values

```go
type FulfillmentType string

const (
    FulfillmentTypeDelivery         FulfillmentType = "DELIVERY"
    FulfillmentTypePickup           FulfillmentType = "PICKUP"
    FulfillmentTypeAppointment      FulfillmentType = "APPOINTMENT"
    FulfillmentTypeDigital          FulfillmentType = "DIGITAL"
    FulfillmentTypeEventEntry       FulfillmentType = "EVENT_ENTRY"
    FulfillmentTypeTravelTicket     FulfillmentType = "TRAVEL_TICKET"
    FulfillmentTypeServiceCompletion FulfillmentType = "SERVICE_COMPLETION"
    FulfillmentTypePropertyVisit    FulfillmentType = "PROPERTY_VISIT"
    FulfillmentTypeNone             FulfillmentType = "NONE"
)
```

---

## Metadata by Type

### DELIVERY / PICKUP

```json
{
  "address": { "street": "...", "city": "...", "postal_code": "...", "country": "..." },
  "contact_phone": "+15551234567",
  "instructions": "Leave at door",
  "package_count": 1,
  "weight_kg": 2.5,
  "dimensions_cm": { "l": 30, "w": 20, "h": 10 }
}
```

### APPOINTMENT / SERVICE_COMPLETION

```json
{
  "location": "123 Main St, Suite 100",
  "service_provider": "provider-id",
  "duration_minutes": 60,
  "requirements": ["Bring ID", "Arrive 10 min early"]
}
```

### EVENT_ENTRY / TRAVEL_TICKET

```json
{
  "event_name": "Concert 2024",
  "venue": "Arena",
  "seat": "Section A, Row 5, Seat 12",
  "gate": "Gate 3",
  "valid_from": "2024-06-15T19:00:00Z",
  "valid_until": "2024-06-15T23:00:00Z"
}
```

### DIGITAL

```json
{
  "delivery_method": "EMAIL",
  "download_url": "https://...",
  "expires_at": "2024-12-31T23:59:59Z",
  "license_key": "XXXX-XXXX-XXXX"
}
```

### PROPERTY_VISIT

```json
{
  "property_address": "123 Main St",
  "agent_id": "agent-uuid",
  "duration_minutes": 30,
  "access_instructions": "Lockbox code: 1234"
}
```

---

## Errors

| Error | Code | Description |
|-------|------|-------------|
| `ErrFulfillmentNotFound` | `FULFILLMENT_NOT_FOUND` | Fulfillment not found |
| `ErrInvalidTransition` | `INVALID_TRANSITION` | Status transition not allowed |
| `ErrNotAssigned` | `NOT_ASSIGNED` | Actor not assigned to fulfillment |
| `ErrOptimisticLock` | `OPTIMISTIC_LOCK` | Version mismatch |
| `ErrInsufficientPermissions` | `FORBIDDEN` | Actor not authorized |
| `ErrTenantMismatch` | `TENANT_MISMATCH` | Cross-tenant access |

---

## Repository Interface

```go
type FulfillmentRepository interface {
    Create(ctx context.Context, fulfillment *Fulfillment) error
    FindByID(ctx context.Context, tenantID, id string) (*Fulfillment, error)
    FindByOrderID(ctx context.Context, tenantID, orderID string) (*Fulfillment, error)
    FindByReservationID(ctx context.Context, tenantID, reservationID string) (*Fulfillment, error)
    FindByAssignedTo(ctx context.Context, tenantID, actorID string, status *FulfillmentStatus) ([]*Fulfillment, error)
    Update(ctx context.Context, fulfillment *Fulfillment) error
}
```

---

## Tenant Isolation

All operations scoped by `tenant_id`. Actor identity resolved server-side.

---

## Audit Events

| Event | When |
|-------|------|
| `fulfillment.created` | Fulfillment created |
| `fulfillment.assigned` | Assigned to actor |
| `fulfillment.transitioned` | Status changed |
| `fulfillment.tracking_updated` | Tracking ref added |
| `fulfillment.cancelled` | Cancelled |
| `fulfillment.failed` | Marked failed |