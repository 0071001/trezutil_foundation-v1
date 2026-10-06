package missionexec

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/missionorch"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Phase 4 end to end, without a database: the real orchestrator, the real executor and the in-memory store with the
 * semantics of the Postgres one (the PostgreSQL behaviour itself is tested in infrastructure/postgres/integration).
 *
 * The mission is a trip with four steps:
 *
 *	T  find transport       (search)
 *	S  find accommodation   (search)
 *	L  find local transport (search)
 *	C  check availability   (waits for S, takes its listing)
 */

type graphEnv struct {
	*env
	T, S, L, C *domain.MissionStep
	fail       map[string]bool // verticals whose search fails
	delay      time.Duration
}

const (
	vTransport = string(domain.VerticalTransport)
	vHotel     = string(domain.VerticalHotel)
	vLocal     = string(domain.VerticalVehicleRental)
)

func newGraphEnv(t *testing.T) *graphEnv {
	t.Helper()
	e := newEnv(t)
	g := &graphEnv{env: e, fail: map[string]bool{}}
	m := e.addMission(alice, domain.MissionTypeTravel, "Kinshasa")
	m.StartDate, m.Participants = base.Unix()+30*86400, 2
	e.mission = m
	g.T = e.addStep(m, "FIND_TRANSPORT", domain.ActionSearchListings, 1, 1)
	g.S = e.addStep(m, "FIND_ACCOMMODATION", domain.ActionSearchListings, 1, 2)
	g.L = e.addStep(m, "FIND_LOCAL_TRANSPORT", domain.ActionSearchListings, 1, 3)
	g.C = e.addStep(m, "CHECK_AVAILABILITY", domain.ActionCheckAvailability, 1, 4)
	e.store.current[m.ID] = 1
	e.store.deps = append(e.store.deps, domain.StepDependency{StepID: g.C.ID, DependsOnID: g.S.ID})
	e.rt.do = func(_ context.Context, a *domain.CommerceAction, _ domain.CommerceActor) (*commerce.ExecuteResult, error) {
		if g.delay > 0 {
			time.Sleep(g.delay)
		}
		if a.ActionType == domain.ActionCheckAvailability {
			return availabilitySuccess(true), nil
		}
		if v, _ := a.Parameters["vertical"].(string); g.fail[v] {
			return failed("ACTION_FAILED"), nil
		}
		return searchSuccess(3), nil
	}
	return g
}

func (g *graphEnv) orch() *missionorch.Orchestrator {
	return missionorch.New(g.store, g.store, g.ex, zerolog.Nop()).WithClock(func() time.Time { return time.Unix(g.clock, 0) })
}

func (g *graphEnv) run(opts missionorch.Options) (*missionorch.State, error) {
	return g.orch().Run(context.Background(), alice, g.mission.ID, opts)
}

// sequence is what the runtime was asked, in order: the vertical of a search, CHECK for an availability check.
func (g *graphEnv) sequence() []string {
	g.rt.mu.Lock()
	defer g.rt.mu.Unlock()
	var out []string
	for _, c := range g.rt.calls {
		if c.ActionType == domain.ActionCheckAvailability {
			out = append(out, "CHECK")
			continue
		}
		out = append(out, c.Parameters["vertical"].(string))
	}
	return out
}

func (g *graphEnv) attempts(s *domain.MissionStep) int { return len(g.store.executions(s.ID)) }

func same(a, b []string) bool { return strings.Join(a, ",") == strings.Join(b, ",") }

func flowStates(st *missionorch.State) map[string]domain.StepFlowState {
	out := map[string]domain.StepFlowState{}
	for _, s := range st.Flow.Steps {
		out[s.Step.ID] = s.State.Flow
	}
	return out
}

