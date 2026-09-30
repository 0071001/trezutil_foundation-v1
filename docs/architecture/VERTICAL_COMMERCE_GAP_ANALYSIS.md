# Vertical commerce — gap analysis

Date: 2026-09-29 (updated after the operational vertical phase and the Hotel pack) · Model: [VERTICAL_COMMERCE_PACKS.md](VERTICAL_COMMERCE_PACKS.md) · Hotel pack: [HOTEL_COMMERCE_PACK.md](HOTEL_COMMERCE_PACK.md)

Method: code inspection of domain, application, infrastructure, migrations and service wiring (`cmd/*`). "Existing" means it exists in code and runs in a service; code that exists but is not wired says so.

## 1. Capability matrix (current)

| Capability | Existing | Reusable | Needs change | Missing |
|---|---|---|---|---|
| **Seller** | Seller profile (type, display name, verification, trust); `GET /sellers/me`; seller console (listings, orders, reservations, inventory, business, **hotel rooms**). Server-side authorization by membership and role. | Entirely: a hotel, restaurant or agency is a seller profile of its organization. | — | Explicit vertical membership (derived from listings, §2); self-serve onboarding (L1). |
| **Organization** | Tenant, members (owner/admin/manager/staff), stores with location. Location fields are now read back correctly (§3.2). | Entirely. | — | Store opening hours; per-property time zone. |
| **Listing** | Type, **vertical**, pricing model, lifecycle, metadata, media, attributes, locations; seller create chooses a vertical. | Entirely: a hotel room type is a HOTEL listing. | `ListingType` still mixes kinds and domains (kept for compatibility). | Price unit and rate plans (one nightly rate per room type). |
| **Product** | `physical_product`, COUNTABLE stock, orders. | Yes. | — | Variants/SKUs as first-class. |
| **Service** | `service`, SERVICES pack, TIME_SLOT resources, appointments. | Yes. | — | Staff shared across services (see Inventory). |
| **Availability** | One port. Native provider: stock, capacity windows (tightest window, gaps refused), single units. External implementation hardened (explicit AVAILABLE/UNAVAILABLE/UNKNOWN) but unwired. | Yes. | — | An external provider (intentionally postponed). |
| **Inventory / capacity** | ROOM inventory per room type; nightly capacity windows with a no-overlap constraint; transactional reservation; seller API and UI for hotel rooms and nights. | Yes: tables, seats, vehicles and time slots use the same model. | Other verticals still have no management API for their resources (L2 outside hotels). | Resources shared across listings; room numbers and assignment. |
| **Reservation** | Period, quantity (rooms), party size, idempotency, cancel. Exclusion constraint for single units; **capacity-managed** reservations for multi-unit listings. Vertical booking rules via `ReservationRule`. Chat bookings require confirmation. | Entirely. | — | Modify-reservation flow; cancellation policies and penalties; provider booking reference for external supply. |
| **Order** | Idempotent create, seller-only confirm, cancel; **stay orders** (`POST /reservations/{id}/checkout`). | Entirely. | — | Multi-item orders (L5); fulfilment transitions over HTTP (L3); an explicit order↔reservation column (the link is the idempotency key `stay:<id>`). |
| **Payment** | Payment Core + Stripe adapter; hotel stays paid through it. | Entirely. | — | Stripe credentials (X1); other rails; enforcing "pay before reservation" where a pack requires it. |
| **Fulfillment** | 9 types; hotel stays get APPOINTMENT. | Yes. | Orders don't expose their fulfilment ID. | HTTP transitions (L3). |
| **Conversation** | Structured intents with `vertical` (catalogue enum), `location`, `nights`, `party_size`; runtime validation and normalization; remembered search criteria; keyword fallback detects verticals. | Yes. | Pack resolution still never succeeds at runtime (§3.3); vertical resolution replaced its purpose. | Place, nights and party-size extraction without a model key (they stay search words). |
| **Search** | Vertical and location filters, price range, vertical in results; one engine. | Yes. | — | Date-availability filtering in search (checked per listing today); geo radius in the public API. |
| **Provider** | Org-scoped providers with explicit capabilities, adapter interfaces per capability, registry, Stripe adapter. | Yes. | Provider-management HTTP routes exist but are unmounted (operators configure providers in SQL). | SEARCH/QUOTE/ORDER adapter interfaces; provider event inbox. |
| **Provider orchestration** | Per-attempt deadlines, retry and fallback, normalized error codes, idempotency key on every attempt, **every attempt recorded** (request ID, attempt, retry class); success taken from the provider result. Production use: PAYMENT. | Yes. | — | Vertical-aware provider selection; merge with native supply; external-ID mapping; circuit breaking. |
| **Vertical / Pack concept** | 10 verticals with capabilities, concepts, listing types, packs, **flows** and keywords; `GET /api/v1/verticals`; **Hotel pack operational**. | Yes. | Per-organization pack activation is still an in-memory stub (L12). | Other vertical packs (RESTAURANT next); organization vertical membership. |
| Category / Attribute | Category tree, typed attributes. | Yes. | — | Seeded vertical taxonomies. |
| Review | — | — | — | Reviews and ratings (L9). |
| Usage / Credits | Metering, credit reservations. | Yes. | — | — |

