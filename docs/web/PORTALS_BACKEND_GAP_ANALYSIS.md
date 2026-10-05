# Portals — backend gap analysis (Phase 0)

> **Report only.** No backend code was written or changed to produce this document, and none will be until
> it is explicitly requested (brief, phase 5). Every contract below is a **proposal** for discussion, not an
> existing API. Companion documents:
> [PORTALS_IMPLEMENTATION_MAP.md](PORTALS_IMPLEMENTATION_MAP.md) (where each gap blocks a screen) and
> [PORTALS_INFORMATION_ARCHITECTURE.md](PORTALS_INFORMATION_ARCHITECTURE.md). Written 2026-10-04 from the
> working tree; **verified live** marks facts also checked against the local staging stack.
>
> Related existing documents: `docs/finalization/KNOWN_LIMITATIONS.md` (L1–L12, referred to as `Lnn`),
> `docs/architecture/VERTICAL_COMMERCE_GAP_ANALYSIS.md`.

## 1. Classification

| Group | Meaning | Typical cost |
|---|---|---|
| **A — coded, not routed** | A use case or handler already exists and is tested; no service mounts it, or it needs only authorisation wiring | **S**: mount + guard + tests |
| **B — missing read model** | Data exists in tables (or can be aggregated) but no list/aggregate/search endpoint serves it | **M**: repository query + handler + tests |
| **C — missing persistence** | The capability has no table, or the table is unused/unexposed | **L**: migration + domain + use cases + API |
| **D — authorisation and security-linked** | Needs a permission model, an admin action, or a hardening change | **S–M** |
| **E — product capability** | A market or product decision (payment rails, reviews) | **L**, owner decision |

Priority: **P0** a security exposure · **P1** blocks a core module of a portal · **P2** important · **P3** later.

## 2. Summary

