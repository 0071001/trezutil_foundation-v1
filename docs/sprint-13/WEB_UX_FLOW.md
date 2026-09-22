# Sprint 13 — Web UX Flow

## Overview

Extend the existing Trezutil Web design system (Sprint 11) with Inventory, Capacity, and Fulfillment views.

**Design Tokens:**
- Trezutil Blue: `#155EEF`
- Deep Navy: `#0B1220`
- Ice: `#F5F8FC`
- Mint: `#35D0BA`
- Coral: `#FF7A66`

**Visual States:**
- `Available` → Mint badge
- `Limited` → Amber badge (≤20% available)
- `Reserved` → Blue badge
- `Out of Stock` → Coral badge
- `In Progress` → Blue spinner
- `Ready` → Mint badge
- `Completed` → Green check
- `Cancelled` → Gray strikethrough

---

## Customer Views

### Product/Listing Detail — Availability

**Component:** `AvailabilityIndicator`

```jsx
<AvailabilityIndicator 
  available={7} 
  total={100} 
  type="COUNTABLE" 
  showLimitedThreshold={0.2}
/>
```

**States:**
- `Available 7` → Mint pill
- `Limited: 2 remaining` → Amber pill with warning icon
- `Out of stock` → Coral pill
- `Coming soon` → Gray pill

### Order Detail — Fulfillment Status

**Component:** `FulfillmentTimeline`

```
┌─────────────────────────────────────────────┐
│  Order #CMD-2024-001                        │
├─────────────────────────────────────────────┤
│  📦 Fulfillment: DELIVERY                   │
│  Status: IN_PROGRESS                        │
│  Reference: FUL-2024-001                    │
├─────────────────────────────────────────────┤
│  ▸ PENDING        ✓  Jan 15, 10:00         │
│  ▸ ACCEPTED       ✓  Jan 15, 10:15         │
│  ● IN_PROGRESS       Jan 15, 14:30         │
│  ○ READY                              │
│  ○ COMPLETED                          │
├─────────────────────────────────────────────┤
│  Tracking: TRK-12345 [Track Shipment →]    │
└─────────────────────────────────────────────┘
```

**Status Colors:**
- Completed steps: Mint checkmark
- Current step: Blue pulse animation
- Future steps: Gray

### Reservation Detail — Fulfillment Status

Similar to order detail, shows fulfillment for appointment/event reservations.

---

## Seller Views

### Inventory Dashboard

**Route:** `/seller/inventory`

**Layout:**
```
┌─────────────────────────────────────────────────────────────┐
│  Inventory                                    [+ Adjust]    │
├─────────────────────────────────────────────────────────────┤
│  Search: [________________]  Filter: [Type ▼] [Status ▼]   │
├──────┬────────────┬────────┬────────┬────────┬──────────────┤
│ SKU  │ Product    │ Type   │ Total  │ Avail  │ Actions      │
├──────┼────────────┼────────┼────────┼────────┼──────────────┤
│ ABC  │ Widget A   │ COUNT  │ 100    │ 🟢 70  │ [Adjust]     │
│ XYZ  │ Gadget B   │ COUNT  │ 50     │ 🟠 8   │ [Adjust]     │
│ SEAT │ Table 4    │ SEAT   │ 10     │ 🟢 7   │ [View Cap.]  │
├──────┴────────────┴────────┴────────┴────────┴──────────────┘
│  Showing 1-20 of 45                    [Prev] [Next]        │
└─────────────────────────────────────────────────────────────┘
```

**Adjust Modal:**
```
┌────────────────────────────────────┐
│  Adjust Inventory: Widget A        │
├────────────────────────────────────┤
│  Reason: [RECEIVE ▼]               │
│  Δ Total:    [ +10 ]               │
│  Δ Reserved: [  0  ]               │
│  Δ Consumed: [  0  ]               │
│  Note: [________________________]  │
├────────────────────────────────────┤
│  [Cancel]              [Save]      │
└────────────────────────────────────┘
```

### Capacity Calendar

**Route:** `/seller/capacity/:inventoryId`

**Weekly View:**
```
        Mon 15    Tue 16    Wed 17    Thu 18    Fri 19
09:00  ████░░░░  ████████  ████░░░░  ████████  ████░░░░
       4/10      10/10     4/10      10/10     4/10
10:00  ████░░░░  ████░░░░  ████████  ████░░░░  ████████
       4/10      4/10      10/10     4/10      10/10
...
Legend: ████ Reserved  ░░░░ Available
```

### Fulfillment Dashboard

**Route:** `/seller/fulfillment`

**Tabs:** `Assigned to Me` | `All` | `Pending` | `In Progress` | `Ready`

