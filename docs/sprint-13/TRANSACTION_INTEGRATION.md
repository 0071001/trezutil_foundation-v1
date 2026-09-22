# Sprint 13 — Transaction Integration

## Overview

This document describes how Inventory, Capacity, and Fulfillment integrate with existing Order and Reservation transactions. The key principle: **atomicity within the same PostgreSQL persistence boundary**.

---

## Order → Inventory Integration

### Consumption Point

Inventory is consumed at **Order Confirmation** (`PAID` → `CONFIRMED`):

```go
func (uc *OrderUseCases) Confirm(ctx context.Context, orderID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        // 1. Lock order
        order, err := uc.orderRepo.FindByIDForUpdate(tx, orderID)
        if err != nil {
            return err
        }
        if order.Status != domain.OrderStatusPaid {
            return domain.ErrInvalidStatusTransition
        }

        // 2. For each item, consume inventory
        for _, item := range order.Items {
            // Find inventory for this listing
            inv, err := uc.inventoryRepo.FindByListingIDForUpdate(tx, item.ListingID)
            if err != nil {
                // If no inventory resource exists, skip (non-inventory-controlled)
                if errors.Is(err, domain.ErrInventoryNotFound) {
                    continue
                }
                return err
            }

            // Consume quantity
            if err := inv.Consume(item.Quantity); err != nil {
                return err
            }
            if err := uc.inventoryRepo.Update(tx, inv); err != nil {
                return err
            }
        }

        // 3. Confirm order
        now := time.Now().Unix()
        if err := order.Confirm(now); err != nil {
            return err
        }
        return uc.orderRepo.Update(tx, order)
    })
}
```

### Rollback on Failure

If any step fails:
- Transaction rolls back
- Order remains `PAID`
- Inventory unchanged
- Caller retries or escalates

### Direct Consumption (No Reservation)

For orders without prior reservation:
- `Inventory.Consume(quantity)` directly increments `Consumed`
- No `Reserved` change

### From Reservation

For orders created from confirmed reservations:
- `Inventory.Consume(quantity)` transfers `Reserved` → `Consumed`
- Reservation status should be `CONFIRMED` or `COMPLETED`

---

## Reservation → Capacity Integration

### Reservation Creation

When reservation is created for time-based resource:

```go
func (uc *ReservationUseCases) CreateReservation(ctx context.Context, ...) (*domain.Reservation, error) {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        // ... validation ...

        // 1. Check availability (includes capacity)
        available, err := uc.availabilityProvider.CheckAvailability(ctx, query)
        if err != nil || !available.Available {
            return domain.ErrAvailabilityUnavailable
        }

        // 2. Reserve capacity units
        for _, unit := range available.Units {
            if err := uc.capacityRepo.Reserve(tx, unit.ID, quantity); err != nil {
                return err
            }
        }

        // 3. Create reservation
        reservation := domain.NewReservation(...)
        if err := uc.reservationRepo.Create(tx, reservation); err != nil {
            return err
        }

        // 4. Also reserve inventory if listing has COUNTABLE inventory
        if inv, err := uc.inventoryRepo.FindByListingIDForUpdate(tx, listingID); err == nil {
            if err := inv.Reserve(quantity); err != nil {
                return err
            }
            if err := uc.inventoryRepo.Update(tx, inv); err != nil {
                return err
            }
        }

        return nil
    })
}
```

### Reservation Cancellation

```go
func (uc *ReservationUseCases) CancelReservation(ctx context.Context, actorID, reservationID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        reservation, err := uc.reservationRepo.FindByIDForUpdate(tx, reservationID)
        if err != nil {
            return err
        }

        // 1. Release capacity
        if err := uc.capacityRepo.ReleaseByReservation(tx, reservationID); err != nil {
            return err
        }

        // 2. Release inventory (if reserved)
        if inv, err := uc.inventoryRepo.FindByListingIDForUpdate(tx, reservation.ListingID); err == nil {
            if err := inv.Release(reservation.Quantity); err != nil {
                return err
            }
            if err := uc.inventoryRepo.Update(tx, inv); err != nil {
                return err
            }
        }

        // 3. Cancel reservation
        now := time.Now().Unix()
        if err := reservation.Cancel(now); err != nil {
            return err
        }
        return uc.reservationRepo.Update(tx, reservation)
    })
}
```

---

## Order → Fulfillment Integration

### Automatic Fulfillment Creation

When order reaches `READY_FOR_FULFILLMENT`:

