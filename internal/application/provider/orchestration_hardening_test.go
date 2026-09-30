package provider

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/provider/providertest"
	"github.com/trezutil/trezutil/internal/domain"
)

// recordingRepo keeps every recorded execution.
type recordingRepo struct {
	mu    sync.Mutex
	execs []*domain.ProviderExecution
	fail  bool
}

func (r *recordingRepo) Create(ctx context.Context, e *domain.ProviderExecution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("database down")
	}
	r.execs = append(r.execs, e)
	return nil
}
func (r *recordingRepo) FindByID(context.Context, string, string) (*domain.ProviderExecution, error) {
	return nil, domain.ErrProviderNotFound
}
func (r *recordingRepo) FindByIdempotencyKey(context.Context, string, string) (*domain.ProviderExecution, error) {
	return nil, domain.ErrProviderNotFound
}
func (r *recordingRepo) FindByProviderID(context.Context, string, string, int, int) ([]*domain.ProviderExecution, error) {
	return nil, nil
}

// availabilityStack registers the emulators by priority (first = highest).
func availabilityStack(emulators ...*providertest.AvailabilityEmulator) (*ProviderOrchestrator, *recordingRepo) {
	reg := NewProviderRegistry()
	for i, e := range emulators {
		p := newTestProvider("org-1", e.ID, []domain.Capability{domain.CapabilityAvailability}, domain.ProviderStatusActive)
		p.Priority = 100 - i
		_ = reg.Register(p, e)
	}
	repo := &recordingRepo{}
	policy := domain.DefaultRetryPolicy()
	policy.BaseDelayMs, policy.MaxDelayMs = 1, 5
	return NewProviderOrchestrator(reg, repo, OrchestratorConfig{Logger: zerolog.Nop(), RetryPolicy: policy}), repo
}

var stayQuery = domain.AvailabilityQuery{TenantID: "org-1", ListingID: "ext-room-1", StartAt: 1791590400, EndAt: 1791590400 + 2*86400, Quantity: 1}

func TestOrchestratedAvailability_ExplicitAnswersOnly(t *testing.T) {
	cases := []struct {
		behaviour providertest.Behaviour
		state     domain.AvailabilityState
		available bool
		unknown   bool
		calls     int
		code      domain.ProviderErrorCode
	}{
		{providertest.Available, domain.AvailabilityAvailable, true, false, 1, ""},
		{providertest.Unavailable, domain.AvailabilityUnavailable, false, false, 1, ""},
		// A successful call that says nothing is UNKNOWN, never AVAILABLE, and is not retried.
		{providertest.NoState, domain.AvailabilityUnknown, false, true, 1, domain.ErrCodeInvalidResponse},
		{providertest.Malformed, domain.AvailabilityUnknown, false, true, 1, domain.ErrCodeInvalidResponse},
		// A provider error that is not transient stops at once.
		{providertest.Rejected, domain.AvailabilityUnknown, false, true, 1, domain.ErrCodeUnknown},
		// A connection error is retried, then UNKNOWN.
		{providertest.Refused, domain.AvailabilityUnknown, false, true, 3, domain.ErrCodeUnknown},
	}
	for _, tc := range cases {
		emu := &providertest.AvailabilityEmulator{ID: "hotels-a", Script: []providertest.Behaviour{tc.behaviour}}
		orch, repo := availabilityStack(emu)
		res, err := NewOrchestratedAvailabilityProvider(orch).CheckAvailability(context.Background(), stayQuery)
		if res.State != tc.state || res.Available != tc.available || errors.Is(err, domain.ErrAvailabilityUnknown) != tc.unknown || emu.Calls() != tc.calls {
			t.Errorf("%s: state %s available %v err %v calls %d", tc.behaviour, res.State, res.Available, err, emu.Calls())
		}
		if len(repo.execs) != tc.calls {
			t.Errorf("%s: %d attempts recorded, want %d", tc.behaviour, len(repo.execs), tc.calls)
		}
		if tc.code != "" && repo.execs[len(repo.execs)-1].ErrorCode != tc.code {
			t.Errorf("%s: recorded code %s, want %s", tc.behaviour, repo.execs[len(repo.execs)-1].ErrorCode, tc.code)
		}
	}
}

