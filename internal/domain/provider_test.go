package domain

import (
	"context"
	"testing"
)

func TestIntegrationProvider_NewProvider(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test Provider", "hotel", []Capability{CapabilityAvailability, CapabilityReservation}, ProviderConfig{}, "secret-1", 10)

	if p.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", p.OrganizationID)
	}
	if p.Code != "prov-001" {
		t.Errorf("expected prov-001, got %s", p.Code)
	}
	if p.Status != ProviderStatusInactive {
		t.Errorf("expected INACTIVE, got %s", p.Status)
	}
	if p.HealthStatus != ProviderHealthUnknown {
		t.Errorf("expected UNKNOWN, got %s", p.HealthStatus)
	}
	if p.Version != 1 {
		t.Errorf("expected version 1, got %d", p.Version)
	}
	if !p.SupportsCapability(CapabilityAvailability) {
		t.Error("expected to support AVAILABILITY")
	}
	if p.SupportsCapability(CapabilityBooking) {
		t.Error("expected to NOT support BOOKING")
	}
}

func TestIntegrationProvider_Enable(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)

	if err := p.Enable(1000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != ProviderStatusActive {
		t.Errorf("expected ACTIVE, got %s", p.Status)
	}
	if p.Version != 2 {
		t.Errorf("expected version 2, got %d", p.Version)
	}

	if err := p.Enable(1001); err == nil {
		t.Error("expected error when enabling already-active provider")
	}
}

func TestIntegrationProvider_Disable(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)
	p.Status = ProviderStatusActive

	if err := p.Disable(1000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != ProviderStatusInactive {
		t.Errorf("expected INACTIVE, got %s", p.Status)
	}

	p.Status = ProviderStatusInactive
	if err := p.Disable(1001); err == nil {
		t.Error("expected error when disabling already-inactive provider")
	}
}

func TestIntegrationProvider_Suspend(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)
	p.Status = ProviderStatusActive

	if err := p.Suspend(1000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Status != ProviderStatusSuspended {
		t.Errorf("expected SUSPENDED, got %s", p.Status)
	}
}

func TestIntegrationProvider_IsUsable(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)

	p.Status = ProviderStatusActive
	p.HealthStatus = ProviderHealthHealthy
	if !p.IsUsable() {
		t.Error("expected usable")
	}

	p.Status = ProviderStatusInactive
	if p.IsUsable() {
		t.Error("expected not usable when inactive")
	}

	p.Status = ProviderStatusActive
	p.HealthStatus = ProviderHealthUnavailable
	if p.IsUsable() {
		t.Error("expected not usable when health unavailable")
	}
}

func TestIntegrationProvider_ValidateInvariants(t *testing.T) {
	p := NewProvider("org-1", "", "Test", "hotel", nil, ProviderConfig{}, "", 0)
	if err := p.ValidateInvariants(); err == nil {
		t.Error("expected error for empty code")
	}

	p.Code = "test"
	p.Name = ""
	if err := p.ValidateInvariants(); err == nil {
		t.Error("expected error for empty name")
	}

	p.Name = "Test"
	p.Status = ProviderStatusActive
	if err := p.ValidateInvariants(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	p.Status = ProviderStatus("INVALID")
	if err := p.ValidateInvariants(); err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestIntegrationProvider_SetHealth(t *testing.T) {
	p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)
	oldVersion := p.Version

	p.SetHealth(ProviderHealthHealthy, 1000)
	if p.HealthStatus != ProviderHealthHealthy {
		t.Errorf("expected HEALTHY, got %s", p.HealthStatus)
	}
	if p.Version == oldVersion {
		t.Error("expected version to increment")
	}
}

func TestProviderStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   ProviderStatus
		expected bool
	}{
		{"active", ProviderStatusActive, true},
		{"inactive", ProviderStatusInactive, true},
		{"suspended", ProviderStatusSuspended, true},
		{"degraded", ProviderStatusDegraded, true},
		{"invalid", ProviderStatus("bogus"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProvider("org-1", "prov-001", "Test", "hotel", nil, ProviderConfig{}, "", 0)
			p.Status = tt.status
			err := p.ValidateInvariants()
			if tt.expected && err != nil {
				t.Errorf("expected valid status, got error: %v", err)
			}
			if !tt.expected && err == nil {
				t.Error("expected error for invalid status")
			}
		})
	}
}

func TestCapability_Constants(t *testing.T) {
	expected := []Capability{
		CapabilityAvailability, CapabilityReservation, CapabilityBooking,
		CapabilityCatalog, CapabilityDelivery, CapabilityTracking,
		CapabilityLocation, CapabilityNotification, CapabilityIdentityVerify,
		CapabilityPayment, CapabilityFulfillment,
	}
	for _, c := range expected {
		if string(c) == "" {
			t.Errorf("capability %v has empty string", c)
		}
	}
}

func TestProviderRepository_Interface(t *testing.T) {
	var _ ProviderRepository = (ProviderRepository)(nil)
	var _ ProviderExecutionRepository = (ProviderExecutionRepository)(nil)
}

type mockProviderRepo struct{}

func (m *mockProviderRepo) Create(ctx context.Context, provider *IntegrationProvider) error {
	return nil
}
func (m *mockProviderRepo) FindByID(ctx context.Context, orgID, id string) (*IntegrationProvider, error) {
	return nil, ErrProviderNotFound
}
func (m *mockProviderRepo) FindByCode(ctx context.Context, orgID, code string) (*IntegrationProvider, error) {
	return nil, ErrProviderNotFound
}
func (m *mockProviderRepo) FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]*IntegrationProvider, error) {
	return nil, nil
}
func (m *mockProviderRepo) FindByCapability(ctx context.Context, orgID string, capability Capability) ([]*IntegrationProvider, error) {
	return nil, nil
}
func (m *mockProviderRepo) Update(ctx context.Context, provider *IntegrationProvider) error {
	return nil
}
func (m *mockProviderRepo) Delete(ctx context.Context, orgID, id string) error { return nil }
func (m *mockProviderRepo) FindUsableByCapability(ctx context.Context, orgID string, capability Capability) ([]*IntegrationProvider, error) {
	return nil, nil
}

func TestMockProviderRepo_ImplementsInterface(t *testing.T) {
	var _ ProviderRepository = &mockProviderRepo{}
}
