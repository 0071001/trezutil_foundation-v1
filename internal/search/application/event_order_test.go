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

type mockProjectionSourceOrder struct{}

func (m *mockProjectionSourceOrder) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:          listingID,
		Status:             "published",
		UpdatedAt:          time.Now().Unix(),
		OrganizationID:     "org-1",
		SellerID:           "seller-1",
		StoreID:            "store-1",
		Title:              "Test",
		CategoryID:         "cat-1",
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		MediaCount:         1,
		SellerDisplayName:  "Seller",
		SellerTrustLevel:   3,
		StoreName:          "Store",
		StoreStatus:        "active",
	}, nil
}

func (m *mockProjectionSourceOrder) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type mockIndexPortOrder struct{}

func (m *mockIndexPortOrder) IndexListingDocument(ctx context.Context, document interface{}) error {
	return nil
}

func (m *mockIndexPortOrder) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

func TestEventOrderTracker_Basic(t *testing.T) {
	tracker := NewEventOrderTracker()

	if !tracker.CheckAndUpdate("listing-1", 1, "evt-1") {
		t.Error("first event should be processed")
	}
	if !tracker.CheckAndUpdate("listing-1", 2, "evt-2") {
		t.Error("second event should be processed")
	}
	if tracker.CheckAndUpdate("listing-1", 1, "evt-3") {
		t.Error("out-of-order event should be rejected")
	}
}

func TestEventOrderTracker_SameVersionSameEvent(t *testing.T) {
	tracker := NewEventOrderTracker()

	if !tracker.CheckAndUpdate("listing-1", 1, "evt-1") {
		t.Error("first event should be processed")
	}
	if tracker.CheckAndUpdate("listing-1", 1, "evt-1") {
		t.Error("duplicate event should be rejected")
	}
}

func TestEventOrderTracker_DifferentListings(t *testing.T) {
	tracker := NewEventOrderTracker()

	if !tracker.CheckAndUpdate("listing-1", 1, "evt-1") {
		t.Error("listing-1 first event should be processed")
	}
	if !tracker.CheckAndUpdate("listing-2", 1, "evt-2") {
		t.Error("listing-2 first event should be processed")
	}
}

func TestEventProcessor_OutOfOrderRejected(t *testing.T) {
	ep := NewEventProcessor(
		&mockProjectionSourceOrder{},
		projection.NewSearchListingDocumentMapper(),
		&mockIndexPortOrder{},
	)

	// Process version 2 first (simulating out-of-order)
	eventV2 := events.Event{
		EventID:    "evt-v2",
		EventName:  "ListingUpdated",
		ListingID:  "listing-1",
		Version:    2,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), eventV2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Now try version 1 (should be rejected as stale)
	eventV1 := events.Event{
		EventID:    "evt-v1",
		EventName:  "ListingUpdated",
		ListingID:  "listing-1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), eventV1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventProcessor_Idempotency(t *testing.T) {
	ep := NewEventProcessor(
		&mockProjectionSourceOrder{},
		projection.NewSearchListingDocumentMapper(),
		&mockIndexPortOrder{},
	)

	event := events.Event{
		EventID:    "event-1",
		EventName:  "ListingPublished",
		ListingID:  "listing-1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	err1 := ep.ProcessEvent(context.Background(), event)
	if err1 != nil {
		t.Fatalf("unexpected error on first process: %v", err1)
	}

	err2 := ep.ProcessEvent(context.Background(), event)
	if err2 != nil {
		t.Fatalf("unexpected error on second process: %v", err2)
	}
}

type failingIndexPort struct {
	failCount *int
	maxFails  int
}

func (f *failingIndexPort) IndexListingDocument(ctx context.Context, document interface{}) error {
	*f.failCount++
	if *f.failCount <= f.maxFails {
		return fmt.Errorf("temporary failure")
	}
	return nil
}

func (f *failingIndexPort) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}
