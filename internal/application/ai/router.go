package ai

import (
	"context"
	"sort"
	"sync"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type ModelRouter struct {
	mu             sync.RWMutex
	providers      map[string]domain.AIProvider
	logger         zerolog.Logger
	tenantPolicies map[string]TenantModelPolicy
}

type TenantModelPolicy struct {
	AllowedModels    map[string]bool
	AllowedProviders map[string]bool
}

func NewModelRouter(logger zerolog.Logger) *ModelRouter {
	return &ModelRouter{
		providers:      make(map[string]domain.AIProvider),
		tenantPolicies: make(map[string]TenantModelPolicy),
		logger:         logger,
	}
}

func (r *ModelRouter) RegisterProvider(provider domain.AIProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[provider.ID()] = provider
}

func (r *ModelRouter) UnregisterProvider(providerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.providers, providerID)
}

func (r *ModelRouter) SetTenantPolicy(orgID string, policy TenantModelPolicy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenantPolicies[orgID] = policy
}

func (r *ModelRouter) Route(ctx context.Context, orgID string, task domain.AITaskType) (*domain.AIModel, domain.AIProvider, error) {
	requiredCap := task.RequiredCapability()

	r.mu.RLock()
	defer r.mu.RUnlock()

	policy, hasPolicy := r.tenantPolicies[orgID]

	var candidates []modelCandidate

	for _, provider := range r.providers {
		if !provider.IsUsable() {
			continue
		}
		providerID := provider.ID()

		if hasPolicy && len(policy.AllowedProviders) > 0 {
			if !policy.AllowedProviders[providerID] {
				continue
			}
		}

		for _, model := range provider.Models() {
			if !model.IsActive() {
				continue
			}
			if hasPolicy && len(policy.AllowedModels) > 0 {
				if !policy.AllowedModels[model.ModelID] {
					continue
				}
			}
			if !model.SupportsCapability(requiredCap) {
				continue
			}
			if task.RequiresStructuredOutput() && !model.SupportsStructured {
				continue
			}

			candidates = append(candidates, modelCandidate{
				model:    model,
				provider: provider,
			})
		}
	}

	if len(candidates) == 0 {
		return nil, nil, domain.NewAIError(domain.AIErrModelUnavailable,
			"no active model available for task "+string(task)+" with capability "+string(requiredCap))
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].model.Priority != candidates[j].model.Priority {
			return candidates[i].model.Priority > candidates[j].model.Priority
		}
		if candidates[i].provider.ID() != candidates[j].provider.ID() {
			return candidates[i].provider.ID() < candidates[j].provider.ID()
		}
		return candidates[i].model.ModelID < candidates[j].model.ModelID
	})

	selected := candidates[0]
	r.logger.Debug().
		Str("task", string(task)).
		Str("provider", selected.provider.ID()).
		Str("model", selected.model.ModelID).
		Msg("Model router selected model")

	return &selected.model, selected.provider, nil
}

type modelCandidate struct {
	model    domain.AIModel
	provider domain.AIProvider
}

func (r *ModelRouter) GetProvider(providerID string) (domain.AIProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[providerID]
	return p, ok
}
