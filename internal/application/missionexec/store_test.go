package missionexec

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// memStore is an in-memory mission store with the semantics of the Postgres one: a claim is atomic (one mutex
// stands for the row lock), an attempt in progress blocks another, a key is one attempt.
type memStore struct {
	domain.MissionRepository     // only Get is used
	domain.MissionPlanRepository // only CurrentPlan is used

	mu       sync.Mutex
	missions map[string]*domain.Mission
	steps    map[string]*domain.MissionStep
	execs    []*domain.MissionStepExecution
	current  map[string]int // current plan version by mission
	deps     []domain.StepDependency
	locks    map[string]memLock
}

type memLock struct {
	token   string
	expires time.Time
}

func newMemStore() *memStore {
	return &memStore{missions: map[string]*domain.Mission{}, steps: map[string]*domain.MissionStep{}, current: map[string]int{}, locks: map[string]memLock{}}
}

func (s *memStore) Get(_ context.Context, id string) (*domain.Mission, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.missions[id]; ok {
		c := *m
		return &c, nil
	}
	return nil, domain.ErrMissionNotFound
}

func (s *memStore) CurrentPlan(_ context.Context, missionID string) (*domain.MissionPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.current[missionID]
	if !ok {
		return nil, domain.ErrMissionPlanNotFound
	}
	plan := &domain.MissionPlan{MissionID: missionID, Version: v, Status: domain.MissionPlanCurrent}
	in := map[string]bool{}
	for _, st := range s.steps {
		if st.MissionID == missionID && st.PlanVersion == v {
			plan.Steps = append(plan.Steps, *st)
			in[st.ID] = true
		}
	}
	sort.Slice(plan.Steps, func(i, j int) bool { return plan.Steps[i].Position < plan.Steps[j].Position })
	for _, d := range s.deps {
		if in[d.StepID] && in[d.DependsOnID] {
			plan.Dependencies = append(plan.Dependencies, d)
		}
	}
	return plan, nil
}

