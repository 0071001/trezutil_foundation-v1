package missionexec

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * The state of the steps of a mission: from the stored step statuses, the latest attempt of each step and the plan's
 * dependency graph, domain.ComputeFlow derives pending, ready, running, completed, failed, blocked or skipped. Nothing is
 * stored for it; this file only gathers what the pure function asks about (what the runtime can run, what a step still
 * lacks, which attempt has gone silent).
 */

var _ domain.StepFlow = (*Executor)(nil)

// stale reports whether a step says in progress but its attempt has gone silent.
func (e *Executor) stale(step *domain.MissionStep, latest *domain.MissionStepExecution, now int64) bool {
	if step.Status != domain.StepInProgress {
		return false
	}
	return latest == nil || (latest.Status == domain.ExecutionInProgress && now-latest.StartedAt >= int64(e.staleAfter/time.Second))
}

// Flow returns the state of every step of the mission's current plan, in position order. It refuses a plan whose
// dependencies are invalid or form a cycle (domain.ErrDependencyInvalid, domain.ErrDependencyCycle), so nothing is run
// from a graph that cannot be ordered, and reports domain.ErrNoPlanToRun when the mission has no plan.
func (e *Executor) Flow(ctx context.Context, m *domain.Mission) (*domain.MissionFlow, error) {
	plan, err := e.plans.CurrentPlan(ctx, m.ID)
	if err != nil {
		if errors.Is(err, domain.ErrMissionPlanNotFound) {
			return nil, domain.ErrNoPlanToRun
		}
		return nil, err
	}
	latest, err := e.execs.LatestExecutions(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	states, err := e.planStates(m, plan, latest)
	if err != nil {
		return nil, err
	}
	flow := &domain.MissionFlow{PlanVersion: plan.Version, Steps: make([]domain.FlowStep, len(plan.Steps))}
	for i := range plan.Steps {
		flow.Steps[i] = domain.FlowStep{Step: plan.Steps[i], State: states[i]}
	}
	return flow, nil
}

// planStates derives the state of each step of the plan, index-aligned with plan.Steps.
func (e *Executor) planStates(m *domain.Mission, plan *domain.MissionPlan, latest map[string]*domain.MissionStepExecution) ([]domain.MissionStepState, error) {
	now := e.now().Unix()
	index := make(map[string]int, len(plan.Steps))
	for i := range plan.Steps {
		index[plan.Steps[i].ID] = i
	}
	needs := make(map[string][]string)
	for _, d := range plan.Dependencies {
		needs[d.StepID] = append(needs[d.StepID], d.DependsOnID)
	}
	upstreamOf := func(s *domain.MissionStep) []domain.UpstreamStep {
		ids := append([]string(nil), needs[s.ID]...)
		sort.Slice(ids, func(a, b int) bool { return plan.Steps[index[ids[a]]].Position < plan.Steps[index[ids[b]]].Position })
		out := make([]domain.UpstreamStep, 0, len(ids))
		for _, id := range ids {
			if i, ok := index[id]; ok {
				out = append(out, domain.UpstreamStep{Step: plan.Steps[i], Execution: latest[id]})
			}
		}
		return out
	}
	states, err := domain.ComputeFlow(plan.Steps, plan.Dependencies, domain.FlowInputs{
		Executable: func(s *domain.MissionStep) bool { return e.Executable(m, s) },
		Stale:      func(s *domain.MissionStep) bool { return e.stale(s, latest[s.ID], now) },
		MissingInput: func(s *domain.MissionStep) string {
			_, missing := buildParams(m, s, upstreamOf(s))
			return missing
		},
		NeedsConfirmation: func(s *domain.MissionStep) bool {
			x := latest[s.ID]
			return x != nil && x.ErrorCode == domain.ErrorCodeConfirmationRequired
		},
	})
	if err != nil {
		return nil, err
	}
	runnable := domain.MissionRunnable(m) == nil
	for i := range plan.Steps {
		s, st := &plan.Steps[i], &states[i]
		st.Execution = latest[s.ID]
		st.Executable = e.Executable(m, s)
		switch st.Flow {
		case domain.FlowReady:
			st.CanExecute = st.Executable && runnable
		case domain.FlowFailed: // a failed or abandoned step can be run again once what it needs is still there
			done := true
			for _, id := range needs[s.ID] {
				if states[index[id]].Flow != domain.FlowCompleted {
					done = false
				}
			}
			_, missing := buildParams(m, s, upstreamOf(s))
			st.CanExecute = st.Executable && runnable && done && missing == ""
		}
	}
	return states, nil
}

// Annotate computes, for each step, whether it can be run and how its latest attempt went. A step of the current plan
// also gets its place in the plan's graph; a step added by hand, or of a plan that has been replaced, has none.
func (e *Executor) Annotate(ctx context.Context, m *domain.Mission, steps []domain.MissionStep) (map[string]domain.MissionStepState, error) {
	latest, err := e.execs.LatestExecutions(ctx, m.ID)
	if err != nil {
		return nil, err
	}
	now := e.now().Unix()
	runnable := domain.MissionRunnable(m) == nil

	// The graph is read only when a step belongs to a plan.
	planned := false
	for i := range steps {
		if steps[i].PlanVersion > 0 {
			planned = true
		}
	}
	var current *domain.MissionPlan
	flow := map[string]domain.MissionStepState{}
	if planned {
		if current, err = e.plans.CurrentPlan(ctx, m.ID); err != nil && !errors.Is(err, domain.ErrMissionPlanNotFound) {
			return nil, err
		}
		if current != nil {
			states, err := e.planStates(m, current, latest)
			switch {
			case err == nil:
				for i := range current.Steps {
					flow[current.Steps[i].ID] = states[i]
				}
			case errors.Is(err, domain.ErrDependencyInvalid) || errors.Is(err, domain.ErrDependencyCycle):
				// A stored graph that cannot be ordered is shown without states and its steps cannot be run (Flow says why).
				e.logger.Warn().Err(err).Str("mission_id", m.ID).Int("plan_version", current.Version).Msg("Mission plan graph is invalid")
			default:
				return nil, err
			}
		}
	}

	out := make(map[string]domain.MissionStepState, len(steps))
	for i := range steps {
		step := &steps[i]
		if st, ok := flow[step.ID]; ok {
			out[step.ID] = st
			continue
		}
		st := domain.MissionStepState{Execution: latest[step.ID], Executable: e.Executable(m, step)}
		st.Stale = e.stale(step, st.Execution, now)
		_, missing := buildParams(m, step, nil)
		st.CanExecute = st.Executable && runnable && missing == "" && step.PlanVersion == 0 &&
			(step.Status == domain.StepPending || step.Status == domain.StepFailed || st.Stale)
		out[step.ID] = st
	}
	return out, nil
}
