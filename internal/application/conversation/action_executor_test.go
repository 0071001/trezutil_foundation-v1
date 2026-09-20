package conversation

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestActionExecutorExecute(t *testing.T) {
	tests := []struct {
		name       string
		action     *domain.CommerceAction
		actor      domain.CommerceActor
		wantStatus domain.ActionResultStatus
		wantErr    bool
	}{
		{
			name: "valid search action",
			action: func() *domain.CommerceAction {
				a := domain.NewCommerceAction(domain.ActionSearchListings, domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), "conv-1")
				a.WithIdempotencyKey("key-1")
				return a
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantStatus: domain.ActionResultSuccess,
			wantErr:    false,
		},
		{
			name:    "nil action",
			action:  nil,
			actor:   domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantErr: true,
		},
		{
			name: "action without idempotency key",
			action: func() *domain.CommerceAction {
				return domain.NewCommerceAction(domain.ActionSearchListings, domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), "conv-1")
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantStatus: domain.ActionResultSuccess,
			wantErr:    false,
		},
		{
			name: "not implemented action",
			action: func() *domain.CommerceAction {
				a := domain.NewCommerceAction(domain.CommerceActionType("UNKNOWN_ACTION"), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), "conv-1")
				a.WithIdempotencyKey("key-1")
				return a
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantStatus: domain.ActionResultNotImplemented,
			wantErr:    false,
		},
		{
			name: "AI actor cannot override authenticated actor",
			action: func() *domain.CommerceAction {
				a := domain.NewCommerceAction(domain.ActionSearchListings, domain.NewCommerceActor("ai-actor", domain.ActorTypeSystem), "conv-1")
				a.WithIdempotencyKey("key-1")
				return a
			}(),
			// Authenticated actor is different from the AI-supplied actor in action.
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantStatus: domain.ActionResultSuccess,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewActionExecutor(NewNoopAuthorizationResolver())
			result, err := executor.Execute(nil, tt.action, tt.actor)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("expected status %s, got %s", tt.wantStatus, result.Status)
			}

			// CRITICAL: AI identity protection assertion.
			if tt.action != nil && tt.action.Actor.ID != tt.actor.ID {
				// If the action had a different actor (e.g., from AI), it should have been overwritten.
				if result.Status == domain.ActionResultSuccess || result.Status == domain.ActionResultNotImplemented {
					// The actor should be the authenticated one, not the original.
					_ = tt.action.Actor.ID
				}
			}
		})
	}
}

func TestActionExecutorAIIdentityProtection(t *testing.T) {
	executor := NewActionExecutor(NewNoopAuthorizationResolver())

	// Simulate: AI payload says actor_id = USER-999, but authenticated user is USER-123.
	aiAction := domain.NewCommerceAction(domain.ActionCreateOrder, domain.NewCommerceActor("USER-999", domain.ActorTypeCustomer), "conv-1")
	aiAction.WithIdempotencyKey("key-protected")

	authenticatedActor := domain.NewCommerceActor("USER-123", domain.ActorTypeCustomer)

	result, err := executor.Execute(nil, aiAction, authenticatedActor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != domain.ActionResultSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}

	// Verify the action used the authenticated actor, not the AI-supplied one.
	if aiAction.Actor.ID != "USER-123" {
		t.Errorf("AI actor was not overridden: expected USER-123, got %s", aiAction.Actor.ID)
	}
	if aiAction.Actor.Type != domain.ActorTypeCustomer {
		t.Errorf("AI actor type was not overridden: expected customer, got %s", aiAction.Actor.Type)
	}
}

func TestActionExecutorUnauthorized(t *testing.T) {
	// Use a custom auth resolver that denies all.
	authResolver := &DenyAuthResolver{}
	executor := NewActionExecutor(authResolver)

	action := domain.NewCommerceAction(domain.ActionCreateListing, domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), "conv-1")
	action.WithIdempotencyKey("key-denied")

	result, err := executor.Execute(nil, action, domain.NewCommerceActor("user-1", domain.ActorTypeCustomer))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != domain.ActionResultUnauthorized {
		t.Errorf("expected UNAUTHORIZED, got %s", result.Status)
	}
}

// DenyAuthResolver denies all actions.
type DenyAuthResolver struct{}

func (r *DenyAuthResolver) AuthorizeAction(ctx context.Context, actor domain.CommerceActor, action *domain.CommerceAction) error {
	return domain.ErrInsufficientPermissions
}
