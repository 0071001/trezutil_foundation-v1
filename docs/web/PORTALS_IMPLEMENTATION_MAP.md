# Portals — implementation map (Phase 0)

> **Phase 0 — audit only.** Nothing in `web/`, `internal/`, `cmd/` or `migrations/` was changed to produce
> this document. Companion documents:
> [PORTALS_INFORMATION_ARCHITECTURE.md](PORTALS_INFORMATION_ARCHITECTURE.md) (navigation, design language,
> implementation order, exact Phase 1 scope) and
> [PORTALS_BACKEND_GAP_ANALYSIS.md](PORTALS_BACKEND_GAP_ANALYSIS.md) (every gap with its contract, plus the
> security and permission findings).
>
> Written 2026-10-04 against the working tree (branch `main`, commit `78b07ca` plus uncommitted work).
> Every claim below was read in code or, where marked **verified live**, checked against the local staging
> stack. Nothing is assumed from what a menu "would need".

> **Update 2026-10-05.** This map is the Phase 0 audit and its rows are not rewritten. What has been built since is listed in [`../finalization/CONSOLE_MODULES.md`](../finalization/CONSOLE_MODULES.md): the seller order lifecycle (G-A6), and in the administration the Orders, Reservations, Organizations and Audit modules (G-B1, G-B2, G-C1) and account suspension (G-D1). A row below that says *BACKEND GAP* for one of these is out of date; the gap analysis carries the dated notes. Counts: seller 8 of 20 modules ready, administration 7 of 20.

## 0. How to read this

### Status

| Status | Meaning |
|---|---|
| **IMPLEMENTED** | A screen exists in `web/frontend` today and talks to a real, routed API. |
| **IMPLEMENTABLE** | The API exists, is routed through the gateway and is authorised for this actor, but no screen (or only part of one) exists. A frontend task only. |
| **BACKEND GAP** | No routed API, or the API cannot serve this actor or this shape. The screen can only be an honest *unavailable* state until the gap in the gap analysis is closed. Never faked. |
| **FUTURE** | Needs a product decision or a capability that is not in the architecture yet (campaign automation, human seller↔customer chat, cart sync…). |

A row can be **IMPLEMENTED** and still carry a limit; the limit is in *Notes*.

### Permission codes (what the **server** enforces — the UI only mirrors it)

| Code | Rule | Enforced in |
|---|---|---|
| `PUBLIC` | No sign-in needed | handler |
| `USER` | Signed in; identity comes from the verified token only (`auth.UserID`) | `auth.RequireUser` |
| `OWN` | The customer who owns the order / reservation / payment | use cases (`canAccessOrder`, `canAccessReservation`), `payment.AccessPolicy` |
| `MEMBER` | Any member of the organization named in the request | `organizationStaffOnly`, `organizationMemberOnly` |
| `MGR` | Organization **owner / admin / manager** (the only org roles that can operate) | `tenantStaffOnly`, use cases |
| `OWNER+ADMIN` | Organization owner or admin | `OrganizationUseCases.AddMember` |
| `OP` | Platform **admin or operator** (roles read from the database on every request) | handlers / use cases |
| `ADMIN` | Platform **admin** only | `AdminHandler.require` |
| `—` | Not applicable (no API) | — |

Platform roles: `customer, seller, staff, admin, operator, moderator, delivery_partner`
(`internal/domain/role.go`). Organization roles: `owner, admin, manager, staff, accountant, sales, operations`
(`internal/domain/membership.go`) — **only owner/admin/manager have any server-side meaning today**; the others are
read-only members.

## 1. Ground truth: what the platform actually exposes

### 1.1 The routed HTTP surface (gateway `internal/gateway/gateway.go`)

The gateway proxies exactly these prefixes. Anything not listed here cannot be reached by a browser.

| Gateway prefix | Service | Endpoints (method path — guard) |
|---|---|---|
| `/api/v1/auth` | identity | `POST request-otp`, `verify-otp` — PUBLIC (IP + per-phone limits) · `POST staff/login` — PUBLIC (lockout) · `POST refresh` · `GET users/me` — USER |
| `/api/v1/admin` | identity | `GET overview` — OP · `GET users` — ADMIN |
| `/api/v1/organizations` | identity | `POST` — OP · `GET ?organization_id=` — MEMBER or OP · `GET/POST/DELETE members` — MEMBER·OWNER+ADMIN·use case |
| `/api/v1/sellers` | identity | `POST` · `GET ?seller_id=` — **PUBLIC (verified live: returns phone and user id)** · `GET me` — USER |
| `/api/v1/stores` | identity | `POST` · `GET ?store_id=` — PUBLIC (returns phone, email, organization id) |
| `/api/v1/search`, `/listings`, `/categories`, `/verticals` | search | `POST search`, `GET search/suggest`, `GET categories`, `GET verticals` — PUBLIC · `POST categories` — admin role · `GET listings?listing_id=` — PUBLIC if published · `POST listings` · `publish\|pause\|resume\|update\|promotion\|promotion/end` — owner/MGR/OP · `GET listings/manage?organization_id=` — MEMBER or OP |
| `/api/v1/conversations`, `/packs`, `/webhooks/whatsapp` | conversation | `POST conversations/message` — PUBLIC or USER · `POST conversations/{id}/confirm\|reject` — confirmation token · `GET packs…` — PUBLIC · `POST packs/{code}/enable\|disable` — OP (**no effect**: `allPacksEnabledRepo`) · `GET/POST webhooks/whatsapp` — verify token / HMAC |
| `/api/v1/orders`, `/reservations`, `/availability`, `/inventory`, `/hotels`, `/fulfillments` | commerce (`order-service`) | `POST orders` (+`Idempotency-Key`) · `GET orders` — OWN · `GET orders/incoming?tenant_id=` — MGR or OP · `GET orders/{id}` — OWN, seller user, MGR, **OP** · `POST orders/{id}/cancel`, `/confirm` · `POST reservations` · `GET reservations` — OWN · `GET reservations/incoming?tenant_id=` — MGR or OP · `GET reservations/{id}`, `POST …/cancel` — OWN or **MGR (not OP)** · `POST reservations/{id}/checkout` · `GET availability` — USER (**verified live**: 401 anonymous) · `GET inventory`, `GET inventory/{id}` — MGR or OP · `GET hotels/property`, `hotels/availability` — PUBLIC · `GET/POST hotels/room-types…`, `units`, `capacity` — owner/MGR · `GET fulfillments/{id}`, `POST …/transition\|assign\|tracking` — MGR or OP |
| `/api/v1/payments`, `/webhooks/payments` | payment | `POST payments?organization_id=` · `GET payments/{id}`, `/status` · `POST …/cancel` · `GET …/refunds/{id}` — OWN or MGR or OP · `POST …/capture`, `/refund` — MGR or OP · `POST webhooks/payments/{provider}` — signature |
| (gateway itself) | — | `GET /healthz`, `GET /readyz` — PUBLIC (**verified live**: `/readyz` returns internal upstream URLs) · `GET /metrics` — only with `METRICS_TOKEN` (404 otherwise, verified live) |

