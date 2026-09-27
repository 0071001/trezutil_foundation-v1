# Sprint 20 — Monetization, Plans & Credit Entitlements

Status: **Design phase** (Sprint 19 retro declared GO; Sprint 20 §2 audit complete)

## 0. Objectives & Invariants

Sprint 20 introduces a monetization model layered on the existing usage/credits system:
per-tenant **Plan catalog** + **Subscription** + **BillingPeriod** + **CreditGrant** +
**CreditPurchase**, with credits purchased via the existing Payment core and consumed
through the existing Usage runtime.

Invariants (must hold at all times):
1. PostgreSQL is authoritative for monetization state.
2. Payment Core owns money movement; Ledger is accounting truth.
3. UsageService is authoritative for credit consumption.
4. CommerceActionRuntime is the enforcement boundary.
5. AI never authorizes, grants, or modifies credits.
6. Client credit balances are never trusted.
7. Tenant + owner separation enforced server-side.
8. No payment-provider SDK in the monetization domain; payment stays provider-agnostic.
9. No duplicate credit system; no second usage counter; credits cannot go negative.
10. All credit mutations are idempotent.

## 1. Existing Usage/Credits Inventory (as audited)

### Domain aggregates (already present)
- `Plan` (`internal/domain/usage.go`) — currently a single global entity. `PlanRepository`
  exposes only `FindActive` (returns one `*Plan`) and `FindByID`. **No per-tenant catalog.**
  Fields: `PlanID`, `Name`, `QuotaPeriod`, `Status` (`PlanStatus`), `PlanID`.
- `CreditAccount` (`internal/domain/credit.go`) — per `(owner_type, owner_id, tenant_id, period_id)`.
  Balance model: `Granted / Reserved / Consumed`, with invariant `Granted = Reserved + Consumed`
  and `Remaining() = Granted - Reserved - Consumed`. Has `CanAfford(cost)`.
- `UsagePolicy` (`internal/domain/usage_policy.go`) — `ActionType / Channel / ActorType / Cost`.
- `UsageEvent` (`internal/domain/credit.go`) — ledger row. `UsageTransactionType`
  = GRANT|RESERVE|CONSUME|RELEASE|PURCHASE|EXPIRE|ADJUST.
- `UsageReservation` (`internal/domain/commerce_usage.go`) — `ReservationID / AccountID /
  ActionType / Amount / ReservedAt / Status`.

### Application service
- `UsageService` (`internal/application/usage.go`) — implements `domain.UsageService`:
  `GetBalance / ResolvePolicy / Check / Reserve / Consume / Release / Grant /
  FindOrCreateCreditAccountByOwner`.

### Infrastructure
- Postgres repos: `CreditAccountRepository`, `UsagePolicyRepository`,
  `UsageLedgerRepository` (`infrastructure/postgres/usage_credits.go`).
  **No `PlanRepository` Postgres implementation.** **No usage_reservations repository.**
- `CommerceActionRuntime` (`internal/application/commerce/runtime.go`) — wires
  UsageService; resolves tenant/owner server-side; gates usage reserve/consume/release
  behind the confirmation state.

### DB schema (current)
- `migrations/000015_usage_credits.sql`: credit_accounts (UUID PK, UNIQUE
  owner_type+owner_id+tenant_id+period_id), usage_policies, usage_ledger (FK→credit_accounts),
  usage_reservations (no idempotency constraint, no tenant_index beyond account FK).

## 2. Audit Findings (bugs to fix in Sprint 20)

F1. **String ID vs UUID mismatch (critical, latent on Postgres).**
`UsageService.FindOrCreateCreditAccountByOwner` builds `AccountID = "acc-"+ownerID+"-"+ts`
(usage.go:309) and `Reserve` builds `reservationID = "res-"+accountID+"-"+ts` (usage.go:96),
but `credit_accounts.account_id` and `usage_reservations.reservation_id` are `UUID`. The
Postgres repos insert these verbatim (`usage_credits.go:20-42`). Ingest via Postgres would
raise `invalid input syntax for type uuid`. The 22-payment integration suite passed only
because it never touches the credits path. Fix: generate UUIDs server-side; store as UUID.

F2. **Reservations are in-process only (critical).**
`Reserve` stores the reservation solely in `UsageService.reservations` (an in-memory map,
usage.go:129-131); `usage_reservations` table is never written. `Consume`/`Release` read
from that map. State is lost on restart and not durable. Fix: persist reservations; add
`UsageReservationRepository`.

