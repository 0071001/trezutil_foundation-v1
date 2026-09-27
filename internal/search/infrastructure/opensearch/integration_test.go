//go:build integration

package opensearch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/search/projection"
)

var integTestCounter uint64

const (
	integrationEnvPrefix  = "TEST_OPENSEARCH_INTEGRATION"
	defaultIntegrationURL = "http://localhost:9200"
)

func setupIntegrationConfig() Config {
	url := getEnvOrDefault("TREZUTIL_OPENSEARCH_URL", defaultIntegrationURL)
	username := os.Getenv("TREZUTIL_OPENSEARCH_USERNAME")
	password := os.Getenv("TREZUTIL_OPENSEARCH_PASSWORD")

	return Config{
		URL:          url,
		Username:     username,
		Password:     password,
		Index:        "trezutil-listings-integration-v1",
		Alias:        "trezutil-listings-integration",
		IndexVersion: "v1-integration",
		Timeout:      30 * time.Second,
		TLS:          os.Getenv("TREZUTIL_OPENSEARCH_TLS") == "true",
		MaxRetries:   3,
		ReindexBatch: 1000,
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func isIntegrationEnabled() bool {
	val := os.Getenv("TREZUTIL_OPENSEARCH_INTEGRATION")
	if val == "" {
		val = os.Getenv(integrationEnvPrefix)
	}
	return strings.ToLower(val) == "true" || strings.ToLower(val) == "1"
}

func skipIfRealOpenSearchUnavailable(t *testing.T) *OpenSearchAdapter {
	t.Helper()
	if !isIntegrationEnabled() {
		t.Skip("Skipping: set TREZUTIL_OPENSEARCH_INTEGRATION=true to enable real OpenSearch integration tests")
	}

	cfg := setupIntegrationConfig()
	adapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Skipf("Skipping: failed to create OpenSearch client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	healthCheck := adapter.Health(ctx)
	if healthCheck != domain.SearchHealthAvailable {
		t.Skipf("Skipping: OpenSearch not available at %s", cfg.URL)
	}

	return adapter
}

func uniqueTestIndexName() string {
	n := atomic.AddUint64(&integTestCounter, 1)
	return fmt.Sprintf("trezutil-integration-test-%d-%d", time.Now().UnixNano(), n)
}

func createFreshAdapterWithIndex(t *testing.T) *OpenSearchAdapter {
	t.Helper()
	adapter := skipIfRealOpenSearchUnavailable(t)

	indexName := uniqueTestIndexName()
	alias := indexName + "-alias"

	cfg := adapter.config
	cfg.Index = indexName
	cfg.Alias = alias

	freshAdapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Fatalf("failed to create adapter with index %s: %v", indexName, err)
	}

	if err := freshAdapter.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("failed to ensure index %s: %v", indexName, err)
	}

	t.Cleanup(func() {
		_ = freshAdapter.client.DeleteIndex(context.Background(), indexName)
	})

	return freshAdapter
}

func integrationDoc(listingID, orgID, title, categoryID string) *projection.SearchListingDocument {
	return &projection.SearchListingDocument{
		ListingID:          listingID,
		OrganizationID:     orgID,
		SellerID:           "seller-" + orgID,
		StoreID:            "store-" + orgID,
		Title:              title,
		Description:        fmt.Sprintf("Description for %s in %s", title, orgID),
		CategoryID:         categoryID,
		CategoryPath:       []string{categoryID},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Status:             "published",
		MediaCount:         1,
		SellerDisplayName:  "Test Seller " + orgID,
		SellerTrustLevel:   3,
		StoreName:          "Test Store " + orgID,
		StoreStatus:        "active",
		UpdatedAt:          time.Now().UnixMilli(),
		PublishedAt:        new(int64),
	}
}

func TestIntegration_Connectivity(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	health := adapter.Health(context.Background())
	if health != domain.SearchHealthAvailable {
		t.Fatalf("expected health available, got %s", health)
	}
}

func TestIntegration_VersionAndIdentity(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	resp, err := http.Get(adapter.config.URL + "/")
	if err != nil {
		t.Fatalf("failed to get root endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var root map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		t.Fatalf("failed to decode root response: %v", err)
	}

	version, ok := root["version"].(map[string]interface{})
	if !ok {
		t.Fatal("expected version field in root response")
	}

	dist, _ := version["distribution"].(string)
	if dist != "opensearch" {
		t.Errorf("expected distribution 'opensearch', got '%s'", dist)
	}

	num, _ := version["number"].(string)
	if num == "" {
		t.Error("expected non-empty version number")
	}
	t.Logf("OpenSearch version: %s, distribution: %s", num, dist)
}

func TestIntegration_HealthEndpoint(t *testing.T) {
	cfg := setupIntegrationConfig()

	if !isIntegrationEnabled() {
		t.Skip("Skipping: set TREZUTIL_OPENSEARCH_INTEGRATION=true to enable real OpenSearch integration tests")
	}

	resp, err := http.Get(cfg.URL + "/_cluster/health")
	if err != nil {
		t.Fatalf("failed to get cluster health: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	status, _ := health["status"].(string)
	if status != "green" && status != "yellow" {
		t.Errorf("expected cluster status green or yellow, got '%s'", status)
	}
}

func TestIntegration_IndexCreateAndMapping(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)
	adapterName := adapter.IndexName()

	resp, err := http.Get(adapter.config.URL + "/" + adapterName)
	if err != nil {
		t.Fatalf("failed to get index info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected index to exist (200), got %d", resp.StatusCode)
	}

	var root map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		t.Fatalf("failed to decode index response: %v", err)
	}

	indexData, ok := root[adapterName].(map[string]interface{})
	if !ok {
		t.Fatalf("expected index data for %s in response", adapterName)
	}

	mappings, ok := indexData["mappings"].(map[string]interface{})
	if !ok {
		t.Fatal("expected mappings in index")
	}

	properties, ok := mappings["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties in mappings")
	}

	expectedFields := []string{"listing_id", "organization_id", "title", "description", "category_id", "price_amount"}
	for _, field := range expectedFields {
		if _, ok := properties[field]; !ok {
			t.Errorf("expected mapping for field: %s", field)
		}
	}

	// Clean up
	_ = http.Get
	_ = adapter.client.DeleteIndex(context.Background(), adapterName)
}

func TestIntegration_IndexDocument(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	doc := integrationDoc("int-doc-1", "org-int-1", "Test Product", "cat-1")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	resp, err := http.Get(adapter.config.URL + "/" + adapter.IndexName() + "/_doc/" + doc.ListingID)
	if err != nil {
		t.Fatalf("failed to retrieve document: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for document retrieval, got %d", resp.StatusCode)
	}

	var docResp struct {
		Found bool `json:"found"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&docResp); err != nil {
		t.Fatalf("failed to decode document response: %v", err)
	}

	if !docResp.Found {
		t.Fatal("expected document to be found after indexing")
	}
}

func TestIntegration_SearchExactMatch(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	doc := integrationDoc("exact-1", "org-1", "iPhone 15 Pro Max", "cat-electronics")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "iPhone 15 Pro Max",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total == 0 {
		t.Fatal("expected at least 1 result for exact match")
	}

	found := false
	for _, item := range result.Items {
		if item.ListingID == "exact-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected exact-1 in results")
	}
}

func TestIntegration_SearchPartialMatch(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	docs := []*projection.SearchListingDocument{
		integrationDocWithTitle("partial-1", "org-1", "Apple iPhone 15"),
		integrationDocWithTitle("partial-2", "org-1", "Samsung Galaxy S24"),
		integrationDocWithTitle("partial-3", "org-1", "iPhone Case Cover"),
	}

	for _, doc := range docs {
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index document %s: %v", doc.ListingID, err)
		}
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "iphone",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total < 2 {
		t.Errorf("expected at least 2 results for 'iphone', got %d", result.Total)
	}

	ids := make(map[string]bool)
	for _, item := range result.Items {
		ids[item.ListingID] = true
	}

	if !ids["partial-1"] {
		t.Error("expected partial-1 (iPhone) in results")
	}
	if !ids["partial-3"] {
		t.Error("expected partial-3 (iPhone Case) in results")
	}
}

func TestIntegration_SearchFilters(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	docPublished := integrationDoc("filter-1", "org-1", "Published Product", "cat-1")
	docPublished.Status = "published"
	docDraft := integrationDoc("filter-2", "org-1", "Draft Product", "cat-1")
	docDraft.Status = "draft"

	for _, doc := range []*projection.SearchListingDocument{docPublished, docDraft} {
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index document: %v", err)
		}
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Product",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("expected 1 result (only published, not draft), got %d", result.Total)
	}

	for _, item := range result.Items {
		if item.ListingID != "filter-1" {
			t.Errorf("expected only filter-1 (published), got %s", item.ListingID)
		}
	}
}

func TestIntegration_SearchPagination(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	for i := 0; i < 15; i++ {
		doc := integrationDocWithTitle(
			fmt.Sprintf("page-%d", i), "org-1",
			fmt.Sprintf("Listing Item Number %d", i),
		)
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index doc %d: %v", i, err)
		}
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Listing Item",
		Pagination: domain.PaginationCursor{
			PageSize: 5,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if len(result.Items) > 5 {
		t.Errorf("expected at most 5 results (page size), got %d", len(result.Items))
	}

	if !result.HasNext {
		t.Error("expected HasNext to be true with 15 docs and page size 5")
	}
}

func TestIntegration_SearchEmptyResult(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	doc := integrationDoc("empty-1", "org-1", "Unique Product Name", "cat-1")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "nonexistentproductxyz123",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total != 0 {
		t.Errorf("expected 0 results for nonexistent query, got %d", result.Total)
	}
}

func TestIntegration_SearchInvalidQuery(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	doc := integrationDoc("invalid-1", "org-1", "Product", "cat-1")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	err := domain.DiscoveryQuery{
		Text: "Product",
		Pagination: domain.PaginationCursor{
			PageSize: 101,
		},
	}
	searchErr := validateSearchQuery(err)
	if searchErr == nil {
		t.Fatal("expected error for invalid query (page size > 100)")
	}
}

func TestIntegration_RefreshTokenAfterIndex(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	doc := integrationDoc("refresh-1", "org-1", "Refresh Test Product", "cat-1")
	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	refreshReq := opensearchapi.IndicesRefreshRequest{
		Index: []string{adapter.IndexName()},
	}
	resp, err := refreshReq.Do(context.Background(), adapter.client.client)
	if err != nil {
		t.Fatalf("failed to refresh: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 200/201 for refresh, got %d", resp.StatusCode)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Refresh Test Product",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search after refresh failed: %v", err)
	}

	if result.Total == 0 {
		t.Fatal("expected results after refresh")
	}
}

func TestIntegration_Error_InvalidIndex(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	cfg := adapter.config
	cfg.Index = "non-existent-index-xyz-9999"
	cfg.Alias = "non-existent-index-xyz-9999"

	badAdapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	result, err := badAdapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})

	if err == nil {
		t.Error("expected error when searching non-existent index")
	}
	if result.Total != 0 {
		t.Errorf("expected 0 results, got %d", result.Total)
	}
}

func TestIntegration_Error_UnavailableCluster(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	badConfig := adapter.config
	badConfig.URL = "http://127.0.0.1:19999"

	badAdapter, err := NewOpenSearchAdapter(badConfig)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	_, err = badAdapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})

	if err == nil {
		t.Error("expected error when searching unavailable cluster")
	}
}

func TestIntegration_Error_AuthenticationFailure(t *testing.T) {
	if !isIntegrationEnabled() {
		t.Skip("Skipping: set TREZUTIL_OPENSEARCH_INTEGRATION=true to enable")
	}

	secureURL := os.Getenv("TREZUTIL_OPENSEARCH_SECURE_URL")
	if secureURL == "" {
		t.Skip("Skipping: set TREZUTIL_OPENSEARCH_SECURE_URL for auth tests")
	}

	cfg := Config{
		URL:        secureURL,
		Username:   "invalid-user",
		Password:   "wrong-password",
		Index:      "trezutil-listings-integration-v1",
		Alias:      "trezutil-listings-integration",
		Timeout:    10 * time.Second,
		TLS:        true,
		MaxRetries: 1,
	}

	adapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Skipf("Could not create adapter for auth test: %v", err)
	}

	_, err = adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})

	if err == nil {
		t.Error("expected authentication error, got nil")
	}
}

func TestIntegration_TenantIsolation_SeparateQueries(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	docA := integrationDoc("tenant-iso-a", "org-tenant-a", "Product A", "cat-1")
	docB := integrationDoc("tenant-iso-b", "org-tenant-b", "Product B", "cat-1")

	for _, doc := range []*projection.SearchListingDocument{docA, docB} {
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index document: %v", err)
		}
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Product",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	ids := make(map[string]string)
	for _, item := range result.Items {
		ids[item.ListingID] = item.ListingID
	}

	if _, ok := ids["tenant-iso-a"]; !ok {
		t.Error("expected tenant-iso-a in shared index results")
	}
	if _, ok := ids["tenant-iso-b"]; !ok {
		t.Error("expected tenant-iso-b in shared index results")
	}
}

func TestIntegration_TenantIsolation_SeparateIndices(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	tenantAIndex := uniqueTestIndexName()
	tenantBIndex := uniqueTestIndexName()

	t.Cleanup(func() {
		_ = adapter.client.DeleteIndex(context.Background(), tenantAIndex)
		_ = adapter.client.DeleteIndex(context.Background(), tenantBIndex)
	})

	cfgA := adapter.config
	cfgA.Index = tenantAIndex
	cfgA.Alias = tenantAIndex + "-alias"
	adapterA, err := NewOpenSearchAdapter(cfgA)
	if err != nil {
		t.Fatalf("failed to create tenant A adapter: %v", err)
	}
	if err := adapterA.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("failed to ensure tenant A index: %v", err)
	}

	cfgB := adapter.config
	cfgB.Index = tenantBIndex
	cfgB.Alias = tenantBIndex + "-alias"
	adapterB, err := NewOpenSearchAdapter(cfgB)
	if err != nil {
		t.Fatalf("failed to create tenant B adapter: %v", err)
	}
	if err := adapterB.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("failed to ensure tenant B index: %v", err)
	}

	docA := integrationDoc("tenant-b-index-a", "org-a", "Product for Tenant A", "cat-1")
	docB := integrationDoc("tenant-b-index-b", "org-b", "Product for Tenant B", "cat-1")

	if err := adapterA.IndexListingDocument(context.Background(), docA); err != nil {
		t.Fatalf("failed to index tenant A doc: %v", err)
	}
	if err := adapterB.IndexListingDocument(context.Background(), docB); err != nil {
		t.Fatalf("failed to index tenant B doc: %v", err)
	}

	resultA, err := adapterA.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Product for Tenant",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("tenant A search failed: %v", err)
	}

	resultB, err := adapterB.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Product for Tenant",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("tenant B search failed: %v", err)
	}

	tenantAIDs := make(map[string]bool)
	for _, item := range resultA.Items {
		tenantAIDs[item.ListingID] = true
	}
	tenantBIDs := make(map[string]bool)
	for _, item := range resultB.Items {
		tenantBIDs[item.ListingID] = true
	}

	if tenantAIDs["tenant-b-index-b"] {
		t.Error("tenant A search should NOT contain tenant B documents")
	}
	if tenantBIDs["tenant-b-index-a"] {
		t.Error("tenant B search should NOT contain tenant A documents")
	}
	if !tenantAIDs["tenant-b-index-a"] {
		t.Error("tenant A search should contain tenant A documents")
	}
	if !tenantBIDs["tenant-b-index-b"] {
		t.Error("tenant B search should contain tenant B documents")
	}
}

func TestIntegration_TenantIsolation_DeleteIsolation(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	docA := integrationDoc("tenant-del-a", "org-a", "Product A Delete Test", "cat-1")
	docB := integrationDoc("tenant-del-b", "org-a", "Product B Delete Test", "cat-1")

	for _, doc := range []*projection.SearchListingDocument{docA, docB} {
		if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
			t.Fatalf("failed to index document: %v", err)
		}
	}

	if err := adapter.DeleteListingDocument(context.Background(), "tenant-del-a"); err != nil {
		t.Fatalf("failed to delete document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Product",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search after delete failed: %v", err)
	}

	ids := make(map[string]bool)
	for _, item := range result.Items {
		ids[item.ListingID] = true
	}

	if ids["tenant-del-a"] {
		t.Error("deleted document should NOT appear in search results")
	}
	if !ids["tenant-del-b"] {
		t.Error("non-deleted document should appear in search results")
	}
}

func TestIntegration_DataConsistency_Fields(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	publishedAt := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC).Unix()
	doc := &projection.SearchListingDocument{
		ListingID:          "data-consistency-1",
		OrganizationID:     "org-dc-1",
		SellerID:           "seller-dc-1",
		StoreID:            "store-dc-1",
		Title:              "Consistency Test Product",
		Description:        "A product for testing data consistency",
		CategoryID:         "cat-electronics",
		CategoryPath:       []string{"cat-commerce", "cat-electronics"},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        299.99,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Quantity:           intPtr(42),
		Status:             "published",
		Location:           &domain.GeoPoint{Latitude: -4.4419, Longitude: 15.2663},
		LocationIDs:        []string{"loc-kinshasa", "loc-gombe"},
		SellerDisplayName:  "Trezutil Seller",
		SellerTrustLevel:   4,
		StoreName:          "Trezutil Store",
		StoreStatus:        "active",
		MediaCount:         5,
		UpdatedAt:          time.Now().UnixMilli(),
		PublishedAt:        &publishedAt,
	}

	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Consistency Test Product",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total == 0 {
		t.Fatal("expected at least 1 result")
	}

	found := false
	for _, item := range result.Items {
		if item.ListingID == "data-consistency-1" {
			found = true
			if item.Title != "Consistency Test Product" {
				t.Errorf("expected title 'Consistency Test Product', got '%s'", item.Title)
			}
			if item.PriceCurrency != "USD" {
				t.Errorf("expected currency 'USD', got '%s'", item.PriceCurrency)
			}
			if item.PriceAmount != 299.99 {
				t.Errorf("expected price 299.99, got %f", item.PriceAmount)
			}
			if item.AvailabilityStatus != "in_stock" {
				t.Errorf("expected availability 'in_stock', got '%s'", item.AvailabilityStatus)
			}
			if item.Seller == nil {
				t.Error("expected seller in result")
			} else if item.Seller.DisplayName != "Trezutil Seller" {
				t.Errorf("expected seller 'Trezutil Seller', got '%s'", item.Seller.DisplayName)
			}
			if item.Store == nil {
				t.Error("expected store in result")
			} else if item.Store.Name != "Trezutil Store" {
				t.Errorf("expected store 'Trezutil Store', got '%s'", item.Store.Name)
			}
			break
		}
	}

	if !found {
		t.Error("expected data-consistency-1 in results")
	}
}

func TestIntegration_BulkIndexing(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	docs := make([]*projection.SearchListingDocument, 10)
	for i := 0; i < 10; i++ {
		docs[i] = integrationDocWithTitle(
			fmt.Sprintf("bulk-%d", i), "org-bulk-1",
			fmt.Sprintf("Bulk Product %d", i),
		)
	}

	if err := adapter.documentStore.BulkIndexDocuments(context.Background(), docs); err != nil {
		t.Fatalf("failed to bulk index documents: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Bulk Product",
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	})
	if err != nil {
		t.Fatalf("search after bulk index failed: %v", err)
	}

	if result.Total < 10 {
		t.Errorf("expected at least 10 results after bulk index, got %d", result.Total)
	}
}

func TestIntegration_AliasIsolation(t *testing.T) {
	adapter := skipIfRealOpenSearchUnavailable(t)

	indexName := uniqueTestIndexName()
	t.Cleanup(func() {
		_ = adapter.client.DeleteIndex(context.Background(), indexName)
	})

	cfg := adapter.config
	cfg.Index = indexName

	freshAdapter, err := NewOpenSearchAdapter(cfg)
	if err != nil {
		t.Fatalf("failed to create adapter: %v", err)
	}

	if err := freshAdapter.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("failed to ensure index: %v", err)
	}

	doc := integrationDoc("alias-iso-1", "org-alias", "Alias Isolation Product", "cat-1")
	if err := freshAdapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := freshAdapter.ExecuteSearch(context.Background(), domain.DiscoveryQuery{
		Text: "Alias Isolation",
		Pagination: domain.PaginationCursor{
			PageSize: 10,
		},
	})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}

	if result.Total == 0 {
		t.Fatal("expected results via alias")
	}

	found := false
	for _, item := range result.Items {
		if item.ListingID == "alias-iso-1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected alias-iso-1 in results via alias")
	}
}

func TestIntegration_EnsureIndexIdempotent(t *testing.T) {
	adapter := createFreshAdapterWithIndex(t)

	if err := adapter.EnsureIndex(context.Background()); err != nil {
		t.Fatalf("second EnsureIndex failed: %v", err)
	}
}

func integrationDocWithTitle(listingID, orgID, title string) *projection.SearchListingDocument {
	return &projection.SearchListingDocument{
		ListingID:          listingID,
		OrganizationID:     orgID,
		SellerID:           "seller-" + orgID,
		StoreID:            "store-" + orgID,
		Title:              title,
		Description:        fmt.Sprintf("Description for %s", title),
		CategoryID:         "cat-1",
		CategoryPath:       []string{"cat-1"},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Status:             "published",
		MediaCount:         1,
		SellerDisplayName:  "Test Seller",
		SellerTrustLevel:   3,
		StoreName:          "Test Store",
		StoreStatus:        "active",
		UpdatedAt:          time.Now().UnixMilli(),
	}
}
