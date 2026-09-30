package commerce

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestParameterValidator_StructuredStayParameters(t *testing.T) {
	v := NewDefaultParameterValidator()
	p := map[string]interface{}{"vertical": " hotel ", "location": "  Gombe ", "nights": "2", "party_size": float64(3)}
	if err := v.Validate(context.Background(), domain.ActionSearchListings, p); err != nil {
		t.Fatalf("valid stay search: %v", err)
	}
	if p["vertical"] != "HOTEL" || p["location"] != "Gombe" || p["nights"] != 2 || p["party_size"] != 3 {
		t.Fatalf("not normalized: %v", p)
	}
}

func TestParameterValidator_RejectsInvalidStayParameters(t *testing.T) {
	v := NewDefaultParameterValidator()
	for name, p := range map[string]map[string]interface{}{
		"unknown vertical":    {"vertical": "CASINO"},
		"malformed vertical":  {"vertical": "hotel rooms!"},
		"non-string vertical": {"vertical": float64(3)},
		"zero nights":         {"text": "room", "nights": 0},
		"too many nights":     {"text": "room", "nights": 400},
		"negative party":      {"text": "room", "party_size": -2},
		"word nights":         {"text": "room", "nights": "many"},
		"long location":       {"location": string(make([]byte, 101))},
	} {
		err := v.Validate(context.Background(), domain.ActionSearchListings, p)
		var invalid *InvalidParameterError
		if !errors.As(err, &invalid) || !errors.Is(err, domain.ErrInvalidParameter) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestParameterValidator_SearchNeedsWordsVerticalOrPlace(t *testing.T) {
	v := NewDefaultParameterValidator()
	var missing *MissingParameterError
	if err := v.Validate(context.Background(), domain.ActionSearchListings, map[string]interface{}{}); !errors.As(err, &missing) || missing.Param != "text" {
		t.Fatalf("empty search: %v", err)
	}
	for _, p := range []map[string]interface{}{{"text": "phone"}, {"vertical": "HOTEL"}, {"location": "Gombe"}} {
		if err := v.Validate(context.Background(), domain.ActionSearchListings, p); err != nil {
			t.Errorf("%v: %v", p, err)
		}
	}
}

func TestParameterValidator_NightsGiveTheCheckOut(t *testing.T) {
	v := NewDefaultParameterValidator()
	p := map[string]interface{}{"listing_id": "room-1", "start_at": "2026-10-10", "nights": 3}
	if err := v.Validate(context.Background(), domain.ActionCreateReservation, p); err != nil {
		t.Fatal(err)
	}
	if p["start_at"] != int64(1791590400) || p["end_at"] != int64(1791590400+3*86400) {
		t.Fatalf("stay = %v → %v", p["start_at"], p["end_at"])
	}
	// An explicit end is never overwritten.
	p = map[string]interface{}{"listing_id": "room-1", "start_at": "2026-10-10", "end_at": "2026-10-11", "nights": 3}
	if err := v.Validate(context.Background(), domain.ActionCreateReservation, p); err != nil || p["end_at"] != "2026-10-11" {
		t.Fatalf("explicit end changed: %v %v", p["end_at"], err)
	}
}

func TestConfirmationPolicy_BookingsNeedConfirmation(t *testing.T) {
	p := NewDefaultConfirmationPolicy()
	for _, a := range []domain.CommerceActionType{domain.ActionCreateReservation, domain.ActionCancelReservation, domain.ActionModifyReservation, domain.ActionCreateOrder} {
		if need, _ := p.RequiresConfirmation(context.Background(), &domain.CommerceAction{ActionType: a}); !need {
			t.Errorf("%s must be confirmed", a)
		}
		if !domain.AIActionRequiresConfirmation(a) && a != domain.ActionCreateOrder {
			t.Errorf("%s: runtime and domain rules disagree", a)
		}
	}
	if need, _ := p.RequiresConfirmation(context.Background(), &domain.CommerceAction{ActionType: domain.ActionSearchListings}); need {
		t.Error("reads need no confirmation")
	}
}
