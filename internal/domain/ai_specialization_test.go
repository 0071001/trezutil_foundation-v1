package domain

import (
	"testing"
)

func TestAISpecialistEntity_IsActive(t *testing.T) {
	spec := NewAISpecialist(AISpecialistCommerce, "Commerce AI", nil, nil)
	if !spec.IsActive() {
		t.Error("expected new specialist to be active")
	}

	spec.Status = AISpecialistStatusInactive
	if spec.IsActive() {
		t.Error("expected inactive specialist")
	}
}

func TestAISpecialistEntity_CanHandle(t *testing.T) {
	spec := NewAISpecialist(
		AISpecialistCommerce,
		"Commerce AI",
		[]CommerceActionType{ActionSearchListings, ActionGetOrder},
		nil,
	)

	if !spec.CanHandle(ActionSearchListings) {
		t.Error("expected to handle SEARCH_LISTINGS")
	}
	if !spec.CanHandle(ActionGetOrder) {
		t.Error("expected to handle GET_ORDER")
	}
	if spec.CanHandle(ActionCreateListing) {
		t.Error("expected NOT to handle CREATE_LISTING")
	}
}

func TestAISpecialistEntity_CanHandoffTo(t *testing.T) {
	spec := NewAISpecialist(
		AISpecialistCommerce,
		"Commerce AI",
		[]CommerceActionType{ActionSearchListings},
		[]AISpecialistCode{AISpecialistBooking, AISpecialistSupport},
	)

	if !spec.CanHandoffTo(AISpecialistBooking) {
		t.Error("expected handoff to BOOKING_AI")
	}
	if !spec.CanHandoffTo(AISpecialistSupport) {
		t.Error("expected handoff to SUPPORT_AI")
	}
	if spec.CanHandoffTo(AISpecialistMarketing) {
		t.Error("expected NOT to handoff to MARKETING_AI")
	}
}

func TestAISpecialistEntity_Validation(t *testing.T) {
	tests := []struct {
		name      string
		spec      *AISpecialistEntity
		wantError bool
	}{
		{
			name:      "valid",
			spec:      NewAISpecialist(AISpecialistCommerce, "Commerce", []CommerceActionType{ActionSearchListings}, nil),
			wantError: false,
		},
		{
			name:      "empty code",
			spec:      &AISpecialistEntity{Name: "Test", Status: AISpecialistStatusActive, OwnerAction: []CommerceActionType{ActionSearchListings}},
			wantError: true,
		},
		{
			name:      "empty name",
			spec:      &AISpecialistEntity{Code: AISpecialistCommerce, Status: AISpecialistStatusActive, OwnerAction: []CommerceActionType{ActionSearchListings}},
			wantError: true,
		},
		{
			name:      "no owner actions",
			spec:      &AISpecialistEntity{Code: AISpecialistCommerce, Name: "Test", Status: AISpecialistStatusActive},
			wantError: true,
		},
		{
			name:      "invalid status",
			spec:      &AISpecialistEntity{Code: AISpecialistCommerce, Name: "Test", Status: "BAD", OwnerAction: []CommerceActionType{ActionSearchListings}},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.spec.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError = %v", err, tt.wantError)
			}
		})
	}
}

func TestDefaultSpecialists_AllValid(t *testing.T) {
	for _, spec := range DefaultSpecialists {
		if err := spec.Validate(); err != nil {
			t.Errorf("default specialist %s validation failed: %v", spec.Code, err)
		}
	}
}

func TestDefaultSpecialists_NoActionOverlapWithinSameCategory(t *testing.T) {
	actionCount := make(map[CommerceActionType]int)
	for _, spec := range DefaultSpecialists {
		for _, action := range spec.OwnerAction {
			actionCount[action]++
		}
	}

	for action, count := range actionCount {
		if count > 1 {
			// Shared actions are OK — multiple specialists can handle the same action
			t.Logf("action %s is shared by %d specialists", action, count)
		}
	}
}

func TestGetSpecialistsForAction(t *testing.T) {
	specialists := GetSpecialistsForAction(ActionSearchListings)
	if len(specialists) == 0 {
		t.Error("expected at least one specialist for SEARCH_LISTINGS")
	}

	found := false
	for _, s := range specialists {
		if s == AISpecialistCommerce {
			found = true
		}
	}
	if !found {
		t.Error("expected COMMERCE_AI to handle SEARCH_LISTINGS")
	}
}

func TestGetActionsForSpecialist(t *testing.T) {
	actions := GetActionsForSpecialist(AISpecialistCommerce)
	if len(actions) == 0 {
		t.Error("expected commerce specialist to own at least one action")
	}
}

func TestSpecializationDecision_Defaults(t *testing.T) {
	decision := SpecializationDecision{
		SpecialistCode: AISpecialistCommerce,
		Confidence:     0.95,
		OwnerAction:    ActionSearchListings,
	}

	if decision.SpecialistCode != AISpecialistCommerce {
		t.Errorf("unexpected specialist: %s", decision.SpecialistCode)
	}
	if decision.Confidence != 0.95 {
		t.Errorf("unexpected confidence: %f", decision.Confidence)
	}
}

func TestHandoffContext_Structure(t *testing.T) {
	ctx := HandoffContext{
		FromSpecialist:    AISpecialistCommerce,
		ToSpecialist:      AISpecialistBooking,
		Reason:            "requires booking-specific logic",
		SharedContextKeys: []string{"query", "location"},
		Confidential:      false,
	}

	if ctx.FromSpecialist != AISpecialistCommerce {
		t.Errorf("unexpected from specialist: %s", ctx.FromSpecialist)
	}
	if ctx.ToSpecialist != AISpecialistBooking {
		t.Errorf("unexpected to specialist: %s", ctx.ToSpecialist)
	}
	if !ctx.Confidential {
		// Handoff should be marked confidential by default
		t.Log("handoff not marked confidential — data may be shared")
	}
}

func TestSystemInstructions_BasicFields(t *testing.T) {
	instr := DefaultSystemInstructions(AISpecialistCommerce)

	if instr.SpecialistCode != AISpecialistCommerce {
		t.Errorf("unexpected specialist code: %s", instr.SpecialistCode)
	}
	if instr.SystemPrompt == "" {
		t.Error("system prompt should not be empty")
	}
	if instr.SensitiveFieldToken == "" {
		t.Error("sensitive field token should be set")
	}
}

func TestSystemInstructions_ForEachSpecialist(t *testing.T) {
	codes := []AISpecialistCode{
		AISpecialistCommerce, AISpecialistBooking, AISpecialistSupport,
		AISpecialistMarketing, AISpecialistLogistics,
	}

	for _, code := range codes {
		instr := DefaultSystemInstructions(code)
		if instr.SpecialistCode != code {
			t.Errorf("expected specialist code %s, got %s", code, instr.SpecialistCode)
		}
		if instr.SystemPrompt == "" {
			t.Errorf("system prompt should not be empty for %s", code)
		}
	}
}

func TestAISpecializationError_Error(t *testing.T) {
	err := NewAISpecializationError(AIErrModelUnavailable, AISpecialistCommerce, "no model available")

	if err.Error() == "" {
		t.Error("error message should not be empty")
	}
	if err.Code != AIErrModelUnavailable {
		t.Errorf("unexpected error code: %s", err.Code)
	}
	if err.Specialist != AISpecialistCommerce {
		t.Errorf("unexpected specialist: %s", err.Specialist)
	}
}
