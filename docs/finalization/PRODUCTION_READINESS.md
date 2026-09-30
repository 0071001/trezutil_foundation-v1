# Trezutil — Production Readiness (MVP finalization report)

Date: 2026-09-28, updated 2026-09-29 · Details of every defect: [PLATFORM_AUDIT.md](PLATFORM_AUDIT.md) · Open items: [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) · Certification: [MVP_FINAL_CERTIFICATION.md](MVP_FINAL_CERTIFICATION.md) · Operations: [PRODUCTION_DEPLOYMENT.md](PRODUCTION_DEPLOYMENT.md)

## Update — finalization phase (2026-09-29)

The gaps that made this report *conditional* on code are closed:
- **Stripe payments** are integrated: adapter, Stripe.js checkout and signed webhooks.
- The **seller workspace** exists (backend endpoints and UI).
- **Model-assisted conversation** runs through the AI gateway (Anthropic adapter, keyword fallback).
- The **web app was rebuilt on the Figma design** and connected to every flow.
- One new authorization defect was found and fixed: a buyer could confirm their own order.

Current evidence:
- Go: 1,226 passing test cases, 106 of them on real PostgreSQL, 0 failures.
- Frontend: 27 tests pass.
- API end to end: 101/101, twice.
- Browser journeys: 16/16 at desktop and mobile sizes (final clean run).

What remains is external configuration: Stripe merchant accounts, WhatsApp Cloud API credentials, and optionally an AI key. **Status: MVP READY** (see MVP_FINAL_CERTIFICATION.md). The sections below are the original 2026-09-28 report, kept for history; where they conflict with this update, this update wins.

## Executive summary

The repository arrived with sound domain logic but a platform that could not run end to end: there was no authentication, most commerce capabilities were not served by any binary, every web conversation and every order creation failed at the database, public search returned nothing, and three money paths (credits, refunds, double payment) could lose or leak funds. **7 P0 and 21 P1 defect groups were found and fixed**, a deployable topology was built (identity service, commerce API, migration runner, Docker image and compose file, CI), and the result was verified with **955 Go test cases (including 97 against real PostgreSQL), 74 real-OpenSearch tests, 25 frontend tests and a 65-check real-stack end-to-end run** through the public gateway.

**Final status: CONDITIONAL MVP READY.** No P0/P1 defect remains open. The remaining blockers are external: a payment provider integration (none exists in the codebase and adding one was out of scope) and WhatsApp Cloud API credentials with an approved sign-in template.

Two business rules were changed with explicit approval: any verified user may order/book published listings (previously staff-only), and WhatsApp senders are auto-provisioned as customers. Phone-OTP → JWT was implemented as the authentication design (approved).

## Architecture status

The documented invariants hold after the pass: PostgreSQL is authoritative and OpenSearch a rebuildable projection; AI can only propose intents (identity, tenant, confirmation and authorization are applied by the CommerceActionRuntime and use cases); IAM (identity-service + shared JWT validation) owns identity; the gateway is the only public port; WhatsApp is a channel into the same ConversationProcessor as the web; client-supplied user/organization/actor identity is ignored everywhere; retryable mutations are idempotent (orders, reservations, payments, refunds, credit reservations, messages, webhooks); financial invariants are enforced in PostgreSQL (balance guard, exclusion constraint, one live payment per order, version checks) plus a per-payment advisory lock.

## Component status

| Component | Status | Notes |
|---|---|---|
| Foundation / config / bootstrap | FIXED | shared server, DB bootstrap, graceful drain, body limits |
| Ledger | GO | exactly-once postings verified on PostgreSQL |
| IAM / identity | FIXED | phone OTP → JWT, typed tokens, onboarding APIs, admin CLI |
| Catalog / listings | FIXED | server-side IDs/status, tenant check, metadata persisted |
| Search / projection | FIXED | caller-scoping bug, sync loop, suggest, guest access |
| Conversation contracts / processor | FIXED | ownership, dedupe, confirmations, rendering, persistence |
| Commerce Action Runtime | FIXED | real handlers, scoped idempotency, named missing params |
| Orders | FIXED | buyer rule, rounding, idempotency, optimistic lock, payment sync |
| Usage / credits | FIXED | reservation key collapse, sweeper |
| Monetization | GO | grants/renewals idempotent (existing suites pass) |
| WhatsApp | FIXED (mock-validated) | bridge, sender, handshake; live delivery needs credentials |
| Availability | FIXED | tenant bug, single-unit availability |
| Reservations | FIXED | compensation, idempotency, overlap conflict |
| Inventory / capacity | GO | atomic, version-checked; staff-only API |
| Fulfillment | GO (API) | staff-only API; see limitations for order lifecycle UI |
| Provider orchestration | GO | registry/retries/timeouts unchanged |
| AI gateway / orchestrator | CONDITIONAL | no production model adapter; keyword routing active |
| Vertical packs | GO | mutations restricted to operators (activation not persisted) |
| Payment Core | FIXED / CONDITIONAL | authz, locking, order sync; live money needs a provider adapter |
| API gateway | FIXED | new routes, 413, containment, timeouts |
| Frontend | FIXED | links, auth, checkout/orders/bookings, build |
| Deployment / CI | FIXED | Dockerfile, app compose, migrate, CI |

