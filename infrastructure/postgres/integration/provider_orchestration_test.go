package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/application/provider/providertest"
	"github.com/trezutil/trezutil/internal/domain"
)

// Journey H6 — external availability failures, on real PostgreSQL. No
// external provider exists; deterministic emulators stand behind real
// provider rows, the real orchestrator and the real execution store.
func TestProviderOrchestration_AvailabilityFailuresAreSafeAndRecorded(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()

	const orgA, orgB = "0bc1e500-0000-4000-8000-00000000000a", "0bc1e500-0000-4000-8000-00000000000b"
	const slowID, backupID, brokenID = "0bc1e500-0000-4000-8000-0000000000e1", "0bc1e500-0000-4000-8000-0000000000e2", "0bc1e500-0000-4000-8000-0000000000e3"
	createTestOrganization(t, db, ctx, orgA)
	createTestOrganization(t, db, ctx, orgB)

	providers := postgres.NewProviderRepository(db.DB)
	executions := postgres.NewProviderExecutionRepository(db.DB)
	newProvider := func(id, org, code string, priority int) *domain.IntegrationProvider {
		p := domain.NewProvider(org, code, code, "availability_emulator", []domain.Capability{domain.CapabilityAvailability}, domain.ProviderConfig{}, "", priority)
		p.ID, p.Status, p.HealthStatus = id, domain.ProviderStatusActive, domain.ProviderHealthHealthy
		if err := providers.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	policy := domain.DefaultRetryPolicy()
	policy.BaseDelayMs, policy.MaxDelayMs = 1, 5
	query := domain.AvailabilityQuery{TenantID: orgA, ListingID: "ext-room-1", StartAt: 1791590400, EndAt: 1791590400 + 86400, Quantity: 1}

	// 1. Timeout on the primary provider falls back to the next one; both attempts are recorded.
	slow := &providertest.AvailabilityEmulator{ID: slowID, Script: []providertest.Behaviour{providertest.Slow}}
	backup := &providertest.AvailabilityEmulator{ID: backupID, Script: []providertest.Behaviour{providertest.Unavailable}}
	reg := provider.NewProviderRegistry()
	_ = reg.Register(newProvider(slowID, orgA, "slow", 10), slow)
	_ = reg.Register(newProvider(backupID, orgA, "backup", 5), backup)
	orch := provider.NewProviderOrchestrator(reg, executions, provider.OrchestratorConfig{Logger: zerolog.Nop(), RetryPolicy: policy})
	res, err := provider.NewOrchestratedAvailabilityProvider(orch).WithTimeout(50*time.Millisecond).CheckAvailability(ctx, query)
	if err != nil || res.Available || res.State != domain.AvailabilityUnavailable {
		t.Fatalf("timeout then unavailable: %+v %v", res, err)
	}
	slowRuns, _ := executions.FindByProviderID(ctx, orgA, slowID, 0, 10)
	backupRuns, _ := executions.FindByProviderID(ctx, orgA, backupID, 0, 10)
	if len(slowRuns) != 1 || slowRuns[0].ErrorCode != domain.ErrCodeProviderTimeout || slowRuns[0].RetryClass != domain.Retryable ||
		len(backupRuns) != 1 || backupRuns[0].Status != domain.ExecutionStatusSuccess || backupRuns[0].Attempt != 2 {
		t.Fatalf("recorded attempts: slow %+v backup %+v", slowRuns, backupRuns)
	}
	attempts, err := executions.FindByRequestID(ctx, orgA, slowRuns[0].RequestID)
	if err != nil || len(attempts) != 2 || attempts[0].ProviderID != slowID || attempts[1].ProviderID != backupID {
		t.Fatalf("one request, two attempts: %+v %v", attempts, err)
	}
	if attempts[0].CompletedAt < attempts[0].StartedAt || attempts[0].ActorID != "" || attempts[0].Capability != domain.CapabilityAvailability {
		t.Fatalf("attempt record = %+v", attempts[0])
	}

	// 2. A malformed answer is UNKNOWN, never AVAILABLE, and is not retried.
	broken := &providertest.AvailabilityEmulator{ID: brokenID, Script: []providertest.Behaviour{providertest.Malformed}}
	reg2 := provider.NewProviderRegistry()
	_ = reg2.Register(newProvider(brokenID, orgA, "broken", 1), broken)
	orch2 := provider.NewProviderOrchestrator(reg2, executions, provider.OrchestratorConfig{Logger: zerolog.Nop(), RetryPolicy: policy})
	res, err = provider.NewOrchestratedAvailabilityProvider(orch2).CheckAvailability(ctx, query)
	if !errors.Is(err, domain.ErrAvailabilityUnknown) || res.Available || res.State != domain.AvailabilityUnknown || broken.Calls() != 1 {
		t.Fatalf("malformed: %+v %v", res, err)
	}
	brokenRuns, _ := executions.FindByProviderID(ctx, orgA, brokenID, 0, 10)
	if len(brokenRuns) != 1 || brokenRuns[0].ErrorCode != domain.ErrCodeInvalidResponse || brokenRuns[0].Status != domain.ExecutionStatusFailed {
		t.Fatalf("malformed attempt = %+v", brokenRuns)
	}

	// 3. Executions are organization-scoped: another organization sees none,
	// even with the provider and execution IDs in hand.
	if runs, _ := executions.FindByProviderID(ctx, orgB, slowID, 0, 10); len(runs) != 0 {
		t.Fatalf("org B sees org A's executions: %+v", runs)
	}
	if _, err := executions.FindByID(ctx, orgB, slowRuns[0].ID); !errors.Is(err, domain.ErrProviderNotFound) {
		t.Fatalf("org B reads an execution by ID: %v", err)
	}
	if runs, _ := executions.FindByRequestID(ctx, orgB, slowRuns[0].RequestID); len(runs) != 0 {
		t.Fatalf("org B reads by request ID: %+v", runs)
	}
	// And org B's own queries never reach org A's providers.
	res, err = provider.NewOrchestratedAvailabilityProvider(orch).CheckAvailability(ctx, domain.AvailabilityQuery{TenantID: orgB, ListingID: "ext-room-1", StartAt: query.StartAt, EndAt: query.EndAt})
	if !errors.Is(err, domain.ErrAvailabilityUnknown) || res.Available {
		t.Fatalf("org B used org A's provider: %+v %v", res, err)
	}
}