func TestOrchestrate_RunsTheWholeMissionInDependencyOrder(t *testing.T) {
	g := newGraphEnv(t)
	st, err := g.run(missionorch.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !same(g.sequence(), []string{vTransport, vHotel, vLocal, "CHECK"}) {
		t.Fatalf("sequence %v", g.sequence())
	}
	if st.Outcome != domain.OutcomeCompleted || st.Mission.Status != domain.MissionCompleted || st.Busy || st.Progress != (domain.Progress{Total: 4, Completed: 4}) {
		t.Fatalf("state: %s %s busy=%v %+v", st.Outcome, st.Mission.Status, st.Busy, st.Progress)
	}
	if len(st.Ran) != 4 || st.Ran[3].StepID != g.C.ID || st.Ran[3].Status != domain.ExecutionCompleted || st.Ran[0].ExecutionID == "" {
		t.Fatalf("ran %+v", st.Ran)
	}
	for _, s := range []*domain.MissionStep{g.T, g.S, g.L, g.C} {
		if g.store.step(s.ID).Status != domain.StepCompleted || g.attempts(s) != 1 {
			t.Fatalf("%s: %s, %d attempts", s.Type, g.store.step(s.ID).Status, g.attempts(s))
		}
	}
	// The result of the search reached the check by reference: the first listing it found.
	for _, c := range g.rt.calls {
		if c.ActionType == domain.ActionCheckAvailability && c.Parameters["listing_id"] != "listing-a" {
			t.Fatalf("the check took %v", c.Parameters["listing_id"])
		}
	}
	if g.store.missionStatus(g.mission.ID) != domain.MissionCompleted {
		t.Fatal("completed only because every step is")
	}
}

func TestOrchestrate_ADependencyBeatsPositionOrder(t *testing.T) {
	g := newGraphEnv(t)
	// The check is listed first but must wait for the accommodation search.
	g.store.steps[g.C.ID].Position = 1
	g.store.steps[g.T.ID].Position = 4
	if _, err := g.run(missionorch.Options{}); err != nil {
		t.Fatal(err)
	}
	seq := g.sequence()
	idx := func(v string) int {
		for i, x := range seq {
			if x == v {
				return i
			}
		}
		return -1
	}
	if idx("CHECK") < idx(vHotel) || len(seq) != 4 {
		t.Fatalf("a step runs after the steps it depends on: %v", seq)
	}
}

func TestOrchestrate_ASingleStepMission(t *testing.T) {
	e := newEnv(t)
	m := e.addMission(alice, domain.MissionTypeTravel, "Goma")
	e.mission = m
	only := e.addStep(m, "FIND_ACCOMMODATION", domain.ActionSearchListings, 1, 1)
	e.store.current[m.ID] = 1
	g := &graphEnv{env: e}
	st, err := g.run(missionorch.Options{})
	if err != nil || st.Outcome != domain.OutcomeCompleted || len(st.Ran) != 1 || e.store.step(only.ID).Status != domain.StepCompleted {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestOrchestrate_AFailureBlocksItsDependentsAndKeepsWhatCompleted(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vHotel] = true
	st, err := g.run(missionorch.Options{})
	if err != nil {
		t.Fatal(err)
	}
	// The default policy: stop at the first failure. T completed and stays; S failed; L was never started; C is blocked.
	if !same(g.sequence(), []string{vTransport, vHotel}) {
		t.Fatalf("sequence %v", g.sequence())
	}
	got := flowStates(st)
	if got[g.T.ID] != domain.FlowCompleted || got[g.S.ID] != domain.FlowFailed || got[g.L.ID] != domain.FlowReady || got[g.C.ID] != domain.FlowBlocked {
		t.Fatalf("states %v", got)
	}
	if st.Outcome != domain.OutcomeNeedsAttention || st.Mission.Status != domain.MissionActive || st.Progress.Completed != 1 {
		t.Fatalf("partial failure: %s %s %+v", st.Outcome, st.Mission.Status, st.Progress)
	}
	var blocked domain.FlowStep
	for _, s := range st.Flow.Steps {
		if s.Step.ID == g.C.ID {
			blocked = s
		}
	}
	if blocked.State.Reason != domain.ReasonDependencyFailed || blocked.State.BlockedBy[0] != g.S.ID {
		t.Fatalf("the check says why: %+v", blocked.State)
	}
	if g.attempts(g.C) != 0 || g.store.step(g.C.ID).Status != domain.StepPending {
		t.Fatal("a blocked step is never run")
	}
	if g.store.step(g.T.ID).Status != domain.StepCompleted {
		t.Fatal("no rollback: what completed stays completed")
	}
	if len(st.Ran) != 2 || st.Ran[1].Status != domain.ExecutionFailed || st.Ran[1].ErrorCode != "ACTION_FAILED" {
		t.Fatalf("ran %+v", st.Ran)
	}
}

func TestOrchestrate_ContinueIndependentCarriesOnWithWhatDoesNotDependOnTheFailure(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vHotel] = true
	st, err := g.run(missionorch.Options{OnFailure: missionorch.ContinueIndependent})
	if err != nil {
		t.Fatal(err)
	}
	if !same(g.sequence(), []string{vTransport, vHotel, vLocal}) {
		t.Fatalf("sequence %v", g.sequence())
	}
	got := flowStates(st)
	if got[g.L.ID] != domain.FlowCompleted || got[g.S.ID] != domain.FlowFailed || got[g.C.ID] != domain.FlowBlocked {
		t.Fatalf("states %v", got)
	}
	if g.attempts(g.S) != 1 {
		t.Fatalf("a failed step is not retried within the same run: %d attempts", g.attempts(g.S))
	}
}

func TestOrchestrate_ResumeRetriesOnlyWhatFailedAndNeverRunsACompletedStepAgain(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vHotel] = true
	if _, err := g.run(missionorch.Options{}); err != nil {
		t.Fatal(err)
	}
	callsBefore := g.rt.callCount()

	g.fail[vHotel] = false // the cause is gone
	st, err := g.run(missionorch.Options{RetryFailed: true})
	if err != nil {
		t.Fatal(err)
	}
	if st.Outcome != domain.OutcomeCompleted || st.Mission.Status != domain.MissionCompleted || st.Progress.Completed != 4 {
		t.Fatalf("after the resume: %s %s %+v", st.Outcome, st.Mission.Status, st.Progress)
	}
	if !same(g.sequence()[callsBefore:], []string{vHotel, vLocal, "CHECK"}) {
		t.Fatalf("the resume ran %v", g.sequence()[callsBefore:])
	}
	if g.attempts(g.T) != 1 {
		t.Fatalf("the transport step completed before the failure and is not run again: %d attempts", g.attempts(g.T))
	}
	if h := g.store.executions(g.S.ID); len(h) != 2 || h[0].Status != domain.ExecutionFailed || h[1].Status != domain.ExecutionCompleted || h[1].Attempt != 2 {
		t.Fatalf("a retry is the next attempt, the failed one stays as history: %+v", h)
	}
}

