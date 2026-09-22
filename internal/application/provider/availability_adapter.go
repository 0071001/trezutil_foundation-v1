package provider

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type OrchestratedAvailabilityProvider struct {
	orchestrator *ProviderOrchestrator
}

func NewOrchestratedAvailabilityProvider(orchestrator *ProviderOrchestrator) *OrchestratedAvailabilityProvider {
	return &OrchestratedAvailabilityProvider{orchestrator: orchestrator}
}

func (p *OrchestratedAvailabilityProvider) CheckAvailability(
	ctx context.Context,
	query domain.AvailabilityQuery,
) (domain.AvailabilityResult, error) {
	orgID, _ := ctx.Value("tenant_id").(string)

	result, err := p.orchestrator.ExecuteCapability(ctx, orgID, "", ExecuteOptions{
		Capability: domain.CapabilityAvailability,
		Operation:  "check_availability",
		Timeout:    30000000000,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		availAdapter, ok := adapter.(domain.AvailabilityProviderAdapter)
		if !ok {
			return domain.ProviderResult{}, fmt.Errorf("%w: %s", domain.ErrUnsupportedCapability, adapter.ProviderID())
		}
		result, aErr := availAdapter.CheckAvailability(ctx, query)
		if aErr != nil {
			return domain.ProviderResult{Success: false}, aErr
		}
		return domain.ProviderResult{
			Success: true,
			Data: map[string]interface{}{
				"available": result.Available,
				"capacity":  result.Capacity,
				"remaining": result.Remaining,
			},
		}, nil
	})
	if err != nil {
		return domain.AvailabilityResult{}, err
	}

	avail, _ := result.Data["available"].(bool)
	capacity, _ := result.Data["capacity"].(int)
	remaining, _ := result.Data["remaining"].(int)

	return domain.AvailabilityResult{
		Available: avail || result.Success,
		Capacity:  capacity,
		Remaining: remaining,
		StartAt:   query.StartAt,
		EndAt:     query.EndAt,
	}, nil
}
