# Trezutil — Production deployment guide

This guide takes a new environment from nothing to a pilot deployment. It contains **no real secrets**: every value shown is a placeholder.

## 1. Topology

| Component | Image / command | Port (internal) | Public? |
|---|---|---|---|
| PostgreSQL 16 | `postgres:16` | 5432 | no |
| OpenSearch 2.17 | `opensearchproject/opensearch:2.17.1` | 9200 | no |
| migrate (one-shot) | `/app/bin/migrate` | — | no |
| identity-service | `/app/bin/identity-service` | 8086 | no |
| search-service (listings, search, categories) | `/app/bin/search-service` | 8081 | no |
| conversation-service (web chat, WhatsApp webhook, AI) | `/app/bin/conversation-service` | 8091 | no |
| order-service (orders, reservations, availability, inventory, fulfilment) | `/app/bin/order-service` | 8085 | no |
| payment-service (payments, refunds, provider webhooks) | `/app/bin/payment-service` | 8083 | no |
| **api-gateway** (SPA + `/api/v1/*` routing) | `/app/bin/api-gateway` | 8080 | **yes (only this one)** |

One image carries every binary, the SQL migrations and the built web app (`deployments/docker/Dockerfile`). `deployments/docker/docker-compose.app.yml` wires the stack together, with the gateway as the only published port. Put TLS termination (a reverse proxy or load balancer) in front of the gateway: HTTPS only, HSTS on, and have it overwrite `X-Forwarded-For`.

## 2. Build

```sh
docker build -f deployments/docker/Dockerfile \
  --build-arg VITE_WHATSAPP_NUMBER=<public WhatsApp Business number, digits only, optional> \
  --build-arg VITE_PLAY_STORE_URL=<public Play Store page of the mobile app (https), only once it is published> \
  --build-arg VITE_APP_STORE_URL=<public App Store page of the mobile app (https), only once it is published> \
  -t trezutil:<version> .
```

The web build type-checks and bundles the SPA. The API base URL defaults to `/api/v1` on the same origin, so no hostname is baked into the bundle.

Leave `VITE_CHANNEL_PLACEHOLDERS` unset in production: it only shows placeholders for channels that are not connected yet (staging), and a production build must mention only the WhatsApp number and the store pages that really exist (see `docs/web/OUR_COMPANY.md`).

## 3. Configuration

Copy `deployments/docker/app.env.example` to `app.env` (never commit it) and fill in the values below.

| Variable | Service(s) | Required | Notes |
|---|---|---|---|
| `APP_ENV` | all | yes | `production`. `development` logs OTP codes and must never be used in production |
| `POSTGRES_DB`, `POSTGRES_USER`, `POSTGRES_PASSWORD` | postgres, all | yes | the compose file builds `DATABASE_URL` from these |
| `JWT_SECRET` | all | yes | 32+ random characters (`openssl rand -base64 48`); compose refuses to start without it |
| `OTP_CODE_SECRET` | identity | recommended | separate key for OTP hashing |
| `JWT_ACCESS_TTL`, `JWT_REFRESH_TTL` | all | no | defaults 15 min / 30 days |
| `CORS_ALLOWED_ORIGINS` | gateway | yes | exact public origin(s); never `*` |
| `WHATSAPP_API_BASE_URL`, `WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`, `WHATSAPP_OTP_TEMPLATE`, `WHATSAPP_OTP_TEMPLATE_LANG` | identity, conversation | **yes for sign-in** | outside `development`, identity-service will not start without an OTP delivery channel |
| `STRIPE_SECRET_KEY` | payment | for card payments | referenced by the provider row (`secret_ref = env:STRIPE_SECRET_KEY`) |
| `STRIPE_WEBHOOK_SECRET` | payment | for card payments | Stripe endpoint signing secret (`whsec_…`), referenced by `webhook_secret_ref` |
| `PAYMENT_PROVIDER_REFRESH_SECONDS` | payment | no | how often provider rows are reloaded (default 60) |
| `AI_PROVIDER` | conversation | no | empty = deterministic keyword routing; `anthropic` = model-assisted interpretation |
| `ANTHROPIC_API_KEY`, `AI_MODEL`, `AI_EFFORT`, `AI_TIMEOUT_SECONDS`, `AI_REQUEST_TIMEOUT_SECONDS`, `AI_MAX_RETRIES` | conversation | with `AI_PROVIDER=anthropic` | defaults: `claude-opus-5`, `low`, 30 s, 15 s, 2 |
| `OPENSEARCH_URL`, `SEARCH_SYNC_INTERVAL` | search, conversation | set by compose | projection sync interval (default 5 s) |
| `OTP_MAX_PER_IP_10MIN`, `OTP_MAX_PER_PHONE_HOUR` | identity | no | defaults 20 and 5 |

