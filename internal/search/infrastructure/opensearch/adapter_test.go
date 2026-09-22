package opensearch

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

func TestOpenSearchAdapter_ImplementsInterfaces(t *testing.T) {
	adapter := ensureTestIndex(t)

	var _ domain.SearchEnginePort = adapter
	var _ domain.SearchIndexPort = adapter
}

func TestOpenSearchAdapter_ExecuteSearch(t *testing.T) {
	adapter := ensureTestIndex(t)

	doc := testListingDocument("l1", "org-1", "seller-1", "store-1", "iPhone 15")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("iphone", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total == 0 {
		t.Error("expected at least 1 result, got 0")
	}
}

func TestOpenSearchAdapter_ExecuteSearch_EmptyIndex(t *testing.T) {
	adapter := ensureTestIndex(t)

	result, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("nonexistent", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected total 0 for empty index, got %d", result.Total)
	}
}

func TestOpenSearchAdapter_ExecuteSearch_Pagination(t *testing.T) {
	adapter := ensureTestIndex(t)

	for i := 0; i < 5; i++ {
		doc := testListingDocument(
			string(rune('a'+i)),
			"org-1", "seller-1", "store-1",
			string(rune('A'+i))+" listing",
		)
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index document %d: %v", i, err)
		}
	}

	result, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("listing", 3))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Items) > 3 {
		t.Errorf("expected at most 3 results (page size), got %d", len(result.Items))
	}
}

func TestOpenSearchAdapter_Health(t *testing.T) {
	adapter := ensureTestIndex(t)

	status := adapter.Health(context.Background())
	if status != domain.SearchHealthAvailable {
		t.Errorf("expected health status %s, got %s", domain.SearchHealthAvailable, status)
	}
}

func TestOpenSearchAdapter_IndexListingAndDelete(t *testing.T) {
	adapter := ensureTestIndex(t)

	doc := testListingDocument("doc-1", "org-1", "seller-1", "store-1", "Test Product")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	if err := adapter.DeleteListingDocument(context.Background(), "doc-1"); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}
}

func TestConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	if config.URL == "" {
		t.Error("expected default URL to be set")
	}

	if config.Index == "" {
		t.Error("expected default index to be set")
	}

	if config.MaxRetries != 3 {
		t.Errorf("expected default max retries 3, got %d", config.MaxRetries)
	}

	if config.ReindexBatch != 1000 {
		t.Errorf("expected default reindex batch 1000, got %d", config.ReindexBatch)
	}
}

func TestOpenSearchAdapter_IndexName(t *testing.T) {
	config := DefaultConfig()
	adapter, err := NewOpenSearchAdapter(config)
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

func TestOpenSearchAdapter_BulkIndex(t *testing.T) {
	adapter := ensureTestIndex(t)

	docs := []*projection.SearchListingDocument{
		testListingDocument("bulk-1", "org-1", "seller-1", "store-1", "Bulk Product 1"),
		testListingDocument("bulk-2", "org-1", "seller-1", "store-1", "Bulk Product 2"),
		testListingDocument("bulk-3", "org-1", "seller-1", "store-1", "Bulk Product 3"),
	}

	if err := adapter.documentStore.BulkIndexDocuments(context.Background(), docs); err != nil {
		t.Fatalf("failed to bulk index documents: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("Bulk", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total < 3 {
		t.Errorf("expected at least 3 results, got %d", result.Total)
	}
}

func TestAdapter_TimeoutConfig(t *testing.T) {
	config := testConfig
	config.Timeout = 5 * time.Second

	adapter, err := NewOpenSearchAdapter(config)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	if adapter.config.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", adapter.config.Timeout)
	}
}
