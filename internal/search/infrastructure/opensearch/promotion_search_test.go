package opensearch

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestQueryBuilder_OnPromotionFilter(t *testing.T) {
	page := domain.PaginationCursor{PageSize: 10}
	with := filters(t, domain.DiscoveryQuery{OnPromotion: true, Pagination: page})
	found := false
	for _, f := range with {
		if r, ok := f.(map[string]interface{})["range"].(map[string]interface{}); ok {
			if d, ok := r["discount_percent"].(map[string]interface{}); ok && d["gte"] == 1 {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("on-promotion query must filter discount_percent >= 1: %v", with)
	}
	for _, f := range filters(t, domain.DiscoveryQuery{Pagination: page}) {
		if r, ok := f.(map[string]interface{})["range"].(map[string]interface{}); ok && r["discount_percent"] != nil {
			t.Fatal("an ordinary query must not be restricted to promotions")
		}
	}
}

func TestQueryBuilder_DiscountSortPutsBiggestSavingFirst(t *testing.T) {
	sort := domain.NewSearchSort(domain.SearchSortFieldDiscount, domain.SearchSortDirectionDesc)
	body := buildSearchQuery(domain.DiscoveryQuery{Sort: &sort, Pagination: domain.PaginationCursor{PageSize: 10}})
	clauses := body["sort"].([]interface{})
	first := clauses[0].(map[string]interface{})["discount_percent"].(map[string]interface{})
	if first["order"] != "desc" || first["missing"] != "_last" {
		t.Fatalf("first sort clause = %v, want discount_percent desc with missing last", first)
	}
	if _, ok := clauses[1].(map[string]interface{})["published_at"]; !ok {
		t.Fatalf("ties must fall back to newest first: %v", clauses)
	}
}

func TestValidateSearchQuery_AcceptsDiscountSort(t *testing.T) {
	sort := domain.NewSearchSort(domain.SearchSortFieldDiscount, domain.SearchSortDirectionDesc)
	q := domain.DiscoveryQuery{OnPromotion: true, Sort: &sort, Pagination: domain.PaginationCursor{PageSize: 10}}
	if err := validateSearchQuery(q); err != nil {
		t.Fatalf("discount sort rejected: %v", err)
	}
}

func TestResultMapper_Promotion(t *testing.T) {
	r, err := mapHitToSearchResult(SearchHit{Source: map[string]interface{}{
		"listing_id": "l1", "title": "Phone", "price_amount": 80.0, "compare_at_amount": 100.0, "discount_percent": 99.0,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if r.CompareAtAmount == nil || *r.CompareAtAmount != 100 {
		t.Fatalf("compare-at = %v, want 100", r.CompareAtAmount)
	}
	if r.DiscountPercent != 20 {
		t.Fatalf("discount = %d, want 20 derived from the prices (the stored 99 must not be trusted)", r.DiscountPercent)
	}
}

func TestResultMapper_NoPromotionWhenNotBelowRegularPrice(t *testing.T) {
	for name, src := range map[string]map[string]interface{}{
		"absent":          {"price_amount": 80.0},
		"equal":           {"price_amount": 100.0, "compare_at_amount": 100.0},
		"price above":     {"price_amount": 120.0, "compare_at_amount": 100.0},
		"under one pct":   {"price_amount": 99.7, "compare_at_amount": 100.0},
		"zero compare":    {"price_amount": 10.0, "compare_at_amount": 0.0},
		"wrong json type": {"price_amount": 80.0, "compare_at_amount": "100"},
	} {
		src["listing_id"] = "l1"
		r, err := mapHitToSearchResult(SearchHit{Source: src})
		if err != nil {
			t.Fatal(err)
		}
		if r.CompareAtAmount != nil || r.DiscountPercent != 0 {
			t.Errorf("%s: compare-at %v discount %d, want none", name, r.CompareAtAmount, r.DiscountPercent)
		}
	}
}

func TestIndexMapping_DeclaresPromotionFields(t *testing.T) {
	m := &IndexManager{}
	full := m.buildMapping()["properties"].(map[string]interface{})
	add := additiveMapping()["properties"].(map[string]interface{})
	for _, props := range []map[string]interface{}{full, add} {
		if props["compare_at_amount"] == nil || props["discount_percent"] == nil {
			t.Fatal("both new and existing indices must map compare_at_amount and discount_percent")
		}
	}
}