### 1.2 Coded and tested, but **not reachable over HTTP**

| Capability | Where it lives | Why it matters to a portal |
|---|---|---|
| Provider management (list, create, enable/disable, capabilities) | `internal/api/provider_handlers.go` — `ProviderHandler` is **never instantiated** in any `cmd/*` | Admin provider orchestration, seller integrations |
| Provider executions (every attempt, request id, retry class) | table `provider_executions` (000011, 000021) — "no API exposes or changes them" | Admin provider health, latency, failures |
| Usage / credits (balance, ledger, policies, reservations) | `application/usage.go` (used **inside** the conversation runtime for metering); tables `credit_accounts, usage_ledger, usage_policies, usage_reservations` | Seller usage & credits, admin monetization |
| Plans, subscriptions, billing periods, credit grants/purchases | `application/monetization.go`; tables `plans, subscriptions, billing_periods, credit_grants, credit_purchases` | Seller plan, admin monetization |
| Update organization / seller profile / store | `UpdateOrganization`, `UpdateSellerProfile`, `UpdateStore` use cases; `UpdateSellerH` handler exists | Seller business profile and stores |
| Adjust inventory | `InventoryUseCases.AdjustInventory` | Seller inventory management |
| Order lifecycle after confirm (`ReadyForFulfillment`, `CompleteOrder`), fulfilment by order/reservation | `OrderUseCases`, `FulfillmentUseCases.GetFulfillmentByOrder/Reservation` | Seller order tracking (KNOWN_LIMITATIONS L3) |
| Modify reservation | `ReservationUseCases.ModifyReservation` | Customer and seller reservation changes |
| Close / suspend / expire a listing | `Listing.MarkAsClosed/Suspend/MarkAsExpired` | Seller archive, admin moderation |
| AI provider/model/policy store | tables `ai_providers, ai_models, ai_tenant_policies, ai_execution_records` (000012); only `ai_providers` has a repository (`infrastructure/ai/provider_repository.go`, tested but **wired into no service**); the other three have no code | Admin AI control centre (real AI config today is **environment variables**) |
| Dispatch (couriers, jobs, offers, positions) | `application/dispatch`, migration 000022 (uncommitted) | Future delivery operations |
| Per-module audit tables | `inventory_audit`, `capacity_audit`, `fulfillment_audit` | Admin audit |
| Webhook idempotency / failure rows | tables `webhook_events`, `payment_webhook_events` | Admin channel / webhook health |
| Roles and permissions tables | `roles`, `permissions`, `role_permissions`, `user_roles` (000002) | RBAC UI (guards are hard-coded today) |

### 1.3 Placeholder services

`admin-service`, `logistics-service`, `marketing-service`, `notification-service`, `seller-service`,
`trust-service` are 9-line `main.go` files that only log "placeholder". `catalog-service` duplicates the
search-service listing routes and is not routed; `fileserver` is a legacy static server. **There is no
marketing, notification, trust or admin backend.** The only admin API is `GET /admin/overview` and
`GET /admin/users`, hosted by `identity-service`.

### 1.4 Audit and observability reality

* **Audit** (`internal/infrastructure/audit`) writes one structured JSON **log line** per critical action
  (`event="audit"`: actor, action, target, request id). It is "a log trail, not a tamper-proof store". There is
  no audit table and no audit API.
* **Metrics** (`packages/observability/metrics.go`): `trezutil_http_requests_total`, `…_errors_total`,
  `…_request_duration_seconds`, `trezutil_api_errors_total`, `trezutil_payment_failures_total`,
  `trezutil_reservation_failures_total`, `trezutil_provider_failures_total`,
  `trezutil_readiness_failures_total`, `trezutil_webhook_failures_total`. Each service serves its own `/metrics`
  on its internal port; the gateway proxies none of them. Prometheus rules exist (`infrastructure/prometheus/alerts.yml`:
  5xx rate, payment failures, webhook failures, reservation errors, provider failures, not-ready, service down);
  Grafana has no dashboards committed.
* **Health**: each service has `/healthz` and `/readyz` (the DB-backed ones check PostgreSQL; search checks
  OpenSearch) but only the **gateway's** `/readyz` is reachable, and it only TCP-pings upstream hosts.

### 1.5 AI and channels reality

* AI is configured by environment: `AI_PROVIDER` (one of `anthropic`, `gemini`, or a priority list such as
  `gemini,anthropic`), `AI_MODEL`, `GEMINI_MODEL`, `AI_EFFORT`, timeouts, retries, `AI_VERIFY_ON_START`. A failover
  wrapper (`internal/infrastructure/ai/failover`) tries providers in order; model names and keys are validated at
  start. Without a provider, keyword routing runs. Nothing about this is readable over HTTP, and keys must never be.
