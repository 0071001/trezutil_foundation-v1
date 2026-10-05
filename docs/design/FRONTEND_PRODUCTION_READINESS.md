# Frontend — production readiness

Date: 2026-09-29 · App: `web/frontend` · Visual source: `Premium Homepage Design.zip` (mapping: [FIGMA_IMPLEMENTATION_MAP.md](FIGMA_IMPLEMENTATION_MAP.md))

## Status: CONDITIONAL PRODUCTION READY

Every customer and seller flow runs against the real backend and is verified in a browser at phone and desktop sizes. What remains is external configuration (live Stripe and WhatsApp credentials) and product scope documented in KNOWN_LIMITATIONS.md. Nothing is mocked in production code.

## Implemented screens

**Customer:**
- Home, Discover, Search, and Listing detail (product/service/food/real-estate variants).
- Ask Trezutil (conversational commerce).
- Bag, Saved, Checkout (review → payment → confirmation), Booking (dates → availability → reserve → confirmation).
- Orders, Order detail (timeline, payment, cancel), Reservations, Reservation detail (cancel), Account, Activity.
- Sign in and Register (phone → 6-digit code).

**Seller:**
- Workspace overview (live metrics, attention list, activity) and onboarding (operator-assisted).
- Listings (Live / Paused / Drafts / Unavailable, publish/pause/resume/edit), listing detail with stock, create listing (6 steps, draft or publish), edit listing.
- Orders (confirm paid orders), Reservations, Inventory, Business.

**Shared states:** 404, 403, 401 / session expired (redirect with return path), 500 (error boundary), network/timeout errors with retry, offline banner, loading skeletons, empty states, toasts.

## Components

Figma primitives:
- Icon (24 glyphs + check/close), Button, ActionLink (routed), Heading (real h1–h3), Brand, PageIntro, Tabs (ARIA tabs), Field, StatusBadge (Figma tones).

Built from the design:
- Commerce: ListingImage (lazy, safe URL, branded fallback), ResultCard, ProductCard, SaveButton, SearchComposer (combobox with live suggestions, voice input), PaymentPanel (Stripe Payment Element + webhook polling).
- Shell and navigation: AppShell/Navbar, mobile drawer, mobile bottom nav, ConversationDock. The seller and administration consoles share one `PortalShell` (see "Portals Phase 1" below).
- States: EmptyState, ErrorState, SkeletonGrid, Loading, OfflineBanner, Toasts, ErrorBoundary.

## Portals Phase 1 — shared console shell (2026-10-04)

The seller and administration consoles now share one frame, built to the plan in [PORTALS_INFORMATION_ARCHITECTURE.md](../web/PORTALS_INFORMATION_ARCHITECTURE.md). No backend change was made, and `figma.css` is untouched: the customer web is visually unchanged.

