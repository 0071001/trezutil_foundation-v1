package opensearch

import "testing"

// A result shows its place: the index keeps it as text ("Limete, Kinshasa") beside a bare geo_point.
func TestResultReadsThePlaceText(t *testing.T) {
	r, err := mapHitToSearchResult(SearchHit{Source: map[string]interface{}{
		"listing_id": "l1", "title": "Ciment", "place": "Limete, Kinshasa",
		"location": map[string]interface{}{"lat": -4.353, "lon": 15.342},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if r.Location == nil || r.Location.Neighborhood == nil || *r.Location.Neighborhood != "Limete" || r.Location.City == nil || *r.Location.City != "Kinshasa" {
		t.Fatalf("place not read: %+v", r.Location)
	}
	if r.Location.Latitude == nil || *r.Location.Latitude != -4.353 {
		t.Errorf("coordinates lost: %+v", r.Location)
	}
	only, _ := mapHitToSearchResult(SearchHit{Source: map[string]interface{}{"listing_id": "l2", "title": "x", "place": "Kinshasa"}})
	if only.Location == nil || only.Location.City == nil || *only.Location.City != "Kinshasa" || only.Location.Neighborhood != nil {
		t.Errorf("city alone: %+v", only.Location)
	}
}
