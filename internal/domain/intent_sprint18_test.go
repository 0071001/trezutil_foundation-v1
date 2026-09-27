package domain

import (
	"testing"
)

func TestConversationIntentToAction(t *testing.T) {
	actor := NewCommerceActor("user-123", ActorTypeCustomer)
	actor.OrgID = "org-1"

	intent := ConversationIntent{
		Name:                 "search_listings",
		Confidence:           0.95,
		Source:               "AI",
		ActionType:           ActionSearchListings,
		RequiresConfirmation: false,
		Entities: []ConversationEntity{
			{Name: "query", Value: "iPhone 15", NormalizedValue: "iPhone 15"},
			{Name: "location", Value: "Kinshasa", NormalizedValue: "Kinshasa"},
		},
	}

	action := intent.ToAction("conv-1", actor, "tenant-1")

	if action.ActionType != ActionSearchListings {
		t.Errorf("action type mismatch")
	}
	if action.Actor.ID != "user-123" {
		t.Errorf("actor mismatch: got %s, want user-123", action.Actor.ID)
	}
	if action.TenantID != "tenant-1" {
		t.Errorf("tenant mismatch: got %s, want tenant-1", action.TenantID)
	}
	if action.ConversationID != "conv-1" {
		t.Errorf("conversation ID mismatch")
	}
	if action.Parameters["query"] != "iPhone 15" {
		t.Errorf("query param mismatch")
	}
	if action.Parameters["location"] != "Kinshasa" {
		t.Errorf("location param mismatch")
	}
}

func TestConversationIntentToActionRequiresConfirmation(t *testing.T) {
	actor := NewCommerceActor("user-123", ActorTypeCustomer)
	intent := ConversationIntent{
		Name:                 "create_order",
		ActionType:           ActionCreateOrder,
		RequiresConfirmation: true,
		Source:               "AI",
	}

	action := intent.ToAction("conv-1", actor, "tenant-1")

	if action.ConfirmationState != ConfirmationPending {
		t.Errorf("expected ConfirmationPending, got %s", action.ConfirmationState)
	}
}

func TestEntitiesFromParams(t *testing.T) {
	params := map[string]interface{}{
		"query":    "iPhone",
		"location": "NYC",
		"quantity": 3,
	}

	entities := EntitiesFromParams(params)

	if len(entities) != 3 {
		t.Fatalf("expected 3 entities, got %d", len(entities))
	}

	entityMap := make(map[string]ConversationEntity)
	for _, e := range entities {
		entityMap[e.Name] = e
	}

	if entityMap["query"].Value != "iPhone" {
		t.Errorf("query value mismatch: got %s", entityMap["query"].Value)
	}
	if entityMap["location"].Source != "AI" {
		t.Errorf("expected source AI, got %s", entityMap["location"].Source)
	}
}

func TestIntentMissingRequiredParams(t *testing.T) {
	intent := ConversationIntent{
		ActionType: ActionGetOrder,
		Entities: []ConversationEntity{
			{Name: "order_id", Value: "ORD-123"},
		},
	}

	missing := intent.MissingRequiredParams()
	if len(missing) != 0 {
		t.Errorf("expected no missing params, got %d: %v", len(missing), missing)
	}
}

func TestIntentMissingRequiredParamsIncomplete(t *testing.T) {
	intent := ConversationIntent{
		ActionType: ActionCreateOrder,
		Entities: []ConversationEntity{
			{Name: "quantity", Value: "2"},
		},
	}

	missing := intent.MissingRequiredParams()
	if len(missing) != 1 {
		t.Fatalf("expected 1 missing param, got %d", len(missing))
	}
	if missing[0].Name != "listing_id" {
		t.Errorf("expected listing_id as missing, got %s", missing[0].Name)
	}
}

func TestIntentIsReadAction(t *testing.T) {
	readIntent := ConversationIntent{ActionType: ActionGetOrder}
	if !readIntent.IsReadAction() {
		t.Error("expected ActionGetOrder to be a read action")
	}

	writeIntent := ConversationIntent{ActionType: ActionCreateOrder}
	if writeIntent.IsReadAction() {
		t.Error("expected ActionCreateOrder to not be a read action")
	}
}

func TestConversationIntentToResponseConversion(t *testing.T) {
	resp := NewConversationResponse("Hello", "en")
	resp.AddCard(ConversationCard{
		Title:       "Test Card",
		Description: "A test card",
	})
	resp.AddAction(ActionSearchListings, "Search")
	resp.SetConfirmation(ConversationConfirmation{
		ActionID: "act-1",
		Summary:  "Confirm?",
	})

	if len(resp.Cards) != 1 {
		t.Errorf("expected 1 card, got %d", len(resp.Cards))
	}
	if len(resp.Actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(resp.Actions))
	}
	if resp.Confirmation == nil {
		t.Error("expected confirmation to be set")
	}
}
