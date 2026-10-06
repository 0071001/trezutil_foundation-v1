// Package missionexec runs a mission step through the existing Commerce Action Runtime.
//
// The MissionStepExecutor is deliberately thin. It loads the mission and the step, checks that the caller owns the
// mission and that the step can run, translates the step's intent into a runtime request, hands it to the runtime
// and records the outcome. It does not search, reserve, order, pay, or check inventory, authorization or credits:
// the runtime does all of that, exactly as for any other action, and credits are never read or written here. One
// step is run per request; running a whole mission is the job of the MissionOrchestrator (package missionorch),
// which only asks this executor to run what is ready.
//
// Since phase 4 a step of a plan may wait for the steps it depends on: the executor refuses to run it until they have
// completed, takes what it needs from them by reference (the listing a search found, never a copy of the catalogue),
// and derives, from the plan's dependency graph, the state of every step (domain.ComputeFlow).
package missionexec

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// Runtime is the part of the Commerce Action Runtime the executor uses (commerce.CommerceActionRuntime).
type Runtime interface {
	Execute(ctx context.Context, action *domain.CommerceAction, authenticatedActor domain.CommerceActor) (*commerce.ExecuteResult, error)
	IsSupported(actionType domain.CommerceActionType) bool
}

// DefaultTimeout is how long the runtime is given to answer for one step.
const DefaultTimeout = 30 * time.Second

// Executor runs one mission step at a time (the MissionStepExecutor).
type Executor struct {
	missions   domain.MissionRepository
	plans      domain.MissionPlanRepository
	execs      domain.MissionExecutionRepository
	graph      domain.MissionGraphRepository
	runtime    Runtime
	logger     zerolog.Logger
	now        func() time.Time
	timeout    time.Duration
	staleAfter time.Duration
}

var _ domain.StepAnnotator = (*Executor)(nil)

// New creates the executor.
func New(missions domain.MissionRepository, plans domain.MissionPlanRepository, execs domain.MissionExecutionRepository, graph domain.MissionGraphRepository, runtime Runtime, logger zerolog.Logger) *Executor {
	return &Executor{missions: missions, plans: plans, execs: execs, graph: graph, runtime: runtime, logger: logger, now: time.Now,
		timeout: DefaultTimeout, staleAfter: domain.DefaultExecutionStaleAfter}
}

// WithClock sets the clock (tests).
func (e *Executor) WithClock(now func() time.Time) *Executor {
	e.now = now
	return e
}

// WithTimeout sets how long the runtime is given to answer (tests).
func (e *Executor) WithTimeout(d time.Duration) *Executor {
	e.timeout = d
	return e
}

// WithStaleAfter sets how long an attempt may stay in progress before it counts as abandoned (tests).
func (e *Executor) WithStaleAfter(d time.Duration) *Executor {
	e.staleAfter = d
	return e
}

// Executable reports whether a step has an executable action behind it that the runtime supports. Whether what the
// step needs is known yet is another question (see buildParams).
func (e *Executor) Executable(m *domain.Mission, step *domain.MissionStep) bool {
	return actionable(m, step) && e.runtime.IsSupported(step.ActionType)
}

// Execute runs one step of one of the actor's missions through the runtime and records the outcome. The same step
// with the same idempotency key is the same execution: a repeated request returns it and runs nothing. A step that
// failed can be run again (a new attempt); a completed one cannot.
func (e *Executor) Execute(ctx context.Context, actorID, missionID, stepID, idempotencyKey string) (*domain.StepExecutionResult, error) {
	m, err := e.missions.Get(ctx, missionID)
	if err != nil {
		return nil, err
	}
	if actorID == "" || !m.OwnedBy(actorID) {
		return nil, domain.ErrMissionForbidden
	}
	step, err := e.execs.GetStep(ctx, m.ID, stepID)
	if err != nil {
		return nil, err
	}

	// A repeated key is answered with what it made, whatever has become of the mission or the step since.
	if idempotencyKey != "" {
		if prior, err := e.execs.ExecutionByKey(ctx, step.ID, idempotencyKey); err == nil {
			return e.result(ctx, m, step.ID, prior, true)
		} else if !errors.Is(err, domain.ErrMissionExecutionNotFound) {
			return nil, err
		}
	}

	if err := domain.MissionRunnable(m); err != nil {
		return nil, err
	}
	if err := e.currentPlanStep(ctx, m, step); err != nil {
		return nil, err
	}
	if !e.Executable(m, step) {
		return nil, domain.ErrStepNotExecutable
	}
	// A step of a plan waits for the steps it depends on, and takes what it needs from them.
	var upstream []domain.UpstreamStep
	if step.PlanVersion > 0 {
		if upstream, err = e.graph.Upstream(ctx, step.ID); err != nil {
			return nil, err
		}
	}
	for i := range upstream {
		if upstream[i].Step.Status != domain.StepCompleted {
			return nil, domain.ErrStepWaiting
		}
	}
	params, missing := buildParams(m, step, upstream)
	if missing != "" {
		return nil, fmt.Errorf("%w: %s", domain.ErrStepInputMissing, missing)
	}

	exec, replayed, err := e.execs.BeginExecution(ctx, domain.BeginExecutionInput{
		MissionID: m.ID, StepID: step.ID, IdempotencyKey: idempotencyKey, ActionType: step.ActionType,
		Now: e.now().Unix(), StaleAfter: e.staleAfter,
	})
	if err != nil {
		return nil, err
	}
	if replayed {
		return e.result(ctx, m, step.ID, exec, true)
	}
	e.run(ctx, m, step, exec, actorID, params)
	return e.result(ctx, m, step.ID, exec, false)
}

