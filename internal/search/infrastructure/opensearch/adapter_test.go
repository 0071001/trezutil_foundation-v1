package opensearch

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestOpenSearchAdapter_ImplementsInterfaces(t *testing.T) {
	config := Config{
		URL:        "http://localhost:9200",
		MaxRetries: 1,
		Timeout:    1 * time.Second,
	}

	adapter, err := NewOpenSearchAdapter(config)
	if err != nil {
		// OpenSearch may not be running in tests; this is expected
		t.Skipf("OpenSearch not available: %v", err)
	}

	// Verify it implements SearchEnginePort
	var _ domain.SearchEnginePort = adapter

	// Verify it implements SearchIndexPort
	var _ domain.SearchIndexPort = adapter
}

func TestOpenSearchAdapter_ExecuteSearch(t *testing.T) {
	config := Config{
		URL:        "http://localhost:9200",
		MaxRetries: 1,
		Timeout:    1 * time.Second,
	}

	adapter, err := NewOpenSearchAdapter(config)
	if err != nil {
		t.Skipf("OpenSearch not available: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected total 0, got %d", result.Total)
	}
}

func TestOpenSearchAdapter_Health(t *testing.T) {
	config := Config{
		URL:        "http://localhost:9200",
		MaxRetries: 1,
		Timeout:    1 * time.Second,
	}

	adapter, err := NewOpenSearchAdapter(config)
	if err != nil {
		t.Skipf("OpenSearch not available: %v", err)
	}

	status := adapter.Health(context.Background())
	if status != domain.SearchHealthAvailable && status != domain.SearchHealthUnavailable {
		t.Errorf("unexpected health status: %s", status)
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
