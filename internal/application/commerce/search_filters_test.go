package commerce

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestSearchFiltersAreValidatedAndNormalized(t *testing.T) {
	p := map[string]interface{}{"price_max": "$600", "price_min": float64(100), "sort": " Price_ASC ", "category_id": "5b3d9e2a-1111-4222-8333-444455556666"}
	if err := normalizeSearchFilters(p); err != nil {
		t.Fatal(err)
	}
	if p["price_max"] != 600.0 || p["price_min"] != 100.0 || p["sort"] != "price_asc" {
		t.Fatalf("normalized = %v", p)
	}

	for name, bad := range map[string]map[string]interface{}{
		"negative price":    {"price_max": -5},
		"text price":        {"price_max": "cheap"},
		"huge price":        {"price_max": 1e12},
		"inverted range":    {"price_min": 300, "price_max": 100},
		"unknown sort":      {"sort": "random"},
		"injected category": {"category_id": "x\" OR 1=1"},
	} {
		if err := normalizeSearchFilters(bad); err == nil {
			t.Errorf("%s accepted: %v", name, bad)
		}
	}

	empty := map[string]interface{}{"sort": "", "category_id": "  ", "price_max": ""}
	if err := normalizeSearchFilters(empty); err != nil || len(empty) != 0 {
		t.Fatalf("empty values are dropped: %v %v", empty, err)
	}
}

func TestSearchFiltersReachTheQuery(t *testing.T) {
	q := domain.DiscoveryQuery{}
	applySearchFilters(&q, map[string]interface{}{"price_max": 20.0, "sort": "price_asc", "category_id": "abcdef0123456789"})
	if q.PriceRange == nil || q.PriceRange.Max == nil || *q.PriceRange.Max != 20 || q.PriceRange.Min != nil {
		t.Fatalf("price range = %+v", q.PriceRange)
	}
	if q.Sort == nil || q.Sort.Field != domain.SearchSortFieldPriceAsc || q.Sort.Direction != domain.SearchSortDirectionAsc {
		t.Fatalf("sort = %+v", q.Sort)
	}
	if q.Category == nil || q.Category.CategoryID != "abcdef0123456789" || !q.Category.IncludeChildren {
		t.Fatalf("category = %+v", q.Category)
	}

	plain := domain.DiscoveryQuery{}
	applySearchFilters(&plain, map[string]interface{}{"sort": "relevance"})
	if plain.PriceRange != nil || plain.Sort != nil || plain.Category != nil {
		t.Fatalf("relevance and no filters leave the query alone: %+v", plain)
	}
}
