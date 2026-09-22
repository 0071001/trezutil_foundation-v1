package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type ProviderUseCases struct {
	repo     domain.ProviderRepository
	orgRepo  domain.OrganizationRepository
	userRepo domain.UserRepository
	logger   zerolog.Logger
}

type ProviderUseCaseConfig struct {
	Logger zerolog.Logger
}

func NewProviderUseCases(
	repo domain.ProviderRepository,
	orgRepo domain.OrganizationRepository,
	userRepo domain.UserRepository,
	config ProviderUseCaseConfig,
) *ProviderUseCases {
	return &ProviderUseCases{
		repo:     repo,
		orgRepo:  orgRepo,
		userRepo: userRepo,
		logger:   config.Logger,
	}
}

type CreateProviderRequest struct {
	Code          string
	Name          string
	Type          string
	Capabilities  []string
	Configuration ProviderConfigInput
	SecretRef     string
	Priority      int
}

type ProviderConfigInput struct {
	Endpoints map[string]string
	Settings  map[string]string
}

type ProviderResponse struct {
	ID             string
	OrganizationID string
	Code           string
	Name           string
	Type           string
	Status         string
	Capabilities   []string
	Configuration  ProviderConfigInput
	SecretRef      string `json:"-"`
	Priority       int
	HealthStatus   string
	CreatedAt      int64
	UpdatedAt      int64
}

func (uc *ProviderUseCases) CreateProvider(ctx context.Context, actorID string, orgID string, req CreateProviderRequest) (*ProviderResponse, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}
	if req.Code == "" || req.Name == "" {
		return nil, fmt.Errorf("code and name are required")
	}

	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return nil, domain.ErrInsufficientPermissions
	}
	if !(actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator)) {
		if !uc.canManageProviders(ctx, orgID, actorID) {
			return nil, domain.ErrInsufficientPermissions
		}
	}

	if _, err := uc.orgRepo.FindByID(ctx, orgID); err != nil {
		return nil, domain.ErrOrganizationNotFound
	}

	provider := &domain.IntegrationProvider{
		ID:             generateID(),
		OrganizationID: orgID,
		Code:           req.Code,
		Name:           req.Name,
		Type:           req.Type,
		Status:         domain.ProviderStatusInactive,
		Capabilities:   toCapabilities(req.Capabilities),
		Configuration:  domain.ProviderConfig{Endpoints: req.Configuration.Endpoints, Settings: req.Configuration.Settings},
		SecretRef:      req.SecretRef,
		Priority:       req.Priority,
		HealthStatus:   domain.ProviderHealthUnknown,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
		Version:        1,
	}

	if err := provider.ValidateInvariants(); err != nil {
		return nil, fmt.Errorf("invalid provider: %w", err)
	}

	if err := uc.repo.Create(ctx, provider); err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	uc.logger.Info().
		Str("provider_id", provider.ID).
		Str("organization_id", orgID).
		Str("actor_id", actorID).
		Msg("Provider created")

	return toProviderResponse(provider), nil
}

func (uc *ProviderUseCases) GetProvider(ctx context.Context, actorID string, orgID string, providerID string) (*ProviderResponse, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}

	if err := uc.authorizeProviderAccess(ctx, orgID, actorID); err != nil {
		return nil, err
	}

	provider, err := uc.repo.FindByID(ctx, orgID, providerID)
	if err != nil {
		return nil, err
	}

	return toProviderResponse(provider), nil
}

func (uc *ProviderUseCases) ListProviders(ctx context.Context, actorID string, orgID string, offset, limit int) ([]*ProviderResponse, error) {
	if actorID == "" {
		return nil, domain.ErrInsufficientPermissions
	}
	if orgID == "" {
		return nil, fmt.Errorf("organization_id is required")
	}

	if err := uc.authorizeProviderAccess(ctx, orgID, actorID); err != nil {
		return nil, err
	}

	providers, err := uc.repo.FindByOrganizationID(ctx, orgID, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list providers: %w", err)
	}

	responses := make([]*ProviderResponse, len(providers))
	for i, p := range providers {
		responses[i] = toProviderResponse(p)
	}
	return responses, nil
}

