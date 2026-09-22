package provider

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestCreateProvider(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{"admin-1": "org-1"}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1", Roles: []domain.Role{domain.RoleAdmin}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	req := CreateProviderRequest{
		Code:         "test-prov",
		Name:         "Test Provider",
		Type:         "hotel",
		Capabilities: []string{"AVAILABILITY", "RESERVATION"},
		SecretRef:    "secret-ref-1",
		Priority:     5,
	}

	result, err := uc.CreateProvider(context.Background(), "admin-1", "org-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Code != "test-prov" {
		t.Errorf("expected code test-prov, got %s", result.Code)
	}
	if result.Name != "Test Provider" {
		t.Errorf("expected name Test Provider, got %s", result.Name)
	}
	if result.Status != string(domain.ProviderStatusInactive) {
		t.Errorf("expected INACTIVE on creation, got %s", result.Status)
	}
	if len(result.Capabilities) != 2 {
		t.Errorf("expected 2 capabilities, got %d", len(result.Capabilities))
	}
}

func TestCreateProvider_Unauthorized(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"customer-1": {ID: "customer-1", Roles: []domain.Role{domain.RoleCustomer}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	req := CreateProviderRequest{
		Code: "test-prov",
		Name: "Test Provider",
	}

	_, err := uc.CreateProvider(context.Background(), "customer-1", "org-1", req)
	if err != domain.ErrInsufficientPermissions {
		t.Errorf("expected ErrInsufficientPermissions, got %v", err)
	}
}

func TestGetProvider_NotFound(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{"admin-1": "org-1"}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1", Roles: []domain.Role{domain.RoleAdmin}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	_, err := uc.GetProvider(context.Background(), "admin-1", "org-1", "nonexistent")
	if err != domain.ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestEnableProvider(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{"admin-1": "org-1"}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1", Roles: []domain.Role{domain.RoleAdmin}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	req := CreateProviderRequest{
		Code: "test-prov",
		Name: "Test Provider",
		Type: "hotel",
	}

	result, err := uc.CreateProvider(context.Background(), "admin-1", "org-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	provider, _ := uc.repo.FindByID(context.Background(), "org-1", result.ID)
	if provider.Status != domain.ProviderStatusInactive {
		t.Errorf("expected INACTIVE on creation, got %s", provider.Status)
	}

	if err := uc.EnableProvider(context.Background(), "admin-1", "org-1", result.ID); err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	provider, err = uc.repo.FindByID(context.Background(), "org-1", result.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Status != domain.ProviderStatusActive {
		t.Errorf("expected ACTIVE after enable, got %s", provider.Status)
	}
}

func TestDisableProvider(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{"admin-1": "org-1"}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1", Roles: []domain.Role{domain.RoleAdmin}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	req := CreateProviderRequest{
		Code: "test-prov",
		Name: "Test Provider",
	}

	result, err := uc.CreateProvider(context.Background(), "admin-1", "org-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := uc.EnableProvider(context.Background(), "admin-1", "org-1", result.ID); err != nil {
		t.Fatalf("unexpected error enabling: %v", err)
	}

	if err := uc.DisableProvider(context.Background(), "admin-1", "org-1", result.ID); err != nil {
		t.Fatalf("unexpected error disabling: %v", err)
	}

	provider, err := uc.repo.FindByID(context.Background(), "org-1", result.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if provider.Status != domain.ProviderStatusInactive {
		t.Errorf("expected INACTIVE after disable, got %s", provider.Status)
	}
}

func TestGetProviderCapabilities(t *testing.T) {
	repo := newMockProviderRepo("org-1")
	orgRepo := &mockOrgRepo{orgIDs: map[string]bool{"org-1": true}, memberOrg: map[string]string{"admin-1": "org-1"}}
	userRepo := &mockUserRepo{users: map[string]*domain.User{
		"admin-1": {ID: "admin-1", Roles: []domain.Role{domain.RoleAdmin}},
	}}

	uc := NewProviderUseCases(repo, orgRepo, userRepo, ProviderUseCaseConfig{})

	req := CreateProviderRequest{
		Code:         "test-prov",
		Name:         "Test Provider",
		Capabilities: []string{"AVAILABILITY", "RESERVATION", "TRACKING"},
	}

	result, err := uc.CreateProvider(context.Background(), "admin-1", "org-1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	caps, err := uc.GetProviderCapabilities(context.Background(), "admin-1", "org-1", result.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caps) != 3 {
		t.Errorf("expected 3 capabilities, got %d", len(caps))
	}
}
