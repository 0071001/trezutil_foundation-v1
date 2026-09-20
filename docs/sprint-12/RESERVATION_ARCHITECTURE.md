# Sprint 12 — Reservation Architecture

## Overview

This document describes the architecture for the generic Availability + Reservation capability introduced in Sprint 12. The design follows the existing Trezutil patterns established in Sprints 1-11.

## Core Principles

1. **Listing ≠ Reservation** — A Listing represents an offer ("what is being offered?"). A Reservation represents a commitment against availability ("has this availability been held/booked for this customer?"). They are separate aggregates.

2. **Generic Reservation Model** — One `Reservation` aggregate supports all bookable categories (hotel rooms, restaurant tables, appointments, vehicle rentals, events, property viewings, service slots, etc.) without creating vertical-specific reservation types.

3. **Availability as a Boundary** — Availability is checked via an `AvailabilityProvider` port, allowing internal data, seller schedules, inventory, or future external adapters to serve as sources without changing the Reservation domain.

4. **Tenant Isolation** — All operations enforce tenant (organization) boundaries server-side.

5. **Concurrency Protection** — Double-booking prevention uses PostgreSQL exclusion constraints (GiST index with `tsrange`) for temporal overlap prevention on the same resource.

## Architecture Layers

```
┌─────────────────────────────────────────────────────────────┐
│                        CHANNELS                              │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                  │
│  │   REST   │  │ WhatsApp │  │   Web    │                  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                  │
└───────┼─────────────┼─────────────┼────────────────────────┘
        ▼             ▼             ▼
┌─────────────────────────────────────────────────────────────┐
│                   COMMERCE ACTION RUNTIME                    │
│  Identity → Tenant → Validation → Idempotency → Confirmation │
│  → Usage → Handler Dispatch                                  │
└────────────────────────┬────────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                    RESERVATION USE CASES                     │
│  CheckAvailability → CreateReservation → GetReservation     │
│  → CancelReservation → ModifyReservation (NOT_IMPLEMENTED)  │
└────────────────────────┬────────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                      DOMAIN LAYER                            │
│  ┌───────────────┐  ┌───────────────┐  ┌────────────────┐  │
│  │  Reservation  │  │  Availability │  │ Availability   │  │
│  │  (Aggregate)  │  │    Query      │  │   Provider     │  │
│  └───────────────┘  └───────────────┘  │   (Port)       │  │
│                                        └────────────────┘  │
└────────────────────────┬────────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   POSTGRESQL PERSISTENCE                     │
│  ┌────────────────┐  ┌───────────────────────────────────┐  │
│  │ Reservations   │  │ Exclusion Constraint              │  │
│  │ Table          │  │ no_overlapping_reservations       │  │
│  └────────────────┘  └───────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## Data Flow

### Create Reservation
```
Client Request
    ↓
REST API /v1/reservations (POST)
    ↓
ReservationHandler.CreateReservationH
    ↓
ReservationUseCases.CreateReservation
    ↓
1. Validate actor, tenant, listing
2. Check Availability (AvailabilityProvider)
3. Idempotency check
4. Create Reservation aggregate (PENDING)
5. Apply ConfirmationPolicy (auto-confirm if configured)
6. Persist via ReservationRepository (with exclusion constraint)
7. Audit log
8. Return Reservation
```

### Check Availability
```
Client Request
    ↓
REST API /v1/availability (GET)
    ↓
ReservationHandler.CheckAvailabilityH
    ↓
AvailabilityUseCases.CheckAvailability
    ↓
AvailabilityProvider.CheckAvailability
    ↓
Return AvailabilityResult
```

## Status

**DONE** — Architecture established with:
- Generic Reservation aggregate (`internal/domain/reservation.go`)
- Availability Query/Result/Provider (`internal/domain/availability.go`)
- ReservationUseCases interface and implementation (`internal/application/reservations/`)
- AvailabilityUseCases interface and implementation (`internal/application/availability/`)
- CommerceActionRuntime integration for all reservation actions
- REST API handlers (`internal/api/handlers.go`)
- WhatsApp service integration (`internal/application/whatsapp/service.go`)
- PostgreSQL migration with exclusion constraint (`migrations/000009_reservations_schema.sql`)
- PostgreSQL ReservationRepository (`infrastructure/postgres/reservations.go`)
- OpenAPI specification updates (`api/openapi/openapi.yaml`)

## Files Created/Modified

| File | Status |
|------|--------|
| `migrations/000009_reservations_schema.sql` | ✅ DONE |
| `infrastructure/postgres/reservations.go` | ✅ DONE |
| `internal/application/reservations/reservation_use_cases.go` | ✅ DONE (pre-existing) |
| `internal/application/availability/availability_use_cases.go` | ✅ DONE (pre-existing) |
| `internal/application/commerce/reservation_handlers.go` | ✅ DONE |
| `internal/application/commerce/validation.go` | ✅ DONE (updated) |
| `internal/application/commerce/commerce_use_cases.go` | ✅ DONE (updated) |
| `internal/application/whatsapp/service.go` | ✅ DONE (updated) |
| `internal/api/handlers.go` | ✅ DONE (updated) |
| `api/openapi/openapi.yaml` | ✅ DONE (updated) |