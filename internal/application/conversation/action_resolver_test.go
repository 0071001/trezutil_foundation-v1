package conversation

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestActionResolverResolve(t *testing.T) {
	validator := NewDefaultIntentValidator()
	resolver := NewActionResolver(validator)

	tests := []struct {
		name       string
		intent     domain.ConversationIntent
		actor      domain.CommerceActor
		wantErr    bool
		wantAction domain.CommerceActionType
	}{
		{
			name: "valid search intent",
			intent: func() domain.ConversationIntent {
				i := domain.NewConversationIntent("search", domain.ActionSearchListings)
				i.AddEntity(domain.ConversationEntity{Name: "query", Value: "Samsung"})
				return i
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantErr:    false,
			wantAction: domain.ActionSearchListings,
		},
		{
			name: "create order with required listing_id",
			intent: func() domain.ConversationIntent {
				i := domain.NewConversationIntent("buy", domain.ActionCreateOrder)
				i.AddEntity(domain.ConversationEntity{Name: "listing_id", Value: "listing-1"})
				return i
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantErr:    false,
			wantAction: domain.ActionCreateOrder,
		},
		{
			name: "create order missing required listing_id",
			intent: func() domain.ConversationIntent {
				i := domain.NewConversationIntent("buy", domain.ActionCreateOrder)
				return i
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantErr:    true,
			wantAction: domain.ActionCreateOrder,
		},
		{
			name: "unknown action type",
			intent: func() domain.ConversationIntent {
				return domain.NewConversationIntent("unknown", domain.CommerceActionType("UNKNOWN"))
			}(),
			actor:      domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
			wantErr:    false,
			wantAction: domain.CommerceActionType("UNKNOWN"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			action, err := resolver.Resolve(nil, tt.intent, tt.actor, "conv-1", "org-1")

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if action.ActionType != tt.wantAction {
				t.Errorf("expected %s, got %s", tt.wantAction, action.ActionType)
			}
			if action.ConversationID != "conv-1" {
				t.Errorf("expected conv-1, got %s", action.ConversationID)
			}
			if action.TenantID != "org-1" {
				t.Errorf("expected org-1, got %s", action.TenantID)
			}
			if action.Source != "AI" {
				t.Errorf("expected AI source, got %s", action.Source)
			}
		})
	}
}
