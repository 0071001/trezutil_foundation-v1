# Trezutil — Platform Audit (MVP finalization)

Date: 2026-09-28 · Scope: the whole repository as found on `main` (commit `dde5499`) plus the uncommitted work-in-progress, audited, fixed and re-validated in one pass.

Companion documents: [PRODUCTION_READINESS.md](PRODUCTION_READINESS.md) (final report, status, evidence) and [KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md).

---

## 1. What was found on arrival

The domain and application layers were largely sound and well tested in isolation, but the **deployable platform did not work end to end**:

| Area | State found |
|---|---|
| Authentication | None. Every endpoint trusted `?user_id=`, `organization_id` and, in conversations, a self-declared `actor.type` (e.g. `platform_operator`). `identity-service` was a placeholder; the JWT service was unused. |
| Service wiring | Orders, reservations, availability, inventory, fulfillment and the WhatsApp webhook had handlers but **no binary served them**; the gateway had no routes for them. |
| Conversation | Runtime had an **empty handler registry**; every web conversation insert failed (NOT NULL organization/customer); guests failed validation. |
| Search | Every public search was filtered to `seller_id = <searching user>` (returned nothing); published listings were never projected to OpenSearch. |
| Money | Credit reservations collapsed onto one empty idempotency key platform-wide; payments had no authorization; concurrent refunds could each refund at the provider. |
| Persistence | Several constructors produced empty UUIDs: order creation always failed; message and fulfillment writes silently failed. |
| Deployment | Only an infrastructure compose file; no app images, no migration runner; CI pinned Go 1.23 against a 1.26.6 module and had no database for DB-backed suites. |

All of the above are fixed (section 5) and verified by unit, PostgreSQL integration, OpenSearch integration and real-stack E2E tests (see PRODUCTION_READINESS.md §Test evidence).

---

## 2. Services

All services share PostgreSQL (authoritative) through `DATABASE_URL`, validate the same HS256 JWT (`JWT_SECRET`) and use `packages/common` for timeouts (read-header 10s, read/write 30s), a 1 MiB body limit and graceful drain on SIGTERM.

| Binary | Responsibility | Port | Depends on | Health / readiness |
|---|---|---|---|---|
| `api-gateway` | Only public entry point. Serves the SPA, proxies `/api/v1/*`, request IDs, CORS allow-list, 1 MiB body cap, 404/502/504 normalization. | 8080 | all upstreams | `/healthz`; `/readyz` TCP-pings every upstream |
| `identity-service` | Phone-OTP sign-in → JWT (access 15 min / refresh 30 d), `users/me`; operator-assisted onboarding (organizations, members, seller profiles, stores). | 8086 | PostgreSQL, WhatsApp Cloud API (OTP) | `/healthz`, `/readyz` (DB ping) |
| `search-service` | Public discovery (`/v1/search`, `/v1/search/suggest`), listing management (`/v1/listings*`), category catalogue; **PostgreSQL → OpenSearch projection sync** (keyset tail every 5 s + hourly full resync). | 8081 | PostgreSQL, OpenSearch | `/healthz`; `/readyz` (OpenSearch + DB) |
| `conversation-service` | Channel-neutral ConversationProcessor for web (`/api/v1/conversations*`) and WhatsApp (`/api/v1/webhooks/whatsapp`), CommerceActionRuntime with real handlers, vertical packs; orphaned-credit sweeper. | 8091 | PostgreSQL, OpenSearch, WhatsApp Cloud API (replies) | `/healthz`, `/readyz` |
| `order-service` (commerce API) | Orders, "my orders", reservations, "my reservations", availability; inventory and fulfillment for organization staff. | 8085 | PostgreSQL | `/healthz`, `/readyz` |
| `payment-service` | Payment Core (`/api/v1/payments*`), provider webhooks, ledger, per-payment advisory locking, order status sync. | 8083 | PostgreSQL, payment provider adapter (none shipped) | `/healthz`, `/readyz` |
| `catalog-service` | Same listing API as search-service; **not routed** by the gateway (kept for internal use). | 8082 | PostgreSQL | `/healthz`, `/readyz` |
| `migrate` | Applies `migrations/*.sql` transactionally, tracked in `schema_migrations`; `-status`, `-baseline`. | CLI | PostgreSQL | — |
| `trezutil-admin` | Operator CLI: `grant-role` / `revoke-role` by phone (bootstraps the first platform operator). | CLI | PostgreSQL | — |
| `fileserver`, `admin-service`, `logistics-service`, `marketing-service`, `notification-service`, `seller-service`, `trust-service` | Legacy static server / foundation placeholders that log and exit. Not deployed. | — | — | — |

