package domain

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

/*
 * The execution graph of a mission (Organiser, phase 4). A step may depend on other steps of the same plan: it cannot
 * run until each of them has COMPLETED. From the stored step statuses and those dependencies, a pure function derives
 * what each step is: PENDING (waiting), READY, RUNNING, COMPLETED, FAILED, BLOCKED or SKIPPED. READY and BLOCKED are
 * derived, never stored: the stored statuses stay the five of the step lifecycle (pending, in_progress, completed,
 * skipped, failed), so there is one source of truth and nothing to keep in step with it.
 *
 * Nothing here runs anything. The MissionOrchestrator reads this graph and asks the MissionStepExecutor to run what is
 * READY; the Commerce Action Runtime stays the only place where commerce actions run and credits are handled.
 */

// StepDependency says DependsOnID must be COMPLETED before StepID may run. Both are steps of one plan of one mission.
type StepDependency struct {
	StepID      string
	DependsOnID string
}

// Dependency errors.
var (
	// ErrDependencyInvalid: a dependency refers to itself, repeats, or names a step that is not in the plan.
	ErrDependencyInvalid = fmt.Errorf("invalid step dependency")
	// ErrDependencyCycle: the dependencies form a cycle, so the plan could never be executed.
	ErrDependencyCycle = fmt.Errorf("the step dependencies form a cycle")
	// ErrNoPlanToRun: the mission has no plan whose steps could be run.
	ErrNoPlanToRun = fmt.Errorf("the mission has no plan to run")
	// ErrStepWaiting: the step depends on steps that have not completed.
	ErrStepWaiting = fmt.Errorf("the step is waiting for the steps it depends on")
	// ErrStepInputMissing: the step cannot run because something it needs is not known (the mission's dates, a result).
	ErrStepInputMissing = fmt.Errorf("the step is missing something it needs")
	// ErrMissionBusy: another orchestration of the mission is running.
	ErrMissionBusy = fmt.Errorf("the mission is being run")
)

// ErrorCodeConfirmationRequired: the runtime stopped the action before running it because it needs the person's
// explicit confirmation. A step does not handle confirmation (nothing is confirmed on the person's behalf), so the step
// is BLOCKED for the person to deal with; it is never retried by itself.
const ErrorCodeConfirmationRequired = "CONFIRMATION_REQUIRED"

// StepFlowState is what a step is, derived from its status and its dependencies.
type StepFlowState string

const (
	// FlowPending: waiting, or nothing can run it yet (a step with nothing executable behind it).
	FlowPending StepFlowState = "pending"
	// FlowReady: every dependency is completed and the step can be run now.
	FlowReady StepFlowState = "ready"
	// FlowRunning: an attempt is in progress.
	FlowRunning StepFlowState = "running"
	// FlowCompleted: done; final.
	FlowCompleted StepFlowState = "completed"
	// FlowFailed: the latest attempt failed (or went silent); it can be retried on request.
	FlowFailed StepFlowState = "failed"
	// FlowBlocked: it cannot run until something outside it is resolved: a dependency failed, was blocked or skipped, an
	// input is missing, or the action needs the person's confirmation.
	FlowBlocked StepFlowState = "blocked"
	// FlowSkipped: set aside on purpose; its dependents stay blocked.
	FlowSkipped StepFlowState = "skipped"
)

// Reasons a step is blocked or cannot run yet.
const (
	ReasonDependencyFailed  = "dependency_failed"
	ReasonDependencyBlocked = "dependency_blocked"
	ReasonDependencySkipped = "dependency_skipped"
	ReasonNeedsInput        = "needs_input"
	ReasonNeedsConfirmation = "needs_confirmation"
	ReasonNotExecutable     = "not_executable"
	ReasonWaiting           = "waiting"
	ReasonAbandoned         = "abandoned"
)

