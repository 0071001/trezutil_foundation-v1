package commerce

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// Handler handles a specific commerce action type.
type Handler interface {
	Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error)
	ActionType() domain.CommerceActionType
}

// HandlerRegistry maps action types to their handlers.
type HandlerRegistry struct {
	handlers map[domain.CommerceActionType]Handler
}

// NewHandlerRegistry creates a new HandlerRegistry.
func NewHandlerRegistry() *HandlerRegistry {
	return &HandlerRegistry{
		handlers: make(map[domain.CommerceActionType]Handler),
	}
}

// Register registers a handler for an action type.
func (r *HandlerRegistry) Register(handler Handler) {
	r.handlers[handler.ActionType()] = handler
}

// Get returns the handler for an action type, or nil if not registered.
func (r *HandlerRegistry) Get(actionType domain.CommerceActionType) (Handler, bool) {
	h, ok := r.handlers[actionType]
	return h, ok
}

// Has returns true if a handler is registered for the action type.
func (r *HandlerRegistry) Has(actionType domain.CommerceActionType) bool {
	_, ok := r.handlers[actionType]
	return ok
}
