# Sprint 12 — Web UX Flow

## Overview

Extends the Sprint 11 Web UI with reservation capabilities using the existing Trezutil Design System. No new visual language — reuses Deep Navy, Trezutil Blue, Ice, Mint, Coral tokens.

## New Pages/Components

### 1. Availability Picker (Reusable)
```tsx
<AvailabilityPicker
  listingId="uuid"
  onAvailabilityChange={(result) => {...}}
  onSelect={(startAt, endAt) => {...}}
/>
```
- Date range picker for date-based (hotel, rental)
- Time slot picker for slot-based (restaurant, appointment)
- Quantity/party size selector
- Shows real-time availability status

### 2. Reservation Summary
```tsx
<ReservationSummary
  reservation={reservation}
  listing={listing}
  showActions={true}
  onCancel={handleCancel}
/>
```
- Status badge (Pending/Confirmed/Completed/Cancelled)
- Date/time display
- Party size / quantity
- Pricing snapshot (if available)
- Cancel action (if allowed)

### 3. Confirmation Sheet
```tsx
<ConfirmationSheet
  title="Confirm Reservation"
  details={...}
  onConfirm={handleConfirm}
  onCancel={handleClose}
/>
```
- Modal overlay with reservation details
- Shows pricing, dates, policies
- Primary: Confirm | Secondary: Cancel

### 4. Reservation List
```
/reservations
```
- Tabbed: Upcoming | Past | Cancelled
- Cards with key info + actions

### 5. Reservation Detail
```
/reservations/:id
```
- Full details
- Timeline (Created → Confirmed → Completed)
- Cancel button (if PENDING/CONFIRMED)

## User Flows

### Flow 1: Hotel-Style Date Range
```
Search Results → Listing Detail → Availability Picker (Calendar)
                                         ↓
                              Select Check-in/Check-out
                                         ↓
                              Party Size / Rooms
                                         ↓
                              [Check Availability] → Loading → Result
                                         ↓
                              [Reserve] → Confirmation Sheet → [Confirm]
                                         ↓
                              Success Toast → Redirect to /reservations/:id
```

### Flow 2: Restaurant-Style Time Slot
```
Listing Detail → "Reserve Table"
              → Date Picker → Time Slot Grid (14:00, 14:30, 15:00...)
              → Party Size (covers)
              → [Check Availability] → Result
              → [Reserve] → Confirmation → Success
```

### Flow 3: Appointment
```
Listing Detail → "Book Appointment"
              → Date → Time Slots (15-min increments)
              → [Book] → Confirmation → Success
```

### Flow 4: Quantity-Based (Tickets)
```
Listing Detail → "Get Tickets"
              → Quantity Selector (1-10)
              → [Check Availability] → Remaining: 47
              → [Reserve] → Confirmation → Success
```

## Design System Usage

### Colors
| Token | Usage |
|-------|-------|
| Deep Navy (`#0B1220`) | Primary text, headers |
| Trezutil Blue (`#1E6FCC`) | Primary actions, links, focus rings |
| Ice (`#F0F4F8`) | Backgrounds, cards |
| Mint (`#00C896`) | Success states, available slots |
| Coral (`#FF6B6B`) | Error states, unavailable, destructive actions |

### Motion
| Transition | Duration | Easing |
|------------|----------|--------|
| Availability found | 200ms | ease-out |
| Slot selection | 150ms | ease-out |
| Confirmation open | 250ms | ease-out |
| Success toast | 300ms | spring |

Respects `prefers-reduced-motion`.

### Components Reused
- `Button` (primary, secondary, ghost, destructive)
- `Input` / `Select` / `DatePicker`
- `Card` / `Modal` / `Toast`
- `Badge` (status: mint=confirmed, coral=cancelled, blue=pending)
- `Spinner` / `Skeleton`

## API Integration

```typescript
// Check availability
const result = await api.get('/v1/availability', {
  params: { listing_id, start_at, end_at, quantity, party_size }
});

// Create reservation
const reservation = await api.post('/v1/reservations', {
  listing_id, start_at, end_at, quantity, party_size
}, {
  headers: { 'Idempotency-Key': generateKey() }
});

// Get reservation
const reservation = await api.get(`/v1/reservations/${id}`);

// Cancel reservation
await api.post(`/v1/reservations/${id}/cancel`);
```

## Responsive

| Breakpoint | Layout |
|------------|--------|
| Mobile (< 640px) | Stacked, full-width modals |
| Tablet (640-1024px) | Side-by-side calendar + details |
| Desktop (> 1024px) | Multi-column, persistent summary |

## Accessibility

- ARIA labels on all pickers
- Keyboard navigation for date/time selection
- Focus trap in confirmation modal
- Screen reader announcements for status changes
- Color contrast ≥ 4.5:1

## Status

**PARTIAL** — Core components designed; implementation pending in `web/` directory

## Files (Planned)

- `web/js/components/AvailabilityPicker.js`
- `web/js/components/ReservationSummary.js`
- `web/js/components/ConfirmationSheet.js`
- `web/js/pages/ReservationsPage.js`
- `web/js/pages/ReservationDetailPage.js`
- `web/reservations.html`
- `web/reservation-detail.html`