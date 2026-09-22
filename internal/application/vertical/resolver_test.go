package vertical

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeOrgPackRepo struct{}

func (fakeOrgPackRepo) Create(_ interface{}, _ *domain.OrganizationPack) error { return nil }
func (fakeOrgPackRepo) FindByOrgAndPack(_ interface{}, _ string, _ domain.PackCode) (*domain.OrganizationPack, error) {
	return nil, nil
}
func (fakeOrgPackRepo) ListByOrg(_ interface{}, _ string) ([]domain.OrganizationPack, error) {
	return nil, nil
}
func (fakeOrgPackRepo) Enable(_ interface{}, _ string, _ domain.PackCode) error  { return nil }
func (fakeOrgPackRepo) Disable(_ interface{}, _ string, _ domain.PackCode) error { return nil }
func (fakeOrgPackRepo) IsPackEnabled(_ interface{}, _ string, _ domain.PackCode) (bool, error) {
	return false, nil
}

func newFakeOrgPackRepo() *fakeOrgPackRepo {
	return &fakeOrgPackRepo{}
}

func newTestResolver() *PackResolver {
	registry := newTestRegistry()
	LoadAllPacks(registry)

	return NewPackResolver(PackResolverConfig{
		Logger:   newTestLogger(),
		Registry: registry,
		Policies: newTestPolicyEngine(),
		PackRepo: newFakeOrgPackRepo(),
	})
}

func TestPackResolver_EnablePackForOrg(t *testing.T) {
	resolver := newTestResolver()

	if err := resolver.EnablePackForOrg("org-1", domain.PackECommerce); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resolver.IsPackEnabled("org-1", domain.PackECommerce) {
		t.Error("expected pack to be enabled")
	}
}

func TestPackResolver_EnableUnknownPack(t *testing.T) {
	resolver := newTestResolver()

	err := resolver.EnablePackForOrg("org-1", "UNKNOWN_PACK")
	if err == nil {
		t.Error("expected error for unknown pack")
	}
}

func TestPackResolver_DisablePackForOrg(t *testing.T) {
	resolver := newTestResolver()

	resolver.EnablePackForOrg("org-1", domain.PackECommerce)
	resolver.DisablePackForOrg("org-1", domain.PackECommerce)

	if resolver.IsPackEnabled("org-1", domain.PackECommerce) {
		t.Error("expected pack to be disabled")
	}
}

func TestPackResolver_IsPackEnabled_UnknownOrg(t *testing.T) {
	resolver := newTestResolver()

	if resolver.IsPackEnabled("unknown-org", domain.PackECommerce) {
		t.Error("expected pack to not be enabled for unknown org")
	}
}

func TestPackResolver_GetAvailablePacks(t *testing.T) {
	resolver := newTestResolver()

	resolver.EnablePackForOrg("org-1", domain.PackECommerce)
	resolver.EnablePackForOrg("org-1", domain.PackBooking)
	resolver.DisablePackForOrg("org-1", domain.PackServices)

	packs := resolver.GetAvailablePacks("org-1")

	if len(packs) != 2 {
		t.Errorf("expected 2 available packs, got %d", len(packs))
	}
}

func TestPackResolver_GetAvailablePacks_UnknownOrg(t *testing.T) {
	resolver := newTestResolver()

	packs := resolver.GetAvailablePacks("unknown-org")

	if len(packs) != 0 {
		t.Errorf("expected 0 available packs, got %d", len(packs))
	}
}

func TestPackResolver_ResolvePackForAction(t *testing.T) {
	resolver := newTestResolver()

	resolver.EnablePackForOrg("org-1", domain.PackECommerce)
	resolver.EnablePackForOrg("org-1", domain.PackBooking)
	resolver.EnablePackForOrg("org-1", domain.PackServices)

	// ActionCreateOrder is only supported by ECommerce
	code, ok := resolver.ResolvePackForAction("org-1", domain.ActionCreateOrder)
	if !ok {
		t.Fatal("expected to find pack for ActionCreateOrder")
	}
	if code != domain.PackECommerce {
		t.Errorf("expected ECOMMERCE, got %s", code)
	}

	// ActionCreateReservation is supported by both Booking and Services
	code, ok = resolver.ResolvePackForAction("org-1", domain.ActionCreateReservation)
	if !ok {
		t.Fatal("expected to find pack for ActionCreateReservation")
	}
	if code != domain.PackBooking && code != domain.PackServices {
		t.Errorf("expected Booking or Services, got %s", code)
	}
}

