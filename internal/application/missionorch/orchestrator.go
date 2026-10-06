// Package missionorch runs the steps of a mission in the order of its dependency graph (the MissionOrchestrator).
//
// The orchestrator is deliberately thin and deliberately dumb. It reads the state of the mission's current plan (what
// is READY, BLOCKED, FAILED… as derived by domain.ComputeFlow), asks the MissionStepExecutor to run the next ready
// step, and looks again. It never talks to the Commerce Action Runtime, never searches, reserves, orders or pays, and
// never reads or writes a credit: the runtime does all of that, one step at a time, exactly as when a single step is
// run by hand. What the orchestrator adds is order (a step runs after the steps it depends on), memory (a completed
// step is never run again), safety (one orchestration of a mission at a time) and a safe reaction to failure (what
// depends on a failed step is blocked; what completed stays; nothing is rolled back and nothing is retried by itself).
//
// Steps are run one after another, in position order. Independent steps could be run side by side, but sequential is
// the safe default: the runtime's search and availability actions are cheap, and a person waiting for a mission gains
// little from saving a second at the price of a harder concurrency story.
package missionorch

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/domain"
)

// StepRunner is the part of the step executor the orchestrator uses (missionexec.Executor).
type StepRunner interface {
	// Execute runs one step of one of the actor's missions; the same step with the same key is the same execution.
	Execute(ctx context.Context, actorID, missionID, stepID, idempotencyKey string) (*domain.StepExecutionResult, error)
	// Flow returns the state of every step of the mission's current plan.
	Flow(ctx context.Context, m *domain.Mission) (*domain.MissionFlow, error)
}

// OnFailure is what the orchestration does when a step fails.
type OnFailure string

const (
	// StopOnFailure (the default) stops at the first failure: what depends on the failed step is blocked, nothing else
	// is started, and what already completed stays.
	StopOnFailure OnFailure = "stop"
	// ContinueIndependent carries on with the steps that do not depend on the failed one; its dependents stay blocked.
	ContinueIndependent OnFailure = "continue"
)

// IsValid reports whether the policy is one of the two.
func (p OnFailure) IsValid() bool { return p == StopOnFailure || p == ContinueIndependent }

// Options changes how one orchestration runs.
type Options struct {
	// RetryFailed also runs the steps whose latest attempt failed or went silent, once each in this call (a resume).
	// A step stopped by a confirmation requirement is never retried: that needs the person.
	RetryFailed bool
	// OnFailure is the policy; the default is StopOnFailure.
	OnFailure OnFailure
}

// Ran is one step this orchestration ran.
type Ran struct {
	StepID      string
	StepType    string
	ActionType  domain.CommerceActionType
	ExecutionID string
	Attempt     int
	Status      domain.ExecutionStatus
	ErrorCode   string
}

// State is the execution state of a mission: what an orchestration (or a read) answers.
type State struct {
	Mission *domain.Mission
	// Flow is the current plan's steps with their states.
	Flow     *domain.MissionFlow
	Outcome  domain.MissionOutcome
	Progress domain.Progress
	// Busy is true when another orchestration holds the mission: nothing was run by this request.
	Busy bool
	// Ran lists the steps this request ran, in order; empty for a read or a request that found nothing to run.
	Ran []Ran
}

// Orchestrator runs missions.
type Orchestrator struct {
	missions domain.MissionRepository
	store    domain.MissionRunStore
	steps    StepRunner
	logger   zerolog.Logger
	now      func() time.Time
	lease    time.Duration
	newToken func() string
}

// New creates the orchestrator.
func New(missions domain.MissionRepository, store domain.MissionRunStore, steps StepRunner, logger zerolog.Logger) *Orchestrator {
	return &Orchestrator{missions: missions, store: store, steps: steps, logger: logger, now: time.Now, lease: domain.DefaultRunLease, newToken: domain.NewID}
}

// WithClock sets the clock (tests).
func (o *Orchestrator) WithClock(now func() time.Time) *Orchestrator {
	o.now = now
	return o
}

// WithLease sets how long a run may hold a mission before another may take it over (tests).
func (o *Orchestrator) WithLease(d time.Duration) *Orchestrator {
	o.lease = d
	return o
}

