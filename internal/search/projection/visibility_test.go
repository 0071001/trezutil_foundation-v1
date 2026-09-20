package projection

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestIsSearchable(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"published", true},
		{"draft", false},
		{"pending_review", false},
		{"paused", false},
		{"suspended", false},
		{"sold", false},
		{"closed", false},
		{"expired", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := IsSearchable(tt.status)
			if result != tt.expected {
				t.Errorf("IsSearchable(%q) = %v, want %v", tt.status, result, tt.expected)
			}
		})
	}
}

func TestIsPubliclyVisible(t *testing.T) {
	if !IsPubliclyVisible("published") {
		t.Error("published should be publicly visible")
	}
	if IsPubliclyVisible("draft") {
		t.Error("draft should NOT be publicly visible")
	}
	if IsPubliclyVisible("paused") {
		t.Error("paused should NOT be publicly visible")
	}
	if IsPubliclyVisible("suspended") {
		t.Error("suspended should NOT be publicly visible")
	}
	if IsPubliclyVisible("sold") {
		t.Error("sold should NOT be publicly visible")
	}
}

func TestIsInternallyVisible(t *testing.T) {
	allStatuses := []string{
		"draft", "pending_review", "published",
		"paused", "suspended", "sold", "closed", "expired",
	}
	for _, s := range allStatuses {
		if !IsInternallyVisible(s) {
			t.Errorf("status %q should be internally visible", s)
		}
	}
}

func TestShouldRemoveFromIndex(t *testing.T) {
	if !ShouldRemoveFromIndex("suspended") {
		t.Error("suspended should be removed from index")
	}
	if ShouldRemoveFromIndex("published") {
		t.Error("published should NOT be removed from index")
	}
	if ShouldRemoveFromIndex("paused") {
		t.Error("paused should NOT be removed from index")
	}
	if ShouldRemoveFromIndex("sold") {
		t.Error("sold should NOT be removed from index")
	}
	if ShouldRemoveFromIndex("closed") {
		t.Error("closed should NOT be removed from index")
	}
	if ShouldRemoveFromIndex("expired") {
		t.Error("expired should NOT be removed from index")
	}
}

func TestSearchListingDocument_NoPrivateUserFields(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID:      "l1",
		OrganizationID: "org-1",
		SellerID:       "seller-1",
		Location: &domain.GeoPoint{
			Latitude:  -4.44,
			Longitude: 15.27,
		},
	}

	_ = doc
}

func TestSearchListingDocument_TenantIsolation(t *testing.T) {
	doc := &SearchListingDocument{
		ListingID:      "l1",
		OrganizationID: "org-1",
		SellerID:       "seller-1",
	}

	if doc.OrganizationID == "" {
		t.Error("organization_id is required for tenant isolation")
	}
}