* WhatsApp is configured by environment (`WHATSAPP_PHONE_NUMBER_ID`, token, app secret, verify token). The
  `channel_accounts` table maps sender ids to users; there is no status endpoint. Delivery of OTP and replies
  depends on a valid token (the local staging token expired on 2026-09-30).

## 2. Customer portal

Purpose: **discover, buy, book and manage my activity.** Existing routes are kept (they are public URLs).

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Sign-in | Phone + code | `/login`, `/register` | `POST /auth/request-otp`, `/auth/verify-otp`, `/auth/refresh`, `GET /auth/users/me` | PUBLIC → USER | IMPLEMENTED | OTP delivery depends on the WhatsApp token (external) |
| Home | Public home, conversational entry, deals, new arrivals | `/` | `POST /search`, `GET /verticals` | PUBLIC | IMPLEMENTED | Chat widget on every page |
| Home | Signed-in home: active orders, upcoming reservations | `/` (variant) | `GET /orders`, `GET /reservations` | USER | IMPLEMENTABLE | Both lists exist (limit/offset, default 20, max 100); no server-side status filter |
| Home | Personalised recommendations | — | — | — | BACKEND GAP | No recommendation endpoint (G-B11) |
| Home / Discover | "Nearby opportunities" by distance | — | — | — | BACKEND GAP | `POST /search` takes a place **name** only; no lat/lng/radius (L8, G-B12) |
| Discover | Verticals, categories, new, homes & stays, deals | `/discover` | `GET /verticals`, `GET /categories`, `POST /search` | PUBLIC | IMPLEMENTED | |
| Discover | Trending, curated collections | — | — | — | BACKEND GAP | No trending/collection endpoint (G-B11) |
| Search | Text, vertical, category, place, sort, on-promotion | `/search` | `POST /search`, `GET /search/suggest` | PUBLIC | IMPLEMENTED | |
| Search | Price range filter | `/search` | `POST /search` (`price_min`, `price_max`) | PUBLIC | IMPLEMENTABLE | Supported by the API, no control in the filter panel |
| Search | Date-aware search (hotels, tables) | — | — | — | BACKEND GAP | Availability is checked per listing, not searched (L8) |
| Listing | Detail adapted by vertical flow (buy / book) | `/products/:id`, `/services/:id`, `/restaurants/:id`, `/hotels/:id`, `/properties/:id` | `GET /listings`, `GET /stores`, `GET /verticals`, `GET /hotels/property`, `GET /hotels/availability` (all PUBLIC); `GET /availability` (**USER**, verified live: 401 anonymous) | PUBLIC / USER | IMPLEMENTED | Reviews/ratings: none exist (L9). A guest sees the listing but must sign in to check dates |
| Compare | Side-by-side comparison | — | — | — | FUTURE | Lifecycle step "COMPARE" is only conversational today |
| Conversations | Full conversation page + floating widget, confirm/reject cards | `/conversation` | `POST /conversations/message`, `POST /conversations/{id}/confirm\|reject` | PUBLIC / USER | IMPLEMENTED | Thread kept in the browser tab |
| Conversations | History, saved conversations | — | — | — | BACKEND GAP | No list or read API (L6, G-B7) |
| Orders | List with Active / Completed / Cancelled, detail, cancel, pay | `/orders`, `/orders/:id` | `GET /orders`, `GET /orders/{id}`, `POST /orders/{id}/cancel`, `POST /payments` | OWN | IMPLEMENTED | Tabs filter client-side |
| Orders | Pending / confirmed / paid sub-filters | `/orders` | `GET /orders` | OWN | IMPLEMENTABLE | Client-side over the status field |
| Orders | Delivery / fulfilment tracking | — | — | — | BACKEND GAP | Orders do not expose their fulfilment id; no transition route for customers (L3, G-A6) |
| Reservations | List (Upcoming / Pending / Past / Cancelled), detail, cancel | `/reservations`, `/reservations/:id` | `GET /reservations`, `GET /reservations/{id}`, `POST …/cancel` | OWN | IMPLEMENTED | Hotel stays use `POST /reservations/{id}/checkout` |
| Reservations | Modify a booking | — | — | — | BACKEND GAP | `ModifyReservation` has no route (G-A7) |
| Booking | Book dates / table / appointment | `/book/:id`, hotel flow | `GET /availability`, `POST /reservations`, hotel endpoints | USER | IMPLEMENTED | |
| Bag | Items, quantities, one order per seller and currency | `/cart` | `POST /orders` (`items`, max 30 lines, `Idempotency-Key`) | USER | IMPLEMENTED | Bag is device-local (L5); grouping by seller is done in the UI |
| Bag | Cart that follows the customer across devices | — | — | — | FUTURE | Server-side cart/wishlist (L5) |
| Saved | Wishlist | `/wishlist` | — (device-local) | — | IMPLEMENTED | Local only |
| Checkout | Review → card payment → confirmation | `/checkout` | `POST /orders`, `POST /payments`, `GET /payments/{id}/status` | OWN | IMPLEMENTED | Card (Stripe) only; **mobile money is not connected** (G-E1) |
| Activity | Unified order + reservation activity | `/notifications` (named *Activity*) | `GET /orders`, `GET /reservations` | USER | IMPLEMENTED | Derived; not a notification feed |
| Notifications | Real notifications (push/email/WhatsApp) | — | — | — | BACKEND GAP | `notification-service` is a placeholder (L7, G-B9) |
| Account | Profile (read), theme, language, sign-out | `/account` | `GET /auth/users/me` | USER | IMPLEMENTED | |
| Account | Edit profile / preferences | — | — | — | BACKEND GAP | No profile-update API (L9, G-B14) |
| Account | Privacy, security, active sessions, revoke | — | — | — | BACKEND GAP | Stateless refresh tokens; no session store (L10, G-C2) |
| Account | Usage / credits | — | — | — | BACKEND GAP | No customer-facing credits API (G-A2); the PRD marks it "where applicable" |

