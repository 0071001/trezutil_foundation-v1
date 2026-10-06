package domain

import (
	"errors"
	"fmt"
	"testing"
)

func gstep(id string, pos int, status MissionStepStatus) MissionStep {
	return MissionStep{ID: id, MissionID: "m", Position: pos, Status: status, Type: "T"}
}

func dep(step, on string) StepDependency { return StepDependency{StepID: step, DependsOnID: on} }

func flowByID(t *testing.T, steps []MissionStep, deps []StepDependency, in FlowInputs) map[string]MissionStepState {
	t.Helper()
	states, err := ComputeFlow(steps, deps, in)
	if err != nil {
		t.Fatalf("ComputeFlow: %v", err)
	}
	out := map[string]MissionStepState{}
	for i := range steps {
		out[steps[i].ID] = states[i]
	}
	return out
}

// everything runs and nothing is missing.
var plainInputs = FlowInputs{Executable: func(*MissionStep) bool { return true }}

func TestValidateGraph(t *testing.T) {
	steps := []MissionStep{gstep("a", 1, StepPending), gstep("b", 2, StepPending), gstep("c", 3, StepPending)}
	cases := []struct {
		name string
		deps []StepDependency
		want error
	}{
		{"none", nil, nil},
		{"a chain", []StepDependency{dep("b", "a"), dep("c", "b")}, nil},
		{"a diamond", []StepDependency{dep("b", "a"), dep("c", "a"), dep("c", "b")}, nil},
		{"itself", []StepDependency{dep("a", "a")}, ErrDependencyInvalid},
		{"an unknown step", []StepDependency{dep("a", "zzz")}, ErrDependencyInvalid},
		{"from an unknown step", []StepDependency{dep("zzz", "a")}, ErrDependencyInvalid},
		{"repeated", []StepDependency{dep("b", "a"), dep("b", "a")}, ErrDependencyInvalid},
		{"empty ids", []StepDependency{dep("", "a")}, ErrDependencyInvalid},
		{"two steps waiting for each other", []StepDependency{dep("a", "b"), dep("b", "a")}, ErrDependencyCycle},
		{"a cycle of three", []StepDependency{dep("a", "c"), dep("b", "a"), dep("c", "b")}, ErrDependencyCycle},
		{"a cycle behind a valid step", []StepDependency{dep("b", "a"), dep("c", "b"), dep("b", "c")}, ErrDependencyCycle},
	}
	for _, c := range cases {
		err := ValidateGraph(steps, c.deps)
		if c.want == nil && err != nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}

func TestComputeFlow_RefusesAnInvalidGraphBeforeAnythingElse(t *testing.T) {
	steps := []MissionStep{gstep("a", 1, StepPending), gstep("b", 2, StepPending)}
	if _, err := ComputeFlow(steps, []StepDependency{dep("a", "b"), dep("b", "a")}, plainInputs); !errors.Is(err, ErrDependencyCycle) {
		t.Fatalf("cycle = %v", err)
	}
}

func TestComputeFlow_StatesOfAChain(t *testing.T) {
	steps := []MissionStep{gstep("a", 1, StepPending), gstep("b", 2, StepPending), gstep("c", 3, StepPending)}
	deps := []StepDependency{dep("b", "a"), dep("c", "b")}

	f := flowByID(t, steps, deps, plainInputs)
	if f["a"].Flow != FlowReady || f["b"].Flow != FlowPending || f["c"].Flow != FlowPending || f["b"].Reason != ReasonWaiting {
		t.Fatalf("nothing started: %+v", f)
	}

	steps[0].Status = StepInProgress
	f = flowByID(t, steps, deps, plainInputs)
	if f["a"].Flow != FlowRunning || f["b"].Flow != FlowPending {
		t.Fatalf("a running: %+v", f)
	}

	steps[0].Status = StepCompleted
	f = flowByID(t, steps, deps, plainInputs)
	if f["a"].Flow != FlowCompleted || f["b"].Flow != FlowReady || f["c"].Flow != FlowPending {
		t.Fatalf("a completed unlocks b only: %+v", f)
	}
	if got := f["b"].DependsOn; len(got) != 1 || got[0] != "a" {
		t.Fatalf("depends on: %v", got)
	}
}

func TestComputeFlow_OnlyCompletedUnlocks(t *testing.T) {
	for _, c := range []struct {
		status MissionStepStatus
		want   StepFlowState
		reason string
	}{
		{StepFailed, FlowBlocked, ReasonDependencyFailed},
		{StepSkipped, FlowBlocked, ReasonDependencySkipped},
		{StepInProgress, FlowPending, ReasonWaiting},
		{StepPending, FlowPending, ReasonWaiting},
	} {
		steps := []MissionStep{gstep("a", 1, c.status), gstep("b", 2, StepPending)}
		f := flowByID(t, steps, []StepDependency{dep("b", "a")}, plainInputs)
		if f["b"].Flow != c.want || f["b"].Reason != c.reason {
			t.Errorf("dependency %s: %+v", c.status, f["b"])
		}
		if c.want == FlowBlocked && (len(f["b"].BlockedBy) != 1 || f["b"].BlockedBy[0] != "a") {
			t.Errorf("dependency %s: blocked by %v", c.status, f["b"].BlockedBy)
		}
	}
}

func TestComputeFlow_BlockedPropagatesDownTheChain(t *testing.T) {
	steps := []MissionStep{gstep("a", 1, StepFailed), gstep("b", 2, StepPending), gstep("c", 3, StepPending), gstep("free", 4, StepPending)}
	f := flowByID(t, steps, []StepDependency{dep("b", "a"), dep("c", "b")}, plainInputs)
	if f["a"].Flow != FlowFailed || f["b"].Flow != FlowBlocked || f["b"].Reason != ReasonDependencyFailed {
		t.Fatalf("direct dependent: %+v", f)
	}
	if f["c"].Flow != FlowBlocked || f["c"].Reason != ReasonDependencyBlocked || f["c"].BlockedBy[0] != "b" {
		t.Fatalf("the dependent of a blocked step is blocked too: %+v", f["c"])
	}
	if f["free"].Flow != FlowReady {
		t.Fatalf("an independent step is unaffected: %+v", f["free"])
	}
}

func TestComputeFlow_ADiamondWaitsForBothBranches(t *testing.T) {
	steps := []MissionStep{gstep("root", 1, StepCompleted), gstep("left", 2, StepCompleted), gstep("right", 3, StepPending), gstep("join", 4, StepPending)}
	deps := []StepDependency{dep("left", "root"), dep("right", "root"), dep("join", "left"), dep("join", "right")}
	f := flowByID(t, steps, deps, plainInputs)
	if f["right"].Flow != FlowReady || f["join"].Flow != FlowPending {
		t.Fatalf("join waits for the right branch: %+v", f)
	}
	steps[2].Status = StepCompleted
	if f = flowByID(t, steps, deps, plainInputs); f["join"].Flow != FlowReady {
		t.Fatalf("join is ready once both are done: %+v", f["join"])
	}
}

func TestComputeFlow_FailedStepsAndTheQuestionsTheExecutorAnswers(t *testing.T) {
	steps := []MissionStep{gstep("a", 1, StepFailed), gstep("conf", 2, StepFailed), gstep("old", 3, StepInProgress), gstep("live", 4, StepInProgress)}
	in := FlowInputs{
		NeedsConfirmation: func(s *MissionStep) bool { return s.ID == "conf" },
		Stale:             func(s *MissionStep) bool { return s.ID == "old" },
	}
	f := flowByID(t, steps, nil, in)
	if f["a"].Flow != FlowFailed {
		t.Fatalf("failed: %+v", f["a"])
	}
	if f["conf"].Flow != FlowBlocked || f["conf"].Reason != ReasonNeedsConfirmation {
		t.Fatalf("a confirmation is the person's: %+v", f["conf"])
	}
	if f["old"].Flow != FlowFailed || f["old"].Reason != ReasonAbandoned || !f["old"].Stale {
		t.Fatalf("a silent attempt: %+v", f["old"])
	}
	if f["live"].Flow != FlowRunning {
		t.Fatalf("running: %+v", f["live"])
	}
}

func TestComputeFlow_MissingInputBlocksAndNonExecutableWaits(t *testing.T) {
	steps := []MissionStep{gstep("search", 1, StepCompleted), gstep("check", 2, StepPending), gstep("manual", 3, StepPending)}
	deps := []StepDependency{dep("check", "search")}
	in := FlowInputs{
		Executable:   func(s *MissionStep) bool { return s.ID != "manual" },
		MissingInput: func(s *MissionStep) string { return map[string]string{"check": "dates"}[s.ID] },
	}
	f := flowByID(t, steps, deps, in)
	if f["check"].Flow != FlowBlocked || f["check"].Reason != ReasonNeedsInput || f["check"].Detail != "dates" {
		t.Fatalf("missing input: %+v", f["check"])
	}
	if f["manual"].Flow != FlowPending || f["manual"].Reason != ReasonNotExecutable {
		t.Fatalf("nothing executable behind it: %+v", f["manual"])
	}
	// Input is asked only once the dependencies are done: a waiting step is just waiting.
	steps[0].Status = StepPending
	if f = flowByID(t, steps, deps, in); f["check"].Flow != FlowPending || f["check"].Reason != ReasonWaiting {
		t.Fatalf("waiting: %+v", f["check"])
	}
}

func TestComputeFlow_OrderDoesNotDependOnTheOrderGiven(t *testing.T) {
	// A step listed before the one it depends on still gets the right state.
	steps := []MissionStep{gstep("b", 2, StepPending), gstep("a", 1, StepCompleted)}
	f := flowByID(t, steps, []StepDependency{dep("b", "a")}, plainInputs)
	if f["b"].Flow != FlowReady {
		t.Fatalf("%+v", f["b"])
	}
}

func TestComputeFlow_ALongChainIsNotQuadratic(t *testing.T) {
	const n = 2000
	steps := make([]MissionStep, n)
	var deps []StepDependency
	for i := range steps {
		steps[i] = gstep(fmt.Sprintf("s%04d", i), i+1, StepPending)
		if i > 0 {
			deps = append(deps, dep(steps[i].ID, steps[i-1].ID))
		}
	}
	steps[0].Status = StepCompleted
	f := flowByID(t, steps, deps, plainInputs)
	if f["s0001"].Flow != FlowReady || f["s1999"].Flow != FlowPending {
		t.Fatal("a long chain resolves")
	}
}

func TestMissionFlow_OutcomeAndProgress(t *testing.T) {
	mk := func(flows ...StepFlowState) *MissionFlow {
		f := &MissionFlow{PlanVersion: 1}
		for i, s := range flows {
			f.Steps = append(f.Steps, FlowStep{Step: gstep(fmt.Sprintf("s%d", i), i+1, StepPending), State: MissionStepState{Flow: s}})
		}
		return f
	}
	cases := []struct {
		name string
		flow *MissionFlow
		busy bool
		want MissionOutcome
	}{
		{"empty", mk(), false, OutcomeEmpty},
		{"all completed", mk(FlowCompleted, FlowCompleted), false, OutcomeCompleted},
		{"ready", mk(FlowCompleted, FlowReady), false, OutcomeReady},
		{"waiting", mk(FlowCompleted, FlowPending), false, OutcomeWaiting},
		{"running", mk(FlowCompleted, FlowRunning, FlowPending), false, OutcomeInProgress},
		{"another run holds it", mk(FlowReady), true, OutcomeInProgress},
		{"a failure needs the person", mk(FlowCompleted, FlowFailed, FlowBlocked), false, OutcomeNeedsAttention},
		{"blocked needs the person", mk(FlowBlocked), false, OutcomeNeedsAttention},
		{"a step set aside does not complete the mission", mk(FlowCompleted, FlowSkipped), false, OutcomeNeedsAttention},
		{"failure wins over what is ready", mk(FlowFailed, FlowReady), false, OutcomeNeedsAttention},
		{"running wins over a failure", mk(FlowFailed, FlowRunning), false, OutcomeInProgress},
	}
	for _, c := range cases {
		if got := c.flow.Outcome(c.busy); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
	p := mk(FlowCompleted, FlowCompleted, FlowSkipped, FlowReady).Progress()
	if p.Total != 4 || p.Completed != 2 || p.Skipped != 1 {
		t.Fatalf("progress %+v", p)
	}
}
