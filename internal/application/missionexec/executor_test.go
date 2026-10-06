package missionexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestExecute_HappyPath_RuntimeToResultToCompleted(t *testing.T) {
	e := newEnv(t)
	res, err := e.run(alice, e.stay, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Replayed || res.Execution.Status != domain.ExecutionCompleted || res.Execution.Attempt != 1 || res.Step.Status != domain.StepCompleted {
		t.Fatalf("result %+v / %+v", res.Execution, res.Step)
	}
	if res.Execution.ResultReference != res.Execution.ID || res.Step.ResultRef != res.Execution.ID || res.Step.ActionRef != res.Execution.ID {
		t.Fatalf("the step points at the execution that holds the result: %q %q %q", res.Execution.ResultReference, res.Step.ResultRef, res.Step.ActionRef)
	}
	p := res.Execution.ResultPayload
	ids, _ := p["listing_ids"].([]string)
	if p["total"] != 3 || len(ids) != 3 || ids[0] != "listing-a" || p["vertical"] != "HOTEL" || p["location"] != "Kinshasa" || p["has_more"] != false {
		t.Fatalf("a small summary with references, not the dataset: %v", p)
	}
	if res.Execution.CompletedAt == 0 || res.Execution.RuntimeStatus != "SUCCESS" || res.Execution.ErrorCode != "" {
		t.Fatalf("execution %+v", res.Execution)
	}

	// The runtime got the existing action, as the person, with the mission as its context.
	if e.rt.callCount() != 1 {
		t.Fatalf("one runtime call, got %d", e.rt.callCount())
	}
	a := e.rt.calls[0]
	if a.ActionType != domain.ActionSearchListings || a.ConversationID != "mission:"+e.mission.ID || a.Source != "USER" || a.ActionID != res.Execution.ID {
		t.Fatalf("action %+v", a)
	}
	if a.Parameters["vertical"] != "HOTEL" || a.Parameters["location"] != "Kinshasa" || a.Parameters["page_size"] != 10 {
		t.Fatalf("parameters %v", a.Parameters)
	}
	if actor := e.rt.actors[0]; actor.ID != alice || actor.Type != domain.ActorTypeCustomer {
		t.Fatalf("the runtime acts as the signed-in person: %+v", actor)
	}
	if !strings.HasPrefix(a.IdempotencyKey, "mission-step:"+e.stay.ID+":") {
		t.Fatalf("runtime idempotency key %q", a.IdempotencyKey)
	}
	// The mission is neither activated nor completed by a step.
	if m, _ := e.store.Get(context.Background(), e.mission.ID); m.Status != domain.MissionDraft {
		t.Fatalf("mission status %s", m.Status)
	}
	// Other steps are untouched.
	if e.store.step(e.activities.ID).Status != domain.StepPending {
		t.Fatal("only the requested step runs")
	}
}

func TestExecute_NoPlaceMeansNoPlaceParameter(t *testing.T) {
	e := newEnv(t)
	e.mission.Location = ""
	e.store.missions[e.mission.ID].Location = ""
	if _, err := e.run(alice, e.stay, ""); err != nil {
		t.Fatal(err)
	}
	if _, has := e.rt.calls[0].Parameters["location"]; has {
		t.Fatalf("a place is never made up: %v", e.rt.calls[0].Parameters)
	}
}

func TestExecute_VerticalFollowsTheKindOfMission(t *testing.T) {
	for typ, want := range map[domain.MissionType]string{domain.MissionTypeStay: "HOTEL", domain.MissionTypeTravel: "HOTEL", domain.MissionTypeMove: "REAL_ESTATE"} {
		e := newEnv(t)
		m := e.addMission(alice, typ, "")
		step := e.addStep(m, "FIND_ACCOMMODATION", domain.ActionSearchListings, 0, 1)
		if _, err := e.ex.Execute(context.Background(), alice, m.ID, step.ID, ""); err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if got := e.rt.calls[0].Parameters["vertical"]; got != want {
			t.Errorf("%s searches %v, want %s", typ, got, want)
		}
	}
}

func TestExecute_OwnershipAndLookups(t *testing.T) {
	e := newEnv(t)
	if _, err := e.run(bob, e.stay, ""); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("User B on User A's step = %v, want ErrMissionForbidden", err)
	}
	if _, err := e.run("", e.stay, ""); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("no user = %v", err)
	}
	if _, err := e.ex.Execute(context.Background(), alice, "missing", e.stay.ID, ""); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
	if _, err := e.ex.Execute(context.Background(), alice, e.mission.ID, "missing", ""); !errors.Is(err, domain.ErrMissionStepNotFound) {
		t.Fatalf("unknown step = %v", err)
	}
	other := e.addMission(alice, domain.MissionTypeTravel, "")
	foreign := e.addStep(other, "FIND_ACCOMMODATION", domain.ActionSearchListings, 0, 1)
	if _, err := e.ex.Execute(context.Background(), alice, e.mission.ID, foreign.ID, ""); !errors.Is(err, domain.ErrMissionStepNotFound) {
		t.Fatalf("a step of another mission = %v", err)
	}
	if e.rt.callCount() != 0 || len(e.store.executions(e.stay.ID)) != 0 {
		t.Fatal("a refused request runs nothing and records nothing")
	}
	if e.store.step(e.stay.ID).Status != domain.StepPending {
		t.Fatal("a refused request leaves the step as it was")
	}
}

