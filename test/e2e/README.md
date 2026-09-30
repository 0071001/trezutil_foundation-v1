# Real-stack E2E journeys

Two suites drive the public gateway (`/api/v1/*` and the web app it serves)
against **real PostgreSQL and real OpenSearch**:

- `journeys.py` — API journeys: sign-in, seller onboarding, E (seller →
  publish → searchable), A (discovery), B (conversation), C (booking,
  8-way concurrency race), D (order up to the payment boundary with no
  provider), G (WhatsApp), the **seller console isolation** checks, **S**
  (Stripe payments with signed webhooks, confirm, refunds), **AI**
  (model-interpreted conversation → contextual reference → order with
  explicit confirmation), the **Hotel Commerce Pack** and security probes:
  - **H1** discovery: vertical + location search, and the AI intent "a hotel in Gombe for two nights for two people";
  - **H2** availability: property → dates → guests → rooms left;
  - **H3** reservation: stays sharing nights, full night, occupancy, whole nights, stay order, Stripe payment, calendar, cancellation;
  - **H4** seller: room type → rooms → nightly capacity → publish, and conflicts;
  - **H5** isolation between two hotels, customers and non-hotel IDs.

  **H6** (external availability: timeout, unavailable, malformed, recorded attempts) runs in Go on real PostgreSQL with deterministic emulators: `infrastructure/postgres/integration/provider_orchestration_test.go`.
- `web/journeys.spec.mjs` — browser journeys A–J through the React app at
  desktop and mobile sizes (Playwright): discovery, phone sign-in and
  session expiry, conversational commerce, booking, order/checkout,
  account, seller workspace, failure handling, **I** hotel stay (search →
  hotel → dates/guests → availability → room → reserve → pay) and **J**
  hotel seller nights.

What is *not* live: OTP codes are read from the identity-service log
(`APP_ENV=development`); WhatsApp webhooks are signed locally with the test
app secret and replies are not delivered; **Stripe and the AI model are
emulated** by `fakeproviders` (a Stripe PaymentIntents/refunds emulator and an
Anthropic Messages API emulator). The emulators exercise the real adapters,
HTTP, signatures, idempotency keys, timeouts and error mapping; they are test
tools and must never be deployed.

## Run

1. Start PostgreSQL and OpenSearch (e.g. `docker compose -f deployments/docker/docker-compose.yml up -d postgres opensearch`).
2. Create an empty database and migrate it:
   `DATABASE_URL=postgres://trezutil:trezutil@localhost:5432/trezutil_e2e?sslmode=disable go run ./cmd/migrate`
3. Build the web app: `cd web/frontend && pnpm install && pnpm run build`.
4. Start the emulators and the services with a shared environment:

   ```sh
   export APP_ENV=development JWT_SECRET=e2e-secret-0123456789abcdef0123456789 \
          DATABASE_URL=postgres://trezutil:trezutil@localhost:5432/trezutil_e2e?sslmode=disable \
          OPENSEARCH_URL=http://localhost:9200 SEARCH_SYNC_INTERVAL=1s WEB_ROOT=./web/frontend/dist \
          WHATSAPP_APP_SECRET=wa-e2e-secret WHATSAPP_VERIFY_TOKEN=wa-verify \
          OTP_MAX_PER_IP_10MIN=500 OTP_MAX_PER_PHONE_HOUR=50 \
          STRIPE_E2E_SECRET=sk_test_e2e STRIPE_E2E_WEBHOOK=whsec_e2e PAYMENT_PROVIDER_REFRESH_SECONDS=2
   go run ./test/e2e/fakeproviders &            # Stripe :12111, Anthropic :12112
   HTTP_PORT=8086 go run ./cmd/identity-service > identity.log 2>&1 &
   HTTP_PORT=8081 go run ./cmd/search-service &
   HTTP_PORT=8091 AI_PROVIDER=anthropic ANTHROPIC_API_KEY=sk-ant-e2e AI_BASE_URL=http://127.0.0.1:12112 \
     go run ./cmd/conversation-service > conversation.log 2>&1 &
   HTTP_PORT=8085 go run ./cmd/order-service &
   HTTP_PORT=8083 go run ./cmd/payment-service &
   GATEWAY_PORT=:8080 go run ./cmd/api-gateway &
   ```

   `conversation.log` must sit next to `identity.log` (the AI journey checks
   that the model was called).
5. API journeys: `E2E_IDENTITY_LOG=identity.log python test/e2e/journeys.py`
6. Browser journeys (after step 5, which seeds the seller, listing and Stripe row):

   ```sh
   cd test/e2e/web && pnpm install && npx playwright install chromium
   E2E_IDENTITY_LOG=../../../identity.log npx playwright test
   ```

   `node capture.mjs <figma-origin> <out-dir>` captures each Figma screen next
   to its implementation at mobile, tablet and desktop sizes for visual QA
   (serve the unmodified export with `vite preview`).

Environment variables: `E2E_GATEWAY` / `E2E_APP` (default `http://localhost:8080`),
`E2E_IDENTITY_LOG` (required), `E2E_PSQL`, `E2E_DB`, `E2E_DB_USER`,
`E2E_DB_PASSWORD`, `E2E_WHATSAPP_APP_SECRET`, `E2E_WHATSAPP_VERIFY_TOKEN`.

The API script seeds one operator-provisioned seller organization per run
through `psql` (there is no self-serve seller onboarding API in the MVP) and
uses per-run idempotency keys, so both suites can be re-run against the same
database.