| ID | Gap | Portals blocked | Group | Priority | Cost |
|---|---|---|---|---|---|
| G-A1 | Provider management + executions API | Admin (providers, dashboard, health), Seller (integrations) | A | P1 | S–M |
| G-A2 | Usage / credits read API | Seller (usage), Admin (monetization, dashboard), Customer (usage) | A | P1 | S |
| G-A3 | Plans and subscriptions read API | Seller (usage/plan), Admin (monetization) | A | P1 | S |
| G-A4 | Update routes for organization, seller profile, store | Seller (business, stores, settings), Admin (verification) | A | P1 | S |
| G-A5 | Inventory and capacity **write** APIs outside hotels | Seller (inventory, availability, restaurant/services/vehicle) | A→C | P1 | S (stock) / L (resources) |
| G-A6 | Order lifecycle after confirm + fulfilment id on orders | Seller (orders, fulfilment), Customer (tracking), Admin | A | P1 | S |
| G-A7 | Reservation modify (and optional manual confirm/reject) | Customer, Seller | A | P2 | S |
| G-A8 | Listing close/archive and suspend (moderation) | Seller (archive), Admin (moderation) | A | P1 | S |
| G-A9 | Dispatch HTTP API | Future couriers/drivers, Admin operations | A | P3 | M |
| G-A10 | Category update/delete and attribute definitions API | Seller, Admin (catalog governance) | A→C | P2 | M |
| G-B1 | Admin organizations/sellers list, search and detail aggregate | Admin (organizations, verticals) | B | P1 | M |
| G-B2 | Platform orders / reservations / payments search + operational exceptions | Admin (commerce, operations, dashboard) | B | P1 | M |
| G-B3 | Admin listing queue across organizations and statuses | Admin (catalog governance) | B | P1 | M |
| G-B4 | Payments list (by organization/order) + ledger/settlement read | Seller (finance), Admin (payments) | B | P1 | M |
| G-B5 | Seller-facing customers | Seller (customers) | B | P2 | M + policy |
| G-B6 | ~~Aggregates (sales, orders, reservations over time)~~ **closed 2026-10-04** (`GET /admin/analytics`, `GET /sellers/analytics`); server-side filter/paging for incoming lists **still open** | Seller (orders), Admin | B | P2 | M |
| G-B7 | Conversation history (customer) and platform conversation metrics | Customer, Admin (AI) | B | P2 | M |
| G-B8 | Seller inbox (a seller-addressed conversation model) | Seller (inbox) | B→C | P2 | L |
| G-B9 | Notifications (service + inbox API) | Customer, Seller | B→C | P2 | L |
| G-B10 | Platform usage and AI usage metrics | Admin (AI, monetization, dashboard) | B | P1 | M |
| G-B11 | Recommendations, trending, curated collections | Customer (home, discover) | B→C | P3 | L |
| G-B12 | Geo search (lat/lng/radius, distance) | Customer (nearby), Seller | B | P2 | M |
| G-B13 | Capacity calendar read (windows per listing/resource) | Seller (availability) | B | P2 | S–M |
| G-B14 | User profile update | Customer (account) | B | P2 | S |
| G-C1 | Durable audit store + query API | Admin (audit), Seller (history) | C | P1 | L |
| G-C2 | Refresh-token/session store (list, revoke) | Customer, Admin (security, sessions) | C | P1 | L |
| G-C3 | `organization_verticals` (explicit vertical activation) | Seller (verticals), Admin (governance) | C | P3 | M |
| G-C4 | Marketing entities (campaigns, segments, automations, executions) | Seller (marketing) | C | P3 | L |
| G-C5 | Platform settings and feature flags | Admin (settings) | C | P3 | M |
| G-C6 | Media storage and upload | Seller (listings, media) | C | P2 | L |
| G-C7 | Channel status and webhook health read | Admin (channels), Seller (integrations) | B/C | P1 | M |
| G-C8 | Incidents and support cases | Admin (operations) | C | P3 | L |
| G-D1 | Admin user management actions (get, suspend, reactivate, roles) | Admin (users) | D | P1 | M |
| G-D2 | Team: member display data and a privacy-preserving invite | Seller (team) | D | P1 | M |
| G-D3 | Enforce the `accountant / sales / operations` organization roles | Seller (team, finance) | D | P2 | M |
| G-D4 | Define the `moderator` role's permissions | Admin | D | P3 | S |
| G-D5 | Staff-gated system health aggregator | Admin (health, search, dashboard) | D | P1 | M |
| G-D6 | Staff-gated metrics access | Admin (health, security) | D | P2 | M |
| G-D7 | Rate limiting beyond OTP and staff sign-in | All | D | P2 | M |
| G-E1 | Mobile-money (and other) payment rails | Customer (checkout), Seller (finance) | E | P1 (market) | L |
| G-E2 | Ratings and reviews | Customer, Seller | E | P3 | L |
| G-E3 | Server-side cart and wishlist | Customer | E | P3 | M |
| G-E4 | Self-serve seller onboarding | Seller | E | P3 | L |

## 3. Contracts

All paths are under `/api/v1` behind the gateway; the gateway route prefix needs to exist for each new family
(`internal/gateway/gateway.go` `routes()`). Authorisation uses the existing codes (`MEMBER`, `MGR`, `OP`,
`ADMIN`, see the implementation map); roles are always re-read from the database.

### Group A — coded, not routed

