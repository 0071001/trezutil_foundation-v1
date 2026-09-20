package domain

import (
	"testing"
)

func TestNewConversationResponse(t *testing.T) {
	response := NewConversationResponse("Hello", "en")

	if response.Text != "Hello" {
		t.Errorf("expected Hello, got %s", response.Text)
	}
	if response.Language != "en" {
		t.Errorf("expected en, got %s", response.Language)
	}
	if response.StructuredContent != nil {
		t.Error("expected no structured content")
	}
	if response.Cards != nil {
		t.Error("expected no cards")
	}
	if response.Actions != nil {
		t.Error("expected no actions")
	}
	if response.Confirmation != nil {
		t.Error("expected no confirmation")
	}
	if response.Metadata == nil {
		t.Error("expected metadata map")
	}
}

func TestConversationResponseAddCard(t *testing.T) {
	response := NewConversationResponse("Found results", "en")
	card := ConversationCard{
		ID:    "card-1",
		Type:  "listing",
		Title: "Samsung Phone",
	}
	response.AddCard(card)

	if len(response.Cards) != 1 {
		t.Fatalf("expected 1 card, got %d", len(response.Cards))
	}
	if response.Cards[0].Title != "Samsung Phone" {
		t.Errorf("expected Samsung Phone, got %s", response.Cards[0].Title)
	}
}

func TestConversationResponseAddAction(t *testing.T) {
	response := NewConversationResponse("Found results", "en")
	response.AddAction(ActionGetListing, "View listing")

	if len(response.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(response.Actions))
	}
	if response.Actions[0].ActionType != ActionGetListing {
		t.Errorf("expected GET_LISTING, got %s", response.Actions[0].ActionType)
	}
	if response.Actions[0].Label != "View listing" {
		t.Errorf("expected View listing, got %s", response.Actions[0].Label)
	}
}

func TestConversationResponseSetConfirmation(t *testing.T) {
	response := NewConversationResponse("Confirm order?", "en")
	confirmation := ConversationConfirmation{
		ActionID: "action-1",
		Summary:  "1 Samsung Phone for $250",
		Details:  map[string]interface{}{"quantity": 1},
	}
	response.SetConfirmation(confirmation)

	if response.Confirmation == nil {
		t.Fatal("expected confirmation")
	}
	if response.Confirmation.Summary != "1 Samsung Phone for $250" {
		t.Errorf("expected summary, got %s", response.Confirmation.Summary)
	}
}

func TestConversationResponseStructuredContent(t *testing.T) {
	response := NewConversationResponse("Results", "en")
	response.StructuredContent = &StructuredContent{
		Type: "listings",
		Data: map[string]interface{}{"count": 5},
	}

	if response.StructuredContent.Type != "listings" {
		t.Errorf("expected listings, got %s", response.StructuredContent.Type)
	}
	if response.StructuredContent.Data["count"] != 5 {
		t.Errorf("expected count 5, got %v", response.StructuredContent.Data["count"])
	}
}
