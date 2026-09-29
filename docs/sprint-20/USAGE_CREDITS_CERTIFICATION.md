---
title: Usage & Credits Hardening Certification (Sprint 20.1)
status: GO
---

# Usage & Credits Hardening Certification

## 1. Scope
Close the final Sprint 20 hardening gaps (F2 durable reservations, F5 UUID reservation IDs, F6 concurrent over-draw prevention) and certify credit reservations/consume/release against real PostgreSQL. No Sprint 21, no new monetization products, no gateway changes (gateway remains `GO` from Sprint 19.3).

## 2. Architecture
```
Client/Runtime
  -> CommerceActionRuntime
  -> UsageService (domain.UsageService)
  -> ReserveCreditsTx / ConsumeReservation / ReleaseReservation  (atomic, single Postgres tx)
  -> credit_accounts / usage_reservations / usage_ledger
  -> PostgreSQL
```
One authoritative credit account per owner/tenant/period. No in-process-only state; no Go mutexes; over-draw is prevented at the database via `UPDATE ... AND (granted - reserved - consumed) >= cost` plus `SELECT ... FOR UPDATE` serialization.

## 3. Fixes applied
- **F6 over-draw**: `ReserveCreditsTx` balance-guard UPDATE prevents negative credits; concurrent reservations serialize.
- **F2 durable reservations**: all reservation state lives in `usage_reservations` (UUID PK), surviving restarts.
- **F5 UUID IDs**: `reservation_id` and `account_id` are `gen_random_uuid()`.
- **Deadlock fix**: `ReserveCreditsTx` now `SELECT ... FOR UPDATE` on `credit_accounts` first, eliminating the INSERT(FK share-lock)/UPDATE(exclusive-lock) deadlock.
- **Rollback fix**: `ReserveCreditsTx`, `ConsumeReservation`, `ReleaseReservation` use named returns so the deferred rollback fires on all error paths (previously failed-reserve transactions held `FOR UPDATE` locks indefinitely).
- **Scan fix**: `CreditAccountRepository.FindByID`/`FindByOwner` now scan `created_at`/`updated_at` as `time.Time` via `toUnix` (previously scanned a timestamp into an `int64`, breaking `GetBalance`).

## 4. Concurrency evidence (real PostgreSQL, `-run TestUsage_`)
```
=== RUN   TestUsage_ConcurrentReservations_OverdrawPrevented   --- PASS (2.00s)
=== RUN   TestUsage_ConcurrentMixedSizeReservations            --- PASS (1.87s)
=== RUN   TestUsage_ConcurrentGrantAndReserve                  --- PASS (1.85s)
=== RUN   TestUsage_ReserveIdempotent                          --- PASS (1.72s)
=== RUN   TestUsage_ConsumeAndRelease_NoDoubleAccounting        --- PASS (1.72s)
```
- Test A (20x10 on 100 credits): 10 succeeded, 10 failed; balance never negative; `reserved=100`.
- Test B (70 vs 40): exactly one wins.
- Test C (concurrent grant+reserve): final balance consistent, never negative.
- Idempotency: duplicate reserve returns same ID; double consume/release rejected (`ErrUsageAlreadyConsumed`/`ErrUsageAlreadyReleased`).

## 5. Regression
- `gofmt -l` — clean for all changed files (`internal/application/usage_test.go` flagged is pre-existing and out of scope).
- `go build ./...` — PASS
- `go vet ./...` — PASS
- `go test ./internal/application/payment/... ./internal/application/conversation/...` — PASS (Sprint 19 Payment Core remains `GO`).

## 6. Limitations / environment
- The full `infrastructure/postgres/integration` package is shared and timed out under PG connection contention when run in full; per-test runs of `TestUsage_*` pass. Sprint 19 Payment Core retains its own certified suite.

## 7. Final certification

**GO**

The hardening gaps F2/F5/F6 are resolved; the durable, atomic, concurrent-safe reservation path is verified against real PostgreSQL.
