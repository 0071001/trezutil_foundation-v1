package domain

import "time"

// LocationType represents the type of location.
type LocationType string

const (
	LocationTypeCountry      LocationType = "country"
	LocationTypeProvince     LocationType = "province"
	LocationTypeCity         LocationType = "city"
	LocationTypeNeighborhood LocationType = "neighborhood"
)

// Location represents a geographic location.
type Location struct {
	ID           string
	ParentID     *string
	Name         string
	Type         LocationType
	Country      string
	Province     *string
	City         *string
	Neighborhood *string
	Latitude     *float64
	Longitude    *float64
	Timezone     *string
	CreatedAt    int64
	UpdatedAt    int64
}

// NewLocation creates a new Location.
func NewLocation(name string, locationType LocationType, country string) *Location {
	return &Location{
		Name:      name,
		Type:      locationType,
		Country:   country,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
}

// ListingLocation represents a location associated with a listing.
type ListingLocation struct {
	ID         string
	ListingID  string
	LocationID string
	Label      *string
	IsPrimary  bool
	CreatedAt  int64
	UpdatedAt  int64
}

// NewListingLocation creates a new ListingLocation.
func NewListingLocation(listingID, locationID string) *ListingLocation {
	return &ListingLocation{
		ListingID:  listingID,
		LocationID: locationID,
		IsPrimary:  false,
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}
}

// UserLocation represents a user's location.
type UserLocation struct {
	ID         string
	UserID     string
	LocationID string
	Label      *string
	IsDefault  bool
	CreatedAt  int64
	UpdatedAt  int64
}

// NewUserLocation creates a new UserLocation.
func NewUserLocation(userID, locationID string) *UserLocation {
	return &UserLocation{
		UserID:     userID,
		LocationID: locationID,
		IsDefault:  false,
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}
}
