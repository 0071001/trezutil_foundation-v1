// Package hotel is the native Hotel Commerce Pack: hotel concepts and rules
// over the Commerce Core. A room type is a HOTEL listing, its rooms are the
// listing's ROOM inventory, its nights are capacity units, a stay is a core
// reservation and a stay payment is a core order paid through Payment Core.
// This package adds no booking, availability or payment engine of its own.
package hotel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

var (
	// ErrNotRoomType: the listing is not a configured hotel room type.
	ErrNotRoomType = errors.New("not a hotel room type")
	// ErrNoHotel: the store has no published room types.
	ErrNoHotel = errors.New("no hotel at this property")
	// ErrStayNotPayable: the reservation cannot be paid online.
	ErrStayNotPayable = errors.New("stay cannot be paid online")
)

// ListingCreator is the core listing use case (authorization, vertical
// resolution, media and availability rows).
type ListingCreator interface {
	CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error
}

// StayOrders is the core order use case.
type StayOrders interface {
	CreateOrderIdempotent(ctx context.Context, actorID string, listingID string, quantity int, idempotencyKey string) (*domain.Order, error)
}

// NightCapacityStore reads and sets a resource's capacity windows.
type NightCapacityStore interface {
	FindByInventoryIDAndRange(ctx context.Context, tenantID, inventoryID string, startAt, endAt int64) ([]*domain.CapacityUnit, error)
	SetWindowCapacities(ctx context.Context, inventoryID string, windows []domain.CapacityWindow) error
	MaxWindowCapacityFrom(ctx context.Context, inventoryID string, from int64) (int64, error)
}

// Config wires the service to Commerce Core ports.
type Config struct {
	Listings     ListingCreator
	ListingRepo  domain.ListingRepository
	Media        domain.ListingMediaRepository
	RoomTypes    domain.HotelRoomTypeRepository
	Inventory    domain.InventoryRepository
	Capacity     NightCapacityStore
	Availability domain.AvailabilityProvider
	Reservations domain.ReservationRepository
	Orders       StayOrders
	Stores       domain.StoreRepository
	Orgs         domain.OrganizationRepository
	Users        domain.UserRepository
	Now          func() time.Time
}

// Service implements the Hotel pack use cases.
type Service struct{ c Config }

// NewService creates the Hotel pack service.
func NewService(c Config) *Service {
	if c.Now == nil {
		c.Now = time.Now
	}
	return &Service{c: c}
}