**G-A1 · Providers.** *Exists:* `ProviderHandler` (list/get/create/enable/disable/capabilities), `ProviderUseCases`,
registry, orchestrator, `provider_executions` (request id, attempt, retry class, duration, error code).
*Needed:* mount the handler in a service; add read-only executions.
```
GET  /providers?organization_id=            → {items:[{id, code, name, type, status, health_status, capabilities[], priority}]}   (secrets and configuration never returned)
GET  /providers/{id}/capabilities
POST /providers/{id}/enable|disable         (OP, audited)
GET  /provider-executions?organization_id=&provider_id=&status=&capability=&from=&to=&limit=&offset=
                                            → {items:[{id, provider_id, capability, operation, status, error_code, attempt, retry_class, duration_ms, request_id, started_at}], total}
GET  /admin/providers/summary               → per provider: attempts, failures, p50/p95 latency, last failure (OP) — platform-wide view
```
*Authz:* organization `MGR` for its own rows; `OP` platform-wide. *Persistence:* none new (indexes exist).
*Frontend:* Admin Providers, Seller Integrations. *Notes:* providers are **per-organization** rows; a platform view needs the cross-organization summary. Native sellers remain first-class; external supply stays optional.

**G-A2 · Usage / credits.** *Exists:* `UsageService.GetBalance/Check/Reserve/Consume/Release/Grant`, tables `credit_accounts, usage_ledger, usage_policies, usage_reservations`. Policies are **not seeded**, so metering is off today.
```
GET /usage/account?owner_type=&owner_id=            → {account_id, balance, currency?, period?}      (MGR / the customer for self)
GET /usage/ledger?account_id=&from=&to=&limit=&offset= → {items:[{id, action_type, amount, kind, created_at, reference}], total}
GET /usage/policies                                 → {items:[{action_type, channel, actor_type, cost}]}      (MEMBER read; OP write later)
```
*Persistence:* none. *Frontend:* Seller Usage & credits, Admin Monetization, Customer account (optional).

**G-A3 · Plans / subscriptions.** *Exists:* `PlanService.ListPlans/FindPlan`, `SubscriptionService.CreateSubscription/RenewPeriod`, `CreditPurchaseService`. Tables `plans, subscriptions, billing_periods, credit_grants, credit_purchases`.
```
GET /plans?tenant_id=
GET /subscriptions?tenant_id=&owner_type=&owner_id=  → current plan, status, period, cancel_at_period_end
GET /admin/subscriptions?status=&limit=&offset=      (OP)
```
No second billing system: reads only.

**G-A4 · Updates.** *Exists:* `UpdateOrganization`, `UpdateSellerProfile`, `UpdateStore` use cases; `UpdateSellerH` handler.
```
PATCH /organizations?organization_id=   {display_name, legal_name, description, phone, email, country, province, city, address}    (OWNER+ADMIN)
PATCH /sellers?seller_id=               {display_name, bio, phone, email}                                                        (owner of the profile or OWNER+ADMIN)
PATCH /stores?store_id=                 {name, description, phone, email, address, latitude, longitude}                         (MGR)
```
Verification/status changes stay `OP`-only. *Gaps beyond routing:* there is **no logo/branding field** and no store opening hours in the model (needs a migration if wanted).

**G-A5 · Inventory and capacity writes.** *Exists:* `InventoryUseCases.AdjustInventory`; hotel capacity API (`POST /hotels/room-types/{id}/capacity|units`). *Needed:*
```
POST /inventory/{listing_id}/adjust  {delta_total, delta_reserved?, reason, idempotency_key}   (MGR, writes inventory_audit)
POST /inventory                      {listing_id, resource_type, total}                        (create a resource: COUNTABLE, SEAT, TIME_SLOT, VEHICLE)
POST /capacity/{inventory_id}        {from, to, units}                                         (windows, blocked periods = 0)
```
RESTAURANT tables, SERVICES slots and VEHICLE fleets depend on this (L2). *Persistence:* tables exist (`inventory_resources`, `capacity_units`, `*_audit`); a **low-stock threshold** needs a column if wanted.

**G-A6 · Order lifecycle.** *Exists:* `OrderUseCases.ReadyForFulfillment`, `CompleteOrder` (seller-only); fulfilment transition/assign/tracking routes (by fulfilment id).
```
POST /orders/{id}/ready        POST /orders/{id}/complete        (MGR)
GET  /orders/{id}  → adds fulfillment_id, payment_id, status history
GET  /fulfillments?order_id=|reservation_id=                     (OWN or MGR or OP)
```
*Frontend:* seller fulfilment queue and customer tracking (L3).

