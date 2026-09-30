package vertical

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/trezutil/trezutil/internal/domain"
)

// PlatformCapabilities are provided by Commerce Core services (payments,
// delivery and tracking providers, notifications) rather than by a
// transaction pack, so any vertical may rely on them.
var PlatformCapabilities = []domain.Capability{
	domain.CapabilityPayment,
	domain.CapabilityDelivery,
	domain.CapabilityTracking,
	domain.CapabilityLocation,
	domain.CapabilityNotification,
}

// VerticalDefinitions is the vertical catalogue. Adding a vertical means adding
// an entry here; the Commerce Core does not change. Every concept maps onto an
// existing core entity, and every workflow comes from an existing transaction
// pack (PackDefinitions).
var VerticalDefinitions = []domain.VerticalDefinition{
	{
		Code:        domain.VerticalRetail,
		Name:        "Retail",
		Description: "Products sold from stock and delivered or collected.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityOrdering,
			domain.CapabilityInventory, domain.CapabilityPayment, domain.CapabilityFulfillment,
			domain.CapabilityDelivery,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "product", Core: domain.CoreListing},
			{Name: "variant / SKU", Core: domain.CoreAttribute},
			{Name: "stock", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeCountable},
			{Name: "price", Core: domain.CorePrice},
			{Name: "purchase", Core: domain.CoreOrder},
			{Name: "delivery or pickup", Core: domain.CoreFulfillment},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypePhysicalProduct, domain.ListingTypeVehicle, domain.ListingTypeFood, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeDelivery, domain.FulfillmentTypePickup, domain.FulfillmentTypeDigital},
		Packs:            []domain.PackCode{domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBuy},
	},
	{
		Code:        domain.VerticalServices,
		Name:        "Services",
		Description: "Work done by a person or team, booked as an appointment or bought outright.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityScheduling, domain.CapabilityCapacity, domain.CapabilityReservation,
			domain.CapabilityOrdering, domain.CapabilityPayment, domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "service", Core: domain.CoreListing},
			{Name: "duration", Core: domain.CoreAttribute},
			{Name: "staff member / resource", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeTimeSlot},
			{Name: "time slot", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeTimeSlot},
			{Name: "appointment", Core: domain.CoreReservation},
			{Name: "fixed-price job", Core: domain.CoreOrder},
			{Name: "job completion", Core: domain.CoreFulfillment},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeService, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeAppointment, domain.FulfillmentTypeServiceCompletion},
		Packs:            []domain.PackCode{domain.PackServices, domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
		Keywords:         []string{"plumber", "plombier", "electrician", "électricien", "mechanic", "mécanicien", "hairdresser", "coiffeur", "coiffeuse"},
	},
	{
		Code:        domain.VerticalHotel,
		Name:        "Hotels & accommodation",
		Description: "Rooms and units booked by the night, with optional hotel services.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityCapacity, domain.CapabilityOrdering,
			domain.CapabilityPayment, domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "property", Core: domain.CoreStore},
			{Name: "room type", Core: domain.CoreListing},
			{Name: "room / unit", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeRoom},
			{Name: "nightly availability", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeRoom},
			{Name: "rate", Core: domain.CorePrice},
			{Name: "occupancy", Core: domain.CoreReservation},
			{Name: "amenities", Core: domain.CoreAttribute},
			{Name: "stay", Core: domain.CoreReservation},
			{Name: "hotel service", Core: domain.CoreListing},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeRealEstate, domain.ListingTypeService, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeAppointment, domain.FulfillmentTypeServiceCompletion},
		Packs:            []domain.PackCode{domain.PackBooking, domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
		Keywords:         []string{"hotel", "hotels", "hôtel", "hôtels", "motel", "lodge", "guesthouse", "auberge"},
	},
	{
		Code:        domain.VerticalRestaurant,
		Name:        "Restaurants",
		Description: "Menus ordered for delivery or pickup, and tables booked for a time.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityCapacity, domain.CapabilityOrdering,
			domain.CapabilityPayment, domain.CapabilityFulfillment, domain.CapabilityDelivery,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "restaurant", Core: domain.CoreStore},
			{Name: "menu", Core: domain.CoreCategory},
			{Name: "menu item", Core: domain.CoreListing},
			{Name: "table", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeSeat},
			{Name: "seating capacity", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeSeat},
			{Name: "table booking", Core: domain.CoreReservation},
			{Name: "food order", Core: domain.CoreOrder},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeFood, domain.ListingTypeService, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeDelivery, domain.FulfillmentTypePickup, domain.FulfillmentTypeAppointment},
		Packs:            []domain.PackCode{domain.PackBooking, domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBook, domain.FlowBuy},
		Keywords:         []string{"restaurant", "restaurants", "resto"},
	},
	{
		Code:        domain.VerticalRealEstate,
		Name:        "Real estate",
		Description: "Properties to rent or buy, with visits booked in advance.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityPayment, domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "property", Core: domain.CoreListing},
			{Name: "property type", Core: domain.CoreCategory},
			{Name: "location", Core: domain.CoreLocation},
			{Name: "features", Core: domain.CoreAttribute},
			{Name: "visit", Core: domain.CoreReservation},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeRealEstate},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypePropertyVisit},
		Packs:            []domain.PackCode{domain.PackBooking},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
		Keywords:         []string{"apartment", "apartments", "appartement", "appartements", "villa", "villas"},
	},
	{
		Code:        domain.VerticalVehicleRental,
		Name:        "Vehicle rental",
		Description: "Vehicles booked for a rental period.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityCapacity, domain.CapabilityPayment,
			domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "vehicle category", Core: domain.CoreListing},
			{Name: "vehicle", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeVehicle},
			{Name: "rental period", Core: domain.CoreReservation},
			{Name: "daily rate", Core: domain.CorePrice},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeVehicle},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypePickup, domain.FulfillmentTypeDelivery},
		Packs:            []domain.PackCode{domain.PackBooking},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
	},
	{
		Code:        domain.VerticalTransport,
		Name:        "Transport",
		Description: "Seats on scheduled trips and on-demand rides.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityCapacity, domain.CapabilityOrdering,
			domain.CapabilityPayment, domain.CapabilityFulfillment, domain.CapabilityTracking,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "route / ride type", Core: domain.CoreListing},
			{Name: "seat", Core: domain.CoreInventory, ResourceType: domain.ResourceTypeSeat},
			{Name: "departure", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeSeat},
			{Name: "ticket", Core: domain.CoreReservation},
			{Name: "trip", Core: domain.CoreFulfillment},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeService, domain.ListingTypeVehicle, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeTravelTicket, domain.FulfillmentTypeServiceCompletion},
		Packs:            []domain.PackCode{domain.PackBooking, domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
		Keywords:         []string{"taxi", "bus", "shuttle", "navette"},
	},
	{
		Code:        domain.VerticalEvents,
		Name:        "Events",
		Description: "Tickets for dated events with limited capacity.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityCapacity, domain.CapabilityReservation, domain.CapabilityOrdering,
			domain.CapabilityPayment, domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "event", Core: domain.CoreListing},
			{Name: "venue", Core: domain.CoreLocation},
			{Name: "ticket type", Core: domain.CoreListing},
			{Name: "capacity", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeSeat},
			{Name: "schedule", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeSeat},
			{Name: "seat reservation / RSVP", Core: domain.CoreReservation},
			{Name: "ticket", Core: domain.CoreOrder},
			{Name: "entry", Core: domain.CoreFulfillment},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeService, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeEventEntry, domain.FulfillmentTypeDigital},
		Packs:            []domain.PackCode{domain.PackECommerce, domain.PackBooking},
		Flows:            []domain.CustomerFlow{domain.FlowBuy},
		Keywords:         []string{"concert", "concerts", "festival", "festivals", "event", "events", "événement", "événements"},
	},
	{
		Code:        domain.VerticalTravel,
		Name:        "Travel",
		Description: "Trips and packages; the vertical most likely to add external supply later.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityAvailability,
			domain.CapabilityReservation, domain.CapabilityPayment, domain.CapabilityFulfillment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "trip / package", Core: domain.CoreListing},
			{Name: "departure", Core: domain.CoreCapacity, ResourceType: domain.ResourceTypeSeat},
			{Name: "booking", Core: domain.CoreReservation},
			{Name: "travel document", Core: domain.CoreFulfillment},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeService, domain.ListingTypeRealEstate, domain.ListingTypeOther},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeTravelTicket},
		Packs:            []domain.PackCode{domain.PackBooking},
		Flows:            []domain.CustomerFlow{domain.FlowBook},
	},
	{
		Code:        domain.VerticalOther,
		Name:        "Other",
		Description: "Anything that does not fit another vertical yet.",
		Capabilities: []domain.Capability{
			domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityOrdering,
			domain.CapabilityPayment,
		},
		Concepts: []domain.VerticalConcept{
			{Name: "offer", Core: domain.CoreListing},
			{Name: "purchase", Core: domain.CoreOrder},
		},
		ListingTypes:     []domain.ListingType{domain.ListingTypeOther, domain.ListingTypePhysicalProduct, domain.ListingTypeService, domain.ListingTypeFood, domain.ListingTypeRealEstate, domain.ListingTypeVehicle},
		FulfillmentTypes: []domain.FulfillmentType{domain.FulfillmentTypeNone},
		Packs:            []domain.PackCode{domain.PackECommerce},
		Flows:            []domain.CustomerFlow{domain.FlowBuy},
	},
}

