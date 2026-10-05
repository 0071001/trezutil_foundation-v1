# Portals — information architecture (Phase 0)

> **Phase 0 — design and planning only.** No code was written. This document defines how the three portals
> differ, how a user reaches the right one, the navigation of each, the shared design language, the honesty
> rules for data, the implementation order and the **exact Phase 1 scope**.
>
> Facts about what the platform can and cannot do come from
> [PORTALS_IMPLEMENTATION_MAP.md](PORTALS_IMPLEMENTATION_MAP.md); every missing capability has a contract in
> [PORTALS_BACKEND_GAP_ANALYSIS.md](PORTALS_BACKEND_GAP_ANALYSIS.md). Written 2026-10-04.

Legend used in the navigation trees: **●** implemented today · **◐** partly (read-only, or one vertical) ·
**○** implementable (API exists, screen to build) · **◌** backend gap (honest unavailable state) · **◇** future.

## 1. Principles

1. **One Commerce Core, three control planes.** The portals are different *views and permissions* over the same
   APIs. No portal gets its own engine, table or cache of business logic.
2. **The server decides.** Every route guard and hidden button is a convenience; each request is authorised
   server-side (roles are re-read from the database per request). The UI never claims more than it was allowed.
3. **Real data or an honest state.** A screen is built only on an API that exists. Where it does not, the screen
   is an *unavailable* state that names the missing capability and links to its contract. No placeholder numbers,
   charts, campaigns, audit rows or health lights.
4. **Distinct, not themed copies.** Customer, seller and admin differ in purpose, primary object, rhythm, density,
   navigation model and visual signature (section 2) — not only in colour.
5. **The existing work is the base.** The customer web app is Figma-faithful and stays that way; the seller and
   admin consoles already exist and are migrated into a shared shell, not rewritten.

## 2. Three control planes, side by side

| | **Customer** | **Seller** | **Admin** |
|---|---|---|---|
| Purpose | Use Trezutil to discover, buy, book and manage my activity | Run my business on Trezutil | Operate and govern Trezutil |
| Primary object | A need → a listing → my order / reservation | A business (organization) and its operations queue | The platform: people, businesses, money, providers, health |
| Core question | "What can I get, and where is mine?" | "What needs me now, and how is my business doing?" | "Is the platform healthy, fair and compliant?" |
| Rhythm | Short, intent-driven sessions; mobile first | Daily operating loop (confirm, fulfil, restock, reprice) | Investigation and intervention; exceptions first |
| Navigation | Top bar + conversation entry; bottom tabs on phones | Left rail grouped by job (Run · Sell · Operate · Grow · Money · Business) with business switcher | Always-dark left rail grouped by domain (People · Catalog · Commerce · Platform · Operations), command bar |
| Dominant layout | Imagery-led pages, cards, rails | Queues, tables with detail drawers, calendars, split views | Dense tables, detail panels, timelines, key/value inspectors |
| Density | Spacious | Comfortable, compact on demand | Compact by default |
| Typography accent | Large display headings (Figma scale) | Manrope headings, **Inter with tabular figures** for money and counts | Same families; identifiers in a monospace stack |
| Colour role | Brand blue + mint, imagery | Ice surfaces, blue for primary action, mint for confirmed/available, **coral only for "needs attention"** | Navy rail, neutral surfaces; status by text + dot, colour only as signal |
| Writes | Place/cancel orders and bookings, pay | Operate the business within the member's role | Few, deliberate, confirmed, attributable actions |
| Never shown | Another user's data; business or admin menus | Another organization's data; platform data | Secrets, tokens, provider credentials |

## 3. Portal entry, role resolution and switching

### 3.1 Resolution (all data already exists)

```
GET /auth/users/me      → roles[]            (customer, seller, staff, admin, operator, moderator, delivery_partner)
GET /sellers/me         → organizations[]    (organization, member role, stores, seller profiles)

portals =
  customer                                   always, once signed in
  seller   if organizations.length ≥ 1       role in org: owner|admin|manager  → can operate
                                             anything else                     → read-only
  admin    if roles ∩ {admin, operator, moderator}   (moderator: no screens until the backend has some)
```

* One portal → open it. Several → open the last used one and show the **portal switcher** (a menu in the header
  listing only the portals the user holds, and for sellers the business picker).
