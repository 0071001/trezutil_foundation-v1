package vertical

import (
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func newTestPolicyEngine() *PolicyEngine {
	return NewPolicyEngine(PolicyEngineConfig{
		Logger: newTestLogger(),
	})
}

func newTestEcommercePack() *domain.VerticalPack {
	pack := domain.NewVerticalPack(
		domain.PackECommerce,
		"E-Commerce Pack",
		[]domain.PackCapability{
			domain.CapabilityCatalog,
			domain.CapabilitySearch,
			domain.CapabilityOrdering,
			domain.CapabilityInventory,
		},
		domain.AISpecialistCommerce,
	)
	pack.SupportedActions = []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionCreateOrder,
		domain.ActionInitiatePayment,
		domain.ActionAssignFulfillment,
	}
	pack.Policies = []domain.PackPolicy{
		{Key: "require_payment_before_fulfillment", Value: "true"},
		{Key: "inventory_decrement_on", Value: "order_created"},
	}
	pack.Workflow = &domain.PackWorkflow{
		Name:      "ecommerce_checkout",
		EntryStep: "listing",
		Steps: []domain.PackWorkflowStep{
			{Name: "listing", Action: domain.ActionSearchListings, NextSteps: []string{"checkout"}},
			{Name: "checkout", Action: domain.ActionCreateOrder, NextSteps: []string{"payment"}},
			{Name: "payment", Action: domain.ActionInitiatePayment, NextSteps: []string{"fulfillment"}},
			{Name: "fulfillment", Action: domain.ActionAssignFulfillment, NextSteps: []string{}},
		},
	}
	return pack
}

func TestPolicyEngine_Evaluate_AllowedAction(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()

	result := engine.Evaluate(pack, domain.ActionSearchListings, nil)

	if !result.Allowed {
		t.Errorf("expected action to be allowed, got reason: %s", result.Reason)
	}
	if result.PackCode != domain.PackECommerce {
		t.Errorf("expected pack code ECOMMERCE, got %s", result.PackCode)
	}
}

func TestPolicyEngine_Evaluate_NilPack(t *testing.T) {
	engine := newTestPolicyEngine()

	result := engine.Evaluate(nil, domain.ActionSearchListings, nil)

	if result.Allowed {
		t.Error("expected action to be denied for nil pack")
	}
}

func TestPolicyEngine_Evaluate_InactivePack(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()
	pack.Status = domain.PackStatusInactive

	result := engine.Evaluate(pack, domain.ActionSearchListings, nil)

	if result.Allowed {
		t.Error("expected action to be denied for inactive pack")
	}
}

func TestPolicyEngine_Evaluate_UnsupportedAction(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()

	result := engine.Evaluate(pack, domain.ActionCancelReservation, nil)

	if result.Allowed {
		t.Error("expected action to be denied when not supported by pack")
	}
}

func TestPolicyEngine_Evaluate_PaymentRequired_PaymentComplete(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()

	ctx := map[string]interface{}{
		"payment_status": "paid",
	}

	result := engine.Evaluate(pack, domain.ActionAssignFulfillment, ctx)

	if !result.Allowed {
		t.Errorf("expected action to be allowed when payment is complete, got reason: %s", result.Reason)
	}
}

func TestPolicyEngine_Evaluate_PaymentRequired_PaymentPending(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()

	ctx := map[string]interface{}{
		"payment_status": "pending",
	}

	result := engine.Evaluate(pack, domain.ActionAssignFulfillment, ctx)

	if result.Allowed {
		t.Error("expected action to be denied when payment is pending")
	}
}

func TestPolicyEngine_Evaluate_PaymentRequired_NoContext(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := newTestEcommercePack()

	result := engine.Evaluate(pack, domain.ActionAssignFulfillment, nil)

	if !result.Allowed {
		t.Errorf("expected action to be allowed when no payment context (not yet applicable), got reason: %s", result.Reason)
	}
}

func TestPolicyEngine_Evaluate_BookingPackPolicies(t *testing.T) {
	engine := newTestPolicyEngine()
	pack := domain.NewVerticalPack(
		domain.PackBooking,
		"Booking Pack",
		[]domain.PackCapability{domain.CapabilityReservation},
		domain.AISpecialistBooking,
	)
	pack.SupportedActions = []domain.CommerceActionType{
		domain.ActionCreateReservation,
		domain.ActionCancelReservation,
	}
	pack.Policies = []domain.PackPolicy{
		{Key: "require_payment_before_reservation", Value: "true"},
		{Key: "allow_overbooking", Value: "false"},
		{Key: "min_advance_booking_hours", Value: "1"},
	}

	ctx := map[string]interface{}{
		"payment_status": "paid",
	}

	result := engine.Evaluate(pack, domain.ActionCreateReservation, ctx)
	if !result.Allowed {
		t.Errorf("expected allowed, got: %s", result.Reason)
	}
}

func TestPolicyEngine_EvaluateAll(t *testing.T) {
	engine := newTestPolicyEngine()
	registry := newTestRegistry()
	pack := newTestEcommercePack()
	registry.Register(pack)

	results := engine.EvaluateAll(registry, domain.ActionSearchListings, nil)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Allowed {
		t.Errorf("expected allowed, got: %s", results[0].Reason)
	}
}

func TestPolicyEngine_FindAllowedPacks(t *testing.T) {
	engine := newTestPolicyEngine()
	registry := newTestRegistry()

	pack1 := newTestEcommercePack()
	pack2 := domain.NewVerticalPack(domain.PackBooking, "Booking", []domain.PackCapability{domain.CapabilityReservation}, domain.AISpecialistBooking)
	pack2.SupportedActions = []domain.CommerceActionType{domain.ActionSearchListings}
	pack2.Policies = []domain.PackPolicy{
		{Key: "require_payment_before_reservation", Value: "true"},
	}

	registry.Register(pack1)
	registry.Register(pack2)

	allowed := engine.FindAllowedPacks(registry, domain.ActionSearchListings, nil)

	if len(allowed) != 2 {
		t.Errorf("expected 2 allowed packs, got %d", len(allowed))
	}
}
