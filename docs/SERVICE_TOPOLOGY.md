# Service Topology

Reference for local development of the Trezutil service mesh.

## Local dev topology

| Service             | Port  | HTTP_PORT env   | Startup command                       |
| ------------------- | ----- | ---------------- | ------------------------------------- |
| gateway             | 8080  | `GATEWAY_HTTP_PORT` | `go run ./cmd/gateway`            |
| catalog-service     | 8082  | `HTTP_PORT`       | `go run ./cmd/catalog-service`      |
| payment-service     | 8083  | `HTTP_PORT`       | `go run ./cmd/payment-service`      |
| fileserver          | 8084  | `HTTP_PORT`       | `go run ./cmd/fileserver`           |
| auth-service        | 8090  | `HTTP_PORT`       | `go run ./cmd/auth-service`         |
| order-service       | 8091  | `HTTP_PORT`       | `go run ./cmd/order-service`        |
| search-service      | 8092  | `HTTP_PORT`       | `go run ./cmd/search-service`       |
| ai-service          | 8093  | `HTTP_PORT`       | `go run ./cmd/ai-service`           |

## Shared dependencies

| Dependency         | Default URL / value                                              | Env var             |
| ------------------ | --------------------------------------------------------------- | ------------------- |
| PostgreSQL         | `postgres://trezutil:trezutil@localhost:5432/trezutil?sslmode=disable` | `DATABASE_URL`  |
| Stripe webhook     | (secret)                                                        | `STRIPE_WEBHOOK_SECRET` |
| PayPal webhook     | (secret)                                                        | `PAYPAL_WEBHOOK_SECRET` |
| Internal token     | (unset)                                                         | `INTERNAL_SERVICE_TOKEN` |
| CORS origins       | (none)                                                          | `CORS_ALLOWED_ORIGINS`   |

## External API boundary
The gateway exposes `/api/v1/*` externally and proxies to internal services
using their `/v1/*` route space. SPA routes fall through to the fileserver
static fallback. See `docs/ADR/0009-unified-api-gateway.md` for details.

## Unavailable endpoints (require external services)
- `/api/v1/payments/**` — requires a configured payment provider (Stripe / PayPal).
- `/api/v1/webhooks/payments/{provider_code}` — requires provider webhook delivery.
- `/api/v1/internal/api/v1/ai/interpret` — requires in-cluster `INTERNAL_SERVICE_TOKEN`.
- `/api/v1/search/**` — requires OpenSearch cluster (see `docs/search/`).
- `/api/v1/packs/**` — pack registry is populated by migration scripts.
