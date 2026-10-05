package projection

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type placeLocations struct {
	domain.LocationRepository
	byID map[string]*domain.Location
}

func (p placeLocations) FindByID(ctx context.Context, id string) (*domain.Location, error) {
	if l, ok := p.byID[id]; ok {
		return l, nil
	}
	return nil, errors.New("not found")
}

func TestBuildPlace(t *testing.T) {
	s := func(v string) *string { return &v }
	store := &domain.Store{Address: s("12 Avenue du Fleuve, Gombe"), City: s("Kinshasa"), Province: s("Kinshasa")}
	locs := placeLocations{byID: map[string]*domain.Location{
		"loc-1": {Name: "Gombe", Neighborhood: s("Gombe"), City: s("Kinshasa")},
	}}
	got := buildPlace(context.Background(), locs, store, []domain.ListingLocation{{LocationID: "loc-1"}, {LocationID: "missing"}})
	if got != "Gombe, Kinshasa" {
		t.Fatalf("a listing is found by its own place, not its store's street: %q", got)
	}
	// A listing without a location of its own falls back to its store's address.
	if got := buildPlace(context.Background(), locs, store, nil); got != "12 Avenue du Fleuve, Gombe, Kinshasa" {
		t.Fatalf("fallback place = %q", got)
	}
	// A flat in Limete sold by a store in Gombe is not a Gombe flat.
	locs.byID["loc-2"] = &domain.Location{Name: "Limete", Neighborhood: s("Limete"), City: s("Kinshasa")}
	if got := buildPlace(context.Background(), locs, store, []domain.ListingLocation{{LocationID: "loc-2"}}); got != "Limete, Kinshasa" {
		t.Fatalf("place = %q", got)
	}
	if buildPlace(context.Background(), locs, &domain.Store{}, nil) != "" {
		t.Fatal("a store without an address has no place")
	}
}

func TestMapToDocument_VerticalAndPlace(t *testing.T) {
	doc, err := NewSearchListingDocumentMapper().MapToDocument(domain.ListingProjection{ListingID: "l1", Vertical: "HOTEL", Place: "Gombe, Kinshasa"})
	if err != nil || doc.Vertical != "HOTEL" || doc.Place != "Gombe, Kinshasa" {
		t.Fatalf("doc = %+v, %v", doc, err)
	}
}
