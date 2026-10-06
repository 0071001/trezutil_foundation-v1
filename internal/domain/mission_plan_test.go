package domain

import (
	"errors"
	"testing"
)

func proposal() *PlanProposal {
	return &PlanProposal{Planner: "rules", PlannerVersion: "1", Steps: []PlannedStep{
		{Type: "FIND_VENUE", Title: "Find a venue", ActionType: ActionSearchListings, ActionSupport: ActionSupported},
		{Type: "FIND_CATERING", Title: "Find catering", ActionType: ActionSearchListings, ActionSupport: ActionSupported},
	}}
}

func TestNewMissionPlan_FirstVersionStartsPending(t *testing.T) {
	m := newTestMission(t)
	p, err := NewMissionPlan(m, nil, proposal(), " key-1 ", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != 1 || p.Status != MissionPlanCurrent || p.MissionID != m.ID || p.MissionType != MissionTypeEvent || p.IdempotencyKey != "key-1" || p.Fingerprint == "" {
		t.Fatalf("plan %+v", p)
	}
	for i, s := range p.Steps {
		if s.Position != i+1 || s.Status != StepPending || s.PlanVersion != 1 || s.MissionID != m.ID || s.ResultRef != "" || s.ActionRef != "" {
			t.Fatalf("step %d: %+v", i, s)
		}
	}
	if p.Steps[0].ID == p.Steps[1].ID || p.Steps[0].ID == "" {
		t.Fatal("steps need distinct ids")
	}
}

func TestNewMissionPlan_ReplacementIsTheNextVersion(t *testing.T) {
	m := newTestMission(t)
	first, _ := NewMissionPlan(m, nil, proposal(), "", 1000)
	second, err := NewMissionPlan(m, first, proposal(), "", 2000)
	if err != nil || second.Version != 2 || second.Steps[0].PlanVersion != 2 || second.Steps[0].Position != 1 {
		t.Fatalf("second plan %+v %v", second, err)
	}
	if first.Fingerprint != second.Fingerprint {
		t.Fatal("the same content has the same fingerprint, whatever the version")
	}
}

func TestNewMissionPlan_RefusesNothingToPlan(t *testing.T) {
	m := newTestMission(t)
	for name, p := range map[string]*PlanProposal{
		"nil":        nil,
		"needs info": {NeedsInformation: true, Reason: ReasonNoTemplate},
		"no steps":   {Planner: "rules", PlannerVersion: "1"},
	} {
		if _, err := NewMissionPlan(m, nil, p, "", 1); !errors.Is(err, ErrMissionInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	bad := proposal()
	bad.Steps[0].Type = "not a code"
	if _, err := NewMissionPlan(m, nil, bad, "", 1); !errors.Is(err, ErrMissionInvalid) {
		t.Errorf("an invalid step = %v", err)
	}
}

func TestPlanFingerprint_FollowsContent(t *testing.T) {
	a := PlanFingerprint("rules", "1", MissionTypeEvent, proposal().Steps)
	if a != PlanFingerprint("rules", "1", MissionTypeEvent, proposal().Steps) {
		t.Fatal("not stable")
	}
	changed := proposal().Steps
	changed[1].ActionSupport = ActionNotYetSupported
	for name, f := range map[string]string{
		"planner":   PlanFingerprint("ai", "1", MissionTypeEvent, proposal().Steps),
		"version":   PlanFingerprint("rules", "2", MissionTypeEvent, proposal().Steps),
		"type":      PlanFingerprint("rules", "1", MissionTypeTravel, proposal().Steps),
		"steps":     PlanFingerprint("rules", "1", MissionTypeEvent, proposal().Steps[:1]),
		"supported": PlanFingerprint("rules", "1", MissionTypeEvent, changed),
	} {
		if f == a {
			t.Errorf("a different %s must change the fingerprint", name)
		}
	}
	titleOnly := proposal().Steps
	titleOnly[0].Title = "Trouver un lieu"
	if PlanFingerprint("rules", "1", MissionTypeEvent, titleOnly) != a {
		t.Error("a wording change alone is not a different plan")
	}
}

func TestMissionPlan_HasStartedSteps(t *testing.T) {
	m := newTestMission(t)
	p, _ := NewMissionPlan(m, nil, proposal(), "", 1)
	if p.HasStartedSteps() {
		t.Fatal("pending steps have not started")
	}
	p.Steps[0].Status = StepSkipped
	if p.HasStartedSteps() {
		t.Fatal("a skipped step has not started")
	}
	for _, started := range []MissionStepStatus{StepInProgress, StepCompleted, StepFailed} {
		p.Steps[1].Status = started
		if !p.HasStartedSteps() {
			t.Errorf("%s counts as started", started)
		}
	}
}

func TestStepActionSupport_Validation(t *testing.T) {
	m := newTestMission(t)
	s, _ := m.NewStep("VENUE", "Salle", "", missionNow)
	for _, ok := range []StepActionSupport{"", ActionSupported, ActionNotYetSupported} {
		s.ActionSupport = ok
		if err := s.Validate(); err != nil {
			t.Errorf("%q: %v", ok, err)
		}
	}
	s.ActionSupport = "maybe"
	if err := s.Validate(); !errors.Is(err, ErrMissionInvalid) {
		t.Fatalf("unknown support = %v", err)
	}
}

func TestNormalizeMissionType_PlanningNames(t *testing.T) {
	for in, want := range map[string]MissionType{"accommodation": MissionTypeStay, "Moving": MissionTypeMove, "transport": MissionTypeTransport} {
		if got := NormalizeMissionType(in); got != want {
			t.Errorf("NormalizeMissionType(%q) = %q, want %q", in, got, want)
		}
	}
}

func dependentProposal() *PlanProposal {
	return &PlanProposal{Planner: "rules", PlannerVersion: "2", Steps: []PlannedStep{
		{Type: "FIND_ACCOMMODATION", Title: "Find accommodation", ActionType: ActionSearchListings, ActionSupport: ActionSupported},
		{Type: "CHECK_AVAILABILITY", Title: "Check availability", ActionType: ActionCheckAvailability, ActionSupport: ActionSupported, DependsOn: []string{"FIND_ACCOMMODATION"}},
	}}
}

func TestNewMissionPlan_StoresTheDependenciesBetweenItsOwnSteps(t *testing.T) {
	m := newTestMission(t)
	p, err := NewMissionPlan(m, nil, dependentProposal(), "", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dependencies) != 1 || p.Dependencies[0].StepID != p.Steps[1].ID || p.Dependencies[0].DependsOnID != p.Steps[0].ID {
		t.Fatalf("dependencies %+v", p.Dependencies)
	}
	// A new version has its own steps, so its own dependencies: never a link to the old plan's steps.
	next, err := NewMissionPlan(m, p, dependentProposal(), "", 2000)
	if err != nil || next.Dependencies[0].StepID == p.Dependencies[0].StepID || next.Dependencies[0].DependsOnID == p.Dependencies[0].DependsOnID {
		t.Fatalf("the next version links its own steps: %+v %v", next.Dependencies, err)
	}
}

func TestNewMissionPlan_RefusesDependenciesThatCannotBeRun(t *testing.T) {
	m := newTestMission(t)
	for name, mutate := range map[string]func(p *PlanProposal){
		"on a later step":   func(p *PlanProposal) { p.Steps[0].DependsOn = []string{"CHECK_AVAILABILITY"} },
		"on itself":         func(p *PlanProposal) { p.Steps[1].DependsOn = []string{"CHECK_AVAILABILITY"} },
		"on an unknown one": func(p *PlanProposal) { p.Steps[1].DependsOn = []string{"FIND_NOTHING"} },
	} {
		p := dependentProposal()
		mutate(p)
		if _, err := NewMissionPlan(m, nil, p, "", 1); !errors.Is(err, ErrDependencyInvalid) {
			t.Errorf("%s = %v, want ErrDependencyInvalid", name, err)
		}
	}
}

func TestPlanFingerprint_FollowsTheDependencies(t *testing.T) {
	with := PlanFingerprint("rules", "2", MissionTypeStay, dependentProposal().Steps)
	free := dependentProposal().Steps
	free[1].DependsOn = nil
	if with == PlanFingerprint("rules", "2", MissionTypeStay, free) {
		t.Fatal("a plan whose steps wait for each other is not the same plan as one whose steps do not")
	}
}
