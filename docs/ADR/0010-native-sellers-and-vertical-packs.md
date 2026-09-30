# ADR 0010 — Native sellers first; verticals as packs over the Commerce Core

## Decision
- A hotel, restaurant, service provider, agency, retailer, transport operator or event organizer is a **native Trezutil seller**: an organization with a seller profile that operates in one or more **verticals**.
- A vertical is capabilities, vocabulary and rules over the **Commerce Core**. It reuses the core Listing, Inventory, Capacity, Reservation, Order, Payment and Fulfillment. It composes the existing transaction packs (ECOMMERCE, BOOKING, SERVICES) and never adds its own engine.
- Every listing belongs to exactly one vertical (`listings.vertical`). Vertical definitions live in code (`application/vertical`); adding one needs no migration.
- External APIs are **optional supply**, reached only through Provider Orchestration and kept distinguishable from native supply (source marker, provider reference, execution record). They always resolve to core Orders/Reservations owned by an organization.
- The AI returns intents and parameters, including a vertical. It never selects a provider.

## Rationale
- Trezutil's supply in Kinshasa is local businesses, not APIs; treating them as "providers" would put the platform's core customers behind an integration layer they don't have.
- One core keeps authorization, payments, the ledger, the seller console and conversations identical across verticals.

## Invariants
- The Commerce Core never imports a vertical, and a vertical never imports a provider.
- An external offer never masquerades as a native listing.

Details: `docs/architecture/VERTICAL_COMMERCE_PACKS.md`.
