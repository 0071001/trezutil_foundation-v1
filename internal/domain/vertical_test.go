package domain

import (
	"strings"
	"testing"
)

func TestNormalizeVertical(t *testing.T) {
	for raw, want := range map[string]Vertical{
		"hotel":          VerticalHotel,
		" Real estate ":  VerticalRealEstate,
		"vehicle-rental": VerticalVehicleRental,
		"EVENTS":         VerticalEvents,
	} {
		if got := NormalizeVertical(raw); got != want {
			t.Errorf("NormalizeVertical(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestVertical_IsWellFormed(t *testing.T) {
	for v, want := range map[Vertical]bool{
		VerticalHotel:                     true,
		VerticalVehicleRental:             true,
		"B2B_WHOLESALE":                   true,
		"hotel":                           false,
		"":                                false,
		"H":                               false,
		"1HOTEL":                          false,
		"HOTEL ROOMS":                     false,
		Vertical(strings.Repeat("A", 33)): false,
	} {
		if got := v.IsWellFormed(); got != want {
			t.Errorf("%q.IsWellFormed() = %v, want %v", v, got, want)
		}
	}
}

func TestDefaultVerticalForListingType(t *testing.T) {
	want := map[ListingType]Vertical{
		ListingTypePhysicalProduct: VerticalRetail,
		ListingTypeVehicle:         VerticalRetail,
		ListingTypeFood:            VerticalRestaurant,
		ListingTypeService:         VerticalServices,
		ListingTypeRealEstate:      VerticalRealEstate,
		ListingTypeOther:           VerticalOther,
		"unknown":                  VerticalOther,
	}
	for lt, v := range want {
		if got := DefaultVerticalForListingType(lt); got != v {
			t.Errorf("DefaultVerticalForListingType(%q) = %q, want %q", lt, got, v)
		}
	}
	for _, lt := range ListingTypes() {
		if _, ok := want[lt]; !ok {
			t.Errorf("listing type %q has no expected default", lt)
		}
	}
	if l := NewListing("o", "s", "st", "c", "Room", string(ListingTypeFood), "fixed", "USD", 1); l.Vertical != VerticalRestaurant {
		t.Errorf("NewListing vertical = %q", l.Vertical)
	}
}

func validDefinition() VerticalDefinition {
	return VerticalDefinition{
		Code:             "MARKETS",
		Name:             "Markets",
		Capabilities:     []Capability{CapabilityCatalog, CapabilityOrdering},
		Concepts:         []VerticalConcept{{Name: "stall", Core: CoreStore}, {Name: "crate", Core: CoreInventory, ResourceType: ResourceTypeCountable}},
		ListingTypes:     []ListingType{ListingTypePhysicalProduct, ListingTypeFood},
		FulfillmentTypes: []FulfillmentType{FulfillmentTypePickup},
		Packs:            []PackCode{PackECommerce},
		Flows:            []CustomerFlow{FlowBuy},
	}
}

func TestVerticalDefinition_Validate(t *testing.T) {
	if err := validDefinition().Validate(); err != nil {
		t.Fatalf("valid definition: %v", err)
	}
	for name, mutate := range map[string]func(*VerticalDefinition){
		"malformed code":       func(d *VerticalDefinition) { d.Code = "markets" },
		"no name":              func(d *VerticalDefinition) { d.Name = " " },
		"no capabilities":      func(d *VerticalDefinition) { d.Capabilities = nil },
		"no listing types":     func(d *VerticalDefinition) { d.ListingTypes = nil },
		"unknown listing type": func(d *VerticalDefinition) { d.ListingTypes = []ListingType{"spaceship"} },
		"concept outside core": func(d *VerticalDefinition) {
			d.Concepts = []VerticalConcept{{Name: "booking engine", Core: "HOTEL_BOOKINGS"}}
		},
		"unnamed concept": func(d *VerticalDefinition) { d.Concepts = []VerticalConcept{{Core: CoreListing}} },
		"unknown resource type": func(d *VerticalDefinition) {
			d.Concepts = []VerticalConcept{{Name: "bed", Core: CoreInventory, ResourceType: "BED"}}
		},
		"unknown fulfillment":  func(d *VerticalDefinition) { d.FulfillmentTypes = []FulfillmentType{"TELEPORT"} },
		"no transaction pack":  func(d *VerticalDefinition) { d.Packs = nil },
		"no customer flow":     func(d *VerticalDefinition) { d.Flows = nil },
		"BOOK without booking": func(d *VerticalDefinition) { d.Flows = []CustomerFlow{FlowBook} },
		"BUY without ordering": func(d *VerticalDefinition) { d.Capabilities = []Capability{CapabilityCatalog} },
		"unknown flow":         func(d *VerticalDefinition) { d.Flows = []CustomerFlow{"RENT"} },
	} {
		d := validDefinition()
		mutate(&d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestVerticalDefinition_AcceptsAndCapabilities(t *testing.T) {
	d := validDefinition()
	if !d.Accepts(ListingTypeFood) || d.Accepts(ListingTypeRealEstate) {
		t.Error("Accepts does not follow ListingTypes")
	}
	if !d.HasCapability(CapabilityOrdering) || d.HasCapability(CapabilityReservation) {
		t.Error("HasCapability does not follow Capabilities")
	}
}