// Catalog is the read-only vertical catalogue. It implements
// domain.VerticalResolver.
type Catalog struct {
	order  []domain.Vertical
	byCode map[domain.Vertical]domain.VerticalDefinition
}

// NewCatalog validates the definitions against each other and against the
// transaction packs, and returns the catalogue.
func NewCatalog(defs []domain.VerticalDefinition, packs map[domain.PackCode]PackDefinition) (*Catalog, error) {
	platform := map[domain.Capability]bool{}
	for _, c := range PlatformCapabilities {
		platform[c] = true
	}
	c := &Catalog{byCode: make(map[domain.Vertical]domain.VerticalDefinition, len(defs))}
	for _, def := range defs {
		if err := def.Validate(); err != nil {
			return nil, err
		}
		if _, dup := c.byCode[def.Code]; dup {
			return nil, fmt.Errorf("vertical %s is defined twice", def.Code)
		}
		provided := map[domain.Capability]bool{}
		for _, code := range def.Packs {
			build, ok := packs[code]
			if !ok {
				return nil, fmt.Errorf("vertical %s: transaction pack %s is not defined", def.Code, code)
			}
			for _, capability := range build().Capabilities {
				provided[capability] = true
			}
		}
		for _, capability := range def.Capabilities {
			if !provided[capability] && !platform[capability] {
				return nil, fmt.Errorf("vertical %s: capability %s is provided by neither its packs nor the platform", def.Code, capability)
			}
		}
		c.byCode[def.Code] = def
		c.order = append(c.order, def.Code)
	}
	owner := map[string]domain.Vertical{}
	for _, code := range c.order {
		for _, kw := range c.byCode[code].Keywords {
			if kw != strings.ToLower(strings.TrimSpace(kw)) || kw == "" {
				return nil, fmt.Errorf("vertical %s: keyword %q must be lower-case", code, kw)
			}
			if other, dup := owner[kw]; dup {
				return nil, fmt.Errorf("keyword %q belongs to both %s and %s", kw, other, code)
			}
			owner[kw] = code
		}
	}
	for _, t := range domain.ListingTypes() {
		def, ok := c.byCode[domain.DefaultVerticalForListingType(t)]
		if !ok || !def.Accepts(t) {
			return nil, fmt.Errorf("listing type %s has no default vertical that accepts it", t)
		}
	}
	return c, nil
}

