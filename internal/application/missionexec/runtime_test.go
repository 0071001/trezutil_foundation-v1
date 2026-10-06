package missionexec

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * The executor against the REAL Commerce Action Runtime (its parameter validation, idempotency store, usage check
 * and the real search handler), with only the catalogue search and the credit store faked. They show that credits,
 * authorization and idempotency stay the runtime's: the mission layer never touches them.
 */

// searchStub answers the catalogue search and records what the real search handler asked it.
type searchStub struct {
	mu      sync.Mutex
	queries []domain.DiscoveryQuery
	page    domain.SearchResultPage
	err     error
}

func (s *searchStub) ExecuteSearch(_ context.Context, _ string, q domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, q)
	return s.page, s.err
}
func (s *searchStub) Suggest(context.Context, string, domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{}, nil
}
func (s *searchStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queries)
}

// creditStore is the credit service the runtime talks to; it counts every call so a test can say who touched credits.
type creditStore struct {
	mu                           sync.Mutex
	cost, balance                int64
	reserved, consumed, released int
	open                         map[string]int64
	checks                       int
}

func newCreditStore(cost, balance int64) *creditStore {
	return &creditStore{cost: cost, balance: balance, open: map[string]int64{}}
}

func (c *creditStore) GetBalance(context.Context, string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{AccountID: "acc", Granted: c.balance}, nil
}
func (c *creditStore) ResolvePolicy(_ context.Context, a domain.CommerceActionType, _ domain.Channel, _ domain.ActorType) (*domain.UsagePolicy, error) {
	if a == domain.ActionSearchListings && c.cost > 0 {
		return &domain.UsagePolicy{ActionType: a, Cost: c.cost, Active: true}, nil
	}
	return nil, domain.ErrUsagePolicyNotFound
}
func (c *creditStore) Check(_ context.Context, _ string, cost int64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	return c.balance >= cost, nil
}
func (c *creditStore) Reserve(_ context.Context, _ string, _ domain.CommerceActionType, cost int64, actionID, _ string) (*domain.UsageReservation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reserved++
	c.balance -= cost
	id := fmt.Sprintf("res-%s", actionID)
	c.open[id] = cost
	return &domain.UsageReservation{ReservationID: id, Amount: cost, IdempotencyKey: actionID}, nil
}
func (c *creditStore) Consume(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.open[id]; !ok {
		return domain.ErrUsageReservationNotFound
	}
	delete(c.open, id)
	c.consumed++
	return nil
}
func (c *creditStore) Release(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cost, ok := c.open[id]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	delete(c.open, id)
	c.balance += cost
	c.released++
	return nil
}
func (c *creditStore) Grant(context.Context, string, int64, string) error { return nil }
func (c *creditStore) FindOrCreateCreditAccountByOwner(_ context.Context, _ domain.ActorType, ownerID, _, _, _ string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{AccountID: "acc-" + ownerID, Granted: c.balance}, nil
}

// realEnv wires the executor to a real runtime.
func realEnv(t *testing.T, search *searchStub, credits *creditStore) *env {
	t.Helper()
	registry := commerce.NewHandlerRegistry()
	registry.Register(commerce.NewSearchHandler(search))
	rt := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(), ParameterValidator: commerce.NewDefaultParameterValidator(),
		UsageService: credits,
	})
	e := newEnv(t)
	e.ex = New(e.store, e.store, e.store, e.store, rt, zerolog.Nop()).WithClock(func() time.Time { return time.Unix(e.clock, 0) })
	return e
}

func TestRealRuntime_SearchRunsThroughTheExistingHandler(t *testing.T) {
	search := &searchStub{page: domain.SearchResultPage{
		Items: []domain.SearchResult{{ListingID: "l1", Title: "Hotel Fleuve"}, {ListingID: "l2", Title: "Hotel Gombe"}}, Total: 2,
	}}
	e := realEnv(t, search, newCreditStore(0, 0))
	res, err := e.run(alice, e.stay, "k-1")
	if err != nil {
		t.Fatal(err)
	}
	if res.Execution.Status != domain.ExecutionCompleted || res.Execution.RuntimeStatus != "SUCCESS" {
		t.Fatalf("execution %+v", res.Execution)
	}
	if p := res.Execution.ResultPayload; p["total"] != 2 || len(p["listing_ids"].([]string)) != 2 {
		t.Fatalf("payload %v", p)
	}
	// The real handler received what the step asked: the vertical and the place, validated by the runtime.
	if search.count() != 1 {
		t.Fatalf("the catalogue is searched once, got %d", search.count())
	}
	if q := search.queries[0]; q.Vertical != domain.VerticalHotel || q.Place != "Kinshasa" {
		t.Fatalf("query %+v", q)
	}
}

func TestRealRuntime_AHandlerErrorIsAStepFailureWithTheRuntimesCode(t *testing.T) {
	// The handler fails (the search backend is down): the runtime answers ACTION_FAILED, and the step keeps that
	// code, not the backend's message.
	search := &searchStub{err: errors.New("opensearch: connection refused (10.1.2.3:9200)")}
	e := realEnv(t, search, newCreditStore(0, 0))
	res, err := e.run(alice, e.stay, "")
	if err != nil || res.Execution.Status != domain.ExecutionFailed || res.Execution.ErrorCode != "ACTION_FAILED" || res.Step.Status != domain.StepFailed {
		t.Fatalf("handler error: %+v %v", res, err)
	}
	if res.Execution.RuntimeStatus != "FAILED" {
		t.Fatalf("runtime status %q", res.Execution.RuntimeStatus)
	}
}

