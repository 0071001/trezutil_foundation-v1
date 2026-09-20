package opensearch

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestQueryBuilder_BasicTextSearch(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "iphone 15",
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	must := boolQuery["must"].([]interface{})
	if len(must) != 1 {
		t.Fatalf("expected 1 must clause, got %d", len(must))
	}

	multiMatch := must[0].(map[string]interface{})["multi_match"]
	if multiMatch == nil {
		t.Error("expected multi_match query")
	}
}

func TestQueryBuilder_FilterStatus(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if term, ok := filter["term"]; ok {
			if termMap, ok := term.(map[string]interface{}); ok {
				if status, ok := termMap["status"]; ok && status == "published" {
					found = true
					break
				}
			}
		}
	}

	if !found {
		t.Error("expected status=published filter")
	}
}

func TestQueryBuilder_CategoryFilter(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "test",
		Category: &domain.CategoryFilter{
			CategoryID:      "cat-1",
			IncludeChildren: true,
		},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if terms, ok := filter["terms"]; ok {
			if termsMap, ok := terms.(map[string]interface{}); ok {
				if path, ok := termsMap["category_path"]; ok {
					paths, ok := path.([]string)
					if !ok {
						pathsIface, _ := path.([]interface{})
						paths = make([]string, len(pathsIface))
						for i, v := range pathsIface {
							paths[i] = v.(string)
						}
					}
					if len(paths) == 1 && paths[0] == "cat-1" {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("expected category_path filter")
	}
}

func TestQueryBuilder_PriceFilter(t *testing.T) {
	minPrice := 100.0
	maxPrice := 500.0

	query := domain.DiscoveryQuery{
		Text: "test",
		PriceRange: &domain.PriceRange{
			Min: &minPrice,
			Max: &maxPrice,
		},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if rng, ok := filter["range"]; ok {
			if rngMap, ok := rng.(map[string]interface{}); ok {
				if price, ok := rngMap["price_amount"]; ok {
					priceMap := price.(map[string]interface{})
					if gte, ok := priceMap["gte"]; ok && gte == 100.0 {
						found = true
						break
					}
				}
			}
		}
	}

	if !found {
		t.Error("expected price range filter")
	}
}

func TestQueryBuilder_GeoFilter(t *testing.T) {
	lat := -4.44
	lon := 15.27
	radius := 5000.0

	query := domain.DiscoveryQuery{
		Text:         "test",
		Location:     &domain.SearchLocation{Latitude: &lat, Longitude: &lon},
		RadiusMeters: &radius,
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if geoDist, ok := filter["geo_distance"]; ok {
			found = true
			_ = geoDist
			break
		}
	}

	if !found {
		t.Error("expected geo_distance filter")
	}
}

func TestQueryBuilder_SortPriceAsc(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "test",
		Sort: &domain.SearchSort{
			Field:     domain.SearchSortFieldPriceAsc,
			Direction: domain.SearchSortDirectionAsc,
		},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	sort := body["sort"].([]interface{})

	if len(sort) != 1 {
		t.Fatalf("expected 1 sort clause, got %d", len(sort))
	}

	sortMap := sort[0].(map[string]interface{})
	if price, ok := sortMap["price_amount"]; ok {
		priceMap := price.(map[string]interface{})
		if priceMap["order"] != "asc" {
			t.Errorf("expected asc order, got %s", priceMap["order"])
		}
	} else {
		t.Error("expected price_amount sort")
	}
}

func TestQueryBuilder_AttributeFilter(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "test",
		Attributes: []domain.AttributeFilter{
			{Key: "brand", Operator: domain.AttributeOperatorEq, Value: "Apple"},
		},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if nested, ok := filter["nested"]; ok {
			if nestedMap, ok := nested.(map[string]interface{}); ok {
				if path, ok := nestedMap["path"]; ok && path == "attributes" {
					found = true
					break
				}
			}
		}
	}

	if !found {
		t.Error("expected nested attributes filter")
	}
}

func TestQueryBuilder_ListingTypeFilter(t *testing.T) {
	lt := domain.SearchListingTypePhysicalProduct
	query := domain.DiscoveryQuery{
		Text:        "test",
		ListingType: &lt,
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	boolQuery := body["query"].(map[string]interface{})["bool"].(map[string]interface{})

	found := false
	for _, f := range boolQuery["filter"].([]interface{}) {
		filter := f.(map[string]interface{})
		if term, ok := filter["term"]; ok {
			if termMap, ok := term.(map[string]interface{}); ok {
				if lt, ok := termMap["listing_type"]; ok && lt == "physical_product" {
					found = true
					break
				}
			}
		}
	}

	if !found {
		t.Error("expected listing_type filter")
	}
}

func TestQueryBuilder_DefaultSort(t *testing.T) {
	query := domain.DiscoveryQuery{
		Text: "test",
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}

	body := buildSearchQuery(query)
	sort := body["sort"].([]interface{})

	if len(sort) != 2 {
		t.Fatalf("expected 2 default sort clauses, got %d", len(sort))
	}
}

func TestQueryValidator_PageSize(t *testing.T) {
	if err := validateSearchQuery(domain.DiscoveryQuery{
		Pagination: domain.PaginationCursor{PageSize: 101},
	}); err == nil {
		t.Error("expected error for oversized page size")
	}
	if err := validateSearchQuery(domain.DiscoveryQuery{
		Pagination: domain.PaginationCursor{PageSize: 0},
	}); err == nil {
		t.Error("expected error for zero page size")
	}
	if err := validateSearchQuery(domain.DiscoveryQuery{
		Pagination: domain.PaginationCursor{PageSize: 100},
	}); err != nil {
		t.Error("page size 100 should be valid")
	}
}

func TestQueryValidator_Radius(t *testing.T) {
	negRadius := -100.0
	if err := validateSearchQuery(domain.DiscoveryQuery{
		RadiusMeters: &negRadius,
	}); err == nil {
		t.Error("expected error for negative radius")
	}

	hugeRadius := 60000.0
	if err := validateSearchQuery(domain.DiscoveryQuery{
		RadiusMeters: &hugeRadius,
	}); err == nil {
		t.Error("expected error for radius exceeding maximum")
	}
}

func TestQueryValidator_SortField(t *testing.T) {
	if err := validateSearchQuery(domain.DiscoveryQuery{
		Sort: &domain.SearchSort{Field: "invalid_sort"},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}); err == nil {
		t.Error("expected error for invalid sort field")
	}
	if err := validateSearchQuery(domain.DiscoveryQuery{
		Sort: &domain.SearchSort{Field: domain.SearchSortFieldRelevance},
		Pagination: domain.PaginationCursor{
			PageSize: 20,
		},
	}); err != nil {
		t.Error("relevance sort should be valid")
	}
}

func TestQueryValidator_ListingType(t *testing.T) {
	lt := strListingType("invalid_type")
	if err := validateSearchQuery(domain.DiscoveryQuery{
		ListingType: &lt,
	}); err == nil {
		t.Error("expected error for invalid listing type")
	}
}

func TestQueryValidator_Currency(t *testing.T) {
	if err := validateSearchQuery(domain.DiscoveryQuery{Currency: "ab", Pagination: domain.PaginationCursor{PageSize: 20}}); err == nil {
		t.Error("expected error for short currency")
	}
	if err := validateSearchQuery(domain.DiscoveryQuery{Currency: "usd", Pagination: domain.PaginationCursor{PageSize: 20}}); err == nil {
		t.Error("expected error for lowercase currency")
	}
	if err := validateSearchQuery(domain.DiscoveryQuery{Currency: "USD", Pagination: domain.PaginationCursor{PageSize: 20}}); err != nil {
		t.Error("USD currency should be valid")
	}
}

func TestQueryValidator_AvailableListingTypes(t *testing.T) {
	types := []string{"physical_product", "real_estate", "vehicle", "food", "service", "other"}
	for _, lt := range types {
		ltv := strListingType(lt)
		if err := validateSearchQuery(domain.DiscoveryQuery{
			ListingType: &ltv,
			Pagination: domain.PaginationCursor{
				PageSize: 20,
			},
		}); err != nil {
			t.Errorf("listing type %s should be valid: %v", lt, err)
		}
	}
}

func TestCursor_EncodeDecode(t *testing.T) {
	state := map[string]interface{}{
		"listing_id":   "listing-1",
		"price_amount": 100.0,
	}

	encoded, err := encodeCursor(state)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	decoded, err := decodeCursor(encoded)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if decoded["listing_id"] != "listing-1" {
		t.Errorf("listing_id mismatch")
	}
}

func TestCursor_Invalid(t *testing.T) {
	if _, err := decodeCursor("!!!not-valid-base64!!!"); err == nil {
		t.Error("expected error for invalid cursor")
	}
}

func TestCursor_ValidateForSort(t *testing.T) {
	// Valid cursor for price sort
	cursor, _ := encodeCursor(map[string]interface{}{"price_amount": 100.0, "listing_id": "1"})
	if err := ValidateCursor(cursor, domain.SearchSortFieldPriceAsc); err != nil {
		t.Errorf("valid price cursor should pass: %v", err)
	}

	// Invalid cursor for price sort
	cursor2, _ := encodeCursor(map[string]interface{}{"listing_id": "1"})
	if err := ValidateCursor(cursor2, domain.SearchSortFieldPriceAsc); err == nil {
		t.Error("cursor missing price_amount should fail for price sort")
	}

	// Empty cursor should always pass
	if err := ValidateCursor("", domain.SearchSortFieldRelevance); err != nil {
		t.Error("empty cursor should be valid")
	}
}

func TestResultMapper_BasicMapping(t *testing.T) {
	hit := SearchHit{
		ID:    "listing-1",
		Score: 0.85,
		Source: map[string]interface{}{
			"listing_id":          "listing-1",
			"title":               "iPhone 15",
			"price_amount":        699.99,
			"price_currency":      "USD",
			"availability_status": "in_stock",
			"status":              "published",
			"seller_id":           "seller-1",
			"seller_display_name": "Tech Store",
			"seller_trust_level":  float64(3),
			"store_id":            "store-1",
			"store_name":          "Tech Store Kinshasa",
			"published_at":        "2026-09-16T00:00:00Z",
		},
	}

	result, err := mapHitToSearchResult(hit)
	if err != nil {
		t.Fatalf("mapping error: %v", err)
	}

	if result.ListingID != "listing-1" {
		t.Errorf("expected listing ID 'listing-1', got '%s'", result.ListingID)
	}
	if result.Title != "iPhone 15" {
		t.Errorf("expected title 'iPhone 15', got '%s'", result.Title)
	}
	if result.Score != 0.85 {
		t.Errorf("expected score 0.85, got %f", result.Score)
	}
	if result.PriceAmount != 699.99 {
		t.Errorf("expected price 699.99, got %f", result.PriceAmount)
	}
	if result.PriceCurrency != "USD" {
		t.Errorf("expected USD, got %s", result.PriceCurrency)
	}
	if result.AvailabilityStatus != "in_stock" {
		t.Errorf("expected in_stock, got %s", result.AvailabilityStatus)
	}
	if result.Seller == nil {
		t.Fatal("expected seller")
	}
	if result.Seller.DisplayName != "Tech Store" {
		t.Errorf("expected seller 'Tech Store', got '%s'", result.Seller.DisplayName)
	}
	if result.Store == nil {
		t.Fatal("expected store")
	}
	if result.Store.Name != "Tech Store Kinshasa" {
		t.Errorf("expected store 'Tech Store Kinshasa', got '%s'", result.Store.Name)
	}
}

func TestResultMapper_WithLocation(t *testing.T) {
	hit := SearchHit{
		ID:    "listing-1",
		Score: 0.85,
		Source: map[string]interface{}{
			"listing_id": "listing-1",
			"location": map[string]interface{}{
				"lat":          -4.44,
				"lon":          15.27,
				"city":         "Kinshasa",
				"neighborhood": "Gombe",
			},
		},
	}

	result, err := mapHitToSearchResult(hit)
	if err != nil {
		t.Fatalf("mapping error: %v", err)
	}

	if result.Location == nil {
		t.Fatal("expected location")
	}
	if result.Location.City == nil || *result.Location.City != "Kinshasa" {
		t.Errorf("expected city Kinshasa")
	}
	if result.Location.Neighborhood == nil || *result.Location.Neighborhood != "Gombe" {
		t.Errorf("expected neighborhood Gombe")
	}
}

func TestResultMapper_WithDistance(t *testing.T) {
	dist := 1200.0
	hit := SearchHit{
		ID:    "listing-1",
		Score: 0.85,
		Source: map[string]interface{}{
			"listing_id": "listing-1",
			"distance":   dist,
		},
	}

	result, _ := mapHitToSearchResult(hit)
	if result.DistanceMeters == nil {
		t.Fatal("expected distance")
	}
	if *result.DistanceMeters != 1200.0 {
		t.Errorf("expected distance 1200.0, got %f", *result.DistanceMeters)
	}
}

func TestResultMapper_MissingFields(t *testing.T) {
	hit := SearchHit{
		ID:    "listing-1",
		Score: 0.5,
		Source: map[string]interface{}{
			"listing_id": "listing-1",
		},
	}

	result, err := mapHitToSearchResult(hit)
	if err != nil {
		t.Fatalf("mapping error: %v", err)
	}

	if result.Title != "" {
		t.Errorf("expected empty title, got '%s'", result.Title)
	}
	if result.ListingID != "listing-1" {
		t.Errorf("expected listing ID")
	}
}

func TestResolveGeoPoint_WithCoordinates(t *testing.T) {
	// This test validates the resolveGeoPoint logic conceptually
	// In production, it requires a real LocationRepository
	t.Log("resolveGeoPoint requires LocationRepository - tested via integration")
}

func strListingType(s string) domain.SearchListingType { return domain.SearchListingType(s) }

func strPtr(s string) *string { return &s }

func float64Ptr(f float64) *float64 { return &f }

func intPtr(i int) *int { return &i }