## Defects found

P0: 7 · P1: 21 groups · P2: 11 · P3: 6 (documented). Full list with files: PLATFORM_AUDIT.md §5.

## Defects fixed

All P0, P1 and P2 items. Highlights:

- **Identity**: `packages/auth` middleware on every service; handlers read only the verified principal; `identity-service` issues typed access/refresh JWTs after an HMAC-stored, attempt-limited, rate-limited OTP.
- **Money**: credit reservations keyed per action and per account; payment authorization policy; per-payment advisory lock (try-lock, no pool deadlock); payable-order check and one-live-payment unique index; capture → order PAID; full refund → order REFUNDED.
- **Commerce reachability**: commerce API (`order-service`), WhatsApp bridge, gateway routes, runtime handlers.
- **Data integrity**: UUIDs from constructors, nullable marketplace conversations, order optimistic locking, reservation compensation, listing metadata, projection sync.
- **Frontend**: working navigation, session refresh, checkout, order detail/pay/cancel, my orders, booking with availability, my reservations.

## Test evidence

All commands were run on this workstation (Windows 11, Go 1.26.6, PostgreSQL 16 on :5432, OpenSearch 2.17.1 on :9200).

| Suite | Command | Result |
|---|---|---|
| Format | `gofmt -l cmd internal infrastructure packages` | no output |
| Vet | `go vet ./...` | clean |
| Build | `go build -buildvcs=false ./...` | OK |
| Unit + PostgreSQL integration | `go test -count=1 -p 1 -timeout 30m ./...` | 20/20 packages ok, **955** passing test cases, 0 failures |
| PostgreSQL integration (subset of the above) | `go test -p 1 ./infrastructure/postgres/integration/ -v` | **97/97** pass (fresh DB + all 18 migrations per test) |
| OpenSearch integration | `TREZUTIL_OPENSEARCH_INTEGRATION=true go test -tags=integration ./internal/search/...` | **74** pass, 1 skip (auth-failure case needs the security plugin) |
| Payment determinism | `go test -count=30 ./internal/application/payment/` | 30/30 (previously flaky at `HEAD`) |
| Migrations from empty DB | `go run ./cmd/migrate` on a new database, then again | 000001→000018 applied; second run no-op |
| Frontend | `pnpm exec vitest run`, `tsc --noEmit`, `vite build` | 25/25, clean, built |
| Frozen install | `CI=true pnpm install --frozen-lockfile` (clean copy) | OK |
| Compose | `docker compose -f deployments/docker/docker-compose.app.yml config` | valid; only the gateway publishes a port |
| **Real-stack E2E** | `python test/e2e/journeys.py` (all services + gateway + real PostgreSQL + real OpenSearch) | **65/65**, twice in a row on the same database |

New regression tests (selection): token-type confusion and `alg=none`; forged `?user_id=`; verified principal wins over a body claiming `platform_operator`; customer cannot capture; runtime idempotency cache not shared across actors; named missing parameters; numeric coercion; order rounding/idempotency/unpublished/suspended; reservation overlap + cross-customer key conflict; concurrent full refunds → exactly one provider call (real PostgreSQL); capture marks order paid and blocks a second payment (real PostgreSQL); stale credit reservations released and keys scoped per account (real PostgreSQL); projection sync; gateway routing/413/containment; frontend refresh-and-retry.

### E2E journeys demonstrated (real services, real PostgreSQL, real OpenSearch)