// NewRoomType is what a hotel seller enters for a room type.
type NewRoomType struct {
	StoreID          string   `json:"store_id"`
	SellerID         string   `json:"seller_id"`
	CategoryID       string   `json:"category_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	PriceAmount      float64  `json:"price_amount"`
	PriceCurrency    string   `json:"price_currency"`
	MaxOccupancy     int      `json:"max_occupancy"`
	BedConfiguration string   `json:"bed_configuration"`
	Amenities        []string `json:"amenities"`
	Units            int      `json:"units"`
	ImageURLs        []string `json:"image_urls"`
}

// RoomTypeView is a room type as sellers and customers see it.
type RoomTypeView struct {
	ListingID        string   `json:"listing_id"`
	OrganizationID   string   `json:"organization_id"`
	StoreID          string   `json:"store_id"`
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Status           string   `json:"status"`
	NightlyPrice     float64  `json:"nightly_price"`
	Currency         string   `json:"currency"`
	MaxOccupancy     int      `json:"max_occupancy"`
	BedConfiguration string   `json:"bed_configuration"`
	Amenities        []string `json:"amenities"`
	Units            int64    `json:"units"`
	Images           []string `json:"images"`
}

// CreateRoomType creates a HOTEL listing (draft), its room-type facts and its
// ROOM inventory. Authorization is the core listing rule: the seller, or the
// organization's owner/admin/manager.
func (s *Service) CreateRoomType(ctx context.Context, actorID string, in NewRoomType) (*RoomTypeView, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || len(title) > 255 {
		return nil, fmt.Errorf("%w: the room type needs a name of up to 255 characters", domain.ErrInvalidParameter)
	}
	if in.PriceAmount <= 0 || in.PriceAmount > 1e9 {
		return nil, fmt.Errorf("%w: the nightly price must be positive", domain.ErrInvalidParameter)
	}
	if in.Units < 1 || in.Units > domain.MaxHotelUnits {
		return nil, fmt.Errorf("%w: a room type has from 1 to %d rooms", domain.ErrInvalidParameter, domain.MaxHotelUnits)
	}
	rt := &domain.HotelRoomType{MaxOccupancy: in.MaxOccupancy, BedConfiguration: in.BedConfiguration, Amenities: in.Amenities}
	rt.Normalize()
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	var media []*domain.ListingMedia
	for i, u := range in.ImageURLs {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		if !strings.HasPrefix(strings.ToLower(u), "https://") || len(u) > 2048 || i >= 8 {
			return nil, fmt.Errorf("%w: images are up to 8 https:// links", domain.ErrInvalidParameter)
		}
		media = append(media, &domain.ListingMedia{Type: domain.MediaTypeImage, URL: u, Primary: len(media) == 0})
	}
	currency := strings.ToUpper(strings.TrimSpace(in.PriceCurrency))
	if currency == "" {
		currency = "USD"
	}
	desc := strings.TrimSpace(in.Description)
	listing := &domain.Listing{
		StoreID: in.StoreID, SellerID: in.SellerID, CategoryID: in.CategoryID, Title: title, Description: &desc,
		ListingType: domain.ListingTypeRealEstate, Vertical: domain.VerticalHotel, PricingModel: domain.PricingModelFixed,
		PriceAmount: in.PriceAmount, PriceCurrency: currency, Metadata: map[string]interface{}{},
	}
	if err := s.c.Listings.CreateListing(ctx, actorID, listing, nil, media, domain.NewListingAvailability("", domain.AvailabilityStatusInStock), nil); err != nil {
		return nil, err
	}
	now := s.c.Now().Unix()
	rt.ListingID, rt.OrganizationID, rt.Version, rt.CreatedAt, rt.UpdatedAt = listing.ID, listing.OrganizationID, 1, now, now
	undo := func() { _ = s.c.ListingRepo.Delete(ctx, listing.ID) }
	if err := s.c.RoomTypes.Create(ctx, rt); err != nil {
		undo()
		return nil, err
	}
	if err := s.c.Inventory.Create(ctx, domain.NewInventoryResource(listing.OrganizationID, listing.ID, domain.ResourceTypeRoom, int64(in.Units), now)); err != nil {
		undo()
		return nil, err
	}
	return s.view(ctx, listing, rt)
}

// RoomTypePatch changes hotel facts; nil fields are unchanged.
type RoomTypePatch struct {
	MaxOccupancy     *int     `json:"max_occupancy"`
	BedConfiguration *string  `json:"bed_configuration"`
	Amenities        []string `json:"amenities"`
}

// UpdateRoomType changes occupancy, beds or amenities (name, description and
// price are listing fields, edited through the core listing update).
func (s *Service) UpdateRoomType(ctx context.Context, actorID, listingID string, p RoomTypePatch) (*RoomTypeView, error) {
	listing, rt, err := s.roomType(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if !s.canManage(ctx, actorID, listing.OrganizationID) {
		return nil, domain.ErrInsufficientPermissions
	}
	if p.MaxOccupancy != nil {
		rt.MaxOccupancy = *p.MaxOccupancy
	}
	if p.BedConfiguration != nil {
		rt.BedConfiguration = *p.BedConfiguration
	}
	if p.Amenities != nil {
		rt.Amenities = p.Amenities
	}
	rt.Normalize()
	if err := rt.Validate(); err != nil {
		return nil, err
	}
	rt.Version++
	rt.UpdatedAt = s.c.Now().Unix()
	if err := s.c.RoomTypes.Update(ctx, rt); err != nil {
		return nil, err
	}
	return s.view(ctx, listing, rt)
}

// SetUnits sets how many rooms the room type has. A night can never offer
// more rooms than that, so units cannot drop below an upcoming night.
func (s *Service) SetUnits(ctx context.Context, actorID, listingID string, units int) (*RoomTypeView, error) {
	if units < 1 || units > domain.MaxHotelUnits {
		return nil, fmt.Errorf("%w: a room type has from 1 to %d rooms", domain.ErrInvalidParameter, domain.MaxHotelUnits)
	}
	listing, rt, err := s.roomType(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if !s.canManage(ctx, actorID, listing.OrganizationID) {
		return nil, domain.ErrInsufficientPermissions
	}
	inv, err := s.rooms(ctx, listing)
	if err != nil {
		return nil, err
	}
	max, err := s.c.Capacity.MaxWindowCapacityFrom(ctx, inv.ID, today(s.c.Now()))
	if err != nil {
		return nil, err
	}
	if int64(units) < max {
		return nil, fmt.Errorf("%w: an upcoming night offers %d rooms; lower its capacity first", domain.ErrCapacityConflict, max)
	}
	inv.Total = int64(units)
	inv.Version++
	inv.UpdatedAt = s.c.Now().Unix()
	if err := s.c.Inventory.Update(ctx, inv); err != nil {
		return nil, err
	}
	return s.view(ctx, listing, rt)
}

// SetNightlyCapacity offers `rooms` rooms on every night from `from` up to
// (not including) `to`. 0 closes those nights. It never drops a night below
// the rooms already booked on it; the whole range changes or nothing does.
func (s *Service) SetNightlyCapacity(ctx context.Context, actorID, listingID, from, to string, rooms int) ([]domain.NightCapacity, error) {
	nights, err := nightRange(from, to)
	if err != nil {
		return nil, err
	}
	listing, _, err := s.roomType(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if !s.canManage(ctx, actorID, listing.OrganizationID) {
		return nil, domain.ErrInsufficientPermissions
	}
	inv, err := s.rooms(ctx, listing)
	if err != nil {
		return nil, err
	}
	if rooms < 0 || int64(rooms) > inv.Total {
		return nil, fmt.Errorf("%w: a night offers from 0 to %d rooms (the room type's units)", domain.ErrInvalidParameter, inv.Total)
	}
	windows := make([]domain.CapacityWindow, 0, len(nights))
	for _, n := range nights {
		windows = append(windows, domain.CapacityWindow{StartAt: n, EndAt: n + 86400, Capacity: int64(rooms)})
	}
	if err := s.c.Capacity.SetWindowCapacities(ctx, inv.ID, windows); err != nil {
		return nil, err
	}
	return s.nights(ctx, listing, inv, nights)
}

// NightlyCapacity lists every night of the range with its rooms offered,
// booked and left (0 offered when the night is not open).
func (s *Service) NightlyCapacity(ctx context.Context, actorID, listingID, from, to string) ([]domain.NightCapacity, error) {
	nights, err := nightRange(from, to)
	if err != nil {
		return nil, err
	}
	listing, _, err := s.roomType(ctx, listingID)
	if err != nil {
		return nil, err
	}
	if !s.canView(ctx, actorID, listing.OrganizationID) {
		return nil, domain.ErrInsufficientPermissions
	}
	inv, err := s.rooms(ctx, listing)
	if err != nil {
		return nil, err
	}
	return s.nights(ctx, listing, inv, nights)
}

// RoomTypes lists an organization's room types (drafts included) for its
// members.
func (s *Service) RoomTypes(ctx context.Context, actorID, organizationID string) ([]RoomTypeView, error) {
	if organizationID == "" || !s.canView(ctx, actorID, organizationID) {
		return nil, domain.ErrInsufficientPermissions
	}
	listings, err := s.c.ListingRepo.FindByOrganizationID(ctx, organizationID, 0, 500)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, listings, false)
}

// PropertyView is a hotel as customers see it.
type PropertyView struct {
	StoreID   string         `json:"store_id"`
	Name      string         `json:"name"`
	City      string         `json:"city"`
	Address   string         `json:"address"`
	RoomTypes []RoomTypeView `json:"room_types"`
}

// Property returns a store's published room types (public).
func (s *Service) Property(ctx context.Context, storeID string) (*PropertyView, error) {
	store, err := s.c.Stores.FindByID(ctx, storeID)
	if err != nil || store == nil {
		return nil, ErrNoHotel
	}
	listings, err := s.c.ListingRepo.FindByStoreID(ctx, storeID, 0, 200)
	if err != nil {
		return nil, err
	}
	rooms, err := s.views(ctx, listings, true)
	if err != nil {
		return nil, err
	}
	if len(rooms) == 0 {
		return nil, ErrNoHotel
	}
	return &PropertyView{StoreID: store.ID, Name: store.Name, City: deref(store.City), Address: deref(store.Address), RoomTypes: rooms}, nil
}

// RoomOffer is one room type's answer for a stay.
type RoomOffer struct {
	RoomTypeView
	Rooms          int     `json:"rooms"`
	AvailableRooms int64   `json:"available_rooms"`
	Available      bool    `json:"available"`
	FitsGuests     bool    `json:"fits_guests"`
	Nights         int     `json:"nights"`
	TotalPrice     float64 `json:"total_price"`
	Reason         string  `json:"reason,omitempty"`
}

// StayQuote answers "which rooms are free for these dates and guests".
type StayQuote struct {
	StoreID  string      `json:"store_id"`
	CheckIn  string      `json:"check_in"`
	CheckOut string      `json:"check_out"`
	Nights   int         `json:"nights"`
	Guests   int         `json:"guests"`
	Offers   []RoomOffer `json:"offers"`
}

// Availability checks every published room type of a property for a stay
// through the core availability port (the tightest night decides). rooms 0
// means "as many as the guests need" per room type.
func (s *Service) Availability(ctx context.Context, storeID string, stay domain.StayDates, guests, rooms int) (*StayQuote, error) {
	if guests < 1 || guests > 100 {
		return nil, fmt.Errorf("%w: guests must be from 1 to 100", domain.ErrInvalidParameter)
	}
	if rooms < 0 || rooms > domain.MaxHotelUnits {
		return nil, fmt.Errorf("%w: rooms must be from 1 to %d", domain.ErrInvalidParameter, domain.MaxHotelUnits)
	}
	property, err := s.Property(ctx, storeID)
	if err != nil {
		return nil, err
	}
	quote := &StayQuote{StoreID: storeID, CheckIn: domain.NightDate(stay.StartAt()), CheckOut: domain.NightDate(stay.EndAt()), Nights: stay.Nights(), Guests: guests}
	for _, rt := range property.RoomTypes {
		want := rooms
		if want == 0 {
			want = (&domain.HotelRoomType{MaxOccupancy: rt.MaxOccupancy}).RoomsNeeded(guests)
		}
		offer := RoomOffer{RoomTypeView: rt, Rooms: want, Nights: stay.Nights(), FitsGuests: guests <= rt.MaxOccupancy*want,
			TotalPrice: rt.NightlyPrice * float64(stay.Nights()*want)}
		res, err := s.c.Availability.CheckAvailability(ctx, domain.AvailabilityQuery{
			TenantID: rt.OrganizationID, ListingID: rt.ListingID, StartAt: stay.StartAt(), EndAt: stay.EndAt(), Quantity: want, PartySize: guests,
		})
		switch {
		case err != nil:
			offer.Reason = "availability could not be checked"
		default:
			offer.AvailableRooms = int64(res.Remaining)
			offer.Available = res.Available && offer.FitsGuests
			offer.Reason = res.Reason
			if !offer.FitsGuests {
				offer.Reason = fmt.Sprintf("%d guests need more rooms of this type", guests)
			}
		}
		quote.Offers = append(quote.Offers, offer)
	}
	return quote, nil
}

// CheckReservation is the Hotel pack's booking rule (domain.ReservationRule):
// a stay of a configured room type covers whole nights (UTC dates), lasts 1 to
// MaxStayNights nights and fits the rooms' occupancy.
func (s *Service) CheckReservation(ctx context.Context, listing *domain.Listing, startAt, endAt int64, quantity, partySize int) error {
	if listing == nil || listing.Vertical != domain.VerticalHotel {
		return nil
	}
	rt, err := s.c.RoomTypes.FindByListingID(ctx, listing.ID)
	if errors.Is(err, domain.ErrHotelRoomTypeNotFound) {
		return nil // a HOTEL listing that is not a room type follows the generic rules
	}
	if err != nil {
		return err
	}
	if _, err := domain.NewStay(startAt, endAt); err != nil {
		return err
	}
	if quantity < 1 {
		quantity = 1
	}
	if partySize < 1 {
		return fmt.Errorf("%w: say how many guests are staying", domain.ErrInvalidParameter)
	}
	if partySize > rt.MaxOccupancy*quantity {
		return fmt.Errorf("%w: %d guests need %d rooms of this type (up to %d guests each)", domain.ErrInvalidParameter, partySize, rt.RoomsNeeded(partySize), rt.MaxOccupancy)
	}
	return nil
}

// CheckoutStay returns the order that pays a hotel stay: nights × rooms at the
// nightly rate, created once per reservation (key "stay:<reservation id>") and
// paid through the normal payment flow. Only the guest who booked can pay.
func (s *Service) CheckoutStay(ctx context.Context, actorID, reservationID string) (*domain.Order, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	res, err := s.c.Reservations.FindByID(ctx, reservationID)
	if err != nil || res == nil {
		return nil, domain.ErrReservationNotFound
	}
	if res.CustomerID != actorID {
		return nil, domain.ErrInsufficientPermissions
	}
	if res.Status != domain.ReservationStatusPending && res.Status != domain.ReservationStatusConfirmed {
		return nil, fmt.Errorf("%w: the reservation is %s", ErrStayNotPayable, res.Status)
	}
	listing, err := s.c.ListingRepo.FindByID(ctx, res.ListingID)
	if err != nil || listing.Vertical != domain.VerticalHotel {
		return nil, fmt.Errorf("%w: only hotel stays are paid online from a reservation", ErrStayNotPayable)
	}
	stay, err := domain.NewStay(res.StartAt, res.EndAt)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStayNotPayable, err)
	}
	return s.c.Orders.CreateOrderIdempotent(ctx, actorID, listing.ID, stay.Nights()*res.Quantity, "stay:"+res.ID)
}

// --- helpers ---

func (s *Service) roomType(ctx context.Context, listingID string) (*domain.Listing, *domain.HotelRoomType, error) {
	listing, err := s.c.ListingRepo.FindByID(ctx, listingID)
	if err != nil || listing == nil || listing.Vertical != domain.VerticalHotel {
		return nil, nil, ErrNotRoomType
	}
	rt, err := s.c.RoomTypes.FindByListingID(ctx, listingID)
	if errors.Is(err, domain.ErrHotelRoomTypeNotFound) {
		return nil, nil, ErrNotRoomType
	}
	if err != nil {
		return nil, nil, err
	}
	return listing, rt, nil
}

func (s *Service) rooms(ctx context.Context, listing *domain.Listing) (*domain.InventoryResource, error) {
	inv, err := s.c.Inventory.FindByListingID(ctx, listing.OrganizationID, listing.ID)
	if err != nil || inv == nil || inv.ResourceType != domain.ResourceTypeRoom {
		return nil, ErrNotRoomType
	}
	return inv, nil
}

func (s *Service) nights(ctx context.Context, listing *domain.Listing, inv *domain.InventoryResource, nights []int64) ([]domain.NightCapacity, error) {
	units, err := s.c.Capacity.FindByInventoryIDAndRange(ctx, listing.OrganizationID, inv.ID, nights[0], nights[len(nights)-1]+86400)
	if err != nil {
		return nil, err
	}
	byStart := map[int64]*domain.CapacityUnit{}
	for _, u := range units {
		byStart[u.StartAt] = u
	}
	out := make([]domain.NightCapacity, 0, len(nights))
	for _, n := range nights {
		nc := domain.NightCapacity{Date: domain.NightDate(n)}
		if u := byStart[n]; u != nil {
			nc.Capacity, nc.Reserved, nc.Available = u.Capacity, u.Reserved, u.Available()
		}
		out = append(out, nc)
	}
	return out, nil
}

func (s *Service) views(ctx context.Context, listings []domain.Listing, publishedOnly bool) ([]RoomTypeView, error) {
	var ids []string
	byID := map[string]*domain.Listing{}
	for i := range listings {
		l := &listings[i]
		if l.Vertical != domain.VerticalHotel || (publishedOnly && !l.IsPublished()) {
			continue
		}
		ids = append(ids, l.ID)
		byID[l.ID] = l
	}
	facts, err := s.c.RoomTypes.FindByListingIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := []RoomTypeView{}
	for _, id := range ids {
		rt, ok := facts[id]
		if !ok {
			continue // a HOTEL listing that is not a room type
		}
		v, err := s.view(ctx, byID[id], rt)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, nil
}

func (s *Service) view(ctx context.Context, l *domain.Listing, rt *domain.HotelRoomType) (*RoomTypeView, error) {
	v := &RoomTypeView{
		ListingID: l.ID, OrganizationID: l.OrganizationID, StoreID: l.StoreID, Title: l.Title, Description: deref(l.Description),
		Status: string(l.Status), NightlyPrice: l.PriceAmount, Currency: l.PriceCurrency, MaxOccupancy: rt.MaxOccupancy,
		BedConfiguration: rt.BedConfiguration, Amenities: rt.Amenities, Images: []string{},
	}
	if v.Amenities == nil {
		v.Amenities = []string{}
	}
	if inv, err := s.c.Inventory.FindByListingID(ctx, l.OrganizationID, l.ID); err == nil && inv != nil {
		v.Units = inv.Total
	}
	if s.c.Media != nil {
		if media, err := s.c.Media.FindByListingID(ctx, l.ID); err == nil {
			for _, m := range media {
				if m.Primary {
					v.Images = append([]string{m.URL}, v.Images...)
				} else {
					v.Images = append(v.Images, m.URL)
				}
			}
		}
	}
	return v, nil
}

// canManage: the organization's owner/admin/manager, or a platform
// admin/operator. canView adds staff.
func (s *Service) canManage(ctx context.Context, actorID, orgID string) bool {
	return s.hasRole(ctx, actorID, orgID, domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager)
}

func (s *Service) canView(ctx context.Context, actorID, orgID string) bool {
	return s.hasRole(ctx, actorID, orgID, domain.OrganizationRoleOwner, domain.OrganizationRoleAdmin, domain.OrganizationRoleManager, domain.OrganizationRoleStaff)
}

func (s *Service) hasRole(ctx context.Context, actorID, orgID string, roles ...domain.OrganizationRole) bool {
	if actorID == "" || orgID == "" {
		return false
	}
	user, err := s.c.Users.FindByID(ctx, actorID)
	if err != nil || user == nil || user.IsSuspended() {
		return false
	}
	if user.HasRole(domain.RoleAdmin) || user.HasRole(domain.RoleOperator) {
		return true
	}
	role, err := s.c.Orgs.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return false
	}
	for _, r := range roles {
		if role == r {
			return true
		}
	}
	return false
}

// nightRange returns the UTC midnights of the nights from `from` up to (not
// including) `to`.
func nightRange(from, to string) ([]int64, error) {
	start, err := domain.ParseNight(from)
	if err != nil {
		return nil, fmt.Errorf("%w: from must be a date (YYYY-MM-DD)", domain.ErrInvalidParameter)
	}
	end, err := domain.ParseNight(to)
	if err != nil {
		return nil, fmt.Errorf("%w: to must be a date (YYYY-MM-DD)", domain.ErrInvalidParameter)
	}
	n := int(end.Sub(start).Hours() / 24)
	if n < 1 || n > domain.MaxCapacityNights {
		return nil, fmt.Errorf("%w: choose from 1 to %d nights", domain.ErrInvalidParameter, domain.MaxCapacityNights)
	}
	out := make([]int64, n)
	for i := range out {
		out[i] = start.Unix() + int64(i)*86400
	}
	return out, nil
}

func today(now time.Time) int64 {
	y, m, d := now.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