* The result is computed once per session in a `usePortals()` hook and re-computed on sign-in/out and when another
  tab changes the session. It is **advisory**: it chooses menus and landing pages; it grants nothing.

### 3.2 URL spaces and sign-in

| Space | Prefix | Sign-in | Guard |
|---|---|---|---|
| Customer / public | `/` and existing paths (`/orders`, `/reservations`, `/account`, …) — kept, they are public URLs | Phone code (`/login`) | `RequireSession` for personal pages |
| Seller | `/seller/*` (kept) | Same phone code | `RequirePortal('seller')` → no organization → onboarding explanation (no data) |
| Admin | `/admin/*` (kept) | Staff email + password (`/admin/login`) | `RequirePortal('admin')` → 403 page for a non-staff user |

A customer who opens `/seller` or `/admin` directly gets a 403 / explanation page; an anonymous visitor is sent
to the right sign-in with `returnTo`. Direct URL access to another organization's data is refused by the server
(403), and the UI shows the same Forbidden state — it is tested (section 12).

### 3.3 Guard layers (defence in depth, in this order)

1. **Portal resolver** — which shells exist for this user.
2. **Route guard** — redirects/403 before rendering a portal.
3. **Permission-aware controls** — an action the member's role cannot perform is hidden, or shown disabled with
   the reason ("Only owners, admins and managers can confirm orders"). Never merely hidden when the user would
   wonder where it went.
4. **Server authorisation** — the only real boundary. `401` → sign-in; `403` → Forbidden state; the UI never
   retries a refused action.
5. **Cache hygiene** — on sign-out and on business switch all portal state is dropped (the seller shell already
   re-keys its outlet by organization id).

## 4. Customer portal

**Shell.** The existing customer shell: top bar (Discover · Products · Services · Real estate · Reservations),
account menu, language and theme, conversation entry (floating widget and dock), bottom tab bar on phones
(Home · Discover · Ask · Orders · Account). Unchanged visually — it follows the Figma export.

```
Home            ● public home · ○ signed-in strip (active orders, upcoming reservations) · ◌ recommendations · ◌ nearby by distance
Discover        ● verticals, categories, new, homes & stays, deals · ◌ trending / collections
Search          ● text · vertical · category · place · sort · on promotion · ○ price range · ◌ date-aware
Ask (Conversations)  ● page + widget, confirm/reject · ◌ history / saved
Orders          ● list, detail, cancel, pay · ○ finer status filters · ◌ fulfilment tracking
Reservations    ● list, detail, cancel · ◌ modify
Bag             ● device-local · ◇ cart across devices
Saved           ● device-local
Activity        ● derived from orders + reservations (route /notifications)
Notifications   ◌ (no service)
Account         ● profile (read), theme, language, sign out · ◌ edit · ◌ privacy/security/sessions · ◌ usage/credits
```

Customer patterns: pages are composed of imagery-led cards; lists (orders, reservations) are rows with a status
badge and one primary action; details use a timeline derived from the real status; mobile uses bottom sheets for
filters and the existing drawer for the menu.

**What changes in Phase 2** (no redesign): a signed-in home strip; order/reservation sub-filters; price-range
filter; an Account area split into Profile / Preferences (language, theme) / Security (honest unavailable
states) ; direct-URL and navigation tests. Everything else stays.

## 5. Seller portal — "run my business"

**Shell (new, replaces the customer navbar inside `/seller`).** Left rail 248 px (drawer under 900 px, bottom tab
bar of the five most-used modules on phones), business switcher at the top, a header with breadcrumbs, the page
title, one primary action, and the member's role chip ("Manager"). A quiet "Today" band on the dashboard lists
what needs action. Role-aware: members below manager see a persistent, dismissible "view only" notice and
disabled actions with reasons.

