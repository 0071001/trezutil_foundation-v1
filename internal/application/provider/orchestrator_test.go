package provider

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type mockRepo struct {
	err       error
	execution *domain.ProviderExecution
}

func (m *mockRepo) Create(ctx context.Context, exec *domain.ProviderExecution) error {
	m.execution = exec
	return m.err
}
func (m *mockRepo) FindByID(ctx context.Context, orgID, id string) (*domain.ProviderExecution, error) {
	return m.execution, m.err
}
func (m *mockRepo) FindByIdempotencyKey(ctx context.Context, orgID, key string) (*domain.ProviderExecution, error) {
	if m.execution != nil && m.execution.IdempotencyKey == key {
		return m.execution, nil
	}
	return nil, domain.ErrProviderNotFound
}
func (m *mockRepo) FindByProviderID(ctx context.Context, orgID, providerID string, offset, limit int) ([]*domain.ProviderExecution, error) {
	return nil, nil
}

func newTestOrchestrator(reg *ProviderRegistry, repo domain.ProviderExecutionRepository) *ProviderOrchestrator {
	return NewProviderOrchestrator(reg, repo, OrchestratorConfig{
		Logger: zerolog.Nop(),
	})
}

func TestOrchestrator_SelectsProviderByCapability(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	orch := newTestOrchestrator(reg, repo)

	entry, err := orch.SelectProvider(ctx, "org-1", domain.CapabilityAvailability, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.GetProvider().ID != "prov-1" {
		t.Errorf("expected prov-1, got %s", entry.GetProvider().ID)
	}
}

func TestOrchestrator_InactiveProviderExcluded(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusInactive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	orch := newTestOrchestrator(reg, repo)

	entry, err := orch.SelectProvider(ctx, "org-1", domain.CapabilityAvailability, nil)
	if err != domain.ErrNoProvidersAvailable {
		t.Errorf("expected ErrNoProvidersAvailable, got %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry for inactive provider")
	}
}

func TestOrchestrator_UnsupportedCapability(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	orch := newTestOrchestrator(reg, repo)

	entry, err := orch.SelectProvider(ctx, "org-1", domain.CapabilityDelivery, nil)
	if err != domain.ErrNoProvidersAvailable {
		t.Errorf("expected ErrNoProvidersAvailable, got %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry")
	}
}

func TestOrchestrator_RetryableErrorRetries(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	origPolicy := reg
	_ = origPolicy
	orch := newTestOrchestrator(reg, repo)
	orch.retryPolicy = domain.RetryPolicy{
		MaxAttempts: 3,
		BaseDelayMs: 1,
		MaxDelayMs:  10,
		ClassifyError: func(err error) domain.RetryClassification {
			if err == nil {
				return domain.NonRetryable
			}
			if err.Error() == "transient error" {
				return domain.Retryable
			}
			return domain.NonRetryable
		},
	}

	callCount := 0
	result, err := orch.ExecuteCapability(ctx, "org-1", "actor-1", ExecuteOptions{
		Capability:     domain.CapabilityAvailability,
		Operation:      "check_availability",
		IdempotencyKey: "idem-1",
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		callCount++
		if callCount < 3 {
			return domain.ProviderResult{}, errors.New("transient error")
		}
		return domain.ProviderResult{Success: true, Data: map[string]interface{}{"available": true}}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success after retries")
	}
	if result.RetryCount != 2 {
		t.Errorf("expected 2 retries, got %d", result.RetryCount)
	}
}

func TestOrchestrator_NonRetryableErrorNoRetry(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	orch := newTestOrchestrator(reg, repo)
	orch.retryPolicy = domain.RetryPolicy{
		MaxAttempts: 3,
		BaseDelayMs: 1,
		MaxDelayMs:  10,
		ClassifyError: func(err error) domain.RetryClassification {
			if err == nil {
				return domain.NonRetryable
			}
			if err.Error() == "not found" {
				return domain.NonRetryable
			}
			return domain.Retryable
		},
	}

	callCount := 0
	_, err := orch.ExecuteCapability(ctx, "org-1", "actor-1", ExecuteOptions{
		Capability: domain.CapabilityAvailability,
		Operation:  "check_availability",
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		callCount++
		return domain.ProviderResult{}, errors.New("not found")
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry), got %d", callCount)
	}
}

func TestOrchestrator_ExponentialBackoff(t *testing.T) {
	policy := domain.DefaultRetryPolicy()
	if policy.MaxAttempts != 3 {
		t.Errorf("expected max attempts 3, got %d", policy.MaxAttempts)
	}
	if policy.BaseDelayMs != 100 {
		t.Errorf("expected base delay 100ms, got %d", policy.BaseDelayMs)
	}
}

func TestOrchestrator_Idempotency(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	callCount := 0
	result, err := newTestOrchestrator(reg, &mockRepo{}).ExecuteCapability(ctx, "org-1", "actor-1", ExecuteOptions{
		Capability:     domain.CapabilityAvailability,
		Operation:      "check_availability",
		IdempotencyKey: "idem-key-1",
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		callCount++
		if callCount > 1 {
			return domain.ProviderResult{}, fmt.Errorf("duplicate call with idempotency key")
		}
		return domain.ProviderResult{Success: true, Data: map[string]interface{}{"available": true}}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestOrchestrator_TenantIsolation(t *testing.T) {
	reg := NewProviderRegistry()
	ctx := context.Background()

	p := newTestProvider("org-1", "prov-1", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
	adapter := &mockProviderAdapter{id: "prov-1", caps: p.Capabilities, health: domain.ProviderHealthHealthy}
	reg.Register(p, adapter)

	repo := &mockRepo{}
	orch := newTestOrchratorHelper(reg, repo)

	result, err := orch.ExecuteCapability(ctx, "org-2", "actor-1", ExecuteOptions{
		Capability: domain.CapabilityAvailability,
		Operation:  "check_availability",
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		return domain.ProviderResult{Success: true}, nil
	})

	if err != domain.ErrNoProvidersAvailable {
		t.Errorf("expected ErrNoProvidersAvailable for wrong tenant, got %v", err)
	}
	if result != nil {
		t.Error("expected nil result")
	}
}

func newTestOrchratorHelper(reg *ProviderRegistry, repo domain.ProviderExecutionRepository) *ProviderOrchestrator {
	return newTestOrchestrator(reg, repo)
}

func TestRetryPolicy_BackoffCalculation(t *testing.T) {
	policy := domain.RetryPolicy{
		MaxAttempts: 5,
		BaseDelayMs: 100,
		MaxDelayMs:  5000,
	}

	orch := &ProviderOrchestrator{retryPolicy: policy}

	d1 := orch.calculateBackoff(0)
	d2 := orch.calculateBackoff(1)
	d3 := orch.calculateBackoff(2)
	d4 := orch.calculateBackoff(10)

	if d1 != 100*time.Millisecond {
		t.Errorf("expected 100ms for attempt 0, got %v", d1)
	}
	if d2 != 200*time.Millisecond {
		t.Errorf("expected 200ms for attempt 1, got %v", d2)
	}
	if d3 != 400*time.Millisecond {
		t.Errorf("expected 400ms for attempt 2, got %v", d3)
	}
	if d4 != 5000*time.Millisecond {
		t.Errorf("expected 5000ms (max) for attempt 10, got %v", d4)
	}
}
