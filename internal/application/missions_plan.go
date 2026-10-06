package application

import (
	"context"
	"errors"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Planning a mission. The planner (domain.MissionPlanner) proposes the steps; these use cases check who may ask,
 * keep the versions of the plan and make planning repeatable: asking again for the same mission with the same
 * planner returns the same plan and never a second set of steps. Replacing a plan has to be asked for, and the
 * old version stays in the history. Planning changes nothing about the mission itself (it stays a draft unless the
 * person moves it) and runs no commerce action.
 */

// WithPlanning enables planning with a planner and the store of plan versions.
func (u *MissionUseCases) WithPlanning(planner domain.MissionPlanner, plans domain.MissionPlanRepository) *MissionUseCases {
	u.planner, u.plans = planner, plans
	return u
}

// PlanOptions say how a plan is asked for.
type PlanOptions struct {
	// Replace asks for a new version when the mission already has a different plan.
	Replace bool
	// IdempotencyKey makes a retried request return the plan the first one stored.
	IdempotencyKey string
}

// PlanResult is the answer to a request for a plan, or to reading one.
type PlanResult struct {
	Mission *domain.Mission
	// Plan is nil when the mission has no plan yet (or the planner needs more information).
	Plan *domain.MissionPlan
	// Created is true when this request stored a new plan version.
	Created bool
	// NeedsInformation is true when the planner has no plan for the mission as it is; Reason says why.
	NeedsInformation bool
	Reason           string
	// Plannable is whether the planner has a plan for the mission's type.
	Plannable bool
	// Versions lists the versions of the mission's plan, oldest first.
	Versions []int
	// States holds, by step id, what a client needs to know about each step of Plan (see domain.MissionStepState).
	States map[string]domain.MissionStepState
}

// WithStepStates enables the state of steps (can they be run, how did their latest attempt go).
func (u *MissionUseCases) WithStepStates(a domain.StepAnnotator) *MissionUseCases {
	u.annotator = a
	return u
}

// annotate computes the state of steps. A step of a plan that is no longer current cannot be run.
func (u *MissionUseCases) annotate(ctx context.Context, m *domain.Mission, steps []domain.MissionStep, current bool) (map[string]domain.MissionStepState, error) {
	if u.annotator == nil || len(steps) == 0 {
		return map[string]domain.MissionStepState{}, nil
	}
	states, err := u.annotator.Annotate(ctx, m, steps)
	if err != nil {
		return nil, err
	}
	if !current {
		for id, st := range states {
			st.CanExecute = false
			states[id] = st
		}
	}
	return states, nil
}

// StepsResult is a mission's steps with their states.
type StepsResult struct {
	Steps  []domain.MissionStep
	States map[string]domain.MissionStepState
}

// StepsView lists the steps of one of the actor's missions (those added by hand, then those of the current plan)
// with their states.
func (u *MissionUseCases) StepsView(ctx context.Context, actorID, missionID string) (*StepsResult, error) {
	m, err := u.owned(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	steps, err := u.repo.Steps(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	states, err := u.annotate(ctx, m, steps, true)
	if err != nil {
		return nil, err
	}
	return &StepsResult{Steps: steps, States: states}, nil
}

func (u *MissionUseCases) planning() error {
	if u.planner == nil || u.plans == nil {
		return domain.ErrPlanningUnavailable
	}
	return nil
}

// Plan prepares the plan of one of the actor's missions.
func (u *MissionUseCases) Plan(ctx context.Context, actorID, missionID string, opts PlanOptions) (*PlanResult, error) {
	if err := u.planning(); err != nil {
		return nil, err
	}
	m, err := u.owned(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	res := &PlanResult{Mission: m, Plannable: u.planner.Plannable(m.Type)}

	// A retry with the key of an earlier request is answered with what that request stored.
	if opts.IdempotencyKey != "" {
		if plan, err := u.plans.PlanByIdempotencyKey(ctx, m.ID, opts.IdempotencyKey); err == nil {
			res.Plan = plan
			return u.withVersions(ctx, res)
		} else if !errors.Is(err, domain.ErrMissionPlanNotFound) {
			return nil, err
		}
	}
	if m.Status.IsTerminal() {
		return nil, domain.ErrMissionClosed
	}
	proposal, err := u.planner.Plan(ctx, m)
	if err != nil {
		return nil, err
	}
	if proposal.NeedsInformation {
		res.NeedsInformation, res.Reason = true, proposal.Reason
		return u.withVersions(ctx, res)
	}
	fingerprint := domain.PlanFingerprint(proposal.Planner, proposal.PlannerVersion, m.Type, proposal.Steps)

	// One more try when a concurrent request stored a plan between the read and the write: the second read then
	// finds that plan, and the same request is answered with it.
	for attempt := 0; attempt < 2; attempt++ {
		current, err := u.plans.CurrentPlan(ctx, m.ID)
		if err != nil && !errors.Is(err, domain.ErrMissionPlanNotFound) {
			return nil, err
		}
		if current != nil {
			if current.Fingerprint == fingerprint { // the same plan: nothing to store
				res.Plan = current
				return u.withVersions(ctx, res)
			}
			if !opts.Replace {
				return nil, domain.ErrMissionPlanExists
			}
			if current.HasStartedSteps() {
				return nil, domain.ErrMissionPlanInProgress
			}
		}
		plan, err := domain.NewMissionPlan(m, current, proposal, opts.IdempotencyKey, u.now().Unix())
		if err != nil {
			return nil, err
		}
		expected := 0
		if current != nil {
			expected = current.Version
		}
		switch err := u.plans.SavePlan(ctx, plan, expected); {
		case err == nil:
			res.Plan, res.Created = plan, true
			return u.withVersions(ctx, res)
		case errors.Is(err, domain.ErrMissionPlanConflict):
			continue
		case errors.Is(err, domain.ErrIdempotencyConflict) && opts.IdempotencyKey != "": // a concurrent retry won
			if stored, ferr := u.plans.PlanByIdempotencyKey(ctx, m.ID, opts.IdempotencyKey); ferr == nil {
				res.Plan = stored
				return u.withVersions(ctx, res)
			}
			return nil, err
		default:
			return nil, err
		}
	}
	return nil, domain.ErrMissionPlanConflict
}

// GetPlan reads the plan of one of the actor's missions: the current version, or the given one (history).
func (u *MissionUseCases) GetPlan(ctx context.Context, actorID, missionID string, version int) (*PlanResult, error) {
	if err := u.planning(); err != nil {
		return nil, err
	}
	m, err := u.owned(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	res := &PlanResult{Mission: m, Plannable: u.planner.Plannable(m.Type)}
	var plan *domain.MissionPlan
	if version > 0 {
		plan, err = u.plans.PlanVersion(ctx, m.ID, version)
	} else {
		plan, err = u.plans.CurrentPlan(ctx, m.ID)
	}
	switch {
	case err == nil:
		res.Plan = plan
	case errors.Is(err, domain.ErrMissionPlanNotFound) && version == 0: // no plan yet is a state, not an error
	default:
		return nil, err
	}
	return u.withVersions(ctx, res)
}

func (u *MissionUseCases) withVersions(ctx context.Context, res *PlanResult) (*PlanResult, error) {
	versions, err := u.plans.PlanVersions(ctx, res.Mission.ID)
	if err != nil {
		return nil, err
	}
	res.Versions = versions
	res.States = map[string]domain.MissionStepState{}
	if res.Plan != nil {
		if res.States, err = u.annotate(ctx, res.Mission, res.Plan.Steps, res.Plan.Status == domain.MissionPlanCurrent); err != nil {
			return nil, err
		}
	}
	return res, nil
}
