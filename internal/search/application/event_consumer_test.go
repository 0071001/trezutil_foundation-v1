package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
	"github.com/trezutil/trezutil/packages/events"
)

type mockSearchSource struct{}

func (m *mockSearchSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:          listingID,
		OrganizationID:     "org-1",
		SellerID:           "seller-1",
		StoreID:            "store-1",
		Title:              "Test",
		CategoryID:         "cat-1",
		CategoryPath:       []string{"cat-1"},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Status:             "published",
		MediaCount:         1,
		SellerDisplayName:  "Seller",
		SellerTrustLevel:   3,
		StoreName:          "Store",
		StoreStatus:        "active",
		UpdatedAt:          time.Now().Unix(),
	}, nil
}

func (m *mockSearchSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type mockSearchIndex struct{}

func (m *mockSearchIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	return nil
}

func (m *mockSearchIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

type mockOrderSource struct{}

func (m *mockOrderSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:      listingID,
		Status:         "published",
		UpdatedAt:      1234567890,
		Title:          "Test",
		OrganizationID: "org-1",
	}, nil
}

func (m *mockOrderSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type failingIndex struct {
	failCount *int
	maxFails  int
}

func (f *failingIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	*f.failCount++
	if *f.failCount <= f.maxFails {
		return fmt.Errorf("temporary failure")
	}
	return nil
}

func (f *failingIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

func TestEventProcessor_RetryBehavior(t *testing.T) {
	failCount := 0
	ip := &failingIndex{failCount: &failCount, maxFails: 100}

	ep := NewEventProcessor(
		&mockSearchSource{},
		projection.NewSearchListingDocumentMapper(),
		ip,
	)

	event := events.Event{
		EventID:    "retry-event",
		EventName:  "ListingPublished",
		ListingID:  "listing-1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	err := ep.ProcessEvent(context.Background(), event)
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}

	if failCount != 4 {
		t.Errorf("expected 4 attempts (initial + 3 retries), got %d", failCount)
	}
}

func TestEventProcessor_NonSearchableListing(t *testing.T) {
	ep := NewEventProcessor(
		&nonSearchableSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockSearchIndex{},
	)

	event := events.Event{
		EventID:    "ns-event",
		EventName:  "ListingUpdated",
		ListingID:  "listing-1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type nonSearchableSource struct{}

func (n *nonSearchableSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:          listingID,
		Status:             "draft",
		OrganizationID:     "org-1",
		SellerID:           "seller-1",
		StoreID:            "store-1",
		Title:              "Test",
		CategoryID:         "cat-1",
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "draft",
		UpdatedAt:          time.Now().Unix(),
	}, nil
}

func (n *nonSearchableSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}
