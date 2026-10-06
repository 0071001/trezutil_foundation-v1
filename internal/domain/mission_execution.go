package domain

import (
	"context"
	"fmt"
	"time"
)

/*
 * Executing a mission step (Organiser, phase 3). A step is run ONE at a time, on request, through the existing
 * Commerce Action Runtime: the mission layer translates the step's intent into a runtime request and records the
 * outcome. It does not search, reserve, order or pay itself, and it never touches credits: the runtime checks,
 * reserves, consumes or releases them exactly as for any other action.
 *
 * Every attempt is a MissionStepExecution, kept as the durable record of what the runtime answered: a small
 * summary and references to commerce entities, never a copy of a commerce dataset.
 */

// ExecutionStatus is where one attempt to run a step stands.
type ExecutionStatus string

const (
	ExecutionInProgress ExecutionStatus = "in_progress"
	ExecutionCompleted  ExecutionStatus = "completed"
	ExecutionFailed     ExecutionStatus = "failed"
)

// IsValid reports whether s is a known execution status.
func (s ExecutionStatus) IsValid() bool {
	return s == ExecutionInProgress || s == ExecutionCompleted || s == ExecutionFailed
}

// Execution error classifications. They are stable codes, not runtime messages: a runtime error code is kept as it
// is when the runtime gave one (INSUFFICIENT_CREDITS, INVALID_PARAM, ACTION_FAILED…), these are the mission
// layer's own.
const (
	// ErrorCodeRuntime: the runtime itself returned an error instead of an answer.
	ErrorCodeRuntime = "RUNTIME_ERROR"
	// ErrorCodeTimeout: the runtime did not answer in time.
	ErrorCodeTimeout = "RUNTIME_TIMEOUT"
	// ErrorCodeAbandoned: an earlier attempt never recorded its outcome and was given up on.
	ErrorCodeAbandoned = "ABANDONED"
	// ErrorCodeUnexpected: the runtime answered something the mission layer cannot use.
	ErrorCodeUnexpected = "UNEXPECTED_RESULT"
)

// Execution errors.
var (
	// ErrMissionStepNotFound: no such step in the mission.
	ErrMissionStepNotFound = fmt.Errorf("mission step not found")
	// ErrMissionExecutionNotFound: no execution with that key for the step.
	ErrMissionExecutionNotFound = fmt.Errorf("mission step execution not found")
	// ErrStepNotExecutable: the step cannot be run yet (no executable action behind it, or the runtime has none).
	ErrStepNotExecutable = fmt.Errorf("this step cannot be executed yet")
	// ErrStepInProgress: an attempt is already running.
	ErrStepInProgress = fmt.Errorf("the step is already being executed")
	// ErrStepNotRunnable: the step is not in a state that allows running it (completed or skipped).
	ErrStepNotRunnable = fmt.Errorf("the step cannot be executed in its current state")
	// ErrMissionNotRunnable: the mission is paused, completed or cancelled, so its steps are not run.
	ErrMissionNotRunnable = fmt.Errorf("the mission does not allow running steps now")
	// ErrExecutionSuperseded: the attempt was given up on while it was running; its late outcome is not recorded.
	ErrExecutionSuperseded = fmt.Errorf("the execution was superseded")
)

// DefaultExecutionStaleAfter is how long an attempt may stay in progress before it is treated as abandoned (the
// process died, or the outcome could not be recorded). It is longer than the time the runtime is given to answer.
const DefaultExecutionStaleAfter = 2 * time.Minute

// MissionStepExecution is one attempt to run a step through the Commerce Action Runtime.
type MissionStepExecution struct {
	ID        string
	StepID    string
	MissionID string
	// Attempt counts from 1 per step; a retry is the next attempt, never an edit of the failed one.
	Attempt    int
	Status     ExecutionStatus
	ActionType CommerceActionType
	// IdempotencyKey is the caller's key: the same step and key is the same execution.
	IdempotencyKey string
	// ResultReference points at what the attempt produced. For a search it is this execution (a result set is not
	// a commerce entity); for an action that creates one (an order, a reservation) it will be that entity's id.
	ResultReference string
	// ResultPayload is a small summary (counts, ids of the entities found), never a copy of commerce data.
	ResultPayload map[string]interface{}
	// RuntimeStatus is the status the runtime answered, for diagnosis; clients see ErrorCode.
	RuntimeStatus string
	ErrorCode     string
	StartedAt     int64
	CompletedAt   int64
	CreatedAt     int64
}

