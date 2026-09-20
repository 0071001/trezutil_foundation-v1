# Sprint 9 — Usage & Credits

## Overview

Adds usage and billing domain: credit accounts, usage policies, reservations, and ledger events. Integrates UsageService with CommerceActionRuntime via check/reserve-before-dispatch and consume/release-after-execution pattern.

## Planned Areas

- **9A**: Usage domain entities and interfaces (CreditAccount, UsagePolicy, UsageReservation, UsageLedger)
- **9B**: UsageService application layer (Reserve, Consume, Release, Grant, Check, ResolvePolicy, GetBalance)
- **9C**: CommerceActionRuntime integration (policy check, credit reserve, consume/release lifecycle)
- **9D**: Tests (18 UsageService tests + 5 runtime integration tests)

## Completed Areas

All areas complete.

## Gate Status

- Unit tests: PASS (94 commerce + 18 usage tests)
- Integration tests: BLOCKED (PostgreSQL unavailable for ledger/account persistence)
- Production: NOT_READY