// load returns the mission if the actor owns it.
func (o *Orchestrator) load(ctx context.Context, actorID, missionID string) (*domain.Mission, error) {
	m, err := o.missions.Get(ctx, missionID)
	if err != nil {
		return nil, err
	}
	if actorID == "" || !m.OwnedBy(actorID) {
		return nil, domain.ErrMissionForbidden
	}
	return m, nil
}

// State reads the execution state of one of the actor's missions. It changes nothing.
func (o *Orchestrator) State(ctx context.Context, actorID, missionID string) (*State, error) {
	m, err := o.load(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	busy, err := o.store.RunInProgress(ctx, m.ID, o.now())
	if err != nil {
		return nil, err
	}
	return o.snapshot(ctx, m, busy, nil)
}

// Run orchestrates one of the actor's missions: it runs, in dependency order, the steps that are ready, until none is
// left to run (or the policy stops it). Calling it again runs only what is still to be done: a completed step is never
// run again, so a repeated request, or one that finds the mission already done, runs nothing. When another
// orchestration holds the mission it runs nothing and says so (State.Busy). Errors: ErrMissionNotFound,
// ErrMissionForbidden, ErrNoPlanToRun, ErrDependencyInvalid / ErrDependencyCycle, ErrMissionNotRunnable.
func (o *Orchestrator) Run(ctx context.Context, actorID, missionID string, opts Options) (*State, error) {
	if opts.OnFailure == "" {
		opts.OnFailure = StopOnFailure
	}
	m, err := o.load(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	// Refuse before taking the lock what could never run: no plan, a graph that cannot be ordered.
	if _, err := o.steps.Flow(ctx, m); err != nil {
		return nil, err
	}
	if m.Status == domain.MissionCompleted {
		return o.snapshot(ctx, m, false, nil) // done: nothing to run, nothing wrong
	}
	if err := domain.MissionRunnable(m); err != nil {
		return nil, err
	}

	token := o.newToken()
	acquired, err := o.store.AcquireRun(ctx, m.ID, token, o.now(), o.lease)
	if err != nil {
		return nil, err
	}
	if !acquired {
		o.logger.Info().Str("mission_id", m.ID).Msg("Mission orchestration skipped: another one is running")
		return o.snapshot(ctx, m, true, nil)
	}
	defer func() {
		release, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := o.store.ReleaseRun(release, m.ID, token); err != nil {
			o.logger.Warn().Err(err).Str("mission_id", m.ID).Msg("Mission run lock could not be released; it expires by itself")
		}
	}()

	// What was read before the lock may be stale: read again now that nobody else can change the mission's steps.
	if m, err = o.load(ctx, actorID, missionID); err != nil {
		return nil, err
	}
	ran, busy, err := o.drive(ctx, actorID, m, token, opts)
	if err != nil {
		return nil, err
	}
	if m, err = o.finish(ctx, actorID, missionID); err != nil {
		return nil, err
	}
	return o.snapshot(ctx, m, busy, ran)
}

// drive runs ready steps one at a time until none is left to run. busy is true when the lock was lost on the way.
func (o *Orchestrator) drive(ctx context.Context, actorID string, m *domain.Mission, token string, opts Options) (ran []Ran, busy bool, err error) {
	log := o.logger.With().Str("mission_id", m.ID).Logger()
	attempted := map[string]bool{} // a step is tried at most once per orchestration, so a failure cannot loop
	for ctx.Err() == nil {
		flow, err := o.steps.Flow(ctx, m)
		if err != nil {
			return ran, false, err
		}
		next := pickNext(flow, opts, attempted)
		if next == nil {
			break
		}
		step := next.Step
		attempted[step.ID] = true

		if m.Status == domain.MissionDraft { // running the first step of a draft is what activates it
			if _, err := o.store.TransitionMission(ctx, m.ID, domain.MissionDraft, domain.MissionActive, o.now()); err != nil {
				return ran, false, err
			}
			if m, err = o.missions.Get(ctx, m.ID); err != nil {
				return ran, false, err
			}
			if err := domain.MissionRunnable(m); err != nil {
				return ran, false, nil // paused or cancelled meanwhile: stop, the person decided
			}
		}
		if err := o.store.ExtendRun(ctx, m.ID, token, o.now(), o.lease); err != nil {
			if errors.Is(err, domain.ErrMissionBusy) {
				log.Warn().Msg("Mission run lock was lost; stopping")
				return ran, true, nil
			}
			return ran, false, err
		}

		// A key of its own per orchestration and step: a repeated request never replays an old attempt, and the executor's
		// claim (one attempt in progress per step) is the second line of defence if two orchestrations ever overlap.
		key := "orch:" + token + ":" + step.ID
		res, err := o.steps.Execute(ctx, actorID, m.ID, step.ID, key)
		if err != nil {
			switch {
			case errors.Is(err, domain.ErrStepInProgress):
				log.Info().Int("plan_version", flow.PlanVersion).Str("step_id", step.ID).Msg("Step is already being executed elsewhere; skipping it")
				continue
			case errors.Is(err, domain.ErrStepWaiting), errors.Is(err, domain.ErrStepInputMissing),
				errors.Is(err, domain.ErrStepNotExecutable), errors.Is(err, domain.ErrStepNotRunnable):
				log.Info().Err(err).Int("plan_version", flow.PlanVersion).Str("step_id", step.ID).Msg("Step could not be run now; skipping it")
				continue // the state changed since it was read: the next look decides
			case errors.Is(err, domain.ErrMissionNotRunnable):
				return ran, false, nil
			}
			return ran, false, err
		}
		r := Ran{StepID: step.ID, StepType: step.Type, ActionType: step.ActionType}
		if x := res.Execution; x != nil {
			r.ExecutionID, r.Attempt, r.Status, r.ErrorCode = x.ID, x.Attempt, x.Status, x.ErrorCode
		}
		ran = append(ran, r)
		log.Info().Int("plan_version", flow.PlanVersion).Str("step_id", step.ID).Str("execution_id", r.ExecutionID).
			Str("action_type", string(step.ActionType)).Str("status", string(r.Status)).Str("error_code", r.ErrorCode).Msg("Mission step orchestrated")

		if r.Status != domain.ExecutionCompleted && opts.OnFailure != ContinueIndependent {
			break // the default: stop at the first failure; what depends on it is blocked, what completed stays
		}
	}
	return ran, false, nil
}

// pickNext returns the next step to run: the first, in position order, that is ready (or, on a resume, failed or gone
// silent) and has not been tried in this orchestration.
func pickNext(flow *domain.MissionFlow, opts Options, attempted map[string]bool) *domain.FlowStep {
	for i := range flow.Steps {
		s := &flow.Steps[i]
		if attempted[s.Step.ID] || !s.State.CanExecute {
			continue
		}
		if s.State.Flow == domain.FlowReady || (opts.RetryFailed && s.State.Flow == domain.FlowFailed) {
			return s
		}
	}
	return nil
}

// finish completes the mission when every step is completed, and returns the mission as it now is.
func (o *Orchestrator) finish(ctx context.Context, actorID, missionID string) (*domain.Mission, error) {
	m, err := o.load(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	if m.Status == domain.MissionCompleted || (m.Status != domain.MissionActive && m.Status != domain.MissionDraft) {
		return m, nil
	}
	flow, err := o.steps.Flow(ctx, m)
	if err != nil {
		return nil, err
	}
	if p := flow.Progress(); p.Total == 0 || p.Completed != p.Total {
		return m, nil
	}
	if m.Status == domain.MissionDraft { // steps completed one by one while it was still a draft
		if _, err := o.store.TransitionMission(ctx, m.ID, domain.MissionDraft, domain.MissionActive, o.now()); err != nil {
			return nil, err
		}
	}
	changed, err := o.store.TransitionMission(ctx, m.ID, domain.MissionActive, domain.MissionCompleted, o.now())
	if err != nil {
		return nil, err
	}
	if changed {
		o.logger.Info().Str("mission_id", m.ID).Int("plan_version", flow.PlanVersion).Msg("Mission completed: every step is completed")
	}
	return o.load(ctx, actorID, missionID)
}

// snapshot assembles the answer from what is stored.
func (o *Orchestrator) snapshot(ctx context.Context, m *domain.Mission, busy bool, ran []Ran) (*State, error) {
	flow, err := o.steps.Flow(ctx, m)
	if err != nil {
		return nil, err
	}
	return &State{Mission: m, Flow: flow, Outcome: flow.Outcome(busy), Progress: flow.Progress(), Busy: busy, Ran: ran}, nil
}
