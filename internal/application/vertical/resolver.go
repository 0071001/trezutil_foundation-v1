package vertical

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type PackResolverConfig struct {
	Logger   zerolog.Logger
	Registry *PackRegistry
	Policies *PolicyEngine
	PackRepo domain.OrganizationPackRepository
}

type PackResolver struct {
	mu       sync.RWMutex
	logger   zerolog.Logger
	registry *PackRegistry
	policies *PolicyEngine
	packRepo domain.OrganizationPackRepository
	enabled  map[string]map[domain.PackCode]bool
}

func NewPackResolver(config PackResolverConfig) *PackResolver {
	if config.Logger.GetLevel() == 0 {
		config.Logger = zerolog.Nop()
	}
	return &PackResolver{
		logger:   config.Logger,
		registry: config.Registry,
		policies: config.Policies,
		packRepo: config.PackRepo,
		enabled:  make(map[string]map[domain.PackCode]bool),
	}
}

func (r *PackResolver) EnablePackForOrg(orgID string, code domain.PackCode) error {
	if _, ok := r.registry.Get(code); !ok {
		return fmt.Errorf("pack %s is not registered", code)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.enabled[orgID] == nil {
		r.enabled[orgID] = make(map[domain.PackCode]bool)
	}
	r.enabled[orgID][code] = true
	r.logger.Info().
		Str("org_id", orgID).
		Str("pack_code", string(code)).
		Msg("Enabled pack for organization")
	return nil
}

func (r *PackResolver) DisablePackForOrg(orgID string, code domain.PackCode) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.enabled[orgID] != nil {
		r.enabled[orgID][code] = false
	}
	r.logger.Info().
		Str("org_id", orgID).
		Str("pack_code", string(code)).
		Msg("Disabled pack for organization")
}

func (r *PackResolver) IsPackEnabled(orgID string, code domain.PackCode) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orgPacks, ok := r.enabled[orgID]
	if !ok {
		return false
	}
	return orgPacks[code]
}

func (r *PackResolver) GetAvailablePacks(orgID string) []domain.PackCode {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []domain.PackCode
	orgPacks, ok := r.enabled[orgID]
	if !ok {
		return result
	}
	for code, enabled := range orgPacks {
		if enabled {
			result = append(result, code)
		}
	}
	return result
}

func (r *PackResolver) ResolvePackForAction(orgID string, action domain.CommerceActionType) (domain.PackCode, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	orgPacks, ok := r.enabled[orgID]
	if !ok {
		return "", false
	}

	for code, enabled := range orgPacks {
		if !enabled {
			continue
		}
		pack, exists := r.registry.Get(code)
		if !exists {
			continue
		}
		if pack.SupportsAction(action) {
			return code, true
		}
	}
	return "", false
}

func (r *PackResolver) ResolveSpecialistForAction(action domain.CommerceActionType) (domain.AISpecialistCode, bool) {
	for _, pack := range r.registry.GetAll() {
		if pack.SupportsAction(action) {
			return pack.AIProfile, true
		}
	}
	return "", false
}

func (r *PackResolver) IsActionSupported(orgID string, action domain.CommerceActionType) bool {
	_, ok := r.ResolvePackForAction(orgID, action)
	return ok
}

func (r *PackResolver) ResolvePackWithPolicy(
	orgID string,
	action domain.CommerceActionType,
	ctx map[string]interface{},
) (domain.PackCode, *PolicyEvaluationResult) {
	code, ok := r.ResolvePackForAction(orgID, action)
	if !ok {
		return "", &PolicyEvaluationResult{
			Allowed: false,
			Reason:  fmt.Sprintf("action %s not supported by any enabled pack for org %s", action, orgID),
		}
	}

	pack, _ := r.registry.Get(code)
	result := r.policies.Evaluate(pack, action, ctx)
	if !result.Allowed {
		return "", result
	}

	return code, result
}

func (r *PackResolver) CanTransition(orgID string, action domain.CommerceActionType, fromStep, toStep string) bool {
	code, ok := r.ResolvePackForAction(orgID, action)
	if !ok {
		return false
	}

	pack, _ := r.registry.Get(code)
	if pack.Workflow == nil {
		return false
	}

	return pack.Workflow.ValidTransitions(fromStep, toStep)
}
