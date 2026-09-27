package domain

import (
	"encoding/json"
	"testing"
)

func TestConversationContextHasPendingConfirmation(t *testing.T) {
	ctx := &ConversationContext{}

	if ctx.HasPendingConfirmation() {
		t.Error("expected no pending confirmation for empty context")
	}

	ctx.ConfirmationState = ConfirmationFlowState{
		ActionID: "action-1",
		State:    ConfirmationRequested,
	}
	if !ctx.HasPendingConfirmation() {
		t.Error("expected pending confirmation with ConfirmationRequested state")
	}

	ctx.ConfirmationState.State = ConfirmationApproved
	if ctx.HasPendingConfirmation() {
		t.Error("expected no pending confirmation with ConfirmationApproved state")
	}
}

func TestConversationContextHasMissingParameters(t *testing.T) {
	ctx := &ConversationContext{}
	if ctx.HasMissingParameters() {
		t.Error("expected no missing parameters for empty context")
	}

	ctx.MissingParameters = []MissingParameter{
		{Name: "listing_id", Description: "listing ID is required"},
	}
	if !ctx.HasMissingParameters() {
		t.Error("expected missing parameters when set")
	}
}

func TestConversationContextWithReservation(t *testing.T) {
	ctx := &ConversationContext{
		CurrentReservation: &ReservationRef{
			ID:        "res-1",
			Status:    "confirmed",
			StartAt:   1727000000,
			EndAt:     1727086400,
			PartySize: 2,
			Quantity:  1,
		},
	}

	if ctx.CurrentReservation.ID != "res-1" {
		t.Errorf("expected res-1, got %s", ctx.CurrentReservation.ID)
	}
	if ctx.CurrentReservation.PartySize != 2 {
		t.Errorf("expected party size 2, got %d", ctx.CurrentReservation.PartySize)
	}
}

func TestConversationContextJSONRoundTrip(t *testing.T) {
	ctx := &ConversationContext{
		Language:       "en",
		PackCode:       PackBooking,
		SpecialistCode: AISpecialistBooking,
		ConfirmationState: ConfirmationFlowState{
			ActionID: "action-1",
			State:    ConfirmationRequested,
			Summary:  "Please confirm",
		},
		PendingParameters: []PendingParameter{
			{Name: "listing_id", Description: "which listing"},
		},
		MissingParameters: []MissingParameter{
			{Name: "start_at", Description: "when", Examples: []string{"June 15", "tomorrow"}},
		},
		LastActivityAt: 1727000000,
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var restored ConversationContext
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if restored.Language != ctx.Language {
		t.Errorf("Language mismatch: got %s, want %s", restored.Language, ctx.Language)
	}
	if restored.PackCode != ctx.PackCode {
		t.Errorf("PackCode mismatch: got %s, want %s", restored.PackCode, ctx.PackCode)
	}
	if restored.SpecialistCode != ctx.SpecialistCode {
		t.Errorf("SpecialistCode mismatch: got %s, want %s", restored.SpecialistCode, ctx.SpecialistCode)
	}
	if restored.ConfirmationState.ActionID != ctx.ConfirmationState.ActionID {
		t.Errorf("ActionID mismatch")
	}
	if restored.ConfirmationState.State != ctx.ConfirmationState.State {
		t.Errorf("State mismatch")
	}
	if len(restored.MissingParameters) != 1 {
		t.Fatalf("expected 1 missing param, got %d", len(restored.MissingParameters))
	}
	if restored.MissingParameters[0].Name != "start_at" {
		t.Errorf("missing param name mismatch")
	}
}

func TestConversationContextWithResultItems(t *testing.T) {
	ctx := &ConversationContext{
		ResultItems: []ResultItemRef{
			{RefType: "listing", ID: "list-1", Index: 0, Title: "iPhone 15", Subtitle: "$999"},
			{RefType: "listing", ID: "list-2", Index: 1, Title: "Samsung S24", Subtitle: "$899"},
		},
	}

	if len(ctx.ResultItems) != 2 {
		t.Fatalf("expected 2 result items, got %d", len(ctx.ResultItems))
	}
	if ctx.ResultItems[0].ID != "list-1" {
		t.Errorf("expected list-1, got %s", ctx.ResultItems[0].ID)
	}
	if ctx.ResultItems[1].Title != "Samsung S24" {
		t.Errorf("expected Samsung S24, got %s", ctx.ResultItems[1].Title)
	}
}