func TestOrchestratedAvailability_TooFewPlacesIsUnavailable(t *testing.T) {
	orch, _ := availabilityStack(&providertest.AvailabilityEmulator{ID: "hotels-a"}) // 4 left
	q := stayQuery
	q.Quantity = 5
	res, err := NewOrchestratedAvailabilityProvider(orch).CheckAvailability(context.Background(), q)
	if err != nil || res.Available || res.State != domain.AvailabilityUnavailable {
		t.Fatalf("5 rooms of 4: %+v %v", res, err)
	}
}

func TestOrchestratedAvailability_TimeoutFallsBackToNextProvider(t *testing.T) {
	slow := &providertest.AvailabilityEmulator{ID: "hotels-slow", Script: []providertest.Behaviour{providertest.Slow}}
	backup := &providertest.AvailabilityEmulator{ID: "hotels-backup"}
	orch, repo := availabilityStack(slow, backup)
	res, err := NewOrchestratedAvailabilityProvider(orch).WithTimeout(30*time.Millisecond).CheckAvailability(context.Background(), stayQuery)
	if err != nil || !res.Available || slow.Calls() != 1 || backup.Calls() != 1 {
		t.Fatalf("fallback: %+v %v (slow %d, backup %d)", res, err, slow.Calls(), backup.Calls())
	}
	if len(repo.execs) != 2 {
		t.Fatalf("attempts recorded = %d", len(repo.execs))
	}
	first, second := repo.execs[0], repo.execs[1]
	if first.ProviderID != "hotels-slow" || first.Status != domain.ExecutionStatusFailed || first.ErrorCode != domain.ErrCodeProviderTimeout ||
		first.RetryClass != domain.Retryable || first.Attempt != 1 {
		t.Errorf("first attempt = %+v", first)
	}
	if second.ProviderID != "hotels-backup" || second.Status != domain.ExecutionStatusSuccess || second.Attempt != 2 || second.RequestID != first.RequestID || first.RequestID == "" {
		t.Errorf("second attempt = %+v", second)
	}
	if first.TenantID != "org-1" || first.Capability != domain.CapabilityAvailability || first.Operation != "check_availability" {
		t.Errorf("execution scope = %+v", first)
	}
}

func TestOrchestratedAvailability_TimeoutEverywhereIsUnknown(t *testing.T) {
	slow := &providertest.AvailabilityEmulator{ID: "hotels-slow", Script: []providertest.Behaviour{providertest.Slow}}
	orch, repo := availabilityStack(slow)
	res, err := NewOrchestratedAvailabilityProvider(orch).WithTimeout(20*time.Millisecond).CheckAvailability(context.Background(), stayQuery)
	if !errors.Is(err, domain.ErrAvailabilityUnknown) || res.Available || res.State != domain.AvailabilityUnknown || res.Reason != string(domain.ErrCodeProviderTimeout) {
		t.Fatalf("timeout: %+v %v", res, err)
	}
	if slow.Calls() != 3 || len(repo.execs) != 3 {
		t.Fatalf("timeouts are retried: calls %d, recorded %d", slow.Calls(), len(repo.execs))
	}
}

func TestOrchestratedAvailability_NoProviderIsUnknown(t *testing.T) {
	orch, _ := availabilityStack()
	res, err := NewOrchestratedAvailabilityProvider(orch).CheckAvailability(context.Background(), stayQuery)
	if !errors.Is(err, domain.ErrAvailabilityUnknown) || res.Available {
		t.Fatalf("no provider: %+v %v", res, err)
	}
	q := stayQuery
	q.TenantID = ""
	if _, err := NewOrchestratedAvailabilityProvider(orch).CheckAvailability(context.Background(), q); !errors.Is(err, domain.ErrAvailabilityUnknown) {
		t.Fatalf("a query without tenant must not be answered: %v", err)
	}
}