func TestOrchestrate_AResumeThatFailsAgainStopsAndRetriesEachStepOncePerCall(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vHotel] = true
	_, _ = g.run(missionorch.Options{})
	for i := 0; i < 3; i++ {
		if _, err := g.run(missionorch.Options{RetryFailed: true}); err != nil {
			t.Fatal(err)
		}
	}
	// One attempt per call: no infinite retry, no hidden retry.
	if g.attempts(g.S) != 4 {
		t.Fatalf("attempts of S = %d, want 1 + one per resume", g.attempts(g.S))
	}
}

func TestOrchestrate_AnOrdinaryRunNeverRetriesAFailedStep(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vHotel] = true
	_, _ = g.run(missionorch.Options{})
	g.fail[vHotel] = false
	st, err := g.run(missionorch.Options{}) // a run, not a resume
	if err != nil {
		t.Fatal(err)
	}
	if g.attempts(g.S) != 1 || flowStates(st)[g.S.ID] != domain.FlowFailed {
		t.Fatalf("S was retried by a plain run: %d attempts, %v", g.attempts(g.S), flowStates(st))
	}
	// What is still ready does run (L), and what depends on the failure stays blocked.
	if flowStates(st)[g.L.ID] != domain.FlowCompleted || flowStates(st)[g.C.ID] != domain.FlowBlocked || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("states %v %s", flowStates(st), st.Outcome)
	}
}

func TestOrchestrate_RepeatedOrchestrationRunsNothingTwice(t *testing.T) {
	g := newGraphEnv(t)
	first, err := g.run(missionorch.Options{})
	if err != nil {
		t.Fatal(err)
	}
	calls := g.rt.callCount()
	for i := 0; i < 3; i++ {
		for _, opts := range []missionorch.Options{{}, {RetryFailed: true}} {
			st, err := g.run(opts)
			if err != nil {
				t.Fatalf("a completed mission is not an error: %v", err)
			}
			if len(st.Ran) != 0 || st.Outcome != domain.OutcomeCompleted || st.Mission.Status != domain.MissionCompleted {
				t.Fatalf("repeat %d: %+v", i, st)
			}
		}
	}
	if g.rt.callCount() != calls || calls != 4 {
		t.Fatalf("the runtime was asked %d times, want exactly the 4 steps once", g.rt.callCount())
	}
	again, _ := g.orch().State(context.Background(), alice, g.mission.ID)
	if again.Progress != first.Progress {
		t.Fatalf("progress %+v vs %+v", again.Progress, first.Progress)
	}
}

