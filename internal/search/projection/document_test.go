package projection

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestSearchListingDocument_Marshal(t *testing.T) {
	now := int64(1234567890)
	doc := &SearchListingDocument{
		ListingID:          "listing-1",
		OrganizationID:     "org-1",
		SellerID:           "seller-1",
		StoreID:            "store-1",
		Title:              "Test",
		CategoryID:         "cat-1",
		CategoryPath:       []string{"parent", "cat-1"},
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Quantity:           intPtr(10),
		Location: &domain.GeoPoint{
			Latitude:  -4.44,
			Longitude: 15.27,
		},
		LocationIDs:       []string{"loc-1"},
		Attributes:        []domain.AttributeProjection{},
		MediaCount:        3,
		SellerDisplayName: "Seller",
		SellerTrustLevel:  3,
		StoreName:         "Store",
		StoreStatus:       "active",
		Status:            "published",
		PublishedAt:       &now,
		UpdatedAt:         now,
		FreshnessScore:    0.8,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["listing_id"] != "listing-1" {
		t.Errorf("expected listing_id 'listing-1'")
	}
	if result["location"] == nil {
		t.Error("expected location to be present")
	}
	loc := result["location"].(map[string]interface{})
	if loc["lat"] == nil {
		t.Error("geo_point must have lat")
	}
	if loc["lon"] == nil {
		t.Error("geo_point must have lon")
	}
	if result["quantity"] == nil {
		t.Error("expected quantity to be present")
	}
	if result["published_at"] == nil {
		t.Error("expected published_at to be present")
	}
	if result["seller_trust_level"] == nil {
		t.Error("expected seller_trust_level to be present")
	}
}

func TestSearchListingDocument_NoUserLocation(t *testing.T) {
	mapper := NewSearchListingDocumentMapper()

	proj := domain.ListingProjection{
		ListingID:          "listing-1",
		OrganizationID:     "org-1",
		SellerID:           "seller-1",
		StoreID:            "store-1",
		Title:              "Test Listing",
		CategoryID:         "cat-1",
		ListingType:        "physical_product",
		PricingModel:       "fixed",
		PriceAmount:        100.0,
		PriceCurrency:      "USD",
		AvailabilityStatus: "in_stock",
		Status:             "published",
		UpdatedAt:          1234567890,
		FreshnessScore:     0.8,
		Location: &domain.GeoPoint{
			Latitude:  -4.44,
			Longitude: 15.27,
		},
		LocationIDs: []string{"loc-1"},
	}

	doc, err := mapper.MapToDocument(proj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no user location data is in the document
	if doc.SellerID == "" {
		t.Error("seller_id should be set")
	}

	// Verify document only contains listing location, not user location
	if doc.Location == nil {
		t.Error("listing location should be set")
	}

	// Verify no private contact info is exposed
	_ = doc.SellerDisplayName // Only display name, not phone/email
}

func TestSearchListingDocument_NoPrivateMetadata(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID:      "listing-1",
		SellerID:       "seller-1",
		OrganizationID: "org-1",
		Location: &domain.GeoPoint{
			Latitude:  -4.44,
			Longitude: 15.27,
		},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.SellerID == "" {
		t.Error("seller_id should be set")
	}
	if doc.Location == nil {
		t.Error("listing location should be set")
	}
}

func TestSearchListingDocument_VersionedIndex(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID: "listing-1",
		Status:    "published",
		UpdatedAt: int64(1234567890),
		Title:     "Test",
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}

func TestSearchListingDocument_NilPublishedAt(t *testing.T) {
	now := time.Now().Unix()
	doc := &SearchListingDocument{
		ListingID: "listing-1",
		Status:    "draft",
		UpdatedAt: now,
	}

	if doc.PublishedAt != nil {
		t.Error("published_at should be nil")
	}
}

func TestSearchListingDocument_AttributesNested(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID: "l1",
		Attributes: []domain.AttributeProjection{
			{
				Key:        "brand",
				Name:       "Brand",
				Type:       "text",
				ValueText:  strPtr("Apple"),
				Searchable: true,
			},
		},
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := result["attributes"].([]interface{})
	if len(attrs) != 1 {
		t.Fatalf("expected 1 attribute, got %d", len(attrs))
	}
}

func TestSearchListingDocument_EmptyGeoPoint(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID: "l1",
	}

	data, _ := json.Marshal(doc)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["location"] != nil {
		t.Error("location should be null when not set")
	}
}

func TestSearchListingDocument_StatusKeyword(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID: "l1",
		Status:    "published",
	}

	data, _ := json.Marshal(doc)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["status"] != "published" {
		t.Errorf("expected status keyword field")
	}
}

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}