```
RUN
  Dashboard              ●  sales (latest orders), orders to confirm, bookings, live listings, recent activity  (fix: paging)
  Inbox                  ◇  needs a seller-addressed conversation model
SELL
  Listings               ●  tabs · ○ search/filter/sort · ○ detail drawer · ◌ archive
    Promotions           ●  sale prices (per listing)
  Categories & media     ◐  browse categories, image links · ◌ upload · ◌ manage categories
OPERATE
  Orders                 ●  queue + confirm · ○ detail drawer · ◌ ready/complete/tracking
  Reservations           ●  queue · ○ calendar · ○ detail/cancel · ◌ manual confirm
  Inventory              ●  read-only list · ◌ adjust · ◌ low-stock · ◌ history
  Availability           ◐  hotel nights ● · other verticals ◌
  Hotel rooms            ●  room types, units, rates, nightly capacity   (appears only for HOTEL sellers)
  Fulfilment             ◌
GROW
  Marketing              ◇  campaigns, automations, segments, abandoned intent, messaging, WhatsApp, analytics (Promotions ● lives here too)
  Analytics              ◌
  Customers              ◌
MONEY
  Finance                ◐  revenue from orders ○ · payments list ◌ · refunds (by id) ○ · settlements ◌
  Usage & credits        ◌
BUSINESS
  Business profile       ◐  read ● · edit ◌
  Stores                 ◐  list ● · detail ○ · create ○ · edit ◌
  Verticals              ○  derived from listings + catalogue
  Team                   ○  list · add (needs a user id) · remove
  Integrations           ◌
  Settings               ◐  language, theme ● · the rest ◌
```

Implementation priority follows the brief's order (Dashboard → Business → Stores → Verticals → Listings →
Inventory → Availability → Orders → Reservations → Customers → Conversations → Finance → Analytics → Usage →
Marketing → Team → Integrations → Settings); the rail groups them by *job*, which is how a business thinks.

### 5.1 Vertical-adaptive behaviour

The seller never "has one vertical". The set is derived from the organization's listings
(`SELECT DISTINCT vertical`, documented model) intersected with `GET /verticals`. Rules:

| Rule | Behaviour |
|---|---|
| A vertical-specific module appears in the rail only if the business operates that vertical **and** the module has an API | HOTEL → *Hotel rooms* (exists). RESTAURANT → *Tables*, SERVICES → *Appointments*, VEHICLE_RENTAL → *Fleet*: not built (no capacity API, L2) — not shown; the vertical's gap is listed on the Verticals page |
| Create-listing forms adapt to the chosen vertical's `listing_types` and `flows` (BUY / BOOK) | Exists for type and category; attributes need an API (gap) |
| Reservation rows show vertical-relevant fields | Hotel: rooms × nights; restaurant: party size; service: appointment time — only fields present in the payload |
| The Verticals page lists, per vertical: capabilities (from the catalogue), the business's listing count, and what the platform does not support yet for it | |
| Calendars are one component with vertical-specific resources: hotel = room type × night (exists); other resources arrive with their capacity APIs | |

### 5.2 Role-aware capability matrix (mirrors the server)

| Capability | owner | admin | manager | staff / accountant / sales / operations |
|---|---|---|---|---|
| See the portal, listings, business, own stores | ● | ● | ● | ● (read) |
| Orders, reservations, inventory, payments data | ● | ● | ● | ✕ (the server refuses) |
| Publish / pause / edit listings, promotions, hotel rooms | ● | ● | ● | ✕ |
| Add / remove team members | ● | ● | ✕ | ✕ |
| Create the business, seller profile | operator-assisted | | | |

The three read-only roles `accountant`, `sales`, `operations` exist as values but have **no differentiated
server behaviour**; the UI must not imply finance-only or sales-only access until the server enforces it (gap G-D3).

### 5.3 Screen archetypes

* **Queue + drawer** (orders, reservations, listings): a table with the primary identity column, status, key
  amount/date, one row action; clicking a row opens a right-hand **detail drawer** with the full record, a
  timeline and the allowed actions; the URL reflects the open record so it is shareable.
* **Calendar** (reservations, hotel nights): month/week grid, colour only as status; side panel for the day.
* **Inspector** (listing, store): key/value sections with inline edit where the API allows, otherwise read-only.
* **Wizard** (create listing, create store): steps with a persistent summary, dirty-state protection.

## 6. Admin portal — "run Trezutil"

**Shell (new).** A deep-navy rail in **both** themes (a visual signature distinct from the seller), grouped by
domain; a thin header with a command bar, the signed-in staff member and role, and a build/environment label; a
persistent banner whenever a screen is showing a *limited* source (for example "gateway reachability only").
Density is compact; identifiers use a monospace stack and one-click copy.