Customer portal must not expose seller/admin menus: today the customer shell has a "Sell on Trezutil" link only,
and `/seller` shows the onboarding page to anyone without an organization (no data is returned).

## 3. Seller portal

Purpose: **run my business on Trezutil.** Existing routes live under `/seller`. Operating actions need role
`MGR`; any member can open the portal and see read-only data.

### 3.1 Dashboard

| Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|
| Dashboard: sales, orders to confirm, upcoming bookings, live listings, recent activity | `/seller` | `GET /sellers/me`, `GET /listings/manage`, `GET /orders/incoming`, `GET /reservations/incoming` | MEMBER / MGR | IMPLEMENTED | Sales are summed client-side from the **first 100** orders only (`limit=100`): wrong for a busy seller; needs paging or a server aggregate (G-B6) |
| Customers metric | — | — | — | BACKEND GAP | No customers API (G-B5) |
| Inventory / availability / fulfilment tiles | `/seller` | `GET /inventory` (read) | MGR | IMPLEMENTABLE | Counts of tracked items only; no low-stock threshold exists (G-A5) |
| Usage / credits tile | — | — | — | BACKEND GAP | G-A2 |

### 3.2 Business

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Business profile | Organization, contact, status, verification (read) | `/seller/business` | `GET /sellers/me` | MEMBER | IMPLEMENTED | Read-only |
| Business profile | Edit legal/contact/description/branding | — | — | MGR | BACKEND GAP | Use cases exist, no routes (G-A4); **no branding/logo field exists** |
| Stores / branches | List, location (read) | `/seller/business` | `GET /sellers/me` | MEMBER | IMPLEMENTED | |
| Stores / branches | Store detail | `/seller/stores/:id` | `GET /stores?store_id=` | PUBLIC | IMPLEMENTABLE | Store has address, lat/lng, phone, email |
| Stores / branches | Create store | `/seller/stores/new` | `POST /stores` | MGR | IMPLEMENTABLE | Authorisation lives in `StoreUseCases` |
| Stores / branches | Edit store, opening hours | — | — | MGR | BACKEND GAP | G-A4; no opening-hours model |
| Verticals | The verticals this business operates | `/seller/verticals` | `GET /verticals`, `GET /listings/manage` | MEMBER | IMPLEMENTABLE | Derived: distinct `vertical` of the organization's listings (documented model) |
| Verticals | Explicit activation / deactivation | — | — | OP | FUTURE | `organization_verticals` is a documented future table (G-C3) |

### 3.3 Catalog

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Listings | List with status tabs | `/seller/listings` | `GET /listings/manage?organization_id=` | MEMBER | IMPLEMENTED | Search/filter/sort beyond the tabs: IMPLEMENTABLE client-side (limit 100) |
| Listings | Create (6 steps, vertical-aware) | `/seller/listings/new` | `POST /listings` (+media, availability), `GET /verticals`, `GET /categories` | MGR | IMPLEMENTED | Per-vertical forms beyond type/category: BACKEND GAP for attributes (no attribute API) |
| Listings | Edit title/description/price | `/seller/listings/:id/edit` | `POST /listings/update` | MGR | IMPLEMENTED | Draft/pending/paused only (server rule); category, media, vertical, locations **not editable** (G-A4) |
| Listings | Publish, pause, resume | list + detail | `POST /listings/publish\|pause\|resume` | MGR | IMPLEMENTED | |
| Listings | Promotion (sale price) | `/seller/listings/:id` | `POST /listings/promotion`, `/promotion/end` | MGR | IMPLEMENTED | Works on a live listing |
| Listings | Archive / close | — | — | MGR | BACKEND GAP | `MarkAsClosed` has no route (G-A8) |
| Listings | Detail | `/seller/listings/:id` | `GET /listings`, `GET /inventory` | MEMBER | IMPLEMENTED | |
| Categories | Browse | (in editor) | `GET /categories` | PUBLIC | IMPLEMENTED | |
| Categories | Create / edit | — | `POST /categories` | ADMIN | BACKEND GAP for sellers | Only platform admins may create; no update/delete (G-A10) |
| Media | Image links | (in editor) | `POST /listings` `media[].URL` | MGR | IMPLEMENTED | https links only |
| Media | Upload / library | — | — | — | BACKEND GAP | No storage or upload endpoint (L4, G-C6) |

### 3.4 Inventory and availability (vertical-adaptive)

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Inventory | Stock list (total / available / reserved / sold) | `/seller/inventory` | `GET /inventory?tenant_id=` | MGR | IMPLEMENTED | Read-only |
| Inventory | Adjust stock | — | — | MGR | BACKEND GAP | `AdjustInventory` has no route (G-A5) |
| Inventory | Low-stock indicator | — | — | MGR | BACKEND GAP | No threshold concept in the data model; a number invented in the UI would be fake |
| Inventory | History | — | — | MGR | BACKEND GAP | `inventory_audit` is not exposed (G-C1) |
| Hotel | Property, room types, units, rates, nightly capacity | `/seller/hotel` | `GET /hotels/property`, `GET/POST /hotels/room-types…`, `POST …/units`, `GET/POST …/capacity` | MGR | IMPLEMENTED | The only vertical with a capacity API |
| Availability | Calendar of nights per room type | `/seller/hotel` | `GET /hotels/room-types/{id}/capacity?from&to` | MGR | IMPLEMENTED (hotel) | |
| Availability | Calendar for other verticals, blocked periods | — | `GET /availability` (check only) | MGR | BACKEND GAP | No create/adjust capacity API outside hotels (L2, G-A5/G-B13) |
| Restaurant | Tables, seating capacity | — | — | MGR | BACKEND GAP | "RESTAURANT pack next" (L2) |
| Services | Appointment slots, staff capacity | — | — | MGR | BACKEND GAP | |
| Vehicle rental | Fleet | — | — | MGR | BACKEND GAP | |

