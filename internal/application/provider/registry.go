package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/trezutil/trezutil/internal/domain"
)

type ProviderRegistry struct {
	mu        sync.RWMutex
	providers map[string]*RegistryEntry
}

type RegistryEntry struct {
	Provider *domain.IntegrationProvider
	Adapter  domain.ProviderAdapter
}

func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]*RegistryEntry),
	}
}

func (r *ProviderRegistry) Register(provider *domain.IntegrationProvider, adapter domain.ProviderAdapter) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}
	if provider.ID == "" {
		return fmt.Errorf("provider ID is required")
	}
	if adapter == nil {
		return fmt.Errorf("adapter cannot be nil")
	}
	if adapter.ProviderID() != provider.ID {
		return fmt.Errorf("adapter provider ID mismatch")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.registryKey(provider.OrganizationID, provider.ID)
	if _, exists := r.providers[key]; exists {
		return fmt.Errorf("provider %s already registered for organization %s", provider.ID, provider.OrganizationID)
	}

	r.providers[key] = &RegistryEntry{
		Provider: provider,
		Adapter:  adapter,
	}
	return nil
}

// UpdateProvider replaces the stored record of a registered provider (status, priority, health) and keeps its
// adapter, so a provider switched off in the database stops being selected at the next reload. It reports
// whether the provider was registered.
func (r *ProviderRegistry) UpdateProvider(provider *domain.IntegrationProvider) bool {
	if provider == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.providers[r.registryKey(provider.OrganizationID, provider.ID)]
	if !ok {
		return false
	}
	entry.Provider = provider
	return true
}

func (r *ProviderRegistry) Unregister(orgID, providerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.registryKey(orgID, providerID)
	if _, exists := r.providers[key]; !exists {
		return fmt.Errorf("provider %s not found for organization %s", providerID, orgID)
	}
	delete(r.providers, key)
	return nil
}

func (r *ProviderRegistry) FindByID(ctx context.Context, orgID, providerID string) (*RegistryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := r.registryKey(orgID, providerID)
	entry, exists := r.providers[key]
	if !exists {
		return nil, domain.ErrProviderNotFound
	}
	return entry, nil
}

func (r *ProviderRegistry) FindByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*RegistryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*RegistryEntry
	for _, entry := range r.providers {
		if entry.Provider.OrganizationID != orgID {
			continue
		}
		if !entry.Provider.IsUsable() {
			continue
		}
		if !entry.Provider.SupportsCapability(capability) {
			continue
		}
		results = append(results, entry)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Provider.Priority > results[j].Provider.Priority
	})
	return results, nil
}

func (r *ProviderRegistry) FindAll(ctx context.Context, orgID string, offset, limit int) ([]*RegistryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*RegistryEntry
	for _, entry := range r.providers {
		if entry.Provider.OrganizationID != orgID {
			continue
		}
		results = append(results, entry)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Provider.Priority > results[j].Provider.Priority
	})

	if offset >= len(results) {
		return []*RegistryEntry{}, nil
	}
	end := offset + limit
	if end > len(results) {
		end = len(results)
	}
	return results[offset:end], nil
}

func (r *RegistryEntry) GetAdapter() domain.ProviderAdapter {
	return r.Adapter
}

func (r *RegistryEntry) GetProvider() *domain.IntegrationProvider {
	return r.Provider
}

func (r *ProviderRegistry) registryKey(orgID, providerID string) string {
	return orgID + ":" + providerID
}
