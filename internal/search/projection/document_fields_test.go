package projection

import (
	"encoding/json"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestSearchListingDocument_JSONHasRequiredFields(t *testing.T) {
	now := int64(1234567890)
	doc := SearchListingDocument{
		ListingID:          "l1",
		OrganizationID:     "o1",
		SellerID:           "s1",
		StoreID:            "st1",
		Title:              "Test",
		Description:        "Desc",
		CategoryID:         "c1",
		CategoryPath:       []string{"c1"},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Location:           &domain.GeoPoint{Latitude: -4.44, Longitude: 15.27},
		LocationIDs:        []string{"loc1"},
		Attributes:         []domain.AttributeProjection{},
		MediaCount:         1,
		SellerDisplayName:  "Seller",
		SellerTrustLevel:   3,
		StoreName:          "Store",
		StoreStatus:        "active",
		Status:             "published",
		PublishedAt:        &now,
		UpdatedAt:          now,
		FreshnessScore:     0.5,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	requiredFields := []string{
		"listing_id", "organization_id", "seller_id", "store_id",
		"title", "description", "category_id", "category_path",
		"listing_type", "pricing_model", "price_amount", "price_currency",
		"availability_status", "location", "location_ids",
		"attributes", "media_count", "seller_display_name", "seller_trust_level",
		"store_name", "store_status", "status", "published_at", "updated_at",
		"freshness_score",
	}

	for _, field := range requiredFields {
		if _, exists := raw[field]; !exists {
			t.Errorf("required field %q missing from JSON", field)
		}
	}
}
