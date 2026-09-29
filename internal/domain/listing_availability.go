package domain

import "time"

// AvailabilityStatus represents the availability status of a listing.
type AvailabilityStatus string

const (
	AvailabilityStatusInStock     AvailabilityStatus = "in_stock"
	AvailabilityStatusOutOfStock  AvailabilityStatus = "out_of_stock"
	AvailabilityStatusPreOrder    AvailabilityStatus = "pre_order"
	AvailabilityStatusUnavailable AvailabilityStatus = "unavailable"
)

// ListingAvailability represents the availability status of a listing.
type ListingAvailability struct {
	ID               string             `json:"id,omitempty"`
	ListingID        string             `json:"listing_id,omitempty"`
	Status           AvailabilityStatus `json:"status,omitempty"`
	Quantity         *int               `json:"quantity,omitempty"`
	ReservedQuantity *int               `json:"reserved_quantity,omitempty"`
	AvailableFrom    *int64             `json:"available_from,omitempty"`
	AvailableTo      *int64             `json:"available_to,omitempty"`
	CreatedAt        int64              `json:"created_at,omitempty"`
	UpdatedAt        int64              `json:"updated_at,omitempty"`
}

// NewListingAvailability creates a new ListingAvailability.
func NewListingAvailability(listingID string, status AvailabilityStatus) *ListingAvailability {
	now := time.Now().Unix()
	return &ListingAvailability{
		ID:        NewID(),
		ListingID: listingID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsAvailable returns true if the listing is currently available.
func (a *ListingAvailability) IsAvailable() bool {
	return a.Status == AvailabilityStatusInStock || a.Status == AvailabilityStatusPreOrder
}
