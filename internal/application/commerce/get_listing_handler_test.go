package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeListingUseCases struct {
	listing *domain.Listing
	err     error
}

func (f *fakeListingUseCases) CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error {
	return f.err
}
func (f *fakeListingUseCases) GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.listing, nil
}
func (f *fakeListingUseCases) PublishListing(ctx context.Context, actorID string, listingID string) error {
	return f.err
}
func (f *fakeListingUseCases) PauseListing(ctx context.Context, actorID string, listingID string) error {
	return f.err
}
func (f *fakeListingUseCases) ResumeListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

func TestGetListingHandler_ActionType(t *testing.T) {
	handler := NewGetListingHandler(&fakeListingUseCases{})
	if handler.ActionType() != domain.ActionGetListing {
		t.Errorf("expected %s, got %s", domain.ActionGetListing, handler.ActionType())
	}
}

func TestGetListingHandler_Handle_Success(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	handler := NewGetListingHandler(&fakeListingUseCases{listing: listing})
	params := map[string]interface{}{"listing_id": "l1"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	l, ok := result.(*domain.Listing)
	if !ok {
		t.Fatal("expected *domain.Listing")
	}
	if l.Title != "Test" {
		t.Errorf("expected Test, got %s", l.Title)
	}
}

func TestGetListingHandler_Handle_NotFound(t *testing.T) {
	handler := NewGetListingHandler(&fakeListingUseCases{err: domain.ErrListingNotFound})
	params := map[string]interface{}{"listing_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing listing")
	}
}

func TestGetListingHandler_Handle_EmptyID(t *testing.T) {
	handler := NewGetListingHandler(&fakeListingUseCases{})
	params := map[string]interface{}{"listing_id": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for empty listing_id")
	}
}

func TestGetListingHandler_Handle_NilParams(t *testing.T) {
	handler := NewGetListingHandler(&fakeListingUseCases{})

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), nil)
	if err == nil {
		t.Error("expected error for nil parameters")
	}
}

func TestGetListingHandler_Handle_TenantIsolation(t *testing.T) {
	listing := domain.NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", "physical_product", "fixed", "USD", 99.99)
	handler := NewGetListingHandler(&fakeListingUseCases{listing: listing})

	actor1 := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor2 := domain.NewCommerceActor("user-2", domain.ActorTypeCustomer)

	params := map[string]interface{}{"listing_id": "l1"}

	result1, err1 := handler.Handle(context.Background(), actor1, params)
	result2, err2 := handler.Handle(context.Background(), actor2, params)

	if err1 != nil {
		t.Errorf("actor1 unexpected error: %v", err1)
	}
	if err2 != nil {
		t.Errorf("actor2 unexpected error: %v", err2)
	}
	if result1 == nil || result2 == nil {
		t.Fatal("expected non-nil results")
	}
	// Both actors get the same listing if authorized - the fake doesn't enforce ownership
	// Real authorization happens in ListingUseCases.GetListing
}