F3. **Grant is non-idempotent and non-transactional (high).**
`Grant` (usage.go:243) runs `Create`-like Update + `Record` ledger as two independent
statements with no DB transaction, and dedup key is a timestamp-based event ID — concurrent
or retried grants duplicate the credit and the ledger row. Fix: add `idempotency_key`
unique constraint; wrap in `tx`; ON CONFLICT no-op.

F4. **FindOrCreate has a create-vs-find race + wrong period scope (high).**
`FindOrCreateCreditAccountByOwner` does `FindByOwner` then `Create` (check-then-act race),
and `FindByOwner` filters only on `(owner_type, owner_id, tenant_id)` ignoring `period_id`,
yet the table UNIQUE is on the 4-tuple. With multiple periods it returns the wrong period's
account. Fix: `INSERT ... ON CONFLICT (owner_type,owner_id,tenant_id,period_id) DO NOTHING
RETURNING` upsert; extend `FindByOwner` with `period_id`.

F5. **Reserve ID collision-prone under concurrency (medium).**
Timestamp-based reservation IDs (`fmt.Sprintf("res-%s-%d", accountID, now)`) collide when
two requests hit the same millisecond. Fix: UUIDv7 / UUIDv4.

F6. **Consumption can over-draw on concurrent Reserves (medium).**
Reserve reads-then-updates the balance without a DB-side guard; parallel reserves can both
pass `CanAfford` and exceed the grant. Fix: `UPDATE credit_accounts ... WHERE
reserved+consumed+<new_reserve> <= granted` returning affected rows; retry/abort on 0.

## 3. Sprint 20 Architecture Map

```
┌──────────────────┐   defines    ┌────────────────────┐
│   Plan (tenant)  │◄─┐           │   BillingPeriod    │
└────────┬─────────┘  │ owns      └─────────┬──────────┘
         │            │                     │ contains
         ▼            │                     ▼
┌──────────────────┐  │           ┌────────────────────┐
│  Subscription    │  │ 1..*      │    CreditGrant     │
│  (tenant-scoped) │  │ period    │  idempotent,tx     │
└────────┬─────────┘  │           └─────────┬──────────┘
         │ 1          │                     │ credits
         ▼            │                     ▼
┌──────────────────────────────────────────────────────┐
│                 CreditAccount (existing)             │
│  granted / reserved / consumed   — PostgreSQL truth   │
└──────────────┬───────────────────────────────────────┘
               │ consumption (UsageService.Reserve/Consume)
               ▼
┌──────────────────────────────────────────────────────┐
│ UsagePolicy  →  CommerceActionRuntime (enforcement)  │
└──────────────────────────────────────────────────────┘
               ▲ purchase (Payment-gated Grant)
               │
┌──────────────┴──────────────────────────────────────┐
│  PaymentService (core)  ← CreditPurchaseService    │
│  PaymentProviderAdapter (provider-agnostic)         │
└──────────────────────────────────────────────────────┘
```

Data-flow summary:
- **Subscription** is created from a tenant-scoped **Plan**; each period a **BillingPeriod**
  is opened and **CreditGrant** adds (`Granted += N`) to the tenant's **CreditAccount** for
  that period (monthly allocation, free tier, or paid allocation).
- **Usage** reservation/consumption is unchanged in shape but persisted and made
  transactionally safe (F2/F6).
- **CreditPurchaseService** authorizes a **Payment** for N credits and, on `Captured`,
  **idempotently** `Grant`s credits to the account (F3). Payment↔Credit link recorded in
  `credit_purchases` and mirrored in the ledger as `UsagePurchase`.

## 4. Reusable vs New

| Concept | Decision | Rationale |
|---|---|---|
| `CreditAccount` | **Reuse** (fix F1–F6) | Balance model + ledger + reservation already match Sprint 20 needs. |
| `UsageService` | **Extend** | Add idempotent `GrantWithIdempotency`, persist `Reserve`; keep Reserve/Consume/Release. |
| `UsageReservation` | **Persist** | Add repository; persist on Reserve. |
| `Plan` | **Extend** | Add `TenantID`, `PriceMinorUnits`, `BillingCycle`, `Status`; per-tenant catalog. |
| `Subscription` | **New aggregate** | Lifecycle (active/canceled/past_due) + Plan ref + BillingPeriod ownership. |
| `BillingPeriod` | **New aggregate** | Period window + CreditGrant rollup + close/renew. |
| `CreditGrant` | **New aggregate** | Idempotent grant event w/ source (allocation/purchase/manual), IdempotencyKey, PaymentID. |
| `CreditPurchase` | **New aggregate** | Purchase record (PaymentID→CreditGrant), amount, status. |
| API handlers | **New** | `/plans`, `/subscriptions`, `/billing-periods`, `/credit-purchases`; reuse Payment handler style. |