### 3.5 Orders, reservations, fulfilment

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Orders | Incoming orders, tabs (To confirm, Awaiting payment, In progress, Completed, Cancelled) | `/seller/orders` | `GET /orders/incoming?tenant_id=` | MGR | IMPLEMENTED | |
| Orders | Confirm a paid order | `/seller/orders` | `POST /orders/{id}/confirm` | MGR (seller-only in the use case) | IMPLEMENTED | |
| Orders | Order detail drawer (lines, payment, customer) | — | `GET /orders/{id}` | MGR | IMPLEMENTABLE | Seller sees the customer **id** only; display policy undefined (G-B5) |
| Orders | Ready for fulfilment / complete / tracking | — | `POST /fulfillments/{id}/transition\|assign\|tracking` exist | MGR | BACKEND GAP | Orders do not return their fulfilment id; no list route (L3, G-A6) |
| Orders | Server-side search/filter/paging | — | — | MGR | BACKEND GAP | Only `limit`/`offset` (G-B6) |
| Reservations | Incoming list (Pending, Confirmed, Past, Cancelled) | `/seller/reservations` | `GET /reservations/incoming?tenant_id=` | MGR | IMPLEMENTED | Bookings auto-confirm (policy `AutoConfirm: true`) |
| Reservations | Calendar view | `/seller/reservations` | same | MGR | IMPLEMENTABLE | Client-side over loaded rows |
| Reservations | Detail, cancel | — | `GET /reservations/{id}`, `POST …/cancel` | MGR | IMPLEMENTABLE | Vertical-specific fields (rooms/nights, party size, appointment) come from the reservation payload |
| Reservations | Manual confirm / reject | — | — | MGR | BACKEND GAP | No confirm route; auto-confirm only |

### 3.6 Customers, conversations, finance, analytics

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Customers | Customer list, history | — | — | MGR | BACKEND GAP | No endpoint, no privacy policy for what a seller may see (G-B5) |
| Conversations | Seller inbox (customer ↔ business) | — | — | MGR | BACKEND GAP | Conversations are customer ↔ Trezutil assistant; there is no seller-addressed conversation model and no list API (G-B8) — **FUTURE** product concept |
| Conversations | Confirm/reject supported commerce actions | — | `POST /conversations/{id}/confirm\|reject` | token | IMPLEMENTABLE | Confirmation tokens go to the acting customer; a seller-side queue does not exist |
| Finance | Revenue from orders | `/seller/finance` | `GET /orders/incoming` | MGR | IMPLEMENTABLE | Same 100-row limit as the dashboard |
| Finance | Payments list, status per order | — | — | MGR | BACKEND GAP | Payments are readable **one at a time** by id; no list by org or order (G-B4) |
| Finance | Refunds | — | `POST /payments/{id}/refund` | MGR | IMPLEMENTABLE | Needs a payment id the UI cannot list yet |
| Finance | Settlements, balances, reconciliation | — | — | MGR | BACKEND GAP | `payment_ledger` table, no API (G-B4) |
| Analytics | Sales, orders, reservations, conversion, listing performance | — | — | MGR | BACKEND GAP | No aggregation endpoints (G-B6). Only counts derivable from lists may be shown, labelled as such |

### 3.7 Growth, usage, team, integrations, settings

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Usage & credits | Plan, balance, usage, history, limits | `/seller/usage` | — | MGR | BACKEND GAP | `UsageService`, `PlanService` exist; no HTTP (G-A2, G-A3) |
| Marketing | Promotions (sale prices) | `/seller/marketing/promotions` | `POST /listings/promotion` | MGR | IMPLEMENTED (via listing) | The only marketing capability that exists |
| Marketing | Campaigns, automations, segments, abandoned intent, WhatsApp campaigns, messaging, analytics | `/seller/marketing/*` | — | MGR | FUTURE | `marketing-service` is a placeholder; no tables; outbound WhatsApp exists only for OTP/replies (G-C4) |
| Marketing | Recommendations | — | — | MGR | FUTURE | |
| Team | Members and roles (list) | `/seller/team` | `GET /organizations/members` | MEMBER | IMPLEMENTABLE | Response has user ids, not names (G-D2) |
| Team | Add member | `/seller/team` | `POST /organizations/members` (`user_id`, `membership_role`) | OWNER+ADMIN | IMPLEMENTABLE | The API needs a **user id**; there is no lookup by phone/email (G-D2) |
| Team | Remove member | `/seller/team` | `DELETE /organizations/members` | OWNER+ADMIN | IMPLEMENTABLE | |
| Team | Permission matrix per role (accountant, sales, operations) | — | — | — | BACKEND GAP | Those roles exist as values but no guard treats them differently (G-D3) |
| Integrations | Stripe / payment provider status | — | — | MGR | BACKEND GAP | `ProviderHandler` unmounted; configured by operators (G-A1) |
| Integrations | WhatsApp channel status | — | — | MGR | BACKEND GAP | Environment-configured; no status API (G-C7). Never show tokens |
| Settings | Language, theme (device) | `/account` | — | USER | IMPLEMENTED | |
| Settings | Business settings, notifications, security, preferences | — | — | MGR | BACKEND GAP | G-A4, G-B9, G-C2 |

## 4. Admin portal

Purpose: **operate and govern Trezutil.** Sign-in is the staff email + password flow (`/auth/staff/login`);
the route `/admin` exists with two screens. Every call below is re-authorised by the server per request.

### 4.1 Platform overview

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Executive dashboard | Users (total, active, suspended, new 7 d, by role), organizations, listings by status, orders (total, 7 d), reservations, conversations 7 d | `/admin` | `GET /admin/overview` | OP | IMPLEMENTED | Counts only; nothing invented |
| Executive dashboard | Payments, platform usage, provider health, AI usage, channel health | — | — | OP | BACKEND GAP | No aggregate endpoints (G-B2, G-A1, G-B10, G-C7) |
| Executive dashboard | Service reachability | `/admin` | `GET /readyz` | PUBLIC | IMPLEMENTABLE | Per-service TCP reachability only (see 4.6) |

