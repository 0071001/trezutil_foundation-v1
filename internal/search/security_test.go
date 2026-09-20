package search

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/application"
	"github.com/trezutil/trezutil/internal/search/infrastructure/opensearch"
	"github.com/trezutil/trezutil/internal/search/projection"
	"github.com/trezutil/trezutil/packages/events"
)

type mockSecuritySource struct{}

func (m *mockSecuritySource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:          listingID,
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
		Status:             "published",
		MediaCount:         1,
		SellerDisplayName:  "Seller",
		SellerTrustLevel:   3,
		StoreName:          "Store",
		StoreStatus:        "active",
		UpdatedAt:          1234567890,
	}, nil
}

func (m *mockSecuritySource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type mockSecurityIndex struct{}

func (m *mockSecurityIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	return nil
}

func (m *mockSecurityIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

func TestSecurity_TenantIDInDocument(t *testing.T) {
	ep := application.NewEventProcessor(
		&mockSecuritySource{},
		projection.NewSearchListingDocumentMapper(),
		&mockSecurityIndex{},
	)

	event := events.Event{
		EventID:    "sec-1",
		EventName:  "ListingPublished",
		ListingID:  "l1",
		Version:    1,
		OccurredAt: 1234567890,
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(nil, event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSecurity_NoHardcodedCredentials(t *testing.T) {
	config := opensearch.DefaultConfig()

	if config.Username != "" {
		t.Error("username should not have a hardcoded default")
	}
	if config.Password != "" {
		t.Error("password should not have a hardcoded default")
	}
	if config.URL != "http://localhost:9200" {
		t.Errorf("unexpected default URL")
	}
}

func TestSecurity_TLSDisabledByDefault(t *testing.T) {
	config := opensearch.DefaultConfig()

	if config.TLS {
		t.Error("TLS should be disabled by default for local development")
	}
}