## 2. Seller vertical association

- **Today:** derived. An organization operates in the verticals of its listings (`idx_listings_org_vertical`). The Hotel pack did not need explicit activation: a seller becomes a hotel by adding room types.
- **Contract when needed:**
  - `organization_verticals (organization_id, vertical, status, activated_at)`, backfilled from listings.
  - Activation is operator-only while onboarding is assisted.
  - It replaces `organization_packs`.

## 3. Findings

### 3.1 Resolved in the vertical foundation phase
- One pack catalogue.
- BOOKING regained CAPACITY/SCHEDULING, and SERVICES declares RESERVATION.
- `POST /listings` answers 400/403 instead of 500.
- Listings carry a vertical (000019).

### 3.2 Resolved in the operational phase (this change)
1. **Vertical-aware search:** vertical, location and price filters, vertical in results; an unknown vertical is 400, never OTHER.
2. **Structured AI intents** validated inside the commerce runtime.
3. **Web flows follow the backend's vertical flows**, not `listing_type`. Real estate is now book-only, and retail buy-only.
4. **Multi-unit booking was impossible:** the reservation exclusion constraint refused any overlap per listing. Capacity-managed reservations fix it.
5. **Multi-window availability was summed and gaps went unnoticed:** it is now the tightest window, with full coverage.
6. **Capacity reservation was not atomic** (partial holds on races). It now runs in one transaction with row locks.
7. **Time-based resources drained the lifetime inventory counter:** only stock uses it now.
8. **Store/organization city, province and address were never read back** (scanned into unused variables).
9. **Chat bookings ran without confirmation:** reservation create/cancel/modify now require it, like orders.
10. **Reservation errors were 500:** they now map to 400, 409 and 503 with `errors.Is`.
11. **Provider executions were never persisted.** Every attempt is now recorded (000021), and the repository can read NULLs and timestamps.
12. **The orchestrator treated "no error" as success and returned no error when retries ran out.** Success now comes from the provider result, and exhausted retries return the error. The payment service's own contract ("transient failures leave the payment for retry") now holds.
13. **External availability reported "available" on any successful call.** It now has explicit states, UNKNOWN on doubt, and the tenant comes from the query.

### 3.3 Documented, not changed
- **Pack resolution never succeeds at runtime:** harmless, since nothing reads `PackCode`, and vertical resolution covers its purpose.
- **Unmounted provider-management API.**
- **Search does not filter by date availability:** the hotel page checks availability per room type.

## 4. Foundations and changes

| Change | Contract | Backward compatibility | Tests |
|---|---|---|---|
| Vertical types, catalogue, flows, keywords | Open codes; catalogue refuses inconsistent definitions (flows need capabilities). | New. | domain + vertical package tests |
| `listings.vertical` (000019) | Nullable, backfilled. | Old binaries keep working. | `listing_vertical_test.go` |
| Search filters | `vertical`, `location`, `price_min`, `price_max` on `POST /search`; `Vertical` on results; index fields `vertical`, `place`. | Optional fields; additive mapping. | API, query builder, mapping, projection tests; E2E H1 |
| Structured intents | Schema enum + runtime validation; criteria in context. | Existing intents unchanged; search needs text, vertical or location. | runtime, conversation and model-path tests; E2E H1 |
| `GET /api/v1/verticals` | Public catalogue. | New. | handler + gateway tests |
| Hotel pack (000020) | See HOTEL_COMMERCE_PACK.md. | `capacity_managed` defaults to false. | domain, PostgreSQL (incl. 8-way race), E2E H2–H5, browser I/J, web tests |
| Provider execution audit (000021) | request_id, attempt, retry_class. | Additive columns. | orchestrator/availability tests; PostgreSQL journey H6 |