## 3. Public API (`/api/v1/*` → upstream)

| Prefix | Upstream | Auth |
|---|---|---|
| `/auth` (`request-otp`, `verify-otp`, `refresh`, `users/me`) | identity | public except `users/me` |
| `/organizations`, `/organizations/members`, `/sellers`, `/stores` | identity | JWT; org creation = platform operator; member/store changes = org owner/admin; org reads = members |
| `/search`, `/search/suggest`, `/listings`, `/categories` | search (strips `/api`) | search/suggest/listing read/category read public; listing writes = seller owner or org owner/admin/manager; category writes = operator |
| `/conversations/message`, `/conversations/{id}/confirm|reject` | conversation | guest or JWT; confirm/reject = conversation owner |
| `/packs` | conversation | read public; mutations = platform operator |
| `/webhooks/whatsapp` (GET handshake, POST events) | conversation | Meta verify token / `X-Hub-Signature-256` HMAC |
| `/orders`, `/orders/{id}`, `/orders/{id}/cancel|confirm` | commerce | JWT; customer or org staff |
| `/reservations`, `/reservations/{id}`, `/reservations/{id}/cancel`, `/availability` | commerce | JWT |
| `/inventory`, `/fulfillments/*` | commerce | JWT + org owner/admin/manager of `tenant_id` |
| `/payments*` | payment | JWT; create/view/cancel = order's customer or org staff; capture/refund = org staff |
| `/webhooks/payments/{provider_code}` | payment | provider signature |

The browser only ever talks to the gateway origin; the SPA uses relative `/api/v1` URLs and contains no internal hosts or ports.

## 4. Data and flows

**Stores.** PostgreSQL is the system of record (18 migrations, reproducible from an empty database). OpenSearch holds a rebuildable read projection (`trezutil-listings-v1` behind alias `trezutil-listings`). Redis, Kafka and MinIO appear in the infrastructure compose file but are **not used** by any MVP service.

**Frontend.** `Browser → api-gateway (SPA + /api/v1) → service`. Access token in `localStorage`, refreshed transparently on 401 with the refresh token.

**AI / conversation.**
`Client → ConversationHandler (principal or guest) → ConversationProcessor (ownership, dedupe, context, confirmation, clarification) → IntentClassifier (keyword; AI adapter optional) → CommerceActionRuntime (identity override, tenant resolution, validation, scoped idempotency, confirmation policy, credits) → handler → application use case → PostgreSQL / OpenSearch`.
The AI layer can only propose an intent + parameters; identity, tenant, confirmation and authorization are always applied by the runtime and use cases. No production model adapter ships (see limitations); a failing or unsure classifier degrades to keyword routing.

**Payments.**
`Order (draft) → CreatePayment (order must be draft/pending_payment; one live payment per order; amount/currency must equal order total) → order pending_payment → provider authorize/capture (per-payment advisory lock) → ledger DEBIT cash / CREDIT revenue (exactly once by reference) → order PAID`. Refund: `lock → refundable check → provider refund → refund row → ledger → payment refunded amount → order REFUNDED when fully refunded`. Webhooks reconcile the same states and sync the order.

**Usage / credits.**
`Plan → Subscription → Billing period → Credit grant → Credit account → Reserve (atomic, balance-guarded, idempotent per account+key) → Consume | Release → usage_ledger`. Metering applies only to actions with an active `usage_policies` row (none seeded). Orphaned reservations (>15 min active) are released by the conversation-service sweeper.

## 5. Defects found and fixed

Severity: **P0** production blocker / security / money or data corruption · **P1** serious functional or reliability defect · **P2** important, fixed because it blocked a coherent MVP.

### P0