func TestExecute_RuntimeFailureIsRecordedAsFailedWithAStableCode(t *testing.T) {
	cases := []struct {
		name string
		do   func(ctx context.Context, a *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error)
		want string
		rt   string
	}{
		{"handler failed", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return failed("ACTION_FAILED"), nil
		}, "ACTION_FAILED", "FAILED"},
		{"insufficient credits", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return failed("INSUFFICIENT_CREDITS"), nil
		}, "INSUFFICIENT_CREDITS", "FAILED"},
		{"validation", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return &commerce.ExecuteResult{Status: domain.ActionResultValidationFailed, Errors: []domain.ActionError{domain.NewActionError("INVALID_PARAM", "vertical unknown", "vertical")}}, nil
		}, "INVALID_PARAM", "VALIDATION_FAILED"},
		{"not implemented", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return &commerce.ExecuteResult{Status: domain.ActionResultNotImplemented, Errors: []domain.ActionError{domain.NewActionError("ACTION_NOT_IMPLEMENTED", "x", "")}}, nil
		}, "ACTION_NOT_IMPLEMENTED", "NOT_IMPLEMENTED"},
		{"no error code, a status", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return &commerce.ExecuteResult{Status: domain.ActionResultTemporaryFailure}, nil
		}, "TEMPORARY_FAILURE", "TEMPORARY_FAILURE"},
		{"free text is not a code", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return &commerce.ExecuteResult{Status: domain.ActionResultFailed, Errors: []domain.ActionError{domain.NewActionError("connection to db at 10.0.0.5 refused", "x", "")}}, nil
		}, "FAILED", "FAILED"},
		{"asks for confirmation", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return &commerce.ExecuteResult{Status: domain.ActionResultRequiresConfirmation, RequiresConfirmation: true}, nil
		}, domain.ErrorCodeConfirmationRequired, "REQUIRES_CONFIRMATION"},
		{"runtime error", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return nil, errors.New("postgres: password authentication failed for user trezutil")
		}, domain.ErrorCodeRuntime, ""},
		{"nil result", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			return nil, nil
		}, domain.ErrorCodeRuntime, ""},
		{"panic", func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
			panic("nil pointer in handler")
		}, domain.ErrorCodeRuntime, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.rt.do = c.do
			res, err := e.run(alice, e.stay, "")
			if err != nil {
				t.Fatalf("a runtime failure is a recorded outcome, not an error: %v", err)
			}
			if res.Execution.Status != domain.ExecutionFailed || res.Step.Status != domain.StepFailed || res.Execution.ErrorCode != c.want || res.Execution.RuntimeStatus != c.rt {
				t.Fatalf("execution %+v step %s", res.Execution, res.Step.Status)
			}
			if res.Step.ResultRef != "" || res.Execution.ResultReference != "" || res.Execution.CompletedAt == 0 {
				t.Fatalf("a failure has no result: %+v", res.Execution)
			}
			stored := e.store.executions(e.stay.ID)
			raw, _ := json.Marshal(stored)
			for _, secret := range []string{"hunter2", "password", "10.0.0.5", "nil pointer"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("internal detail %q leaked into the record: %s", secret, raw)
				}
			}
			if e.store.step(e.stay.ID).Status == domain.StepInProgress {
				t.Fatal("a step is never left in progress")
			}
		})
	}
}

