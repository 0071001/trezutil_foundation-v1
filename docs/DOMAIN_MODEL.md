# Trezutil Domain Model

**Status:** Version 1.0 — Domain baseline

## 1. Modeling principles

- Use stable domain IDs (UUID recommended).
- Store money as integer minor units plus ISO-like currency code.
- Store timestamps in UTC; preserve user locale/time zone separately.
- Never use floating point for monetary values.
- Separate identity from role and organization membership.
- Separate listing lifecycle from publication/distribution state.
- Keep external-provider IDs alongside internal IDs.

## 2. Core aggregates

### User
Represents a human identity.

Fields:
- id
- phone
- email nullable
- display_name
- status
- created_at
- updated_at
- verified_at nullable

User statuses:
- active
- suspended
- pending_verification
- admin
- operator

### Identity
Prevents the possibility of rattaching several identites externals:

- identity_id
- user_id
- provider
- provider_subject
- metadata
- created_at

Examples:
- whatsapp
- google
- apple

### Role
Predefined roles for authorization:

- CUSTOMER
- SELLER
- STAFF
- ADMIN
- OPERATOR
- MODERATOR
- DELIVERY_PARTNER

### Permission
Explicit permissions rather than hardcoded logic:

- catalog.read
- catalog.write
- listing.create
- listing.update
- order.read
- order.create
- payment.read
- payment.execute
- seller.manage
- admin.manage

### ExternalIdentity
Associates an external provider identity (WhatsApp, Google, Apple) with a Trezutil user.

## 2. Core aggregates

### User
Represents a human identity.

Fields:
- id
- phone
- email nullable
- display_name
- status
- created_at
- updated_at
- verified_at nullable

User statuses:
- active
- suspended
- pending_verification
- admin
- operator

Lifecycle:
- A user starts with pending_verification status
- After OTP verification, status changes to active
- Admin/operator users have elevated privileges

### Organization
Represents a business, agency, shop or operational entity.

Fields:
- id
- legal_name
- display_name
- type
- status
- verification_status
- default_currency
- contact information
- locations

### Membership
Associates a user to an organization with a role and permissions.

Roles initially:
- owner
- manager
- seller
- operations
- finance
- support
- analyst

### SellerProfile
The commerce-facing seller identity.

Includes:
- organization/user owner
- display name
- bio
- categories
- verification state
- trust state
- business locations
- contact channels

### Listing
Generic commerce offer.

Required concepts:
- id
- seller_id
- type
- title
- description
- status
- pricing model
- currency
- location
- availability
- media references
- attributes
- published_at
- expires_at when applicable
- created_at / updated_at

Listing types:
- product
- property
- vehicle
- food
- service
- accommodation
- other

Lifecycle:
`draft -> review -> published -> paused -> sold/rented/completed -> archived`

### ListingDistribution
Represents a listing's state on an external channel.

Fields:
- listing_id
- channel
- external_id
- state
- last_published_at
- last_sync_at
- error_code
- error_message

Canonical listing data remains in Trezutil.

### InventoryItem
Represents available quantity or availability units.

Fields:
- listing_id
- sku where relevant
- quantity_available
- quantity_reserved
- unit
- availability_state
- version

Property/service listings may use availability slots rather than quantity.

### SearchDocument
Derived index representation. Not authoritative.

### TrustProfile
Aggregates trust signals for a user, seller or listing.

Signals may include:
- identity verification
- business verification
- listing verification
- account age
- transaction outcomes
- dispute outcomes
- complaints
- review quality
- suspicious activity signals

A trust/risk score MUST include explainable contributing signals internally.

### Conversation
A user/channel conversation.

Fields:
- id
- participant_id
- channel
- external_conversation_id
- state
- assigned_agent_id
- handoff_state
- context reference
- created_at / updated_at

### Message
Immutable inbound/outbound message record.

Fields:
- id
- conversation_id
- direction
- message_type
- provider_message_id
- content reference
- media references
- status
- sent/received timestamps

### Intent
Structured interpretation of a user request.

Examples:
- search_listing
- create_listing
- update_listing
- check_order
- create_order
- cancel_order
- initiate_payment
- schedule_visit
- seller_report
- support_request

Intent is not an authorization decision.

### Order
Aggregate representing purchase/reservation/booking intent that has been accepted by Trezutil.

States:
`pending -> confirmed -> paid -> preparing -> fulfilled -> completed`

Alternative terminal states:
`cancelled`, `refunded`, `disputed`, `expired`

### OrderItem
References a listing/offering snapshot with quantity and price at order time.

### PaymentIntent
Represents a request to collect payment.

States:
`created -> pending -> authorized -> succeeded`

Failure/terminal states:
`failed`, `cancelled`, `expired`, `refunded`

### PaymentTransaction
Immutable ledger-like external payment record with provider references and reconciliation state.

### Delivery
Represents fulfilment.

States:
`requested -> assigned -> picked_up -> in_transit -> delivered`

Alternative states:
`cancelled`, `failed`, `returned`

### Campaign
Marketing/distribution initiative linked to one or more listings.

### Review
Customer feedback associated with a completed interaction where policy permits.

### Dispute
Structured case between customer, seller and/or platform.

### AuditLog
Immutable record of sensitive administrative or system actions.

## 3. Value objects

Recommended reusable value objects:

- Money(amount_minor, currency)
- GeoPoint(latitude, longitude)
- Address
- PhoneNumber
- DateRange
- TimeRange
- PriceRange
- Pagination
- MediaReference

## 4. State machine invariants

### Listing
A listing cannot be published unless:
- seller is eligible to publish;
- required fields are complete;
- moderation/verification policy is satisfied;
- pricing is valid;
- media requirements for the listing type are satisfied where configured.

### Order
An order cannot be marked paid merely because a conversational message says “I paid”. Payment status MUST come from a verified payment workflow or an authorised manual operation.

### Payment
Payment success MUST be based on trusted provider confirmation or controlled manual reconciliation, never on client-provided text.

### Delivery
A delivery cannot be marked completed without the configured proof-of-delivery rule.

## 5. Real-estate extension

Property-specific attributes should live in a type-specific extension rather than polluting generic listing columns:

- property_type
- bedrooms
- bathrooms
- area_m2
- furnished
- parking
- amenities
- lease_term
- rent_period
- deposit
- visit_required
- virtual_tour_url

## 6. Identity and tenancy model

Trezutil supports:

- individual sellers;
- organizations;
- organization staff;
- platform administrators.

Every privileged data access MUST resolve an authorization context:

`actor -> organization/tenant -> resource -> permission -> policy decision`

## 7. Data retention

Retention policies MUST be defined by data class. Payment, audit, identity and conversation data require explicit retention classification before production launch.