func (s *memStore) PlanDependencies(ctx context.Context, missionID string, version int) ([]domain.StepDependency, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []domain.StepDependency{}
	for _, d := range s.deps {
		if st, ok := s.steps[d.StepID]; ok && st.MissionID == missionID && st.PlanVersion == version {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *memStore) Upstream(_ context.Context, stepID string) ([]domain.UpstreamStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.UpstreamStep
	for _, d := range s.deps {
		if d.StepID != stepID {
			continue
		}
		up := domain.UpstreamStep{Step: *s.steps[d.DependsOnID]}
		if l := s.latest(d.DependsOnID); l != nil {
			c := *l
			up.Execution = &c
		}
		out = append(out, up)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Step.Position < out[j].Step.Position })
	return out, nil
}

func (s *memStore) GetStep(_ context.Context, missionID, stepID string) (*domain.MissionStep, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.steps[stepID]; ok && st.MissionID == missionID {
		c := *st
		return &c, nil
	}
	return nil, domain.ErrMissionStepNotFound
}

func (s *memStore) latest(stepID string) *domain.MissionStepExecution {
	var out *domain.MissionStepExecution
	for _, e := range s.execs {
		if e.StepID == stepID && (out == nil || e.Attempt > out.Attempt) {
			out = e
		}
	}
	return out
}

func (s *memStore) ExecutionByKey(_ context.Context, stepID, key string) (*domain.MissionStepExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.execs {
		if e.StepID == stepID && e.IdempotencyKey == key {
			c := *e
			return &c, nil
		}
	}
	return nil, domain.ErrMissionExecutionNotFound
}

func (s *memStore) BeginExecution(_ context.Context, in domain.BeginExecutionInput) (*domain.MissionStepExecution, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	step, ok := s.steps[in.StepID]
	if !ok || step.MissionID != in.MissionID {
		return nil, false, domain.ErrMissionStepNotFound
	}
	if in.IdempotencyKey != "" {
		for _, e := range s.execs {
			if e.StepID == in.StepID && e.IdempotencyKey == in.IdempotencyKey {
				c := *e
				return &c, true, nil
			}
		}
	}
	latest := s.latest(in.StepID)
	abandon, err := domain.ExecutionClaim(step, latest, in.Now, in.StaleAfter)
	if err != nil {
		return nil, false, err
	}
	if abandon && latest != nil && latest.Status == domain.ExecutionInProgress {
		latest.Status, latest.ErrorCode, latest.CompletedAt = domain.ExecutionFailed, domain.ErrorCodeAbandoned, in.Now
	}
	attempt := 1
	if latest != nil {
		attempt = latest.Attempt + 1
	}
	e := &domain.MissionStepExecution{ID: domain.NewID(), StepID: in.StepID, MissionID: in.MissionID, Attempt: attempt, Status: domain.ExecutionInProgress,
		ActionType: in.ActionType, IdempotencyKey: in.IdempotencyKey, ResultPayload: map[string]interface{}{}, StartedAt: in.Now, CreatedAt: in.Now}
	s.execs = append(s.execs, e)
	step.Status, step.ActionRef, step.ResultRef = domain.StepInProgress, e.ID, ""
	c := *e
	return &c, false, nil
}

func (s *memStore) FinishExecution(_ context.Context, exec *domain.MissionStepExecution, now int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.execs {
		if e.ID != exec.ID {
			continue
		}
		if e.Status != domain.ExecutionInProgress {
			return domain.ErrExecutionSuperseded
		}
		e.Status, e.ResultReference, e.ResultPayload, e.RuntimeStatus, e.ErrorCode, e.CompletedAt = exec.Status, exec.ResultReference, exec.ResultPayload, exec.RuntimeStatus, exec.ErrorCode, now
		step := s.steps[e.StepID]
		if exec.Status == domain.ExecutionCompleted {
			step.Status, step.ResultRef = domain.StepCompleted, exec.ResultReference
		} else {
			step.Status, step.ResultRef = domain.StepFailed, ""
		}
		exec.CompletedAt = now
		return nil
	}
	return domain.ErrExecutionSuperseded
}

func (s *memStore) LatestExecutions(_ context.Context, missionID string) (map[string]*domain.MissionStepExecution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]*domain.MissionStepExecution{}
	for _, e := range s.execs {
		if e.MissionID == missionID && (out[e.StepID] == nil || e.Attempt > out[e.StepID].Attempt) {
			c := *e
			out[e.StepID] = &c
		}
	}
	return out, nil
}

func (s *memStore) executions(stepID string) []domain.MissionStepExecution {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []domain.MissionStepExecution
	for _, e := range s.execs {
		if e.StepID == stepID {
			out = append(out, *e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Attempt < out[j].Attempt })
	return out
}

func (s *memStore) step(id string) domain.MissionStep {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.steps[id]
}

// fakeRuntime is a scripted runtime that counts what it is asked.
type fakeRuntime struct {
	mu        sync.Mutex
	do        func(ctx context.Context, a *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error)
	unsupport map[domain.CommerceActionType]bool
	calls     []domain.CommerceAction
	actors    []domain.CommerceActor
}

func (r *fakeRuntime) Execute(ctx context.Context, a *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, *a)
	r.actors = append(r.actors, actor)
	do := r.do
	r.mu.Unlock()
	if do == nil {
		return searchSuccess(3), nil
	}
	return do(ctx, a, actor)
}

func (r *fakeRuntime) IsSupported(t domain.CommerceActionType) bool { return !r.unsupport[t] }

func (r *fakeRuntime) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

// searchSuccess is what the real search handler answers through the runtime: data["data"] holds the page.
func searchSuccess(n int) *commerce.ExecuteResult {
	items := make([]domain.SearchResult, 0, n)
	for i := 0; i < n; i++ {
		items = append(items, domain.SearchResult{ListingID: "listing-" + string(rune('a'+i)), Title: "Hotel " + string(rune('A'+i))})
	}
	return &commerce.ExecuteResult{
		Status: domain.ActionResultSuccess, MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{"data": map[string]interface{}{"items": items, "total": n, "has_next": false}},
	}
}

func failed(code string) *commerce.ExecuteResult {
	return &commerce.ExecuteResult{Status: domain.ActionResultFailed, MessageKey: "ACTION_FAILED", Errors: []domain.ActionError{domain.NewActionError(code, "internal detail that must not be kept: db password=hunter2", "")}}
}

