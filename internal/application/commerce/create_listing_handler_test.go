package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestCreateListingHandler_ActionType(t *testing.T) {
	handler := NewCreateListingHandler(&fakeListingUseCases{})
	if handler.ActionType() != domain.ActionCreateListing {
		t.Errorf("expected %s, got %s", domain.ActionCreateListing, handler.ActionType())
	}
}

func TestCreateListingHandler_Handle_Success(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	handler := NewCreateListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{
		"listing": listing,
	}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["status"] != "created" {
		t.Errorf("expected created, got %s", m["status"])
	}
}

func TestCreateListingHandler_Handle_MissingListing(t *testing.T) {
	handler := NewCreateListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestCreateListingHandler_Handle_NilListing(t *testing.T) {
	handler := NewCreateListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{
		"listing": nil,
	}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for nil listing")
	}
}

func TestCreateListingHandler_Handle_ServiceError(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	handler := NewCreateListingHandler(&fakeListingUseCases{err: domain.ErrInsufficientPermissions})
	params := map[string]interface{}{"listing": listing}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}

func TestCreateListingHandler_Handle_WithAttributes(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	handler := NewCreateListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{
		"listing":    listing,
		"attributes": []*domain.AttributeValue{},
		"media":      []*domain.ListingMedia{},
	}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
