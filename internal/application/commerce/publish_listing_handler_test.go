package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestPublishListingHandler_ActionType(t *testing.T) {
	handler := NewPublishListingHandler(&fakeListingUseCases{})
	if handler.ActionType() != domain.ActionPublishListing {
		t.Errorf("expected %s, got %s", domain.ActionPublishListing, handler.ActionType())
	}
}

func TestPublishListingHandler_Handle_Success(t *testing.T) {
	handler := NewPublishListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{"listing_id": "l1"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["status"] != "published" {
		t.Errorf("expected published, got %s", m["status"])
	}
}

func TestPublishListingHandler_Handle_NotFound(t *testing.T) {
	handler := NewPublishListingHandler(&fakeListingUseCases{err: domain.ErrListingNotFound})
	params := map[string]interface{}{"listing_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestPublishListingHandler_Handle_EmptyID(t *testing.T) {
	handler := NewPublishListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{"listing_id": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for empty listing_id")
	}
}

func TestPublishListingHandler_Handle_Forbidden(t *testing.T) {
	handler := NewPublishListingHandler(&fakeListingUseCases{err: domain.ErrInsufficientPermissions})
	params := map[string]interface{}{"listing_id": "l1"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}