**List View:**
```
┌─────────────────────────────────────────────────────────────┐
│  Fulfillments                              Filters ▼        │
├────────┬──────────┬──────────┬────────┬────────┬───────────┤
│ Ref    │ Order/Res│ Type     │ Status │ Assigned│ Actions  │
├────────┼──────────┼──────────┼────────┼────────┼───────────┤
│ FUL-001│ CMD-001  │ DELIVERY │ 🔵 IP  │ Me      │ [→ Ready]│
│ FUL-002│ RES-005  │ APPOINT  │ 🟢 RDY │ John    │ [Complete]│
├────────┴──────────┴──────────┴────────┴────────┴───────────┘
```

**Detail Modal:**
```
┌────────────────────────────────────────────┐
│  FUL-2024-001                    [×]       │
├────────────────────────────────────────────┤
│  Order: #CMD-2024-001                      │
│  Type: DELIVERY                            │
│  Status: IN_PROGRESS                       │
│  Assigned: John Doe                        │
│  Tracking: TRK-12345 [Edit]                │
├────────────────────────────────────────────┤
│  Address:                                  │
│  123 Main St, City, 12345                  │
│  Phone: +15551234567                       │
├────────────────────────────────────────────┤
│  Timeline:                                 │
│  ✓ PENDING      Jan 15, 10:00              │
│  ✓ ACCEPTED     Jan 15, 10:15              │
│  ● IN_PROGRESS  Jan 15, 14:30 (You)        │
│  ○ READY                                    │
│  ○ COMPLETED                                │
├────────────────────────────────────────────┤
│  [Mark Ready]  [Mark Failed]  [Cancel]     │
└────────────────────────────────────────────┘
```

---

## Components to Implement

| Component | File | Props |
|-----------|------|-------|
| `AvailabilityIndicator` | `web/js/components.js` | `available, total, type, variant` |
| `FulfillmentTimeline` | `web/js/components.js` | `fulfillment, compact` |
| `InventoryTable` | `web/js/components.js` | `items, onAdjust, onViewCapacity` |
| `CapacityCalendar` | `web/js/components.js` | `units, onReserve` |
| `FulfillmentTable` | `web/js/components.js` | `items, onTransition, onAssign` |
| `AdjustInventoryModal` | `web/js/components.js` | `inventory, onSave, onClose` |
| `FulfillmentDetailModal` | `web/js/components.js` | `fulfillment, onTransition, onClose` |

---

## Pages to Implement

| Page | Route | Components |
|------|-------|------------|
| `InventoryDashboard` | `/seller/inventory` | `InventoryTable`, `AdjustInventoryModal` |
| `CapacityCalendar` | `/seller/capacity/:id` | `CapacityCalendar` |
| `FulfillmentDashboard` | `/seller/fulfillment` | `FulfillmentTable`, `FulfillmentDetailModal` |
| `OrderDetail` (update) | `/orders/:id` | `FulfillmentTimeline` |
| `ReservationDetail` (update) | `/reservations/:id` | `FulfillmentTimeline` |
| `ListingDetail` (update) | `/listings/:id` | `AvailabilityIndicator` |

---

## API Integration

All components use existing API client:

```javascript
// web/js/api.js additions
export const inventoryApi = {
  list: (params) => api.get('/v1/inventory', params),
  get: (id) => api.get(`/v1/inventory/${id}`),
  adjust: (id, data) => api.post(`/v1/inventory/${id}/adjust`, data),
};

export const capacityApi = {
  get: (inventoryId, params) => api.get(`/v1/capacity/${inventoryId}`, params),
  check: (data) => api.post('/v1/capacity/check', data),
};

export const fulfillmentApi = {
  get: (id) => api.get(`/v1/fulfillments/${id}`),
  byOrder: (orderId) => api.get(`/v1/orders/${orderId}/fulfillment`),
  byReservation: (resId) => api.get(`/v1/reservations/${resId}/fulfillment`),
  transition: (id, data) => api.post(`/v1/fulfillments/${id}/transition`, data),
  assign: (id, data) => api.post(`/v1/fulfillments/${id}/assign`, data),
  tracking: (id, data) => api.post(`/v1/fulfillments/${id}/tracking`, data),
};
```

---

## Motion Language

| Transition | Animation |
|------------|-----------|
| Status change | 200ms ease-out color morph |
| Timeline step activation | 300ms pulse + checkmark draw |
| Modal open/close | 150ms scale + fade |
| Table row hover | 100ms background tint |
| Button press | 50ms scale(0.98) |

---

## Accessibility

- All status indicators have text labels (not color-only)
- Keyboard navigation for all interactive elements
- ARIA labels for status badges
- Focus indicators on all clickable elements
- Screen reader announcements for status changes