**G-A7 · Reservations.** `PATCH /reservations/{id}` (ModifyReservation, OWN or MGR); optional `POST /reservations/{id}/confirm|reject` if manual confirmation is wanted (today `AutoConfirm: true`).

**G-A8 · Listing lifecycle.** `POST /listings/close` (seller/MGR/OP, `MarkAsClosed`), `POST /listings/suspend` (`OP`, with reason, audited), `POST /listings/reopen`. Search projection already removes non-published listings.

**G-A9 · Dispatch.** Application + tables exist (uncommitted work); needs an HTTP surface (agents online/offline, jobs, offers, positions, proof code). Out of scope for the three portals; relevant to admin operations later.

**G-A10 · Catalog taxonomy.** `PATCH/DELETE /categories/{id}` (`ADMIN`), `GET/POST /attributes` (definitions per category). Tables `attribute_definitions`, `categories` exist; no update paths and no attribute API.

### Group B — missing read models

**G-B1 · Organizations (admin).**
```
GET /admin/organizations?q=&status=&verification=&vertical=&limit=&offset=
    → {items:[{id, display_name, legal_name, status, verification_status, created_at, owner:{user_id}, counts:{members, stores, listings, orders_30d}}], total}
GET /admin/organizations/{id}
    → organization + seller profiles + stores + members(roles) + verticals (distinct listings.vertical) + counts
```
`ADMIN|OP`. No new tables (joins over existing). Today `GET /organizations` returns **one** organization by id and requires membership or `OP`.

**G-B2 · Platform commerce search and exceptions.**
```
GET /admin/orders?q=&status=&organization_id=&seller_id=&customer_id=&vertical=&from=&to=&limit=&offset=
GET /admin/reservations?…same filters…
GET /admin/payments?status=&organization_id=&provider=&from=&to=&limit=&offset=
GET /admin/exceptions?kind=failed_order|failed_reservation|failed_payment|failed_webhook&from=&to=  → counts + recent rows
```
`OP`. Rows carry ids and statuses, not full personal data. *Frontend:* Admin Orders, Reservations, Payments, Operations, Dashboard.

**G-B3 · Admin listing queue.** `GET /admin/listings?status=&organization_id=&vertical=&q=&created_from=&limit=&offset=` (`OP`); optional quality flags (missing image, price = 0, duplicate title) computed server-side.

**G-B4 · Payments and settlement reads.**
```
GET /payments?organization_id=&order_id=&status=&from=&to=&limit=&offset=     (MGR for the organization, OP platform-wide)
GET /payments/{id}/ledger                                                     (payment_ledger rows)
GET /finance/summary?organization_id=&from=&to=   → {captured_minor, refunded_minor, fees?, net_minor, currency, count}   (MGR) — **one currency per call**
```
Today a payment is readable only by id (`GET /payments/{id}?organization_id=`). Settlements and balances have no model (`payment_ledger` is an internal ledger, not a payout schedule).

**G-B5 · Customers (seller).** `GET /customers?organization_id=&q=&limit=&offset=` → `{id, display_name?, first_order_at, orders_count, reservations_count}`. **Policy first:** which customer fields a seller may see (name, phone, nothing); today the seller receives only the customer **id** on orders.

