package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

/*
 * Mission planning (Organiser, phase 2). A MissionPlanner turns a mission into a plan: the ordered list of things
 * that will need doing, each one an INTENT to act (a step type, the commerce action it will run, and whether the
 * Commerce OS can run that action today). It never searches, reserves, orders, pays, meters a credit or calls the
 * Commerce Action Runtime; the runtime stays the one place where actions run, in a later phase.
 *
 * The planner is an interface so that a rule-based planner (templates per mission type, no model) can be replaced
 * or completed by another one later without touching Mission, MissionStep or the runtime.
 */

// StepActionSupport says whether the Commerce OS can run the action a step names.
type StepActionSupport string

const (
	// ActionSupported: a handler for ActionType is registered in the Commerce Action Runtime.
	ActionSupported StepActionSupport = "supported"
	// ActionNotYetSupported: the action the step needs does not exist in the runtime yet; the step is recorded
	// as an intention only and cannot be connected to an action until it does.
	ActionNotYetSupported StepActionSupport = "not_yet_supported"
)

// IsValid reports whether s is a known support value (the empty value is a step added by hand).
func (s StepActionSupport) IsValid() bool {
	return s == "" || s == ActionSupported || s == ActionNotYetSupported
}

// MissionPlanStatus is where a plan stands among the versions of a mission's plan.
type MissionPlanStatus string

const (
	// MissionPlanCurrent: the plan in force; a mission has at most one.
	MissionPlanCurrent MissionPlanStatus = "current"
	// MissionPlanSuperseded: replaced by a later version; kept so the history stays traceable.
	MissionPlanSuperseded MissionPlanStatus = "superseded"
)

// Plan errors.
var (
	// ErrMissionPlanNotFound: the mission has no plan (or not that version).
	ErrMissionPlanNotFound = fmt.Errorf("mission plan not found")
	// ErrMissionPlanExists: the mission already has a different plan; replacing it must be asked for.
	ErrMissionPlanExists = fmt.Errorf("the mission already has a plan")
	// ErrMissionPlanInProgress: some step of the current plan has started, so the plan cannot be replaced.
	ErrMissionPlanInProgress = fmt.Errorf("the plan has steps under way and cannot be replaced")
	// ErrMissionPlanConflict: the plan changed while a new one was being stored; read it again.
	ErrMissionPlanConflict = fmt.Errorf("the plan changed meanwhile")
	// ErrPlanningUnavailable: no planner is configured.
	ErrPlanningUnavailable = fmt.Errorf("planning is not available")
)

// MissionPlan is one version of a mission's plan and its steps.
type MissionPlan struct {
	ID        string
	MissionID string
	// Version counts from 1 per mission; a replacement is the next version, never an edit of the old one.
	Version int
	// MissionType is the mission's type when it was planned, so a plan prepared for another kind of goal can be told.
	MissionType    MissionType
	Planner        string
	PlannerVersion string
	Status         MissionPlanStatus
	// Fingerprint identifies what the plan contains (planner, templates version, mission type, steps): planning
	// the same mission again with the same planner yields the same fingerprint, hence the same plan.
	Fingerprint    string
	IdempotencyKey string
	CreatedAt      int64
	SupersededAt   int64
	Steps          []MissionStep
	// Dependencies are the edges of the plan's execution graph, between the ids of its steps.
	Dependencies []StepDependency
}

// HasStartedSteps reports whether any step has left the not-started states: such a plan is not replaced.
func (p *MissionPlan) HasStartedSteps() bool {
	for _, s := range p.Steps {
		if s.Status != StepPending && s.Status != StepSkipped {
			return true
		}
	}
	return false
}

// PlannedStep is one intention of a plan, before it becomes a stored step.
type PlannedStep struct {
	// Type is a stable code naming what is to be found or done (FIND_VENUE, FIND_ACCOMMODATION…).
	Type        string
	Title       string
	Description string
	// ActionType is the commerce action the step will run, whether or not the runtime has it yet.
	ActionType CommerceActionType
	// ActionSupport is set by the planner from what the runtime can run today.
	ActionSupport StepActionSupport
	// DependsOn are the types of the earlier steps of this proposal that must be completed before this one may run
	// (a check of availability depends on the search that found what it checks). Empty for a step that stands alone.
	DependsOn []string
}

// PlanProposal is what a planner answers: the steps, or the fact that it cannot plan this mission yet.
type PlanProposal struct {
	Planner        string
	PlannerVersion string
	Steps          []PlannedStep
	// NeedsInformation is true when the planner cannot plan the mission as it is (no template for its kind): no
	// step is made up, and the person is asked to say more.
	NeedsInformation bool
	Reason           string
}