// env is a mission with steps, an executor and its store.
type env struct {
	t       *testing.T
	store   *memStore
	rt      *fakeRuntime
	ex      *Executor
	mission *domain.Mission
	// steps by role
	stay, activities, availability, hand *domain.MissionStep
	clock                                int64
}

const (
	alice = "11111111-1111-4111-8111-111111111111"
	bob   = "22222222-2222-4222-8222-222222222222"
)

var base = time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

func newEnv(t *testing.T) *env {
	t.Helper()
	e := &env{t: t, store: newMemStore(), rt: &fakeRuntime{unsupport: map[domain.CommerceActionType]bool{}}, clock: base.Unix()}
	e.ex = New(e.store, e.store, e.store, e.store, e.rt, zerolog.Nop()).WithClock(func() time.Time { return time.Unix(e.clock, 0) })
	e.mission = e.addMission(alice, domain.MissionTypeTravel, "Kinshasa")
	e.stay = e.addStep(e.mission, "FIND_ACCOMMODATION", domain.ActionSearchListings, 1, 1)
	e.activities = e.addStep(e.mission, "FIND_ACTIVITIES", domain.ActionSearchListings, 1, 2)
	e.availability = e.addStep(e.mission, "CHECK_AVAILABILITY", domain.ActionCheckAvailability, 1, 3)
	e.hand = e.addStep(e.mission, "OTHER", "", 0, 1)
	e.store.current[e.mission.ID] = 1
	e.store.deps = append(e.store.deps, domain.StepDependency{StepID: e.availability.ID, DependsOnID: e.stay.ID}) // as the templates do
	return e
}

func (e *env) addMission(owner string, typ domain.MissionType, location string) *domain.Mission {
	m, err := domain.NewMission(owner, typ, "Mission", "Un voyage à Goma", base)
	if err != nil {
		e.t.Fatal(err)
	}
	m.Location = location
	e.store.missions[m.ID] = m
	return m
}

func (e *env) addStep(m *domain.Mission, typ string, action domain.CommerceActionType, planVersion, position int) *domain.MissionStep {
	s := &domain.MissionStep{ID: domain.NewID(), MissionID: m.ID, Type: typ, Title: "Step " + typ, Status: domain.StepPending, Position: position,
		ActionType: action, PlanVersion: planVersion, CreatedAt: base.Unix(), UpdatedAt: base.Unix()}
	e.store.steps[s.ID] = s
	return s
}

func (e *env) run(user string, step *domain.MissionStep, key string) (*domain.StepExecutionResult, error) {
	return e.ex.Execute(context.Background(), user, e.mission.ID, step.ID, key)
}

// The run lock and the status change, with the semantics of the Postgres ones (one mutex stands for the atomic statement).
func (s *memStore) AcquireRun(_ context.Context, missionID, token string, now time.Time, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.locks[missionID]; ok && l.expires.After(now) {
		return false, nil
	}
	s.locks[missionID] = memLock{token: token, expires: now.Add(ttl)}
	return true, nil
}

func (s *memStore) ExtendRun(_ context.Context, missionID, token string, now time.Time, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.locks[missionID]; !ok || l.token != token || !l.expires.After(now) {
		return domain.ErrMissionBusy
	}
	s.locks[missionID] = memLock{token: token, expires: now.Add(ttl)}
	return nil
}

func (s *memStore) ReleaseRun(_ context.Context, missionID, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if l, ok := s.locks[missionID]; ok && l.token == token {
		delete(s.locks, missionID)
	}
	return nil
}

func (s *memStore) RunInProgress(_ context.Context, missionID string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	l, ok := s.locks[missionID]
	return ok && l.expires.After(now), nil
}

func (s *memStore) TransitionMission(_ context.Context, missionID string, from, to domain.MissionStatus, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.missions[missionID]; ok && m.Status == from {
		m.Status = to
		return true, nil
	}
	return false, nil
}

func (s *memStore) missionStatus(id string) domain.MissionStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.missions[id].Status
}
