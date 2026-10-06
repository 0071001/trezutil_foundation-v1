package missionplanning

import (
	"context"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

var runtimeActions = map[domain.CommerceActionType]bool{
	domain.ActionSearchListings: true, domain.ActionCheckAvailability: true, domain.ActionCreateReservation: true, domain.ActionCreateOrder: true,
}

func supportsRuntime(a domain.CommerceActionType) bool { return runtimeActions[a] }

func mission(t *testing.T, typ domain.MissionType) *domain.Mission {
	t.Helper()
	m, err := domain.NewMission("alice", typ, "Titre", "Organiser quelque chose", time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func stepTypes(p *domain.PlanProposal) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Type)
	}
	return out
}

func TestRuleBased_TemplatesPerMissionType(t *testing.T) {
	want := map[domain.MissionType][]string{
		domain.MissionTypeEvent:     {FindVenue, FindCatering, FindDecoration, FindEntertainment, FindPhotography, FindTransport},
		domain.MissionTypeTravel:    {FindTransport, FindAccommodation, FindLocalTransport, FindActivities},
		domain.MissionTypeMove:      {FindAccommodation, FindMovingVehicle, FindMovers, FindStorage},
		domain.MissionTypeTransport: {FindTransportOpt, CheckAvailability},
		domain.MissionTypeStay:      {FindAccommodation, CheckAvailability},
	}
	planner := NewRuleBased(supportsRuntime)
	for typ, types := range want {
		p, err := planner.Plan(context.Background(), mission(t, typ))
		if err != nil || p.NeedsInformation {
			t.Fatalf("%s: %v needs=%v", typ, err, p.NeedsInformation)
		}
		if got := stepTypes(p); !reflect.DeepEqual(got, types) {
			t.Errorf("%s steps = %v, want %v", typ, got, types)
		}
		if !planner.Plannable(typ) {
			t.Errorf("%s must be plannable", typ)
		}
	}
	if got := TemplateTypes(); len(got) != len(want) {
		t.Errorf("TemplateTypes = %v, want the %d planned types", got, len(want))
	}
}

