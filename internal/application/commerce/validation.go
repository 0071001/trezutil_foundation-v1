package commerce

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// MissingParameterError reports a required parameter that is absent or empty.
// The conversation layer uses Param to ask the user for exactly that value.
type MissingParameterError struct {
	Param      string
	ActionType domain.CommerceActionType
	Reason     string
}

func (e *MissingParameterError) Error() string {
	return fmt.Sprintf("required parameter '%s' %s for action %s", e.Param, e.Reason, e.ActionType)
}

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
			return &MissingParameterError{Param: param, ActionType: actionType, Reason: "is missing"}
		}
		switch v := val.(type) {
		case string:
			if v == "" {
				return &MissingParameterError{Param: param, ActionType: actionType, Reason: "must not be empty"}
			}
		case int:
			if v <= 0 {
				return &MissingParameterError{Param: param, ActionType: actionType, Reason: "must be positive"}
			}
		case float64:
			if v <= 0 {
				return &MissingParameterError{Param: param, ActionType: actionType, Reason: "must be positive"}
			}
		}
	}
	return nil
}
