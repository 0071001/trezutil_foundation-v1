package domain

import (
	"testing"
)

func TestNewConversationIntent(t *testing.T) {
	intent := NewConversationIntent("search", ActionSearchListings)

	if intent.Name != "search" {
		t.Errorf("expected search, got %s", intent.Name)
	}
	if intent.ActionType != ActionSearchListings {
		t.Errorf("expected SEARCH_LISTINGS, got %s", intent.ActionType)
	}
	if intent.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", intent.Confidence)
	}
	if intent.Source != "USER" {
		t.Errorf("expected USER, got %s", intent.Source)
	}
	if len(intent.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(intent.Entities))
	}
}

func TestConversationIntentAddEntity(t *testing.T) {
	intent := NewConversationIntent("search", ActionSearchListings)
	entity := ConversationEntity{Name: "query", Value: "Samsung phone"}

	intent.AddEntity(entity)

	if len(intent.Entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(intent.Entities))
	}
	if intent.Entities[0].Name != "query" {
		t.Errorf("expected query, got %s", intent.Entities[0].Name)
	}
	if intent.Entities[0].Value != "Samsung phone" {
		t.Errorf("expected Samsung phone, got %s", intent.Entities[0].Value)
	}
}

func TestConversationIntentGetEntity(t *testing.T) {
	intent := NewConversationIntent("create_order", ActionCreateOrder)
	intent.AddEntity(ConversationEntity{Name: "listing_id", Value: "listing-1"})

	entity, found := intent.GetEntity("listing_id")
	if !found {
		t.Fatal("expected to find listing_id entity")
	}
	if entity.Value != "listing-1" {
		t.Errorf("expected listing-1, got %s", entity.Value)
	}
}

func TestConversationIntentGetEntityNotFound(t *testing.T) {
	intent := NewConversationIntent("search", ActionSearchListings)

	_, found := intent.GetEntity("nonexistent")
	if found {
		t.Error("expected not to find nonexistent entity")
	}
}

func TestConversationEntityWithConfidence(t *testing.T) {
	entity := ConversationEntity{
		Name:            "price",
		Value:           "250",
		NormalizedValue: "250.00",
		Confidence:      0.85,
		Source:          "AI",
	}

	if entity.Confidence != 0.85 {
		t.Errorf("expected 0.85, got %f", entity.Confidence)
	}
	if entity.Source != "AI" {
		t.Errorf("expected AI, got %s", entity.Source)
	}
}
