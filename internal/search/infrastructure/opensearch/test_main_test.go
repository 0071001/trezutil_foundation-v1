package opensearch

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

var (
	testServer *MockOpenSearchServer
	testConfig Config
)

var testCounter uint64

func TestMain(m *testing.M) {
	testServer = NewMockOpenSearchServer()
	testConfig = Config{
		URL:        testServer.URL(),
		MaxRetries: 1,
		Timeout:    0,
	}

	exitCode := m.Run()
	testServer.Close()
	os.Exit(exitCode)
}

func nextTestID() string {
	n := atomic.AddUint64(&testCounter, 1)
	return fmt.Sprintf("test-%d", n)
}

func testConfigWithIndex(indexName string) Config {
	cfg := testConfig
	cfg.Index = indexName
	cfg.Alias = indexName + "-alias"
	return cfg
}

func ensureTestIndex(t *testing.T) *OpenSearchAdapter {
	t.Helper()
	cfg := testConfigWithIndex(nextTestID())
	return ensureAdapterWithIndex(t, cfg)
}

func ensureTestIndexWithAlias(t *testing.T, alias string) *OpenSearchAdapter {
	t.Helper()
	testID := nextTestID()
	cfg := testConfigWithIndex(testID)
	cfg.Alias = alias
	return ensureAdapterWithIndex(t, cfg)
}

func ensureAdapterWithIndex(t *testing.T, cfg Config) *OpenSearchAdapter {
	t.Helper()
	adapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}
	if err := adapter.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("failed to ensure index: %v", err)
	}
	return adapter
}

func testSearchQuery(text string, pageSize int) domain.DiscoveryQuery {
	return domain.DiscoveryQuery{
		Text: text,
		Pagination: domain.PaginationCursor{
			PageSize: pageSize,
		},
	}
}

func testListingDocument(listingID, orgID, sellerID, storeID, title string) *projection.SearchListingDocument {
	return &projection.SearchListingDocument{
		ListingID:          listingID,
		OrganizationID:     orgID,
		SellerID:           sellerID,
		StoreID:            storeID,
		Title:              title,
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
		UpdatedAt:          1234567890,
	}
}
