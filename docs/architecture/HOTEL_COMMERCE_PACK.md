# Hotel Commerce Pack

Status: operational (native sellers) · Date: 2026-09-29 · Model: [VERTICAL_COMMERCE_PACKS.md](VERTICAL_COMMERCE_PACKS.md) · Code: `internal/application/hotel`, `internal/domain/hotel.go`, migration 000020

The Hotel pack is the first native vertical built on the Commerce Core. A hotel is a Trezutil seller, not an external provider. The pack adds hotel vocabulary and rules; it has **no** booking, availability, reservation or payment engine of its own.

## 1. Domain model and mapping to the Commerce Core

| Hotel concept | Commerce Core entity | Notes |
|---|---|---|
| Hotel / property | **Store** of the seller's organization | Name, address, city (the property page and place search read them). |
| Room type ("Deluxe room") | **Listing**, vertical HOTEL | Name, description, nightly rate (listing price), photos (listing media), draft/published/paused lifecycle. |
| Room-type facts | **`hotel_room_types`** (new, 1:1 with the listing) | Guests per room (1–20), bed configuration, amenities. The only hotel-specific table. |
| Room / unit | The listing's **InventoryResource**, type ROOM | `total` = number of rooms of that type (1–500). Rooms are logical units; room numbers are future work. |
| Nightly capacity | **CapacityUnit** of that resource, one per UTC night | `capacity` = rooms offered that night (≤ units), `reserved` = rooms booked. No window = closed night. |
| Availability | Core **AvailabilityProvider** (`CapacityAvailabilityProvider`) | A stay is available only if every night is open, and it is as available as its tightest night. |
| Stay | Core **Reservation** | `quantity` = rooms, `party_size` = guests, `start_at`/`end_at` = check-in/check-out UTC midnights, `capacity_managed = true`. |
| Stay payment | Core **Order** + **Payment Core** | One order per stay (key `stay:<reservation id>`): nights × rooms at the nightly rate; paid with the normal `POST /payments`. |
| Guest arrival | Core **Fulfillment**, type APPOINTMENT | Created with the confirmed reservation. |

## 2. Rules

- **Nights are UTC calendar dates.**
  - A stay from 2026-10-10 to 2026-10-12 covers the nights of the 10th and 11th.
  - Its reservation runs from 2026-10-10T00:00Z to 2026-10-12T00:00Z.
  - Check-in and check-out times are presentation only.
- **Stay rule** (`hotel.Service.CheckReservation`, a `domain.ReservationRule` registered on every reservation use case, web and chat):
  - A stay of a configured room type starts and ends at night boundaries and lasts 1–30 nights.
  - It has at least one guest, and the guests fit the rooms booked (guests ≤ occupancy × rooms).
  - A HOTEL listing without room-type facts follows the generic rules.
- **Capacity:**
  - A night offers 0 to *units* rooms; setting 0 closes it.
  - A night can never drop below the rooms already booked on it, and units can never drop below an upcoming night's capacity. Both are refused with 409 `CAPACITY_CONFLICT`.
- **No overbooking:** booking locks every night's capacity window in one PostgreSQL transaction. The whole stay must be covered and each night must have room, or nothing changes. The integration test races 8 guests for 3 rooms, and exactly 3 win.
- **Payment:**
  - Only the guest who booked can create or pay the stay order.
  - Reserving holds the rooms; paying online is optional (pay at the property is also offered).

## 3. Commerce Core changes this pack required

These are generic fixes: every multi-unit booking vertical (restaurant tables, event seats, vehicles) benefits.

| Change | Why |
|---|---|
| Period availability = the tightest covering window, with gap detection (`domain.CapacityCoverage`) | Availability used to be summed across windows, so a 2-night stay with 10 free rooms per night showed 20; a missing night went unnoticed. |
| `ReserveCapacity` / `ReleaseCapacity` in one transaction with row locks and coverage | A race could leave earlier nights reserved when a later one failed; a missing night was silently skipped. |
| `reservations.capacity_managed`; the exclusion constraint applies only to non-capacity-managed reservations | The constraint refused any two overlapping reservations of one listing, so a room type with 10 rooms could only be booked once per night. Single-unit listings keep the constraint. |
| `capacity_units` windows of one resource never overlap (exclusion constraint) | Makes "the tightest window" well defined. |
| The lifetime inventory counter is only used for COUNTABLE stock | Each stay would otherwise have permanently consumed a ROOM unit. |
| `POST /reservations` maps errors with `errors.Is`: 400 invalid parameter, 409 lost race for the last rooms, 503 availability unknown | These returned 500 before. |
| Store and organization reads return city, province and address | They were scanned into the wrong variables and always came back empty (property page, "Sold by … city", place search). |
| Reservation actions in chat require explicit confirmation | The runtime policy confirmed orders but not bookings, contrary to `domain.AIActionRequiresConfirmation`. |

## 4. Data model (migration 000020)

```sql
hotel_room_types (
  listing_id        UUID PK → listings(id) ON DELETE CASCADE,
  organization_id   UUID → organizations(id),
  max_occupancy     INTEGER CHECK (1..20),
  bed_configuration VARCHAR(120),
  amenities         TEXT[] (≤ 30),
  version, created_at, updated_at
)
reservations.capacity_managed BOOLEAN NOT NULL DEFAULT false
no_overlapping_reservations  … WHERE status <> 'cancelled' AND NOT capacity_managed
capacity_units_no_overlap    EXCLUDE (inventory_id =, tstzrange(start_at, end_at) &&)
```

