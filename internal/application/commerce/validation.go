package commerce

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ParameterValidator validates action parameters before execution.
type ParameterValidator interface {
	Validate(ctx context.Context, actionType domain.CommerceActionType, parameters map[string]interface{}) error
}

// DefaultParameterValidator validates parameters based on action type requirements.
type DefaultParameterValidator struct {
	RequiredParams map[domain.CommerceActionType][]string
}

// NewDefaultParameterValidator creates a DefaultParameterValidator with default required parameters.
func NewDefaultParameterValidator() *DefaultParameterValidator {
	return &DefaultParameterValidator{
		RequiredParams: map[domain.CommerceActionType][]string{
			domain.ActionSearchListings:    {"text"},
			domain.ActionGetListing:        {"listing_id"},
			domain.ActionPublishListing:    {"listing_id"},
			domain.ActionPauseListing:      {"listing_id"},
			domain.ActionCreateListing:     {"listing"},
			domain.ActionCreateOrder:       {"listing_id", "quantity"},
			domain.ActionCancelOrder:       {"order_id"},
			domain.ActionCheckAvailability: {"listing_id", "start_at", "end_at"},
			domain.ActionCreateReservation: {"listing_id", "start_at", "end_at"},
			domain.ActionGetReservation:    {"reservation_id"},
			domain.ActionCancelReservation: {"reservation_id"},
		},
	}
}

// Validate checks that required parameters are present and non-empty for the action type.
func (v *DefaultParameterValidator) Validate(ctx context.Context, actionType domain.CommerceActionType, parameters map[string]interface{}) error {
	required, exists := v.RequiredParams[actionType]
	if !exists {
		return nil
	}
	if parameters == nil {
		parameters = make(map[string]interface{})
	}

	for _, param := range required {
		val, ok := parameters[param]
		if !ok || val == nil {
			return fmt.Errorf("required parameter '%s' is missing for action %s", param, actionType)
		}
		switch v := val.(type) {
		case string:
			if v == "" {
				return fmt.Errorf("required parameter '%s' must not be empty for action %s", param, actionType)
			}
		case int:
			if v <= 0 {
				return fmt.Errorf("required parameter '%s' must be positive for action %s", param, actionType)
			}
		}
	}
	return nil
}