func TestOrchestrator_IdempotencyKeyReachesEveryAttempt(t *testing.T) {
	emu := &providertest.AvailabilityEmulator{ID: "hotels-a", Script: []providertest.Behaviour{providertest.Refused, providertest.Refused, providertest.Available}}
	orch, repo := availabilityStack(emu)
	res, err := orch.ExecuteCapability(context.Background(), "org-1", "", ExecuteOptions{
		Capability: domain.CapabilityAvailability, Operation: "check_availability", IdempotencyKey: "quote-42",
	}, func(ctx context.Context, _ *domain.ProviderExecutionContext, a domain.ProviderAdapter) (domain.ProviderResult, error) {
		r, err := a.(domain.AvailabilityProviderAdapter).CheckAvailability(ctx, stayQuery)
		return domain.ProviderResult{Success: err == nil && r.State != ""}, err
	})
	if err != nil || !res.Success || res.Attempts != 3 {
		t.Fatalf("result %+v %v", res, err)
	}
	for _, k := range emu.IdempotencyKeys() {
		if k != "quote-42" {
			t.Fatalf("keys = %v", emu.IdempotencyKeys())
		}
	}
	for _, e := range repo.execs {
		if e.IdempotencyKey != "quote-42" {
			t.Fatalf("recorded key %q", e.IdempotencyKey)
		}
	}
}

func TestOrchestrator_SuccessComesFromTheProviderResult(t *testing.T) {
	orch, repo := availabilityStack(&providertest.AvailabilityEmulator{ID: "hotels-a"})
	calls := 0
	res, err := orch.ExecuteCapability(context.Background(), "org-1", "", ExecuteOptions{Capability: domain.CapabilityAvailability, Operation: "op"},
		func(context.Context, *domain.ProviderExecutionContext, domain.ProviderAdapter) (domain.ProviderResult, error) {
			calls++
			return domain.ProviderResult{}, nil // the call returned, but nothing was performed
		})
	if err != nil || res.Success || res.ErrorCode != domain.ErrCodeProviderRejected || calls != 1 {
		t.Fatalf("an empty answer is not a success: %+v %v (calls %d)", res, err, calls)
	}
	if repo.execs[0].Status != domain.ExecutionStatusFailed {
		t.Fatalf("recorded %s", repo.execs[0].Status)
	}
}

func TestOrchestrator_CapabilityValidationAndUnsupportedAdapters(t *testing.T) {
	orch, _ := availabilityStack(&providertest.AvailabilityEmulator{ID: "hotels-a"})
	ok := func(context.Context, *domain.ProviderExecutionContext, domain.ProviderAdapter) (domain.ProviderResult, error) {
		return domain.ProviderResult{Success: true}, nil
	}
	for name, opts := range map[string]ExecuteOptions{
		"no capability": {Operation: "op"},
		"no operation":  {Capability: domain.CapabilityAvailability},
	} {
		if _, err := orch.ExecuteCapability(context.Background(), "org-1", "", opts, ok); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := orch.ExecuteCapability(context.Background(), "org-1", "", ExecuteOptions{Capability: domain.CapabilityBooking, Operation: "book"}, ok); !errors.Is(err, domain.ErrNoProvidersAvailable) {
		t.Errorf("no provider declares BOOKING: %v", err)
	}
	res, err := orch.ExecuteCapability(context.Background(), "org-1", "", ExecuteOptions{Capability: domain.CapabilityAvailability, Operation: "op"},
		func(context.Context, *domain.ProviderExecutionContext, domain.ProviderAdapter) (domain.ProviderResult, error) {
			return domain.ProviderResult{}, domain.ErrUnsupportedCapability
		})
	if err == nil || res.ErrorCode != domain.ErrCodeUnsupportedCapability || res.Attempts != 1 {
		t.Errorf("unsupported capability: %+v %v", res, err)
	}
}

func TestOrchestrator_RecordingFailureNeverChangesTheOutcome(t *testing.T) {
	orch, repo := availabilityStack(&providertest.AvailabilityEmulator{ID: "hotels-a"})
	repo.fail = true
	res, err := NewOrchestratedAvailabilityProvider(orch).CheckAvailability(context.Background(), stayQuery)
	if err != nil || !res.Available {
		t.Fatalf("result must not depend on the execution store: %+v %v", res, err)
	}
}
