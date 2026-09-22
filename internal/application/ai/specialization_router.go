package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type SpecializationRouter struct {
	mu          sync.RWMutex
	logger      zerolog.Logger
	specialists map[domain.AISpecialistCode]*domain.AISpecialistEntity
	gateways    map[domain.AISpecialistCode]*AIGateway
	defaultCode domain.AISpecialistCode
}

type SpecializationRouterConfig struct {
	Logger      zerolog.Logger
	DefaultSpec domain.AISpecialistCode
}

func NewSpecializationRouter(config SpecializationRouterConfig) *SpecializationRouter {
	if config.DefaultSpec == "" {
		config.DefaultSpec = domain.AISpecialistCommerce
	}
	return &SpecializationRouter{
		logger:      config.Logger,
		specialists: make(map[domain.AISpecialistCode]*domain.AISpecialistEntity),
		gateways:    make(map[domain.AISpecialistCode]*AIGateway),
		defaultCode: config.DefaultSpec,
	}
}

func (r *SpecializationRouter) RegisterSpecialist(specialist *domain.AISpecialistEntity, gateway *AIGateway) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.specialists[specialist.Code] = specialist
	if gateway != nil {
		r.gateways[specialist.Code] = gateway
	}
}

func (r *SpecializationRouter) GetSpecialist(code domain.AISpecialistCode) (*domain.AISpecialistEntity, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, ok := r.specialists[code]
	return s, ok
}

func (r *SpecializationRouter) GetGateway(code domain.AISpecialistCode) (*AIGateway, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	g, ok := r.gateways[code]
	return g, ok
}

func (r *SpecializationRouter) GetDefaultCode() domain.AISpecialistCode {
	return r.defaultCode
}

func (r *SpecializationRouter) RouteIntent(ctx context.Context, intent *domain.AIIntent) (*domain.AISpecialistEntity, error) {
	if intent == nil {
		return r.getDefaultSpecialist(), nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// First, check if the intent action is exclusively owned by one specialist
	specialists := domain.GetSpecialistsForAction(intent.Intent)
	if len(specialists) == 0 {
		return r.getDefaultSpecialist(), nil
	}

	// Filter to active specialists
	active := make([]domain.AISpecialistCode, 0, len(specialists))
	for _, code := range specialists {
		if s, ok := r.specialists[code]; ok && s.IsActive() {
			active = append(active, code)
		}
	}

	if len(active) == 0 {
		return r.getDefaultSpecialist(), nil
	}

	if len(active) == 1 {
		return r.specialists[active[0]], nil
	}

	// Multiple specialists can handle this action — use confidence and priority
	selected := r.selectByConfidence(active, intent.Confidence)
	if selected == nil {
		return r.getDefaultSpecialist(), nil
	}

	return selected, nil
}

func (r *SpecializationRouter) selectByConfidence(candidates []domain.AISpecialistCode, confidence float64) *domain.AISpecialistEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var best *domain.AISpecialistEntity
	for _, code := range candidates {
		s, ok := r.specialists[code]
		if !ok || !s.IsActive() {
			continue
		}
		if best == nil || len(s.OwnerAction) > len(best.OwnerAction) {
			best = s
		}
	}
	return best
}

func (r *SpecializationRouter) getDefaultSpecialist() *domain.AISpecialistEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if s, ok := r.specialists[r.defaultCode]; ok {
		return s
	}
	return nil
}

func (r *SpecializationRouter) AllActiveSpecialists() []*domain.AISpecialistEntity {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*domain.AISpecialistEntity, 0, len(r.specialists))
	for _, s := range r.specialists {
		if s.IsActive() {
			result = append(result, s)
		}
	}
	return result
}

func (r *SpecializationRouter) CanHandoff(from, to domain.AISpecialistCode) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	spec, ok := r.specialists[from]
	if !ok {
		return false
	}
	return spec.CanHandoffTo(to)
}