| # | Defect | Fix (files) |
|---|---|---|
| P0-1 | No authentication: identity from `?user_id=`/body; conversation actor type self-declared. | Phone-OTP → JWT identity service; `packages/auth` Bearer middleware in every service; all handlers use the verified principal (`packages/auth/*`, `internal/application/identity/*`, `internal/api/identity_handlers.go`, `handlers.go`, `payment_handlers.go`, `search_handlers.go`, `provider_handlers.go`, `conversation_handlers.go`, all `cmd/*`). |
| P0-2 | Refresh tokens (30 days) accepted as access tokens. | Typed `typ` claim, `ValidateRefreshToken`, HS256-only, expiry required (`internal/domain/token.go`). |
| P0-3 | Credits: `CommerceAction.ActionID` empty → every metered action reused one reservation key platform-wide (free usage after the first action, cross-tenant lookup). | IDs generated in constructors; empty keys rejected; lookup scoped by account; nullable tenant scan (`internal/domain/action.go`, `id.go`, `infrastructure/postgres/monetization.go`). |
| P0-4 | Payment API unauthorized: anyone knowing IDs could capture/refund. | `payment.AccessPolicy` (customer vs. org staff) enforced on every endpoint (`internal/application/payment/access.go`, `internal/api/payment_handlers.go`). |
| P0-5 | Concurrent refunds/captures passed the checks together and each hit the provider (over-refund). | Per-payment PostgreSQL advisory lock around the whole operation; try-lock loop to avoid pool-starvation deadlock (`internal/application/payment/lock.go`, `service.go`, `infrastructure/postgres/advisory_lock.go`). |
| P0-6 | Paid/cancelled orders accepted new payments; several live payments per order possible (double charge). | Order must be draft/pending_payment; partial unique index "one live payment per order"; duplicate mapping (`service.go`, `payments.go`, migration 000018). |
| P0-7 | Conversation hijack: presenting another user's external ID continued their conversation, and "yes" executed their pending action **as them**. | Ownership check/claim; pending actions execute only as, and only for, the requesting authenticated actor; HTTP confirm requires owner (`internal/application/conversation/processor.go`). |

### P1