- **Tokens.** `src/styles/portal.css` holds the brand layer (`--tz-*`, from DESIGN_TOKENS.md) scoped to `.app.portal`, plus an adapter that maps the Figma variable names onto it, so existing pages follow the brand without being rewritten. A test (`src/portal/tokens.test.ts`) checks every text pair for 4.5:1 and every focus ring for 3:1 in the light theme, the dark theme and the navy administration rail.
- **One `PortalShell`, two signatures.** Light rail for the seller, always-navy rail for the administration. Header with breadcrumbs, portal switcher, language, theme, signed-in person and sign-out. Below 900 px the rail is a modal slide-over (focus trapped, Escape closes and returns focus); on phones the seller also has a four-module tab bar plus "More", and the language switch moves into the menu. The seller console is no longer wrapped in the customer navbar. The administration shows the build environment (`VITE_APP_ENV`, empty or `production` shows nothing).
- **Who can open what.** `usePortals()` derives the portals a person holds from `GET /auth/users/me` and `GET /sellers/me` (loaded once, only when a console asks, dropped on sign-out or when another person signs in). Menus are filtered by role and the switcher lists only held portals; all of it is advisory, the server decides on every request, and a 403 is shown in place.
- **Every module has a route.** `src/portal/modules.ts` declares 20 seller and 20 administration modules once: navigation, routes and status all follow it. Seller: 8 built, 3 planned (data exists, screen not built), 9 unavailable. Administration: 3 built, 8 planned, 9 unavailable (Analytics and Site content were built after Phase 1; see below). A module that is not built renders `PlannedState` (the endpoints it will use) or `UnavailableState` (the platform gap it waits for, `src/portal/gaps.ts`), never sample data or a dead control.
- **Shared components** (`src/components/portal/`): `DataTable` (sortable with `aria-sort`, client or server paging, keyboard-operable rows, density, stated limits), `FilterBar`, `Pagination`, `DetailDrawer`, `ConfirmationDialog` (cannot be double-submitted, shows the server's refusal in place), `Breadcrumbs`, `Metric` (names its source and flags a figure computed from a full page of rows as a lower bound), `UnavailableState`, `PlannedState`.
- **Seller dashboard figures** are now counted by the platform for the whole window (see "Dashboards and site content"); the earlier "first 100 rows" caveat no longer applies to them.
- **Tests.** After this phase `vitest run` was 361 tests in 16 files (134 in 11 before it): token contrast, module registry integrity, the portal components, the Security rows (guest, customer, seller and staff at every console address; cross-business scope; a remembered business that is not the person's; 403 shown in place; switcher; sign-out clears portal state) and the Shell rows (landmarks, rail grouping and state, breadcrumbs, theme, narrow-screen behaviour, every module routed).
- **Checked by hand.** Keyboard-only run of both shells (tab order, switcher with arrows and Escape, rail dialog trap and focus return, route-change focus on `main`); screenshots at 1440, 820 and 390 px in light and dark for both shells, with no horizontal overflow and no console errors.
- **Not done in this phase (by design).** New data screens, charts, the command bar, customer redesign, and every backend gap (the anonymous seller phone exposure S-1 is reported, not fixed). Phase 2 has not been started.

## Dashboards and site content (2026-10-04)

Full contracts, definitions and tests: [../web/DASHBOARDS_AND_SITE_CONTENT.md](../web/DASHBOARDS_AND_SITE_CONTENT.md). This adds a small, tested backend to the earlier frontend-only phases: migration `000024_site_content.sql`, `GET /admin/analytics`, `GET /sellers/analytics`, `GET /site/content` (public), `GET`/`PUT /admin/site/content`, `POST /admin/site/assets` and `GET /site/assets/{id}`.

- **Dashboards.** The administration Overview and the seller Dashboard and Analytics module show KPI cards with the change against the previous period, per-day charts (revenue with a currency switch, orders, bookings, sign-ups, conversations, new listings), status breakdowns, best sellers and top businesses. Every number is counted by the server from its own tables; a quiet period shows real zeros and plain sentences, never invented points. Charts are SVG without a library, readable by keyboard, with a "View as a table" switch on every card.
- **Home page, managed.** The **live-figures strip** (*Trezutil en chiffres*) replaces the earlier marketing-style strip: "197 annonces en ligne", "38 vendeurs vérifiés"… each figure is computed by the platform, hidden until it reaches the minimum staff set, and absent if it cannot be loaded. The **partner band** (at the foot of the page) and the **visibility of each section** come from the same content (the order of the sections is the product story and lives in the code). Administrators edit all three in **Administration → Site content** (draft, preview with today's real figures, publish with a version check so two editors cannot overwrite each other, logo upload).
- **Partners are unconfirmed.** The default list was assembled from the categories requested and well-known Kinshasa organisations; the repository holds no record of these partnerships, and every logo is a third-party trademark. Confirm each partner and its brand rules before launch (sources and terms: [../web/PARTNER_LOGOS.md](../web/PARTNER_LOGOS.md)).
- **Tests.** `vitest run` is 489 tests in 21 files; `go test ./...` passes, including integration tests on real PostgreSQL that build a known platform and check every figure and every authorization rule.
- **Also changed.** The administration sign-in form checks the shape of the email address before sending it (the server answers every refusal the same way by design, so a typo was indistinguishable from a wrong password); the web build takes `VITE_APP_ENV` (administration header label).

## Home page redesign (2026-10-04)

Trezutil is not a listing site: a person arrives with a need, says it, and Trezutil turns it into an action. The home page now follows that experience, **frontend only** (no backend, API or business feature was changed). The audit of the previous page, the new information architecture, what was deliberately not shipped and the open points are in [../web/HOME_PAGE_REDESIGN.md](../web/HOME_PAGE_REDESIGN.md).

- **Order.** Welcome ("What do you need today?" and a place to say it, with Dictate and Send, six example sentences) → *What can we help you with?* (problems, not categories) → *Discover what exists around you* (three short lists, hidden when there is nothing) → *From your need to the action* (you → Trezutil → the offers → you → the business; buy, book and pay are live, contact the seller and request a quote are marked *Soon*) → live figures → *Have something to sell or offer?* → partners.
- **No listing grid.** Offers appear only as rows inside one block; with every offer removed the page is still understandable (`src/pages/home.test.tsx` checks it). No AI wording, robot or sparkle decoration in the page body, no gradients or glowing shapes.
- **Not shipped, because the platform cannot do it yet:** Photo and Location actions in the composer (the conversation takes text only; no geolocation parameter), and *Get around* shows only when cars or transport have live offers.
- **Site content.** Staff now choose which of six blocks show; the order is fixed in code. Older stored section keys are kept by the server and ignored by the page.
- **Chat.** The floating "Ask Trezutil" chat (and the WhatsApp button inside it) was **removed on 2026-10-05 at the owner's request**: `App.tsx` no longer mounts `ChatWidgetProvider` or `ChatWidget`. The component and its tests stay in `src/components/` (unmounted, not deleted), so mounting it again is a two-line change. The conversation lives on `/conversation` (reached by *Make a request*, the *Ask* tab on phones and the *Ask Trezutil anything about this page* bars, which now simply link there); WhatsApp is offered in the footer and in the home page's WhatsApp block ([OUR_COMPANY.md](../web/OUR_COMPANY.md)).
- **Styles.** `src/styles/home.css`, scoped under `.app.home`; `figma.css` untouched. The dead showcase, banner, stage and hero rules and `components/promo.tsx` were removed.
- **Welcome picture (2026-10-05).** The welcome sits on a photograph of a shopping centre supplied by the owner (Unsplash Licence; 57 to 291 KB per size, a portrait crop for phones; worst-case text contrast 4.8:1). It is Spanish and shows third-party brands: see [../web/HOME_IMAGES.md](../web/HOME_IMAGES.md), which also records the earlier Kinshasa photograph and aerial film (the film's author and licence were never recorded; its original is still in `bg/`). Five original vector drawings illustrate the needs.
- **Primary menu (2026-10-05).** The menu is now the needs (*Find a place, Get something, Find a service, Go out*) then *Discover*, read from one shared list on the home page, the bar of every page and the drawer; *Reservations* moved with the person's own pages. The mobile drawer of inner pages, which was squeezed into the height of the header, was fixed. [../web/PRIMARY_MENU.md](../web/PRIMARY_MENU.md).
- **Open point.** The logo mark is still the Figma export's sparkle glyph (on every page); changing a brand mark is the owner's decision.

## Backend integration (all through the gateway, `/api/v1`)

| Area | Endpoints |
|---|---|
| Auth | `POST /auth/request-otp`, `POST /auth/verify-otp`, `POST /auth/refresh` (single-flight, automatic on 401), `GET /auth/users/me` |
| Discovery | `POST /search` (text, category, sort, cursor paging), `GET /search/suggest`, `GET /categories`, `GET /listings` (+ `Media`), `GET /stores` |
| Conversation | `POST /conversations/message` (idempotency key, retry with the same key), `POST /conversations/{id}/confirm`, `/reject` |
| Booking | `GET /availability`, `POST /reservations` (Idempotency-Key), `GET /reservations`, `GET /reservations/{id}`, `POST /reservations/{id}/cancel` |
| Orders & payments | `POST /orders` (Idempotency-Key), `GET /orders`, `GET /orders/{id}`, `POST /orders/{id}/cancel`, `POST /payments` (idempotency key per order; Stripe client action) |
| Seller | `GET /sellers/me`, `GET /listings/manage`, `POST /listings` (media, availability), `POST /listings/{publish,pause,resume,update}`, `GET /orders/incoming`, `POST /orders/{id}/confirm`, `GET /reservations/incoming`, `GET /inventory` |

Missing backend contracts, whose UI boundaries are in place: conversation history, notifications, image upload, inventory adjustment, fulfilment transitions on orders, geo/type search filters, reviews, profile edit, server-side cart/wishlist (see KNOWN_LIMITATIONS L2–L9). One backend addition was made because the Figma detail gallery needs it: listing media in `GET /listings`.

## Security

- Identity always comes from the bearer token, never from bodies or query parameters.
- Tokens live in `localStorage` (the existing architecture), refresh is single-flight, and an expired session is cleared and announced.
- No raw HTML rendering anywhere; seller-supplied media is limited to http(s) URLs.
- Post-login redirects accept same-site paths only.
- External links use `noopener noreferrer`.
- No secrets or hosts are in the bundle (checked by grep of `src/` and `dist/`).
- Card data only ever enters Stripe's iframe.
- Every authorization decision is made server-side; the UI hides actions for convenience only (the seller view-only role is tested both ways).

## Accessibility

Semantic landmarks and headings, a skip link, focus moved to the page on navigation, visible `:focus-visible` rings, ARIA tabs/combobox/dialog/alert patterns, labelled controls (hints outside labels), live regions for chat, toasts and status. Reduced motion is honoured. Deliberate deviations from Figma: minimum text sizes and 40 px tap targets.

## Performance

- Route-level code splitting: the homepage chunk is about 100 KB gzipped, and each other route is 1–5 KB.
- Lazy images, identical concurrent GETs de-duplicated, stale responses ignored, cursor paging.
- 30 s request timeout, and a "still searching" hint after 3 s.

## Tests

| Suite | Result |
|---|---|
| TypeScript (`tsc --noEmit`) | PASS |
| Unit/component (`vitest run`) | **27/27**: client (refresh/replay, expiry, envelope, network, dedupe, idempotency), mappers, redirect/URL safety, sign-in/OTP/rate limit, search/paging/error-retry, conversation (context → confirm with token, retry same key), checkout/payment boundary, seller (publish, view-only, onboarding, six-step create with URL validation), 404/403, bag |
| Production build (`vite build`) | PASS, no warnings |
| Browser journeys (Playwright, real stack, desktop + mobile) | **16/16** (final clean run; earlier runs surfaced and fixed the order-DTO contract and sign-out defects): A discovery, B sign-in + sign-out + session expiry, C conversation → order, D booking, E checkout → payment step, F account, G seller workspace, H failure → retry and 404 |
| Visual QA | 12 screens × 3 viewports captured next to the Figma export; 0 px horizontal overflow and 0 console errors on all 36 |

## Deployment

The build is served by the gateway (`WEB_ROOT`) with SPA fallback for deep links, and the API is same-origin (`/api/v1`, overridable with `VITE_API_BASE_URL`). The optional public build variable `VITE_WHATSAPP_NUMBER` (the WhatsApp Business number, digits only, with the country code) enables WhatsApp calls-to-action, including the floating WhatsApp button that opens a real chat with that number; with no number the button is not shown and no placeholder link exists. Every page except the full conversation and the sign-in pages also carries a floating chat with the assistant (`ChatWidget`): it uses the same API and the same per-tab thread as `/conversation`, so a question can move between the two, and the "Ask Trezutil" bar of a page opens it with that page's question. In Docker the number is passed as a build argument (`VITE_WHATSAPP_NUMBER` in the compose environment file); changing it needs an image rebuild. The same applies to `VITE_APP_ENV`, the label shown in the administration header (`staging`, `demo`…; empty or `production` shows none). The Dockerfile builds and type-checks the app, and CI runs typecheck, tests and build.