func TestExecute_TimeoutEndsTheAttemptAsFailed(t *testing.T) {
	e := newEnv(t)
	e.ex.WithTimeout(20 * time.Millisecond)
	e.rt.do = func(ctx context.Context, _ *domain.CommerceAction, _ domain.CommerceActor) (*commerce.ExecuteResult, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	res, err := e.run(alice, e.stay, "")
	if err != nil || res.Execution.Status != domain.ExecutionFailed || res.Execution.ErrorCode != domain.ErrorCodeTimeout || res.Step.Status != domain.StepFailed {
		t.Fatalf("timeout: %+v %v", res, err)
	}
}

func TestExecute_TheOutcomeIsRecordedEvenIfTheRequestIsCancelled(t *testing.T) {
	e := newEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		cancel() // the caller goes away while the runtime works
		return searchSuccess(2), nil
	}
	_, _ = e.ex.Execute(ctx, alice, e.mission.ID, e.stay.ID, "")
	if st := e.store.step(e.stay.ID); st.Status != domain.StepCompleted {
		t.Fatalf("the outcome is recorded: %s", st.Status)
	}
}

func TestExecute_FailedStepCanBeRetriedAndCompletes(t *testing.T) {
	e := newEnv(t)
	n := 0
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		n++
		if n == 1 {
			return failed("ACTION_FAILED"), nil
		}
		return searchSuccess(2), nil
	}
	first, _ := e.run(alice, e.stay, "k-1")
	if first.Step.Status != domain.StepFailed {
		t.Fatalf("first attempt: %+v", first.Step)
	}
	second, err := e.run(alice, e.stay, "k-2")
	if err != nil || second.Execution.Attempt != 2 || second.Execution.Status != domain.ExecutionCompleted || second.Step.Status != domain.StepCompleted {
		t.Fatalf("retry: %+v %v", second, err)
	}
	hist := e.store.executions(e.stay.ID)
	if len(hist) != 2 || hist[0].Status != domain.ExecutionFailed || hist[0].ErrorCode != "ACTION_FAILED" || hist[1].Status != domain.ExecutionCompleted {
		t.Fatalf("the failed attempt stays as history: %+v", hist)
	}
	if e.rt.callCount() != 2 {
		t.Fatalf("one runtime call per attempt, got %d", e.rt.callCount())
	}
	if e.store.step(e.stay.ID).ResultRef != second.Execution.ID {
		t.Fatal("the step points at the attempt that worked")
	}
}

func TestExecute_ACompletedStepIsNeverRunAgainNorReset(t *testing.T) {
	e := newEnv(t)
	if _, err := e.run(alice, e.stay, "k-1"); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "k-other"} {
		if _, err := e.run(alice, e.stay, key); !errors.Is(err, domain.ErrStepNotRunnable) {
			t.Fatalf("a completed step with key %q = %v, want ErrStepNotRunnable", key, err)
		}
	}
	if e.rt.callCount() != 1 || e.store.step(e.stay.ID).Status != domain.StepCompleted || len(e.store.executions(e.stay.ID)) != 1 {
		t.Fatal("a completed step is not run again and not reset")
	}
}

