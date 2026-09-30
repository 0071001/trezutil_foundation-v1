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
- Shell and navigation: AppShell/Navbar, mobile drawer, mobile bottom nav, ConversationDock, SellerShell with sidebar and mobile tabs, Metric.
- States: EmptyState, ErrorState, SkeletonGrid, Loading, OfflineBanner, Toasts, ErrorBoundary.

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

The build is served by the gateway (`WEB_ROOT`) with SPA fallback for deep links, and the API is same-origin (`/api/v1`, overridable with `VITE_API_BASE_URL`). The optional public build variable `VITE_WHATSAPP_NUMBER` enables WhatsApp calls-to-action. The Dockerfile builds and type-checks the app, and CI runs typecheck, tests and build.