**G-B6 · Aggregates and server-side lists.** *Update 2026-10-04: the aggregates (sales, orders and bookings per day, status breakdowns, best sellers, buyers, with the period before) are built as `GET /admin/analytics` and `GET /sellers/analytics`, see [DASHBOARDS_AND_SITE_CONTENT.md](DASHBOARDS_AND_SITE_CONTENT.md); the dashboard no longer sums the first 100 orders. What remains is the server-side filtering and paging of the incoming lists below.*
```
GET /orders/incoming?tenant_id=&status=&q=&from=&to=&limit=&offset=&sort=      (adds filters and a total)
GET /reservations/incoming?…same…
GET /analytics/sales?organization_id=&from=&to=&bucket=day|week|month  → {currency, buckets:[{start, orders, gross_minor}]}      (one currency per call)
GET /analytics/reservations?…  GET /analytics/listings?organization_id=&from=&to= → views if tracked, else orders per listing
```
`MGR`. Needed because the dashboard today sums only the first 100 orders (`limit=100`). **No chart is built before this exists.**

**G-B7 · Conversation history.** `GET /conversations?limit=&offset=` (own), `GET /conversations/{id}/messages` (own) — L6; plus `GET /admin/conversations/summary?from=&to=` (counts by channel, action, outcome) for the AI centre. Tables `conversations`, `messages`, `conversation_contexts` exist.

**G-B8 · Seller inbox.** A concept that does not exist yet: today a conversation is customer ↔ Trezutil assistant. A seller inbox needs a "thread addressed to an organization" model (customer message about a listing/order/reservation, seller reply, status), `GET/POST /inbox?organization_id=`, and a decision on whether replies travel over WhatsApp. Product decision before design.

**G-B9 · Notifications.** `notification-service` is a placeholder (L7). Minimum: `notifications` table (user, kind, ref, read_at), `GET /notifications`, `POST /notifications/{id}/read`; producers on order/reservation/payment events; WhatsApp templates as the first channel.

**G-B10 · Platform and AI usage.** `GET /admin/usage/summary?from=&to=&group=action|channel|organization` over `usage_ledger`; `GET /admin/ai/summary` over `ai_execution_records` once the AI gateway writes them (today **nothing writes it**: `ai_providers` has an unwired repository, and `ai_models`, `ai_tenant_policies`, `ai_execution_records` have no code; provider, model, latency and cost of an AI call are **not recorded anywhere queryable**, only metrics and logs exist).

**G-B11 · Recommendations / trending / collections.** `GET /discover/recommendations`, `GET /discover/trending?vertical=&place=`, `GET /collections`. Needs an event signal (views, orders) or editorial tables. Not required for the portals to be useful.

**G-B12 · Geo search.** Add `lat`, `lng`, `radius_m` to `POST /search` (the OpenSearch document already has `location` as `geo_point` and the result type already has `distance_meters`; `RadiusMeters` exists in `DiscoveryQuery` but the HTTP request does not accept it) — L8.

**G-B13 · Capacity calendar.** `GET /capacity?listing_id=&from=&to=` → windows with total/reserved per unit of time (`GetCapacityUnits` exists).

**G-B14 · User profile.** `PATCH /auth/users/me {display_name, email}` plus preference storage if wanted (language, notification opt-ins). Phone changes need re-verification.

### Group C — missing persistence

**G-C1 · Durable audit.** Today: structured log lines only (`internal/infrastructure/audit`, "not a tamper-proof store"). Needed:
```
table audit_events (id, occurred_at, actor_id, actor_role, organization_id?, action, target_type, target_id, outcome, request_id, ip?, metadata jsonb, prev_hash?)
GET /admin/audit?actor_id=&action=&target_type=&target_id=&organization_id=&request_id=&from=&to=&limit=&offset=     (ADMIN)
GET /audit?organization_id=…                                                                                        (OWNER+ADMIN, own organization)
```
Implement `domain.AuditLogger` with a database-backed writer behind the same interface; make it append-only (revoke UPDATE/DELETE, optional hash chain). The per-module audit tables (`inventory_audit`, `capacity_audit`, `fulfillment_audit`) stay or are folded in. Until then the console states the limitation and shows **no** audit rows.

