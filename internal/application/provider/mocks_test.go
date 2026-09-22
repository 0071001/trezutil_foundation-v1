package provider

import (
	"context"

	"github.com/trezutil/trezutil/internal/domain"
)

type mockProviderRepo struct {
	providers map[string]*domain.IntegrationProvider
	orgIDs    map[string]bool
}

func newMockProviderRepo(org string) *mockProviderRepo {
	return &mockProviderRepo{
		providers: make(map[string]*domain.IntegrationProvider),
		orgIDs:    map[string]bool{org: true},
	}
}

func (m *mockProviderRepo) Create(ctx context.Context, provider *domain.IntegrationProvider) error {
	m.providers[provider.ID] = provider
	return nil
}

func (m *mockProviderRepo) FindByID(ctx context.Context, orgID, id string) (*domain.IntegrationProvider, error) {
	if !m.orgIDs[orgID] {
		return nil, domain.ErrProviderNotFound
	}
	p, ok := m.providers[id]
	if !ok {
		return nil, domain.ErrProviderNotFound
	}
	return p, nil
}

func (m *mockProviderRepo) FindByCode(ctx context.Context, orgID, code string) (*domain.IntegrationProvider, error) {
	for _, p := range m.providers {
		if p.Code == code {
			return p, nil
		}
	}
	return nil, domain.ErrProviderNotFound
}

func (m *mockProviderRepo) FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]*domain.IntegrationProvider, error) {
	var result []*domain.IntegrationProvider
	for _, p := range m.providers {
		if p.OrganizationID == orgID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProviderRepo) FindByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error) {
	var result []*domain.IntegrationProvider
	for _, p := range m.providers {
		if p.OrganizationID == orgID && p.SupportsCapability(capability) {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockProviderRepo) Update(ctx context.Context, provider *domain.IntegrationProvider) error {
	m.providers[provider.ID] = provider
	return nil
}

func (m *mockProviderRepo) Delete(ctx context.Context, orgID, id string) error {
	delete(m.providers, id)
	return nil
}

func (m *mockProviderRepo) FindUsableByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error) {
	var result []*domain.IntegrationProvider
	for _, p := range m.providers {
		if p.OrganizationID == orgID && p.IsUsable() && p.SupportsCapability(capability) {
			result = append(result, p)
		}
	}
	return result, nil
}

type mockOrgRepo struct {
	orgIDs    map[string]bool
	memberOrg map[string]string
}

func (m *mockOrgRepo) Create(ctx context.Context, org *domain.Organization) error { return nil }
func (m *mockOrgRepo) FindByID(ctx context.Context, id string) (*domain.Organization, error) {
	if m.orgIDs[id] {
		return &domain.Organization{ID: id}, nil
	}
	return nil, domain.ErrOrganizationNotFound
}
func (m *mockOrgRepo) Update(ctx context.Context, org *domain.Organization) error { return nil }
func (m *mockOrgRepo) AddMember(ctx context.Context, member *domain.OrganizationMember) error {
	return nil
}
func (m *mockOrgRepo) RemoveMember(ctx context.Context, organizationID, userID string) error {
	return nil
}
func (m *mockOrgRepo) GetMembers(ctx context.Context, organizationID string) ([]domain.OrganizationMember, error) {
	return nil, nil
}
func (m *mockOrgRepo) HasMember(ctx context.Context, organizationID, userID string) (bool, error) {
	return false, nil
}
func (m *mockOrgRepo) GetMemberRole(ctx context.Context, organizationID, userID string) (domain.OrganizationRole, error) {
	if m.memberOrg != nil {
		if org, ok := m.memberOrg[userID]; ok && org == organizationID {
			return domain.OrganizationRoleOwner, nil
		}
	}
	return "", domain.ErrInsufficientPermissions
}

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrOrganizationNotFound
	}
	return u, nil
}
func (m *mockUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) AssignRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *mockUserRepo) RemoveRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *mockUserRepo) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	if u, ok := m.users[userID]; ok {
		return u.Roles, nil
	}
	return nil, nil
}
func (m *mockUserRepo) HasRole(ctx context.Context, userID string, role domain.Role) (bool, error) {
	if u, ok := m.users[userID]; ok {
		return u.HasRole(role), nil
	}
	return false, nil
}

type mockProviderExecutionRepo struct{}

func (m *mockProviderExecutionRepo) Create(ctx context.Context, exec *domain.ProviderExecution) error {
	return nil
}
func (m *mockProviderExecutionRepo) FindByID(ctx context.Context, orgID, id string) (*domain.ProviderExecution, error) {
	return nil, domain.ErrProviderNotFound
}
func (m *mockProviderExecutionRepo) FindByIdempotencyKey(ctx context.Context, orgID, key string) (*domain.ProviderExecution, error) {
	return nil, domain.ErrProviderNotFound
}
func (m *mockProviderExecutionRepo) FindByProviderID(ctx context.Context, orgID, providerID string, offset, limit int) ([]*domain.ProviderExecution, error) {
	return nil, nil
}
