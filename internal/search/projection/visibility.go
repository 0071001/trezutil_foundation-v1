package projection

import "github.com/trezutil/trezutil/internal/domain"

// IsSearchable returns true if the listing status allows the document to exist in the index.
func IsSearchable(status string) bool {
	return status == string(domain.ListingStatusPublished)
}

// IsPubliclyVisible returns true if the listing is visible in public search results.
func IsPubliclyVisible(status string) bool {
	return status == string(domain.ListingStatusPublished)
}

// IsInternallyVisible returns true if the listing is visible in internal search results.
// Internal search includes non-published listings for seller/store/org/admin actors.
func IsInternallyVisible(status string) bool {
	switch status {
	case string(domain.ListingStatusDraft),
		string(domain.ListingStatusPendingReview),
		string(domain.ListingStatusPublished),
		string(domain.ListingStatusPaused),
		string(domain.ListingStatusSuspended),
		string(domain.ListingStatusSold),
		string(domain.ListingStatusClosed),
		string(domain.ListingStatusExpired):
		return true
	}
	return false
}

// ShouldRemoveFromIndex returns true if the listing should be removed from the search index.
// SUSPENDED is the only status that removes from ALL search (public + internal).
func ShouldRemoveFromIndex(status string) bool {
	return status == string(domain.ListingStatusSuspended)
}