| Journey | Demonstrated |
|---|---|
| Sign-in | OTP request → wrong code rejected → verify → JWT → `users/me`; tampered token rejected; DRC local number normalized |
| Onboarding | non-operator refused; operator creates org, adds seller as owner; seller profile bound to caller with no self-declared trust; store; operator-only categories; public catalogue |
| E Seller | non-member refused; create (server ID, forced draft, metadata) → not searchable → customer cannot publish → seller publishes → searchable within seconds |
| A Discovery | guest search, suggest, listing detail |
| B Conversation | guest "find an apartment in Gombe" → results text + cards; redelivered message replayed without re-execution; "1" opens the first result; messages and context persisted; booking asks guest to sign in; signed-in owner claims the conversation; another user gets 403 |
| C Booking | availability → reservation (confirmed) → retry returns same → overlap 409 → owner sees it, stranger cannot → cancel → slot free again → **8 concurrent bookers of one slot: exactly one succeeds** |
| D Commerce | non-member customer orders (exact minor units) → retry returns same order → listed in "my orders", invisible to others → payment reaches Payment Core and stops at **503 PAYMENT_PROVIDER_UNAVAILABLE** (external) → stranger cannot pay, customer cannot capture |
| G WhatsApp | Meta handshake; bad HMAC 401; signed message → sender auto-provisioned → same processor → search reply; redelivery not re-processed |
| Security | forged `user_id` 401; inventory staff-only; pack activation operator-only; 2 MiB body → 413 |

Capture → ledger → order PAID and refund concurrency are demonstrated in the PostgreSQL integration suite with a test payment adapter (clearly a test double), because no real provider exists.

## Security status

Implemented and verified: JWT authentication on every service (HS256 only, expiry required, issuer/audience, typed tokens); principal-only identity; object-level authorization for orders, reservations, payments, listings, organizations, inventory and fulfillment; conversation ownership; OTP stored as HMAC with attempt limit (5), per-phone (5/h) and per-IP (20/10 min) limits; WhatsApp HMAC + constant-time verify-token; payment webhook signatures; parameterized SQL throughout; 1 MiB body limits; server timeouts; CORS allow-list; web-root containment; no secrets in code (dev defaults only, required-secret guards in compose); OTP codes logged only when `APP_ENV=development`. Residual items (token revocation window, single-instance rate limiter, TLS termination) are in KNOWN_LIMITATIONS.md.

## Data integrity status

Reproducible migrations 000001–000018 with a tracked runner; UUIDs assigned by constructors; optimistic locking on orders, reservations, payments, inventory, capacity; DB exclusion constraint against double booking with compensation of holds; unique keys for idempotency (orders per customer, reservations, payments, refunds, credit reservations per key, messages per conversation, webhooks per channel); search is a rebuildable projection resynchronized hourly.

## Financial integrity status

Credits: atomic, balance-guarded reservations keyed per action and account; no negative balances under concurrency; orphaned holds released. Payments: amount/currency must equal the order total; only payable orders; one live payment per order; money-moving operations serialized per payment; ledger postings exactly once; refunds capped by the remaining refundable amount. No raw card/PIN data is stored (tokenized method references only).

## Frontend status

Working end to end against the gateway: discovery and search (guest), listing detail, phone sign-in with transparent session refresh, conversation, checkout → order → pay (shows a clear message while no provider is configured) → my orders, booking with availability check → my reservations. The existing visual direction is preserved; new pages reuse its tokens and components. The seller console (listing creation form, order queue, booking calendar) remains a "coming soon" screen.

## Deployment status

`deployments/docker/Dockerfile` (one image, all binaries + migrations + SPA, non-root) and `deployments/docker/docker-compose.app.yml` (PostgreSQL, OpenSearch, one-shot `migrate`, six services, gateway as the only published port). Startup order: PostgreSQL/OpenSearch healthy → migrate completes → services → gateway. Configuration: `deployments/docker/app.env.example`, root `.env.example`. CI runs format, vet, build, all Go tests against PostgreSQL and OpenSearch service containers, and the frontend typecheck/test/build. First operator: sign in once, then `trezutil-admin grant-role -phone … -role operator`. The image and compose stack were validated statically (`docker compose config`); Docker was not running on this workstation, so the image itself was not built here — the equivalent binaries were built and run natively for every test above.

## Remaining limitations

See KNOWN_LIMITATIONS.md. Blocking for launch: payment provider adapter; WhatsApp Cloud API credentials + approved OTP template.

## MVP readiness

**CONDITIONAL MVP READY** — every production-critical path works and is tested; the only blockers require external providers/credentials.
