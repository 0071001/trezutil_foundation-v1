# Sprint 19 — Payment Core

Provider-agnostic payment orchestration with idempotent state management,
exactly-once ledger accounting, and secure webhook reconciliation.

## Architecture

```
Client/API → CommerceActionRuntime → PaymentService → ProviderOrchestrator → PaymentProviderAdapter
                              ↓
                        PaymentRepository (PostgreSQL)
                              ↓
                        LedgerRepository (PostgreSQL — exactly-once)
                              ↓
                        PaymentWebhookEventStore (PostgreSQL — idempotency)
```

### Key Invariants

1. **Domain is authoritative**: All payment state mutations flow through
   `Payment` / `Refund` / `LedgerEntry` aggregates. No direct SQL updates.
2. **AI never authorizes payment**: All payment mutations require human
   confirmation via `ConfirmationPolicy` and authenticated actor context.
3. **PostgreSQL is authoritative for transactional state**: Payment, refund,
   and webhook event state is persisted in PostgreSQL with row-level locks
   (`FOR UPDATE`) for concurrency safety.
4. **Ledger is authoritative for accounting**: The ledger uses
   `INSERT ... ON CONFLICT DO NOTHING` on a per-tenant idempotency key to
   guarantee exactly-once posting, even under retries or duplicate webhooks.
5. **External providers are invoked only through ProviderOrchestration**:
   The `ProviderOrchestrator` handles retry/backoff, health checks, and
   error classification. No direct provider API calls outside this boundary.
6. **Tenant isolation**: Every repository query is scoped by `organization_id`.

## Domain Model

### Payment Aggregate

States: `created` → `pending` → `authorized` → `captured`
                                           → `failed`
                                           → `cancelled`
                                           → `expired`
`captured` → `refunded` / `partially_refunded` → `failed`
`pending` → `expired`
`created` → `failed` / `cancelled`

Terminal states: `failed`, `cancelled`, `expired`, `refunded`
(Non-terminal: `created`, `pending`, `authorized`, `captured`,
 `partially_refunded` — these allow further transitions.)

### Refund Aggregate

`pending` → `succeeded` / `failed` / `cancelled`
Total refunded amount is tracked on the Payment aggregate and enforced
to never exceed the captured amount.

### LedgerEntry Aggregate

`pending` → `posted` / `failed`
Posting is exactly-once via database-level idempotency keys.

## Provider Integration

The `PaymentProviderAdapter` interface defines five operations:
- `CreatePayment` — initiate payment
- `GetPayment` — check status
- `CancelPayment` — void before capture
- `CapturePayment` — capture an authorization
- `RefundPayment` — refund (full or partial)

Providers register with `CapabilityPayment` and are selected via
`ProviderOrchestrator.SelectProvider` / `ExecuteCapability`.

The orchestrator passes `domain.ProviderAdapter` to the perform callback.
The `PaymentService` type-asserts to `PaymentProviderAdapter` — providers
that don't implement payment operations are rejected at runtime.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/payments` | Create a payment |
| GET | `/api/v1/payments/{payment_id}` | Get payment details |
| POST | `/api/v1/payments/{payment_id}/cancel` | Cancel a payment |
| POST | `/api/v1/payments/{payment_id}/capture` | Capture a payment |
| POST | `/api/v1/payments/{payment_id}/refund` | Refund a payment |
| GET | `/api/v1/payments/{payment_id}/status` | Get/refund payment status |
| GET | `/api/v1/payments/{payment_id}/refunds/{refund_id}` | Get refund details |
| POST | `/api/v1/webhooks/payments/{provider_code}` | Provider webhook callback |

All payment operations require `organization_id` and `user_id` query
parameters. Financial actions also require an `Idempotency-Key` header.

## Security

- Webhook signatures are verified via the `WebhookSignatureVerifier` interface
  in `internal/infrastructure/payment/webhook_verifier.go`. The concrete HMAC-SHA256
  implementation uses provider-specific secrets keyed by `providerCode`, preventing
  signature spoofing. Provider-specific logic is confined to infrastructure; the
  domain/application core depends only on the interface.
- Idempotency keys prevent duplicate processing for both payments and
  refunds.
- The `payment_webhook_events` table enforces deduplication via
  `UNIQUE(provider_code, provider_msg_id)` and the `TryClaim` pattern
  (`UPDATE ... SET status='processing' WHERE status='pending'`).

## Composition Root

The `cmd/payment-service/main.go` wires the full payment subsystem:

- **Database**: PostgreSQL with connection pool (25 max open, 5 idle)
- **Repositories**: `PaymentRepository`, `RefundRepository`, `LedgerRepository`,
  `PaymentWebhookEventStore`, `ProviderRepository`, `ProviderExecutionRepository`
- **Provider layer**: `ProviderRegistry` + `ProviderOrchestrator` (retry/backoff,
  health-aware selection)
- **Application layer**: `PaymentService` (orchestrates domain + provider calls),
  `WebhookHandler` (signature verification + reconciliation)
- **HTTP layer**: `PaymentHandler` with 8 routes (7 payment endpoints + 1 webhook)
- **Webhook secrets**: Configured via `STRIPE_WEBHOOK_SECRET` / `PAYPAL_WEBHOOK_SECRET`
  environment variables, keyed by provider code

Provider adapters register at startup via `ProviderRegistry.Register()`. Providers
must implement `PaymentProviderAdapter` (extending `ProviderAdapter` with payment
operations) and declare `CapabilityPayment`.

## Testing

- **Domain tests**: 31 unit tests covering state machine transitions,
  refund limits, terminal states, and invariants.
- **Service tests**: 11 tests covering payment creation, capture,
  cancel, refund, idempotency, and error paths with mock provider.
- **Webhook handler tests**: 5 tests covering reconciliation, signature
  verification, idempotency, and unknown event types.
- **Webhook verifier tests**: 6 tests covering valid/invalid signatures,
  provider-specific secrets, empty configurations, and tampered payloads.
- **Domain aggregate tests**: 31 (payment) + 6 (refund) + 5 (ledger) + 3 (payment method) = 45

Run: `go test ./internal/... ./infrastructure/postgres/...`
