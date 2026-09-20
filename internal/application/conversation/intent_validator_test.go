package conversation

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestDefaultIntentValidatorValidateIntent(t *testing.T) {
	validator := NewDefaultIntentValidator()

	tests := []struct {
		name    string
		intent  domain.ConversationIntent
		wantErr bool
		errMsg  string
	}{
		{
			name: "search has no required params",
			intent: func() domain.ConversationIntent {
				return domain.NewConversationIntent("search", domain.ActionSearchListings)
			}(),
			wantErr: false,
		},
		{
			name: "get order with order_id",
			intent: func() domain.ConversationIntent {
				i := domain.NewConversationIntent("get", domain.ActionGetOrder)
				i.AddEntity(domain.ConversationEntity{Name: "order_id", Value: "order-1"})
				return i
			}(),
			wantErr: false,
		},
		{
			name: "get order missing order_id",
			intent: func() domain.ConversationIntent {
				return domain.NewConversationIntent("get", domain.ActionGetOrder)
			}(),
			wantErr: true,
			errMsg:  "order_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateIntent(nil, tt.intent)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errMsg != "" && err != nil && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDefaultIntentValidatorEntityConfidence(t *testing.T) {
	validator := NewDefaultIntentValidator()

	// Even with low confidence, the entity is present and validation passes.
	// Confidence is NOT a substitute for business validation.
	intent := domain.NewConversationIntent("buy", domain.ActionCreateOrder)
	intent.AddEntity(domain.ConversationEntity{
		Name:            "listing_id",
		Value:           "listing-1",
		NormalizedValue: "listing-1",
		Confidence:      0.3, // Low confidence
		Source:          "AI",
	})

	if err := validator.ValidateIntent(nil, intent); err != nil {
		t.Errorf("expected no error for low-confidence entity: %v", err)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
