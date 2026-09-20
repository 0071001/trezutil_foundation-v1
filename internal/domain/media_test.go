package domain

import (
	"testing"
)

func TestNewListingMedia(t *testing.T) {
	media := NewListingMedia("listing-1", MediaTypeImage, "https://example.com/image.jpg")

	if media.ListingID != "listing-1" {
		t.Errorf("Expected ListingID 'listing-1', got '%s'", media.ListingID)
	}
	if media.Type != MediaTypeImage {
		t.Errorf("Expected Type 'image', got '%s'", media.Type)
	}
	if media.URL != "https://example.com/image.jpg" {
		t.Errorf("Expected URL 'https://example.com/image.jpg', got '%s'", media.URL)
	}
	if media.Primary {
		t.Error("Expected Primary to be false")
	}
	if media.SortOrder != 0 {
		t.Errorf("Expected SortOrder 0, got %d", media.SortOrder)
	}
}