func TestOrchestrate_TenSimultaneousOrchestrationsRunEachStepOnce(t *testing.T) {
	g := newGraphEnv(t)
	g.delay = 15 * time.Millisecond
	const n = 10
	var wg sync.WaitGroup
	states := make([]*missionorch.State, n)
	errs := make([]error, n)
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			states[i], errs[i] = g.run(missionorch.Options{})
		}(i)
	}
	close(start)
	wg.Wait()

	busy, ran := 0, 0
	for i := range states {
		if errs[i] != nil {
			t.Fatalf("orchestration %d: %v", i, errs[i])
		}
		if states[i].Busy {
			busy++
		}
		ran += len(states[i].Ran)
	}
	if g.rt.callCount() != 4 || ran != 4 {
		t.Fatalf("the runtime was asked %d times and %d steps were reported, want 4 and 4", g.rt.callCount(), ran)
	}
	for _, s := range []*domain.MissionStep{g.T, g.S, g.L, g.C} {
		if g.attempts(s) != 1 || g.store.step(s.ID).Status != domain.StepCompleted {
			t.Fatalf("%s: %d attempts, %s", s.Type, g.attempts(s), g.store.step(s.ID).Status)
		}
	}
	if busy == 0 {
		t.Fatal("the others were told the mission is being run")
	}
	if g.store.missionStatus(g.mission.ID) != domain.MissionCompleted {
		t.Fatal("the mission completed once")
	}
	if held, _ := g.store.RunInProgress(context.Background(), g.mission.ID, time.Unix(g.clock, 0)); held {
		t.Fatal("the lock is released")
	}
}

// noLock hands the lock to everybody: only the step claim stands between two orchestrations and a double run.
type noLock struct{ *memStore }

func (noLock) AcquireRun(context.Context, string, string, time.Time, time.Duration) (bool, error) {
	return true, nil
}
func (noLock) ExtendRun(context.Context, string, string, time.Time, time.Duration) error { return nil }

func TestOrchestrate_TheStepClaimIsTheSecondLineOfDefenceWithoutTheLock(t *testing.T) {
	g := newGraphEnv(t)
	g.delay = 10 * time.Millisecond
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := missionorch.New(g.store, noLock{g.store}, g.ex, zerolog.Nop()).WithClock(func() time.Time { return time.Unix(g.clock, 0) })
			if _, err := o.Run(context.Background(), alice, g.mission.ID, missionorch.Options{}); err != nil {
				t.Errorf("%v", err)
			}
		}()
	}
	wg.Wait()
	for _, s := range []*domain.MissionStep{g.T, g.S, g.L, g.C} {
		if g.attempts(s) != 1 {
			t.Fatalf("%s ran %d times without the lock", s.Type, g.attempts(s))
		}
	}
	if g.rt.callCount() != 4 {
		t.Fatalf("the runtime was asked %d times", g.rt.callCount())
	}
}

func TestOrchestrate_AMissionBeingRunRunsNothingAndALapsedLeaseIsTakenOver(t *testing.T) {
	g := newGraphEnv(t)
	now := time.Unix(g.clock, 0)
	if ok, _ := g.store.AcquireRun(context.Background(), g.mission.ID, "someone-else", now, time.Minute); !ok {
		t.Fatal("setup")
	}
	st, err := g.run(missionorch.Options{})
	if err != nil || !st.Busy || st.Outcome != domain.OutcomeInProgress || len(st.Ran) != 0 || g.rt.callCount() != 0 {
		t.Fatalf("busy: %+v %v calls=%d", st, err, g.rt.callCount())
	}
	read, _ := g.orch().State(context.Background(), alice, g.mission.ID)
	if !read.Busy || read.Outcome != domain.OutcomeInProgress {
		t.Fatalf("a read says so too: %+v", read)
	}
	// The holder died: after its lease the mission can be run again.
	g.clock += 120
	st, err = g.run(missionorch.Options{})
	if err != nil || st.Busy || st.Outcome != domain.OutcomeCompleted {
		t.Fatalf("takeover: %+v %v", st, err)
	}
}

