# Real-stack E2E journeys

`journeys.py` drives the public gateway (`/api/v1/*`) the way the web app and
WhatsApp do, against **real PostgreSQL and real OpenSearch**. It covers sign-in,
journeys A (discovery), B (conversation), C (booking), D (commerce up to the
payment-provider boundary), E (seller → publish → searchable), G (WhatsApp),
concurrency (one slot, eight bookers) and security probes.

What is *not* live: OTP codes are read from the identity-service log
(`APP_ENV=development`), WhatsApp webhooks are signed locally with the test app
secret and replies are not delivered (no Cloud API credentials), and no payment
provider is registered, so payment creation must return `503
PAYMENT_PROVIDER_UNAVAILABLE`.

## Run

1. Start PostgreSQL and OpenSearch (e.g. `docker compose -f deployments/docker/docker-compose.yml up -d postgres opensearch`).
2. Create an empty database and migrate it:
   `DATABASE_URL=postgres://trezutil:trezutil@localhost:5432/trezutil_e2e?sslmode=disable go run ./cmd/migrate`
3. Start the services with a shared environment:

   ```sh
   export APP_ENV=development JWT_SECRET=e2e-secret-0123456789abcdef0123456789 \
          DATABASE_URL=postgres://trezutil:trezutil@localhost:5432/trezutil_e2e?sslmode=disable \
          OPENSEARCH_URL=http://localhost:9200 SEARCH_SYNC_INTERVAL=1s \
          WHATSAPP_APP_SECRET=wa-e2e-secret WHATSAPP_VERIFY_TOKEN=wa-verify \
          OTP_MAX_PER_IP_10MIN=500 OTP_MAX_PER_PHONE_HOUR=50
   HTTP_PORT=8086 go run ./cmd/identity-service > identity.log 2>&1 &
   HTTP_PORT=8081 go run ./cmd/search-service &
   HTTP_PORT=8091 go run ./cmd/conversation-service &
   HTTP_PORT=8085 go run ./cmd/order-service &
   HTTP_PORT=8083 go run ./cmd/payment-service &
   GATEWAY_PORT=:8080 go run ./cmd/api-gateway &
   ```

4. Run: `E2E_IDENTITY_LOG=identity.log python test/e2e/journeys.py`

Environment variables: `E2E_GATEWAY` (default `http://localhost:8080`),
`E2E_IDENTITY_LOG` (required), `E2E_PSQL`, `E2E_DB`, `E2E_DB_USER`,
`E2E_DB_PASSWORD`, `E2E_WHATSAPP_APP_SECRET`, `E2E_WHATSAPP_VERIFY_TOKEN`.

The script seeds one operator-provisioned seller organization per run through
`psql` (there is no self-serve seller onboarding API in the MVP) and uses
per-run idempotency keys, so it can be re-run against the same database.