## 5. Migration 000017_plan

`migrations/000017_monetization.sql` (Up):
- `DROP`/recreate-safe: add `tenant_id UUID` to `credit_accounts` (already present); no
  schema change needed to existing credit tables beyond indexes.
- `plans`: `id UUID PK`, `tenant_id UUID NOT NULL REFERENCES organizations`, `name`,
  `slug`, `price_minor BIGINT NOT NULL DEFAULT 0`, `currency CHAR(3) NOT NULL DEFAULT 'USD'`,
  `billing_cycle VARCHAR(16) NOT NULL` (monthly/yearly), `quota_amount BIGINT`,
  `credit_pack BIGINT`, `status VARCHAR(16) NOT NULL DEFAULT 'active'`, `created_at`,
  `updated_at`. UNIQUE(tenant_id, slug). — `price_minor=0` ⟹ free tier allocation.
- `subscriptions`: `id UUID PK`, `tenant_id UUID NOT NULL`, `plan_id UUID NOT NULL REFERENCES plans`,
  `owner_type VARCHAR(32) NOT NULL`, `owner_id VARCHAR(255) NOT NULL`, `status VARCHAR(16) NOT NULL`
  (active/pending/canceled/past_due), `current_period_id VARCHAR(100)`,
  `cancel_at_period_end BOOLEAN NOT NULL DEFAULT false`, `created_at`, `updated_at`, `started_at`.
  UNIQUE(tenant_id, owner_type, owner_id).
- `billing_periods`: `id VARCHAR(100) PK` (e.g. `2026-01`), `tenant_id UUID NOT NULL`,
  `subscription_id UUID NOT NULL REFERENCES subscriptions`, `plan_id UUID REFERENCES plans`,
  `starts_at`, `ends_at`, `status VARCHAR(16) NOT NULL` (open/closed), `created_at`.
- `credit_grants`: `id UUID PK`, `account_id UUID NOT NULL REFERENCES credit_accounts`,
  `idempotency_key VARCHAR(255) NOT NULL UNIQUE`, `source VARCHAR(16) NOT NULL`
  (allocation/purchase/manual), `amount BIGINT NOT NULL`, `tenant_id UUID NOT NULL`,
  `period_id VARCHAR(100)`, `payment_id UUID` (FK payments, nullable — allocation/manual),
  `created_at`. Guard: `CHECK (amount > 0)`.
- `credit_purchases`: `id UUID PK`, `tenant_id UUID NOT NULL`, `owner_type`, `owner_id`,
  `plan_id` (credit-pack plan), `amount_credits BIGINT NOT NULL`, `payment_id UUID NOT NULL
  REFERENCES payments`, `status VARCHAR(16) NOT NULL` (pending/captured/failed/canceled),
  `created_at`, `updated_at`. UNIQUE(payment_id).
- `credit_accounts`: add index `idx_credit_accounts_period ON credit_accounts(tenant_id, period_id)`.
- `usage_reservations`: add `idempotency_key VARCHAR(255)`, `tenant_id UUID`, unique
  constraint on idempotency_key for dedup.
- Down reverses all.

`credit_grants.idempotency_key` UNIQUE + `credit_purchases.payment_id` UNIQUE ensure
idempotent credit mutations (invariant 10).

## 6. Domain changes

- `Plan` (extend): add `TenantID string`, `PriceMinor int64`, `Currency string`,
  `BillingCycle string`, `QuotaAmount int64`, `CreditPack int64`. New `PlanRepository`
  interface: `ListByTenant(ctx, tenantID)`, `FindByTenantAndSlug(ctx, tenantID, slug)`,
  keep `FindByID`.
- `Subscription` (new): fields above; methods `Cancel(now)` (sets cancel_at_period_end),
  `Reactivate()`, `CurrentPeriodID()`. Status transitions guarded.
- `BillingPeriod` (new): `Open`/`Close`/`Contains(t)`.
- `CreditGrant` (new): carries `IdempotencyKey`, `Source`, `Amount`, `PaymentID`.
- `CreditPurchase` (new): `Create` (status pending), `MarkCaptured(paymentID)` → emits
  idempotent Grant via UsageService extension.

## 7. Service layer

- `PlanService` — list/find plans per tenant (read-only, no authz bypass).
- `SubscriptionService` — Create (validate Plan active + tenant), Cancel/Reactivate,
  OpenBillingPeriod, CloseAndRenewPeriod (creates next BillingPeriod + CreditGrant for
  allocation). All tenant-scoped; actor resolved server-side.