### 4.2 People and organizations

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Users | Search, list (name, email, phone, status, roles, last sign-in) | `/admin/users` | `GET /admin/users?q=&page=&page_size=` | ADMIN | IMPLEMENTED | Shows personal data: admin only by design |
| Users | User detail | `/admin/users/:id` | — | ADMIN | BACKEND GAP | No get-by-id; list row is all there is (G-D1) |
| Users | Suspend / reactivate, grant or revoke role | — | — | ADMIN | BACKEND GAP | Only the `trezutil-admin` CLI changes accounts (G-D1) |
| Users | Sessions | — | — | ADMIN | BACKEND GAP | G-C2 |
| Organizations / sellers | List and search all organizations | `/admin/organizations` | — | OP | BACKEND GAP | `GET /organizations` returns one org by id, membership-guarded (G-B1) |
| Organizations / sellers | Operator-assisted onboarding wizard (org → seller profile → store → owner) | `/admin/organizations/new` | `POST /organizations`, `POST /sellers`, `POST /stores`, `POST /organizations/members` | OP | IMPLEMENTABLE | All four routes exist; the owner must be picked from `GET /admin/users` ids |
| Organizations / sellers | Org detail: stores, members, verticals, activity | `/admin/organizations/:id` | `GET /organizations`, `/organizations/members`, `/listings/manage`, `/orders/incoming`, `/reservations/incoming` | OP | IMPLEMENTABLE | **Once an id is known** (no list); `OP` passes every one of these guards |
| Organizations / sellers | Verification / status changes | — | — | OP | BACKEND GAP | G-A4, G-D1 |

### 4.3 Catalog and vertical governance

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Verticals | Catalogue: capabilities, flows, listing types, concepts | `/admin/verticals` | `GET /verticals`, `GET /packs`, `GET /packs/{code}` | PUBLIC | IMPLEMENTABLE | Read-only **by design**: definitions live in code (`vertical.DefaultCatalog()`) |
| Verticals | Listings per vertical | `/admin/verticals` | `POST /search` (`total` with `page_size: 1`, one call per vertical) | PUBLIC | IMPLEMENTABLE | Published listings only |
| Verticals | Active sellers per vertical | — | — | OP | BACKEND GAP | G-B1 |
| Verticals | Enable / disable a pack | — | `POST /packs/{code}/enable\|disable` | OP | FUTURE | Accepts the call but **has no effect** (`allPacksEnabledRepo`); do not present a working switch |
| Categories | List, create | `/admin/categories` | `GET /categories`, `POST /categories` | ADMIN (role check) | IMPLEMENTABLE | No update/delete (G-A10) |
| Attributes | Definitions | — | — | — | BACKEND GAP | Table exists, no API (G-A10) |
| Listings | Moderation queue, all statuses, suspicious listings, quality | `/admin/listings` | — | OP | BACKEND GAP | No cross-organization list; `listings/manage` needs an organization id (G-B3) |
| Listings | Publish / pause / resume a known listing | `/admin/listings/:id` | `GET /listings`, `POST /listings/publish\|pause\|resume` | OP | IMPLEMENTABLE | Admin/operator passes `canManageListing`; **suspend has no route** (G-A8) |

### 4.4 Commerce operations

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Orders | Platform-wide search and filters | `/admin/orders` | — | OP | BACKEND GAP | No platform list (G-B2) |
| Orders | Orders of one organization | `/admin/orders` (org picker) | `GET /orders/incoming?tenant_id=` | OP | IMPLEMENTABLE | After choosing an organization id |
| Orders | Order detail (seller, customer id, payment state) | `/admin/orders/:id` | `GET /orders/{id}` | OP | IMPLEMENTABLE | `OP` is allowed by `canAccessOrder` |
| Reservations | Platform-wide search | `/admin/reservations` | — | OP | BACKEND GAP | G-B2 |
| Reservations | Reservations of one organization | `/admin/reservations` | `GET /reservations/incoming?tenant_id=` | OP | IMPLEMENTABLE | |
| Reservations | Reservation detail | `/admin/reservations/:id` | `GET /reservations/{id}` | OP | BACKEND GAP | **Platform staff is refused**: `canAccessReservation` knows only the customer and org owner/admin/manager (security-permission gap S-5) |
| Payments | List, captures, refunds, failures, reconciliation | `/admin/payments` | — | OP | BACKEND GAP | G-B4 |
| Payments | One payment (and its refunds) | `/admin/payments/:id` | `GET /payments/{id}`, `GET …/refunds/{id}` | OP | IMPLEMENTABLE | Needs `organization_id` and payment id |
| Payments | Capture / refund | — | `POST /payments/{id}/capture\|refund` | OP | IMPLEMENTABLE | Behind a confirmation dialog and the idempotency keys the API expects |
| Fulfilment | Fulfilment by id; transition / assign / tracking | — | `GET /fulfillments/{id}`, `POST …` | OP | IMPLEMENTABLE | No way to find the id from an order (G-A6) |