**Secrets.** Provider credentials never live in the database. Provider rows hold only `env:NAME` references, which the service resolves from its environment. Inject secrets from the platform's secret store (Docker/Kubernetes secrets, a vault) rather than files on disk where possible, and rotate them by updating the variable and restarting the affected service.

## 4. First start

```sh
docker compose --env-file app.env -f deployments/docker/docker-compose.app.yml up -d
```

The startup order is enforced by the compose dependencies:
1. PostgreSQL and OpenSearch become healthy.
2. `migrate` applies migrations `000001`–`000021` (tracked, idempotent; re-running is a no-op). Migration `000022` (dispatch tables for food delivery and rides) is on hold and not part of the production sequence. 000020 adds the Hotel pack (`hotel_room_types`, capacity-managed reservations, non-overlapping capacity windows) and 000021 the provider execution audit fields. 000019 adds `listings.vertical` and backfills existing listings from their listing type; services built before it keep working during a rolling deploy.
3. The services start, then the gateway.

**Health:**
- Each service exposes `GET /healthz` (liveness) and `GET /readyz` (database; search-service also checks OpenSearch).
- The gateway's `GET /readyz` aggregates every upstream. Point the load balancer at it.

**First operator:** sign in once on the web app with the operator's phone, then run:

```sh
docker compose run --rm migrate /app/bin/trezutil-admin grant-role -phone +243XXXXXXXXX -role operator
```

## 5. Onboarding a seller (operator-assisted)

Onboarding is a platform rule: only operators create organizations.

1. The seller signs in once with their phone.
2. The operator calls `POST /api/v1/organizations` and then `POST /api/v1/organizations/members?organization_id=<org>` with `{"user_id": "<seller user id>", "membership_role": "owner"}`, and creates the categories the seller needs (`POST /api/v1/categories`).
3. The seller (owner) creates their seller profile (`POST /api/v1/sellers`) and store (`POST /api/v1/stores`).
4. The seller workspace (`/seller`) then opens automatically: listings, orders, bookings, inventory.

## 6. Card payments (Stripe) — per organization

Integration code is complete and verified against a Stripe emulator (see TESTS in MVP_FINAL_CERTIFICATION.md). **Live payments need the merchant's Stripe account.**

1. Put the secret key and the endpoint signing secret into `STRIPE_SECRET_KEY` and `STRIPE_WEBHOOK_SECRET` for payment-service.
2. Insert the provider row for the organization (values other than the org are fixed):

   ```sql
   INSERT INTO providers (organization_id, code, name, type, status, capabilities, configuration, secret_ref, health_status)
   VALUES ('<organization uuid>', 'stripe', 'Stripe', 'PAYMENT', 'ACTIVE', '{PAYMENT}',
           '{"settings":{"publishable_key":"pk_live_…","capture_method":"automatic","webhook_secret_ref":"env:STRIPE_WEBHOOK_SECRET"}}',
           'env:STRIPE_SECRET_KEY', 'HEALTHY');
   ```

   `capture_method` is `automatic` (charge at confirmation) or `manual` (authorize, then capture from the API). The publishable key is public and is returned to the paying customer for Stripe.js.
3. In the Stripe dashboard, add the webhook endpoint `https://<public host>/api/v1/webhooks/payments/stripe?organization_id=<organization uuid>` with these events: `payment_intent.succeeded`, `payment_intent.amount_capturable_updated`, `payment_intent.payment_failed`, `payment_intent.canceled`, `refund.created`, `refund.updated`, `charge.refund.updated`.
4. Within `PAYMENT_PROVIDER_REFRESH_SECONDS` the organization's orders show **Pay securely** (Stripe Payment Element). Make one small live payment and refund it before opening to customers.

Card data is entered in Stripe's iframe and never reaches Trezutil. Trezutil stores no PAN, CVV or PIN. The order moves to *paid* only when the signed webhook (or the provider's synchronous answer) confirms it.

Several merchants on one deployment: give each organization its own variables (e.g. `STRIPE_SECRET_KEY_ORG2`) and reference them from that organization's row.

## 7. WhatsApp

