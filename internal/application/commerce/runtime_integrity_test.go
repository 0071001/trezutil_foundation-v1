package commerce

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

/* Credits and idempotency when something breaks: no double effect, no free action, nothing run unrecorded. */

type countingHandler struct {
	calls int32
	err   error
	wait  time.Duration
	keys  []string
	mu    sync.Mutex
}

func (h *countingHandler) ActionType() domain.CommerceActionType { return domain.ActionCancelOrder }

func (h *countingHandler) Handle(ctx context.Context, _ domain.CommerceActor, _ map[string]interface{}) (interface{}, error) {
	atomic.AddInt32(&h.calls, 1)
	h.mu.Lock()
	h.keys = append(h.keys, domain.EffectKey(ctx))
	h.mu.Unlock()
	time.Sleep(h.wait)
	if h.err != nil {
		return nil, h.err
	}
	return map[string]interface{}{"done": true}, nil
}

type brokenJournal struct{ IdempotencyStore }

func (brokenJournal) Claim(context.Context, string, string) (Claim, error) {
	return Claim{}, errors.New("database unavailable")
}

func integrityRuntime(h Handler, usage domain.UsageService, journal IdempotencyStore) *CommerceActionRuntime {
	registry := NewHandlerRegistry()
	registry.Register(h)
	return NewCommerceActionRuntime(RuntimeConfig{Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: journal, UsageService: usage})
}

func pricedUsage(cost int64) *fakeUsageService {
	u := newFakeUsageService()
	u.policies[domain.ActionCancelOrder] = domain.UsagePolicy{ActionType: domain.ActionCancelOrder, Cost: cost, Active: true}
	return u
}

var buyer = domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)

func request(key, order string) *domain.CommerceAction {
	a := domain.NewCommerceAction(domain.ActionCancelOrder, buyer, "conv-1")
	a.IdempotencyKey = key
	a.ConfirmationState = domain.ConfirmationApproved
	a.Parameters["order_id"] = order
	return a
}

func run(t *testing.T, rt *CommerceActionRuntime, a *domain.CommerceAction) *ExecuteResult {
	t.Helper()
	res, err := rt.Execute(context.Background(), a, buyer)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestAnUnreadablePriceRunsNothing(t *testing.T) {
	h, usage := &countingHandler{}, pricedUsage(2)
	usage.resolveErr = errors.New("database unavailable")
	res := run(t, integrityRuntime(h, usage, NewInMemoryIdempotencyStore()), request("k", "o1"))
	if res.Status != domain.ActionResultFailed || res.MessageKey != "USAGE_POLICY_FAILED" || h.calls != 0 {
		t.Fatalf("a priced action ran for free: %s %s (%d calls)", res.Status, res.MessageKey, h.calls)
	}
}

func TestAnActionThatCannotBeRecordedDoesNotRun(t *testing.T) {
	h := &countingHandler{}
	res := run(t, integrityRuntime(h, nil, brokenJournal{NewInMemoryIdempotencyStore()}), request("k", "o1"))
	if res.MessageKey != "IDEMPOTENCY_UNAVAILABLE" || h.calls != 0 {
		t.Fatalf("ran without a record: %s (%d calls)", res.MessageKey, h.calls)
	}
	usage := pricedUsage(2)
	usage.markErr = errors.New("database unavailable")
	h = &countingHandler{}
	res = run(t, integrityRuntime(h, usage, NewInMemoryIdempotencyStore()), request("k", "o1"))
	if res.Status != domain.ActionResultFailed || h.calls != 0 || len(usage.released) != 1 {
		t.Fatalf("ran although the reservation could not be marked: %s (%d calls, %d released)", res.Status, h.calls, len(usage.released))
	}
}

func TestAChargeThatFailsIsOwedNotForgiven(t *testing.T) {
	h, usage := &countingHandler{}, pricedUsage(2)
	usage.consumeErr = errors.New("database unavailable")
	res := run(t, integrityRuntime(h, usage, NewInMemoryIdempotencyStore()), request("k", "o1"))
	if res.Status != domain.ActionResultSuccess || len(usage.released) != 0 || len(usage.marks) != 1 {
		t.Fatalf("%s, released %d, marks %v", res.Status, len(usage.released), usage.marks)
	}
	for _, state := range usage.marks {
		if state != domain.UsageActionSucceeded {
			t.Fatalf("the sweep would not know the charge is owed: %s", state)
		}
	}
}

func TestAFailedActionIsReleasedAndAnUnknownOneIsHeld(t *testing.T) {
	h, usage := &countingHandler{err: errors.New("order already shipped")}, pricedUsage(2)
	run(t, integrityRuntime(h, usage, NewInMemoryIdempotencyStore()), request("k", "o1"))
	if len(usage.released) != 1 || len(usage.consumed) != 0 {
		t.Fatalf("failure: released %d consumed %d", len(usage.released), len(usage.consumed))
	}
	h, usage = &countingHandler{err: context.DeadlineExceeded}, pricedUsage(2)
	run(t, integrityRuntime(h, usage, NewInMemoryIdempotencyStore()), request("k", "o1"))
	if len(usage.released) != 0 || len(usage.consumed) != 0 {
		t.Fatalf("a timeout may have done it: released %d consumed %d", len(usage.released), len(usage.consumed))
	}
	for _, state := range usage.marks {
		if state != domain.UsageActionRunning {
			t.Fatalf("state %s", state)
		}
	}
}

func TestTheSameRequestRunsOnceWhateverArrivesTogether(t *testing.T) {
	h := &countingHandler{wait: 30 * time.Millisecond}
	rt := integrityRuntime(h, pricedUsage(1), NewInMemoryIdempotencyStore())
	var wg sync.WaitGroup
	var done, waiting int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := rt.Execute(context.Background(), request("k", "o1"), buyer)
			switch {
			case err != nil:
			case res.Status == domain.ActionResultSuccess:
				atomic.AddInt32(&done, 1)
			case res.MessageKey == "ACTION_IN_PROGRESS":
				atomic.AddInt32(&waiting, 1)
			}
		}()
	}
	wg.Wait()
	if h.calls != 1 || done+waiting != 20 || done < 1 {
		t.Fatalf("%d executions, %d done, %d told to wait", h.calls, done, waiting)
	}
	if res := run(t, rt, request("k", "o1")); res.MessageKey != "ACTION_IDEMPOTENT_CACHE_HIT" || h.calls != 1 {
		t.Fatalf("replay: %s", res.MessageKey)
	}
}

func TestAFailureCanBeTriedAgainWithTheSameEffectKey(t *testing.T) {
	h := &countingHandler{err: errors.New("temporary")}
	rt := integrityRuntime(h, nil, NewInMemoryIdempotencyStore())
	run(t, rt, request("k", "o1"))
	h.err = nil
	if res := run(t, rt, request("k", "o1")); res.Status != domain.ActionResultSuccess || h.calls != 2 {
		t.Fatalf("retry: %s after %d calls", res.Status, h.calls)
	}
	if h.keys[0] == "" || h.keys[0] != h.keys[1] {
		t.Fatalf("effect keys %q", h.keys)
	}
}

func TestAKeyCannotStandForAnotherRequestWithAnEffect(t *testing.T) {
	h := &countingHandler{}
	rt := integrityRuntime(h, nil, NewInMemoryIdempotencyStore())
	run(t, rt, request("k", "o1"))
	if res := run(t, rt, request("k", "o2")); res.MessageKey != "IDEMPOTENCY_CONFLICT" || h.calls != 1 {
		t.Fatalf("%s after %d calls", res.MessageKey, h.calls)
	}
}