func TestOrchestrate_TheLockIsReleasedAfterAFailureToo(t *testing.T) {
	g := newGraphEnv(t)
	g.fail[vTransport] = true
	if _, err := g.run(missionorch.Options{}); err != nil {
		t.Fatal(err)
	}
	if held, _ := g.store.RunInProgress(context.Background(), g.mission.ID, time.Unix(g.clock, 0)); held {
		t.Fatal("a failed run must not keep the mission locked")
	}
	g.fail[vTransport] = false
	if st, err := g.run(missionorch.Options{RetryFailed: true}); err != nil || st.Busy || st.Outcome != domain.OutcomeCompleted {
		t.Fatalf("%+v %v", st, err)
	}
}

func TestOrchestrate_MissionStatusIsRespected(t *testing.T) {
	for _, status := range []domain.MissionStatus{domain.MissionPaused, domain.MissionCancelled} {
		g := newGraphEnv(t)
		g.store.missions[g.mission.ID].Status = status
		if _, err := g.run(missionorch.Options{}); !errors.Is(err, domain.ErrMissionNotRunnable) {
			t.Errorf("%s = %v, want ErrMissionNotRunnable", status, err)
		}
		if g.rt.callCount() != 0 {
			t.Errorf("%s mission ran something", status)
		}
	}
	// A completed mission answers with its state: nothing to run, nothing wrong.
	g := newGraphEnv(t)
	g.store.missions[g.mission.ID].Status = domain.MissionCompleted
	if st, err := g.run(missionorch.Options{}); err != nil || len(st.Ran) != 0 || g.rt.callCount() != 0 {
		t.Fatalf("completed: %+v %v", st, err)
	}
	// A draft becomes active when its first step is run.
	g = newGraphEnv(t)
	g.fail[vTransport] = true // stops after the first step
	if st, _ := g.run(missionorch.Options{}); st.Mission.Status != domain.MissionActive {
		t.Fatalf("a draft that was run is active, got %s", st.Mission.Status)
	}
	// A draft with nothing to run stays a draft.
	g = newGraphEnv(t)
	for _, s := range []*domain.MissionStep{g.T, g.S, g.L, g.C} {
		g.store.steps[s.ID].Status = domain.StepSkipped
	}
	if st, _ := g.run(missionorch.Options{}); st.Mission.Status != domain.MissionDraft || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("nothing ran: %s %s", st.Mission.Status, st.Outcome)
	}
}

func TestOrchestrate_OwnershipExistenceAndInvalidGraphs(t *testing.T) {
	g := newGraphEnv(t)
	o := g.orch()
	if _, err := o.Run(context.Background(), bob, g.mission.ID, missionorch.Options{}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("User B = %v", err)
	}
	if _, err := o.State(context.Background(), bob, g.mission.ID); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("User B reading = %v", err)
	}
	if _, err := o.Run(context.Background(), "", g.mission.ID, missionorch.Options{}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("nobody = %v", err)
	}
	if _, err := o.Run(context.Background(), alice, "no-such-mission", missionorch.Options{}); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
	if g.rt.callCount() != 0 {
		t.Fatal("a refused request reaches nothing")
	}
	if held, _ := g.store.RunInProgress(context.Background(), g.mission.ID, time.Unix(g.clock, 0)); held {
		t.Fatal("a refused request takes no lock")
	}

	// No plan.
	delete(g.store.current, g.mission.ID)
	if _, err := o.Run(context.Background(), alice, g.mission.ID, missionorch.Options{}); !errors.Is(err, domain.ErrNoPlanToRun) {
		t.Fatalf("no plan = %v", err)
	}
	g.store.current[g.mission.ID] = 1

	// Circular and invalid dependencies are refused before anything runs.
	g.store.deps = append(g.store.deps, domain.StepDependency{StepID: g.S.ID, DependsOnID: g.C.ID})
	if _, err := o.Run(context.Background(), alice, g.mission.ID, missionorch.Options{}); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatalf("cycle = %v", err)
	}
	if _, err := o.State(context.Background(), alice, g.mission.ID); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatalf("cycle on read = %v", err)
	}
	g.store.deps[len(g.store.deps)-1] = domain.StepDependency{StepID: g.S.ID, DependsOnID: g.S.ID}
	if _, err := o.Run(context.Background(), alice, g.mission.ID, missionorch.Options{}); !errors.Is(err, domain.ErrDependencyInvalid) {
		t.Fatalf("self dependency = %v", err)
	}
	if g.rt.callCount() != 0 || g.store.missionStatus(g.mission.ID) != domain.MissionDraft {
		t.Fatal("nothing ran from a graph that cannot be ordered")
	}
}

