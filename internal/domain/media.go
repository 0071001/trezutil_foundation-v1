package domain

import "time"

// MediaType represents the type of media.
type MediaType string

const (
	MediaTypeImage    MediaType = "image"
	MediaTypeVideo    MediaType = "video"
	MediaTypeDocument MediaType = "document"
)

// ListingMedia represents media attached to a listing.
type ListingMedia struct {
	ID        string
	ListingID string
	Type      MediaType
	URL       string
	Primary   bool
	SortOrder int
	CreatedAt int64
	UpdatedAt int64
}

// NewListingMedia creates a new ListingMedia.
func NewListingMedia(listingID string, mediaType MediaType, url string) *ListingMedia {
	return &ListingMedia{
		ListingID: listingID,
		Type:      mediaType,
		URL:       url,
		Primary:   false,
		SortOrder: 0,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
}