```
OVERVIEW
  Dashboard                ●  platform counts · ○ service reachability · ◌ payments, usage, providers, AI, channels
PEOPLE & BUSINESSES
  Users                    ●  search/list · ◌ detail · ◌ suspend/roles · ◌ sessions
  Organizations & sellers  ○  onboarding wizard (4 existing routes) · ○ org detail by id · ◌ list/search · ◌ verification
CATALOG
  Verticals                ○  catalogue, packs, listings per vertical (read-only by design) · ◇ pack switches
  Categories & attributes  ○  list/create categories · ◌ attributes
  Listings                 ◐  publish/pause/resume by id ○ · ◌ moderation queue, suspend
COMMERCE
  Orders                   ○  per organization + detail by id · ◌ platform search
  Reservations             ○  per organization · ◌ detail (staff is refused today) · ◌ platform search
  Payments & finance       ○  one payment, refunds, capture/refund · ◌ list, reconciliation
PLATFORM
  Providers                ◌  (handlers exist, unmounted)
  AI control centre        ◌  (env-configured; keys never shown)
  Channels                 ◌  Web / Mobile / WhatsApp status
  Monetization             ◌  plans, subscriptions, credits, ledger
  Search & indexing        ◌
OPERATIONS
  Incidents & support      ◌
  Audit                    ◌  log trail only — shown as a stated limitation, never as fake rows
  Security                 ◌
  System health            ◐  reachability ○ · database, OpenSearch, queues, metrics ◌
SETTINGS                   ◌  flags, platform settings
```

**Console conventions**

* Read first: every screen opens as an inspector; actions are explicit buttons, grouped, never inline toggles.
* Every state-changing action goes through a `ConfirmationDialog` that names the target and the effect, and
  displays the idempotency/request id after success so the action can be found in the audit log trail.
* Unavailable modules are **listed** (the console shows the full intended scope) with a lock-style state that
  says what is missing and links to the contract in the gap analysis — never a spinner, never sample data.
* A **command bar** (`/` or ⌘K) jumps between modules and resolves a pasted id to its screen. It searches
  navigation and ids the user typed; it does not pretend to be a global search.

## 7. Shared design language

### 7.1 Tokens — three sets exist today (a decision is needed)

| Token | `docs/design` (brand docs, the brief) | Live web (`figma.css`, Figma export) | Mobile Figma |
|---|---|---|---|
| Primary blue | `#155EEF` | `#176BFF` | `#1A69F0` |
| Deep navy | `#0B1220` | `#071A2F` (`#0B2440`) | `#0B1929` |
| Ice / surface | `#F5F8FC` | `#F4F8FB` | `#F9F8F5` |
| Mint | `#35D0BA` | `#9BE5C2` | `#1A7A4A` |
| Coral | `#FF7A66` | `#FF8064` | `#E8634A` |
| Dark background | `#080D17` / `#101827` / `#172238` | `#071725` / `#0D2134` | `#080F1A` |
| Headings / body | Manrope / Inter | Manrope / DM Sans | Instrument Sans / Inter |
| Radius | 4 / 8 / 12 / 16 px | 0.75 / 1.15 / 1.75 / 2.5 rem | 12–16 px |

The brief asks for the brand-document tokens; the shipped customer web follows the Figma export (byte-for-byte
`figma.css`, kept unmodified). **Recommendation (decision DEC-1):** introduce the brand tokens as a `--tz-*` token
layer used by the **seller and admin shells only** (their own stylesheet, never touching `figma.css`), with the
customer portal unchanged until a deliberate migration. A short adapter file records the mapping so a later
unification is one file. This honours "use the existing design system" for the consoles without breaking
Figma fidelity for customers.

### 7.2 Typography and scale

* **Manrope** for headings and navigation, **Inter** for tables, forms and numbers
  (`font-variant-numeric: tabular-nums` on every money/count/date column). Both are in the brand document; Inter
  is not loaded by the web today (it loads DM Sans) — Phase 1 adds it for the consoles only (decision DEC-1).
* Console scale (px): 12 caption · 13 table · 14 body · 16 section · 20 page title · 24 dashboard figure. The
  brand scale (display 48, H1 32) is used by the customer portal only.
* Density: base unit 4 px; table row 44 px comfortable / 36 px compact; control height 36 px (40 on touch).

### 7.3 Colour semantics (from `BRAND_SYSTEM.md`)

| Use | Colour |
|---|---|
| Primary action, links, focus | Trezutil blue |
| Confirmed, available, success, active | Mint |
| Needs attention, destructive confirmation, error emphasis — **sparingly** | Coral |
| Neutral status (pending, draft, paused) | Text + grey dot |
| Chrome | Ice (light) / dark surfaces 1–3 (dark) |

