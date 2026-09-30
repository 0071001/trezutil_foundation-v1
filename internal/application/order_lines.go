package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// CreateOrderWithLines creates one order holding several items from a single
// store (a cart). A retry with the same non-empty idempotency key by the same
// customer returns the original order when the items match, and conflicts
// otherwise.
func (u *OrderUseCases) CreateOrderWithLines(ctx context.Context, actorID string, lines []domain.OrderLine, idempotencyKey string) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	lines, err := domain.NormalizeOrderLines(lines)
	if err != nil {
		return nil, err
	}

	if idempotencyKey != "" {
		if existing, err := u.orderRepo.FindByCustomerIdempotencyKey(ctx, actorID, idempotencyKey); err == nil {
			if !existing.SameLines(lines) {
				return nil, domain.ErrIdempotencyConflict
			}
			return existing, nil
		} else if !errors.Is(err, domain.ErrOrderNotFound) {
			return nil, err
		}
	}

	now := time.Now().Unix()
	var (
		tenantID, storeID, sellerID, currency string
		items                                 []*domain.OrderItem
		snapshot                              []domain.OrderItem
	)
	for i, line := range lines {
		listing, err := u.listingRepo.FindByID(ctx, line.ListingID)
		if err != nil {
			return nil, domain.ErrOrderNotFound
		}
		if listing.OrganizationID == "" {
			return nil, fmt.Errorf("listing has no organization")
		}
		if i == 0 {
			tenantID, storeID, sellerID, currency = listing.OrganizationID, listing.StoreID, listing.SellerID, listing.PriceCurrency
		} else if listing.OrganizationID != tenantID || listing.StoreID != storeID || listing.PriceCurrency != currency {
			return nil, domain.ErrOrderMixedStores
		}
		// Only live listings can be purchased.
		if !listing.IsPublished() {
			return nil, domain.ErrListingUnavailable
		}
		// Listing prices are stored as decimal major units; round to the
		// nearest minor unit so 19.99 becomes 1999, not a truncated 1998.
		unit := domain.NewMoney(int64(math.Round(listing.PriceAmount*100)), listing.PriceCurrency)
		item := domain.NewOrderItem("", line.ListingID, listing.Title, line.Quantity, unit, now)
		items = append(items, item)
		snapshot = append(snapshot, *item)
	}

	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}
	if !u.canCreateOrder(ctx, actor, tenantID) {
		return nil, domain.ErrInsufficientPermissions
	}

	order := domain.NewOrder(tenantID, sellerID, actorID, storeID, items, currency, now)
	for _, it := range items {
		it.OrderID = order.ID
	}
	order.PricingSnapshot = domain.NewPricingSnapshot(snapshot,
		domain.NewMoney(0, currency), domain.NewMoney(0, currency), now)
	order.IdempotencyKey = idempotencyKey

	if err := u.orderRepo.Create(ctx, order); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) && idempotencyKey != "" {
			// Lost a race with a concurrent retry: return the winner.
			if existing, ferr := u.orderRepo.FindByCustomerIdempotencyKey(ctx, actorID, idempotencyKey); ferr == nil {
				return existing, nil
			}
		}
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order.created",
			TargetType: "order",
			TargetID:   order.ID,
			Metadata: map[string]interface{}{
				"organization_id": tenantID,
				"items":           len(items),
				"total_amount":    order.TotalAmount,
			},
		})
	}
	return order, nil
}
