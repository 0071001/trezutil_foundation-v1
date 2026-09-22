package vertical

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

func newTestLogger() zerolog.Logger {
	return zerolog.Nop()
}

func newTestRegistry() *PackRegistry {
	return NewPackRegistry(PackRegistryConfig{
		Logger: newTestLogger(),
	})
}

func TestPackRegistry_New(t *testing.T) {
	r := newTestRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if r.Count() != 0 {
		t.Error("expected empty registry")
	}
}

func TestPackRegistry_RegisterAndGet(t *testing.T) {
	r := newTestRegistry()
	pack := domain.NewVerticalPack(domain.PackECommerce, "E-Commerce Pack", []domain.PackCapability{domain.CapabilityCatalog}, domain.AISpecialistCommerce)
	pack.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}

	if err := r.Register(pack); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := r.Get(domain.PackECommerce)
	if !ok {
		t.Fatal("expected pack to be found")
	}
	if got.Code != domain.PackECommerce {
		t.Errorf("expected %s, got %s", domain.PackECommerce, got.Code)
	}
}

func TestPackRegistry_RegisterDuplicate(t *testing.T) {
	r := newTestRegistry()
	pack := domain.NewVerticalPack(domain.PackECommerce, "E-Commerce Pack", []domain.PackCapability{domain.CapabilityCatalog}, domain.AISpecialistCommerce)
	pack.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}

	r.Register(pack)

	pack2 := domain.NewVerticalPack(domain.PackECommerce, "Another", []domain.PackCapability{domain.CapabilityCatalog}, domain.AISpecialistCommerce)
	pack2.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}
	err := r.Register(pack2)
	if err == nil {
		t.Error("expected error on duplicate registration")
	}
}

func TestPackRegistry_RegisterNil(t *testing.T) {
	r := newTestRegistry()
	err := r.Register(nil)
	if err == nil {
		t.Error("expected error on nil registration")
	}
}

func TestPackRegistry_RegisterInvalidPack(t *testing.T) {
	r := newTestRegistry()
	pack := domain.NewVerticalPack(domain.PackECommerce, "", nil, "")
	pack.SupportedActions = nil

	err := r.Register(pack)
	if err == nil {
		t.Error("expected error on invalid pack registration")
	}
}

func TestPackRegistry_GetAll(t *testing.T) {
	r := newTestRegistry()
	pack1 := domain.NewVerticalPack(domain.PackECommerce, "E-Commerce", []domain.PackCapability{domain.CapabilityCatalog}, domain.AISpecialistCommerce)
	pack1.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}
	pack2 := domain.NewVerticalPack(domain.PackBooking, "Booking", []domain.PackCapability{domain.CapabilityReservation}, domain.AISpecialistBooking)
	pack2.SupportedActions = []domain.CommerceActionType{domain.ActionCreateReservation}

	r.Register(pack1)
	r.Register(pack2)

	all := r.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 packs, got %d", len(all))
	}
}

func TestPackRegistry_FindByAction(t *testing.T) {
	r := newTestRegistry()
	pack1 := domain.NewVerticalPack(domain.PackECommerce, "E-Commerce", []domain.PackCapability{domain.CapabilityCatalog}, domain.AISpecialistCommerce)
	pack1.SupportedActions = []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionCreateOrder,
	}
	pack2 := domain.NewVerticalPack(domain.PackBooking, "Booking", []domain.PackCapability{domain.CapabilityReservation}, domain.AISpecialistBooking)
	pack2.SupportedActions = []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionCreateReservation,
	}

	r.Register(pack1)
	r.Register(pack2)

	// ActionSearchListings is supported by both packs
	packs := r.FindByAction(domain.ActionSearchListings)
	if len(packs) != 2 {
		t.Errorf("expected 2 packs for ActionSearchListings, got %d", len(packs))
	}

	// ActionCreateOrder is only supported by ECommerce
	packs = r.FindByAction(domain.ActionCreateOrder)
	if len(packs) != 1 {
		t.Errorf("expected 1 pack for ActionCreateOrder, got %d", len(packs))
	}
	if len(packs) > 0 && packs[0] != domain.PackECommerce {
		t.Errorf("expected ECOMMERCE, got %s", packs[0])
	}

	// ActionUpdateListing is not supported by any pack
	packs = r.FindByAction(domain.ActionUpdateListing)
	if len(packs) != 0 {
		t.Errorf("expected 0 packs for ActionUpdateListing, got %d", len(packs))
	}
}

func TestPackRegistry_FindByCapability(t *testing.T) {
	r := newTestRegistry()
	pack1 := domain.NewVerticalPack(domain.PackECommerce, "E-Commerce",
		[]domain.PackCapability{domain.CapabilityCatalog, domain.CapabilitySearch, domain.CapabilityOrdering},
		domain.AISpecialistCommerce)
	pack1.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}
	pack2 := domain.NewVerticalPack(domain.PackBooking, "Booking",
		[]domain.PackCapability{domain.CapabilityCatalog, domain.CapabilityReservation},
		domain.AISpecialistBooking)
	pack2.SupportedActions = []domain.CommerceActionType{domain.ActionCreateReservation}

	r.Register(pack1)
	r.Register(pack2)

	// CapabilityCatalog is in both
	packs := r.FindByCapability(domain.CapabilityCatalog)
	if len(packs) != 2 {
		t.Errorf("expected 2 packs with Catalog, got %d", len(packs))
	}

	// CapabilityOrdering is only in ECommerce
	packs = r.FindByCapability(domain.CapabilityOrdering)
	if len(packs) != 1 {
		t.Errorf("expected 1 pack with Ordering, got %d", len(packs))
	}
}

func TestLoadAllPacks(t *testing.T) {
	r := newTestRegistry()
	err := LoadAllPacks(r)
	if err != nil {
		t.Fatalf("LoadAllPacks failed: %v", err)
	}

	if r.Count() != 3 {
		t.Errorf("expected 3 packs loaded, got %d", r.Count())
	}

	// Verify each pack exists
	for _, code := range []domain.PackCode{domain.PackECommerce, domain.PackBooking, domain.PackServices} {
		pack, ok := r.Get(code)
		if !ok {
			t.Errorf("expected pack %s to be loaded", code)
		}
		if !pack.IsActive() {
			t.Errorf("expected pack %s to be active", code)
		}
		if err := pack.Validate(); err != nil {
			t.Errorf("pack %s failed validation: %v", code, err)
		}
	}
}

func TestPackDefinitions_Valid(t *testing.T) {
	for code, def := range PackDefinitions {
		t.Run(string(code), func(t *testing.T) {
			pack := def()
			if err := pack.Validate(); err != nil {
				t.Errorf("pack definition %s failed validation: %v", code, err)
			}
			if pack.Workflow == nil {
				t.Errorf("pack %s should have a workflow", code)
			}
			if len(pack.SupportedActions) == 0 {
				t.Errorf("pack %s should have supported actions", code)
			}
			if len(pack.Policies) == 0 {
				t.Errorf("pack %s should have policies", code)
			}
		})
	}
}
