# Sprint 12 — Capability Matrix

## Overview

Maps reservation capabilities to vertical categories to demonstrate generic design.

## Capability × Vertical Matrix

| Capability | Hotel Room | Restaurant Table | Appointment | Vehicle Rental | Event Ticket | Property Viewing | Service Slot |
|------------|:----------:|:----------------:|:-----------:|:--------------:|:------------:|:----------------:|:------------:|
| **Check Availability** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Date-range query | ✅ | — | — | ✅ | — | ✅ | — |
| Time-slot query | — | ✅ | ✅ | — | — | — | ✅ |
| Quantity query | ✅ | — | — | ✅ | ✅ | — | — |
| Party-size query | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Create Reservation** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| PENDING → CONFIRMED | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Auto-confirm option | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Idempotency | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Get Reservation** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Customer view | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Seller view | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Cancel Reservation** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Customer cancel | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Seller cancel | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Policy-aware | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **Modify Reservation** | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Change dates | — | — | — | — | — | — | — |
| Change quantity | — | — | — | — | — | — | — |
| Change party size | — | — | — | — | — | — | — |

## Availability Model per Vertical

| Vertical | Model | Capacity Source | Example Query |
|----------|-------|-----------------|---------------|
| Hotel Room | Date-range | Rooms per type | Oct 12-15, 1 room, 2 guests |
| Restaurant Table | Time-slot | Covers per slot | Oct 12 19:00-21:00, 4 covers |
| Appointment | Time-slot | 1 per slot | Oct 12 14:00-14:30 |
| Vehicle Rental | Date-range | Fleet count | Oct 12-15, 1 car |
| Event Ticket | Quantity | Total tickets | 2 tickets, GA section |
| Property Viewing | Time-slot | Agent schedule | Oct 12 10:00-10:30 |
| Service Slot | Time-slot | Provider slots | Oct 12 14:00-15:00, 1 slot |

## Reservation Fields per Vertical

| Field | Hotel | Restaurant | Appointment | Vehicle | Event | Viewing | Service |
|-------|:-----:|:----------:|:-----------:|:-------:|:-----:|:-------:|:-------:|
| StartAt | Check-in | Slot start | Appt start | Pickup | Event start | Viewing start | Slot start |
| EndAt | Check-out | Slot end | Appt end | Return | Event end | Viewing end | Slot end |
| Quantity | Rooms | — | — | Vehicles | Tickets | — | — |
| PartySize | Guests | Covers | Attendees | Passengers | — | Viewers | — |
| PricingSnapshot | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |

## API Coverage per Vertical

All verticals use the **same endpoints**:

```
GET  /v1/availability?listing_id=...&start_at=...&end_at=...&quantity=...&party_size=...
POST /v1/reservations
GET  /v1/reservations/{id}
POST /v1/reservations/{id}/cancel
```

Vertical-specific behavior controlled by:
- Listing attributes (type, capacity, pricing)
- AvailabilityProvider implementation
- ConfirmationPolicy

## Channel Coverage

| Capability | REST API | WhatsApp | Web |
|------------|:--------:|:--------:|:---:|
| Check Availability | ✅ | ✅ | ✅ |
| Create Reservation | ✅ | ✅ | ✅ |
| Get Reservation | ✅ | ✅ | ✅ |
| Cancel Reservation | ✅ | ✅ | ✅ |
| Modify Reservation | ❌ | ❌ | ❌ |

## Status

| Capability | Domain | Application | API | Web | WhatsApp | Tests |
|------------|:------:|:-----------:|:---:|:---:|:--------:|:-----:|
| Check Availability | ✅ | ✅ | ✅ | 🔄 | ✅ | 🔄 |
| Create Reservation | ✅ | ✅ | ✅ | 🔄 | ✅ | 🔄 |
| Get Reservation | ✅ | ✅ | ✅ | 🔄 | ✅ | 🔄 |
| Cancel Reservation | ✅ | ✅ | ✅ | 🔄 | ✅ | 🔄 |
| Modify Reservation | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |

✅ = Done | 🔄 = In Progress | ❌ = Not Implemented

## Notes

1. **ModifyReservation** returns `ErrNotImplemented` — documented in `reservation_use_cases.go:250`
2. **Web UI** components designed but not yet implemented in `web/`
3. **Tests** strategy documented; unit tests exist for domain; application/integration tests pending