var defaultCatalog = func() *Catalog {
	c, err := NewCatalog(VerticalDefinitions, PackDefinitions)
	if err != nil {
		panic(fmt.Sprintf("vertical catalogue is invalid: %v", err))
	}
	return c
}()

// DefaultCatalog returns the catalogue built from VerticalDefinitions.
func DefaultCatalog() *Catalog { return defaultCatalog }

// Get returns the definition of vertical v.
func (c *Catalog) Get(v domain.Vertical) (domain.VerticalDefinition, bool) {
	def, ok := c.byCode[v]
	return def, ok
}

// All returns every vertical in catalogue order.
func (c *Catalog) All() []domain.VerticalDefinition {
	out := make([]domain.VerticalDefinition, 0, len(c.order))
	for _, code := range c.order {
		out = append(out, c.byCode[code])
	}
	return out
}

// DetectVertical finds the vertical named by keywords in free text ("un hôtel à
// Gombe" → HOTEL) and returns the matched words so callers can drop them from
// the search text. Text naming several verticals yields none: an ambiguous
// guess would hide results.
func (c *Catalog) DetectVertical(text string) (domain.Vertical, []string) {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
	var found domain.Vertical
	var matched []string
	for _, code := range c.order {
		for _, kw := range c.byCode[code].Keywords {
			for _, w := range words {
				if w != kw {
					continue
				}
				if found != "" && found != code {
					return "", nil
				}
				found = code
				matched = append(matched, w)
			}
		}
	}
	return found, matched
}

// ResolveForListing implements domain.VerticalResolver.
func (c *Catalog) ResolveForListing(requested domain.Vertical, listingType domain.ListingType) (domain.Vertical, error) {
	if requested == "" {
		return domain.DefaultVerticalForListingType(listingType), nil
	}
	code := domain.NormalizeVertical(string(requested))
	def, ok := c.byCode[code]
	if !ok {
		return "", fmt.Errorf("%w: unknown vertical %q", domain.ErrInvalidParameter, requested)
	}
	if !def.Accepts(listingType) {
		return "", fmt.Errorf("%w: vertical %s does not accept listing type %q", domain.ErrInvalidParameter, code, listingType)
	}
	return code, nil
}
