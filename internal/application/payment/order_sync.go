package payment

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// submitOrderForPayment moves a draft order to pending_payment when a payment
// is initiated for it. Orders already past draft are left untouched.
func submitOrderForPayment(ctx context.Context, orders domain.OrderRepository, orgID, orderID string) error {
	if orders == nil {
		return nil
	}
	return retryOnConflict(func() error {
		order, err := orders.FindByIDAndOrg(ctx, orgID, orderID)
		if err != nil {
			return err
		}
		if order.Status != domain.OrderStatusDraft {
			return nil
		}
		if err := order.Submit(); err != nil {
			return err
		}
		return orders.Update(ctx, order)
	})
}

// syncOrderWithPayment keeps the order lifecycle consistent with its payment:
// a captured payment marks the order paid, a fully refunded payment marks a
// paid/confirmed order refunded. It is idempotent and safe to call from both
// the synchronous API path and provider webhooks. Failures are logged, not
// returned: the payment and ledger are the financial source of truth and must
// not be rolled back because the order projection lagged.
func syncOrderWithPayment(ctx context.Context, orders domain.OrderRepository, p *domain.Payment, logger zerolog.Logger) {
	if orders == nil || p == nil || p.OrderID == "" {
		return
	}
	err := retryOnConflict(func() error {
		order, err := orders.FindByIDAndOrg(ctx, p.TenantID, p.OrderID)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		switch {
		case p.Status == domain.PaymentStatusRefunded:
			if order.Status != domain.OrderStatusPaid && order.Status != domain.OrderStatusConfirmed {
				return nil
			}
			if err := order.Refund(now); err != nil {
				return err
			}
			return orders.Update(ctx, order)

		case p.IsCaptured():
			if order.IsPaid() || order.Status == domain.OrderStatusRefunded {
				return nil
			}
			if order.Status == domain.OrderStatusDraft {
				if err := order.Submit(); err != nil {
					return err
				}
				if err := orders.Update(ctx, order); err != nil {
					return err
				}
			}
			if order.Status != domain.OrderStatusPendingPayment {
				// e.g. the order was cancelled/expired while the payment was in
				// flight: money was captured for an order that cannot be paid.
				logger.Error().Str("order", order.ID).Str("payment", p.ID).Str("order_status", string(order.Status)).
					Msg("payment captured for an order that is no longer payable; refund required")
				return nil
			}
			if err := order.ConfirmPayment(p.ProviderID, p.ProviderPaymentID, p.Amount, now); err != nil {
				return err
			}
			return orders.Update(ctx, order)
		}
		return nil
	})
	if err != nil {
		logger.Warn().Err(err).Str("order", p.OrderID).Str("payment", p.ID).Msg("failed to sync order with payment")
	}
}

// retryOnConflict re-runs fn a few times when it hits an optimistic lock.
func retryOnConflict(fn func() error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		if err = fn(); !errors.Is(err, domain.ErrOptimisticLock) {
			return err
		}
	}
	return err
}
