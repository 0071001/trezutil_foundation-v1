package domain

import (
	"testing"
)

func TestNewConversationContext(t *testing.T) {
	ctx := &ConversationContext{
		CurrentIntent: &ConversationIntent{
			Name:       "search",
			ActionType: ActionSearchListings,
		},
		Language:       "en",
		LastActivityAt: 1000,
	}

	if ctx.CurrentIntent == nil {
		t.Fatal("expected intent")
	}
	if ctx.CurrentIntent.Name != "search" {
		t.Errorf("expected search, got %s", ctx.CurrentIntent.Name)
	}
	if ctx.Language != "en" {
		t.Errorf("expected en, got %s", ctx.Language)
	}
	if ctx.LastActivityAt != 1000 {
		t.Errorf("expected 1000, got %d", ctx.LastActivityAt)
	}
	if ctx.CurrentEntities == nil {
		t.Log("entities nil is acceptable")
	}
	if ctx.CurrentListing != nil {
		t.Error("expected nil listing ref")
	}
	if ctx.CurrentOrder != nil {
		t.Error("expected nil order ref")
	}
	if ctx.CurrentSearchQuery != nil {
		t.Error("expected nil search query")
	}
}

func TestConversationContextWithListing(t *testing.T) {
	ctx := &ConversationContext{
		CurrentListing: &ListingRef{
			ID:     "listing-1",
			Title:  "Samsung Phone",
			Status: "published",
		},
	}

	if ctx.CurrentListing.ID != "listing-1" {
		t.Errorf("expected listing-1, got %s", ctx.CurrentListing.ID)
	}
	if ctx.CurrentListing.Title != "Samsung Phone" {
		t.Errorf("expected Samsung Phone, got %s", ctx.CurrentListing.Title)
	}
}

func TestConversationContextWithOrder(t *testing.T) {
	ctx := &ConversationContext{
		CurrentOrder: &OrderRef{
			ID:          "order-1",
			Status:      "pending_payment",
			TotalAmount: 10000,
			Currency:    "USD",
		},
	}

	if ctx.CurrentOrder.ID != "order-1" {
		t.Errorf("expected order-1, got %s", ctx.CurrentOrder.ID)
	}
	if ctx.CurrentOrder.Status != "pending_payment" {
		t.Errorf("expected pending_payment, got %s", ctx.CurrentOrder.Status)
	}
}

func TestConversationContextWithSearchQuery(t *testing.T) {
	ctx := &ConversationContext{
		CurrentSearchQuery: &DiscoveryQuery{
			Text: "Samsung",
			Pagination: PaginationCursor{
				PageSize: 20,
			},
		},
	}

	if ctx.CurrentSearchQuery.Text != "Samsung" {
		t.Errorf("expected Samsung, got %s", ctx.CurrentSearchQuery.Text)
	}
}

func TestConversationContextSetLanguage(t *testing.T) {
	ctx := &ConversationContext{}
	ctx.Language = "fr"

	if ctx.Language != "fr" {
		t.Errorf("expected fr, got %s", ctx.Language)
	}
}