**G-C2 · Sessions.** Refresh tokens are stateless (L10): a copied token works until expiry unless the user is suspended. Needed: `auth_sessions (id, user_id, refresh_hash, created_at, last_seen_at, ip, user_agent, revoked_at)`, rotation on refresh, `GET /auth/sessions`, `DELETE /auth/sessions/{id}`, `POST /auth/sessions/revoke-all`, and `GET /admin/users/{id}/sessions` + revoke (`ADMIN`).

**G-C3 · `organization_verticals`** `(organization_id, vertical, status, activated_at)`, backfilled from listings, operator-activated (documented in `VERTICAL_COMMERCE_GAP_ANALYSIS.md` §2). Also makes pack activation real (today `allPacksEnabledRepo`: enable/disable accept and do nothing).

**G-C4 · Marketing.** Entities for `TRIGGER → AUDIENCE → CONDITIONS → AI/DECISION → ACTION → MEASUREMENT`: `segments`, `campaigns`, `automations`, `automation_runs`, `message_templates`, delivery results; an executor in `marketing-service` (placeholder today), consent/opt-out, WhatsApp template approval, usage metering. Nothing executes today; **no UI may simulate it**.

**G-C5 · Platform settings and feature flags.** `platform_settings (key, value, updated_by, updated_at)` + `feature_flags`; `GET /admin/settings`, `PUT /admin/settings/{key}` (`ADMIN`, audited, safe keys only, never secrets).

**G-C6 · Media.** Object storage (MinIO config exists in `infrastructure/minio`) + signed upload endpoint `POST /media/uploads {content_type, size}` → `{upload_url, media_id}`, size/type limits, malware scan decision, listing attach. Today listings take https links (L4).

**G-C7 · Channel status and webhook health.** Read-only, secret-free:
```
GET /admin/channels → [{channel:"web"|"mobile"|"whatsapp", configured:bool, phone_number_id?:masked, webhook:{last_event_at, events_24h, failed_24h}, sender:{configured:bool, last_error_code?}}]
GET /admin/webhooks?channel=&status=failed&limit=…   (rows of webhook_events / payment_webhook_events with normalised error)
GET /integrations?organization_id=  → payment provider status (name, status, health), WhatsApp linked/not    (MGR; never credentials)
```
Uses `channel_accounts`, `webhook_events`, `payment_webhook_events`, `providers.health_status`. *Facts today:* WhatsApp configuration is environment-only; the local staging access token expired on 2026-09-30.

**G-C8 · Incidents/support.** `incidents`, `support_cases` (+ link to request id/order/payment) if the team wants an in-product queue; otherwise link out to an external tool.

### Group D — authorisation and security-linked

**G-D1 · Admin user management.** `GET /admin/users/{id}`; `POST /admin/users/{id}/suspend|reactivate {reason}`; `POST /admin/users/{id}/roles {grant|revoke, role}`; all `ADMIN`, all audited (G-C1), cannot remove the last admin, cannot act on self. Today only the `trezutil-admin` CLI changes accounts.

**G-D2 · Team.** `GET /organizations/members` returns user ids only, and `POST` needs a `user_id`. Needed: members with `display_name`, masked phone, role, joined date; and an invite by phone number that resolves server-side without revealing whether an arbitrary number has an account (`POST /organizations/members/invite {phone, role}` → pending invite accepted at next sign-in). `OWNER+ADMIN`; role changes `PATCH …/members/{user_id}`.

**G-D3 · Organization role enforcement.** `accountant`, `sales`, `operations` exist but every guard knows only owner/admin/manager. Define the matrix (e.g. accountant: payments/finance read; sales: orders/customers/marketing; operations: orders/reservations/inventory) and enforce it in the guards, the use cases and tests — otherwise the portal cannot offer per-role navigation honestly.

**G-D4 · Moderator.** The role can sign in to the console (`/auth/staff/login`) but no endpoint treats it specially. Define what a moderator may do (listing review/suspend, category edit) or remove it from staff sign-in.