**Backward compatible:**
- `capacity_managed` defaults to false, so existing reservations keep the exclusion constraint.
- No hotel database, and no change to orders or payments.
- The migration is re-runnable.

## 5. API (all under `/api/v1`, served by the commerce service)

| Endpoint | Who | Purpose |
|---|---|---|
| `POST /hotels/room-types` | seller (owner/admin/manager of the store's organization) | Create a room type: HOTEL listing (draft) + facts + ROOM inventory. |
| `GET /hotels/room-types?organization_id=` | organization members (staff read) | The organization's room types, drafts included. |
| `POST /hotels/room-types/{listing_id}` | manager | Change occupancy, beds, amenities (name, description, price: `POST /listings/update`). |
| `POST /hotels/room-types/{listing_id}/units` | manager | `{"units": 10}` |
| `POST /hotels/room-types/{listing_id}/capacity` | manager | `{"from":"2026-10-10","to":"2026-10-20","rooms":8}`; `to` is excluded; 0 closes. |
| `GET /hotels/room-types/{listing_id}/capacity?from=&to=` | members | Each night: capacity, reserved, available. |
| `GET /hotels/property?store_id=` | public | Property and its published room types. |
| `GET /hotels/availability?store_id=&check_in=&check_out=&guests=[&rooms=]` | public | Per room type: rooms needed, rooms left (tightest night), fits guests, nights, total price. |
| `POST /reservations` | signed-in customer | The core booking endpoint: `start_at`/`end_at` = UTC midnights, `quantity` = rooms, `party_size` = guests. |
| `POST /reservations/{id}/checkout` | the guest who booked | The stay's order; then `POST /payments` as for any order. |
| `POST /listings/publish?listing_id=` | manager | Publish or pause a room type (core listing lifecycle). |

**Errors:**

| Status | Code | When |
|---|---|---|
| 400 | `INVALID_PARAMETER` | The stay rule is broken |
| 403 | `FORBIDDEN` | Not a member, or not a manager |
| 404 | `ROOM_TYPE_NOT_FOUND` | The ID is not a HOTEL room type (no existence leak for other listings) |
| 404 | `HOTEL_NOT_FOUND` | No published room types at the property |
| 409 | `CAPACITY_CONFLICT` | A seller change would go below what is booked |
| 409 | `UNAVAILABLE` | A booking lost the race for the last rooms |
| 409 | `STAY_NOT_PAYABLE` | The stay cannot be paid online |

## 6. Seller workflow

1. The Trezutil team onboards the hotel: organization, seller profile and store with address and city.
2. **Hotel rooms** (`/seller/hotel`): add a room type with name, category, nightly price, guests per room, number of rooms, beds, amenities and photos. It can be published at once.
3. **Nights:** open *n* rooms for a date range, close a range with 0, or reduce one night (a room under repair).
4. See the next 14 nights (offered, booked, left). Staff see everything read-only.
5. Stays arrive in **Reservations**; paid stays also appear as orders in **Orders**.

The generic "Create listing" flow steers HOTEL to this page. A room type needs its occupancy and nights to be bookable.

## 7. Customer workflow

**Web:**
1. Search (`/search?vertical=HOTEL&where=Gombe`) and open a hotel result.
2. The hotel page shows the property, its room types, dates and guests.
3. Check availability: every room type is checked, and sold-out ones cannot be selected.
4. Select a room and continue to **Reserve** (sign-in required): the stay summary is re-checked live, then you reserve.
5. Confirmation, then **Pay now** (the stay order through Stripe, via Payment Core) or **Pay at the property**.

**Conversation:**
- "Find me a hotel in Gombe for two nights for two people" becomes `SEARCH_LISTINGS` with `{vertical: HOTEL, location: Gombe, nights: 2, party_size: 2}`, validated by the commerce runtime.
- The reply is headed "Hotels & accommodation in Gombe · 2 nights · 2 guests".
- The criteria are remembered: after "1" and "book it from 2026-10-10", the booking gets `party_size = 2` and check-out = check-in + 2 nights. It then waits for explicit confirmation.

## 8. Date handling and known limits

- Nights are UTC dates; there is no per-property time zone. Kinshasa is UTC+1, so a night is the local 01:00–01:00 day, which is harmless for date-based stays.
- "For two nights" without a start date is kept as a criterion; the start date is asked for when booking.
- No rate plans, seasonal prices or per-night prices: one nightly rate per room type.
- No room numbers or room assignment.
- No cancellation policy or penalties: cancellation releases the rooms.
- No check-in/check-out times.
- Taxes and fees are not modelled.
- Paying online is optional; "require payment before reservation" is not enforced (BOOKING pack policy).
- Seller onboarding is still operator-assisted (L1).

## 9. Tests

- **Unit:** stay parsing, occupancy, coverage (`internal/domain/hotel_test.go`); structured stay intents through the real runtime (`internal/application/conversation/hotel_intent_test.go`).
- **Real PostgreSQL** (`infrastructure/postgres/integration/hotel_pack_test.go`):
  - seller setup → public property → quote → stays sharing nights;
  - full nights, occupancy, whole nights, closed nights, conflicts;
  - cancellation, checkout, payment ownership;
  - an 8-way race for 3 rooms;
  - cross-organization, customer and staff boundaries.
- **API end to end:** journeys H1–H5 in `test/e2e/journeys.py`.
- **Browser:** journeys I (stay) and J (seller nights) in `test/e2e/web`.
- **Web app:** `src/pages/hotel.test.tsx`.
