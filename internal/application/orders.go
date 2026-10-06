package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// OrderUseCases handles order-related operations.
type OrderUseCases struct {
	orderRepo       domain.OrderRepository
	listingRepo     domain.ListingRepository
	sellerRepo      domain.SellerRepository
	orgRepo         domain.OrganizationRepository
	userRepo        domain.UserRepository
	inventoryRepo   domain.InventoryRepository
	fulfillmentRepo domain.FulfillmentRepository
	auditLogger     domain.AuditLogger
}

// NewOrderUseCases creates a new OrderUseCases.
func NewOrderUseCases(
	orderRepo domain.OrderRepository,
	listingRepo domain.ListingRepository,
	sellerRepo domain.SellerRepository,
	orgRepo domain.OrganizationRepository,
	userRepo domain.UserRepository,
	inventoryRepo domain.InventoryRepository,
	fulfillmentRepo domain.FulfillmentRepository,
	auditLogger domain.AuditLogger,
) *OrderUseCases {
	return &OrderUseCases{
		orderRepo:       orderRepo,
		listingRepo:     listingRepo,
		sellerRepo:      sellerRepo,
		orgRepo:         orgRepo,
		userRepo:        userRepo,
		inventoryRepo:   inventoryRepo,
		fulfillmentRepo: fulfillmentRepo,
		auditLogger:     auditLogger,
	}
}

// CreateOrder creates a new order from a listing and quantity.
func (u *OrderUseCases) CreateOrder(ctx context.Context, actorID string, listingID string, quantity int) (*domain.Order, error) {
	return u.CreateOrderIdempotent(ctx, actorID, listingID, quantity, "")
}

// CreateOrderIdempotent creates a new order; a retry with the same non-empty
// idempotency key by the same customer returns the original order.
func (u *OrderUseCases) CreateOrderIdempotent(ctx context.Context, actorID string, listingID string, quantity int, idempotencyKey string) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if listingID == "" {
		return nil, fmt.Errorf("listing_id is required")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	return u.CreateOrderWithLines(ctx, actorID, []domain.OrderLine{{ListingID: listingID, Quantity: quantity}}, idempotencyKey)
}

// GetOrder retrieves an order by ID with authorization.
func (u *OrderUseCases) GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orderID == "" {
		return nil, domain.ErrOrderNotFound
	}

	order, err := u.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}

	if !u.canAccessOrder(ctx, order, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	return order, nil
}

// CancelOrder cancels an order by ID.
func (u *OrderUseCases) CancelOrder(ctx context.Context, actorID string, orderID string) error {
	if actorID == "" {
		return domain.ErrInsufficientPermissions
	}
	if orderID == "" {
		return domain.ErrOrderNotFound
	}

	order, err := u.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return domain.ErrOrderNotFound
	}

	if !u.canAccessOrder(ctx, order, actorID) {
		return domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	if order.Status != domain.OrderStatusDraft && order.Status != domain.OrderStatusPendingPayment {
		return domain.ErrOrderCannotBeCancelled
	}

	if err := order.Cancel(now); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	if err := u.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("failed to cancel order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order.cancelled",
			TargetType: "order",
			TargetID:   orderID,
			Metadata: map[string]interface{}{
				"previous_status": string(order.Status),
			},
		})
	}

	return nil
}

// canCreateOrder checks if an actor can place an order with a tenant.
// Any active (verified, non-suspended) user may buy from any organization;
// the listing itself must be published (checked by the caller).
func (u *OrderUseCases) canCreateOrder(ctx context.Context, actor *domain.User, tenantID string) bool {
	return actor != nil && actor.IsActive()
}

// canAccessOrder checks if an actor can access an order.
func (u *OrderUseCases) canAccessOrder(ctx context.Context, order *domain.Order, actorID string) bool {
	if order.CustomerID == actorID {
		return true
	}

	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err == nil && (actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		return true
	}

	seller, err := u.sellerRepo.FindByUserID(ctx, actorID)
	if err == nil && seller != nil && seller.ID == order.SellerID {
		return true
	}

	if order.OrganizationID != "" {
		memberRole, err := u.orgRepo.GetMemberRole(ctx, order.OrganizationID, actorID)
		if err == nil {
			if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
				return true
			}
		}
	}

	return false
}

// canManageOrder is the seller side of an order: the seller profile owner,
// the organization's owner/admin/manager, or a platform admin/operator. Unlike
// canAccessOrder it excludes the buyer, who must not move their own order
// through seller-only transitions (confirm, ready, complete).
func (u *OrderUseCases) canManageOrder(ctx context.Context, order *domain.Order, actorID string) bool {
	if actor, err := u.userRepo.FindByID(ctx, actorID); err == nil && !actor.IsSuspended() &&
		(actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		return true
	}
	if seller, err := u.sellerRepo.FindByUserID(ctx, actorID); err == nil && seller != nil && seller.ID == order.SellerID {
		return true
	}
	if order.OrganizationID != "" {
		if role, err := u.orgRepo.GetMemberRole(ctx, order.OrganizationID, actorID); err == nil {
			return role == domain.OrganizationRoleOwner || role == domain.OrganizationRoleAdmin || role == domain.OrganizationRoleManager
		}
	}
	return false
}