func (uc *ProviderUseCases) EnableProvider(ctx context.Context, actorID string, orgID string, providerID string) error {
	if err := uc.authorizeProviderManagement(ctx, orgID, actorID); err != nil {
		return err
	}

	provider, err := uc.repo.FindByID(ctx, orgID, providerID)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if err := provider.Enable(now); err != nil {
		return err
	}
	if err := provider.ValidateInvariants(); err != nil {
		return err
	}

	if err := uc.repo.Update(ctx, provider); err != nil {
		return err
	}

	uc.logger.Info().
		Str("provider_id", providerID).
		Str("organization_id", orgID).
		Str("actor_id", actorID).
		Msg("Provider enabled")

	return nil
}

func (uc *ProviderUseCases) DisableProvider(ctx context.Context, actorID string, orgID string, providerID string) error {
	if err := uc.authorizeProviderManagement(ctx, orgID, actorID); err != nil {
		return err
	}

	provider, err := uc.repo.FindByID(ctx, orgID, providerID)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	if err := provider.Disable(now); err != nil {
		return err
	}
	if err := provider.ValidateInvariants(); err != nil {
		return err
	}

	if err := uc.repo.Update(ctx, provider); err != nil {
		return err
	}

	uc.logger.Info().
		Str("provider_id", providerID).
		Str("organization_id", orgID).
		Str("actor_id", actorID).
		Msg("Provider disabled")

	return nil
}

func (uc *ProviderUseCases) GetProviderCapabilities(ctx context.Context, actorID string, orgID string, providerID string) ([]string, error) {
	if err := uc.authorizeProviderAccess(ctx, orgID, actorID); err != nil {
		return nil, err
	}

	provider, err := uc.repo.FindByID(ctx, orgID, providerID)
	if err != nil {
		return nil, err
	}

	caps := make([]string, len(provider.Capabilities))
	for i, c := range provider.Capabilities {
		caps[i] = string(c)
	}
	return caps, nil
}

func (uc *ProviderUseCases) authorizeProviderAccess(ctx context.Context, orgID string, actorID string) error {
	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return nil
	}

	memberRole, err := uc.orgRepo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}
	if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin || memberRole == domain.OrganizationRoleManager {
		return nil
	}

	return domain.ErrInsufficientPermissions
}

func (uc *ProviderUseCases) authorizeProviderManagement(ctx context.Context, orgID string, actorID string) error {
	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}

	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return nil
	}

	memberRole, err := uc.orgRepo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return domain.ErrInsufficientPermissions
	}
	if memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin {
		return nil
	}

	return domain.ErrInsufficientPermissions
}

func (uc *ProviderUseCases) canManageProviders(ctx context.Context, orgID string, actorID string) bool {
	actor, err := uc.userRepo.FindByID(ctx, actorID)
	if err != nil {
		return false
	}
	if actor.HasRole(domain.RoleAdmin) || actor.HasRole(domain.RoleOperator) {
		return true
	}
	memberRole, err := uc.orgRepo.GetMemberRole(ctx, orgID, actorID)
	if err != nil {
		return false
	}
	return memberRole == domain.OrganizationRoleOwner || memberRole == domain.OrganizationRoleAdmin
}

func toCapabilities(caps []string) []domain.Capability {
	result := make([]domain.Capability, len(caps))
	for i, c := range caps {
		result[i] = domain.Capability(c)
	}
	return result
}

func toProviderResponse(p *domain.IntegrationProvider) *ProviderResponse {
	caps := make([]string, len(p.Capabilities))
	for i, c := range p.Capabilities {
		caps[i] = string(c)
	}
	return &ProviderResponse{
		ID:             p.ID,
		OrganizationID: p.OrganizationID,
		Code:           p.Code,
		Name:           p.Name,
		Type:           p.Type,
		Status:         string(p.Status),
		Capabilities:   caps,
		Configuration: ProviderConfigInput{
			Endpoints: p.Configuration.Endpoints,
			Settings:  p.Configuration.Settings,
		},
		SecretRef:    p.SecretRef,
		Priority:     p.Priority,
		HealthStatus: string(p.HealthStatus),
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
	}
}

func generateID() string {
	return fmt.Sprintf("p_%d", time.Now().UnixNano())
}