func TestExecute_SameStepAndKeyIsTheSameExecution(t *testing.T) {
	e := newEnv(t)
	first, err := e.run(alice, e.stay, "k-1")
	if err != nil || first.Replayed {
		t.Fatal(err)
	}
	again, err := e.run(alice, e.stay, "k-1")
	if err != nil || !again.Replayed || again.Execution.ID != first.Execution.ID || again.Execution.Status != domain.ExecutionCompleted {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if e.rt.callCount() != 1 || len(e.store.executions(e.stay.ID)) != 1 {
		t.Fatalf("one actual execution: %d calls", e.rt.callCount())
	}
	// A replay answers even when the mission has meanwhile been closed.
	e.store.missions[e.mission.ID].Status = domain.MissionCancelled
	if again, err := e.run(alice, e.stay, "k-1"); err != nil || !again.Replayed {
		t.Fatalf("replay on a closed mission: %v", err)
	}
	// ...but only for the owner.
	if _, err := e.run(bob, e.stay, "k-1"); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("another person replaying the key = %v", err)
	}
}

func TestExecute_AFailedKeyReplaysTheFailureAndNeedsANewKeyToRetry(t *testing.T) {
	e := newEnv(t)
	calls := 0
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		calls++
		if calls == 1 {
			return failed("ACTION_FAILED"), nil
		}
		return searchSuccess(1), nil
	}
	first, _ := e.run(alice, e.stay, "k-1")
	replay, err := e.run(alice, e.stay, "k-1")
	if err != nil || !replay.Replayed || replay.Execution.ID != first.Execution.ID || replay.Execution.Status != domain.ExecutionFailed {
		t.Fatalf("same key, same (failed) execution: %+v %v", replay, err)
	}
	if calls != 1 {
		t.Fatal("a replay runs nothing")
	}
	retry, err := e.run(alice, e.stay, "k-2")
	if err != nil || retry.Execution.Status != domain.ExecutionCompleted || calls != 2 {
		t.Fatalf("a new key retries: %+v %v", retry, err)
	}
}

