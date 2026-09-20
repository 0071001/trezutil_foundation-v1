package domain

import (
	"testing"
)

func TestNewListingAvailability(t *testing.T) {
	avail := NewListingAvailability("listing-1", AvailabilityStatusInStock)

	if avail.ListingID != "listing-1" {
		t.Errorf("Expected ListingID 'listing-1', got '%s'", avail.ListingID)
	}
	if avail.Status != AvailabilityStatusInStock {
		t.Errorf("Expected Status 'in_stock', got '%s'", avail.Status)
	}
}

func TestIsAvailable(t *testing.T) {
	avail := NewListingAvailability("listing-1", AvailabilityStatusInStock)
	if !avail.IsAvailable() {
		t.Error("Expected in_stock to be available")
	}

	availOut := NewListingAvailability("listing-1", AvailabilityStatusOutOfStock)
	if availOut.IsAvailable() {
		t.Error("Expected out_of_stock to not be available")
	}

	availPre := NewListingAvailability("listing-1", AvailabilityStatusPreOrder)
	if !availPre.IsAvailable() {
		t.Error("Expected pre_order to be available")
	}

	availUnavail := NewListingAvailability("listing-1", AvailabilityStatusUnavailable)
	if availUnavail.IsAvailable() {
		t.Error("Expected unavailable to not be available")
	}
}