Status is never colour alone: always the word, plus a shape/dot. Contrast ≥ 4.5:1 for text, 3:1 for UI parts,
checked in both themes (a token contrast test is part of Phase 1).

### 7.4 Rules that keep it from looking like a template

| Do | Do not |
|---|---|
| Tables, split views, drawers, timelines, calendars | Walls of equal rounded cards |
| Radius 8 px on controls and tables, 12 px on drawers/dialogs | Large (≥ 24 px) radii on data surfaces |
| Hairline borders (1 px, `--tz-line`) and whitespace for grouping | Drop-shadow stacks, glass, blur |
| One metric strip per page at most, each metric with its source | Giant KPI cards everywhere; sparkline decoration |
| Solid, restrained colour | Gradients, glow, blobs, purple/blue "AI" washes, decorative graphics |
| Subtle motion: 150–220 ms ease on drawers, rows, focus; none on data refresh | Count-up animations, shimmer on loaded data, bouncing |
| Real empty states that say what to do next | Illustrations as filler; fake assistants |

Motion follows `MOTION_GUIDELINES.md` and `prefers-reduced-motion` (the global reset already exists).

### 7.5 Shared component catalogue

| Component | Purpose / behaviour | Exists today | Phase |
|---|---|---|---|
| `PortalShell` (+ `Sidebar`, `MobileNav`, `Header`, `PortalSwitcher`, `Breadcrumbs`) | Seller/admin frame; keyboard-operable rail; skip link; focus to `main` on navigation | Partial (`SellerLayout`, `AdminLayout`) | 1 |
| `PortalGuard` / `usePortals` | Resolution + guards (section 3) | New | 1 |
| `DataTable` | Real `<table>`, sortable headers (`aria-sort`), column hierarchy, row action, row-to-drawer, sticky header, density toggle; below 760 px becomes stacked rows | New (existing "tables" are `div role=table`) | 1 |
| `FilterBar`, `Pagination` | Server `limit/offset` where the API supports it; otherwise client paging with a visible "showing the latest N" notice | New | 1 |
| `DetailDrawer` / `DetailPanel` | Focus trap, `Esc`, URL-addressable, actions footer | New (the customer drawer exists for the menu) | 1 |
| `ConfirmationDialog` | Names target and effect; busy state; error inline | New | 1 |
| `StatusBadge` | Word + dot; one mapping table for order, reservation, listing, payment, provider statuses | Exists (19 statuses) | 1 (extend) |
| `EmptyState`, `ErrorState`, `LoadingState`, **`UnavailableState`** | `UnavailableState` = "Backend dependency": what is missing, which endpoint (gap id), who decides | Exists except Unavailable | 1 |
| `Metric` | Value + label + **source** + "as of"; shows "based on the latest N" when truncated | Exists (without provenance) | 1 (extend) |
| `Tabs`, `Field`, `Form` / `FormSection` | Validation, server-error mapping, success/error states, **unsaved-change guard** (router blocker) | Tabs, Field exist | 1–3 |
| `Toast`, `Notification` | Existing `useToast`; no notification centre without a backend | Toast exists | — |
| `Timeline` / `ActivityFeed` | Real status history only | New | 3 |
| `Calendar` | Month/week, resource rows (room type × night) | New | 3 |
| `Chart` | **Not built until a real time series exists** | — | after gap closure |
| `CommandBar` | Navigation + pasted-id resolution only | New | 4 |

## 8. Honesty rules for data (apply to every screen)

1. **Source on every number.** A metric names its endpoint or derivation in an accessible tooltip.
2. **Truncation is disclosed.** The dashboard sums the first 100 incoming orders today; a derived figure shows
   "Based on the latest 100 orders" and, once a server aggregate exists, drops the notice. If more rows exist
   than were loaded, the figure is labelled a lower bound — never a total.
3. **Unavailable ≠ zero.** A backend gap renders `UnavailableState`, not `0` and not `—` with no explanation.
4. **No simulated activity.** No fake rows, seeded demo data, optimistic counters or sample charts.
5. **Limits stated.** For the audit module: "Audit events are written to the service logs; there is no queryable
   audit store yet."