// Reasons a planner gives when it needs more information.
const ReasonNoTemplate = "no_template"

// MissionPlanner turns a mission into a plan. Implementations are pure: they read the mission and answer; they
// do not search, reserve, order, pay, spend a credit or run a commerce action.
type MissionPlanner interface {
	// Name identifies the planner ("rules", later for example "ai").
	Name() string
	// Version identifies its templates or model; a change of version is a change of plan.
	Version() string
	// Plannable reports whether the planner has a plan for missions of that type.
	Plannable(t MissionType) bool
	// Plan proposes the plan of a mission.
	Plan(ctx context.Context, m *Mission) (*PlanProposal, error)
}

// PlanFingerprint identifies a plan's content. Two plans of one mission with the same fingerprint are the same plan.
func PlanFingerprint(planner, version string, typ MissionType, steps []PlannedStep) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s", planner, version, typ)
	for _, s := range steps {
		fmt.Fprintf(h, "|%s:%s:%s:%s", s.Type, s.ActionType, s.ActionSupport, strings.Join(s.DependsOn, "+"))
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// NewMissionPlan builds the plan that follows `current` (nil when there is none) from a proposal. Steps start
// pending at positions 1..n of the new version. It validates every step.
func NewMissionPlan(m *Mission, current *MissionPlan, p *PlanProposal, idempotencyKey string, nowUnix int64) (*MissionPlan, error) {
	if p == nil || p.NeedsInformation || len(p.Steps) == 0 {
		return nil, missionInvalid("there is nothing to plan")
	}
	version := 1
	if current != nil {
		version = current.Version + 1
	}
	plan := &MissionPlan{
		ID: NewID(), MissionID: m.ID, Version: version, MissionType: m.Type, Planner: p.Planner, PlannerVersion: p.PlannerVersion,
		Status: MissionPlanCurrent, Fingerprint: PlanFingerprint(p.Planner, p.PlannerVersion, m.Type, p.Steps),
		IdempotencyKey: strings.TrimSpace(idempotencyKey), CreatedAt: nowUnix,
	}
	idOfType := make(map[string]string, len(p.Steps))
	for i, ps := range p.Steps {
		step := MissionStep{
			ID: NewID(), MissionID: m.ID, Type: ps.Type, Title: ps.Title, Description: ps.Description, Status: StepPending,
			Position: i + 1, ActionType: ps.ActionType, ActionSupport: ps.ActionSupport, PlanVersion: version,
			CreatedAt: nowUnix, UpdatedAt: nowUnix,
		}
		if err := step.Validate(); err != nil {
			return nil, err
		}
		if _, dup := idOfType[ps.Type]; dup && len(ps.DependsOn) > 0 {
			return nil, missionInvalid("step type %s is used twice, so a dependency cannot name it", ps.Type)
		}
		idOfType[ps.Type] = step.ID
		for _, dep := range ps.DependsOn {
			depID, ok := idOfType[dep]
			if !ok || dep == ps.Type {
				return nil, fmt.Errorf("%w: step %s depends on %s, which is not an earlier step of the plan", ErrDependencyInvalid, ps.Type, dep)
			}
			plan.Dependencies = append(plan.Dependencies, StepDependency{StepID: step.ID, DependsOnID: depID})
		}
		plan.Steps = append(plan.Steps, step)
	}
	if err := ValidateGraph(plan.Steps, plan.Dependencies); err != nil {
		return nil, err
	}
	return plan, nil
}

// MissionPlanRepository stores the versions of a mission's plan and their steps.
type MissionPlanRepository interface {
	// CurrentPlan returns the plan in force with its steps, or ErrMissionPlanNotFound.
	CurrentPlan(ctx context.Context, missionID string) (*MissionPlan, error)
	// PlanVersion returns one version (current or superseded) with its steps, or ErrMissionPlanNotFound.
	PlanVersion(ctx context.Context, missionID string, version int) (*MissionPlan, error)
	// PlanVersions lists the versions of the mission's plan, oldest first.
	PlanVersions(ctx context.Context, missionID string) ([]int, error)
	// PlanByIdempotencyKey returns the plan an earlier request with that key stored, or ErrMissionPlanNotFound.
	PlanByIdempotencyKey(ctx context.Context, missionID, key string) (*MissionPlan, error)
	// SavePlan stores a new plan version with its steps and supersedes the current one, in one transaction, only
	// if the mission's current version is still expectedCurrent (0 for none); otherwise ErrMissionPlanConflict.
	// A repeated idempotency key is ErrIdempotencyConflict.
	SavePlan(ctx context.Context, plan *MissionPlan, expectedCurrent int) error
}