```go
func (uc *OrderUseCases) ReadyForFulfillment(ctx context.Context, orderID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        order, err := uc.orderRepo.FindByIDForUpdate(tx, orderID)
        if err != nil {
            return err
        }
        if order.Status != domain.OrderStatusProcessing {
            return domain.ErrInvalidStatusTransition
        }

        // 1. Transition order
        now := time.Now().Unix()
        if err := order.ReadyForFulfillment(now); err != nil {
            return err
        }

        // 2. Create fulfillment if needed
        listing, err := uc.listingRepo.FindByID(tx, order.Items[0].ListingID)
        if err == nil && listing.FulfillmentType != domain.FulfillmentTypeNone {
            fulfillment := domain.NewFulfillment(
                order.OrganizationID,
                &order.ID,
                nil,
                listing.FulfillmentType,
                now,
            )
            if err := uc.fulfillmentRepo.Create(tx, fulfillment); err != nil {
                return err
            }
        }

        return uc.orderRepo.Update(tx, order)
    })
}
```

### Fulfillment Completion → Order Completion

```go
func (uc *FulfillmentUseCases) CompleteFulfillment(ctx context.Context, fulfillmentID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        fulfillment, err := uc.fulfillmentRepo.FindByIDForUpdate(tx, fulfillmentID)
        if err != nil {
            return err
        }
        if err := fulfillment.TransitionTo(domain.FulfillmentStatusCompleted, actorID, now); err != nil {
            return err
        }
        if err := uc.fulfillmentRepo.Update(tx, fulfillment); err != nil {
            return err
        }

        // If order-based, check if all fulfillments complete
        if fulfillment.OrderID != nil {
            order, err := uc.orderRepo.FindByIDForUpdate(tx, *fulfillment.OrderID)
            if err == nil && order.Status == domain.OrderStatusFulfilled {
                // Check other fulfillments
                fulfillments, _ := uc.fulfillmentRepo.FindByOrderID(tx, *fulfillment.OrderID)
                allComplete := true
                for _, f := range fulfillments {
                    if f.Status != domain.FulfillmentStatusCompleted {
                        allComplete = false
                        break
                    }
                }
                if allComplete {
                    order.Complete(now)
                    uc.orderRepo.Update(tx, order)
                }
            }
        }
        return nil
    })
}
```

---

## Reservation → Fulfillment Integration

### Appointment/Event Fulfillment

For time-based reservations, fulfillment created at confirmation:

```go
func (uc *ReservationUseCases) ConfirmReservation(ctx context.Context, reservationID string) error {
    return uc.txManager.RunInTx(ctx, func(tx *sql.Tx) error {
        reservation, err := uc.reservationRepo.FindByIDForUpdate(tx, reservationID)
        if err != nil {
            return err
        }
        now := time.Now().Unix()
        if err := reservation.Confirm(now); err != nil {
            return err
        }

        // Create fulfillment for appointment/event types
        listing, _ := uc.listingRepo.FindByID(tx, reservation.ListingID)
        if listing != nil {
            ftype := inferFulfillmentType(listing)
            if ftype != domain.FulfillmentTypeNone {
                fulfillment := domain.NewFulfillment(
                    reservation.OrganizationID,
                    nil,
                    &reservation.ID,
                    ftype,
                    now,
                )
                uc.fulfillmentRepo.Create(tx, fulfillment)
            }
        }

        return uc.reservationRepo.Update(tx, reservation)
    })
}

func inferFulfillmentType(listing *domain.Listing) domain.FulfillmentType {
    switch listing.ListingType {
    case domain.ListingTypeService:
        return domain.FulfillmentTypeAppointment
    case domain.ListingTypeRealEstate:
        return domain.FulfillmentTypePropertyVisit
    case domain.ListingTypeVehicle:
        return domain.FulfillmentTypePickup
    // Add more mappings
    default:
        return domain.FulfillmentTypeNone
    }
}
```

---

## Transaction Boundaries

| Operation | Tables Involved | Single TX? |
|-----------|----------------|------------|
| Order Confirm + Inventory Consume | `orders`, `inventory_resources` | ✅ Yes |
| Reservation Create + Capacity Reserve | `reservations`, `capacity_units` | ✅ Yes |
| Reservation Cancel + Capacity Release | `reservations`, `capacity_units` | ✅ Yes |
| Order Ready + Fulfillment Create | `orders`, `fulfillments` | ✅ Yes |
| Fulfillment Complete + Order Complete | `fulfillments`, `orders` | ✅ Yes |
| Inventory Adjust | `inventory_resources` | ✅ Yes |

All operations within single PostgreSQL database use single transaction.

---

## Cross-Boundary (Future)

For future microservice decomposition:
- Event-driven (outbox pattern)
- Saga for distributed transactions
- Not in Sprint 13 scope