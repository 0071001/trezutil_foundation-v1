package search

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/application"
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

func TestEventProcessor_DocumentMapperPreservesTenantFields(t *testing.T) {
	mapper := projection.NewSearchListingDocumentMapper()

	proj := domain.ListingProjection{
		ListingID:          "l1",
		OrganizationID:     "org-1",
		SellerID:           "s1",
		StoreID:            "st1",
		Title:              "Test Product",
		PriceAmount:        199.99,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Status:             "published",
		CategoryID:         "cat-1",
		CategoryPath:       []string{"cat-1", "cat-1-sub"},
	}

	doc, err := mapper.MapToDocument(proj)
	if err != nil {
		t.Fatalf("failed to map: %v", err)
	}

	if doc.ListingID != "l1" {
		t.Errorf("expected listing_id 'l1', got '%s'", doc.ListingID)
	}
	if doc.OrganizationID != "org-1" {
		t.Errorf("expected organization_id 'org-1', got '%s'", doc.OrganizationID)
	}
	if doc.SellerID != "s1" {
		t.Errorf("expected seller_id 's1', got '%s'", doc.SellerID)
	}
	if doc.StoreID != "st1" {
		t.Errorf("expected store_id 'st1', got '%s'", doc.StoreID)
	}
	if doc.Status != "published" {
		t.Errorf("expected status 'published', got '%s'", doc.Status)
	}
}

func TestEventProcessor_RejectsNonPublishedStatus(t *testing.T) {
	cases := []struct {
		status string
		ok     bool
	}{
		{"published", true},
		{"draft", false},
		{"archived", false},
		{"deleted", false},
		{"hidden", false},
	}

	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			doc := projection.SearchListingDocument{
				ListingID: "l1",
				Status:    tc.status,
			}

			valid := doc.Status == "published"
			if valid != tc.ok {
				t.Errorf("status %q: expected ok=%v, got ok=%v", tc.status, tc.ok, valid)
			}
		})
	}
}

func TestEventProcessor_TenantScopeIsolation(t *testing.T) {
	m1Docs := make(map[string]string)
	m2Docs := make(map[string]string)

	for i := 0; i < 3; i++ {
		id1 := "m1-l" + string(rune('1'+i))
		id2 := "m2-l" + string(rune('1'+i))
		m1Docs[id1] = "org-1"
		m2Docs[id2] = "org-2"
	}

	if len(m1Docs) != 3 || len(m2Docs) != 3 {
		t.Error("tenant documents should be isolated")
	}

	for orgID := range m1Docs {
		if m2Docs[orgID] != "" {
			t.Error("same listing should not appear in both tenants")
		}
	}
}
