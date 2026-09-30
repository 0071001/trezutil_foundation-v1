# Vertical Commerce Packs

Status: accepted (ADR 0010), operational · Date: 2026-09-29 · Audit: [VERTICAL_COMMERCE_GAP_ANALYSIS.md](VERTICAL_COMMERCE_GAP_ANALYSIS.md) · First pack: [HOTEL_COMMERCE_PACK.md](HOTEL_COMMERCE_PACK.md)

**Operational today:**
- vertical-aware search (§5.7) and structured AI intents (§10);
- web flows chosen from each vertical's customer flows (§5.8);
- the native **Hotel Commerce Pack**;
- recorded provider executions and explicit availability states (§8).

## 1. The principle

**A hotel, restaurant, service provider, real-estate agency, retailer, transport operator or event organizer is a native Trezutil Seller.**

A local hotel is not an "external hotel provider". It is an organization with a seller account that operates in the HOTEL vertical: it publishes rooms, manages availability and receives reservations, orders and payments in Trezutil.

An external API (a hotel channel manager, a flight GDS, a delivery platform) is optional and complementary. It adds supply; it is never what defines a vertical.

## 2. Terms

| Term | Meaning | In code |
|---|---|---|
| **Commerce Core** | The generic model and engines every vertical shares: organizations, sellers, listings, inventory, availability, reservations, orders, payments, fulfilment, conversations, usage. | `internal/domain`, `internal/application` |
| **Vertical** | A business domain (HOTEL, RESTAURANT…): a set of capabilities and domain rules **over** the Commerce Core. It never brings its own order, reservation, payment or seller engine. | `domain.Vertical`, `domain.VerticalDefinition` |
| **Capability** | A generic thing commerce can do: CATALOG, SEARCH, AVAILABILITY, RESERVATION, ORDERING, INVENTORY, CAPACITY, SCHEDULING, FULFILLMENT, PAYMENT… The same identifiers are used by verticals, transaction packs and providers. | `domain.Capability` |
| **Transaction pack** | A reusable workflow over the core: ECOMMERCE (buy), BOOKING (reserve a period), SERVICES (appointment). It carries supported actions, policies, a workflow and an AI profile. | `domain.VerticalPack`, `application/vertical.PackDefinitions` |
| **Vertical pack** ("Hotel Pack") | The product name for a vertical as a seller experiences it: the vertical definition plus the transaction packs it composes. | `application/vertical.VerticalDefinitions` |
| **Supply source** | Where an offer comes from: **native** (a Trezutil seller's listing) or **external** (a third-party API through a provider adapter). | Design only (§8) |
| **Provider** | An integration with a third party, declared with explicit capabilities, executed through Provider Orchestration. Today: payment providers (Stripe). | `domain.IntegrationProvider`, `application/provider` |

The Go type `domain.VerticalPack` predates this model and names what this document calls a *transaction pack*. Renaming it would change the public `/api/v1/packs` contract for no functional gain, so the name stays and this document is the reference for the meaning.

## 3. Layers

```text
Customer / Seller
   │  web · WhatsApp · (mobile)
   ▼
Conversation layer            AI interprets → intent + parameters (never executes)
   ▼
Commerce application          authenticated commerce actions (CommerceActionRuntime:
   │                          allowlist, validation, confirmation, idempotency, usage)
   ▼
Vertical layer                which vertical, which capabilities, which rules
   │                          (vertical catalogue + transaction packs + policies)
   ▼
Commerce Core                 Organization · Seller · Store · Listing · Category · Attribute ·
   │                          Media · Price · Inventory · Capacity · Availability ·
   │                          Reservation · Order · Payment · Fulfillment · Conversation · Usage
   ▼
Supply
   ├── Native: Trezutil sellers' data in PostgreSQL (authoritative)
   └── External (later): Provider Orchestration → adapters → third-party APIs
```

Each layer depends only on the layers below it. The Commerce Core never imports a vertical, and a vertical never imports a provider.

## 4. Commerce Core

These stay generic. A vertical may add vocabulary, rules and UI, never a second copy of them.

| Core concept | What it is | Code |
|---|---|---|
| User | A person, identified by a verified phone (OTP → JWT). | `domain.User` |
| Organization | The business: the tenant that owns listings, stores, orders, reservations, payments and provider settings. | `domain.Organization`, `organization_members` |
| Seller | The organization's selling profile (display name, verification, trust). | `domain.SellerProfile` |
| Customer | Any verified user buying or booking; never a separate account type. | `domain.User` + order/reservation `CustomerID` |
| Store | A physical or virtual point of sale of an organization (a hotel property, a restaurant, a shop). | `domain.Store` |
| Listing | An offer. Its `ListingType` is the kind of thing (product, service, property, vehicle, food, other); its `Vertical` is the business domain. | `domain.Listing` |
| Product / Service | Listings with `ListingType` physical_product / service. Not separate aggregates. | `domain.ListingType` |
| Category, Attribute | Taxonomy and typed, per-category attributes (amenities, bedrooms, duration…). | `domain.Category`, `domain.AttributeDefinition/Value` |
| Media | Listing images. | `domain.ListingMedia` |
| Price | Listing price, currency and pricing model; orders and reservations keep a pricing snapshot. | `Listing.Price*`, `domain.PricingSnapshot` |
| Inventory | A quantified resource of a listing: COUNTABLE stock or CAPACITY / TIME_SLOT / UNIT / SEAT / ROOM / VEHICLE / ASSET. | `domain.InventoryResource` |
| Capacity | Time-windowed capacity of a resource (a room night, a table slot, a departure). | `domain.CapacityUnit` |
| Availability | "Can this be booked or bought for this period and quantity?" behind one port. | `domain.AvailabilityProvider` |
| Reservation | A period booked by a customer (stay, table, visit, rental, appointment), with double-booking prevented in PostgreSQL. | `domain.Reservation` |
| Order | A purchase, with quantity, total and status lifecycle. | `domain.Order` |
| Payment | Payment Core: amount/currency bound to the order, provider-agnostic, ledgered. | `domain.Payment`, `application/payment` |
| Fulfillment | Delivery, pickup, appointment, event entry, travel ticket, service completion, property visit. | `domain.Fulfillment` |
| Review | Not implemented (see gap analysis). | — |
| Conversation | Channel-independent threads, context, intents and confirmations. | `domain.Conversation`, `application/conversation` |
| Usage / Credits | Metering and credit reservation for AI and commerce actions. | `domain.Usage*`, `application/usage.go` |

## 5. Verticals

### 5.1 Initial verticals

RETAIL · SERVICES · HOTEL · RESTAURANT · REAL_ESTATE · VEHICLE_RENTAL · TRANSPORT · EVENTS · TRAVEL · OTHER

A vertical code is an upper-case identifier (`^[A-Z][A-Z0-9_]{1,31}$`). The database checks only the shape, so **adding a vertical is one entry in `application/vertical.VerticalDefinitions`**: no migration and no Commerce Core change. The catalogue refuses to load if an entry is inconsistent (§5.4).

### 5.2 What a vertical is made of

```go
type VerticalDefinition struct {
    Code             Vertical          // HOTEL
    Name             string
    Capabilities     []Capability      // generic capabilities it runs on
    Concepts         []VerticalConcept // its vocabulary → a Commerce Core entity
    ListingTypes     []ListingType     // listing kinds it accepts
    FulfillmentTypes []FulfillmentType
    Packs            []PackCode        // transaction workflows it composes
}
```

A vertical is **capabilities + vocabulary + rules**. Every concept maps onto an existing core entity; there is no `HotelBooking`, `TableOrder` or `TicketPayment` type anywhere.

### 5.3 Capability model per vertical

Capabilities are the generic identifiers in `domain.Capability`. Concepts show how the vertical's language lands on the core.

```text
RETAIL            packs: ECOMMERCE
├── catalog, search         product → Listing · variant/SKU → Attribute
├── inventory               stock → InventoryResource(COUNTABLE)
├── ordering, payment       purchase → Order
└── fulfillment, delivery   delivery or pickup → Fulfillment

SERVICES          packs: SERVICES, ECOMMERCE
├── catalog, search         service → Listing · duration → Attribute
├── scheduling, capacity    staff member/resource → InventoryResource(TIME_SLOT) · time slot → CapacityUnit
├── availability, reservation   appointment → Reservation
└── ordering, payment, fulfillment   fixed-price job → Order · completion → Fulfillment

HOTEL             packs: BOOKING, ECOMMERCE
├── catalog, search         property → Store · room type → Listing · amenities → Attribute
├── inventory, capacity     room/unit → InventoryResource(ROOM) · nightly availability → CapacityUnit
├── availability, reservation   stay, occupancy → Reservation
├── pricing                 rate → Price
└── ordering (services)     hotel service (transfer, spa, conference room) → Listing → Order

RESTAURANT        packs: BOOKING, ECOMMERCE
├── catalog (menu)          restaurant → Store · menu → Category · menu item → Listing(food)
├── capacity, availability  table → InventoryResource(SEAT) · seating → CapacityUnit
├── reservation             table booking → Reservation
└── ordering, delivery      food order → Order → Fulfillment(DELIVERY/PICKUP)

REAL_ESTATE       packs: BOOKING
├── catalog, search         property → Listing · type → Category · location → Location · features → Attribute
└── availability, reservation   visit → Reservation (Fulfillment PROPERTY_VISIT)

VEHICLE_RENTAL    packs: BOOKING
├── catalog, search         vehicle category → Listing · vehicle → InventoryResource(VEHICLE)
├── availability, capacity, reservation   rental period → Reservation
└── pricing                 daily rate → Price

TRANSPORT         packs: BOOKING, ECOMMERCE
├── catalog, search         route/ride type → Listing
├── capacity                seat → InventoryResource(SEAT) · departure → CapacityUnit
├── reservation, ordering   ticket → Reservation
└── fulfillment, tracking   trip → Fulfillment(TRAVEL_TICKET)

EVENTS            packs: ECOMMERCE, BOOKING
├── catalog, search         event, ticket type → Listing · venue → Location
├── capacity                capacity, schedule → CapacityUnit(SEAT)
├── ordering, reservation   ticket → Order · seat reservation/RSVP → Reservation
└── fulfillment             entry → Fulfillment(EVENT_ENTRY)

TRAVEL            packs: BOOKING
├── catalog, search         trip/package → Listing · departure → CapacityUnit
├── reservation             booking → Reservation
└── fulfillment             travel document → Fulfillment(TRAVEL_TICKET)

OTHER             packs: ECOMMERCE
└── catalog, search, ordering, payment   offer → Listing · purchase → Order
```

PAYMENT, DELIVERY, TRACKING, LOCATION and NOTIFICATION are **platform capabilities**: Commerce Core services (and their providers) that any vertical may use without a pack providing them.

### 5.4 Rules the catalogue enforces

`vertical.NewCatalog` refuses to start when:
- a vertical is malformed, defined twice, or has no name, capability, listing type or pack;
- a concept maps onto something that is not a Commerce Core entity, or names an unknown resource or fulfilment type;
- a vertical names a transaction pack that does not exist;
- a vertical claims a capability that neither its packs nor the platform provide;
- a listing type has no default vertical that accepts it.

### 5.5 Listings and verticals

- Every listing has one vertical (`listings.vertical`, migration 000019). It is set when the listing is created and does not change.
- A client may send `Vertical` (`"hotel"`, `"HOTEL"`, `"Real estate"` are normalized). It must exist and accept the listing's type, otherwise the API answers **400 `INVALID_PARAMETER`**.
- A client that sends no vertical (every client written before this change) gets the listing type's default:

| listing_type | default vertical |
|---|---|
| physical_product, vehicle | RETAIL (vehicles are sold today; rentals say VEHICLE_RENTAL) |
| food | RESTAURANT |
| service | SERVICES |
| real_estate | REAL_ESTATE |
| other | OTHER |

- `ListingType` is kept as the *kind* of offer. It predates verticals and mixes kinds with domains (`food`, `real_estate`); normalizing it is future work, and nothing in this change depends on it being clean.

### 5.7 Vertical-aware search

`POST /api/v1/search` takes the generic request plus optional filters. There is one search engine for every vertical.

```text
SearchRequest
├── text          words (optional when a vertical or location is given)
├── category_id
├── vertical      catalogue code, case-insensitive ("hotel"); unknown → 400 INVALID_VERTICAL, never OTHER
├── location      place name ("Gombe"); every word must appear in the listing's place
│                 (store address/city/province, listing locations) or its title/description
├── price_min / price_max   non-negative, min ≤ max, else 400 INVALID_PRICE_RANGE
├── sort_field / sort_direction
└── page_size / cursor
```

- Every result carries its `Vertical`.
- The index gains `vertical` (keyword) and `place` (text). An existing index gets them through an additive mapping update at start-up, and the start-up resync fills them.
- Vertical-specific parameters (nights, party size) are not search filters: a party of four can take two double rooms. They drive the vertical's availability step.
- Geo-radius search stays in the internal `DiscoveryQuery`; it is not exposed yet (no geocoding).

### 5.8 Customer flows

Each vertical declares its **flows**, most prominent first: `BOOK` (core Reservation) and/or `BUY` (core Order). The catalogue refuses BOOK without RESERVATION or BUY without ORDERING.

| Vertical | Flows |
|---|---|
| HOTEL | BOOK (the hotel page with room selection) |
| RESTAURANT | BOOK (reserve a table), BUY (order) |
| SERVICES | BOOK (appointment) |
| REAL_ESTATE, VEHICLE_RENTAL, TRANSPORT, TRAVEL | BOOK |
| RETAIL, EVENTS, OTHER | BUY |

- `GET /api/v1/verticals` (public, cacheable) publishes codes, names, flows, capabilities and accepted listing types.
- The web app chooses its call to action from the flows, never from the listing type.
- If the catalogue cannot be loaded, the app falls back to the legacy listing-type guess (degraded mode).
- Sellers pick the vertical when creating a listing.

### 5.6 Transaction packs

| Pack | Workflow | Used by |
|---|---|---|
| ECOMMERCE | listing → details → checkout (order) → payment → fulfilment → tracking | RETAIL, SERVICES, HOTEL (services), RESTAURANT (food orders), TRANSPORT, EVENTS, OTHER |
| BOOKING | search → details → availability → reservation → confirmation | HOTEL, RESTAURANT (tables), REAL_ESTATE (visits), VEHICLE_RENTAL, TRANSPORT, EVENTS, TRAVEL |
| SERVICES | inquiry → availability → booking → confirmation → fulfilment | SERVICES |

There is one pack catalogue (`PackDefinitions`), used by the conversation service and exposed at `GET /api/v1/packs`.

## 6. Seller model

```text
Organization (tenant: owns everything below)
├── Members ─────────── owner · admin · manager · staff (server-side roles)
├── Seller profile ──── display name, verification, trust
├── Stores ──────────── a hotel property, a restaurant, a shop, an agency office
├── Verticals ───────── the domains it operates in (HOTEL + RESTAURANT + TRANSPORT…)
│     └── Capabilities   from the vertical definitions
├── Listings ────────── each in one vertical, in one store
│     ├── Resources      InventoryResource (ROOM, SEAT, TIME_SLOT, VEHICLE, COUNTABLE…)
│     └── Availability   CapacityUnit windows, or single-unit booking
├── Orders ──────────── incoming purchases (seller confirms, fulfils)
├── Reservations ────── incoming bookings
└── Providers ───────── its own integrations (today: its Stripe account)
```

- **Seller vs organization.** The organization is the business and the tenant boundary; the seller profile is how it presents itself when selling. Authorization is always organization membership plus role, checked server-side.
- **Several verticals per seller.** A hotel can publish rooms (HOTEL), a menu (RESTAURANT), an airport transfer (TRANSPORT) and a conference (EVENTS) from the same organization and store. Nothing forces one category per seller.
- **Vertical membership today** is derived: an organization operates in the verticals of its listings (`SELECT DISTINCT vertical FROM listings WHERE organization_id = …`, indexed). Explicit activation, "this hotel has turned on the RESTAURANT pack before publishing anything", is designed but not built, because nothing gates on it yet. When the first vertical-specific seller tool ships, add `organization_verticals (organization_id, vertical, status, activated_at)`, backfilled from listings, and replace the unused `organization_packs` table (migration 000013).

## 7. Native supply vs external supply

### Native Trezutil supply

The merchant has a Trezutil seller account and runs its commerce in Trezutil.

```text
Hotel Kinshasa
→ Organization + seller profile + store "Hotel Kinshasa, Gombe"
→ operates in HOTEL (and possibly RESTAURANT, TRANSPORT)
→ publishes room types (listings, vertical HOTEL)
→ rooms and nights (inventory resources ROOM + capacity units)
→ receives reservations and orders; takes payments through its Stripe account
```

PostgreSQL is authoritative, and every rule (double booking, idempotency, payment matching, tenant isolation) is enforced by the Commerce Core. This is the only supply that exists today.

### External supply (later)

A third party exposes inventory through an API.

```text
External hotel platform
→ adapter (declares capabilities: SEARCH, AVAILABILITY, QUOTE, BOOKING, CANCELLATION…)
→ Provider abstraction (IntegrationProvider + capability adapter interfaces)
→ Provider Orchestration (select, execute, retry/fallback, normalize, record)
→ Trezutil vertical capability (HOTEL / search, availability, reservation)
```

### How they stay distinguishable

Both can appear in one customer experience (one result list, one checkout), but they never become indistinguishable in data:

| | Native | External |
|---|---|---|
| Offer | `listings` row owned by a seller's organization | provider offer referenced by `(provider_id, external_id)`; not written into `listings` as if it were native |
| Tenant | the seller's organization | the organization that holds the provider integration (a platform-managed supplier organization for aggregator supply) |
| Availability / price truth | Commerce Core | the provider, at quote time, with a quote expiry |
| Booking / order | core `Reservation` / `Order` | core `Reservation` / `Order` **plus** a provider booking reference and execution record |
| Result marker | `source = NATIVE` | `source = EXTERNAL`, `provider = <code>` |

The rule: an external offer is always resolved to a core Order or Reservation owned by an organization, so authorization, payments, the ledger, the seller console and conversations work the same way. What changes is who confirms availability and where the booking is mirrored.

## 8. Provider abstraction

### 8.1 What exists

- **`IntegrationProvider`** (`providers` table, migration 000011): organization-scoped, with code, type, status, health, priority, `secret_ref` (never a secret value) and an explicit list of **capabilities**.
- **Capability-specific adapter interfaces** (`domain/provider_adapter.go`): `ProviderAdapter` (ID, capabilities, health), `AvailabilityProviderAdapter`, `ReservationProviderAdapter`, `TrackingProviderAdapter`, `PaymentProviderAdapter`. A provider implements only the interfaces it supports. An unsupported call is `UNSUPPORTED_CAPABILITY`, never a silent no-op.
- **`ProviderRegistry`**: capability lookup per organization, usable providers only, ordered by priority.
- **`ProviderOrchestrator.ExecuteCapability`**: timeout, retry with exponential backoff, rotation to the next provider on retryable errors, a normalized error-code set (`PROVIDER_UNAVAILABLE`, `TIMEOUT`, `RATE_LIMITED`, `RESOURCE_UNAVAILABLE`, `CONFLICT`…), an execution context with tenant, actor, request ID and idempotency key.
- **Idempotency**: `domain.WithProviderIdempotencyKey` carries the key to adapters so retries never repeat a side effect (used by Stripe).
- **Webhooks**: signed provider events for payments (Stripe), with replay protection and exactly-once ledger effects.
- **Execution records**: every attempt is written to `provider_executions` (migration 000021).
  - Fields: organization, provider, capability, operation, actor (NULL for system calls), idempotency key, request ID shared by the attempts of one call, attempt number, status, normalized error code, retry class, timings and external reference.
  - Recording never changes a call's outcome.
  - Reads are organization-scoped, and no API exposes the records.
- **Result semantics:**
  - success comes from the provider's result, never from a call returning without error;
  - a provider's refusal is a business outcome and is not retried;
  - a retryable failure (timeout, connection error) is retried with backoff on the next provider in priority order;
  - each attempt has its own deadline.
- **Native availability** is served by `CapacityAvailabilityProvider` behind the core `AvailabilityProvider` port. `OrchestratedAvailabilityProvider` is the external implementation of the same port. It is hardened but still unwired: no external provider exists.
- **Availability states:**
  - **AVAILABLE** or **UNAVAILABLE** only when the provider states it explicitly.
  - A missing, contradictory or malformed answer, a provider error or a timeout is **UNKNOWN**, returned with `ErrAvailabilityUnknown`.
  - The API answers 503 `AVAILABILITY_UNKNOWN`, so UNKNOWN can never be booked.
  - A transport success is never availability.

In production the orchestrator runs one capability: **PAYMENT** (Stripe). Deterministic emulators (`internal/application/provider/providertest`) exercise availability in tests only.

### 8.2 The model

The existing code already has a better abstraction than a `NativeTrezutilProvider` class: **one Commerce Core port per capability, with a native and an external implementation.**

```text
Commerce Core port (per capability)      e.g. AvailabilityProvider
├── Native implementation                CapacityAvailabilityProvider (core repositories)
├── External implementation              Orchestrated…Provider → ProviderOrchestrator → adapters
└── Composite (later)                    native first, then external; merged and marked by source
```

Native supply does not go through adapters, retries or provider execution records: it is the core itself, transactional and authoritative. Wrapping it as a "provider" would add latency and a failure mode for no gain.

### 8.3 Capabilities a provider may declare

| Needed | Identifier | Status |
|---|---|---|
| search | `SEARCH` / `CATALOG` | identifiers exist; no adapter interface yet |
| availability | `AVAILABILITY` | `AvailabilityProviderAdapter` exists |
| details | `CATALOG` | no adapter interface yet |
| quote | — | to add (`QUOTE`) with the first external adapter: price and availability held until an expiry |
| booking | `BOOKING` / `RESERVATION` | `ReservationProviderAdapter.CreateExternalReservation` |
| cancellation | `RESERVATION` | `ReservationProviderAdapter.CancelExternalReservation` |
| order | `ORDERING` | no adapter interface yet |
| status | `TRACKING` | `TrackingProviderAdapter.Track` |
| webhook / event | — | per provider (payments today); to generalize as a provider event inbox |

No provider is forced to implement every capability. Capabilities are declared on the provider row and proven by the adapter's interfaces.

### 8.4 Deliberately postponed

- No hotel, flight, restaurant, delivery, travel or aggregator integration.
- No `QUOTE`, search or order adapter interfaces until a real provider's contract is known. They will be shaped by that contract, not guessed.
- No mixed native + external search.

## 9. Provider Orchestration boundary (target)

```text
User
  ↓
Conversation / Web / Mobile
  ↓
Commerce application           authenticated action, allowlist, confirmation, idempotency, usage
  ↓
Vertical                       HOTEL: capabilities SEARCH, AVAILABILITY, RESERVATION…
  ↓
Supply resolution
  ├── Native Trezutil sellers   (Commerce Core, authoritative)
  └── Provider Orchestration    (only if the vertical has external supply enabled)
        ├── External provider A
        └── External provider B
  ↓
Normalized commerce result     one shape, each item marked NATIVE or EXTERNAL
```

**Provider Orchestration will:**
- **select providers** by vertical, capability, tenant, health and priority;
- **execute** one capability per call with a deadline;
- **normalize** results and errors into core shapes and error codes;
- **handle failures**: timeouts, rate limits, degraded health, circuit breaking;
- **fall back** to the next provider, or return native results only;
- **preserve idempotency**, with one key per business operation carried to the provider;
- **map external IDs**: `(provider, external_id)` ↔ core Order/Reservation, never exposed as native IDs;
- **process webhooks/events**: a signed, de-duplicated inbox that updates core state through the same use cases as native flows;
- **record every execution** in `provider_executions` for audit and reconciliation.

Example: "Find me a hotel in Gombe for Friday".
1. SEARCH on HOTEL.
2. Native results come from the index; external results, once enabled, come from the providers declaring SEARCH for HOTEL.
3. The results are merged, marked by source, and ranked by one policy.

**Not implemented now.**

## 10. Conversational layer

The AI resolves **what** the user wants, never **who** supplies it.

```text
"Je cherche un hôtel à Gombe pour deux nuits."
AI  →  { intent: SEARCH_LISTINGS,
         parameters: { vertical: HOTEL, location: "Gombe", nights: 2 } }
```

Then:

```text
AI (intent + parameters, strict JSON schema, allow-listed intents)
↓
Authenticated commerce action (CommerceActionRuntime: validation, confirmation, idempotency, usage)
↓
Vertical capability (is SEARCH / RESERVATION a capability of HOTEL?)
↓
Supply resolution → Provider Orchestration (if external supply is enabled)
↓
Provider
```

Rules:
- The model never names a provider, an endpoint or an external ID. Provider choice is an application decision, made after authorization.
- The vertical is a **parameter**, validated against the catalogue (`NormalizeVertical`, then the catalogue lookup) inside the commerce runtime. An unknown vertical is refused (the user is asked what they are looking for), never trusted and never turned into OTHER.
- The intent allowlist, confidence threshold, confirmation tokens and usage metering are unchanged; a vertical adds no action type.
- A vertical may add vocabulary for the model ("nuit", "chambre", "table pour 4") through its definition; it never adds tools the model can call.

**Implemented:**
- The intent schema carries `vertical` (an enum of the catalogue codes), `location`, `nights` and `party_size`. The prompt lists each vertical with its customer keywords.
- The commerce runtime's parameter validator checks and normalizes them for every channel and interpreter:
  - an unknown vertical is refused with a clarification, never mapped to OTHER;
  - nights must be 1–90 and party size 1–100;
  - a search needs words, a vertical or a location;
  - with a start date, `nights` gives the end date.
- The conversation remembers the latest search's criteria, so "book it" reuses nights and party size.
- Bookings from chat, like orders, run only after explicit confirmation.
- Without a model key, the keyword fallback detects a vertical from the catalogue keywords. Place, nights and party size then stay as search words.

```json
{ "intent": "SEARCH_LISTINGS",
  "parameters": { "vertical": "HOTEL", "location": "Gombe", "nights": 2, "party_size": 2 } }
```

## 11. What we deliberately do not build

- No per-vertical order, reservation, payment, seller or conversation engine.
- No per-vertical database or microservice.
- No fake external providers, and no speculative provider APIs.
- No schema beyond what a pack cannot express with the core. The Hotel pack added one table of room facts (`hotel_room_types`). Vertical definitions live in code, like transaction packs.

## 12. Final architecture decision

### What Trezutil owns natively
- **Seller accounts**: organizations, members and roles, seller profiles, stores.
- **Listings**: products, services and other offers; categories, attributes, media, prices.
- **Inventory, capacity and availability**, including double-booking prevention.
- **Reservations and orders**, with their lifecycle, confirmation and cancellation.
- **Payments**: Payment Core, ledger and refunds. Payment providers are adapters.
- **Fulfilment.**
- **Conversations**, intents, confirmations and usage/credits.
- **Commerce operations** in the seller console.
- **The vertical catalogue** and the rules of each vertical.

### What external providers will eventually provide
- **Additional inventory** Trezutil sellers do not have: external hotel inventory, flights and travel inventory, intercity transport, ticketing.
- **Delivery and logistics.**
- **Specialized services**: identity verification, notifications, mobile money and other payment rails.

### What Provider Orchestration will do
Select providers, execute provider capabilities, normalize results, handle provider failures, support fallback, preserve idempotency, map external IDs, process webhooks/events and record executions. It will always return core shapes owned by an organization and marked by source.

## 13. Code map

| Concern | Where |
|---|---|
| Vertical identifiers, definition, core-entity mapping, resolver port | `internal/domain/vertical.go` |
| Vertical catalogue (10 verticals) and consistency rules | `internal/application/vertical/vertical_definitions.go` |
| Transaction packs | `internal/application/vertical/pack_definitions.go` |
| Listing vertical resolution | `application.ListingUseCases.WithVerticals`, `CreateListing` |
| Listing vertical persistence | `migrations/000019_listing_verticals.sql`, `infrastructure/postgres/listings.go` |
| Provider model and adapters | `internal/domain/provider*.go`, `internal/application/provider` |
| Native availability | `internal/application/availability/capacity_availability_provider.go` |
| Vertical-aware search | `internal/api/search_handlers.go`, `internal/search/…/query_builder.go`, `projection/source.go` |
| Structured intents | `internal/application/ai/orchestrator.go`, `internal/application/commerce/validation.go`, `conversation/processor.go` |
| Vertical catalogue API | `internal/api/verticals_handlers.go` (`GET /api/v1/verticals`) |
| Hotel Commerce Pack | `internal/domain/hotel.go`, `internal/application/hotel`, `internal/api/hotel_handlers.go`, migration 000020 |
| Provider execution records, external availability | `internal/application/provider/orchestrator.go`, `availability_adapter.go`, `infrastructure/postgres/provider_execution.go`, migration 000021 |
