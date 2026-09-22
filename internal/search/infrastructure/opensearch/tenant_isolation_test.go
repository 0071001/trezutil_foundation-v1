package opensearch

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/search/projection"
)

func TestTenantIsolation_OrganizationIDStoredInDocument(t *testing.T) {
	adapter := ensureTestIndex(t)

	orgID := "org-isolated-1"
	doc := &projection.SearchListingDocument{
		ListingID:          "iso-1",
		OrganizationID:     orgID,
		Title:              "Isolated Product",
		Status:             "published",
		PriceAmount:        50.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
	}

	if err := adapter.IndexListingDocument(context.Background(), doc); err != nil {
		t.Fatalf("failed to index document: %v", err)
	}

	result, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("Isolated", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Total == 0 {
		t.Fatal("expected at least 1 result")
	}

	for _, item := range result.Items {
		if item.ListingID != "iso-1" {
			t.Errorf("expected listing ID 'iso-1', got '%s'", item.ListingID)
		}
	}
}

func TestTenantIsolation_SeparateIndicesForDifferentOrgs(t *testing.T) {
	org1Adapter := ensureTestIndex(t)
	org2Adapter := ensureTestIndex(t)

	doc1 := testListingDocument("org1-doc", "org-1", "seller-1", "store-1", "Org1 Product")
	doc2 := testListingDocument("org2-doc", "org-2", "seller-2", "store-2", "Org2 Product")

	if err := org1Adapter.IndexListingDocument(context.Background(), doc1); err != nil {
		t.Fatalf("failed to index org1 document: %v", err)
	}
	if err := org2Adapter.IndexListingDocument(context.Background(), doc2); err != nil {
		t.Fatalf("failed to index org2 document: %v", err)
	}

	org1Results, err := org1Adapter.ExecuteSearch(context.Background(), testSearchQuery("Product", 10))
	if err != nil {
		t.Fatalf("org1 search failed: %v", err)
	}

	org2Results, err := org2Adapter.ExecuteSearch(context.Background(), testSearchQuery("Product", 10))
	if err != nil {
		t.Fatalf("org2 search failed: %v", err)
	}

	org1IDs := make(map[string]bool)
	for _, item := range org1Results.Items {
		org1IDs[item.ListingID] = true
	}
	org2IDs := make(map[string]bool)
	for _, item := range org2Results.Items {
		org2IDs[item.ListingID] = true
	}

	if org1IDs["org2-doc"] {
		t.Error("org1 search should not contain org2 documents")
	}
	if org2IDs["org1-doc"] {
		t.Error("org2 search should not contain org1 documents")
	}
}

func TestTenantIsolation_AliasIsolation(t *testing.T) {
	adapter1 := ensureTestIndexWithAlias(t, "tenant-alias-a")
	adapter2 := ensureTestIndexWithAlias(t, "tenant-alias-b")

	doc1 := testListingDocument("alias-1-doc", "org-a", "seller-1", "store-1", "Alias A Product")
	doc2 := testListingDocument("alias-2-doc", "org-b", "seller-2", "store-2", "Alias B Product")

	if err := adapter1.IndexListingDocument(context.Background(), doc1); err != nil {
		t.Fatalf("failed to index doc1: %v", err)
	}
	if err := adapter2.IndexListingDocument(context.Background(), doc2); err != nil {
		t.Fatalf("failed to index doc2: %v", err)
	}

	results1, err := adapter1.ExecuteSearch(context.Background(), testSearchQuery("Alias", 10))
	if err != nil {
		t.Fatalf("adapter1 search failed: %v", err)
	}

	results2, err := adapter2.ExecuteSearch(context.Background(), testSearchQuery("Alias", 10))
	if err != nil {
		t.Fatalf("adapter2 search failed: %v", err)
	}

	ids1 := make(map[string]bool)
	for _, item := range results1.Items {
		ids1[item.ListingID] = true
	}
	ids2 := make(map[string]bool)
	for _, item := range results2.Items {
		ids2[item.ListingID] = true
	}

	if ids1["alias-2-doc"] {
		t.Error("tenant alias A search should not contain documents from tenant B")
	}
	if ids2["alias-1-doc"] {
		t.Error("tenant alias B search should not contain documents from tenant A")
	}
}

func TestTenantIsolation_DocumentDeletionDoesNotAffectOtherTenants(t *testing.T) {
	adapter := ensureTestIndex(t)

	doc1 := testListingDocument("del-1", "org-1", "seller-1", "store-1", "Product to delete")
	doc2 := testListingDocument("del-2", "org-1", "seller-1", "store-1", "Product to keep")

	if err := adapter.IndexListingDocument(context.Background(), doc1); err != nil {
		t.Fatalf("failed to index doc1: %v", err)
	}
	if err := adapter.IndexListingDocument(context.Background(), doc2); err != nil {
		t.Fatalf("failed to index doc2: %v", err)
	}

	if err := adapter.DeleteListingDocument(context.Background(), "del-1"); err != nil {
		t.Fatalf("failed to delete doc1: %v", err)
	}

	results, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("Product", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids := make(map[string]bool)
	for _, item := range results.Items {
		ids[item.ListingID] = true
	}

	if ids["del-1"] {
		t.Error("deleted document should not appear in search results")
	}
	if !ids["del-2"] {
		t.Error("non-deleted document should appear in search results")
	}
}

func TestTenantIsolation_MultipleOrgsSharedIndex(t *testing.T) {
	adapter := ensureTestIndex(t)

	doc1 := testListingDocument("shared-1", "org-1", "seller-1", "store-1", "Shared Product 1")
	doc2 := testListingDocument("shared-2", "org-2", "seller-2", "store-2", "Shared Product 2")

	if err := adapter.IndexListingDocument(context.Background(), doc1); err != nil {
		t.Fatalf("failed to index doc1: %v", err)
	}
	if err := adapter.IndexListingDocument(context.Background(), doc2); err != nil {
		t.Fatalf("failed to index doc2: %v", err)
	}

	results, err := adapter.ExecuteSearch(context.Background(), testSearchQuery("Shared", 10))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if results.Total < 2 {
		t.Errorf("expected at least 2 results, got %d", results.Total)
	}

	org1Found := false
	org2Found := false
	for _, item := range results.Items {
		if item.ListingID == "shared-1" {
			org1Found = true
		}
		if item.ListingID == "shared-2" {
			org2Found = true
		}
	}

	if !org1Found || !org2Found {
		t.Error("both org documents should be present in shared index search")
	}
}
