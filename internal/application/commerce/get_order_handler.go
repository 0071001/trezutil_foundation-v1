package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// GetOrderHandler handles GET_ORDER actions.
type GetOrderHandler struct {
	orderUseCases OrderUseCases
}

// NewGetOrderHandler creates a new GetOrderHandler.
func NewGetOrderHandler(orderUseCases OrderUseCases) *GetOrderHandler {
	return &GetOrderHandler{orderUseCases: orderUseCases}
}

// Handle executes a get order action.
func (h *GetOrderHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orderID, ok := parameters["order_id"].(string)
	if !ok || orderID == "" {
		return nil, domain.ErrOrderNotFound
	}

	order, err := h.orderUseCases.GetOrder(ctx, actor.ID, orderID)
	if err != nil {
		return nil, err
	}

	return order, nil
}

// ActionType returns the action type this handler handles.
func (h *GetOrderHandler) ActionType() domain.CommerceActionType {
	return domain.ActionGetOrder
}
