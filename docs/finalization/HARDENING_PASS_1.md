# Trezutil — Production hardening, pass 1

Scope: configuration, security, staging deployment and operational verification. No product features. Companion to `PRODUCTION_DEPLOYMENT.md` (operator guide) and `MVP_FINAL_CERTIFICATION.md`.

## 1. AI provider configuration

| Item | Value |
|---|---|
| Provider | Anthropic Messages API, behind the provider-neutral `domain.AIProvider` interface (AI Gateway, model router, capability matrix, action allowlist). Anthropic is optional: without `AI_PROVIDER` the platform uses deterministic keyword routing. No OpenAI or Gemini adapter exists in the code. |
| Default model | `claude-opus-5`, the single constant `DefaultModel` in `internal/infrastructure/ai/anthropic/provider.go`. It is a documented Anthropic model id (Anthropic's model list, cached 2026-09-25); `claude-opus-5-5` is also documented. Not verified against the live API in this pass (no key). |
| Selection | `AI_MODEL`; empty means the default. No other place hard-codes a model id (env examples show the same value as documentation). |
| Related variables | `AI_PROVIDER` (`anthropic` or empty), `ANTHROPIC_API_KEY`, `AI_BASE_URL` (proxy or emulator), `AI_EFFORT` (`low`\|`medium`\|`high`, default `low`), `AI_REQUEST_TIMEOUT_SECONDS` (15), `AI_TIMEOUT_SECONDS` (30), `AI_MAX_RETRIES` (2), `AI_VERIFY_ON_START` (`true`). |
| Malformed `AI_MODEL` | The service **refuses to start** (`invalid AI_MODEL`); the shape must be `claude-<family>-<version>[-date]`. |
| Startup verification | With a key, the service asks the API `GET /v1/models/{id}` once at boot. A definite 400/404 (unknown model) or 401/403 (key rejected) is fatal; a network failure or 5xx only logs a warning and the service starts. Disable with `AI_VERIFY_ON_START=false`. |
| No credentials | `AI_PROVIDER` unset, or set without a key: the service starts and uses keyword routing (logged). |
| Provider unavailable at runtime | Timeouts and errors are bounded by the gateway deadline; the conversation processor falls back to keyword routing, so chat keeps working. |
| Authority | The model only classifies intent into an allow-listed action. Every action still runs through the authenticated Commerce Action Runtime (authorization, usage/credits, explicit confirmation for orders and bookings). |

## 2. Error responses

Every 5xx returns the same body message (`An internal error occurred. Quote the request id (X-Request-ID) if you contact support.`) whatever the handler passed. The original text is logged as `error_detail` with `request_id`, `error_code`, route and service, and never sent: it travels in an internal header that the service middleware removes before the response leaves the process. Permission denials that used to surface as a 500 are now 403; an unavailable dependency (deadline, cancelled request) is 503; refund business rules are 409. Regression tests: `internal/api/error_leak_test.go`.

## 3. Configuration matrix

Legend: **REQUIRED** must be set; **OPTIONAL** feature off when unset; **SAFE DEFAULT** default is fine for production; **NO DEFAULT** must be supplied and has no default in production. "Enforced" means the service or compose refuses to start otherwise.

| Configuration | Class | Source | Production-safe | Notes |
|---|---|---|---|---|
| `APP_ENV` | SAFE DEFAULT (`production` in compose) | app.env | yes | Any value except `development`/`test` (including unset) is strict: strong secrets, explicit CORS, no dev OTP sender, `DATABASE_URL` required. |
| `LOG_LEVEL` | SAFE DEFAULT (`info`) | app.env | yes | |
| `POSTGRES_PASSWORD` | REQUIRED / NO DEFAULT | app.env | enforced by compose (`:?`) | Random, 24+ chars. |
| `DATABASE_URL` | REQUIRED | built by compose | enforced (non-development) | `POSTGRES_SSLMODE` defaults to `disable` (private compose network); use `require` for any external database. |
| `JWT_SECRET` | REQUIRED / NO DEFAULT | app.env | **enforced**: 32+ chars, 10+ distinct characters, no placeholder words | Shared by every service. |
| `JWT_ISSUER`, `JWT_AUDIENCE`, `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` | SAFE DEFAULT | app.env | yes | 15 min access, 30 day refresh. |
| `OTP_CODE_SECRET` | REQUIRED / NO DEFAULT | app.env | **enforced**: strong and different from `JWT_SECRET` | Was derived from the JWT secret; now separate outside development. |
| `CORS_ALLOWED_ORIGINS` | REQUIRED / NO DEFAULT | app.env | **enforced**: set, no `*`, no `example` placeholder | Compose has no default any more. |
| `METRICS_TOKEN` | REQUIRED for monitoring | app.env | gateway serves `/metrics` only with it; unset = not served | Services answer `/metrics` only to private networks. |
| `TRUSTED_PROXY_HOPS` | SAFE DEFAULT (0; set 1 behind a proxy) | app.env | yes | Makes per-IP rate limits see the real client instead of the proxy. |
| `OPENSEARCH_URL`, `OPENSEARCH_INDEX/ALIAS` | SAFE DEFAULT | compose | yes | Internal network only; `DISABLE_SECURITY_PLUGIN=true` is acceptable only there. Enable TLS/auth (`OPENSEARCH_USERNAME/PASSWORD/TLS`) before exposing it. Single node reports `yellow` (replicas unassigned): expected. |
| `SEARCH_SYNC_INTERVAL`, `SEARCH_FULL_RESYNC_INTERVAL`, `OPENSEARCH_*` tuning | SAFE DEFAULT | env | yes | |
| `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_OTP_TEMPLATE` | REQUIRED for sign-in delivery | app.env | identity-service refuses to start without any OTP delivery outside development | Meta credentials and an approved authentication template. |
| `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN` | REQUIRED for WhatsApp conversations | app.env | webhook signature and handshake | Unset: webhook rejects. |
| `STRIPE_SECRET_KEY`, `STRIPE_WEBHOOK_SECRET` | OPTIONAL (payments off per organization when unset) | app.env, referenced by the organization's `providers` row (`env:` refs) | yes | Never stored in the database; use `sk_test_` keys in staging. |
| `PAYMENT_PROVIDER_REFRESH_SECONDS` | SAFE DEFAULT (60) | env | yes | |
| `AI_PROVIDER`, `ANTHROPIC_API_KEY`, `AI_MODEL` and tuning | OPTIONAL | app.env | yes | See section 1. |
| `VITE_WHATSAPP_NUMBER` | OPTIONAL, public | build arg | yes | Baked into the SPA. |
| `GATEWAY_PORT`, `HTTP_PORT`, service URLs, `WEB_ROOT` | SAFE DEFAULT | compose | yes | Only the gateway publishes a port. |
| Service-to-service authentication | by design | — | accepted | Internal services are not published and each validates the caller's JWT itself with the shared secret; there is no separate service credential. Keep the compose network private. |

No real secret is committed: env examples hold empty values and the documentation placeholder `https://trezutil.example` no longer works as a value.

## 4. Alerts (Prometheus)

Rules: `infrastructure/prometheus/alerts.yml`, validated with `promtool check rules` and loaded by Prometheus in the staging stack (7 rules, all healthy). Overlay: `deployments/docker/docker-compose.monitoring.yml`.

| Alert | Fires when | Meaning | First operator action |
|---|---|---|---|
| TrezutilHigh5xxRate | >5% of requests are 5xx for 10 min (and >0.2 req/s) | A service is failing | Service logs, `level=error`, group by `error_code`, follow one `request_id` |
| TrezutilPaymentFailures | ≥5 failed payments in 15 min | Provider or configuration problem (single declines are normal) | `event=payment_failed` (stage, reason); Stripe dashboard; `STRIPE_SECRET_KEY` |
| TrezutilWebhookFailures | ≥3 rejected/failed webhooks in 15 min | Wrong webhook secret, forgery attempt, or processing error | `WEBHOOK_SIGNATURE_INVALID`: compare secrets with the provider; else `event=webhook_failed` by `request_id` |
| TrezutilReservationErrors | ≥3 internal/unknown-availability reservation failures in 15 min | Business refusals (conflict, capacity) are excluded | `event=reservation_failed` with `reason=error`; database and provider executions |
| TrezutilProviderFailures | ≥5 failed provider attempts in 15 min | An external provider is down | `provider execution` logs (`error_code`, `retry_class`); disable the provider |
| TrezutilNotReady | a service reports not ready for 5 min | Database unreachable | `curl /readyz`; PostgreSQL health; restart |
| TrezutilServiceDown | a scrape target is down for 2 min | Process or network problem | `docker compose ps`, `/healthz`; `METRICS_TOKEN` for the gateway job |

Routing to people (Alertmanager, PagerDuty, email) is deployment-specific and not included.

## 5. Security review (focused)

Verified on the staging stack (API E2E 171/171, Playwright 20/20) or by tests:

- Identity from the verified JWT only; a forged `?user_id`, forged `X-User-ID` and an unsigned (`alg:none`) token are 401.
- Object-level authorization: orders, reservations, payments, provider records, hotel inventory, seller console (seller/customer and seller/seller isolation) — E2E H5, SELLER, SEC and CART groups.
- Webhooks: signature required (401 otherwise), replay window enforced, duplicates idempotent — E2E S group.
- Idempotency: orders, carts, payments, stays.
- Metrics: gateway 404 without token, 404 with a wrong token; internal services not reachable from the host.
- Request bodies: gateway answers 413 for an oversized body.
- CORS: an origin outside the allowlist gets no CORS headers; the configured origin does.
- Logs: after a full E2E run, 0 occurrences of the JWT secret, OTP secret, metrics token, database password, Stripe/WhatsApp/Anthropic keys, `Bearer` headers, JWTs or OTP codes in any service log.

Findings fixed in this pass:

| # | Finding | Fix |
|---|---|---|
| 1 | 5xx responses echoed raw internal error text (order creation and ~44 other handlers) | Generic 5xx body, detail to the log only (section 2) |
| 2 | Permission denials in organization, listing, payment and other handlers returned 500 (and would have raised false 5xx alerts) | Mapped to 403; timeouts to 503; refund rules to 409 |
| 3 | Weak or placeholder `JWT_SECRET` (length-only check); OTP hashing key derived from the JWT secret | Strength check and separate strong `OTP_CODE_SECRET` outside development |
| 4 | CORS allowlist could default to a localhost or documentation value | Must be explicitly set; `*` and the placeholder refused |
| 5 | Per-IP rate limits would see only the reverse proxy's address behind a TLS proxy (one shared limit for all users) | `TRUSTED_PROXY_HOPS` |
| 6 | No baseline browser security headers | `nosniff`, `X-Frame-Options: DENY`, referrer and permissions policy, HSTS when served over TLS |
| 7 | The image would have shipped the on-hold dispatch migration | `.dockerignore` |
| 8 | Wrong `AI_MODEL` or key discovered on the first customer request | Boot-time validation and verification (section 1) |
| 9 | Floating assistant dock could cover the last result on phones | Bottom padding (CSS) |

Remaining risks (not fixed here, by decision or dependency):

- No Content-Security-Policy: set it at the reverse proxy once the payment provider's script/frame origins are known.
- Audit trail is a log stream, not a durable tamper-proof table (out of scope for this pass).
- Internal services trust each other's network position; keep the compose network private.
- OpenSearch runs without its security plugin on the internal network.
- Rate limiting is per process (in memory); add an edge limiter for several identity replicas.
- Real Stripe, WhatsApp/Meta and Anthropic behaviour is unverified until credentials exist (section 6).

## 6. External integrations: what was and was not tested

| Integration | Status |
|---|---|
| Stripe test mode | **STRIPE LIVE TEST NOT EXECUTED — CREDENTIALS REQUIRED.** Verified against the Stripe emulator with the real adapter, HTTP, signatures and idempotency keys (E2E S group). |
| WhatsApp / Meta | **WHATSAPP LIVE SANDBOX TEST NOT EXECUTED — CREDENTIALS REQUIRED.** Verified with locally signed webhooks (E2E G group); outbound sign-in codes went to a local stand-in for the Graph API, which proves the code path and not Meta. |
| AI provider | **AI LIVE STAGING TEST NOT EXECUTED — CREDENTIALS REQUIRED.** The conversation journeys ran through the production path against the Anthropic emulator; startup behaviour for a bad model, bad key, missing key and unreachable provider was exercised on the staging image. |

## 7. Staging verification record

Stack: `docker-compose.app.yml` + `docker-compose.monitoring.yml` on a clean database (`docker compose down -v`, then `up`), `APP_ENV=staging`, randomly generated secrets, image built from the repository (`docker build -f deployments/docker/Dockerfile`).

- Containers: PostgreSQL 16 (healthy), OpenSearch 2.17.1 (healthy; cluster `yellow` on one node, as expected), migrate (exit 0), identity, search, conversation, order, payment services, api-gateway (healthy, serves the built SPA), Prometheus.
- Migrations: 000001–000021 applied, `schema_migrations` = 21 rows; the image contains no 000022.
- Probes: gateway `/healthz` 200 and `/readyz` 200 with all five upstreams healthy; each service `/readyz` ready; internal service ports are not reachable from the host; gateway routing (`categories`, `verticals`, `packs`, `search` 200, `orders` 401 without a token).
- Metrics: gateway `/metrics` 404 without a token and with a wrong one, 200 with the token; services serve `/metrics` inside the network. Prometheus: 7 rules loaded and healthy, 6 of 6 scrape targets up, no alert firing after the full E2E run.
- Strict startup: seven misconfigurations (placeholder and short JWT secret, missing OTP secret, OTP secret equal to the JWT secret, missing / wildcard / placeholder CORS) each stop the service with a clear message. AI: malformed model and rejected key are fatal; missing key, unset provider and unreachable provider start normally.
- E2E: API journeys 171/171 and Playwright 20/20 against this stack. Stripe and Anthropic were emulated and sign-in codes went to a local stand-in for the WhatsApp Graph API (section 6).
- Logs: no secret, token, header value, JWT or OTP code in any service log; `audit`, `payment_failed`, `webhook_failed` and `reservation_failed` events present with request ids.

Environment notes for running this on Docker Desktop for Windows: address the stack as `127.0.0.1` (the IPv6 loopback relay can stall after an aborted large upload), and the compose network is `internal`.
