package conversation

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

// AuthorizationResolver defines the interface for action authorization.
type AuthorizationResolver interface {
	AuthorizeAction(ctx context.Context, actor domain.CommerceActor, action *domain.CommerceAction) error
}

// NoopAuthorizationResolver allows all actions (for testing).
type NoopAuthorizationResolver struct{}

// NewNoopAuthorizationResolver creates a new NoopAuthorizationResolver.
func NewNoopAuthorizationResolver() *NoopAuthorizationResolver {
	return &NoopAuthorizationResolver{}
}

// AuthorizeAction implements AuthorizationResolver.
func (r *NoopAuthorizationResolver) AuthorizeAction(ctx context.Context, actor domain.CommerceActor, action *domain.CommerceAction) error {
	return nil
}