// Credits belong to the runtime. A metered search reserves once and consumes once; the executor adds nothing.
func TestRealRuntime_CreditsAreTheRuntimes_ReserveAndConsumeOncePerSuccess(t *testing.T) {
	credits := newCreditStore(2, 10)
	e := realEnv(t, &searchStub{page: domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}}, credits)
	if _, err := e.run(alice, e.stay, "k-1"); err != nil {
		t.Fatal(err)
	}
	if credits.reserved != 1 || credits.consumed != 1 || credits.released != 0 || credits.balance != 8 || len(credits.open) != 0 {
		t.Fatalf("reserved=%d consumed=%d released=%d balance=%d", credits.reserved, credits.consumed, credits.released, credits.balance)
	}
	// A replay of the same request is answered by the mission record: the runtime is not asked, nothing is charged.
	if _, err := e.run(alice, e.stay, "k-1"); err != nil {
		t.Fatal(err)
	}
	if credits.reserved != 1 || credits.consumed != 1 || credits.balance != 8 {
		t.Fatalf("a replay must not charge again: reserved=%d consumed=%d balance=%d", credits.reserved, credits.consumed, credits.balance)
	}
	// A completed step is not run again, so it is never charged again.
	if _, err := e.run(alice, e.stay, "k-2"); !errors.Is(err, domain.ErrStepNotRunnable) || credits.reserved != 1 {
		t.Fatalf("completed step: %v reserved=%d", err, credits.reserved)
	}
}

func TestRealRuntime_AFailedRunReleasesTheReservationAndARetryChargesOnlyItsSuccess(t *testing.T) {
	credits := newCreditStore(2, 10)
	search := &searchStub{err: errors.New("boom")}
	e := realEnv(t, search, credits)
	failedRun, err := e.run(alice, e.stay, "k-1")
	if err != nil || failedRun.Execution.Status != domain.ExecutionFailed {
		t.Fatalf("first run: %+v %v", failedRun, err)
	}
	if credits.reserved != 1 || credits.released != 1 || credits.consumed != 0 || credits.balance != 10 {
		t.Fatalf("a failed action keeps the credits: reserved=%d released=%d consumed=%d balance=%d", credits.reserved, credits.released, credits.consumed, credits.balance)
	}
	// Retry once the search works: one more reservation, consumed once. Net: charged for the success only.
	search.mu.Lock()
	search.err = nil
	search.page = domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}
	search.mu.Unlock()
	retry, err := e.run(alice, e.stay, "k-2")
	if err != nil || retry.Execution.Status != domain.ExecutionCompleted {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	if credits.reserved != 2 || credits.released != 1 || credits.consumed != 1 || credits.balance != 8 || len(credits.open) != 0 {
		t.Fatalf("no double charge: reserved=%d released=%d consumed=%d balance=%d", credits.reserved, credits.released, credits.consumed, credits.balance)
	}
}

func TestRealRuntime_InsufficientCreditsIsTheRuntimesAnswerAndTheStepFails(t *testing.T) {
	credits := newCreditStore(5, 3)
	search := &searchStub{page: domain.SearchResultPage{Total: 1}}
	e := realEnv(t, search, credits)
	res, err := e.run(alice, e.stay, "")
	if err != nil || res.Execution.Status != domain.ExecutionFailed || res.Execution.ErrorCode != "INSUFFICIENT_CREDITS" || res.Step.Status != domain.StepFailed {
		t.Fatalf("insufficient credits: %+v %v", res, err)
	}
	if search.count() != 0 || credits.reserved != 0 || credits.consumed != 0 {
		t.Fatalf("the runtime refused before searching or reserving: searches=%d reserved=%d", search.count(), credits.reserved)
	}
	if credits.checks != 1 {
		t.Fatalf("the runtime checked the balance once, got %d", credits.checks)
	}
	// Credits arrive: the failed step is retried and completes.
	credits.balance = 10
	retry, err := e.run(alice, e.stay, "")
	if err != nil || retry.Execution.Status != domain.ExecutionCompleted || credits.consumed != 1 {
		t.Fatalf("retry after credits: %+v %v", retry, err)
	}
}

func TestRealRuntime_ARepeatedKeySearchesNothing(t *testing.T) {
	// The same step with the same key is the same execution: the mission record answers, the runtime is not asked.
	search := &searchStub{page: domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}}
	e := realEnv(t, search, newCreditStore(0, 0))
	if _, err := e.run(alice, e.stay, "k-1"); err != nil {
		t.Fatal(err)
	}
	if search.count() != 1 {
		t.Fatalf("searches: %d", search.count())
	}
	if _, err := e.run(alice, e.stay, "k-1"); err != nil || search.count() != 1 {
		t.Fatalf("a repeated key searches nothing: %v %d", err, search.count())
	}
}

func TestRealRuntime_OnlyTheOwnerRunsAndNothingLeaksToTheRuntime(t *testing.T) {
	search := &searchStub{page: domain.SearchResultPage{Total: 1}}
	credits := newCreditStore(2, 10)
	e := realEnv(t, search, credits)
	if _, err := e.run(bob, e.stay, ""); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("User B = %v", err)
	}
	if search.count() != 0 || credits.reserved != 0 || credits.checks != 0 {
		t.Fatal("a refused request reaches neither the search nor the credits")
	}
}
