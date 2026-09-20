package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// CancelOrderHandler handles CANCEL_ORDER actions.
type CancelOrderHandler struct {
	orderUseCases OrderUseCases
}

// NewCancelOrderHandler creates a new CancelOrderHandler.
func NewCancelOrderHandler(orderUseCases OrderUseCases) *CancelOrderHandler {
	return &CancelOrderHandler{orderUseCases: orderUseCases}
}

// Handle executes a cancel order action.
func (h *CancelOrderHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orderID, ok := parameters["order_id"].(string)
	if !ok || orderID == "" {
		return nil, domain.ErrOrderNotFound
	}

	if err := h.orderUseCases.CancelOrder(ctx, actor.ID, orderID); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"order_id": orderID,
		"status":   "cancelled",
	}, nil
}

// ActionType returns the action type this handler handles.
func (h *CancelOrderHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCancelOrder
}