// ConfirmOrder confirms a paid order and consumes inventory.
// This transitions the order from PAID to CONFIRMED and consumes inventory for each item.
func (u *OrderUseCases) ConfirmOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orderID == "" {
		return nil, domain.ErrOrderNotFound
	}

	order, err := u.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}

	if !u.canManageOrder(ctx, order, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	if order.Status != domain.OrderStatusPaid {
		return nil, errOrderState("cannot confirm order from status %s: only paid orders can be confirmed", order.Status)
	}

	now := time.Now().Unix()

	// Consume inventory for each item
	for _, item := range order.Items {
		inv, err := u.inventoryRepo.FindByListingIDForUpdate(ctx, order.OrganizationID, item.ListingID)
		if err != nil {
			// If no inventory resource exists, skip (non-inventory-controlled)
			if err == domain.ErrInventoryNotFound {
				continue
			}
			return nil, err
		}

		if err := inv.Consume(int64(item.Quantity), now); err != nil {
			return nil, fmt.Errorf("failed to consume inventory for listing %s: %w", item.ListingID, err)
		}

		if err := u.inventoryRepo.Update(ctx, inv); err != nil {
			return nil, fmt.Errorf("failed to update inventory for listing %s: %w", item.ListingID, err)
		}
	}

	// Confirm order
	if err := order.Confirm(now); err != nil {
		return nil, err
	}

	if err := u.orderRepo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to confirm order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order.confirmed",
			TargetType: "order",
			TargetID:   orderID,
			Metadata: map[string]interface{}{
				"previous_status": string(domain.OrderStatusPaid),
				"new_status":      string(domain.OrderStatusConfirmed),
			},
		})
	}

	return order, nil
}

// errOrderState reports a step the order's current status does not allow (a conflict, not a fault).
func errOrderState(format string, args ...any) error {
	return fmt.Errorf("%w: %s", domain.ErrInvalidOrderTransition, fmt.Sprintf(format, args...))
}

// advanceOrder moves an order one step along the seller-side lifecycle:
// paid → confirmed → processing → ready_for_fulfillment → fulfilled → completed.
// Only the seller side may do it (canManageOrder: never the buyer), only from the one status the step
// starts from, and the order's version guards against two clicks racing: the second one fails with
// ErrOptimisticLock instead of repeating the step. after runs once the order has been saved.
func (u *OrderUseCases) advanceOrder(
	ctx context.Context, actorID, orderID string,
	from, to domain.OrderStatus, action string,
	apply func(order *domain.Order, now int64) error,
	after func(order *domain.Order, now int64) error,
) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orderID == "" {
		return nil, domain.ErrOrderNotFound
	}

	order, err := u.orderRepo.FindByID(ctx, orderID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}

	if !u.canManageOrder(ctx, order, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	if order.Status != from {
		return nil, errOrderState("cannot %s an order that is %s: only %s orders can", action, order.Status, from)
	}

	now := time.Now().Unix()
	if err := apply(order, now); err != nil {
		return nil, err
	}
	if err := u.orderRepo.Update(ctx, order); err != nil {
		if errors.Is(err, domain.ErrOptimisticLock) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order." + string(to),
			TargetType: "order",
			TargetID:   orderID,
			Metadata: map[string]interface{}{
				"previous_status": string(from),
				"new_status":      string(to),
			},
		})
	}

	if after != nil {
		if err := after(order, now); err != nil {
			return order, err
		}
	}
	return order, nil
}

// StartProcessing moves a confirmed order to PROCESSING: the seller has started preparing it.
func (u *OrderUseCases) StartProcessing(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	return u.advanceOrder(ctx, actorID, orderID,
		domain.OrderStatusConfirmed, domain.OrderStatusProcessing, "start preparing",
		func(o *domain.Order, now int64) error { return o.Process(now) }, nil)
}

// ReadyForFulfillment transitions order to READY_FOR_FULFILLMENT and creates the fulfillment record.
// The record is created after the order is saved, so of two clicks racing only the one that won the
// order's version creates it.
func (u *OrderUseCases) ReadyForFulfillment(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	return u.advanceOrder(ctx, actorID, orderID,
		domain.OrderStatusProcessing, domain.OrderStatusReadyForFulfillment, "mark ready",
		func(o *domain.Order, now int64) error { return o.ReadyForFulfillment(now) },
		func(order *domain.Order, now int64) error {
			if len(order.Items) == 0 || u.fulfillmentRepo == nil {
				return nil
			}
			listing, err := u.listingRepo.FindByID(ctx, order.Items[0].ListingID)
			if err != nil {
				return nil
			}
			fulfillmentType := u.inferFulfillmentType(listing)
			if fulfillmentType == domain.FulfillmentTypeNone {
				return nil
			}
			fulfillment := domain.NewFulfillment(order.OrganizationID, &order.ID, nil, fulfillmentType, now)
			if err := u.fulfillmentRepo.Create(ctx, fulfillment); err != nil {
				return fmt.Errorf("order is ready, but its fulfillment record could not be created: %w", err)
			}
			return nil
		})
}

// FulfilOrder moves a ready order to FULFILLED: it has been handed over or delivered.
func (u *OrderUseCases) FulfilOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	return u.advanceOrder(ctx, actorID, orderID,
		domain.OrderStatusReadyForFulfillment, domain.OrderStatusFulfilled, "mark delivered",
		func(o *domain.Order, now int64) error { return o.Fulfil(now) }, nil)
}

// CompleteOrder completes a fulfilled order.
func (u *OrderUseCases) CompleteOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	return u.advanceOrder(ctx, actorID, orderID,
		domain.OrderStatusFulfilled, domain.OrderStatusCompleted, "complete",
		func(o *domain.Order, now int64) error { return o.Complete(now) }, nil)
}

func (u *OrderUseCases) inferFulfillmentType(listing *domain.Listing) domain.FulfillmentType {
	// Simple inference based on listing type
	// This should be made configurable in the future
	return domain.FulfillmentTypeDelivery
}
