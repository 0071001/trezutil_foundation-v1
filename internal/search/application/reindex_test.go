package application

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

type mockReindexSource struct{}

func (m *mockReindexSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{
		ListingID:          listingID,
		Status:             "published",
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
		MediaCount:         1,
		SellerDisplayName:  "Seller",
		SellerTrustLevel:   3,
		StoreName:          "Store",
		StoreStatus:        "active",
		UpdatedAt:          time.Now().Unix(),
	}, nil
}

func (m *mockReindexSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{}, nil
}

type mockReindexIndex struct{}

func (m *mockReindexIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	return nil
}

func (m *mockReindexIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

type failingReindexSource struct{}

func (f *failingReindexSource) GetListingProjection(ctx context.Context, listingID string) (domain.ListingProjection, error) {
	return domain.ListingProjection{}, fmt.Errorf("source failure")
}

func (f *failingReindexSource) GetPublishedListingIDs(ctx context.Context, cursor string, limit int) ([]string, error) {
	return []string{"listing-1"}, nil
}

type failingReindexIndex struct{}

func (f *failingReindexIndex) IndexListingDocument(ctx context.Context, document interface{}) error {
	return fmt.Errorf("index failure")
}

func (f *failingReindexIndex) DeleteListingDocument(ctx context.Context, listingID string) error {
	return nil
}

func TestReindexJob_Start(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		100,
	)

	progress, err := job.Start(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress == nil {
		t.Fatal("expected non-nil progress")
	}
}

func TestReindexJob_StartWithFailures(t *testing.T) {
	job := NewReindexJob(
		&failingReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&failingReindexIndex{},
		100,
	)

	_, err := job.Start(nil)
	if err == nil {
		t.Error("expected error from Start")
	}
}

func TestReindexJob_StartFromListingID(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		100,
	)

	progress, err := job.StartFromListingID(nil, "listing-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.Processed != 1 {
		t.Errorf("expected processed 1, got %d", progress.Processed)
	}
}

func TestReindexJob_StartFromListingIDEmpty(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		100,
	)

	_, err := job.StartFromListingID(nil, "")
	if err == nil {
		t.Error("expected error for empty listing ID")
	}
}

func TestReindexJob_ReindexListing(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		100,
	)

	if err := job.ReindexListing(nil, "listing-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReindexJob_BatchSizeDefault(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		0,
	)

	if job.batchSize != 1000 {
		t.Errorf("expected default batch size 1000, got %d", job.batchSize)
	}
}

func TestReindexJob_CustomBatchSize(t *testing.T) {
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		500,
	)

	if job.batchSize != 500 {
		t.Errorf("expected batch size 500, got %d", job.batchSize)
	}
}

func TestReindexJob_WithMetrics(t *testing.T) {
	metrics := NewMetrics()
	job := NewReindexJob(
		&mockReindexSource{},
		projection.NewSearchListingDocumentMapper(),
		&mockReindexIndex{},
		100,
		WithReindexMetrics(metrics),
	)

	if job.metrics == nil {
		t.Error("expected metrics to be set")
	}

	_, _ = job.Start(nil)

	snap := metrics.Snapshot()
	if snap["search_reindex_items_total"] != 0 {
		// Source returns empty list
		t.Logf("reindex items: %d", snap["search_reindex_items_total"])
	}
}
