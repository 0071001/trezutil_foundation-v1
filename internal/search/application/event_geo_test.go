package application

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
	"github.com/trezutil/trezutil/packages/events"
)

type mockGeoSource struct{}

func (m *mockGeoSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:      listingID,
		OrganizationID: "org-1",
		SellerID:       "seller-1",
		StoreID:        "store-1",
		Title:          "Test",
		CategoryID:     "cat-1",
		Location:       &domain.GeoPoint{Latitude: -4.44, Longitude: 15.27},
		LocationIDs:    []string{"loc-1"},
		Status:         "published",
		UpdatedAt:      time.Now().Unix(),
	}, nil
}

func (m *mockGeoSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type mockGeoIndex struct{}

func (m *mockGeoIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	return nil
}

func (m *mockGeoIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

func TestGeoProjection_WithCoordinates(t *testing.T) {
	ep := NewEventProcessor(
		&mockGeoSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockGeoIndex{},
	)

	event := events.Event{
		EventID:    "geo-1",
		EventName:  "ListingPublished",
		ListingID:  "listing-1",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGeoProjection_WithoutCoordinates(t *testing.T) {
	source := &geoSourceNoCoords{}
	ep := NewEventProcessor(
		source,
		projection.NewSearchListingDocumentMapper(),
		&mockGeoIndex{},
	)

	event := events.Event{
		EventID:    "geo-2",
		EventName:  "ListingPublished",
		ListingID:  "listing-2",
		Version:    1,
		OccurredAt: time.Now().Unix(),
		Payload:    map[string]interface{}{},
	}

	if err := ep.ProcessEvent(context.Background(), event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type geoSourceNoCoords struct{}

func (g *geoSourceNoCoords) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:      listingID,
		OrganizationID: "org-1",
		Title:          "Test",
		Status:         "published",
		UpdatedAt:      time.Now().Unix(),
	}, nil
}

func (g *geoSourceNoCoords) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}
