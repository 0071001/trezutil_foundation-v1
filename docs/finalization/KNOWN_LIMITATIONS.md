# Trezutil — Known Limitations (MVP)

Updated 2026-09-29 after the finalization phase and the Figma frontend implementation. Only external blockers and deliberate MVP boundaries are listed; every code defect found was fixed (see PLATFORM_AUDIT.md §8–9).

## External configuration (launch blockers — no code missing)

| # | Item | Status | What is needed |
|---|---|---|---|
| X1 | **Stripe card payments** | Integration code complete (PaymentIntents, Stripe.js Payment Element, capture, cancel, refunds, signed webhooks, idempotency keys, timeouts). Verified end to end against a Stripe API emulator and real PostgreSQL. | A merchant Stripe account per selling organization: keys in `STRIPE_SECRET_KEY` / `STRIPE_WEBHOOK_SECRET`, a `providers` row and a webhook endpoint (PRODUCTION_DEPLOYMENT.md §6). Until then, **Pay securely** answers "online payment isn't enabled for this seller yet" and orders stay payable later. |
| X2 | **WhatsApp Cloud API** | Code complete (webhook verification, HMAC, sender auto-provisioning, replies, OTP template delivery). Verified with signed test webhooks and an HTTP test server. | Business account, phone number, access token, app secret, verify token and an approved authentication template (§7). Outside `development`, identity-service will not start without an OTP channel, so **nobody can sign in in production until this is configured**. |
| X3 | **AI model key** (optional) | Anthropic adapter behind the AI gateway (router, capability checks, allowlist, confidence threshold, usage metering, timeouts, retries, error normalization). Verified against a Messages API emulator. | `AI_PROVIDER=anthropic` and `ANTHROPIC_API_KEY`. Without them, conversations use deterministic keyword routing (fully functional). |

## Deliberate MVP boundaries

| # | Limitation | Impact | Next action |
|---|---|---|---|
| L1 | Seller onboarding is **operator-assisted** (only platform operators create organizations). | New sellers contact the team; the workspace opens once they are added. `/seller/onboarding` explains the steps. | Business decision on self-serve onboarding. |
| L2 | **Hotels** manage rooms and nightly capacity themselves (Hotel rooms page and API). Other verticals have no API yet to create or adjust inventory/capacity; listings without inventory rows are single bookable units (DB exclusion constraint against double booking). | Multi-unit stock outside hotels (50 phones, restaurant tables) is provisioned by the team; the Inventory page is read-only. | The same capacity API for tables/seats/vehicles (RESTAURANT pack next), plus a stock field for retail. |
| L3 | Post-confirmation order lifecycle (ready → fulfilled → completed): the use cases exist and are now seller-only, but there are no HTTP endpoints, and orders don't expose their fulfilment ID. | Sellers confirm paid orders; delivery is arranged directly. | Expose `ReadyForFulfillment` / `CompleteOrder` and the fulfilment reference on orders; add actions to the seller Orders page. |
| L4 | **Image upload**: listings take https image links; there is no media upload/storage service. | Sellers paste links to hosted photos. | Object storage + signed upload endpoint. |
| L5 | The **bag (cart) and saved items** are device-local. The API now accepts a multi-item order (`items`, one store, one currency, at most 30 lines), but the web bag still places one order per item until the food flow uses it. | A bag with several items becomes one order per item in the current web UI. | Use the multi-item order in the web bag; server-side cart/wishlist, if wanted. |
| L6 | **Conversation history** has no list/read API; the web thread persists per browser tab, and the backend keeps context per channel sender. | Reopening the site in a new tab starts a new thread (orders and bookings are unaffected). | `GET /conversations` and `/conversations/{id}/messages` for the owner. |
| L7 | **Notifications** service does not exist; `/notifications` shows the account's real order and reservation activity. | No push/email notices. | Notification service (WhatsApp templates are the natural channel). |
| L8 | **Search** filters: text, category, vertical, location (place names), price range and sort. There is no geo radius/distance filter (no geocoding) and no date-availability filter; the hotel page checks availability per room type. | "Nearby" means a place name, not a distance. | Geocoded store locations and a radius filter; availability-aware hotel search. |
| L9 | No reviews/ratings, no profile editing, no languages other than English (the Figma "FR" switch is not shown). | — | Product roadmap. |
| L10 | **Sign-out** clears the device session; refresh tokens are stateless and not revoked server-side. Suspending a user blocks refresh immediately; an access token lives at most `JWT_ACCESS_TTL` (15 min). | A copied refresh token stays usable until it expires (30 days) unless the account is suspended. | Refresh-token rotation with a revocation table. |
| L11 | Chat cannot create listings or take payments. | The assistant links to the web flows (checkout, seller workspace). | Payment actions in the runtime now that a provider exists. |
| L12 | Per-organization vertical-pack activation is not persisted (all packs enabled; mutations operator-only). Every listing now has a vertical (migration 000019); an organization's verticals are derived from its listings. | None for MVP routing. | `organization_verticals` when the first vertical-specific seller tool ships (docs/architecture/VERTICAL_COMMERCE_GAP_ANALYSIS.md §2). |

## Operational notes (non-blocking)

- **Access token lifetime:** see L10.
- **OTP per-IP rate limit** is in-process; with several identity-service replicas the effective limit multiplies (the per-phone limit is global, in PostgreSQL). Add an edge limiter when scaling out.
- **Client IP** is the last `X-Forwarded-For` hop; configure the TLS proxy to overwrite that header.
- **TLS** terminates in front of the gateway; OpenSearch runs without its security plugin on the private network only.
- **Chat action idempotency cache** is per process; durable idempotency is in PostgreSQL (orders, reservations, payments, refunds, messages, credits, webhooks).
- **Search freshness:** listing status changes are indexed within `SEARCH_SYNC_INTERVAL` (5 s); media/location/attribute-only edits appear after the hourly resync.
- **Usage policies** are not seeded, so AI and action metering is off until an operator adds `usage_policies` rows.
- **Stripe publishable key** is returned only to the paying customer or organization staff, as part of `client_action`.
- **Docker image** is validated statically (`docker compose config`). Docker is not installed on the finalization workstation; the image is built by CI or on the target host (every binary and the SPA were built and run natively for all tests).
- **Access-token storage:** the web app keeps its tokens in `localStorage`, following the existing architecture (bearer tokens, no cookies). This is an XSS-sensitive choice: the app renders no raw HTML and accepts only http(s) media URLs, and moving to HttpOnly refresh cookies is the recommended hardening.

## Cleanup candidates (P3)

Placeholder binaries (`admin`, `logistics`, `marketing`, `notification`, `seller`, `trust` services), `catalog-service` (duplicate of the search-service listing routes), `cmd/fileserver`, and PascalCase JSON of domain aggregates (add JSON tags when the API contract is versioned).