// currentPlanStep refuses a step of a plan that has been replaced.
func (e *Executor) currentPlanStep(ctx context.Context, m *domain.Mission, step *domain.MissionStep) error {
	if step.PlanVersion == 0 {
		return nil // added by hand
	}
	cur, err := e.plans.CurrentPlan(ctx, m.ID)
	if err != nil {
		if errors.Is(err, domain.ErrMissionPlanNotFound) {
			return fmt.Errorf("%w: plan replaced", domain.ErrStepNotRunnable)
		}
		return err
	}
	if cur.Version != step.PlanVersion {
		return fmt.Errorf("%w: plan replaced", domain.ErrStepNotRunnable)
	}
	return nil
}

var errorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

// run hands the request to the runtime and records how it ended. It always records an outcome: a failure, a
// timeout or a panic of the runtime ends the attempt as failed, so a step is never left stuck in progress.
func (e *Executor) run(ctx context.Context, m *domain.Mission, step *domain.MissionStep, exec *domain.MissionStepExecution, actorID string, params map[string]interface{}) {
	log := e.logger.With().Str("mission_id", m.ID).Str("step_id", step.ID).Str("execution_id", exec.ID).Str("action_type", string(step.ActionType)).Int("attempt", exec.Attempt).Logger()
	log.Info().Msg("Executing mission step")

	// The runtime needs a conversation to correlate an action with; a mission is its own context. The action id is
	// the execution id, so the runtime's logs and its credit reservation carry the same id as the record here.
	actor := domain.NewCommerceActor(actorID, domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(step.ActionType, actor, "mission:"+m.ID).WithParameters(params).WithSource("USER")
	action.ActionID = exec.ID
	action.IdempotencyKey = "mission-step:" + step.ID + ":" + firstNonEmpty(exec.IdempotencyKey, exec.ID)

	runCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	res, err := e.callRuntime(runCtx, action, actor)
	e.interpret(exec, step, res, err, runCtx.Err(), params)

	// The outcome is recorded even when the request has been cancelled.
	finishCtx, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelFinish()
	switch err := e.execs.FinishExecution(finishCtx, exec, e.now().Unix()); {
	case err == nil:
		log.Info().Str("status", string(exec.Status)).Str("error_code", exec.ErrorCode).Msg("Mission step executed")
	case errors.Is(err, domain.ErrExecutionSuperseded):
		log.Warn().Msg("Mission step outcome not recorded: the attempt was given up on")
	default:
		log.Error().Err(err).Msg("Mission step outcome could not be recorded; the step stays in progress until it is given up on")
	}
}

// callRuntime calls the runtime and turns a panic into an error.
func (e *Executor) callRuntime(ctx context.Context, action *domain.CommerceAction, actor domain.CommerceActor) (res *commerce.ExecuteResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			res, err = nil, fmt.Errorf("runtime panic: %v", r)
		}
	}()
	return e.runtime.Execute(ctx, action, actor)
}

// interpret turns the runtime's answer into the execution's outcome. A runtime error code is kept as it is when it
// is a stable code; free text from the runtime is never copied.
func (e *Executor) interpret(exec *domain.MissionStepExecution, step *domain.MissionStep, res *commerce.ExecuteResult, err error, ctxErr error, params map[string]interface{}) {
	exec.Status = domain.ExecutionFailed
	switch {
	case err != nil || res == nil:
		exec.ErrorCode = domain.ErrorCodeRuntime
		if ctxErr != nil {
			exec.ErrorCode = domain.ErrorCodeTimeout
		}
	case res.Status == domain.ActionResultSuccess:
		exec.Status, exec.RuntimeStatus = domain.ExecutionCompleted, string(res.Status)
		exec.ResultPayload, exec.ResultReference = summarize(step, exec, res.Data, params)
	default:
		exec.RuntimeStatus = string(res.Status)
		exec.ErrorCode = failureCode(res)
	}
}

func failureCode(res *commerce.ExecuteResult) string {
	if res.Status == domain.ActionResultRequiresConfirmation {
		// A step never confirms on the person's behalf: the step is left for them (blocked), and never retried by itself.
		return domain.ErrorCodeConfirmationRequired
	}
	for _, ae := range res.Errors {
		if errorCode.MatchString(ae.Code) {
			return ae.Code
		}
	}
	if code := string(res.Status); errorCode.MatchString(code) {
		return code
	}
	return domain.ErrorCodeRuntime
}

// result assembles the answer from what is stored: the step as it now is and the attempt as it was recorded. If the
// outcome could not be recorded, the stored attempt still says in progress, and that is what is answered.
func (e *Executor) result(ctx context.Context, m *domain.Mission, stepID string, exec *domain.MissionStepExecution, replayed bool) (*domain.StepExecutionResult, error) {
	step, err := e.execs.GetStep(ctx, m.ID, stepID)
	if err != nil {
		return nil, err
	}
	states, err := e.Annotate(ctx, m, []domain.MissionStep{*step})
	if err != nil {
		return nil, err
	}
	state := states[step.ID]
	if !replayed && state.Execution != nil {
		exec = state.Execution
	}
	return &domain.StepExecutionResult{Mission: m, Step: step, Execution: exec, State: state, Replayed: replayed}, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
