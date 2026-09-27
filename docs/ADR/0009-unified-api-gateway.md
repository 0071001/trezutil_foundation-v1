# ADR 0009 — Unified External API Gateway

## Context
External traffic enters Trezutil through a single API boundary. Internal
services (catalog, payment, fileserver, AI) must remain independently
deployable while presenting a coherent, versioned external surface.

## Decision
The unified API gateway exposes a single external boundary at `/api/v1/*`.
The gateway serves the SPA web bundle directly and reverse-proxies API
routes to the backing internal service. Internal services keep their own
internal `/v1/*` route space; the gateway rewrites `/api/v1/*` to `/v1/*`
on the relevant upstream.

## Route map
External route (gateway) -> Upstream path -> Service:

- `/api/v1/health`            -> `/health`                 -> catalog-service (shared)
- `/api/v1/listings/**`       -> `/v1/listings/**`         -> catalog-service
- `/api/v1/orders/**`         -> `/v1/orders/**`           -> order-service
- `/api/v1/reservations/**`   -> `/v1/reservations/**`     -> order-service
- `/api/v1/availability`      -> `/v1/availability`        -> order-service
- `/api/v1/search/**`         -> `/v1/search/**`           -> search-service
- `/api/v1/packs/**`          -> `/v1/packs/**`           -> pack/catalog-service
- `/api/v1/payments/**`       -> `/v1/payments/**`         -> payment-service
- `/api/v1/webhooks/**`       -> `/v1/webhooks/**`         -> payment-service
- `/api/v1/auth/**`           -> `/v1/auth/**`             -> auth-service
- `/api/v1/users/**`          -> `/v1/users/**`           -> auth-service
- `/api/v1/organizations/**`  -> `/v1/organizations/**`    -> org-service
- `/api/v1/sellers/**`        -> `/v1/sellers/**`         -> seller-service
- `/api/v1/stores/**`         -> `/v1/stores/**`          -> seller-service
- `/api/v1/internal/...`      -> passthrough             -> reserved (see below)

SPA fallback: any route not matching an API prefix returns the SPA entry
point (`index.html`) from the fileserver so client-side routing resolves.

## Internal passthrough
`/internal/api/v1/*` endpoints require service-to-service authentication
(`X-Internal-Service-Token`) and are not exposed on the public boundary.
They are reachable only between co-located services (e.g. AI interpretation).

## Environment variables
| Variable                | Default   | Purpose                          |
| ----------------------- | --------- | -------------------------------- |
| `GATEWAY_HTTP_PORT`     | `8080`    | Public gateway listen port.      |
| `CATALOG_HTTP_PORT`     | `8082`    | catalog-service listen port.     |
| `PAYMENT_HTTP_PORT`     | `8083`    | payment-service listen port.     |
| `FILESERVER_HTTP_PORT`  | `8084`    | fileserver listen port.          |
| `WEB_DIR`               | `./web`    | Directory served as SPA fallback.|
| `INTERNAL_SERVICE_TOKEN`| `unset`   | Shared token for /internal/* use.|

## CORS
CORS is configured with an explicit allow-list of origins (no wildcard).
Allowed origins are read from `CORS_ALLOWED_ORIGINS` (comma-separated).
Credentials are supported; `Access-Control-Allow-Origin` reflects a single
requesting origin that appears on the allow-list.

## Port topology
| Service             | Port  | Notes                          |
| ------------------- | ----- | ------------------------------ |
| gateway             | 8080  | Public entry; SPA + proxy.     |
| catalog-service     | 8082  | Listings, availability.        |
| payment-service     | 8083  | Payments, webhooks.            |
| fileserver          | 8084  | SPA static serving.            |
| auth-service        | 8090  | Auth, users, tokens.           |
| order-service       | 8091  | Orders, reservations.          |
| search-service      | 8092  | Listing search.                |
| ai-service          | 8093  | AI interpretation gateway.      |

## Consequences
- Single external boundary simplifies client configuration and TLS termination.
- Internal services retain independent ports; collision-free local development.
- `/internal/api/v1/*` stays off the public internet but reachable in-cluster.
- Route rewrites are centralized, easing future path migrations.