func TestExecute_ConcurrentRequestsRunTheRuntimeOnce(t *testing.T) {
	e := newEnv(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		once.Do(func() { close(entered) })
		<-release
		return searchSuccess(2), nil
	}
	done := make(chan error, 1)
	go func() {
		_, err := e.run(alice, e.stay, "k-1")
		done <- err
	}()
	<-entered
	// While it runs, another request cannot start the step again, with or without another key.
	for _, key := range []string{"", "k-2"} {
		if _, err := e.run(alice, e.stay, key); !errors.Is(err, domain.ErrStepInProgress) {
			t.Fatalf("second request (key %q) = %v, want ErrStepInProgress", key, err)
		}
	}
	// The same key during the run returns the attempt in progress.
	if r, err := e.run(alice, e.stay, "k-1"); err != nil || !r.Replayed || r.Execution.Status != domain.ExecutionInProgress {
		t.Fatalf("same key while running: %+v %v", r, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if e.rt.callCount() != 1 || e.store.step(e.stay.ID).Status != domain.StepCompleted {
		t.Fatalf("one runtime call, got %d", e.rt.callCount())
	}
}

func TestExecute_ManySimultaneousRequestsStartOneAttempt(t *testing.T) {
	e := newEnv(t)
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		time.Sleep(20 * time.Millisecond)
		return searchSuccess(1), nil
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ran, refused := 0, 0
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.run(alice, e.stay, "")
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				ran++
			case errors.Is(err, domain.ErrStepInProgress), errors.Is(err, domain.ErrStepNotRunnable):
				refused++
			default:
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if ran != 1 || e.rt.callCount() != 1 || len(e.store.executions(e.stay.ID)) != 1 {
		t.Fatalf("exactly one attempt: ran=%d calls=%d refused=%d", ran, e.rt.callCount(), refused)
	}
}

func TestExecute_AnAbandonedAttemptIsGivenUpOnAndTheStepRunsAgain(t *testing.T) {
	e := newEnv(t)
	// An attempt began and never ended (the process died).
	stuck, _, err := e.store.BeginExecution(context.Background(), domain.BeginExecutionInput{MissionID: e.mission.ID, StepID: e.stay.ID, ActionType: domain.ActionSearchListings, Now: e.clock, StaleAfter: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	// Within the window it is still considered running.
	if _, err := e.run(alice, e.stay, ""); !errors.Is(err, domain.ErrStepInProgress) {
		t.Fatalf("a recent attempt is running: %v", err)
	}
	states, _ := e.ex.Annotate(context.Background(), e.mission, []domain.MissionStep{e.store.step(e.stay.ID)})
	if st := states[e.stay.ID]; st.Stale || st.CanExecute {
		t.Fatalf("not stale yet: %+v", st)
	}
	// Past the window it is given up on.
	e.clock += int64(domain.DefaultExecutionStaleAfter/time.Second) + 1
	states, _ = e.ex.Annotate(context.Background(), e.mission, []domain.MissionStep{e.store.step(e.stay.ID)})
	if st := states[e.stay.ID]; !st.Stale || !st.CanExecute {
		t.Fatalf("stale and runnable again: %+v", st)
	}
	res, err := e.run(alice, e.stay, "")
	if err != nil || res.Execution.Attempt != 2 || res.Step.Status != domain.StepCompleted {
		t.Fatalf("run again: %+v %v", res, err)
	}
	hist := e.store.executions(e.stay.ID)
	if hist[0].ID != stuck.ID || hist[0].Status != domain.ExecutionFailed || hist[0].ErrorCode != domain.ErrorCodeAbandoned {
		t.Fatalf("the silent attempt is recorded as abandoned: %+v", hist[0])
	}
}

func TestExecute_ALateOutcomeOfAnAbandonedAttemptIsNotRecorded(t *testing.T) {
	e := newEnv(t)
	stuck, _, _ := e.store.BeginExecution(context.Background(), domain.BeginExecutionInput{MissionID: e.mission.ID, StepID: e.stay.ID, ActionType: domain.ActionSearchListings, Now: e.clock, StaleAfter: time.Minute})
	e.clock += 600
	if _, err := e.run(alice, e.stay, ""); err != nil { // gives up on the stuck one and completes attempt 2
		t.Fatal(err)
	}
	late := *stuck
	late.Status, late.ResultReference = domain.ExecutionCompleted, "x"
	if err := e.store.FinishExecution(context.Background(), &late, e.clock); !errors.Is(err, domain.ErrExecutionSuperseded) {
		t.Fatalf("a late outcome = %v, want ErrExecutionSuperseded", err)
	}
	if st := e.store.step(e.stay.ID); st.Status != domain.StepCompleted || st.ResultRef == "x" {
		t.Fatalf("the later attempt stands: %+v", st)
	}
}

func TestExecute_StepsThatCannotBeRunYet(t *testing.T) {
	e := newEnv(t)
	for name, step := range map[string]*domain.MissionStep{
		"no vertical of its own": e.activities, "added by hand, no action": e.hand,
	} {
		if _, err := e.run(alice, step, ""); !errors.Is(err, domain.ErrStepNotExecutable) {
			t.Errorf("%s = %v, want ErrStepNotExecutable", name, err)
		}
	}
	// An availability check waits for the search it depends on: it is not run, and not asked of the runtime.
	if _, err := e.run(alice, e.availability, ""); !errors.Is(err, domain.ErrStepWaiting) {
		t.Errorf("availability before its search = %v, want ErrStepWaiting", err)
	}
	// A step with a mapping, whose action the runtime does not have.
	e.rt.unsupport[domain.ActionSearchListings] = true
	if _, err := e.run(alice, e.stay, ""); !errors.Is(err, domain.ErrStepNotExecutable) {
		t.Fatalf("an unsupported action = %v", err)
	}
	if e.rt.callCount() != 0 {
		t.Fatal("nothing unsupported reaches the runtime")
	}
	for _, id := range []string{e.activities.ID, e.availability.ID, e.hand.ID, e.stay.ID} {
		if len(e.store.executions(id)) != 0 || e.store.step(id).Status != domain.StepPending {
			t.Fatalf("a step that cannot run is left as it was: %s", id)
		}
	}
	// A step with a mapping for another kind of mission is not run with a guess.
	wrong := e.addStep(e.mission, "FIND_CATERING", domain.ActionSearchListings, 1, 9) // catering is an EVENT step; this mission is a trip
	e.rt.unsupport[domain.ActionSearchListings] = false
	if _, err := e.run(alice, wrong, ""); !errors.Is(err, domain.ErrStepNotExecutable) {
		t.Fatalf("catering in a trip = %v", err)
	}
}

func TestExecute_MissionAndPlanStateAreRespected(t *testing.T) {
	for _, status := range []domain.MissionStatus{domain.MissionPaused, domain.MissionCompleted, domain.MissionCancelled} {
		e := newEnv(t)
		e.store.missions[e.mission.ID].Status = status
		if _, err := e.run(alice, e.stay, ""); !errors.Is(err, domain.ErrMissionNotRunnable) {
			t.Errorf("%s mission = %v, want ErrMissionNotRunnable", status, err)
		}
		if e.rt.callCount() != 0 {
			t.Errorf("%s mission ran something", status)
		}
	}
	for _, status := range []domain.MissionStatus{domain.MissionDraft, domain.MissionActive} {
		e := newEnv(t)
		e.store.missions[e.mission.ID].Status = status
		if _, err := e.run(alice, e.stay, ""); err != nil {
			t.Errorf("%s mission = %v", status, err)
		}
	}
	// The plan was replaced: the old plan's steps are history.
	e := newEnv(t)
	e.store.current[e.mission.ID] = 2
	if _, err := e.run(alice, e.stay, ""); !errors.Is(err, domain.ErrStepNotRunnable) {
		t.Fatalf("a step of a replaced plan = %v", err)
	}
	// Skipped steps are not run either.
	e = newEnv(t)
	e.store.steps[e.stay.ID].Status = domain.StepSkipped
	if _, err := e.run(alice, e.stay, ""); !errors.Is(err, domain.ErrStepNotRunnable) {
		t.Fatalf("a skipped step = %v", err)
	}
}

func TestAnnotate_SaysWhatCanBeRunAndHowItWent(t *testing.T) {
	e := newEnv(t)
	steps := []domain.MissionStep{*e.stay, *e.activities, *e.availability, *e.hand}
	st, err := e.ex.Annotate(context.Background(), e.mission, steps)
	if err != nil {
		t.Fatal(err)
	}
	if !st[e.stay.ID].Executable || !st[e.stay.ID].CanExecute || st[e.stay.ID].Execution != nil {
		t.Fatalf("stay %+v", st[e.stay.ID])
	}
	for _, id := range []string{e.activities.ID, e.hand.ID} {
		if st[id].Executable || st[id].CanExecute {
			t.Fatalf("%s is not executable yet", id)
		}
	}
	// The availability check has an action behind it, but waits for its search: it cannot be run yet.
	if a := st[e.availability.ID]; !a.Executable || a.CanExecute || a.Flow != domain.FlowPending || a.Reason != domain.ReasonWaiting {
		t.Fatalf("availability waits for its search: %+v", a)
	}
	if a := st[e.stay.ID]; a.Flow != domain.FlowReady || len(a.DependsOn) != 0 {
		t.Fatalf("the search is ready: %+v", a)
	}
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		return failed("ACTION_FAILED"), nil
	}
	_, _ = e.run(alice, e.stay, "")
	st, _ = e.ex.Annotate(context.Background(), e.mission, []domain.MissionStep{e.store.step(e.stay.ID)})
	if s := st[e.stay.ID]; !s.CanExecute || s.Execution == nil || s.Execution.Status != domain.ExecutionFailed {
		t.Fatalf("a failed step can be retried: %+v", s)
	}
	e.rt.do = nil
	_, _ = e.run(alice, e.stay, "")
	st, _ = e.ex.Annotate(context.Background(), e.mission, []domain.MissionStep{e.store.step(e.stay.ID)})
	if s := st[e.stay.ID]; s.CanExecute || !s.Executable || s.Execution.Status != domain.ExecutionCompleted {
		t.Fatalf("a completed step cannot be run again: %+v", s)
	}
	e.store.missions[e.mission.ID].Status = domain.MissionPaused
	m, _ := e.store.Get(context.Background(), e.mission.ID)
	st, _ = e.ex.Annotate(context.Background(), m, []domain.MissionStep{*e.activities, *e.stay})
	if st[e.stay.ID].CanExecute {
		t.Fatal("a paused mission runs nothing")
	}
}

func TestExecute_IsTraceableInTheLogs(t *testing.T) {
	var buf bytes.Buffer
	e := newEnv(t)
	e.ex = New(e.store, e.store, e.store, e.store, e.rt, zerolog.New(&buf)).WithClock(func() time.Time { return time.Unix(e.clock, 0) })
	res, err := e.run(alice, e.stay, "")
	if err != nil {
		t.Fatal(err)
	}
	logs := buf.String()
	for _, want := range []string{`"mission_id":"` + e.mission.ID + `"`, `"step_id":"` + e.stay.ID + `"`, `"execution_id":"` + res.Execution.ID + `"`, `"action_type":"SEARCH_LISTINGS"`} {
		if !strings.Contains(logs, want) {
			t.Errorf("log lacks %s:\n%s", want, logs)
		}
	}
	if strings.Contains(logs, alice) {
		t.Errorf("the person's id is not logged by the executor:\n%s", logs)
	}
}

func TestSummarizeSearch(t *testing.T) {
	params := map[string]interface{}{"vertical": "HOTEL", "location": "Goma"}
	big := make([]domain.SearchResult, 25)
	for i := range big {
		big[i].ListingID = "l" + string(rune('a'+i))
	}
	got := summarizeSearch(map[string]interface{}{"data": map[string]interface{}{"items": big, "total": 120, "has_next": true}}, params)
	if got["total"] != 120 || len(got["listing_ids"].([]string)) != maxListed || got["has_more"] != true || got["vertical"] != "HOTEL" || got["location"] != "Goma" {
		t.Fatalf("bounded references: %v", got)
	}
	// A cached answer comes back with generic values.
	cached := summarizeSearch(map[string]interface{}{"data": map[string]interface{}{
		"items": []interface{}{map[string]interface{}{"ListingID": "x1"}, map[string]interface{}{"listing_id": "x2"}}, "total": float64(2),
	}}, map[string]interface{}{})
	if cached["total"] != 2 || len(cached["listing_ids"].([]string)) != 2 {
		t.Fatalf("cached shape: %v", cached)
	}
	if empty := summarizeSearch(map[string]interface{}{}, map[string]interface{}{}); empty["total"] != 0 || len(empty["listing_ids"].([]string)) != 0 {
		t.Fatalf("empty: %v", empty)
	}
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Title") || len(raw) > 500 {
		t.Fatalf("the summary is small and holds no commerce data: %s", raw)
	}
}

// The mission layer has no credit accounting of its own: nothing in the package reads or writes credits.
func TestExecutorHasNoCreditAccountingOfItsOwn(t *testing.T) {
	files, _ := filepath.Glob("*.go")
	forbidden := []string{"UsageService", "CreditAccount", "UsageReservation", "UsagePolicy", "usageService", ".Reserve(", ".Consume(", ".Release(", "Grant("}
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
				t.Errorf("%s mentions %q: credits belong to the Commerce Action Runtime", f, w)
			}
		}
	}
}
