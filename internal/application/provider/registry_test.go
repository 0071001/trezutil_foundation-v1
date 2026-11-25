package provider

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type mockProviderAdapter struct {
	id     string
	caps   []domain.Capability
	health domain.ProviderHealthStatus
}

func (m *mockProviderAdapter) ProviderID() string                { return m.id }
func (m *mockProviderAdapter) Capabilities() []domain.Capability { return m.caps }
func (m *mockProviderAdapter) Health(ctx context.Context) domain.ProviderHealthStatus {
	return m.health
}
func (m *mockProviderAdapter) CheckAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	return domain.AvailabilityResult{Available: true}, nil
}

func newTestProvider(orgID, id string, caps []domain.Capability, status domain.ProviderStatus) *domain.IntegrationProvider {
	p := domain.NewProvider(orgID, id, "TestProvider", "test", caps, domain.ProviderConfig{}, "", 0)
	p.ID = id
	p.Status = status
	p.HealthStatus = domain.ProviderHealthHealthy
	return p
}

func TestProviderRegistry_Register(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}

	if err := reg.Register(p, adapter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry, err := reg.FindByID(ctx, "org-1", "prov-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.GetProvider().ID != "prov-1" {
		t.Errorf("expected prov-1, got %s", entry.GetProvider().ID)
	}
}

func TestProviderRegistry_DuplicateRegistration(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", nil, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", health: domain.ProviderHealthHealthy}

	if err := reg.Register(p, adapter); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := reg.Register(p, adapter); err == nil {
		t.Error("expected error for duplicate registration")
	}

	_, err := reg.FindByID(ctx, "org-1", "prov-1")
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}
}

func TestProviderRegistry_Unregister(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", nil, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", health: domain.ProviderHealthHealthy}

	reg.Register(p, adapter)

	if err := reg.Unregister("org-1", "prov-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := reg.FindByID(ctx, "org-1", "prov-1"); err != domain.ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestProviderRegistry_FindByCapability(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p1 := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	p1.Priority = 5
	adapter1 := &mockProviderAdapter{id: "prov-1", caps: p1.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p1, adapter1)

	p2 := newTestProvider("org-1", "prov-2", []domain.Capability{domain.CapabilityReservation}, domain.ProviderStatusActive)
	p2.Priority = 10
	adapter2 := &mockProviderAdapter{id: "prov-2", caps: p2.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p2, adapter2)

	p3 := newTestProvider("org-1", "prov-3", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusInactive)
	adapter3 := &mockProviderAdapter{id: "prov-3", caps: p3.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p3, adapter3)

	entries, err := reg.FindByCapability(ctx, "org-1", domain.CapabilityAvailability)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(entries))
	}
	if entries[0].GetProvider().ID != "prov-1" {
		t.Errorf("expected prov-1, got %s", entries[0].GetProvider().ID)
	}
}

func TestProviderRegistry_DisabledProviderExcluded(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	entries, err := reg.FindByCapability(ctx, "org-1", domain.CapabilityDelivery)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 providers for unsupported capability, got %d", len(entries))
	}
}

func TestProviderRegistry_PriorityOrdering(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p1 := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	p1.Priority = 1
	adapter1 := &mockProviderAdapter{id: "prov-1", caps: p1.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p1, adapter1)

	p2 := newTestProvider("org-1", "prov-2", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	p2.Priority = 10
	adapter2 := &mockProviderAdapter{id: "prov-2", caps: p2.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p2, adapter2)

	entries, err := reg.FindByCapability(ctx, "org-1", domain.CapabilityAvailability)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(entries))
	}
	if entries[0].GetProvider().ID != "prov-2" {
		t.Errorf("expected prov-2 first (priority 10), got %s", entries[0].GetProvider().ID)
	}
}

func TestProviderRegistry_TenantIsolation(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p1 := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter1 := &mockProviderAdapter{id: "prov-1", caps: p1.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p1, adapter1)

	p2 := newTestProvider("org-2", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter2 := &mockProviderAdapter{id: "prov-1", caps: p2.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p2, adapter2)

	entries, err := reg.FindByCapability(ctx, "org-1", domain.CapabilityAvailability)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 provider for org-1, got %d", len(entries))
	}
	if entries[0].GetProvider().OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", entries[0].GetProvider().OrganizationID)
	}
}

func TestPlatformProviderServesOrganisationsWithoutTheirOwn(t *testing.T) {
	r := NewProviderRegistry()
	platform := &domain.IntegrationProvider{ID: "sp", OrganizationID: "trezutil", Code: "saspay", Status: domain.ProviderStatusActive,
		Capabilities: []domain.Capability{domain.CapabilityPayment}, Configuration: domain.ProviderConfig{Settings: map[string]string{"scope": "platform"}}}
	own := &domain.IntegrationProvider{ID: "st", OrganizationID: "seller-b", Code: "stripe", Status: domain.ProviderStatusActive,
		Capabilities: []domain.Capability{domain.CapabilityPayment}}
	for _, p := range []*domain.IntegrationProvider{platform, own} {
		if err := r.Register(p, &mockProviderAdapter{id: p.ID, caps: p.Capabilities, health: domain.ProviderHealthHealthy}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	if got, _ := r.FindByCapability(ctx, "seller-a", domain.CapabilityPayment); len(got) != 1 || got[0].Provider.ID != "sp" {
		t.Errorf("a seller without a provider is not served by the platform: %v", got)
	}
	if got, _ := r.FindByCapability(ctx, "seller-b", domain.CapabilityPayment); len(got) != 1 || got[0].Provider.ID != "st" {
		t.Errorf("a seller's own provider must come first: %v", got)
	}
	if e, err := r.FindByID(ctx, "seller-a", "sp"); err != nil || e.Provider.ID != "sp" {
		t.Errorf("platform provider not found by id for a seller: %v", err)
	}
	if _, err := r.FindByID(ctx, "seller-a", "st"); err == nil {
		t.Error("another seller's own provider was lent out")
	}
}
