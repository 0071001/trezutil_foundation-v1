package opensearch

import (
	"fmt"
	"strings"
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

	// A sale: the discount is derived from the two prices rather than trusted
	// from the index, so the badge can never disagree with what is shown.
	if compareAt, ok := hit.Source["compare_at_amount"].(float64); ok {
		if percent := domain.DiscountPercent(compareAt, result.PriceAmount); percent > 0 {
			result.CompareAtAmount, result.DiscountPercent = &compareAt, percent
		}
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
		// The indexed geo_point is {lat, lon}.
		lat, latOK := loc["lat"].(float64)
		lon, lonOK := loc["lon"].(float64)
		if latOK && lonOK {
			result.Location.Latitude, result.Location.Longitude = &lat, &lon
		}
	}
	// The indexed location is only a geo_point: the readable place ("Limete, Kinshasa": the listing's neighbourhood,
	// then city) is the "place" text. Without it a result would show no place at all.
	if place := strings.TrimSpace(getStringField(hit.Source, "place")); place != "" {
		if result.Location == nil {
			result.Location = &domain.SearchResultLocation{}
		}
		if result.Location.Neighborhood == nil && result.Location.City == nil {
			parts := strings.Split(place, ",")
			first := strings.TrimSpace(parts[0])
			if len(parts) > 1 {
				city := strings.TrimSpace(parts[1])
				result.Location.Neighborhood, result.Location.City = &first, &city
			} else {
				result.Location.City = &first
			}
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