func TestOrchestrate_AConfirmationIsLeftToThePersonAndNeverRetried(t *testing.T) {
	g := newGraphEnv(t)
	inner := g.rt.do
	g.rt.do = func(ctx context.Context, a *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error) {
		if a.Parameters["vertical"] == vHotel {
			return &commerce.ExecuteResult{Status: domain.ActionResultRequiresConfirmation, RequiresConfirmation: true}, nil
		}
		return inner(ctx, a, actor)
	}
	st, err := g.run(missionorch.Options{OnFailure: missionorch.ContinueIndependent})
	if err != nil {
		t.Fatal(err)
	}
	got := flowStates(st)
	if got[g.S.ID] != domain.FlowBlocked || got[g.C.ID] != domain.FlowBlocked || got[g.L.ID] != domain.FlowCompleted {
		t.Fatalf("states %v", got)
	}
	calls := g.rt.callCount()
	if st, err = g.run(missionorch.Options{RetryFailed: true}); err != nil || g.rt.callCount() != calls || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("a resume does not decide for the person: %v calls %d -> %d", err, calls, g.rt.callCount())
	}
	for _, c := range g.rt.calls {
		if c.IsConfirmed() {
			t.Fatal("nothing is confirmed on the person's behalf")
		}
	}
}

func TestOrchestrate_AStepThatWentSilentIsResumedNotRun(t *testing.T) {
	g := newGraphEnv(t)
	if _, _, err := g.store.BeginExecution(context.Background(), domain.BeginExecutionInput{MissionID: g.mission.ID, StepID: g.T.ID, ActionType: domain.ActionSearchListings, Now: g.clock, StaleAfter: domain.DefaultExecutionStaleAfter}); err != nil {
		t.Fatal(err)
	}
	g.clock += 600 // the attempt is long gone
	st, err := g.run(missionorch.Options{})
	if err != nil || g.attempts(g.T) != 1 {
		t.Fatalf("a plain run leaves the silent step alone: %v %d", err, g.attempts(g.T))
	}
	if flowStates(st)[g.T.ID] != domain.FlowFailed || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("states %v %s", flowStates(st), st.Outcome)
	}
	if st, err = g.run(missionorch.Options{RetryFailed: true}); err != nil || st.Outcome != domain.OutcomeCompleted || g.attempts(g.T) != 2 {
		t.Fatalf("resume: %v %d", err, g.attempts(g.T))
	}
}

func TestOrchestrate_OnlyThePlansStepsAreRunAndAReadChangesNothing(t *testing.T) {
	g := newGraphEnv(t)
	hand := g.addStep(g.mission, "FIND_ACCOMMODATION", domain.ActionSearchListings, 0, 9) // added by hand: not part of the plan
	before := g.rt.callCount()
	st, err := g.orch().State(context.Background(), alice, g.mission.ID)
	if err != nil || g.rt.callCount() != before || len(st.Ran) != 0 || st.Outcome != domain.OutcomeReady || st.Mission.Status != domain.MissionDraft {
		t.Fatalf("a read runs nothing: %+v %v", st, err)
	}
	if len(st.Flow.Steps) != 4 {
		t.Fatalf("the plan's steps only: %d", len(st.Flow.Steps))
	}
	if _, err := g.run(missionorch.Options{}); err != nil {
		t.Fatal(err)
	}
	if g.attempts(hand) != 0 || g.store.step(hand.ID).Status != domain.StepPending {
		t.Fatal("a hand-added step is never orchestrated")
	}
}

