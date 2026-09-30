package vertical

import (
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

var _ domain.VerticalResolver = (*Catalog)(nil)

func TestDefaultCatalog_InitialVerticals(t *testing.T) {
	want := []domain.Vertical{
		domain.VerticalRetail, domain.VerticalServices, domain.VerticalHotel, domain.VerticalRestaurant,
		domain.VerticalRealEstate, domain.VerticalVehicleRental, domain.VerticalTransport,
		domain.VerticalEvents, domain.VerticalTravel, domain.VerticalOther,
	}
	all := DefaultCatalog().All()
	if len(all) != len(want) {
		t.Fatalf("catalogue has %d verticals, want %d", len(all), len(want))
	}
	for i, def := range all {
		if def.Code != want[i] {
			t.Errorf("vertical %d = %s, want %s", i, def.Code, want[i])
		}
	}
}

// Verticals extend the Commerce Core: reservation and order concepts always
// map onto the core Reservation and Order, and every vertical sells through
// the core payment capability.
func TestDefaultCatalog_VerticalsReuseTheCore(t *testing.T) {
	for _, def := range DefaultCatalog().All() {
		if !def.HasCapability(domain.CapabilityPayment) {
			t.Errorf("%s does not use the core payment capability", def.Code)
		}
		if def.HasCapability(domain.CapabilityReservation) && !hasCore(def, domain.CoreReservation) {
			t.Errorf("%s books but maps no concept onto the core Reservation", def.Code)
		}
		if def.HasCapability(domain.CapabilityOrdering) && !def.HasCapability(domain.CapabilityReservation) && !hasCore(def, domain.CoreOrder) {
			t.Errorf("%s sells but maps no concept onto the core Order", def.Code)
		}
	}
	hotel, _ := DefaultCatalog().Get(domain.VerticalHotel)
	if !hotel.HasCapability(domain.CapabilityReservation) || !hotel.HasCapability(domain.CapabilityOrdering) {
		t.Error("a hotel books rooms and sells hotel services")
	}
}

func hasCore(def domain.VerticalDefinition, core domain.CoreEntity) bool {
	for _, c := range def.Concepts {
		if c.Core == core {
			return true
		}
	}
	return false
}

func TestNewCatalog_RejectsInconsistentDefinitions(t *testing.T) {
	base := func() []domain.VerticalDefinition {
		out := make([]domain.VerticalDefinition, len(VerticalDefinitions))
		copy(out, VerticalDefinitions)
		return out
	}
	cases := map[string]func([]domain.VerticalDefinition) []domain.VerticalDefinition{
		"duplicate vertical": func(d []domain.VerticalDefinition) []domain.VerticalDefinition { return append(d, d[0]) },
		"unknown pack": func(d []domain.VerticalDefinition) []domain.VerticalDefinition {
			d[2].Packs = []domain.PackCode{"HOTEL_ENGINE"}
			return d
		},
		"capability no pack provides": func(d []domain.VerticalDefinition) []domain.VerticalDefinition {
			d[4].Capabilities = append(append([]domain.Capability{}, d[4].Capabilities...), domain.CapabilityMerchantOps)
			return d
		},
		"listing type without a default vertical": func(d []domain.VerticalDefinition) []domain.VerticalDefinition {
			return d[1:] // drops RETAIL, the default for physical products
		},
		"invalid definition": func(d []domain.VerticalDefinition) []domain.VerticalDefinition {
			d[0].Name = ""
			return d
		},
	}
	for name, mutate := range cases {
		if _, err := NewCatalog(mutate(base()), PackDefinitions); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := NewCatalog(VerticalDefinitions, PackDefinitions); err != nil {
		t.Fatalf("default definitions: %v", err)
	}
}

func TestCatalog_ResolveForListing(t *testing.T) {
	c := DefaultCatalog()
	ok := []struct {
		requested domain.Vertical
		lt        domain.ListingType
		want      domain.Vertical
	}{
		{"", domain.ListingTypePhysicalProduct, domain.VerticalRetail},
		{"", domain.ListingTypeFood, domain.VerticalRestaurant},
		{"hotel", domain.ListingTypeRealEstate, domain.VerticalHotel},
		{"Vehicle rental", domain.ListingTypeVehicle, domain.VerticalVehicleRental},
		{domain.VerticalRestaurant, domain.ListingTypeFood, domain.VerticalRestaurant},
		{domain.VerticalOther, domain.ListingTypeVehicle, domain.VerticalOther},
	}
	for _, tc := range ok {
		got, err := c.ResolveForListing(tc.requested, tc.lt)
		if err != nil || got != tc.want {
			t.Errorf("ResolveForListing(%q, %q) = %q, %v; want %q", tc.requested, tc.lt, got, err, tc.want)
		}
	}
	bad := []struct {
		requested domain.Vertical
		lt        domain.ListingType
	}{
		{"SPACESHIPS", domain.ListingTypeOther},
		{domain.VerticalRetail, domain.ListingTypeRealEstate},
		{domain.VerticalRealEstate, domain.ListingTypeFood},
		{domain.VerticalVehicleRental, domain.ListingTypePhysicalProduct},
	}
	for _, tc := range bad {
		if _, err := c.ResolveForListing(tc.requested, tc.lt); !errors.Is(err, domain.ErrInvalidParameter) {
			t.Errorf("ResolveForListing(%q, %q) error = %v, want ErrInvalidParameter", tc.requested, tc.lt, err)
		}
	}
}
