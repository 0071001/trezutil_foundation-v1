# Trezutil — MVP final certification

Date: 2026-09-29 · Scope: finalization phase (payments, seller experience, AI, WhatsApp readiness, deployment) and the Figma frontend implementation. Evidence: PLATFORM_AUDIT.md §7–8, PRODUCTION_READINESS.md, docs/design/FRONTEND_PRODUCTION_READINESS.md. Operator guide: PRODUCTION_DEPLOYMENT.md.

## PLATFORM
Go services (identity, search/listings, conversation, orders/reservations, payments) behind one API gateway; PostgreSQL is authoritative and OpenSearch is a rebuildable projection. Migrations 000001–000018 apply from an empty database. `gofmt` clean, `go vet` clean, build OK. **PASS**

## CUSTOMER EXPERIENCE
The web app implements the Figma design: home, discovery, search with suggestions and paging, listing detail, conversational commerce, bag/saved, checkout with payment, booking with live availability, orders and reservations with detail and cancellation, account, activity, and phone sign-in with session renewal and expiry. It is responsive (phone/tablet/desktop), with light and dark themes and accessible patterns. Verified in a browser on the real stack. **PASS**

## SELLER EXPERIENCE
Seller workspace:
- Overview with metrics computed from real orders, bookings and listings.
- Listings: create in six steps (draft or publish), edit draft/paused, publish/pause/resume, detail with stock.
- Orders (confirm paid ones), Reservations, Inventory (read-only), Business.
- Operator-assisted onboarding page.

All authorization is server-side (membership and role). A staff member can view but not change; another organization gets 403. **PASS** (inventory adjustment and fulfilment transitions are documented boundaries, KNOWN_LIMITATIONS L2/L3)

## CONVERSATIONAL EXPERIENCE
Natural-language requests become allow-listed intents through the AI gateway (Anthropic adapter, strict JSON output, confidence threshold, metering, timeouts, keyword fallback). Results render as structured cards. Contextual references work ("1", "the second one", "book it", "buy it", "pause this listing"). Missing parameters are asked for, and orders need an explicit confirmation with a single-use, owner-bound token. The model never executes actions. **PASS** (live model key = external, optional)

## PAYMENTS
Stripe integration code is complete:
- PaymentIntents with automatic or manual capture, and Stripe.js in the web app.
- Capture, cancel and refunds (sync and async).
- Idempotency keys, signed webhooks with replay protection, timeout and resume, declines.

Payment Core guarantees the amount and currency match the order, one live payment per order, per-payment locking, and exactly-once ledger postings. Verified against a Stripe emulator in unit, real-PostgreSQL and end-to-end tests. **PROVIDER INTEGRATION CODE COMPLETE — PROVIDER CREDENTIALS/CONFIGURATION UNAVAILABLE**

## WHATSAPP
Webhook handshake, HMAC verification, sender auto-provisioning, the same conversation processor as the web, replies and OTP template delivery. Verified with signed test webhooks and an HTTP test server. **CODE READY — EXTERNAL CONFIGURATION REQUIRED** (Meta credentials and an approved template; sign-in in production depends on it)

## SECURITY
- Verified-principal identity everywhere: client identity fields are ignored.
- Object-level authorization for orders, reservations, payments, listings, organizations, inventory, the seller console and conversations.
- One authorization defect fixed in this phase: a buyer could confirm their own order (P1). Covered by an integration test and an E2E check.
- OTP hashing with attempt and rate limits; webhook signatures; body limits.
- Frontend: no raw HTML, safe redirects and media URLs, no secrets in the bundle.

**PASS** (residual items: token revocation window, per-instance OTP IP limiter, `localStorage` tokens — KNOWN_LIMITATIONS)

## DATA INTEGRITY
UUIDs from constructors; optimistic locking on orders, reservations, payments, refunds, inventory and capacity; a DB exclusion constraint prevents double booking (8-way concurrency test: exactly one wins); unique idempotency keys; the projection resyncs hourly. **PASS**

## FINANCIAL INTEGRITY
Money is kept in minor units. Payment amount/currency must equal the order; refunds are capped (over-refund rejected); replayed refunds are de-duplicated; ledger postings happen exactly once; credit reservations are balance-guarded; no PAN, CVV or PIN is stored, and card entry happens in Stripe's iframe. **PASS**

## DEPLOYMENT
- One Dockerfile (all binaries, migrations, SPA) and a compose topology with the gateway as the only public port.
- Env templates include the Stripe/AI settings; required-secret guards are in place.
- CI runs format, vet, Go tests with PostgreSQL/OpenSearch, and the frontend typecheck/tests/build.
- `docker compose config` is valid.
- The image was not built on this workstation (Docker is not installed); every binary and the SPA were built and run natively for all tests.

**PASS (static)** — build the image once in CI or on the target host.

## TESTS
| Suite | Result |
|---|---|
| Go unit + real PostgreSQL integration (`go test -count=1 -p 1 ./...`) | 23/23 packages, **1,226** passing test cases (106 PostgreSQL integration tests), 0 failures |
| Frontend (`vitest run` / `tsc` / `vite build`) | **27/27** / PASS / PASS |
| API end to end (`test/e2e/journeys.py`: real services, PostgreSQL, OpenSearch, Stripe/Anthropic emulators) | **101/101**, twice on the same database |
| Browser end to end (`test/e2e/web`, Playwright, desktop + mobile) | **16/16** (A–H), final clean run |
| Visual QA against the Figma export | 12 screens × 3 viewports; 0 overflow, 0 console errors |
| `gofmt` / `go vet` / `go build` / `docker compose config` | clean / clean / OK / valid |

## REMAINING
External configuration only:
1. Stripe account per selling organization (keys, provider row, webhook endpoint).
2. WhatsApp Cloud API credentials with an approved authentication template (required for production sign-in).
3. Optional: an AI model key.

Product boundaries L1–L12 are listed in KNOWN_LIMITATIONS.md. None of them is a defect in a shipped flow.

## FINAL STATUS

**MVP READY** — Trezutil is technically complete, secure, testable, deployable and ready for real provider configuration and pilot deployment. Going live needs the external configuration above; no code is missing for the certified scope.
