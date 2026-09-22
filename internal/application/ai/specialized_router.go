package ai

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type SpecializedModelRouter struct {
	mu          sync.RWMutex
	modelRouter *ModelRouter
	specRouter  *SpecializationRouter
	logger      zerolog.Logger
	specialties map[domain.AISpecialistCode][]domain.AIModel
}

type SpecializedModelRouterConfig struct {
	Logger      zerolog.Logger
	ModelRouter *ModelRouter
	SpecRouter  *SpecializationRouter
}

func NewSpecializedModelRouter(config SpecializedModelRouterConfig) *SpecializedModelRouter {
	if config.ModelRouter == nil {
		config.ModelRouter = NewModelRouter(config.Logger)
	}
	if config.SpecRouter == nil {
		config.SpecRouter = NewSpecializationRouter(SpecializationRouterConfig{
			Logger:      config.Logger,
			DefaultSpec: domain.AISpecialistCommerce,
		})
	}

	return &SpecializedModelRouter{
		modelRouter: config.ModelRouter,
		specRouter:  config.SpecRouter,
		logger:      config.Logger,
		specialties: make(map[domain.AISpecialistCode][]domain.AIModel),
	}
}

func (r *SpecializedModelRouter) RegisterProvider(provider domain.AIProvider) {
	r.modelRouter.RegisterProvider(provider)
}

func (r *SpecializedModelRouter) RegisterSpecialist(specialist *domain.AISpecialistEntity, gateway *AIGateway) {
	r.specRouter.RegisterSpecialist(specialist, gateway)
}

func (r *SpecializedModelRouter) AssignModelToSpecialist(code domain.AISpecialistCode, model domain.AIModel) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.specialties[code] = append(r.specialties[code], model)
}

func (r *SpecializedModelRouter) Route(ctx context.Context, orgID string, task domain.AITaskType) (*domain.AIModel, domain.AIProvider, domain.AISpecialistCode, error) {
	spec, _, err := r.specRouter.RouteRequest(ctx, orgID, task)
	if err != nil {
		return nil, nil, "", err
	}

	r.mu.RLock()
	specialtyModels := r.specialties[spec.Code]
	r.mu.RUnlock()

	if len(specialtyModels) > 0 {
		for _, model := range specialtyModels {
			if model.IsActive() && model.SupportsCapability(task.RequiredCapability()) {
				if task.RequiresStructuredOutput() && !model.SupportsStructured {
					continue
				}
				provider, _ := r.modelRouter.GetProvider(model.ProviderID)
				if provider != nil && provider.IsUsable() {
					r.logger.Debug().
						Str("specialist", string(spec.Code)).
						Str("model", model.ModelID).
						Str("provider", provider.ID()).
						Str("task", string(task)).
						Msg("Specialized router selected specialist-specific model")
					return &model, provider, spec.Code, nil
				}
			}
		}
	}

	model, provider, err := r.modelRouter.Route(ctx, orgID, task)
	if err != nil {
		return nil, nil, spec.Code, err
	}

	r.logger.Debug().
		Str("specialist", string(spec.Code)).
		Str("model", model.ModelID).
		Str("provider", provider.ID()).
		Str("task", string(task)).
		Msg("Specialized router selected global model")

	return model, provider, spec.Code, nil
}

func (r *SpecializedModelRouter) GetSpecialist(code domain.AISpecialistCode) (*domain.AISpecialistEntity, bool) {
	return r.specRouter.GetSpecialist(code)
}

func (r *SpecializedModelRouter) GetDefaultSpecialistCode() domain.AISpecialistCode {
	return r.specRouter.GetDefaultCode()
}

func (r *SpecializedModelRouter) SystemInstructionsFor(code domain.AISpecialistCode) *domain.SystemInstructions {
	return r.specRouter.SystemInstructionsFor(code)
}

func (r *SpecializedModelRouter) CanHandoff(from, to domain.AISpecialistCode) bool {
	return r.specRouter.CanHandoff(from, to)
}

func (r *SpecializedModelRouter) Handoff(ctx context.Context, from, to domain.AISpecialistCode, request domain.AIRequest, reason string) (*domain.AIResponse, error) {
	return r.specRouter.Handoff(ctx, from, to, request, reason)
}

func (r *SpecializedModelRouter) AllSpecialists() []*domain.AISpecialistEntity {
	return r.specRouter.AllActiveSpecialists()
}

func (r *SpecializedModelRouter) ModelProviders() map[string]domain.AIProvider {
	return r.modelRouter.providers
}

func (r *SpecializedModelRouter) Generate(ctx context.Context, orgID string, request domain.AIRequest) (*domain.AIResponse, domain.AISpecialistCode, error) {
	startTime := time.Now()

	spec, gateway, err := r.specRouter.RouteRequest(ctx, orgID, request.Task)
	if err != nil {
		return nil, "", fmt.Errorf("specialization routing failed: %w", err)
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

type SpecializedGateway struct {
	specializedRouter *SpecializedModelRouter
	orgID             string
}

func NewSpecializedGateway(router *SpecializedModelRouter, orgID string) *SpecializedGateway {
	return &SpecializedGateway{
		specializedRouter: router,
		orgID:             orgID,
	}
}

func (g *SpecializedGateway) Generate(ctx context.Context, orgID string, request domain.AIRequest) (*domain.AIResponse, error) {
	model, provider, specCode, err := g.specializedRouter.Route(ctx, orgID, request.Task)
	if err != nil {
		return nil, domain.NewAIError(domain.AIErrModelUnavailable, fmt.Sprintf("specialization routing failed: %v", err))
	}

	request.ModelID = model.ModelID

	resp, err := provider.Generate(ctx, request)
	if err != nil {
		return nil, domain.NewAIError(domain.AIErrProviderUnavailable, fmt.Sprintf("provider %s failed: %v", provider.ID(), err))
	}

	resp.ProviderID = fmt.Sprintf("%s:%s", specCode, provider.ID())
	resp.ModelID = model.ModelID

	return &resp, nil
}

func (g *SpecializedGateway) SpecializationFor(orgID string, task domain.AITaskType) (domain.AISpecialistCode, error) {
	spec, _, err := g.specializedRouter.specRouter.RouteRequest(context.Background(), orgID, task)
	if err != nil {
		return "", err
	}
	return spec.Code, nil
}
