package commerce

import (
	"context"
	"fmt"
	"strings"

	"github.com/trezutil/trezutil/internal/application/vertical"
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

// InvalidParameterError reports a parameter that is present but unacceptable
// (an unknown vertical, a non-positive night count…). It wraps
// domain.ErrInvalidParameter.
type InvalidParameterError struct {
	Param  string
	Reason string
}

func (e *InvalidParameterError) Error() string {
	return fmt.Sprintf("parameter '%s' %s", e.Param, e.Reason)
}

func (e *InvalidParameterError) Unwrap() error { return domain.ErrInvalidParameter }

// Structured-parameter bounds.
const (
	maxNights    = 90
	maxPartySize = 100
)

// ParameterValidator validates action parameters before execution.
type ParameterValidator interface {
	Validate(ctx context.Context, actionType domain.CommerceActionType, parameters map[string]interface{}) error
}

// DefaultParameterValidator validates parameters based on action type
// requirements, and validates and normalizes the structured parameters
// (vertical, location, nights, party_size) whatever channel or interpreter
// produced them.
type DefaultParameterValidator struct {
	RequiredParams map[domain.CommerceActionType][]string
	// AnyOf lists parameters of which at least one must be present.
	AnyOf map[domain.CommerceActionType][]string
	// Verticals is the vertical catalogue; an unknown vertical is refused,
	// never mapped to OTHER.
	Verticals interface {
		Get(v domain.Vertical) (domain.VerticalDefinition, bool)
	}
}

// NewDefaultParameterValidator creates a DefaultParameterValidator with default required parameters.
func NewDefaultParameterValidator() *DefaultParameterValidator {
	return &DefaultParameterValidator{
		RequiredParams: map[domain.CommerceActionType][]string{
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
		// A search needs words, a vertical or a place ("a hotel in Gombe" is
		// vertical + location with no words left).
		AnyOf: map[domain.CommerceActionType][]string{
			domain.ActionSearchListings: {"text", "vertical", "location"},
		},
		Verticals: vertical.DefaultCatalog(),
	}
}

// Validate checks that required parameters are present and non-empty for the action type.
func (v *DefaultParameterValidator) Validate(ctx context.Context, actionType domain.CommerceActionType, parameters map[string]interface{}) error {
	required := v.RequiredParams[actionType]
	if parameters == nil {
		parameters = make(map[string]interface{})
	}
	if err := v.normalizeStructured(actionType, parameters); err != nil {
		return err
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
	if group := v.AnyOf[actionType]; len(group) > 0 {
		present := false
		for _, param := range group {
			if s, ok := parameters[param].(string); ok && strings.TrimSpace(s) != "" {
				present = true
			}
		}
		if !present {
			return &MissingParameterError{Param: group[0], ActionType: actionType, Reason: "is missing"}
		}
	}
	return nil
}

// normalizeStructured validates vertical, location, nights and party_size when
// present and rewrites them in canonical form: vertical as a catalogue code,
// location trimmed, counts as ints. For availability checks and bookings, a
// start with a night count and no end gets end = start + nights.
func (v *DefaultParameterValidator) normalizeStructured(actionType domain.CommerceActionType, p map[string]interface{}) error {
	if raw, ok := p["vertical"]; ok && raw != nil {
		s, isString := raw.(string)
		if !isString {
			return &InvalidParameterError{Param: "vertical", Reason: "must be a vertical code"}
		}
		if strings.TrimSpace(s) == "" {
			delete(p, "vertical")
		} else {
			code := domain.NormalizeVertical(s)
			known := code.IsWellFormed()
			if known && v.Verticals != nil {
				_, known = v.Verticals.Get(code)
			}
			if !known {
				return &InvalidParameterError{Param: "vertical", Reason: fmt.Sprintf("names an unknown vertical %q", s)}
			}
			p["vertical"] = string(code)
		}
	}
	if raw, ok := p["location"]; ok && raw != nil {
		s, isString := raw.(string)
		if !isString || len(strings.TrimSpace(s)) > domain.MaxSearchPlaceLength {
			return &InvalidParameterError{Param: "location", Reason: fmt.Sprintf("must be a place name of at most %d characters", domain.MaxSearchPlaceLength)}
		}
		if s = strings.TrimSpace(s); s == "" {
			delete(p, "location")
		} else {
			p["location"] = s
		}
	}
	for param, max := range map[string]int{"nights": maxNights, "party_size": maxPartySize} {
		if raw, ok := p[param]; ok && raw != nil {
			if s, isString := raw.(string); isString && strings.TrimSpace(s) == "" {
				delete(p, param)
				continue
			}
			n, isInt := intParam(p, param)
			if !isInt || n < 1 || n > max {
				return &InvalidParameterError{Param: param, Reason: fmt.Sprintf("must be a whole number from 1 to %d", max)}
			}
			p[param] = n
		}
	}
	if actionType == domain.ActionCheckAvailability || actionType == domain.ActionCreateReservation {
		nights, hasNights := p["nights"].(int)
		if _, hasEnd := p["end_at"]; !hasEnd && hasNights {
			if raw, ok := p["start_at"]; ok && raw != nil {
				if start, err := parseInt64(raw); err == nil && start > 0 {
					p["start_at"] = start
					p["end_at"] = start + int64(nights)*86400
				}
			}
		}
	}
	return nil
}