// FlowInputs are the questions the graph asks about a step, answered by the step executor (which knows what the runtime
// can run and what each action needs).
type FlowInputs struct {
	// Executable: the step has an executable action behind it that the runtime supports.
	Executable func(*MissionStep) bool
	// Stale: the step says in progress but its attempt has gone silent.
	Stale func(*MissionStep) bool
	// MissingInput: what the step needs that is not known ("dates", "listing"), "" when it has everything.
	MissingInput func(*MissionStep) string
	// NeedsConfirmation: the step's latest attempt was stopped by a confirmation requirement.
	NeedsConfirmation func(*MissionStep) bool
}

// ValidateGraph checks the dependencies of one plan: every one names two steps of the plan, none refers to itself, none
// repeats, and there is no cycle. It returns ErrDependencyInvalid or ErrDependencyCycle.
func ValidateGraph(steps []MissionStep, deps []StepDependency) error {
	known := make(map[string]bool, len(steps))
	for i := range steps {
		known[steps[i].ID] = true
	}
	seen := make(map[StepDependency]bool, len(deps))
	for _, d := range deps {
		switch {
		case d.StepID == "" || d.DependsOnID == "":
			return fmt.Errorf("%w: a dependency needs two steps", ErrDependencyInvalid)
		case d.StepID == d.DependsOnID:
			return fmt.Errorf("%w: a step cannot depend on itself", ErrDependencyInvalid)
		case !known[d.StepID] || !known[d.DependsOnID]:
			return fmt.Errorf("%w: a dependency names a step that is not in the plan", ErrDependencyInvalid)
		case seen[d]:
			return fmt.Errorf("%w: the same dependency is repeated", ErrDependencyInvalid)
		}
		seen[d] = true
	}
	if _, err := topologicalOrder(steps, deps); err != nil {
		return err
	}
	return nil
}

// topologicalOrder returns the steps so that each comes after the steps it depends on (ties by position, then id), or
// ErrDependencyCycle naming the steps that could not be placed.
func topologicalOrder(steps []MissionStep, deps []StepDependency) ([]int, error) {
	index := make(map[string]int, len(steps))
	for i := range steps {
		index[steps[i].ID] = i
	}
	remaining := make([]int, len(steps)) // unmet dependencies of each step
	dependents := make(map[string][]int)
	for _, d := range deps {
		remaining[index[d.StepID]]++
		dependents[d.DependsOnID] = append(dependents[d.DependsOnID], index[d.StepID])
	}
	less := func(a, b int) bool {
		if steps[a].Position != steps[b].Position {
			return steps[a].Position < steps[b].Position
		}
		return steps[a].ID < steps[b].ID
	}
	var queue []int
	for i := range steps {
		if remaining[i] == 0 {
			queue = append(queue, i)
		}
	}
	order := make([]int, 0, len(steps))
	for len(queue) > 0 {
		sort.Slice(queue, func(i, j int) bool { return less(queue[i], queue[j]) })
		next := queue[0]
		queue = queue[1:]
		order = append(order, next)
		for _, d := range dependents[steps[next].ID] {
			if remaining[d]--; remaining[d] == 0 {
				queue = append(queue, d)
			}
		}
	}
	if len(order) != len(steps) {
		var stuck []string
		for i := range steps {
			if remaining[i] > 0 {
				stuck = append(stuck, steps[i].ID)
			}
		}
		return nil, fmt.Errorf("%w: %s", ErrDependencyCycle, strings.Join(stuck, ", "))
	}
	return order, nil
}

