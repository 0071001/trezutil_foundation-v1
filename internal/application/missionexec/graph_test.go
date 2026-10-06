package missionexec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Phase 4 at the step level: a step waits for the steps it depends on, takes what it needs from them by reference,
 * and the state of every step (pending, ready, running, completed, failed, blocked, skipped) is derived from the
 * stored statuses and the plan's dependencies.
 */

// availabilitySuccess is what the availability handler answers through the runtime.
func availabilitySuccess(available bool) *commerce.ExecuteResult {
	return &commerce.ExecuteResult{
		Status: domain.ActionResultSuccess, MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{"data": map[string]interface{}{
			"available": available, "capacity": 4, "remaining": 3, "reason": "", "start_at": int64(1), "end_at": int64(2),
		}},
	}
}

// routeByAction answers a search with `found` listings and an availability check with `available`.
func routeByAction(found int, available bool) func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
	return func(_ context.Context, a *domain.CommerceAction, _ domain.CommerceActor) (*commerce.ExecuteResult, error) {
		if a.ActionType == domain.ActionCheckAvailability {
			return availabilitySuccess(available), nil
		}
		return searchSuccess(found), nil
	}
}

func (e *env) withDates() {
	m := e.store.missions[e.mission.ID]
	m.StartDate, m.Participants = base.Unix()+30*86400, 2
}

func flowOf(t *testing.T, e *env) map[string]domain.FlowStep {
	t.Helper()
	m, _ := e.store.Get(context.Background(), e.mission.ID)
	f, err := e.ex.Flow(context.Background(), m)
	if err != nil {
		t.Fatalf("flow: %v", err)
	}
	out := map[string]domain.FlowStep{}
	for _, s := range f.Steps {
		out[s.Step.ID] = s
	}
	return out
}

func TestAvailabilityTakesItsListingFromTheSearchAndTheDatesFromTheMission(t *testing.T) {
	e := newEnv(t)
	e.withDates()
	e.rt.do = routeByAction(3, true)
	if _, err := e.run(alice, e.stay, ""); err != nil {
		t.Fatal(err)
	}
	res, err := e.run(alice, e.availability, "")
	if err != nil || res.Execution.Status != domain.ExecutionCompleted || res.Step.Status != domain.StepCompleted {
		t.Fatalf("availability: %+v %v", res, err)
	}
	var asked domain.CommerceAction
	for _, c := range e.rt.calls {
		if c.ActionType == domain.ActionCheckAvailability {
			asked = c
		}
	}
	p := asked.Parameters
	// The first listing the search found (its deterministic top result), the mission's dates and its head count.
	if p["listing_id"] != "listing-a" || p["start_at"] != base.Unix()+30*86400 {
		t.Fatalf("parameters %v", p)
	}
	if p["end_at"] != base.Unix()+31*86400 || p["party_size"] != 2 {
		t.Fatalf("a mission with only a start date is a one-day window: %v", p)
	}
	// What the step keeps is a small summary and a reference to the listing it checked, never the catalogue.
	if got := res.Execution.ResultPayload; got["available"] != true || got["listing_id"] != "listing-a" || got["remaining"] != int64(3) {
		t.Fatalf("payload %v", got)
	}
	if res.Execution.ResultReference != "listing-a" || res.Step.ResultRef != "listing-a" {
		t.Fatalf("the reference is the listing: %q %q", res.Execution.ResultReference, res.Step.ResultRef)
	}
}

func TestAvailabilityEndDateIsHonouredWhenItIsLaterThanOneDay(t *testing.T) {
	e := newEnv(t)
	e.withDates()
	e.store.missions[e.mission.ID].EndDate = base.Unix() + 35*86400
	e.rt.do = routeByAction(1, true)
	_, _ = e.run(alice, e.stay, "")
	if _, err := e.run(alice, e.availability, ""); err != nil {
		t.Fatal(err)
	}
	for _, c := range e.rt.calls {
		if c.ActionType == domain.ActionCheckAvailability && c.Parameters["end_at"] != base.Unix()+35*86400 {
			t.Fatalf("end %v", c.Parameters["end_at"])
		}
	}
}

func TestAvailabilityIsNotRunWithAGuess(t *testing.T) {
	// No dates: the check is refused before the runtime, naming what is missing.
	e := newEnv(t)
	e.rt.do = routeByAction(2, true)
	if _, err := e.run(alice, e.stay, ""); err != nil {
		t.Fatal(err)
	}
	calls := e.rt.callCount()
	_, err := e.run(alice, e.availability, "")
	if !errors.Is(err, domain.ErrStepInputMissing) || !strings.Contains(err.Error(), "dates") {
		t.Fatalf("no dates = %v", err)
	}
	if e.rt.callCount() != calls || len(e.store.executions(e.availability.ID)) != 0 || e.store.step(e.availability.ID).Status != domain.StepPending {
		t.Fatal("a step that is missing something is left as it was and never reaches the runtime")
	}

	// Dates, but the search found nothing to check.
	e = newEnv(t)
	e.withDates()
	e.rt.do = routeByAction(0, true)
	_, _ = e.run(alice, e.stay, "")
	if _, err := e.run(alice, e.availability, ""); !errors.Is(err, domain.ErrStepInputMissing) || !strings.Contains(err.Error(), "listing") {
		t.Fatalf("no listing = %v", err)
	}
}

