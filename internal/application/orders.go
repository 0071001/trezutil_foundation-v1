package application

import (
	"context"
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
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if listingID == "" {
		return nil, fmt.Errorf("listing_id is required")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}

	listing, err := u.listingRepo.FindByID(ctx, listingID)
	if err != nil {
		return nil, domain.ErrOrderNotFound
	}

	tenantID := listing.OrganizationID
	if tenantID == "" {
		return nil, fmt.Errorf("listing has no organization")
	}

	actor, err := u.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to find actor: %w", err)
	}

	if !u.canCreateOrder(ctx, actor, tenantID) {
		return nil, domain.ErrInsufficientPermissions
	}

	now := time.Now().Unix()
	unitPrice := domain.NewMoney(int64(listing.PriceAmount*100), listing.PriceCurrency)

	item := domain.NewOrderItem("", listingID, listing.Title, quantity, unitPrice, now)

	pricingSnapshot := domain.NewPricingSnapshot(
		[]domain.OrderItem{*item},
		domain.NewMoney(0, listing.PriceCurrency),
		domain.NewMoney(0, listing.PriceCurrency),
		now,
	)

	order := domain.NewOrder(tenantID, listing.SellerID, actorID, listing.StoreID, []*domain.OrderItem{item}, listing.PriceCurrency, now)
	order.PricingSnapshot = pricingSnapshot

	if err := u.orderRepo.Create(ctx, order); err != nil {
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
				"listing_id":      listingID,
				"quantity":        quantity,
				"total_amount":    order.TotalAmount,
			},
		})
	}

	return order, nil
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

// canCreateOrder checks if an actor can create orders for a tenant.
func (u *OrderUseCases) canCreateOrder(ctx context.Context, actor *domain.User, tenantID string) bool {
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}

	memberRole, err := u.orgRepo.GetMemberRole(ctx, tenantID, actor.ID)
	if err == nil {
		if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
			return true
		}
	}

	seller, err := u.sellerRepo.FindByUserID(ctx, actor.ID)
	if err == nil && seller != nil {
		if seller.UserID != nil && *seller.UserID == actor.ID {
			return true
		}
	}

	return false
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

	if !u.canAccessOrder(ctx, order, actorID) {
		return nil, domain.ErrInsufficientPermissions
	}

	if order.Status != domain.OrderStatusPaid {
		return nil, fmt.Errorf("cannot confirm order from status %s: only paid orders can be confirmed", order.Status)
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

// ReadyForFulfillment transitions order to READY_FOR_FULFILLMENT and creates fulfillment if needed.
func (u *OrderUseCases) ReadyForFulfillment(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
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

	if order.Status != domain.OrderStatusProcessing {
		return nil, fmt.Errorf("cannot set ready for fulfillment from status %s: only processing orders can be ready", order.Status)
	}

	now := time.Now().Unix()

	// Transition order
	if err := order.ReadyForFulfillment(now); err != nil {
		return nil, err
	}

	// Create fulfillment if needed (for first item's listing)
	if len(order.Items) > 0 && u.fulfillmentRepo != nil {
		listing, err := u.listingRepo.FindByID(ctx, order.Items[0].ListingID)
		if err == nil {
			// Infer fulfillment type from listing
			fulfillmentType := u.inferFulfillmentType(listing)
			if fulfillmentType != domain.FulfillmentTypeNone {
				fulfillment := domain.NewFulfillment(
					order.OrganizationID,
					&order.ID,
					nil,
					fulfillmentType,
					now,
				)
				if err := u.fulfillmentRepo.Create(ctx, fulfillment); err != nil {
					return nil, fmt.Errorf("failed to create fulfillment: %w", err)
				}
			}
		}
	}

	if err := u.orderRepo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order.ready_for_fulfillment",
			TargetType: "order",
			TargetID:   orderID,
			Metadata: map[string]interface{}{
				"previous_status": string(domain.OrderStatusProcessing),
				"new_status":      string(domain.OrderStatusReadyForFulfillment),
			},
		})
	}

	return order, nil
}

// CompleteOrder completes a fulfilled order.
func (u *OrderUseCases) CompleteOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
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

	if order.Status != domain.OrderStatusFulfilled {
		return nil, fmt.Errorf("cannot complete order from status %s: only fulfilled orders can be completed", order.Status)
	}

	now := time.Now().Unix()

	if err := order.Complete(now); err != nil {
		return nil, err
	}

	if err := u.orderRepo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to complete order: %w", err)
	}

	if u.auditLogger != nil {
		u.auditLogger.Record(ctx, domain.AuditEntry{
			ActorID:    actorID,
			Action:     "order.completed",
			TargetType: "order",
			TargetID:   orderID,
			Metadata: map[string]interface{}{
				"previous_status": string(domain.OrderStatusFulfilled),
				"new_status":      string(domain.OrderStatusCompleted),
			},
		})
	}

	return order, nil
}

func (u *OrderUseCases) inferFulfillmentType(listing *domain.Listing) domain.FulfillmentType {
	// Simple inference based on listing type
	// This should be made configurable in the future
	return domain.FulfillmentTypeDelivery
}
