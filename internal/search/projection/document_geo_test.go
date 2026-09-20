package projection

import (
	"encoding/json"
	"testing"
)

func TestSearchListingDocument_NullPublishedAt(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID: "l1",
		Status:    "draft",
		UpdatedAt: 1234567890,
	}

	data, _ := json.Marshal(doc)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["published_at"] != nil {
		t.Error("published_at should be null for draft listings")
	}
}

func TestSearchListingDocument_OmmissionEmptyLists(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID:   "l1",
		Status:      "published",
		UpdatedAt:   1234567890,
		LocationIDs: []string{},
	}

	data, _ := json.Marshal(doc)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	if result["location_ids"] == nil {
		t.Error("location_ids should be present (empty array)")
	}
}