### 4.5 Platform: providers, AI, channels, monetization, search

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Provider orchestration | Providers, capabilities, health, enable/disable | `/admin/providers` | — | OP | BACKEND GAP | Handlers unmounted (G-A1); providers are **per-organization** rows |
| Provider orchestration | Executions, failures, latency, retries, fallback | — | — | OP | BACKEND GAP | `provider_executions` has no API; metric `trezutil_provider_failures_total` is internal (G-A1) |
| Provider orchestration | Native vs external supply | `/admin/providers` | — | OP | FUTURE | Docs: native sellers are first-class; external supply is later and optional |
| AI control centre | Providers, models, routing, usage, failures, latency, cost, safety | `/admin/ai` | — | OP | BACKEND GAP | Config is env-only; the `ai_*` tables are not used by any service; no AI usage endpoint (G-B10). Keys are never shown |
| Channels | Web, Mobile, WhatsApp status; webhook and delivery health; templates | `/admin/channels` | — | OP | BACKEND GAP | Env-configured; `webhook_events` not exposed (G-C7) |
| Monetization | Plans, subscriptions, credit balances, usage ledger, usage policies | `/admin/monetization` | — | OP | BACKEND GAP | Use cases and tables exist, no HTTP (G-A2, G-A3). Policies are not seeded: **metering is off** until rows exist |
| Search & discovery | Index health, indexing lag, failures, query metrics | `/admin/search` | — | OP | BACKEND GAP | Search service `/readyz` checks OpenSearch but is not routed; no index stats API (G-D5) |

### 4.6 Operations, audit, security, health, settings

| Module | Screen | Route | API | Permission | Status | Notes |
|---|---|---|---|---|---|---|
| Operations | Failed orders / reservations / payments / webhooks, support cases | `/admin/operations` | — | OP | BACKEND GAP | No incident store or aggregation (G-B2, G-C8) |
| Audit | Actor, action, resource, time, request id, outcome | `/admin/audit` | — | OP | BACKEND GAP | Audit exists only as log lines; **do not fake a table** (G-C1). Show an explicit "log trail only" limitation |
| Security | Authentication, sessions, suspicious activity, rate limits, webhook security | `/admin/security` | — | OP | BACKEND GAP | G-C2, G-D7 |
| System health | Gateway and per-service reachability | `/admin/health` | `GET /readyz`, `GET /healthz` | PUBLIC | IMPLEMENTABLE | TCP ping only; also exposes internal URLs (S-6) — a staff-gated endpoint is preferred |
| System health | Database, OpenSearch, queue, providers, AI, webhooks, metrics | `/admin/health` | — | OP | BACKEND GAP | G-D5, G-D6 |
| Settings | Platform settings, feature flags, localization, operational config | `/admin/settings` | — | OP | BACKEND GAP | None exist (G-C5) |

## 5. Portal entry and switching

| Capability | API | Status | Notes |
|---|---|---|---|
| Know the platform roles | `GET /auth/users/me` → `roles[]` | IMPLEMENTED | Already used by the admin guard |
| Know which businesses a user belongs to and with which role | `GET /sellers/me` → organizations, role, stores, seller profiles | IMPLEMENTED | Already used by the seller shell |
| Compute available portals and offer a switcher | the two calls above | IMPLEMENTABLE | Pure frontend; the server stays authoritative |
| Single sign-in for staff and customers | `POST /auth/verify-otp` vs `POST /auth/staff/login` | IMPLEMENTED (two flows) | A user with both a phone and a staff password needs a clear entry choice |

## 6. Existing frontend inventory (`web/frontend`, React 19, React Router 7, Vite 8, Tailwind 4)

### 6.1 Routes today

Customer/public: `/`, `/discover`, `/search`, `/conversation`, listing details (`/products|services|restaurants|hotels|properties|real-estate|listings/:id`), `/cart`, `/wishlist`, `/login`, `/register`, `/403`; protected by `RequireSession`: `/checkout`, `/book/:id`, `/orders[/:id]`, `/reservations[/:id]`, `/account`, `/notifications`.
Seller (inside `RequireSession`): `/seller`, `/seller/listings`, `/seller/listings/new`, `/seller/listings/:id`, `/seller/listings/:id/edit`, `/seller/orders`, `/seller/reservations`, `/seller/inventory`, `/seller/hotel`, `/seller/business`; and public `/seller/onboarding`.
Admin: `/admin/login`, `/admin` (overview), `/admin/users`.

### 6.2 Reusable building blocks (what exists, what the portals can reuse)

| Area | Existing | Reuse verdict |
|---|---|---|
| Primitives (`components/ui.tsx`) | `Icon` (29 stroke icons), `Button` (busy/pressed), `ActionLink`, `Heading`, `Brand`, `PageIntro`, `Tabs`, `Field`, `StatusBadge` + `statusLabel` | Reuse; restyle per portal. `StatusBadge` maps 19 order/reservation/listing statuses to four tones |
| States (`components/states.tsx`) | `EmptyState`, `ErrorState` (maps API errors), `Loading`, `SkeletonGrid`, `OfflineBanner` | Reuse; add `UnavailableState` (backend dependency) |
| Customer shell (`components/shell.tsx`) | `AppShell`, navbar, mobile menu drawer, `ConversationDock`, `LanguageSwitch` | Customer only |
| Seller shell (`pages/seller/SellerShell.tsx`) | `SellerLayout` (loads `/sellers/me`, org picker, `useSeller()` with `canManage`), sidebar + mobile tabs, `Metric`, `SellerHead`, `ViewOnly` | Basis for the seller shell; today **wrapped inside the customer `AppShell`** (customer navbar visible) |
| Admin shell (`pages/admin/AdminShell.tsx`) | Sidebar, header with role, guard (`isStaff`), `admin-*` styles | Basis for the admin shell |
| Cards (`components/cards.tsx`, `promo.tsx`) | `ResultCard`, `ListingImage`, `SaveButton`, `PriceTag`, `DiscountBadge`, showcase/deals blocks | Customer only |
| Data hooks (`lib/`) | `useAsync` (loading/error/reload, stale-response guard), `useVerticals` (cached catalogue), `useSession`, `usePrefs` (theme, saved, bag), `useToast`, i18n (`t`, `tn`, FR/EN, test-enforced coverage), `format` (money, dates, safe image) | Reuse as is |
| API clients (`api/`) | Tolerant mappers (PascalCase ↔ snake_case), single-flight token refresh, idempotency keys, typed `ApiError`; `seller.ts`, `admin.ts`, `hotels.ts`, `commerce.ts`, `listings.ts`, `search.ts` | Reuse; add per-portal modules |
| **Missing for the portals** | `DataTable` (sort/filter/paginate/responsive), `FilterBar`, `DetailDrawer`/`DetailPanel`, `ConfirmationDialog`, `Timeline`/`ActivityFeed`, `Calendar`, `Chart`, `Breadcrumbs`, `Pagination`, `CommandBar`, form sections with dirty-state guard, portal switcher, `UnavailableState` | New in Phase 1+ (see the IA document) |