// ComputeFlow derives, for each step of a plan, what it is now (see StepFlowState). It validates the graph first, so a
// plan with a cycle or a bad dependency is refused before anything can run. The steps come back in position order.
func ComputeFlow(steps []MissionStep, deps []StepDependency, in FlowInputs) ([]MissionStepState, error) {
	if err := ValidateGraph(steps, deps); err != nil {
		return nil, err
	}
	order, _ := topologicalOrder(steps, deps)
	index := make(map[string]int, len(steps))
	for i := range steps {
		index[steps[i].ID] = i
	}
	needs := make(map[string][]string)
	for _, d := range deps {
		needs[d.StepID] = append(needs[d.StepID], d.DependsOnID)
	}
	for id := range needs {
		sort.Slice(needs[id], func(a, b int) bool { return steps[index[needs[id][a]]].Position < steps[index[needs[id][b]]].Position })
	}
	out := make([]MissionStepState, len(steps))
	state := func(id string) StepFlowState { return out[index[id]].Flow }
	for _, i := range order {
		s := &steps[i]
		st := MissionStepState{DependsOn: needs[s.ID]}
		switch s.Status {
		case StepCompleted:
			st.Flow = FlowCompleted
		case StepSkipped:
			st.Flow = FlowSkipped
		case StepFailed:
			st.Flow = FlowFailed
			if in.NeedsConfirmation != nil && in.NeedsConfirmation(s) {
				st.Flow, st.Reason = FlowBlocked, ReasonNeedsConfirmation
			}
		case StepInProgress:
			st.Flow = FlowRunning
			if in.Stale != nil && in.Stale(s) {
				st.Flow, st.Reason, st.Stale = FlowFailed, ReasonAbandoned, true
			}
		default: // pending
			st.Flow, st.Reason = pendingFlow(s, needs[s.ID], state, in, &st)
		}
		out[i] = st
	}
	return out, nil
}

