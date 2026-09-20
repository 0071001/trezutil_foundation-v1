package conversation

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ActionResolver resolves a conversation intent to a commerce action.
type ActionResolver struct {
	intentValidator IntentValidator
}

// IntentValidator validates that an intent can be resolved to an action.
type IntentValidator interface {
	ValidateIntent(ctx context.Context, intent domain.ConversationIntent) error
}

// DefaultIntentValidator validates that required parameters are present.
type DefaultIntentValidator struct {
	RequiredParams map[domain.CommerceActionType][]string
}

// NewDefaultIntentValidator creates a new DefaultIntentValidator.
func NewDefaultIntentValidator() *DefaultIntentValidator {
	return &DefaultIntentValidator{
		RequiredParams: map[domain.CommerceActionType][]string{
			domain.ActionCreateOrder:    {"listing_id"},
			domain.ActionGetOrder:       {"order_id"},
			domain.ActionCancelOrder:    {"order_id"},
			domain.ActionCreateListing:  {"listing"},
			domain.ActionPublishListing: {"listing_id"},
			domain.ActionPauseListing:   {"listing_id"},
			domain.ActionResumeListing:  {"listing_id"},
		},
	}
}

// ValidateIntent validates that required parameters are present for the intent's action type.
func (v *DefaultIntentValidator) ValidateIntent(ctx context.Context, intent domain.ConversationIntent) error {
	required, exists := v.RequiredParams[intent.ActionType]
	if !exists {
		return nil
	}

	for _, param := range required {
		found := false
		for _, entity := range intent.Entities {
			if entity.Name == param && entity.Value != "" {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("required parameter %s is missing for action %s", param, intent.ActionType)
		}
	}
	return nil
}

// NewActionResolver creates a new ActionResolver.
func NewActionResolver(intentValidator IntentValidator) *ActionResolver {
	return &ActionResolver{intentValidator: intentValidator}
}

// Resolve converts a conversation intent into a commerce action.
func (r *ActionResolver) Resolve(
	ctx context.Context,
	intent domain.ConversationIntent,
	authenticatedActor domain.CommerceActor,
	conversationID string,
	tenantID string,
) (*domain.CommerceAction, error) {
	if intent.ActionType == "" {
		return nil, fmt.Errorf("intent has no action type")
	}

	// Validate intent (e.g., entity completeness).
	if r.intentValidator != nil {
		if err := r.intentValidator.ValidateIntent(ctx, intent); err != nil {
			return nil, err
		}
	}

	action := domain.NewCommerceAction(intent.ActionType, authenticatedActor, conversationID)
	action.WithTenant(tenantID)
	action.WithSource("AI")

	// Set parameters from intent entities.
	for _, entity := range intent.Entities {
		action.Parameters[entity.Name] = entity.Value
	}

	return action, nil
}