### 6.3 Tests today

Vitest + Testing Library + happy-dom: 134 tests in 11 files. Coverage by area:

| Area | Covered today | Not covered (needed by the brief) |
|---|---|---|
| Customer | auth, search, listing/vertical detail, conversation + widget, order + checkout, bag, hotel stay, reservations basics, promotions | account, direct-URL protection |
| Seller | workspace listings/publish/create, hotel rooms, vertical choice, promotion panel | dashboard metrics, inventory, availability, orders confirm, reservations, usage, **permissions (view-only staff is partly covered)** |
| Admin | sign-in, console, users (16 tests) | everything else (does not exist) |
| Security | staff sign-in lockout messaging, forbidden page | customer→seller, seller→admin, cross-organization, direct URL |
| E2E | `test/e2e/web` Playwright (real stack, 16 journeys) and `test/e2e/journeys.py` | portal journeys |

## 7. Status totals

| Portal | IMPLEMENTED | IMPLEMENTABLE | BACKEND GAP | FUTURE | Rows |
|---|---|---|---|---|---|
| Customer | 14 | 3 | 11 | 2 | 30 |
| Seller | 19 | 13 | 27 | 3 | 62 |
| Admin | 2 | 14 | 24 | 2 | 42 |
| Portal entry | 3 | 1 | 0 | 0 | 4 |
| **All** | **38** | **31** | **62** | **7** | **138** |

Rows are the screens/capabilities in sections 2–5 (one status per row). "Implemented" means a screen exists today; it can still carry a limit stated in *Notes*. The large **backend gap** share, concentrated in the seller finance/analytics/usage/marketing modules and almost the whole admin platform area, is the main finding of this audit (see the gap analysis).

## 8. Phase 1 delta (2026-10-04)

Sections 1–7 are the Phase 0 audit and are kept as written. Phase 1 changed the **frontend only** (no backend change); the status of the rows above did not move, because Phase 1 builds the shell, not data screens. What changed:

| Item | Before | After Phase 1 |
|---|---|---|
| Seller frame | Inside the customer `AppShell` (customer navbar visible) | Own `PortalShell`, light rail, no customer navbar |
| Admin frame | Bespoke `admin-*` layout | The same `PortalShell`, navy rail |
| Navigation source | Hard-coded links per shell | One declarative registry, `src/portal/modules.ts`, filtered by role |
| Portal switcher | None | `PortalSwitcher`, driven by `usePortals()` (`GET /auth/users/me` + `GET /sellers/me`) |
| Routes | 10 seller, 3 admin | 20 seller and 20 admin module routes; **every module in the registry has a route** |
| Missing components (§6.2) | `DataTable`, `FilterBar`, `DetailDrawer`, `ConfirmationDialog`, `Breadcrumbs`, `Pagination`, `UnavailableState`, switcher | Built (`src/components/portal/`). Still missing: `Timeline`, `Calendar`, `Chart`, `CommandBar`, dirty-state form sections |
| Tests (§6.3) | 134 tests in 11 files; no portal security tests | 361 tests in 16 files, including the customer→seller, customer→admin, seller→admin, cross-business and direct-URL rows |

### Module routes and what each renders

`ready` = a screen exists and uses a routed API; `planned` = the API exists, the screen is not built (the page lists the endpoints it will use); `unavailable` = the platform lacks the capability (the page names the gap in [PORTALS_BACKEND_GAP_ANALYSIS.md](PORTALS_BACKEND_GAP_ANALYSIS.md) and shows nothing else).

| Portal | ready | planned | unavailable | Total |
|---|---|---|---|---|
| Seller | 8: dashboard, listings, orders, reservations, inventory, hotel rooms, business profile, **analytics** | 3: verticals, team, settings | 9: inbox, categories and media, availability, fulfilment, marketing, customers, finance, usage and credits, integrations | 20 |
| Administration | 3: overview, users, **site content** | 8: organizations, verticals, categories, listings, orders, reservations, payments, system health | 9: providers, AI control centre, channels, monetization, search and indexing, incidents and support, audit, security, platform settings | 20 |

The registry is the single list: the rail, the routes and this table are all derived from it, and a test fails if a module has no route, a duplicate route, an undefined gap id, or a gap id that nothing cites.

### Dashboards and site content (2026-10-04)

Built after Phase 1, with a small backend addition (migration `000024`, four read endpoints, one write endpoint, one upload). Contracts, definitions and tests: [DASHBOARDS_AND_SITE_CONTENT.md](DASHBOARDS_AND_SITE_CONTENT.md).

| Row of §3–§4 | Before | Now |
|---|---|---|
| Seller · Analytics (sales, orders, reservations, listing performance) | BACKEND GAP | **IMPLEMENTED** for owners, admins and managers (`GET /sellers/analytics`) |
| Seller · Dashboard figures | computed from the first 100 orders and bookings | counted by the platform for the whole window, with the previous period for comparison |
| Admin · Executive dashboard (activity, revenue, growth) | counts only | **IMPLEMENTED**: KPIs, per-day charts and breakdowns (`GET /admin/analytics`) |
| Admin · Site content (new module) | — | **IMPLEMENTED**: live figures, partner logos, home layout, with preview, publish and version check |

Totals in §7 are unchanged by design (they describe the Phase 0 audit); the Analytics row moves from BACKEND GAP to IMPLEMENTED.
