package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// OrchestratedAvailabilityProvider is the external implementation of the
// Commerce Core AvailabilityProvider port: it asks the organization's
// availability providers through Provider Orchestration (selection, timeout,
// retry, fallback, execution records). No external provider is connected yet.
//
// Availability is only reported when the provider states it: AVAILABLE or
// UNAVAILABLE. A missing or contradictory answer, a provider error or a
// timeout is UNKNOWN and comes with ErrAvailabilityUnknown, so nothing can be
// booked on it.
type OrchestratedAvailabilityProvider struct {
	orchestrator *ProviderOrchestrator
	timeout      time.Duration
}

func NewOrchestratedAvailabilityProvider(orchestrator *ProviderOrchestrator) *OrchestratedAvailabilityProvider {
	return &OrchestratedAvailabilityProvider{orchestrator: orchestrator, timeout: 5 * time.Second}
}

// WithTimeout bounds each provider attempt.
func (p *OrchestratedAvailabilityProvider) WithTimeout(d time.Duration) *OrchestratedAvailabilityProvider {
	if d > 0 {
		p.timeout = d
	}
	return p
}

// errUnknownAnswer marks a provider answer that does not establish availability.
var errUnknownAnswer = errors.New("provider answer does not establish availability")

func (p *OrchestratedAvailabilityProvider) CheckAvailability(ctx context.Context, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	unknown := domain.AvailabilityResult{State: domain.AvailabilityUnknown, StartAt: query.StartAt, EndAt: query.EndAt}
	if query.TenantID == "" {
		unknown.Reason = "tenant is required"
		return unknown, fmt.Errorf("%w: tenant is required", domain.ErrAvailabilityUnknown)
	}
	quantity := query.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	var answer domain.AvailabilityResult
	result, err := p.orchestrator.ExecuteCapability(ctx, query.TenantID, "", ExecuteOptions{
		Capability: domain.CapabilityAvailability,
		Operation:  "check_availability",
		Timeout:    p.timeout,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		availAdapter, ok := adapter.(domain.AvailabilityProviderAdapter)
		if !ok {
			return domain.ProviderResult{ErrorCode: domain.ErrCodeUnsupportedCapability}, fmt.Errorf("%w: %s", domain.ErrUnsupportedCapability, adapter.ProviderID())
		}
		res, aErr := availAdapter.CheckAvailability(ctx, query)
		if aErr != nil {
			return domain.ProviderResult{}, aErr
		}
		checked, ok := interpretAnswer(res, quantity)
		if !ok {
			// The provider answered without establishing availability: a
			// definitive "unknown", not a transport failure to retry.
			return domain.ProviderResult{Success: false, ErrorCode: domain.ErrCodeInvalidResponse, ErrorMessage: errUnknownAnswer.Error()}, nil
		}
		answer = checked
		return domain.ProviderResult{Success: true}, nil
	})
	switch {
	case err != nil:
		unknown.Reason = reasonFor(result, err)
		return unknown, fmt.Errorf("%w: %s", domain.ErrAvailabilityUnknown, unknown.Reason)
	case result == nil || !result.Success:
		unknown.Reason = string(domain.ErrCodeInvalidResponse)
		return unknown, fmt.Errorf("%w: %s", domain.ErrAvailabilityUnknown, errUnknownAnswer)
	}
	answer.StartAt, answer.EndAt = query.StartAt, query.EndAt
	return answer, nil
}

// interpretAnswer accepts only an explicit, consistent answer: AVAILABLE with
// enough places (when places are reported) or UNAVAILABLE.
func interpretAnswer(res domain.AvailabilityResult, quantity int) (domain.AvailabilityResult, bool) {
	if res.Capacity < 0 || res.Remaining < 0 || (res.Capacity > 0 && res.Remaining > res.Capacity) {
		return domain.AvailabilityResult{}, false
	}
	switch res.State {
	case domain.AvailabilityAvailable:
		if res.Remaining > 0 && res.Remaining < quantity {
			res.State, res.Reason = domain.AvailabilityUnavailable, "insufficient capacity"
			res.Available = false
			return res, true
		}
		res.Available = true
		return res, true
	case domain.AvailabilityUnavailable:
		res.Available = false
		return res, true
	default:
		return domain.AvailabilityResult{}, false
	}
}

func reasonFor(result *ExecuteResult, err error) string {
	if errors.Is(err, domain.ErrNoProvidersAvailable) {
		return "no availability provider"
	}
	if result != nil && result.ErrorCode != "" {
		return string(result.ErrorCode)
	}
	return string(domain.ErrCodeUnknown)
}
