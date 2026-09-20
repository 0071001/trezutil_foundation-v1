# Sprint 9 — Usage & Credits + Runtime Integration

## Overview

Sprint 9 adds the usage and billing domain (credit accounts, usage policies, reservations, ledger events) and integrates it with the CommerceActionRuntime via a check/reserve-before-dispatch and consume/release-after-execution pattern.

## Planned Areas

- **9A — Usage Domain**: CreditAccount, UsagePolicy, UsageReservation, UsageLedger entities and interfaces
- **9B — UsageService**: Reserve, Consume, Release, Grant, Check, ResolvePolicy, GetBalance
- **9C — Runtime Integration**: UsageService wired into CommerceActionRuntime (policy check, credit reserve, consume/release lifecycle)
- **9D — Tests**: Unit and integration tests for UsageService and runtime usage flow

## Completed Areas

All areas complete. UsageService fully implemented with 18 tests. Runtime integration has 5 tests covering reserve+consume, release on failure, insufficient credits, and system actor skip.

## Gate Status

Per project INTEGRATION_GATE.md:

- Unit tests: PASS (94 commerce + 18 usage tests; all pass)
- Integration tests: BLOCKED (usage ledger persistence depends on PostgreSQL)
- Production: NOT_READY