// ---- credits: the runtime's, never the orchestrator's ----------------------------------------------------------------

// availStub is the availability handler, answering without a database.
type availStub struct {
	mu    sync.Mutex
	asked []map[string]interface{}
}

func (a *availStub) Handle(_ context.Context, _ domain.CommerceActor, p map[string]interface{}) (interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asked = append(a.asked, p)
	return map[string]interface{}{"available": true, "capacity": 5, "remaining": 5, "reason": "", "start_at": p["start_at"], "end_at": p["end_at"]}, nil
}
func (a *availStub) ActionType() domain.CommerceActionType { return domain.ActionCheckAvailability }
func (a *availStub) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.asked)
}

// stayRealEnv is a stay (search, then availability) through the REAL runtime: parameter validation, idempotency store,
// confirmation policy, usage check and the real search handler; only the catalogue and the credit store are faked.
func stayRealEnv(t *testing.T, search *searchStub, credits *creditStore) (*graphEnv, *availStub) {
	t.Helper()
	avail := &availStub{}
	registry := commerce.NewHandlerRegistry()
	registry.Register(commerce.NewSearchHandler(search))
	registry.Register(avail)
	rt := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(), ParameterValidator: commerce.NewDefaultParameterValidator(),
		UsageService: credits,
	})
	e := newEnv(t)
	e.ex = New(e.store, e.store, e.store, e.store, rt, zerolog.Nop()).WithClock(func() time.Time { return time.Unix(e.clock, 0) })
	m := e.addMission(alice, domain.MissionTypeStay, "Kinshasa")
	m.StartDate, m.EndDate, m.Participants = base.Unix()+30*86400, base.Unix()+33*86400, 2
	e.mission = m
	e.stay = e.addStep(m, "FIND_ACCOMMODATION", domain.ActionSearchListings, 1, 1)
	e.availability = e.addStep(m, "CHECK_AVAILABILITY", domain.ActionCheckAvailability, 1, 2)
	e.store.current[m.ID] = 1
	e.store.deps = append(e.store.deps, domain.StepDependency{StepID: e.availability.ID, DependsOnID: e.stay.ID})
	return &graphEnv{env: e, S: e.stay, C: e.availability}, avail
}

func TestOrchestrate_ThroughTheRealRuntime_CreditsAreChargedOncePerSuccessAndNeverByTheOrchestrator(t *testing.T) {
	credits := newCreditStore(2, 10)
	search := &searchStub{page: domain.SearchResultPage{Total: 2, Items: []domain.SearchResult{{ListingID: "l1"}, {ListingID: "l2"}}}}
	g, avail := stayRealEnv(t, search, credits)

	st, err := g.run(missionorch.Options{})
	if err != nil || st.Outcome != domain.OutcomeCompleted || st.Mission.Status != domain.MissionCompleted {
		t.Fatalf("%+v %v", st, err)
	}
	// The search is metered by the runtime: one reservation, consumed once. The availability check is not metered.
	if credits.reserved != 1 || credits.consumed != 1 || credits.released != 0 || credits.balance != 8 || len(credits.open) != 0 {
		t.Fatalf("reserved=%d consumed=%d released=%d balance=%d", credits.reserved, credits.consumed, credits.released, credits.balance)
	}
	// What the check asked: the first listing found, validated by the real runtime, with the mission's dates.
	if avail.count() != 1 || avail.asked[0]["listing_id"] != "l1" || avail.asked[0]["start_at"] != base.Unix()+30*86400 || avail.asked[0]["end_at"] != base.Unix()+33*86400 || avail.asked[0]["party_size"] != 2 {
		t.Fatalf("availability asked %v", avail.asked)
	}
	// Repeating the orchestration charges nothing and asks nothing.
	for i := 0; i < 3; i++ {
		if _, err := g.run(missionorch.Options{RetryFailed: i == 1}); err != nil {
			t.Fatal(err)
		}
	}
	if credits.reserved != 1 || credits.consumed != 1 || credits.balance != 8 || search.count() != 1 || avail.count() != 1 {
		t.Fatalf("a repeat must not charge or search again: reserved=%d consumed=%d balance=%d searches=%d checks=%d", credits.reserved, credits.consumed, credits.balance, search.count(), avail.count())
	}
}

