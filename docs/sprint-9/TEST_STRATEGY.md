# Sprint 9 — Usage & Credits — Test Strategy

## Unit Tests

### UsageService (18 tests)

Location: `internal/application/usage_test.go`

All tests use fake repositories (no database required):

- `GetBalance`: success, not found (2)
- `ResolvePolicy`: success, not found (2)
- `Check`: sufficient, insufficient, zero cost (3)
- `Reserve`: success, insufficient credits, invalid cost (3)
- `Consume`: success, not found, already consumed (3)
- `Release`: success, already released (2)
- `Grant`: success, invalid amount (2)
- Reserve-Consume-Release workflow (1)
- Negative balance prevention (1)

### Runtime Integration (5 tests)

Location: `internal/application/commerce/runtime_test.go`

All tests use `fakeUsageService` and `fakeHandler`:

- `TestExecute_UsageReserveAndConsume` — metered action succeeds, credits consumed
- `TestExecute_UsageReserveAndRelease` — metered action succeeds, credits consumed (alias)
- `TestExecute_UsageReserveAndReleaseOnFailure` — handler fails, credits released
- `TestExecute_UsageInsufficientCredits` — account can't afford cost, action rejected
- `TestExecute_UsageSystemActorSkips` — system actor bypasses usage check

## Infrastructure Tests

None yet. PostgreSQL-backed repository tests require PostgreSQL integration.
