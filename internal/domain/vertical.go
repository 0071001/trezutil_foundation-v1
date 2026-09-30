package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// Vertical identifies a commerce vertical: a business domain (hotels,
// restaurants, retail…) whose capabilities and rules extend the generic
// Commerce Core. A vertical never brings its own order, reservation, payment
// or seller engine. Codes are open: a new vertical is a new catalogue entry
// (application/vertical), not a Commerce Core change.
type Vertical string

// The initial verticals. See docs/architecture/VERTICAL_COMMERCE_PACKS.md.
const (
	VerticalRetail        Vertical = "RETAIL"
	VerticalServices      Vertical = "SERVICES"
	VerticalHotel         Vertical = "HOTEL"
	VerticalRestaurant    Vertical = "RESTAURANT"
	VerticalRealEstate    Vertical = "REAL_ESTATE"
	VerticalVehicleRental Vertical = "VEHICLE_RENTAL"
	VerticalTransport     Vertical = "TRANSPORT"
	VerticalEvents        Vertical = "EVENTS"
	VerticalTravel        Vertical = "TRAVEL"
	VerticalOther         Vertical = "OTHER"
)

var verticalCodePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,31}$`)

// NormalizeVertical turns user or client input ("hotel", "Real estate",
// "vehicle-rental") into a vertical code. It does not check that the vertical
// exists; the vertical catalogue does.
func NormalizeVertical(raw string) Vertical {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.NewReplacer("-", "_", " ", "_").Replace(s)
	return Vertical(s)
}

// IsWellFormed reports whether v has the shape of a vertical code (the same
// rule as the listings.vertical database constraint).
func (v Vertical) IsWellFormed() bool {
	return verticalCodePattern.MatchString(string(v))
}

// ListingTypes returns every listing type the Commerce Core accepts.
func ListingTypes() []ListingType {
	return []ListingType{
		ListingTypePhysicalProduct, ListingTypeRealEstate, ListingTypeVehicle,
		ListingTypeFood, ListingTypeService, ListingTypeOther,
	}
}

// DefaultVerticalForListingType is the vertical of a listing created without
// an explicit vertical. It preserves how each listing type behaves today:
// vehicles are sold (the web app offers them for purchase), so rentals must say
// VEHICLE_RENTAL explicitly. Migration 000019 backfills existing rows with the
// same mapping.
func DefaultVerticalForListingType(t ListingType) Vertical {
	switch t {
	case ListingTypePhysicalProduct, ListingTypeVehicle:
		return VerticalRetail
	case ListingTypeFood:
		return VerticalRestaurant
	case ListingTypeService:
		return VerticalServices
	case ListingTypeRealEstate:
		return VerticalRealEstate
	default:
		return VerticalOther
	}
}

// CoreEntity names a Commerce Core building block.
type CoreEntity string

const (
	CoreOrganization CoreEntity = "ORGANIZATION"
	CoreSeller       CoreEntity = "SELLER"
	CoreStore        CoreEntity = "STORE"
	CoreListing      CoreEntity = "LISTING"
	CoreCategory     CoreEntity = "CATEGORY"
	CoreAttribute    CoreEntity = "ATTRIBUTE"
	CoreMedia        CoreEntity = "MEDIA"
	CoreLocation     CoreEntity = "LOCATION"
	CorePrice        CoreEntity = "PRICE"
	CoreInventory    CoreEntity = "INVENTORY_RESOURCE"
	CoreCapacity     CoreEntity = "CAPACITY_UNIT"
	CoreReservation  CoreEntity = "RESERVATION"
	CoreOrder        CoreEntity = "ORDER"
	CorePayment      CoreEntity = "PAYMENT"
	CoreFulfillment  CoreEntity = "FULFILLMENT"
)

func isCoreEntity(e CoreEntity) bool {
	switch e {
	case CoreOrganization, CoreSeller, CoreStore, CoreListing, CoreCategory,
		CoreAttribute, CoreMedia, CoreLocation, CorePrice, CoreInventory,
		CoreCapacity, CoreReservation, CoreOrder, CorePayment, CoreFulfillment:
		return true
	default:
		return false
	}
}

// VerticalConcept is a term of a vertical's language ("room type", "table",
// "ticket type") and the Commerce Core entity that represents it. It is how a
// vertical extends the core without duplicating it.
type VerticalConcept struct {
	Name string
	Core CoreEntity
	// ResourceType is set when the concept is an inventory resource or a
	// capacity unit of that resource kind (a room, a seat, a vehicle).
	ResourceType ResourceType
}

// VerticalDefinition is the machine-readable form of a vertical: the generic
// capabilities it runs on, its vocabulary mapped onto the core, the listing
// kinds it accepts and the transaction packs (order, booking, appointment
// workflows) it composes.
type VerticalDefinition struct {
	Code        Vertical
	Name        string
	Description string
	// Capabilities are generic Commerce Core capabilities, the same identifiers
	// providers declare, so a vertical capability maps onto a provider
	// capability without translation.
	Capabilities []Capability
	Concepts     []VerticalConcept
	// ListingTypes are the listing kinds a listing of this vertical may have.
	ListingTypes     []ListingType
	FulfillmentTypes []FulfillmentType
	Packs            []PackCode
	// Flows are the customer flows offered for this vertical's listings, most
	// prominent first: BOOK (reserve a period) and/or BUY (order).
	Flows []CustomerFlow
	// Keywords are words customers use for this vertical ("hotel", "hôtel").
	// They guide the AI interpreter and the keyword fallback; they never
	// override an explicit vertical.
	Keywords []string
}

// CustomerFlow is how a customer transacts with a listing.
type CustomerFlow string

const (
	// FlowBook reserves a period through the core Reservation (stays, tables,
	// appointments, visits, rentals).
	FlowBook CustomerFlow = "BOOK"
	// FlowBuy places a core Order.
	FlowBuy CustomerFlow = "BUY"
)

// Accepts reports whether a listing of type t may belong to this vertical.
func (d VerticalDefinition) Accepts(t ListingType) bool {
	for _, lt := range d.ListingTypes {
		if lt == t {
			return true
		}
	}
	return false
}

// HasCapability reports whether the vertical runs on capability c.
func (d VerticalDefinition) HasCapability(c Capability) bool {
	for _, have := range d.Capabilities {
		if have == c {
			return true
		}
	}
	return false
}

// Validate checks the definition's own invariants. Cross-catalogue rules
// (packs exist, capabilities are provided) are checked by the catalogue.
func (d VerticalDefinition) Validate() error {
	if !d.Code.IsWellFormed() {
		return fmt.Errorf("vertical code %q is not well formed", d.Code)
	}
	if strings.TrimSpace(d.Name) == "" {
		return fmt.Errorf("vertical %s: name is required", d.Code)
	}
	if len(d.Capabilities) == 0 {
		return fmt.Errorf("vertical %s: at least one capability is required", d.Code)
	}
	if len(d.ListingTypes) == 0 {
		return fmt.Errorf("vertical %s: at least one listing type is required", d.Code)
	}
	known := map[ListingType]bool{}
	for _, t := range ListingTypes() {
		known[t] = true
	}
	for _, t := range d.ListingTypes {
		if !known[t] {
			return fmt.Errorf("vertical %s: unknown listing type %q", d.Code, t)
		}
	}
	for _, c := range d.Concepts {
		if strings.TrimSpace(c.Name) == "" || !isCoreEntity(c.Core) {
			return fmt.Errorf("vertical %s: concept %q must map onto a Commerce Core entity", d.Code, c.Name)
		}
		if c.ResourceType != "" && !IsValidResourceType(c.ResourceType) {
			return fmt.Errorf("vertical %s: concept %q has unknown resource type %q", d.Code, c.Name, c.ResourceType)
		}
	}
	for _, f := range d.FulfillmentTypes {
		if !IsValidFulfillmentType(f) {
			return fmt.Errorf("vertical %s: unknown fulfillment type %q", d.Code, f)
		}
	}
	if len(d.Packs) == 0 {
		return fmt.Errorf("vertical %s: at least one transaction pack is required", d.Code)
	}
	if len(d.Flows) == 0 {
		return fmt.Errorf("vertical %s: at least one customer flow is required", d.Code)
	}
	for _, f := range d.Flows {
		switch {
		case f == FlowBook && !d.HasCapability(CapabilityReservation):
			return fmt.Errorf("vertical %s: BOOK needs the RESERVATION capability", d.Code)
		case f == FlowBuy && !d.HasCapability(CapabilityOrdering):
			return fmt.Errorf("vertical %s: BUY needs the ORDERING capability", d.Code)
		case f != FlowBook && f != FlowBuy:
			return fmt.Errorf("vertical %s: unknown customer flow %q", d.Code, f)
		}
	}
	return nil
}

// VerticalResolver decides which vertical a new listing belongs to: the
// requested one when it exists and accepts the listing type, otherwise the
// listing type's default. It returns an error wrapping ErrInvalidParameter for
// an unknown vertical or an incompatible listing type.
type VerticalResolver interface {
	ResolveForListing(requested Vertical, listingType ListingType) (Vertical, error)
}