6. **Money** is shown with its currency, from minor units (`total_minor`), never summed across currencies.
7. **Time** is shown in the viewer's locale with the date format used elsewhere in the app.

## 9. Responsive strategy

| Portal | ≥ 1200 | 900–1199 | 640–899 | < 640 |
|---|---|---|---|---|
| Customer | Figma desktop | Figma | Figma tablet | Figma mobile + bottom tabs, filter bottom sheet |
| Seller | Rail + page + optional drawer | Rail collapses to icons; drawer overlays | Rail → slide-over; tables keep key columns; drawer full width | Bottom tab bar (Dashboard, Orders, Reservations, Listings, More); tables → stacked rows; filters in a sheet; actions in a sticky footer |
| Admin | Rail + page + drawer | Icon rail | Slide-over rail | Usable, not optimised: single column inspector, horizontal scroll only for wide tables with a sticky first column |

## 10. Frontend security rules

* No secrets in the bundle or in any UI (tokens, API keys, provider credentials, webhook secrets, passwords).
* Tokens live where the existing client keeps them; the console adds no new storage of personal data. The seller's
  selected business id is stored on the device and **re-validated against `/sellers/me` on every load**.
* Personal data (admin user list: phone, email) is shown only to `ADMIN`, is not copied into logs, URLs or
  local storage, and phone/email are copy-on-demand.
* All provider/AI/channel information the console may show is **status and configuration names** returned by a
  staff-only endpoint — never credentials. The browser never calls a model provider.
* State-changing requests send the idempotency keys the API defines; retries reuse the key.
* The UI never trusts a role it computed itself: refused requests (`403`) are shown as Forbidden.

## 11. Existing Figma and design inputs

* Web Figma export (customer + seller) — see `docs/design/FIGMA_IMPLEMENTATION_MAP.md`; **there is no Figma for
  the admin console** and none for the portal shells proposed here. The seller shell's current visual language
  is the export's seller pages; the admin console was built in that language.
* Mobile app Figma — `docs/mobile/` (separate track, out of scope here).

## 12. Test plan (maps to the brief's section 48)

| Area | Tests to add (by phase) |
|---|---|
| Security (Phase 1) | customer → `/seller` and `/admin` refused; seller → `/admin` refused; seller of org A cannot load org B (server 403 shown as Forbidden); direct URL access to a guarded portal; menus filtered by role; portal switcher lists only held portals; sign-out clears portal state |
| Shell (Phase 1) | keyboard navigation of rail/drawer, skip link, focus to `main`, responsive collapse, theme persistence, token contrast in both themes |
| Customer (Phase 2) | authentication, navigation, search (incl. price), listing, conversation, order, reservation, account, direct URL |
| Seller (Phase 3) | dashboard (truncation notice), listings (search/filter, create/edit, promotion), inventory, availability (hotel), orders (confirm, drawer), reservations (calendar), team, usage (unavailable state), permissions (view-only roles) |
| Admin (Phase 4) | dashboard, users, onboarding wizard, org detail by id, verticals, categories, orders/reservations per org, payment by id, health reachability, every unavailable module renders the contract |
| E2E | Playwright portal journeys on the real stack (extend `test/e2e/web`) |

Each Go-side change (only after the gap decisions) keeps its own tests; Phase 1–4 add **no** backend code.

## 13. Implementation order

| Phase | Content | Depends on |
|---|---|---|
| **0** | This audit and the three documents | — |
| **1** | Shared portal shell (section 14) | Decisions DEC-1 to DEC-3 |
| **2** | Customer portal: signed-in home strip, filters, account split, tests | 1 |
| **3** | Seller portal, in the brief's priority order. Real work first: Dashboard (paging) → Business/Stores/Verticals → Listings (search, drawer) → Inventory/Availability → Orders → Reservations (calendar) → Finance (revenue) → Team → Promotions. Honest unavailable states for Customers, Inbox, Analytics, Usage, Marketing (other than promotions), Integrations | 1 |
| **4** | Admin portal. Real work first: Dashboard + reachability → Users → Organizations (onboarding wizard, detail by id) → Verticals → Categories → Orders/Reservations per organization → Payment by id → Health. Unavailable states with contracts for Providers, AI, Channels, Monetization, Search, Operations, Audit, Security, Settings | 1 |
| **5** | Gap closure: finalise the backend gap report (the gap analysis is its draft), decide which gaps to build; **no backend work starts without an explicit instruction** | 3, 4 |
| **6** | Visual and accessibility polish across all three | 2–5 |

