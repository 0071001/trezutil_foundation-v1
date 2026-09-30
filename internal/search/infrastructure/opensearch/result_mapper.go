package opensearch

import (
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// mapHitToSearchResult maps an OpenSearch hit to a SearchResult.
func mapHitToSearchResult(hit SearchHit) (domain.SearchResult, error) {
	result := domain.SearchResult{
		ListingID: getStringField(hit.Source, "listing_id"),
		Title:     getStringField(hit.Source, "title"),
		Vertical:  getStringField(hit.Source, "vertical"),
		Score:     hit.Score,
	}

	if priceAmount, ok := hit.Source["price_amount"].(float64); ok {
		result.PriceAmount = priceAmount
	}

	result.PriceCurrency = getStringField(hit.Source, "price_currency")
	result.AvailabilityStatus = getStringField(hit.Source, "availability_status")
	result.PrimaryMediaURL = getOptionalString(hit.Source, "primary_media_url")

	if publishedAt, ok := hit.Source["published_at"].(string); ok && publishedAt != "" {
		// Parse ISO8601 timestamp
		result.PublishedAt = parseInt64Field(hit.Source, "published_at")
	}

	// Location (city, neighborhood from associated location)
	if loc, ok := hit.Source["location"].(map[string]interface{}); ok {
		result.Location = &domain.SearchResultLocation{
			City:         getOptionalStringFromMap(loc, "city"),
			Neighborhood: getOptionalStringFromMap(loc, "neighborhood"),
		}
	}

	// Seller
	if seller, ok := hit.Source["seller"].(map[string]interface{}); ok {
		result.Seller = &domain.SearchResultSeller{
			SellerID:    getStringField(hit.Source, "seller_id"),
			DisplayName: getStringField(seller, "display_name"),
			TrustLevel:  int(getFloatField(seller, "trust_level")),
		}
	} else {
		result.Seller = &domain.SearchResultSeller{
			SellerID:    getStringField(hit.Source, "seller_id"),
			DisplayName: getStringField(hit.Source, "seller_display_name"),
			TrustLevel:  int(getFloatField(hit.Source, "seller_trust_level")),
		}
	}

	// Store
	if store, ok := hit.Source["store"].(map[string]interface{}); ok {
		result.Store = &domain.SearchResultStore{
			StoreID: getStringField(hit.Source, "store_id"),
			Name:    getStringField(store, "name"),
		}
	} else {
		result.Store = &domain.SearchResultStore{
			StoreID: getStringField(hit.Source, "store_id"),
			Name:    getStringField(hit.Source, "store_name"),
		}
	}

	// Distance (if geo_distance was used in sort)
	if dist, ok := hit.Source["distance"].(float64); ok {
		result.DistanceMeters = &dist
	}

	return result, nil
}

func getStringField(source map[string]interface{}, field string) string {
	if v, ok := source[field]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getOptionalString(source map[string]interface{}, field string) *string {
	if v, ok := source[field]; ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func getOptionalStringFromMap(source map[string]interface{}, field string) *string {
	if v, ok := source[field]; ok {
		if s, ok := v.(string); ok && s != "" {
			return &s
		}
	}
	return nil
}

func getFloatField(source map[string]interface{}, field string) float64 {
	if v, ok := source[field]; ok {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

func getIntField(source map[string]interface{}, field string) int {
	return int(getFloatField(source, field))
}

func parseInt64Field(source map[string]interface{}, field string) int64 {
	if v, ok := source[field]; ok {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case int64:
			return val
		case int:
			return int64(val)
		case string:
			// ISO8601 timestamp
			t, err := parseISO8601(val)
			if err == nil {
				return t.Unix()
			}
		}
	}
	return 0
}

func parseISO8601(s string) (time.Time, error) {
	// Try common ISO8601 formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999Z",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse ISO8601: %s", s)
}