**G-D5 · Health aggregator.** `GET /admin/health` (`OP`) → per component `{name, status, checked_at, latency_ms, detail_code}` for: each upstream (not just TCP), PostgreSQL, OpenSearch (index health, doc count, projection lag), providers (`health_status`), AI (configured provider names, last verification result, **no keys**), WhatsApp sender configured. It replaces the public `/readyz` as the console's source and lets `/readyz` stop returning internal URLs (S-6).

**G-D6 · Metrics access.** Per-service `/metrics` are internal. Either a staff-gated query endpoint over a fixed allow-list (`GET /admin/metrics/summary` → error rates, p95 latency, payment/provider/webhook failure counts for the last hour/day) or an embedded Grafana with its own authentication. Never expose raw Prometheus to the browser.

**G-D7 · Rate limiting.** Only OTP request (per IP + per phone), OTP verify attempts and staff login lockout are limited. Add limits at the gateway for unauthenticated search/suggest, conversation messages, and the anonymous reads below; expose counters (`trezutil_rate_limited_total`) for the security view.

### Group E — product capability

**G-E1 · Mobile money and other rails.** Only Stripe **card** is wired (`payment_method_type: "CARD"`). The Congolese market's main instrument is not connected (business plan, mobile track). Needs a provider adapter behind Payment Core, its webhooks, reconciliation and the seller/customer UI states. **Not a portal task**, but it dominates seller finance usefulness.
**G-E2 · Reviews** (L9). **G-E3 · Server cart/wishlist** (L5). **G-E4 · Self-serve seller onboarding** (L1): replace operator-assisted creation with a verified flow.

## 4. Security and permission findings

Evidence is code or a live request against the local staging stack (anonymous `curl`, read-only).

| ID | Finding | Evidence | Impact | Recommended fix |
|---|---|---|---|---|
| **S-1** | **Anonymous seller profile read exposes phone number and user id** | `GET /api/v1/sellers?seller_id=<id>` returned **200 with `Phone`, `UserID`, `OrganizationID`, `TrustLevel`, `VerificationStatus`** to an unauthenticated caller (**verified live**). The `seller_id` comes from any public search result (`Seller.SellerID`). `GetSellerH` has no guard. | Personal data of every seller is harvestable; the user id links the seller to platform accounts | Require auth and restrict to members/`OP`, or return a public projection (display name, trust, verification only). **P0** |
| S-2 | Public store read returns organization id, phone and email | `GET /api/v1/stores?store_id=` 200 anonymous with `OrganizationID, Phone, Email, Address, Latitude, Longitude` (**verified live**) | Business contact data is intended for "Sold by", but `OrganizationID` is the tenant key used by many scoped APIs | Return a public projection (name, city, address, coordinates, public phone); drop `OrganizationID` and email. P1 |
| S-3 | Organization roles `accountant`, `sales`, `operations` have no enforcement | `membership.go` defines seven roles; `tenantStaffOnly`, `canAccessOrder`, `canAccessReservation`, hotel `canManage` consider owner/admin/manager only | A least-privilege model cannot be offered; an accountant cannot see finance data | G-D3. P2 |
| S-4 | Team invite needs a raw user id; member list returns ids only | `AddMemberH` body `{user_id, membership_role}`; `GetMembersH` returns `domain.OrganizationMember` | Owners cannot onboard staff without out-of-band ids; UI would display opaque ids | G-D2. P1 |
| **S-5** | **Platform staff cannot read or cancel reservations** while they can read any order | `canAccessReservation` knows only the customer and org owner/admin/manager; `canAccessOrder` and `payment.AccessPolicy` also admit platform admin/operator | Admin reservation operations impossible by API; asymmetry invites ad-hoc workarounds | Add the platform-staff branch (read; cancel behind a reason + audit). P1 |
| S-6 | Public `/readyz` returns internal upstream URLs | `GET /readyz` 200 anonymous: `http://conversation-service:8091`, `http://search-service:8081`… (**verified live**) | Topology disclosure | Return status only publicly; detailed health behind `OP` (G-D5). P2 |
| S-7 | Audit is a log trail, not a store | `internal/infrastructure/audit` docstring; no audit table | No queryable, tamper-evident history for admin audit or disputes | G-C1. P1 |
| S-8 | Refresh tokens cannot be revoked or listed | KNOWN_LIMITATIONS L10 | A stolen refresh token lives 30 days unless the user is suspended | G-C2. P1 |
| S-9 | Rate limiting covers only OTP and staff login | `WindowLimiter` used only in `identity_handlers.go` | Anonymous search, conversation and public reads are unthrottled | G-D7. P2 |
| S-10 | Access and refresh tokens are kept in `localStorage` | KNOWN_LIMITATIONS operational note | XSS-sensitive; the consoles raise the stakes (admin tokens) | HttpOnly refresh cookie + short-lived in-memory access token. P2 |
| S-11 | Pack enable/disable accepts calls but changes nothing, and pack reads are anonymous | `allPacksEnabledRepo` in `cmd/conversation-service`; `requireOperatorForWrites` lets every GET through | A console switch would mislead; pack catalogue is public by design (low risk) | Persist (G-C3) or do not expose switches. P3 |
| S-12 | `moderator` is a staff role with no behaviour | `StaffRoles` includes it; the console shows it an empty page | Confusing least-privilege story | G-D4. P3 |

