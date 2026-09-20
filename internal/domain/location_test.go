package domain

import (
	"testing"
)

func TestNewLocation(t *testing.T) {
	loc := NewLocation("Kinshasa", LocationTypeCity, "CD")

	if loc.Name != "Kinshasa" {
		t.Errorf("Expected Name 'Kinshasa', got '%s'", loc.Name)
	}
	if loc.Type != LocationTypeCity {
		t.Errorf("Expected Type 'city', got '%s'", loc.Type)
	}
	if loc.Country != "CD" {
		t.Errorf("Expected Country 'CD', got '%s'", loc.Country)
	}
}

func TestNewListingLocation(t *testing.T) {
	ll := NewListingLocation("listing-1", "loc-1")

	if ll.ListingID != "listing-1" {
		t.Errorf("Expected ListingID 'listing-1', got '%s'", ll.ListingID)
	}
	if ll.LocationID != "loc-1" {
		t.Errorf("Expected LocationID 'loc-1', got '%s'", ll.LocationID)
	}
	if ll.IsPrimary {
		t.Error("Expected IsPrimary to be false")
	}
}

func TestNewUserLocation(t *testing.T) {
	ul := NewUserLocation("user-1", "loc-1")

	if ul.UserID != "user-1" {
		t.Errorf("Expected UserID 'user-1', got '%s'", ul.UserID)
	}
	if ul.LocationID != "loc-1" {
		t.Errorf("Expected LocationID 'loc-1', got '%s'", ul.LocationID)
	}
	if ul.IsDefault {
		t.Error("Expected IsDefault to be false")
	}
}
