package search

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/application"
	"github.com/trezutil/trezutil/internal/search/infrastructure/opensearch"
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

func TestEventProcessor_WithMetrics(t *testing.T) {
	metrics := application.NewMetrics()
	dl := application.NewDeadLetter(metrics)

	ep := application.NewEventProcessor(
		&mockSearchSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockSearchIndex{},
		application.WithMetrics(metrics),
		application.WithDeadLetter(dl),
	)

	event := events.Event{
		EventID:    "m-test",
		EventName:  "ListingPublished",
		ListingID:  "l1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snap := metrics.Snapshot()
	if snap["search_projection_events_total"] != 1 {
		t.Errorf("expected 1 event, got %d", snap["search_projection_events_total"])
	}
}

func TestOpenSearchAdapter_IndexName(t *testing.T) {
	config := opensearch.DefaultConfig()
	adapter, err := opensearch.NewOpenSearchAdapter(config)
	if err != nil {
		t.Skipf("OpenSearch not available: %v", err)
	}

	if adapter.IndexName() != "trezutil-listings-v1" {
		t.Errorf("expected index name 'trezutil-listings-v1', got '%s'", adapter.IndexName())
	}
	if adapter.AliasName() != "trezutil-listings" {
		t.Errorf("expected alias 'trezutil-listings', got '%s'", adapter.AliasName())
	}
}