func TestAStepDoesNotRunUntilItsDependenciesAreCompleted(t *testing.T) {
	for _, status := range []domain.MissionStepStatus{domain.StepPending, domain.StepInProgress, domain.StepFailed, domain.StepSkipped} {
		e := newEnv(t)
		e.withDates()
		e.store.steps[e.stay.ID].Status = status
		if _, err := e.run(alice, e.availability, ""); !errors.Is(err, domain.ErrStepWaiting) {
			t.Errorf("dependency %s = %v, want ErrStepWaiting", status, err)
		}
		if e.rt.callCount() != 0 {
			t.Errorf("dependency %s: nothing reaches the runtime", status)
		}
	}
}

func TestFlowDerivesTheStateOfEveryStep(t *testing.T) {
	e := newEnv(t)
	e.rt.do = routeByAction(2, true)
	f := flowOf(t, e)
	// A plan of three steps; the hand-added step is not part of the plan's graph.
	if len(f) != 3 || f[e.hand.ID].Step.ID != "" {
		t.Fatalf("the current plan's steps only: %d", len(f))
	}
	if s := f[e.stay.ID].State; s.Flow != domain.FlowReady || !s.CanExecute || len(s.DependsOn) != 0 {
		t.Fatalf("search: %+v", s)
	}
	if s := f[e.activities.ID].State; s.Flow != domain.FlowPending || s.Reason != domain.ReasonNotExecutable || s.CanExecute {
		t.Fatalf("a step with nothing executable behind it: %+v", s)
	}
	if s := f[e.availability.ID].State; s.Flow != domain.FlowPending || s.Reason != domain.ReasonWaiting || len(s.DependsOn) != 1 || s.DependsOn[0] != e.stay.ID {
		t.Fatalf("availability waits for the search: %+v", s)
	}

	// The search completed, the mission has no dates: the check is blocked on an input, naming it.
	if _, err := e.run(alice, e.stay, ""); err != nil {
		t.Fatal(err)
	}
	f = flowOf(t, e)
	if s := f[e.stay.ID].State; s.Flow != domain.FlowCompleted || s.CanExecute {
		t.Fatalf("completed: %+v", s)
	}
	if s := f[e.availability.ID].State; s.Flow != domain.FlowBlocked || s.Reason != domain.ReasonNeedsInput || s.Detail != "dates" || s.CanExecute {
		t.Fatalf("no dates: %+v", s)
	}

	// With dates it is ready, then completed.
	e.withDates()
	if s := flowOf(t, e)[e.availability.ID].State; s.Flow != domain.FlowReady || !s.CanExecute {
		t.Fatalf("dates known: %+v", s)
	}
	if _, err := e.run(alice, e.availability, ""); err != nil {
		t.Fatal(err)
	}
	if s := flowOf(t, e)[e.availability.ID].State; s.Flow != domain.FlowCompleted {
		t.Fatalf("done: %+v", s)
	}
}

func TestFlowBlocksWhatDependsOnAStepThatFailedWasBlockedOrSkipped(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(e *env)
		stay   domain.StepFlowState
		reason string
	}{
		{"failed", func(e *env) {
			e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
				return failed("ACTION_FAILED"), nil
			}
			_, _ = e.run(alice, e.stay, "")
		},
			domain.FlowFailed, domain.ReasonDependencyFailed},
		{"skipped", func(e *env) { e.store.steps[e.stay.ID].Status = domain.StepSkipped }, domain.FlowSkipped, domain.ReasonDependencySkipped},
		{"needs confirmation", func(e *env) {
			e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
				return &commerce.ExecuteResult{Status: domain.ActionResultRequiresConfirmation, RequiresConfirmation: true}, nil
			}
			_, _ = e.run(alice, e.stay, "")
		}, domain.FlowBlocked, domain.ReasonDependencyBlocked},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newEnv(t)
			e.withDates()
			c.setup(e)
			f := flowOf(t, e)
			if s := f[e.stay.ID].State; s.Flow != c.stay {
				t.Fatalf("the step itself: %+v", s)
			}
			s := f[e.availability.ID].State
			if s.Flow != domain.FlowBlocked || s.Reason != c.reason || len(s.BlockedBy) != 1 || s.BlockedBy[0] != e.stay.ID || s.CanExecute {
				t.Fatalf("its dependent is blocked, never run: %+v", s)
			}
		})
	}
}

