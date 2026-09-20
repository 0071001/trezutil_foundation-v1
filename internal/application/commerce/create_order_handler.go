package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// CreateOrderHandler handles CREATE_ORDER actions.
type CreateOrderHandler struct {
	orderUseCases OrderUseCases
}

// NewCreateOrderHandler creates a new CreateOrderHandler.
func NewCreateOrderHandler(orderUseCases OrderUseCases) *CreateOrderHandler {
	return &CreateOrderHandler{orderUseCases: orderUseCases}
}

// Handle executes a create order action.
func (h *CreateOrderHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	listingID, ok := parameters["listing_id"].(string)
	if !ok || listingID == "" {
		return nil, domain.ErrOrderNotFound
	}

	quantity := 1
	if v, ok := parameters["quantity"].(int); ok && v > 0 {
		quantity = v
	}

	order, err := h.orderUseCases.CreateOrder(ctx, actor.ID, listingID, quantity)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order_id":    order.ID,
		"status":      string(order.Status),
		"total":       order.TotalAmount,
		"currency":    order.Currency,
		"customer_id": order.CustomerID,
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *CreateOrderHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCreateOrder
}