- `CreditPurchaseService` — `InitiateCreditPurchase(ctx, tenantID, owner, planID,
  idempotencyKey)` → creates Payment (PaymentService.Initiate, provider-agnostic) +
  CreditPurchase (pending); `OnPaymentCaptured(...)` → `CreditGrant` (idempotent, tx) →
  updates CreditPurchase to captured. Mirrors existing `payment_handlers.go` confirmation
  pattern.
- `UsageService` (extend) — add `GrantWithIdempotency(ctx, accountID, amount, reason,
  idempotencyKey, paymentID) error` wrapping a `tx`; make `Reserve` persist + return the
  reservation row; add `UsageReservationRepository`.

## 8. CreditPurchase flow (Payment-gated Grant)

1. Client → `CommerceActionRuntime` for `ActionInitiateCreditPurchase` (or direct API
   `/credit-purchases`). Actor + tenant resolved from auth context (never from client).
2. `CreditPurchaseService.InitiateCreditPurchase` → PaymentService creates a `Payment`
   (status Pending, provider-agnostic) + `CreditPurchase{status:pending}`.
3. Provider webhook (via PaymentService webhook handler) → `CreditPurchaseService.
   OnPaymentCaptured(paymentID)` (idempotent by PaymentID).
4. Inside a Postgres tx: lookup CreditPurchase → lookup CreditAccount for tenant/period →
   `INSERT credit_grants (... idempotency_key ...) ON CONFLICT DO NOTHING` →
   `UPDATE credit_accounts SET granted = granted + N WHERE account_id = $1` →
   `INSERT usage_ledger (..., transaction_type=PURCHASE, ...)`. If grant row already
   existed, no-op (idempotency). CreditPurchase.status → captured.
5. All mutations fail-fast on negative-balance check; `remaining()` never < 0.

## 9. Concurrency / idempotency test plan (against Postgres)

- `TestGrant_Idempotent` — same idempotency_key twice ⇒ one ledger row / one granted bump.
- `TestReserve_PersistedAndReusable` — Reserve returns an ID; Consume works after a
  process-boundary read (simulate via a fresh repo handle).
- `TestConcurrentReserve_NoOverdraw` — N parallel Reserves on an account with limited
  grants; assert total Reserved+Consumed ≤ Granted.
- `TestCreditPurchase_EndToEnd_MockedProvider` — reuse
  `integrationTestProviderAdapter` from
  `infrastructure/postgres/integration/payment_integration_test.go`; Initiate → Capture →
  assert `credit_grants` + `credit_purchases` + balance bump + ledger PURCHASE row.
- `TestSubscription_PlanRenewal` — open period, grant allocation, close+renew → new
  BillingPeriod + new CreditGrant; balances reset per period.

## 10. Files to touch (Sprint 20)

- `internal/domain/usage.go` (extend Plan), `internal/domain/credit.go` (CreditGrant/CreditPurchase aggregates + error vars), `internal/domain/commerce_usage.go` (extend PlanRepository; add reservation repo interface + idempotency to UsageService interface).
- `internal/application/usage.go` (fix F3/F4; add GrantWithIdempotency; persist Reserve).
- `internal/application/usage/monetization.go` (NEW: PlanService, SubscriptionService, CreditPurchaseService).
- `infrastructure/postgres/monetization.go` (NEW: Plan/Subscription/BillingPeriod/CreditGrant/CreditPurchase/Reservation repos; ON CONFLICT upserts).
- `internal/api/monetization_handlers.go` (NEW: /plans /subscriptions /credit-purchases).
- `migrations/000017_monetization.sql` (NEW).
- `cmd/payment-service/main.go` (wire monetization services + HTTP routes).
- `infrastructure/postgres/integration/monetization_integration_test.go` (NEW: concurrency/idempotency against real Postgres).

## 11. Open design decisions

- D1. Plan vs Subscription separation: confirmed NEW Subscription aggregate owning the
  BillingPeriod lifecycle; Plan is a per-tenant template. (Decided.)
- D2. BillingPeriod as explicit aggregate: yes (`billing_periods` table), owned by
  Subscription, one open period at a time per subscription. (Decided.)
- D3. Grant idempotency key: DB UNIQUE constraint + ON CONFLICT; caller-provided or
  derived from (tenant, period, source) where deterministic. (Decided.)
- D4. Reuse Sprint 19 PaymentService mock for CreditPurchase tests: yes, via
  `integrationTestProviderAdapter` — but CreditPurchase uses provider-agnostic
  `PaymentProviderAdapter` (no Stripe SDK). (Decided.)