What held up (checked, no finding): seller/tenant data endpoints return **401 anonymously**
(`/orders`, `/orders/incoming`, `/inventory`, `/fulfillments/{id}`, `/organizations`, `/organizations/members`,
`/listings/manage`, `/auth/users/me`, `/admin/overview`, **verified live**); admin endpoints re-read the role from
the database per request; payments are scoped by the actor's relation to the order/organization; organization
creation is operator-only; tenant staff guards authorise `?tenant_id=` against membership.

## 5. Recommended closure order (for the Phase 5 decision)

| Tier | Items | Why first |
|---|---|---|
| **0 — security** | S-1, S-2, S-6, S-5 | Exposure and a permission asymmetry; small, independent of portal work |
| **1 — route what exists** | G-A2, G-A3, G-A6, G-A8, G-A4, G-A1 (mount), G-A7 | Use cases are written and tested; unlocks Usage, Orders lifecycle, Archive/Moderation, Business edit, Provider list |
| **2 — read models** | G-B6, G-B4, G-B1, G-B2, G-B3, G-C7, G-D5, G-B10 | Make Dashboard, Finance, Admin Organizations/Orders/Payments/Health/AI truthful |
| **3 — persistence + permissions** | G-C1, G-C2, G-D1, G-D2, G-D3 | Audit, sessions, user actions, team |
| **4 — capability** | G-A5, G-C6, G-B7, G-B9, G-B12 | Inventory writes, uploads, history, notifications, geo |
| **5 — product** | G-E1 (mobile money) in parallel on the business side, G-B8, G-C4, G-B11, G-E2–E4 | Decisions, not plumbing |

## 6. What each portal can ship honestly without any backend change

* **Customer:** everything in the map marked IMPLEMENTED/IMPLEMENTABLE; the gaps (history, notifications, tracking, profile edit, sessions) show unavailable states.
* **Seller:** Dashboard (with truncation disclosure), Business (read) + Stores (read/create), Verticals (derived), Listings (search, drawer, promotions), Inventory (read), Hotel availability, Orders (queue, confirm, detail), Reservations (queue, calendar, detail, cancel), Finance (revenue from orders), Team (list/add by id/remove).
* **Admin:** Dashboard counts, Users (list), Organization onboarding wizard and detail by id, Verticals (read), Categories (list/create), per-organization Orders/Reservations lists, Order and Payment by id, health reachability.

Everything else renders an `UnavailableState` that cites its gap id from this document.
