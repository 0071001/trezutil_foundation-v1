package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Hotel Commerce Pack — hotel concepts over the Commerce Core.
//
//	Property      → Store
//	Room type     → Listing (vertical HOTEL): name, description, nightly rate, media
//	                + HotelRoomType: occupancy, beds, amenities
//	Room / unit   → the listing's ROOM InventoryResource (Total = number of rooms)
//	Nightly room  → CapacityUnit of that resource covering one UTC night
//	Stay          → Reservation (quantity = rooms, party size = guests)
//	Stay payment  → Order for nights × rooms at the nightly rate, paid through Payment Core
//
// There is no hotel booking, availability or payment engine.

// Hotel pack limits.
const (
	MaxHotelOccupancy  = 20
	MaxStayNights      = 30
	MaxHotelUnits      = 500
	MaxCapacityNights  = 366
	maxAmenities       = 30
	maxAmenityLength   = 60
	maxBedConfigLength = 120
	secondsPerNight    = 86400
)

var (
	ErrHotelRoomTypeNotFound = errors.New("hotel room type not found")
	// ErrCapacityConflict: a nightly capacity change would drop below rooms
	// already booked (or units below a configured night).
	ErrCapacityConflict = errors.New("capacity conflict")
)

// HotelRoomType holds the hotel-only facts of a room type. Everything else
// (name, price, media, status, owner) is its Listing.
type HotelRoomType struct {
	ListingID        string
	OrganizationID   string
	MaxOccupancy     int
	BedConfiguration string
	Amenities        []string
	Version          int64
	CreatedAt        int64
	UpdatedAt        int64
}

// Normalize trims and de-duplicates amenities and the bed configuration.
func (r *HotelRoomType) Normalize() {
	r.BedConfiguration = strings.TrimSpace(r.BedConfiguration)
	seen := map[string]bool{}
	out := make([]string, 0, len(r.Amenities))
	for _, a := range r.Amenities {
		a = strings.TrimSpace(a)
		if a == "" || seen[strings.ToLower(a)] {
			continue
		}
		seen[strings.ToLower(a)] = true
		out = append(out, a)
	}
	r.Amenities = out
}

// Validate checks the room type's own invariants.
func (r *HotelRoomType) Validate() error {
	if r.MaxOccupancy < 1 || r.MaxOccupancy > MaxHotelOccupancy {
		return fmt.Errorf("%w: occupancy must be from 1 to %d guests", ErrInvalidParameter, MaxHotelOccupancy)
	}
	if len(r.BedConfiguration) > maxBedConfigLength {
		return fmt.Errorf("%w: bed configuration must be at most %d characters", ErrInvalidParameter, maxBedConfigLength)
	}
	if len(r.Amenities) > maxAmenities {
		return fmt.Errorf("%w: at most %d amenities", ErrInvalidParameter, maxAmenities)
	}
	for _, a := range r.Amenities {
		if len(a) > maxAmenityLength {
			return fmt.Errorf("%w: amenity %q is longer than %d characters", ErrInvalidParameter, a, maxAmenityLength)
		}
	}
	return nil
}

// RoomsNeeded is how many rooms of this type a party of guests needs.
func (r *HotelRoomType) RoomsNeeded(guests int) int {
	if guests <= 0 || r.MaxOccupancy <= 0 {
		return 1
	}
	return (guests + r.MaxOccupancy - 1) / r.MaxOccupancy
}

// HotelRoomTypeRepository persists room-type facts.
type HotelRoomTypeRepository interface {
	Create(ctx context.Context, rt *HotelRoomType) error
	FindByListingID(ctx context.Context, listingID string) (*HotelRoomType, error)
	FindByListingIDs(ctx context.Context, listingIDs []string) (map[string]*HotelRoomType, error)
	Update(ctx context.Context, rt *HotelRoomType) error
}

// StayDates is a hotel stay in nights: check-in and check-out are calendar
// dates, and the stay covers each night from check-in up to (not including)
// check-out. Nights are UTC calendar days, so a stay's timestamps are UTC
// midnights and line up with the nightly capacity units.
type StayDates struct {
	CheckIn  time.Time
	CheckOut time.Time
}

// ParseStay reads "YYYY-MM-DD" check-in and check-out dates.
func ParseStay(checkIn, checkOut string) (StayDates, error) {
	in, err := ParseNight(checkIn)
	if err != nil {
		return StayDates{}, fmt.Errorf("%w: check_in must be a date (YYYY-MM-DD)", ErrInvalidParameter)
	}
	out, err := ParseNight(checkOut)
	if err != nil {
		return StayDates{}, fmt.Errorf("%w: check_out must be a date (YYYY-MM-DD)", ErrInvalidParameter)
	}
	return NewStay(in.Unix(), out.Unix())
}

// NewStay builds a stay from UTC-midnight timestamps and checks its length.
func NewStay(startAt, endAt int64) (StayDates, error) {
	if !IsNightBoundary(startAt) || !IsNightBoundary(endAt) {
		return StayDates{}, fmt.Errorf("%w: a stay starts and ends on a date (UTC midnight)", ErrInvalidParameter)
	}
	s := StayDates{CheckIn: time.Unix(startAt, 0).UTC(), CheckOut: time.Unix(endAt, 0).UTC()}
	if n := s.Nights(); n < 1 || n > MaxStayNights {
		return StayDates{}, fmt.Errorf("%w: a stay is from 1 to %d nights", ErrInvalidParameter, MaxStayNights)
	}
	return s, nil
}

// Nights is the number of nights of the stay.
func (s StayDates) Nights() int {
	return int(s.CheckOut.Sub(s.CheckIn).Hours() / 24)
}

// StartAt and EndAt are the reservation period of the stay.
func (s StayDates) StartAt() int64 { return s.CheckIn.Unix() }
func (s StayDates) EndAt() int64   { return s.CheckOut.Unix() }

// ParseNight reads a "YYYY-MM-DD" date as the UTC midnight starting that night.
func ParseNight(date string) (time.Time, error) {
	return time.Parse("2006-01-02", strings.TrimSpace(date))
}

// IsNightBoundary reports whether ts is a UTC midnight.
func IsNightBoundary(ts int64) bool {
	return ts > 0 && ts%secondsPerNight == 0
}

// NightDate formats the night starting at ts.
func NightDate(ts int64) string {
	return time.Unix(ts, 0).UTC().Format("2006-01-02")
}

// NightCapacity is one night of a room type: rooms offered, booked and left.
type NightCapacity struct {
	Date      string `json:"date"`
	Capacity  int64  `json:"capacity"`
	Reserved  int64  `json:"reserved"`
	Available int64  `json:"available"`
}

// CapacityWindow sets the capacity of one exact window of a resource (for a
// hotel, one night). Capacity 0 closes the window.
type CapacityWindow struct {
	StartAt  int64
	EndAt    int64
	Capacity int64
}