1. Create the WhatsApp Business Account and phone number, and get an **authentication** message template approved for sign-in codes.
2. Set the `WHATSAPP_*` variables.
3. Register the webhook `https://<public host>/api/v1/webhooks/whatsapp` with the verify token, and subscribe to `messages`.
4. Insert the channel mapping so incoming messages reach the right organization: `INSERT INTO channel_accounts (organization_id, channel, external_id, status) VALUES ('<org>', 'whatsapp', '<WABA or phone number id>', 'active');`
5. Verify: a sign-in code arrives on WhatsApp, and a message to the number gets a search reply.

Status: code complete and verified with signed test webhooks and an HTTP test server. **EXTERNAL CONFIGURATION REQUIRED** for live delivery.

## 8. AI interpretation

Set `AI_PROVIDER=anthropic` and `ANTHROPIC_API_KEY` for conversation-service. The model only interprets messages into allow-listed, read-only intents. Identity, tenant, confirmation and authorization stay server-side, and orders and bookings always need the user's explicit confirmation. If the provider is unreachable, slow or unsure, the conversation falls back to deterministic keyword routing. Calls are metered when an `AI_INTERPRETATION` usage policy exists.

## 9. Operations

- **Backups:** daily `pg_dump` (or managed PITR) of the PostgreSQL database is the only backup that matters. OpenSearch is a rebuildable projection: search-service resynchronizes it hourly and on start, so after data loss, delete the index and restart search-service.
- **Logs:** JSON on stdout (zerolog). Every request line carries `request_id` (the gateway's `X-Request-ID`, also returned to the client and present in the service logs), status, duration and, for a refused or failed request, `error_code`; the level follows the status (5xx `error`, 4xx `warn`). Critical failures are structured events: `payment_failed`, `reservation_failed`, `webhook_failed`, `provider execution` (with retry class) and `panic`. Critical commerce actions (order, reservation, listing, inventory, capacity and fulfilment changes) write an `audit` line with actor, action, target and request id; it is a log trail, not a tamper-proof store. Ship logs to the platform's log store. No OTP codes are logged outside `development`.
- **Metrics:** every service serves Prometheus text on `GET /metrics`: `trezutil_http_requests_total`, `trezutil_http_errors_total`, `trezutil_http_request_duration_seconds`, `trezutil_api_errors_total{code}`, `trezutil_payment_failures_total`, `trezutil_reservation_failures_total`, `trezutil_provider_failures_total` and `trezutil_webhook_failures_total`. Labels are bounded (service, method, normalized route, status, error code). Services answer only loopback and private-network clients; the internet-facing gateway answers only when `METRICS_TOKEN` is set and sent as a bearer token. `infrastructure/prometheus/prometheus.yml` lists the scrape targets. Probes: `/healthz` (liveness) and `/readyz` (database and upstreams) on the gateway and every service.
- **Scaling:**
  - Services are stateless except the in-process OTP per-IP limiter and the chat action idempotency cache (durable idempotency is in PostgreSQL). Add an edge rate limiter when running several identity-service replicas.
  - Keep one payment-service replica per provider set, or several: webhook processing is idempotent and money moves under per-payment advisory locks.
- **Upgrades:** build a new image, run `migrate` (forward-only, idempotent), then roll services. Roll back by redeploying the previous image; migrations are additive.
- **Monitoring:** alert on the gateway `/readyz`, the 5xx rate (`trezutil_http_errors_total{class="5xx"}`), `trezutil_webhook_failures_total`, `trezutil_payment_failures_total`, `trezutil_provider_failures_total`, request latency, payment webhook rows without processing (`payment_webhook_events`), and OpenSearch health.

## 9a. Hardening (pass 1)

`HARDENING_PASS_1.md` holds the AI configuration reference, the configuration matrix (what is required, what has no default), the alert table with a first action for each alert, the security findings and the staging verification record. Behind a TLS proxy set `TRUSTED_PROXY_HOPS=1`. To run Prometheus with the alert rules, add `-f deployments/docker/docker-compose.monitoring.yml` (needs `METRICS_TOKEN`).

## 10. Pre-launch checklist

- [ ] `APP_ENV=production`, strong `JWT_SECRET`, `CORS_ALLOWED_ORIGINS` set to the real origin
- [ ] TLS in front of the gateway; only the gateway reachable from the internet
- [ ] WhatsApp credentials and OTP template; a sign-in code received on a real phone
- [ ] First operator granted; the first seller onboarded; categories created
- [ ] Stripe keys, provider row and webhook endpoint for each selling organization; one live payment and refund
- [ ] Optional: `AI_PROVIDER=anthropic` with a key; spot-check conversations
- [ ] Backups scheduled and one restore rehearsed
