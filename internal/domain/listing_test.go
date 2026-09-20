package domain

import (
	"testing"
)

func TestNewListing(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test Listing", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)

	if listing.OrganizationID != "org-1" {
		t.Errorf("Expected OrganizationID 'org-1', got '%s'", listing.OrganizationID)
	}
	if listing.SellerID != "seller-1" {
		t.Errorf("Expected SellerID 'seller-1', got '%s'", listing.SellerID)
	}
	if listing.StoreID != "store-1" {
		t.Errorf("Expected StoreID 'store-1', got '%s'", listing.StoreID)
	}
	if listing.Title != "Test Listing" {
		t.Errorf("Expected Title 'Test Listing', got '%s'", listing.Title)
	}
	if listing.PriceAmount != 100.0 {
		t.Errorf("Expected PriceAmount 100.0, got %f", listing.PriceAmount)
	}
	if listing.Status != ListingStatusDraft {
		t.Errorf("Expected Status 'draft', got '%s'", listing.Status)
	}
}

func TestListingPublish(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()

	if listing.Status != ListingStatusPublished {
		t.Errorf("Expected Status 'published', got '%s'", listing.Status)
	}
	if listing.PublishedAt == nil {
		t.Error("Expected PublishedAt to be set after publish")
	}
}

func TestListingPublishFromPaused(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()
	listing.Pause()
	listing.Publish()

	if listing.Status != ListingStatusPublished {
		t.Errorf("Expected Status 'published' after resume, got '%s'", listing.Status)
	}
}

func TestListingPause(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()
	listing.Pause()

	if listing.Status != ListingStatusPaused {
		t.Errorf("Expected Status 'paused', got '%s'", listing.Status)
	}
}

func TestListingSuspendFromPublished(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()
	listing.Suspend()

	if listing.Status != ListingStatusSuspended {
		t.Errorf("Expected Status 'suspended', got '%s'", listing.Status)
	}
}

func TestListingSuspendFromDraft(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Suspend()

	if listing.Status != ListingStatusDraft {
		t.Errorf("Expected Status to remain 'draft' when suspending from draft, got '%s'", listing.Status)
	}
}

func TestListingReject(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Status = ListingStatusPendingReview
	listing.Reject()

	if listing.Status != ListingStatusDraft {
		t.Errorf("Expected Status 'draft' after reject, got '%s'", listing.Status)
	}
}

func TestListingMarkAsSold(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()
	listing.MarkAsSold()

	if listing.Status != ListingStatusSold {
		t.Errorf("Expected Status 'sold', got '%s'", listing.Status)
	}
}

func TestListingMarkAsExpired(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
	listing.Publish()
	listing.MarkAsExpired()

	if listing.Status != ListingStatusExpired {
		t.Errorf("Expected Status 'expired', got '%s'", listing.Status)
	}
}

func TestListingCanBeModified(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)

	if !listing.CanBeModified() {
		t.Error("Expected draft listing to be modifiable")
	}

	listing.Status = ListingStatusPendingReview
	if !listing.CanBeModified() {
		t.Error("Expected pending_review listing to be modifiable")
	}

	listing.Status = ListingStatusPublished
	if listing.CanBeModified() {
		t.Error("Expected published listing to not be modifiable")
	}

	listing.Status = ListingStatusPaused
	if !listing.CanBeModified() {
		t.Error("Expected paused listing to be modifiable")
	}
}

func TestListingIsPublished(t *testing.T) {
	listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)

	if listing.IsPublished() {
		t.Error("Expected draft listing to not be published")
	}

	listing.Publish()
	if !listing.IsPublished() {
		t.Error("Expected published listing to be published")
	}
}

func TestListingIsTerminal(t *testing.T) {
	tests := []struct {
		status   ListingStatus
		expected bool
	}{
		{ListingStatusDraft, false},
		{ListingStatusPendingReview, false},
		{ListingStatusPublished, false},
		{ListingStatusPaused, false},
		{ListingStatusSold, true},
		{ListingStatusClosed, true},
		{ListingStatusExpired, true},
		{ListingStatusSuspended, true},
	}

	for _, tt := range tests {
		listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
		listing.Status = tt.status

		if listing.IsTerminal() != tt.expected {
			t.Errorf("Expected IsTerminal() to be %v for status '%s', got %v", tt.expected, tt.status, listing.IsTerminal())
		}
	}
}

func TestListingValidateTransition(t *testing.T) {
	tests := []struct {
		from      ListingStatus
		to        ListingStatus
		expectErr bool
	}{
		{ListingStatusDraft, ListingStatusPublished, false},
		{ListingStatusPendingReview, ListingStatusPublished, false},
		{ListingStatusPublished, ListingStatusPaused, false},
		{ListingStatusPaused, ListingStatusPublished, false},
		{ListingStatusPublished, ListingStatusSuspended, false},
		{ListingStatusPendingReview, ListingStatusDraft, false},
		{ListingStatusDraft, ListingStatusPaused, true},
		{ListingStatusPaused, ListingStatusPaused, false},
		{ListingStatusSold, ListingStatusPublished, true},
	}

	for _, tt := range tests {
		listing := NewListing("org-1", "seller-1", "store-1", "cat-1", "Test", string(ListingTypePhysicalProduct), string(PricingModelFixed), "USD", 100.0)
		listing.Status = tt.from

		err := listing.ValidateTransition(tt.to)
		if (err != nil) != tt.expectErr {
			t.Errorf("ValidateTransition(%s -> %s): expected error=%v, got error=%v", tt.from, tt.to, tt.expectErr, err != nil)
		}
	}
}
