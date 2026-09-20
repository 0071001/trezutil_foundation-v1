package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestPauseListingHandler_ActionType(t *testing.T) {
	handler := NewPauseListingHandler(&fakeListingUseCases{})
	if handler.ActionType() != domain.ActionPauseListing {
		t.Errorf("expected %s, got %s", domain.ActionPauseListing, handler.ActionType())
	}
}

func TestPauseListingHandler_Handle_Success(t *testing.T) {
	handler := NewPauseListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{"listing_id": "l1"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["status"] != "paused" {
		t.Errorf("expected paused, got %s", m["status"])
	}
}

func TestPauseListingHandler_Handle_NotFound(t *testing.T) {
	handler := NewPauseListingHandler(&fakeListingUseCases{err: domain.ErrListingNotFound})
	params := map[string]interface{}{"listing_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestPauseListingHandler_Handle_EmptyID(t *testing.T) {
	handler := NewPauseListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{"listing_id": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for empty listing_id")
	}
}

func TestPauseListingHandler_Handle_Forbidden(t *testing.T) {
	handler := NewPauseListingHandler(&fakeListingUseCases{err: domain.ErrInsufficientPermissions})
	params := map[string]interface{}{"listing_id": "l1"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}
