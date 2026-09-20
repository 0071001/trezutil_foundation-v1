# Sprint 5C — Commerce Actions

## Action Catalog

Controlled set of commerce actions that AI can request and the application validates/authorizes.

### READ Actions

| Action | Description | Required Actor | Confirmation |
|--------|-------------|----------------|-------------|
| SEARCH_LISTINGS | Search and discover listings | Customer, Seller, Staff, Admin | No |
| GET_LISTING | View a listing detail | Customer, Seller, Staff, Admin | No |
| COMPARE_LISTINGS | Compare multiple listings | Customer, Seller, Staff, Admin | No |
| GET_ORDER | View order details | Customer (own), Seller (own), Org, Platform | No |
| CHECK_INVENTORY | Check listing inventory | Seller | No |
| GET_PAYMENT_STATUS | View payment status | Customer, Seller | No |
| GET_DELIVERY_STATUS | View delivery status | Customer, Seller | No |
| GET_SELLER_REPORT | View seller analytics | Seller | No |

### WRITE Actions

| Action | Description | Required Actor | Confirmation |
|--------|-------------|----------------|-------------|
| CREATE_LISTING | Create a new listing draft | Seller | No |
| UPDATE_LISTING | Update listing details | Seller (own), Org Admin | No |
| PUBLISH_LISTING | Publish a listing | Seller (own), Org Admin | Yes |
| PAUSE_LISTING | Pause a published listing | Seller (own), Org Admin | Yes |
| CREATE_ORDER | Create a new order | Customer | Yes |
| CANCEL_ORDER | Cancel an order | Customer (own), Seller (policy) | Yes |

### FINANCIAL Actions

| Action | Description | Required Actor | Confirmation |
|--------|-------------|----------------|-------------|
| INITIATE_PAYMENT | Initiate payment for an order | Customer | Yes |

### NOT YET IMPLEMENTED

| Action | Status | Reason |
|--------|--------|--------|
| CREATE_MARKETING_CAMPAIGN | NOT YET IMPLEMENTED | Campaign domain not built |
| GET_SELLER_REPORT | NOT YET IMPLEMENTED | Analytics domain not built |

## Action Contracts

Each action has the following contract:

```text
Action Type
Description
Required Actor
Required Permissions
Input Schema
Validation Rules
Idempotency Requirements
Side Effects (only if implemented)
Expected Result
Failure Modes
Audit Requirements
```

### SEARCH_LISTINGS

```text
Actor: Customer, Seller, Staff, Admin
Permissions: Authenticated
Inputs: DiscoveryQuery (reused from Sprint 4)
Validation: Query validation via existing Search service
Side effects: None (read-only)
Idempotency: Not required (read-only)
Audit: Log query hash, actor, result count
```

### GET_LISTING

```text
Actor: Customer, Seller, Staff, Admin
Permissions: Authenticated, tenant-aware
Inputs: listing_id
Validation: listing exists, visibility rules apply
Side effects: None (read-only)
Audit: Log access, actor, listing_id
```

### CREATE_ORDER

```text
Actor: Customer (authenticated)
Permissions: Can purchase the listing
Inputs: listing_id, quantity, pricing snapshot
Validation:
  - listing exists
  - listing is published/visible
  - listing is purchasable (status check)
  - quantity > 0
  - seller is active
  - pricing is valid (server-side)
  - inventory sufficient (where applicable)
Side effects (NOT YET IMPLEMENTED):
  - Create order (requires PostgreSQL)
  - Create order items (requires PostgreSQL)
  - Reserve inventory (requires PostgreSQL)
  - Emit OrderCreated event (requires Kafka)
Idempotency: Required
Audit: Required (actor, listing, amount, quantity)
Confirmation: Required before payment
```

### CREATE_LISTING

```text
Actor: Seller (authenticated, organization member)
Permissions: Seller in the listing's organization
Inputs: Listing domain model + attributes + media + availability + locations
Validation:
  - Organization exists
  - Seller is organization member
  - Store exists (if specified)
  - Category exists
  - Pricing valid
Side effects (NOT YET IMPLEMENTED):
  - Create listing (requires PostgreSQL)
  - Create attribute values (requires PostgreSQL)
  - Create media (requires PostgreSQL)
  - Create availability (requires PostgreSQL)
  - Create listing locations (requires PostgreSQL)
  - Emit ListingCreated event (requires Kafka)
Idempotency: Supported via idempotency key
Audit: Required
Confirmation: No
```

### PUBLISH_LISTING

```text
Actor: Seller (own listing), Org Admin
Permissions: Ownership or admin
Inputs: listing_id
Validation:
  - Listing exists
  - Actor owns listing or is admin
  - Transition is valid (ValidateTransition)
Side effects (NOT YET IMPLEMENTED):
  - Update listing status (requires PostgreSQL)
  - Emit ListingPublished event (requires Kafka)
Idempotency: Supported
Audit: Required
Confirmation: Yes — publishing is a business-critical action
```

### INITIATE_PAYMENT

```text
Actor: Customer (authenticated)
Permissions: Order owner
Inputs: order_id, payment_method, amount
Validation:
  - Order exists
  - Order is in valid state for payment
  - Amount matches order total
Side effects (NOT YET IMPLEMENTED):
  - Create PaymentIntent (requires PostgreSQL)
  - Emit PaymentIntentCreated event (requires Kafka)
Idempotency: Required
Audit: Required (sensitive — no secrets in logs)
Confirmation: Yes — financial action
```

## Action Execution Boundary

```text
AI Intent
   ↓
Action Resolver (validates intent → action mapping)
   ↓
Action Validator (validates parameters)
   ↓
Authorization Policy (server-side, tenant-aware)
   ↓
Commerce Action Executor (executes via application service)
   ↓
Domain Service
```

The AI must NEVER:
- Execute SQL directly
- Invoke arbitrary application methods
- Bypass authorization
- Set actor identity from payload

The AI provides:
- Intent (what the user wants)
- Entities (parameters extracted from message)
- Conversation context

The application decides:
- Whether the action is valid
- Whether the actor is authorized
- Whether confirmation is needed
- How to execute via domain services

## AI Security Contract

```text
Authenticated: USER-123 (from JWT)
AI payload:   actor_id = USER-999, org_id = ORG-999

Executor behavior: ALWAYS uses USER-123
```

This is enforced in `ActionExecutor.Execute()`:
```go
// CRITICAL: AI identity protection.
// Always use the authenticated actor, never the AI-supplied actor.
action.Actor = authenticatedActor
```