func TestRuleBased_StepsAreIntentionsToAct(t *testing.T) {
	planner := NewRuleBased(supportsRuntime)
	code := regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,31}$`)
	for _, typ := range TemplateTypes() {
		p, _ := planner.Plan(context.Background(), mission(t, typ))
		seen := map[string]bool{}
		for _, s := range p.Steps {
			if !code.MatchString(s.Type) {
				t.Errorf("%s: step type %q is not a code", typ, s.Type)
			}
			if seen[s.Type] {
				t.Errorf("%s: step type %s twice", typ, s.Type)
			}
			seen[s.Type] = true
			if len(strings.Fields(s.Title)) < 2 || strings.EqualFold(s.Title, "Find something") {
				t.Errorf("%s: step %s has a vague title %q", typ, s.Type, s.Title)
			}
			if s.ActionType == "" || s.ActionSupport != domain.ActionSupported {
				t.Errorf("%s: step %s names action %q (%s): every template step names a runtime action", typ, s.Type, s.ActionType, s.ActionSupport)
			}
			if s.ActionType != domain.ActionSearchListings && s.ActionType != domain.ActionCheckAvailability {
				t.Errorf("%s: step %s names %s, but a plan only finds and checks: it never reserves, orders or pays", typ, s.Type, s.ActionType)
			}
		}
	}
}

func TestRuleBased_DoesNotInventDetails(t *testing.T) {
	planner := NewRuleBased(supportsRuntime)
	detail := regexp.MustCompile(`(?i)\d|kinshasa|nairobi|goma|saturday|samedi|\$|usd|budget`)
	for _, typ := range TemplateTypes() {
		p, _ := planner.Plan(context.Background(), mission(t, typ))
		for _, s := range p.Steps {
			if detail.MatchString(s.Title + " " + s.Description) {
				t.Errorf("%s/%s: %q %q states a detail nobody gave", typ, s.Type, s.Title, s.Description)
			}
		}
	}
}

func TestRuleBased_NoTemplateMeansMoreInformationNotMadeUpSteps(t *testing.T) {
	planner := NewRuleBased(supportsRuntime)
	for _, typ := range []domain.MissionType{domain.MissionTypeOther, domain.MissionTypeBusinessTrip, domain.MissionTypeProject} {
		m := mission(t, typ)
		before := *m
		p, err := planner.Plan(context.Background(), m)
		if err != nil || !p.NeedsInformation || p.Reason != domain.ReasonNoTemplate || len(p.Steps) != 0 {
			t.Fatalf("%s: %+v %v", typ, p, err)
		}
		if planner.Plannable(typ) {
			t.Errorf("%s must not be plannable", typ)
		}
		if !reflect.DeepEqual(*m, before) {
			t.Errorf("%s: planning must not change the mission", typ)
		}
	}
}

func TestRuleBased_MarksActionsTheRuntimeLacksAsNotYetSupported(t *testing.T) {
	asked := map[domain.CommerceActionType]bool{}
	onlySearch := func(a domain.CommerceActionType) bool { asked[a] = true; return a == domain.ActionSearchListings }
	p, _ := NewRuleBased(onlySearch).Plan(context.Background(), mission(t, domain.MissionTypeStay))
	if p.Steps[0].ActionSupport != domain.ActionSupported || p.Steps[1].ActionSupport != domain.ActionNotYetSupported {
		t.Fatalf("support = %s, %s", p.Steps[0].ActionSupport, p.Steps[1].ActionSupport)
	}
	if p.Steps[1].ActionType != domain.ActionCheckAvailability {
		t.Fatal("the step keeps the action it will need, marked not yet supported, rather than a made-up execution")
	}
	if len(asked) != 2 {
		t.Fatalf("the registry is only asked about the actions of the steps: %v", asked)
	}
	none, _ := NewRuleBased(nil).Plan(context.Background(), mission(t, domain.MissionTypeStay))
	for _, s := range none.Steps {
		if s.ActionSupport != domain.ActionNotYetSupported {
			t.Fatalf("with no registry nothing counts as supported: %+v", s)
		}
	}
}

func TestRuleBased_IsPureAndRepeatable(t *testing.T) {
	planner := NewRuleBased(supportsRuntime)
	m := mission(t, domain.MissionTypeEvent)
	a, _ := planner.Plan(context.Background(), m)
	b, _ := planner.Plan(context.Background(), m)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same mission gives the same proposal")
	}
	fa := domain.PlanFingerprint(a.Planner, a.PlannerVersion, m.Type, a.Steps)
	fb := domain.PlanFingerprint(b.Planner, b.PlannerVersion, m.Type, b.Steps)
	if fa != fb || len(fa) != 32 {
		t.Fatalf("fingerprints %q %q", fa, fb)
	}
	other, _ := planner.Plan(context.Background(), mission(t, domain.MissionTypeTravel))
	if domain.PlanFingerprint(other.Planner, other.PlannerVersion, domain.MissionTypeTravel, other.Steps) == fa {
		t.Fatal("another kind of mission is another plan")
	}
	if planner.Name() != "rules" || planner.Version() == "" {
		t.Fatal("the planner names itself")
	}
}

// Phase 4: the templates carry only the dependencies that are real data dependencies. An availability check needs the
// listing the search found, so it waits for that search; nothing else in a template waits for anything.
func TestRuleBased_TheAvailabilityCheckWaitsForItsSearch(t *testing.T) {
	planner := NewRuleBased(supportsRuntime)
	want := map[domain.MissionType]string{domain.MissionTypeStay: FindAccommodation, domain.MissionTypeTransport: FindTransportOpt}
	for _, typ := range TemplateTypes() {
		p, _ := planner.Plan(context.Background(), mission(t, typ))
		for _, s := range p.Steps {
			if s.Type == CheckAvailability {
				if !reflect.DeepEqual(s.DependsOn, []string{want[typ]}) {
					t.Errorf("%s: the check depends on %v, want %s", typ, s.DependsOn, want[typ])
				}
			} else if len(s.DependsOn) != 0 {
				t.Errorf("%s: %s must not wait for anything (%v)", typ, s.Type, s.DependsOn)
			}
		}
		// A plan built from the proposal is a valid graph.
		plan, err := domain.NewMissionPlan(mission(t, typ), nil, p, "", 1)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if err := domain.ValidateGraph(plan.Steps, plan.Dependencies); err != nil {
			t.Errorf("%s: %v", typ, err)
		}
	}
	if TemplatesVersion != "2" {
		t.Errorf("the templates changed (dependencies), so the version must too: %s", TemplatesVersion)
	}
}