func TestPackResolver_ResolvePackForAction_NotEnabled(t *testing.T) {
	resolver := newTestResolver()

	_, ok := resolver.ResolvePackForAction("org-1", domain.ActionCreateOrder)
	if ok {
		t.Error("expected no pack to be returned when no packs enabled")
	}
}

func TestPackResolver_ResolveSpecialistForAction(t *testing.T) {
	resolver := newTestResolver()

	spec, ok := resolver.ResolveSpecialistForAction(domain.ActionCreateOrder)
	if !ok {
		t.Fatal("expected to find specialist for ActionCreateOrder")
	}
	if spec != domain.AISpecialistCommerce {
		t.Errorf("expected COMMERCE_AI for ActionCreateOrder, got %s", spec)
	}

	// ActionCreateReservation is supported by both Booking (BOOKING_AI) and Services (COMMERCE_AI)
	spec, ok = resolver.ResolveSpecialistForAction(domain.ActionCreateReservation)
	if !ok {
		t.Fatal("expected to find specialist for ActionCreateReservation")
	}
	if spec != domain.AISpecialistBooking && spec != domain.AISpecialistCommerce {
		t.Errorf("expected BOOKING_AI or COMMERCE_AI, got %s", spec)
	}

	// ActionCheckInventory is only supported by ECommerce and Services (both COMMERCE_AI)
	spec, ok = resolver.ResolveSpecialistForAction(domain.ActionCheckInventory)
	if !ok {
		t.Fatal("expected to find specialist for ActionCheckInventory")
	}
}

func TestPackResolver_IsActionSupported(t *testing.T) {
	resolver := newTestResolver()

	resolver.EnablePackForOrg("org-1", domain.PackECommerce)

	if !resolver.IsActionSupported("org-1", domain.ActionCreateOrder) {
		t.Error("expected ActionCreateOrder to be supported")
	}
	if resolver.IsActionSupported("org-1", domain.ActionCreateReservation) {
		t.Error("expected ActionCreateReservation to not be supported")
	}
}

func TestPackResolver_ResolvePackWithPolicy(t *testing.T) {
	resolver := newTestResolver()
	resolver.EnablePackForOrg("org-1", domain.PackECommerce)

	// Action with no policy dependency should pass
	code, result := resolver.ResolvePackWithPolicy("org-1", domain.ActionSearchListings, nil)
	if code == "" || !result.Allowed {
		t.Errorf("expected allowed, got code=%s, allowed=%v, reason=%s", code, result.Allowed, result.Reason)
	}

	// Action with policy dependency - payment pending
	ctx := map[string]interface{}{
		"payment_status": "pending",
	}
	code, result = resolver.ResolvePackWithPolicy("org-1", domain.ActionAssignFulfillment, ctx)
	if result.Allowed {
		t.Error("expected action to be denied when payment pending")
	}
}

func TestPackResolver_CanTransition(t *testing.T) {
	resolver := newTestResolver()
	resolver.EnablePackForOrg("org-1", domain.PackECommerce)

	// Valid transition in ECommerce workflow: listing -> details
	if !resolver.CanTransition("org-1", domain.ActionSearchListings, "listing", "details") {
		t.Error("expected valid transition listing -> details")
	}

	// Also valid: listing -> order
	if !resolver.CanTransition("org-1", domain.ActionSearchListings, "listing", "order") {
		t.Error("expected valid transition listing -> order")
	}

	// Invalid transition
	if resolver.CanTransition("org-1", domain.ActionSearchListings, "listing", "checkout") {
		t.Error("expected invalid transition listing -> checkout")
	}

	// Transition when no pack enabled
	if resolver.CanTransition("org-2", domain.ActionSearchListings, "listing", "checkout") {
		t.Error("expected transition to fail when no pack enabled")
	}
}