| # | Defect | Fix |
|---|---|---|
| P1-1 | Empty UUIDs from domain constructors: order creation always failed; messages/fulfillments silently not stored; capacity holds recorded without reservation id. | `domain.NewID()` in 12 constructors (`internal/domain/*`). |
| P1-2 | `conversations.organization_id/customer_id` NOT NULL FKs → every web (marketplace/guest) conversation failed. | Nullable columns + NULL-aware store (migration 000018, `conversations.go`). |
| P1-3 | conversation-service runtime had no handlers, no confirmation policy, validator or idempotency store. | Real handlers for search, suggest, listing get/publish/pause/resume, orders, reservations, availability (`cmd/conversation-service/main.go`). |
| P1-4 | Orders, reservations, availability, inventory, fulfillment, WhatsApp webhook not served; gateway missing routes. | `order-service` = commerce API; WhatsApp routes in conversation-service; gateway routes (`cmd/order-service`, `internal/gateway/*`). |
| P1-5 | Public search filtered to the searcher's own seller id; `page_size` 0 rejected; search/suggest required sign-in; suggest decoder broken. | Removed injection; defaults/caps; guest access; phrase-prefix autocomplete (`search_service.go`, `search_handlers.go`, `execution.go`). |
| P1-6 | Published listings never reached OpenSearch; listings without an availability row could never be indexed. | `ProjectionSync` + keyset change source; default availability (`internal/search/application/projection_sync.go`, `listings.go`, `projection/source.go`). |
| P1-7 | Availability used an empty tenant in a UUID comparison (every check errored); listings without inventory were never bookable. | Tenant carried in query; single-unit availability from overlapping reservations (`capacity_availability_provider.go`). |
| P1-8 | Customers could not order or book (staff-only rule contradicting the spec). | **Approved**: any active user may buy/book published listings; unpublished listings rejected (`orders.go`, `reservation_use_cases.go`). |
| P1-9 | Orders: price truncation (19.99 → 1998), no idempotency (TODO), no optimistic lock. | `math.Round`, `(customer_id, idempotency_key)` unique, `WHERE version = v-1` (`orders.go`, migration 000018). |
| P1-10 | Capture never marked the order paid; payment creation never submitted the order. | `syncOrderWithPayment` / `submitOrderForPayment` in service and webhook (`order_sync.go`). |
| P1-11 | Reservations: capacity/inventory leaked when the exclusion constraint rejected a booking; retry failed availability; idempotency key IDOR; cancel could double-release. | Compensation, idempotency first and caller-scoped, 23P01 → conflict, status persisted before release (`reservation_use_cases.go`, `reservations.go`). |
| P1-12 | Conversation: substring yes/no ("book" confirmed via "ok"), confirmations never expired, unusable confirm API + fake `/token`, empty missing-param names, no message dedupe, replies not stored, results not rendered/referenceable, keyword search without text, AI without adapter failed every message. | Word-level matching, 10-min expiry, owner-bound tokens, named parameters, dedupe + reply replay, reply persistence, text + cards + ordinal references, keyword fallback (`processor.go`, `intent_classifier.go`, `validation.go`, `runtime.go`). |
| P1-13 | WhatsApp: separate path bypassing confirmations/context; replies never delivered; unknown senders rejected; no subscription handshake. | Bridge through the ConversationProcessor, Cloud API sender, **approved** sender auto-provisioning, `GET` handshake (`internal/application/whatsapp/bridge.go`, `internal/infrastructure/whatsapp/sender.go`, `whatsapp_handler.go`). |
| P1-14 | Runtime idempotency cache keyed by client key only (cross-actor result disclosure); numeric parameters from JSON/AI ignored. | Tenant/actor/action-scoped keys; `intParam` coercion (`runtime.go`, `reservation_handlers.go`). |
| P1-15 | Listing creation: client-chosen ID and `published` status; seller without organization could attach listings to any store; metadata discarded. | Server IDs, forced draft, org match required, JSON metadata (`listings.go`, `infrastructure/postgres/listings.go`). |
| P1-16 | No request body limits anywhere (memory exhaustion). | Gateway 413 + `MaxBytesReader`; service-side 1 MiB (`gateway.go`, `packages/common/http.go`). |
| P1-17 | Credit reservations orphaned by a crash were held forever. | `ReleaseStaleReservations` sweeper (`monetization.go`, conversation-service). |
| P1-18 | No migration runner; roles never seeded (role assignment impossible); `users.phone` not unique. | `cmd/migrate`, seeded roles, unique phone (migration 000018). |
| P1-19 | Seller onboarding (MVP scope) unreachable; seller profile could be bound to another user or claim "verified". | Onboarding routes on identity-service with existing IAM rules; server-bound owner, earned trust; operator CLI; category catalogue (`cmd/identity-service`, `sellers.go`, `category_handlers.go`, `cmd/trezutil-admin`). |
| P1-20 | Frontend: most internal links pointed at unrouted `/app/*` paths; login redirected to non-existent routes; refresh token discarded; checkout/orders/bookings were hard-coded placeholders; production build broken by an unclosed `@media`; invalid `pnpm-workspace.yaml`. | Link rewrite, safe relative `returnTo`, refresh-and-retry client, real checkout/order/booking pages, CSS fix (`web/frontend/src/*`). |
| P1-21 | CI could not pass (Go 1.23 vs 1.26.6; DB suites without a database); stale vulnerable binaries committed. | CI with Postgres + OpenSearch services, frontend job; binaries removed and ignored (`.github/workflows/ci.yml`, `.gitignore`). |

### P2 (fixed)

Payment-test mocks that shared pointers and skipped version checks (flaky "concurrency" tests; one masked the real P0-5); a concurrent map write in a test adapter; pack activation mutations were unauthenticated; inventory/fulfillment handlers trusted `tenant_id` (now staff-guarded); `RemoveMember` targeted the caller; organization member list was public; AvailabilityResult had no JSON field names; 5xx responses echoed internal errors in conversation/search/WhatsApp paths; explicit web-root containment in the gateway static handler; services used `server.Close()` (no drain) and silent dev database defaults.

### P3 (documented, not changed)

Legacy vanilla web app in `web/js` + `web/index.html` and `HomepageDesign/` (not served); placeholder service binaries; `catalog-service` duplicates search-service's listing API; development defaults for upstream URLs in `internal/gateway/config.go` (env-overridable); domain aggregates serialize as PascalCase JSON (the SPA maps both casings); `ModifyReservation` returns not-implemented (not exposed).

## 6. Keyword sweep (brief §6)

`TODO/FIXME/HACK/XXX`: none left in production Go code (the one TODO — order idempotency — was implemented). `panic`/`log.Fatal`: only in `cmd/*` startup paths. SQL string formatting: none in production code (test harness only, constant names). `NOT_IMPLEMENTED`: runtime response for chat actions without a handler (payments, listing creation from chat) — now answered honestly. `mock`: `infrastructure/ai/mock_provider.go` is test-only and not wired. `localhost`/ports: env-overridable development defaults only; the SPA contains none.