func TestOrchestrate_ThroughTheRealRuntime_AFailedSearchKeepsTheCreditsAndTheResumeChargesItsSuccessOnly(t *testing.T) {
	credits := newCreditStore(2, 10)
	search := &searchStub{err: errors.New("opensearch: connection refused (10.1.2.3:9200)")}
	g, avail := stayRealEnv(t, search, credits)

	st, err := g.run(missionorch.Options{})
	if err != nil || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("%+v %v", st, err)
	}
	if credits.reserved != 1 || credits.released != 1 || credits.consumed != 0 || credits.balance != 10 {
		t.Fatalf("a failed action keeps the credits: reserved=%d released=%d consumed=%d balance=%d", credits.reserved, credits.released, credits.consumed, credits.balance)
	}
	if avail.count() != 0 || flowStates(st)[g.C.ID] != domain.FlowBlocked {
		t.Fatalf("the dependent is blocked and never reaches the runtime: %d %v", avail.count(), flowStates(st))
	}
	search.mu.Lock()
	search.err = nil
	search.page = domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}
	search.mu.Unlock()
	if st, err = g.run(missionorch.Options{RetryFailed: true}); err != nil || st.Outcome != domain.OutcomeCompleted {
		t.Fatalf("resume: %+v %v", st, err)
	}
	if credits.reserved != 2 || credits.released != 1 || credits.consumed != 1 || credits.balance != 8 || len(credits.open) != 0 {
		t.Fatalf("charged for the success only: reserved=%d released=%d consumed=%d balance=%d", credits.reserved, credits.released, credits.consumed, credits.balance)
	}
}

func TestOrchestrate_ThroughTheRealRuntime_InsufficientCreditsStopsTheRunAndAResumeWaitsForCredits(t *testing.T) {
	credits := newCreditStore(5, 3)
	g, avail := stayRealEnv(t, &searchStub{page: domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}}, credits)
	st, err := g.run(missionorch.Options{})
	if err != nil || len(st.Ran) != 1 || st.Ran[0].ErrorCode != "INSUFFICIENT_CREDITS" || st.Outcome != domain.OutcomeNeedsAttention {
		t.Fatalf("%+v %v", st, err)
	}
	if credits.reserved != 0 || avail.count() != 0 {
		t.Fatalf("the runtime refused before reserving: reserved=%d", credits.reserved)
	}
	credits.balance = 10
	if st, err = g.run(missionorch.Options{RetryFailed: true}); err != nil || st.Outcome != domain.OutcomeCompleted || credits.consumed != 1 {
		t.Fatalf("after credits: %+v %v consumed=%d", st, err, credits.consumed)
	}
}

func TestOrchestrate_ThroughTheRealRuntime_TenSimultaneousOrchestrationsChargeOnce(t *testing.T) {
	credits := newCreditStore(2, 10)
	g, avail := stayRealEnv(t, &searchStub{page: domain.SearchResultPage{Total: 1, Items: []domain.SearchResult{{ListingID: "l1"}}}}, credits)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := g.run(missionorch.Options{}); err != nil {
				t.Errorf("%v", err)
			}
		}()
	}
	wg.Wait()
	if credits.reserved != 1 || credits.consumed != 1 || credits.balance != 8 || avail.count() != 1 {
		t.Fatalf("one charge, one check: reserved=%d consumed=%d balance=%d checks=%d", credits.reserved, credits.consumed, credits.balance, avail.count())
	}
}

// The orchestrator has no credit accounting of its own, and no way to reach the runtime: it only asks the executor.
func TestOrchestratorHasNoCreditAccountingNorRuntimeOfItsOwn(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("..", "missionorch", "*.go"))
	if len(files) == 0 {
		t.Fatal("the orchestrator's sources were not found")
	}
	forbidden := []string{"UsageService", "CreditAccount", "UsageReservation", "UsagePolicy", "usageService", ".Reserve(", ".Consume(", ".Release(", "Grant(",
		"CommerceActionRuntime", "commerce.", "NewCommerceAction", "ActionSearchListings", "ActionCheckAvailability", "database/sql"}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range forbidden {
			if strings.Contains(string(src), w) {
				t.Errorf("%s mentions %q: the orchestrator only orders and delegates", f, w)
			}
		}
	}
}