func TestAConfirmationIsNeverGivenOnThePersonsBehalf(t *testing.T) {
	e := newEnv(t)
	e.rt.do = func(context.Context, *domain.CommerceAction, domain.CommerceActor) (*commerce.ExecuteResult, error) {
		return &commerce.ExecuteResult{Status: domain.ActionResultRequiresConfirmation, RequiresConfirmation: true}, nil
	}
	res, err := e.run(alice, e.stay, "")
	if err != nil || res.Execution.ErrorCode != domain.ErrorCodeConfirmationRequired {
		t.Fatalf("%+v %v", res, err)
	}
	if s := flowOf(t, e)[e.stay.ID].State; s.Flow != domain.FlowBlocked || s.Reason != domain.ReasonNeedsConfirmation || s.CanExecute {
		t.Fatalf("a step that needs the person is blocked and cannot be run by itself: %+v", s)
	}
	for _, c := range e.rt.calls {
		if c.IsConfirmed() {
			t.Fatal("nothing is confirmed on the person's behalf")
		}
	}
}

func TestFlowSeesAnAttemptThatWentSilent(t *testing.T) {
	e := newEnv(t)
	if _, _, err := e.store.BeginExecution(context.Background(), domain.BeginExecutionInput{MissionID: e.mission.ID, StepID: e.stay.ID, ActionType: domain.ActionSearchListings, Now: e.clock, StaleAfter: domain.DefaultExecutionStaleAfter}); err != nil {
		t.Fatal(err)
	}
	if s := flowOf(t, e)[e.stay.ID].State; s.Flow != domain.FlowRunning || s.CanExecute {
		t.Fatalf("an attempt under way: %+v", s)
	}
	e.clock += 600
	if s := flowOf(t, e)[e.stay.ID].State; s.Flow != domain.FlowFailed || s.Reason != domain.ReasonAbandoned || !s.Stale || !s.CanExecute {
		t.Fatalf("a silent attempt is failed and can be run again: %+v", s)
	}
}

func TestFlowRefusesAGraphThatCannotBeOrdered(t *testing.T) {
	e := newEnv(t)
	// The search would wait for the availability check, which waits for the search.
	e.store.deps = append(e.store.deps, domain.StepDependency{StepID: e.stay.ID, DependsOnID: e.availability.ID})
	m, _ := e.store.Get(context.Background(), e.mission.ID)
	if _, err := e.ex.Flow(context.Background(), m); !errors.Is(err, domain.ErrDependencyCycle) {
		t.Fatalf("a cycle = %v", err)
	}
	// Listing the steps does not fail because of it; they are shown without states and cannot be run.
	st, err := e.ex.Annotate(context.Background(), m, []domain.MissionStep{*e.stay, *e.availability})
	if err != nil || st[e.stay.ID].Flow != "" || st[e.stay.ID].CanExecute || st[e.availability.ID].CanExecute {
		t.Fatalf("annotate with a cycle: %+v %v", st, err)
	}
	// A step that depends on itself.
	e = newEnv(t)
	m, _ = e.store.Get(context.Background(), e.mission.ID)
	e.store.deps = append(e.store.deps, domain.StepDependency{StepID: e.stay.ID, DependsOnID: e.stay.ID})
	if _, err := e.ex.Flow(context.Background(), m); !errors.Is(err, domain.ErrDependencyInvalid) {
		t.Fatalf("a self dependency = %v", err)
	}
}

func TestFlowOfAMissionWithoutAPlan(t *testing.T) {
	e := newEnv(t)
	delete(e.store.current, e.mission.ID)
	m, _ := e.store.Get(context.Background(), e.mission.ID)
	if _, err := e.ex.Flow(context.Background(), m); !errors.Is(err, domain.ErrNoPlanToRun) {
		t.Fatalf("no plan = %v", err)
	}
}

func TestAHandAddedStepKeepsItsPhase3Behaviour(t *testing.T) {
	// A step added by hand has no dependencies and no flow state: it runs as in phase 3.
	e := newEnv(t)
	hand := e.addStep(e.mission, "FIND_ACCOMMODATION", domain.ActionSearchListings, 0, 5)
	m, _ := e.store.Get(context.Background(), e.mission.ID)
	st, err := e.ex.Annotate(context.Background(), m, []domain.MissionStep{*hand})
	if err != nil || st[hand.ID].Flow != "" || !st[hand.ID].CanExecute {
		t.Fatalf("hand-added: %+v %v", st[hand.ID], err)
	}
	if res, err := e.run(alice, hand, ""); err != nil || res.Execution.Status != domain.ExecutionCompleted {
		t.Fatalf("run: %v", err)
	}
}