## 7. Finalization phase (2026-09-28/29) — gaps closed

| Area | Change | Verified by |
|---|---|---|
| Payments | **Stripe adapter** (`internal/infrastructure/payment/stripe`): PaymentIntents (automatic/manual capture, server confirmation with `pm_` tokens, Stripe.js client action), capture, cancel, refunds (sync and async), `Idempotency-Key` on every mutation, timeouts, typed error mapping (declines vs retryable), `Stripe-Signature` verification with 5-minute replay tolerance; provider rows loaded from `providers` with `env:` secret references and periodic refresh | adapter + webhook unit tests; 7 real-PostgreSQL integration tests (success, decline→retry, browser confirmation via webhook, timeout→resume, capture replay, refund replay/over-refund/async, signature/tenant/amount guards); E2E journey S |
| Payment Core defects | P1: a replayed create or authorize created a **second provider intent** → now resumes the existing one; P1: the provider-config codec never decoded (settings lost) → JSON codec with legacy fallback; P1: async refund updated without a version bump (optimistic-lock failure) → `Refund.MarkSubmitted`; P2: payment/ledger/order were touched for non-succeeded refunds → only on success; P2: reserved metadata keys accepted from clients → stripped | tests above |
| Seller console backend | `GET /sellers/me`, `GET /listings/manage`, `POST /listings/update` (draft/paused only, owner/admin/manager), `GET /orders/incoming`, `GET /reservations/incoming`, all under server-side membership/role guards; listing detail now returns `Media` | real-PostgreSQL isolation tests; E2E SELLER checks (cross-org 403s, edit guard) |
| **Order authorization (P1)** | A **buyer could confirm their own order** (and could reach `ReadyForFulfillment`/`CompleteOrder`) because the seller transitions reused the buyer-inclusive access check. New `canManageOrder` (seller owner, org owner/admin/manager, platform admin/operator) | `TestSellerConsole_OrderConfirmationIsSellerOnly`; E2E "customer cannot confirm their own order as seller" |
| Conversation | Pause/resume/publish routing; "book it" / "buy it" / "pause this listing" resolve the listing in focus; contextual resolution only for `*_id` parameters | `contextual_flow_test.go`; E2E AI journey |
| AI | **Anthropic provider** (`internal/infrastructure/ai/anthropic`) behind the existing AI gateway/router/allowlist; strict JSON-schema output; usage metering on the classifier path; timeouts owned by the gateway; env-configured (`AI_PROVIDER`) with keyword fallback. P1 defect fixed: the AI sanitizer **lowercased every user message** sent to the model (names, places and IDs lost) → case-preserving redaction. The system prompt now lists the allowed intents and parameters (previously the model was never told them) | provider contract tests against an emulator (request shape, retries, 401/404/429/5xx, timeout, malformed/refused/truncated output); classifier tests (allowlist, confidence, identity injection, metering, guests); E2E AI journey |
| Deployment | compose/env templates carry the Stripe and AI settings; SPA build argument for the public WhatsApp number | `docker compose config` |

## 8. Frontend (Figma implementation)

The web app was rebuilt on the Figma "Premium Homepage Design" export (see docs/design/FIGMA_IMPLEMENTATION_MAP.md and FRONTEND_PRODUCTION_READINESS.md). Defects found and fixed while connecting it to the real stack:

- **API contract:** `POST /orders` and `POST /reservations` return short DTOs without the organization or items, while payment needs the organization. The client now reads back the canonical entity (a unit test had masked this by mocking the aggregate; the mocks now use the real shape).
- **Sign-out race:** signing out on a protected page redirected to sign-in (the router navigation is a transition while clearing the session is urgent). Fixed with an explicit signed-out state.
- **Dark mode:** Figma swaps tokens on `.app.dark` but never sets `.app { color }`, so uncolored text (brand, prices, chat header) was dark on dark. Fixed.
- **Mobile (Figma gaps):** filter options, the conversation sidebar, the seller navigation and seller row actions are all hidden below 760 px with no alternative. Added a filter sheet, a chat panel toggle, seller tabs and visible row actions.
- **Honesty:** the listing editor's "stock quantity" was stored in a table that nothing enforces. It was removed, and the gap is documented (KNOWN_LIMITATIONS L2).