// pendingFlow decides what a not-yet-started step is.
func pendingFlow(s *MissionStep, needs []string, stateOf func(string) StepFlowState, in FlowInputs, st *MissionStepState) (StepFlowState, string) {
	var blockedBy []string
	var reason string
	allDone := true
	for _, id := range needs {
		switch stateOf(id) {
		case FlowCompleted:
		case FlowFailed:
			allDone, blockedBy, reason = false, append(blockedBy, id), firstNonEmpty(reason, ReasonDependencyFailed)
		case FlowBlocked:
			allDone, blockedBy, reason = false, append(blockedBy, id), firstNonEmpty(reason, ReasonDependencyBlocked)
		case FlowSkipped:
			allDone, blockedBy, reason = false, append(blockedBy, id), firstNonEmpty(reason, ReasonDependencySkipped)
		default: // pending, ready or running: it will come
			allDone = false
		}
	}
	if len(blockedBy) > 0 {
		st.BlockedBy = blockedBy
		return FlowBlocked, reason
	}
	if !allDone {
		return FlowPending, ReasonWaiting
	}
	if in.MissingInput != nil {
		if missing := in.MissingInput(s); missing != "" {
			st.Detail = missing
			return FlowBlocked, ReasonNeedsInput
		}
	}
	if in.Executable != nil && !in.Executable(s) {
		return FlowPending, ReasonNotExecutable
	}
	return FlowReady, ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// MissionOutcome is where a mission stands as a whole, derived from its steps. It is not a stored status: the mission
// keeps the lifecycle it already has (draft, active, paused, completed, cancelled). A mission with partial progress is
// told apart from a completed one by this, and by the counts.
type MissionOutcome string

const (
	// OutcomeEmpty: the plan has no steps.
	OutcomeEmpty MissionOutcome = "empty"
	// OutcomeCompleted: every step is completed.
	OutcomeCompleted MissionOutcome = "completed"
	// OutcomeInProgress: a step is running, or another orchestration holds the mission and there is still something to
	// do. (If every step is completed the outcome is completed even while the holder is still closing the mission.)
	OutcomeInProgress MissionOutcome = "in_progress"
	// OutcomeNeedsAttention: some step failed, is blocked or was set aside: the person has something to resolve; what
	// completed stays.
	OutcomeNeedsAttention MissionOutcome = "needs_attention"
	// OutcomeReady: nothing is wrong and some step can be run now.
	OutcomeReady MissionOutcome = "ready"
	// OutcomeWaiting: nothing is wrong, but what is left cannot be run yet (it waits for other steps, or has nothing
	// executable behind it).
	OutcomeWaiting MissionOutcome = "waiting"
)

// Progress counts the steps of a plan.
type Progress struct {
	Total, Completed, Skipped int
}

// FlowStep is a step of the plan with its state.
type FlowStep struct {
	Step  MissionStep
	State MissionStepState
}

// MissionFlow is the execution state of a mission's current plan.
type MissionFlow struct {
	PlanVersion int
	// Steps are in position order.
	Steps []FlowStep
}

// Progress counts the steps by what matters to the person.
func (f *MissionFlow) Progress() Progress {
	p := Progress{Total: len(f.Steps)}
	for _, s := range f.Steps {
		switch s.State.Flow {
		case FlowCompleted:
			p.Completed++
		case FlowSkipped:
			p.Skipped++
		}
	}
	return p
}

// Outcome derives the mission's outcome. busy is true when an orchestration holds the mission now.
func (f *MissionFlow) Outcome(busy bool) MissionOutcome {
	if len(f.Steps) == 0 {
		return OutcomeEmpty
	}
	// Every step is required: a mission is completed only when all of them are, so a step set aside (skipped) leaves the
	// mission needing the person's attention rather than counting as done. Optional steps are future work.
	var running, attention, ready, waiting, done int
	for _, s := range f.Steps {
		switch s.State.Flow {
		case FlowRunning:
			running++
		case FlowFailed, FlowBlocked, FlowSkipped:
			attention++
		case FlowReady:
			ready++
		case FlowPending:
			waiting++
		case FlowCompleted:
			done++
		}
	}
	switch {
	case done == len(f.Steps):
		return OutcomeCompleted
	case running > 0 || busy:
		return OutcomeInProgress
	case attention > 0:
		return OutcomeNeedsAttention
	case ready > 0:
		return OutcomeReady
	default:
		return OutcomeWaiting
	}
}

// UpstreamStep is a step a given step depends on, with the latest attempt to run it: what a later step may take from an
// earlier one (references and a small summary, never a copy of commerce data).
type UpstreamStep struct {
	Step      MissionStep
	Execution *MissionStepExecution
}

// MissionGraphRepository reads the dependencies of a mission's plan.
type MissionGraphRepository interface {
	// PlanDependencies returns the dependencies of one plan version of the mission.
	PlanDependencies(ctx context.Context, missionID string, planVersion int) ([]StepDependency, error)
	// Upstream returns the steps the step depends on, each with its latest execution (nil when it has none).
	Upstream(ctx context.Context, stepID string) ([]UpstreamStep, error)
}

// StepFlow derives the state of a mission's current plan (implemented by the step executor).
type StepFlow interface {
	// Flow returns the state of every step of the current plan; ErrNoPlanToRun when there is none, ErrDependencyInvalid or
	// ErrDependencyCycle when the stored graph cannot be ordered.
	Flow(ctx context.Context, m *Mission) (*MissionFlow, error)
}

// DefaultRunLease is how long an orchestration may hold a mission before another may take it over (the process died).
// It is renewed before each step, so it only has to outlast one step's time.
const DefaultRunLease = 5 * time.Minute

// MissionRunStore holds the lock that lets one orchestration at a time run a mission, and changes the mission's status
// from a known one to another (so a stale read cannot overwrite a change made meanwhile).
type MissionRunStore interface {
	// AcquireRun takes the mission's run lock for ttl; acquired=false when another orchestration holds it and its lease
	// has not run out. The token identifies the holder.
	AcquireRun(ctx context.Context, missionID, token string, now time.Time, ttl time.Duration) (acquired bool, err error)
	// ExtendRun renews the lease; ErrMissionBusy when the lock is no longer the holder's.
	ExtendRun(ctx context.Context, missionID, token string, now time.Time, ttl time.Duration) error
	// ReleaseRun gives the lock up (only the holder's own).
	ReleaseRun(ctx context.Context, missionID, token string) error
	// RunInProgress reports whether an orchestration holds the mission now (its lease has not run out).
	RunInProgress(ctx context.Context, missionID string, now time.Time) (bool, error)
	// TransitionMission moves the mission from one status to another; changed=false when it was not in `from`.
	TransitionMission(ctx context.Context, missionID string, from, to MissionStatus, now time.Time) (changed bool, err error)
}
