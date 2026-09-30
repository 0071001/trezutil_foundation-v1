package opensearch

import (
	"context"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func filters(t *testing.T, q domain.DiscoveryQuery) []interface{} {
	t.Helper()
	body := buildSearchQuery(q)
	return body["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"].([]interface{})
}

func TestQueryBuilder_VerticalAndPlaceFilters(t *testing.T) {
	fs := filters(t, domain.DiscoveryQuery{Vertical: domain.VerticalHotel, Place: "Gombe", Pagination: domain.PaginationCursor{PageSize: 10}})
	var vertical, place bool
	for _, f := range fs {
		m := f.(map[string]interface{})
		if term, ok := m["term"].(map[string]interface{}); ok && term["vertical"] == "HOTEL" {
			vertical = true
		}
		if mm, ok := m["multi_match"].(map[string]interface{}); ok {
			place = mm["query"] == "Gombe" && mm["operator"] == "and" &&
				strings.Contains(strings.Join(mm["fields"].([]string), ","), "place")
		}
	}
	if !vertical || !place {
		t.Fatalf("vertical filter %v, place filter %v in %v", vertical, place, fs)
	}

	// Without them, no extra filter is added (text search keeps working unchanged).
	for _, f := range filters(t, domain.DiscoveryQuery{Text: "phone", Pagination: domain.PaginationCursor{PageSize: 10}}) {
		m := f.(map[string]interface{})
		if term, ok := m["term"].(map[string]interface{}); ok && term["vertical"] != nil {
			t.Fatal("unexpected vertical filter")
		}
		if _, ok := m["multi_match"]; ok {
			t.Fatal("unexpected place filter")
		}
	}
}

func TestValidateSearchQuery_VerticalAndPlace(t *testing.T) {
	ok := domain.DiscoveryQuery{Vertical: domain.VerticalHotel, Place: "Gombe", Pagination: domain.PaginationCursor{PageSize: 10}}
	if err := validateSearchQuery(ok); err != nil {
		t.Fatalf("valid query: %v", err)
	}
	bad := ok
	bad.Vertical = "hotel rooms"
	if err := validateSearchQuery(bad); err == nil {
		t.Error("malformed vertical accepted")
	}
	bad = ok
	bad.Place = strings.Repeat("g", domain.MaxSearchPlaceLength+1)
	if err := validateSearchQuery(bad); err == nil {
		t.Error("over-long place accepted")
	}
}

func TestResultMapper_Vertical(t *testing.T) {
	r, err := mapHitToSearchResult(SearchHit{Source: map[string]interface{}{"listing_id": "l1", "title": "Deluxe room", "vertical": "HOTEL"}})
	if err != nil || r.Vertical != "HOTEL" {
		t.Fatalf("vertical = %q (%v)", r.Vertical, err)
	}
}

func TestEnsureIndex_ExtendsExistingIndexMapping(t *testing.T) {
	mock := NewMockOpenSearchServer()
	defer mock.Close()
	cfg := Config{URL: mock.URL(), Index: "trezutil-listings-v1", Alias: "trezutil-listings"}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	m := NewIndexManager(client, cfg)
	ctx := context.Background()
	// An index created by an earlier release already serves the alias.
	if err := client.CreateIndex(ctx, cfg.AliasName(), map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if err := m.EnsureIndex(ctx); err != nil { // existing index: additive mapping
		t.Fatal(err)
	}
	mock.mu.RLock()
	defer mock.mu.RUnlock()
	if mock.mappingUpdates != 1 {
		t.Fatalf("mapping updates = %d, want 1", mock.mappingUpdates)
	}
	props := additiveMapping()["properties"].(map[string]interface{})
	if props["vertical"] == nil || props["place"] == nil {
		t.Fatal("additive mapping must declare vertical and place")
	}
	full := m.buildMapping()["properties"].(map[string]interface{})
	if full["vertical"].(map[string]interface{})["type"] != "keyword" || full["place"] == nil {
		t.Fatal("new indices must map vertical and place")
	}
}