## 14. Exact Phase 1 scope — shared portal shell

**Goal.** One verified foundation the three portals plug into. No new business screens; no backend change.

**In scope**

1. **Token layer** `--tz-*` for the consoles (colour, type, space, radius, line, elevation, motion) with light and
   dark values from the brand documents, a contrast test for every text/background pair, and the adapter note
   mapping to `figma.css`. `figma.css` is not modified.
2. **Portal resolution and guards:** `usePortals()`, `PortalGuard`, the 403 and "no business yet" pages, redirects
   with `returnTo`, role-filtered navigation definitions (a typed, declarative config per portal: label, route,
   permission predicate, **status** `implemented | unavailable(gap id)`).
3. **`PortalShell`** with `Sidebar`, `Header`, `Breadcrumbs`, `MobileNav`, `PortalSwitcher`, business picker
   (kept from the seller shell), skip link, focus management, dark/light, responsive behaviour from section 9.
4. **Seller and admin shells migrated** onto it with **no change of behaviour**: the seller area stops being
   wrapped in the customer navbar; the admin console keeps its login and guards.
5. **Shared components:** `DataTable`, `FilterBar`, `Pagination`, `DetailDrawer`, `ConfirmationDialog`,
   `UnavailableState`, extended `StatusBadge` and `Metric` (provenance), `Breadcrumbs`; `LoadingState`/`EmptyState`/
   `ErrorState` aligned to the tokens.
6. **Navigation skeletons** for every seller and admin module in the maps — each real module routes to its
   existing page, each gap module routes to an `UnavailableState` that names the gap id and links the contract.
7. **Tests:** every item of the "Security" and "Shell" rows in section 12.
8. **Docs:** update `docs/design/FRONTEND_PRODUCTION_READINESS.md` and the implementation map statuses.

**Out of scope:** new modules or data screens, charts, backend changes, customer-portal redesign, the command
bar, marketing, anything marked ◌ beyond its unavailable state.

**Acceptance:** `tsc` and the whole Vitest suite green; new security/shell tests green; keyboard-only run through
both shells; screenshots at 1440 / 820 / 390 px in light and dark for both shells; no `figma.css` diff; the
implementation map shows no module without a route.

**Preconditions / decisions needed before starting (DEC-n)**

| # | Decision | Recommendation |
|---|---|---|
| DEC-1 | Token set for the consoles (and whether to load Inter) | Brand tokens as `--tz-*` for seller/admin; customer keeps Figma tokens (section 7.1) |
| DEC-2 | Where `PortalShell` lives and whether seller/admin share one shell with two themes of navigation | One shell component, two navigation configs and two visual signatures (light rail for seller, navy rail for admin) |
| DEC-3 | Language for the consoles | Keep French default + English, `vous` (existing, test-enforced) |
| DEC-4 | Environment label in the admin header | A build-time variable (`VITE_APP_ENV`); no runtime source exists |
| DEC-5 | Whether gap closure (Phase 5) may begin after the report | Separate explicit instruction |

## 15. Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | Most admin modules and several seller modules are **backend gaps**; the portals will look sparse next to the brief | Build real value on what exists; show the full intended scope with honest unavailable states; make the gap list the roadmap |
| R2 | Misleading numbers from truncated lists (first 100 orders) | Disclosure rule 2; page through the API or label as lower bound |
| R3 | Token conflict between brand docs and the shipped Figma web | Decision DEC-1; a separate `--tz-*` layer limits blast radius |
| R4 | A staff-gated read API does not exist for health, so the public `/readyz` (which leaks internal URLs) would be the only source | Prefer the staff-gated endpoint (gap G-D5); until then show reachability only |
| R5 | Anonymous `GET /sellers?seller_id=` exposes seller phone and user id | Security finding S-1: fix on the backend; the console must not rely on it |
| R6 | i18n is test-enforced: every new string needs a French entry | Budget it per phase |
| R7 | Seller shell currently lives inside the customer shell | Phase 1 migration; covered by tests |
| R8 | Platform staff cannot read reservation details | Security finding S-5; Admin Reservations stays limited |
| R9 | Local WhatsApp token expired (2026-09-30) | Sign-in of customers/sellers on staging needs it; unrelated to portal code |
