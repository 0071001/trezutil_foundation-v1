# ADR 0008 — Payment Provider Abstraction

## Decision
Payments are implemented behind a payment-provider interface. Provider adapters translate external states into Trezutil payment states.

## Rationale
Trezutil must support Mobile Money, cards, bank rails and other providers without coupling order logic to one provider.

## Invariant
Order payment status is authoritative only after trusted provider confirmation or controlled reconciliation.
