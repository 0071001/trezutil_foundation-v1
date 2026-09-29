package payment

import (
	"context"
	"errors"

	"github.com/trezutil/trezutil/internal/domain"
)

// AccessPolicy decides who may act on payments. The organization in the
// request only scopes the lookup; authority always comes from the verified
// actor's relationship to the order (customer) or organization (staff).
type AccessPolicy struct {
	orders   domain.OrderRepository
	payments domain.PaymentRepository
	orgs     domain.OrganizationRepository
	users    domain.UserRepository
}

// NewAccessPolicy creates an AccessPolicy.
func NewAccessPolicy(orders domain.OrderRepository, payments domain.PaymentRepository, orgs domain.OrganizationRepository, users domain.UserRepository) *AccessPolicy {
	return &AccessPolicy{orders: orders, payments: payments, orgs: orgs, users: users}
}

// CanCreate allows the order's customer or the organization's staff to pay an order.
func (a *AccessPolicy) CanCreate(ctx context.Context, actorID, orgID, orderID string) error {
	order, err := a.orders.FindByIDAndOrg(ctx, orgID, orderID)
	if err != nil || order == nil {
		return domain.ErrOrderNotFound
	}
	if order.CustomerID == actorID || a.isStaff(ctx, actorID, orgID) {
		return nil
	}
	return domain.ErrInsufficientPermissions
}

// CanView allows the paying customer or staff to read a payment (and to cancel it
// before capture).
func (a *AccessPolicy) CanView(ctx context.Context, actorID, orgID, paymentID string) error {
	p, err := a.payments.FindByID(ctx, orgID, paymentID)
	if err != nil {
		return domain.ErrPaymentNotFound
	}
	if a.isStaff(ctx, actorID, orgID) {
		return nil
	}
	order, err := a.orders.FindByIDAndOrg(ctx, orgID, p.OrderID)
	if err == nil && order != nil && order.CustomerID == actorID {
		return nil
	}
	// Do not reveal whether the payment exists to unrelated callers.
	return domain.ErrPaymentNotFound
}

// CanManage allows only organization staff (or platform admins) to capture or
// refund money.
func (a *AccessPolicy) CanManage(ctx context.Context, actorID, orgID, paymentID string) error {
	if _, err := a.payments.FindByID(ctx, orgID, paymentID); err != nil {
		return domain.ErrPaymentNotFound
	}
	if a.isStaff(ctx, actorID, orgID) {
		return nil
	}
	return domain.ErrInsufficientPermissions
}

func (a *AccessPolicy) isStaff(ctx context.Context, actorID, orgID string) bool {
	if actorID == "" {
		return false
	}
	if a.users != nil {
		if user, err := a.users.FindByID(ctx, actorID); err == nil {
			if user.IsSuspended() {
				return false
			}
			if user.HasRole(domain.RoleAdmin) || user.HasRole(domain.RoleOperator) {
				return true
			}
		}
	}
	if a.orgs == nil || orgID == "" {
		return false
	}
	role, err := a.orgs.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return false
	}
	switch role {
	case domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager:
		return true
	}
	return false
}

// IsAccessDenied reports whether err is an authorization/visibility refusal.
func IsAccessDenied(err error) bool {
	return errors.Is(err, domain.ErrInsufficientPermissions)
}