// ExecutionClaim says what may be done to a step that is asked to run: it returns an error when the step cannot
// run now, and abandon=true when the step is stuck in progress from an attempt that never ended, which is then
// given up on so that a new one may start. It is a pure decision, so it holds under the repository's row lock.
func ExecutionClaim(step *MissionStep, latest *MissionStepExecution, now int64, staleAfter time.Duration) (abandon bool, err error) {
	switch step.Status {
	case StepPending, StepFailed:
		return false, nil
	case StepInProgress:
		if latest != nil && latest.Status == ExecutionInProgress && now-latest.StartedAt < int64(staleAfter/time.Second) {
			return false, ErrStepInProgress
		}
		return true, nil // nothing is running, or the attempt that was has gone silent
	default: // completed and skipped steps are not run again: a completed step is never reset to pending
		return false, fmt.Errorf("%w: %s", ErrStepNotRunnable, step.Status)
	}
}

// MissionRunnable reports whether a mission's steps may be run: a draft or an active mission, not a paused or a
// closed one. Running a step does not activate a draft.
func MissionRunnable(m *Mission) error {
	if m.Status == MissionDraft || m.Status == MissionActive {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrMissionNotRunnable, m.Status)
}

// BeginExecutionInput asks for an attempt to be claimed.
type BeginExecutionInput struct {
	MissionID      string
	StepID         string
	IdempotencyKey string
	ActionType     CommerceActionType
	Now            int64
	StaleAfter     time.Duration
}

// MissionExecutionRepository stores the attempts to run steps.
type MissionExecutionRepository interface {
	// GetStep returns a step of the mission, or ErrMissionStepNotFound (also when the step belongs to another mission).
	GetStep(ctx context.Context, missionID, stepID string) (*MissionStep, error)
	// ExecutionByKey returns the step's execution made with that key, or ErrMissionExecutionNotFound.
	ExecutionByKey(ctx context.Context, stepID, key string) (*MissionStepExecution, error)
	// BeginExecution claims an attempt: in one transaction, with the step row locked, it returns the execution a
	// request with the same key already made (replayed=true), or applies ExecutionClaim and, if the step may run,
	// moves it to in_progress and stores the new attempt. Errors: ErrMissionStepNotFound, ErrStepInProgress,
	// ErrStepNotRunnable.
	BeginExecution(ctx context.Context, in BeginExecutionInput) (exec *MissionStepExecution, replayed bool, err error)
	// FinishExecution records the outcome of an attempt that is still in progress and moves the step to
	// completed or failed, in one transaction; ErrExecutionSuperseded when the attempt was given up on meanwhile.
	FinishExecution(ctx context.Context, exec *MissionStepExecution, now int64) error
	// LatestExecutions returns the latest attempt of each step of the mission that has one, by step id.
	LatestExecutions(ctx context.Context, missionID string) (map[string]*MissionStepExecution, error)
}

// MissionStepState is what a client needs to know about a step beyond the step itself: whether it can be run at
// all, whether it can be run now, how its latest attempt went and, for a step of a plan, where it stands in the
// plan's dependency graph. It is computed, never stored.
type MissionStepState struct {
	// Executable: the step has an executable action behind it and the runtime supports that action.
	Executable bool
	// CanExecute: Executable, the mission allows running steps, the step's dependencies are completed and its inputs
	// known, and the step is pending, failed or stuck.
	CanExecute bool
	// Stale: the step says in progress but its attempt has gone silent; it can be run again.
	Stale bool
	// Execution is the step's latest attempt, nil when it has none.
	Execution *MissionStepExecution

	// Flow is what the step is in the plan's graph (pending, ready, running, completed, failed, blocked, skipped);
	// empty for a step that is not part of the current plan (added by hand, or of a replaced plan).
	Flow StepFlowState
	// DependsOn are the ids of the steps this one waits for.
	DependsOn []string
	// BlockedBy are the ids of the dependencies that keep a blocked step blocked.
	BlockedBy []string
	// Reason says why the step is blocked or cannot run yet (see the Reason constants).
	Reason string
	// Detail names what is missing when Reason is needs_input ("dates", "listing").
	Detail string
}

// StepAnnotator computes the state of a mission's steps (implemented by the step executor).
type StepAnnotator interface {
	Annotate(ctx context.Context, m *Mission, steps []MissionStep) (map[string]MissionStepState, error)
}

// StepExecutionResult is the answer to a request to run a step.
type StepExecutionResult struct {
	Mission   *Mission
	Step      *MissionStep
	Execution *MissionStepExecution
	State     MissionStepState
	// Replayed is true when the request repeated an idempotency key and nothing was run.
	Replayed bool
}
