# Trezutil — Known Limitations (MVP)

Only genuine blockers and deliberate MVP boundaries are listed. Everything else found during the audit was fixed (see PLATFORM_AUDIT.md).

## Launch blockers (external)

| # | Limitation | Why it remains | Impact | Exact next action |
|---|---|---|---|---|
| B1 | **No payment provider adapter.** `payment-service` registers none; `POST /api/v1/payments` returns `503 PAYMENT_PROVIDER_UNAVAILABLE`. | No provider integration exists in the codebase and adding providers was out of scope; needs a commercial account and credentials. | Customers can order and book, but cannot pay online. Capture → ledger → order PAID is proven only with a test adapter. | Implement `domain.PaymentProviderAdapter` for the chosen mobile-money/card provider, register it in `cmd/payment-service/main.go` (`providerRegistry.Register`), insert its `providers` row, set its webhook secret, run `TestPaymentIntegration_*` against the sandbox. |
| B2 | **WhatsApp Cloud API credentials** (`WHATSAPP_PHONE_NUMBER_ID`, `WHATSAPP_ACCESS_TOKEN`, `WHATSAPP_APP_SECRET`, `WHATSAPP_VERIFY_TOKEN`) and an **approved "authentication" template** (`WHATSAPP_OTP_TEMPLATE`). | Requires a Meta Business account. | Outside development, identity-service refuses to start without an OTP delivery channel (by design), so nobody can sign in; WhatsApp replies are processed but not delivered. Validated only with signed test webhooks and an HTTP test server. | Create the WABA, approve the template, set the variables, register the webhook URL `https://<host>/api/v1/webhooks/whatsapp`, insert a `channel_accounts` row mapping the WABA id to the organization, send one live message. |

## Deliberate MVP boundaries

| # | Limitation | Impact | Next action |
|---|---|---|---|
| L1 | No production AI model adapter (only a test mock). Intent classification uses deterministic keyword routing; setting `AI_PROVIDER` only logs a warning. | Conversations understand search/booking/order phrasing via keywords, not free-form language. | Implement `domain.AIProvider` for the chosen model, register it on the `ModelRouter`, re-enable `ai.NewIntentClassifierAdapter` in `buildIntentClassifier`. |
| L2 | Seller onboarding is **operator-assisted** (existing IAM rule: only platform operators create organizations). No self-serve seller signup or web console for listing creation/order queue/booking calendar. | Sellers are onboarded by the Trezutil team; listings are created through `POST /api/v1/listings`. | Build the seller console on the existing APIs; decide whether self-serve organization creation is allowed (business decision). |
| L3 | Post-payment order lifecycle (processing → ready → fulfilled → completed) has use cases but no customer/seller HTTP endpoints or UI beyond `confirm`. | Orders stop at PAID/CONFIRMED in the product. | Expose `ReadyForFulfillment`/`CompleteOrder` to org staff alongside fulfillment transitions. |
| L4 | Listings without inventory rows are single bookable units (one reservation per period, enforced by the DB exclusion constraint). Multi-unit stock needs inventory/capacity rows; there is no API to create them. | A hotel with 10 identical rooms must be listed as separate listings or provisioned via SQL. | Add staff endpoints to create inventory/capacity units. |
| L5 | Chat cannot create listings or take payments (`CREATE_LISTING` needs a structured payload; payment actions need B1). | The assistant answers that these must be done in the app. | Register payment handlers once B1 is done. |
| L6 | Per-organization vertical-pack activation is not persisted (all packs enabled); mutations are operator-only no-ops. | None for MVP routing. | Implement the org-pack repository on migration 000013 tables. |

## Operational notes (non-blocking)

- **Token revocation window**: suspending a user blocks refresh and all use cases that load the user, but an already-issued access token remains valid for up to `JWT_ACCESS_TTL` (15 min).
- **Rate limiting** of OTP requests per IP is in-process; with several identity-service replicas the effective limit multiplies (the per-phone limit is in PostgreSQL and global). Add an edge limiter when scaling out.
- **Client IP** for rate limiting is the last `X-Forwarded-For` hop (the gateway's peer). Behind an additional proxy, configure that proxy to overwrite `X-Forwarded-For`.
- **TLS** is expected to terminate at a reverse proxy in front of the gateway; OpenSearch runs without its security plugin on the private network only.
- **Runtime idempotency cache** for chat actions is per process; durable idempotency is enforced in PostgreSQL for orders, reservations, payments, messages and credit reservations.
- **Search freshness**: listing status changes are indexed within `SEARCH_SYNC_INTERVAL` (5 s); edits to media/locations/attributes that do not touch the listing row appear after the hourly full resync.
- **Usage policies** match by action type only (not channel or actor type); none are seeded, so metering is off until an operator adds `usage_policies` rows.
- **Asynchronous provider refunds**: a refund reported as pending by the provider is recorded as failed; a later "succeeded" webhook does not update the payment's refunded amount. Revisit with the first real provider (B1).
- **Docker image** was validated statically (`docker compose config`); Docker was not available on the audit workstation, so build the image once in CI or on the target host.

## Cleanup candidates (P3)

Legacy vanilla web app (`web/js`, `web/index.html`, `cmd/fileserver`), `HomepageDesign/` prototype, placeholder binaries (`admin`, `logistics`, `marketing`, `notification`, `seller`, `trust` services), `catalog-service` (duplicate of search-service listing routes), and PascalCase JSON of domain aggregates (add JSON tags when the API contract is versioned).