func (r *SpecializationRouter) RouteRequest(ctx context.Context, orgID string, task domain.AITaskType) (*domain.AISpecialistEntity, *AIGateway, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use organization context to determine specialization
	specialization := r.resolveSpecializationForOrg(orgID)

	spec, ok := r.specialists[specialization]
	if !ok || !spec.IsActive() {
		spec = r.getDefaultSpecialist()
		if spec == nil {
			return nil, nil, fmt.Errorf("no active specialist available")
		}
	}

	gateway, ok := r.gateways[spec.Code]
	if !ok {
		return spec, nil, nil
	}

	return spec, gateway, nil
}

func (r *SpecializationRouter) resolveSpecializationForOrg(orgID string) domain.AISpecialistCode {
	// In a real implementation, this would look up org preferences
	// For now, use the default
	return r.defaultCode
}

func (r *SpecializationRouter) SystemInstructionsFor(code domain.AISpecialistCode) *domain.SystemInstructions {
	return domain.DefaultSystemInstructions(code)
}

func (r *SpecializationRouter) GenerateWithSpecialist(
	ctx context.Context,
	specCode domain.AISpecialistCode,
	orgID string,
	request domain.AIRequest,
) (*domain.AIResponse, domain.AISpecialistCode, error) {
	r.mu.RLock()
	gateway, hasGateway := r.gateways[specCode]
	r.mu.RUnlock()

	if !hasGateway {
		return nil, "", fmt.Errorf("no gateway registered for specialist %s", specCode)
	}

	resp, err := gateway.Generate(ctx, orgID, request)
	if err != nil {
		return nil, specCode, err
	}

	resp.ProviderID = fmt.Sprintf("%s|%s", specCode, resp.ProviderID)

	return resp, specCode, nil
}

func (r *SpecializationRouter) Handoff(ctx context.Context, from, to domain.AISpecialistCode, request domain.AIRequest, reason string) (*domain.AIResponse, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if !r.canHandoffUnlocked(from, to) {
		return nil, fmt.Errorf("specialist %s cannot handoff to %s", from, to)
	}

	gateway, ok := r.gateways[to]
	if !ok {
		return nil, fmt.Errorf("no gateway registered for target specialist %s", to)
	}

	if len(reason) == 0 {
		reason = "cross-specialist handoff"
	}

	r.logger.Info().
		Str("from", string(from)).
		Str("to", string(to)).
		Str("reason", reason).
		Str("org_id", request.TraceID).
		Msg("Performing cross-specialist handoff")

	resp, err := gateway.Generate(ctx, "", request)
	if err != nil {
		return nil, err
	}

	resp.ProviderID = fmt.Sprintf("handoff:%s|%s", from, resp.ProviderID)

	return resp, nil
}

func (r *SpecializationRouter) canHandoffUnlocked(from, to domain.AISpecialistCode) bool {
	spec, ok := r.specialists[from]
	if !ok {
		return false
	}
	return spec.CanHandoffTo(to)
}

func (r *SpecializationRouter) Generate(ctx context.Context, orgID string, request domain.AIRequest) (*domain.AIResponse, domain.AISpecialistCode, error) {
	startTime := time.Now()

	spec, gateway, err := r.RouteRequest(ctx, orgID, request.Task)
	if err != nil {
		return nil, "", err
	}

	if gateway == nil {
		return nil, spec.Code, fmt.Errorf("no gateway available for specialist %s", spec.Code)
	}

	resp, err := gateway.Generate(ctx, orgID, request)
	if err != nil {
		r.logger.Error().
			Str("specialist", string(spec.Code)).
			Str("org_id", orgID).
			Dur("latency", time.Since(startTime)).
			Err(err).
			Msg("Specialist gateway generation failed")
		return nil, spec.Code, err
	}

	resp.ProviderID = fmt.Sprintf("%s|%s", spec.Code, resp.ProviderID)

	r.logger.Debug().
		Str("specialist", string(spec.Code)).
		Str("org_id", orgID).
		Str("task", string(request.Task)).
		Str("model", request.ModelID).
		Dur("latency", time.Since(startTime)).
		Msg("Specialist generated response")

	return resp, spec.Code, nil
}

func (r *SpecializationRouter) CountSpecialists() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.specialists)
}
