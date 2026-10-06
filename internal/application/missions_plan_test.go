package application

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/domain"
)

// The plan side of the in-memory repository (the missions side is in missions_test.go).
type fakePlans struct {
	plans    map[string][]*domain.MissionPlan // by mission, oldest first
	saves    int
	raceOnce func() // runs once inside SavePlan before the version check, to simulate a concurrent request
}

func newFakePlans() *fakePlans { return &fakePlans{plans: map[string][]*domain.MissionPlan{}} }

func (f *fakePlans) current(missionID string) *domain.MissionPlan {
	for _, p := range f.plans[missionID] {
		if p.Status == domain.MissionPlanCurrent {
			return p
		}
	}
	return nil
}
func copyPlan(p *domain.MissionPlan) *domain.MissionPlan {
	c := *p
	c.Steps = append([]domain.MissionStep{}, p.Steps...)
	return &c
}
func (f *fakePlans) CurrentPlan(_ context.Context, id string) (*domain.MissionPlan, error) {
	if p := f.current(id); p != nil {
		return copyPlan(p), nil
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (f *fakePlans) PlanVersion(_ context.Context, id string, v int) (*domain.MissionPlan, error) {
	for _, p := range f.plans[id] {
		if p.Version == v {
			return copyPlan(p), nil
		}
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (f *fakePlans) PlanVersions(_ context.Context, id string) ([]int, error) {
	out := []int{}
	for _, p := range f.plans[id] {
		out = append(out, p.Version)
	}
	sort.Ints(out)
	return out, nil
}
func (f *fakePlans) PlanByIdempotencyKey(_ context.Context, id, key string) (*domain.MissionPlan, error) {
	for _, p := range f.plans[id] {
		if p.IdempotencyKey == key {
			return copyPlan(p), nil
		}
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (f *fakePlans) SavePlan(_ context.Context, plan *domain.MissionPlan, expected int) error {
	if f.raceOnce != nil {
		hook := f.raceOnce
		f.raceOnce = nil
		hook()
	}
	cur := 0
	if p := f.current(plan.MissionID); p != nil {
		cur = p.Version
	}
	if cur != expected {
		return domain.ErrMissionPlanConflict
	}
	if plan.IdempotencyKey != "" {
		for _, p := range f.plans[plan.MissionID] {
			if p.IdempotencyKey == plan.IdempotencyKey {
				return domain.ErrIdempotencyConflict
			}
		}
	}
	if p := f.current(plan.MissionID); p != nil {
		p.Status, p.SupersededAt = domain.MissionPlanSuperseded, plan.CreatedAt
	}
	f.saves++
	f.plans[plan.MissionID] = append(f.plans[plan.MissionID], copyPlan(plan))
	return nil
}

// plannedUseCases wires the use cases with the rule-based planner and a registry that has the search and
// availability actions, like the runtime.
func plannedUseCases() (*MissionUseCases, *fakeMissionRepo, *fakePlans) {
	repo, plans := newFakeMissionRepo(), newFakePlans()
	supports := func(a domain.CommerceActionType) bool {
		return a == domain.ActionSearchListings || a == domain.ActionCheckAvailability
	}
	uc := NewMissionUseCases(repo).WithClock(func() time.Time { return missionClock }).WithPlanning(missionplanning.NewRuleBased(supports), plans)
	return uc, repo, plans
}

func newMission(t *testing.T, uc *MissionUseCases, owner, sentence string) *domain.Mission {
	t.Helper()
	m, _, err := uc.Create(context.Background(), owner, CreateMissionInput{Objective: sentence}, "")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func stepTypesOf(p *domain.MissionPlan) []string {
	var out []string
	for _, s := range p.Steps {
		out = append(out, s.Type)
	}
	return out
}

func TestPlanning_Scenarios(t *testing.T) {
	cases := []struct {
		name, sentence string
		typ            domain.MissionType
		steps          []string
	}{
		{"event", "Anniversaire pour 40 personnes à Kinshasa", domain.MissionTypeEvent,
			[]string{"FIND_VENUE", "FIND_CATERING", "FIND_DECORATION", "FIND_ENTERTAINMENT", "FIND_PHOTOGRAPHY", "FIND_TRANSPORT"}},
		{"travel", "Voyage à Nairobi pendant 5 jours", domain.MissionTypeTravel,
			[]string{"FIND_TRANSPORT", "FIND_ACCOMMODATION", "FIND_LOCAL_TRANSPORT", "FIND_ACTIVITIES"}},
		{"moving", "Je veux organiser mon déménagement", domain.MissionTypeMove,
			[]string{"FIND_ACCOMMODATION", "FIND_MOVING_TRANSPORT", "FIND_MOVERS", "FIND_STORAGE"}},
		{"transport", "J'ai besoin d'un transfert pour l'aéroport", domain.MissionTypeTransport, []string{"FIND_TRANSPORT_OPTION", "CHECK_AVAILABILITY"}},
		{"stay", "Un séjour de 3 jours", domain.MissionTypeStay, []string{"FIND_ACCOMMODATION", "CHECK_AVAILABILITY"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			uc, _, _ := plannedUseCases()
			m := newMission(t, uc, "alice", c.sentence)
			if m.Type != c.typ {
				t.Fatalf("understood as %s, want %s", m.Type, c.typ)
			}
			res, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{})
			if err != nil || !res.Created || res.NeedsInformation || res.Plan == nil {
				t.Fatalf("Plan = %+v, %v", res, err)
			}
			if got := stepTypesOf(res.Plan); len(got) != len(c.steps) || got[0] != c.steps[0] || got[len(got)-1] != c.steps[len(c.steps)-1] {
				t.Fatalf("steps = %v, want %v", got, c.steps)
			}
			if res.Plan.Version != 1 || res.Plan.Status != domain.MissionPlanCurrent || !res.Plannable {
				t.Fatalf("plan %+v", res.Plan)
			}
			for i, s := range res.Plan.Steps {
				if s.Status != domain.StepPending || s.Position != i+1 || s.ResultRef != "" || s.ActionRef != "" {
					t.Fatalf("step %d is an intention, not a result: %+v", i, s)
				}
			}
		})
	}
}

func TestPlanning_UnknownGoalGetsNoInventedPlanOrDetails(t *testing.T) {
	uc, repo, plans := plannedUseCases()
	m := newMission(t, uc, "alice", "Je veux organiser quelque chose")
	if m.Type != domain.MissionTypeOther || m.Location != "" || m.Participants != 0 || m.StartDate != 0 || m.BudgetMinor != 0 {
		t.Fatalf("nothing may be invented: %+v", m)
	}
	res, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.NeedsInformation || res.Reason != domain.ReasonNoTemplate || res.Plan != nil || res.Created || res.Plannable {
		t.Fatalf("an unknown goal asks for more information: %+v", res)
	}
	if plans.saves != 0 || len(plans.plans[m.ID]) != 0 {
		t.Fatal("nothing is stored for a goal that cannot be planned")
	}
	after, _ := repo.Get(context.Background(), m.ID)
	if after.Type != domain.MissionTypeOther || after.Status != domain.MissionDraft || after.Location != "" || after.Participants != 0 {
		t.Fatalf("planning must not change the mission: %+v", after)
	}
}

func TestPlanning_UnderstoodDetailsAreNotInventedAcrossPlanning(t *testing.T) {
	uc, repo, _ := plannedUseCases()
	m := newMission(t, uc, "alice", "Je veux organiser un anniversaire")
	if m.Type != domain.MissionTypeEvent || m.Location != "" || m.Participants != 0 {
		t.Fatalf("only what was said is known: %+v", m)
	}
	if _, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{}); err != nil {
		t.Fatal(err)
	}
	after, _ := repo.Get(context.Background(), m.ID)
	if after.Location != "" || after.Participants != 0 || after.StartDate != 0 || after.BudgetMinor != 0 || after.Currency != "" {
		t.Fatalf("planning must leave the unknown unknown: %+v", after)
	}
}

func TestPlanning_SameRequestGivesTheSamePlan(t *testing.T) {
	uc, _, plans := plannedUseCases()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	first, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{})
	if err != nil || !first.Created {
		t.Fatal(err)
	}
	for i, opts := range []PlanOptions{{}, {Replace: true}, {IdempotencyKey: "k-1"}} {
		again, err := uc.Plan(context.Background(), "alice", m.ID, opts)
		if err != nil || again.Created || again.Plan.ID != first.Plan.ID || again.Plan.Version != 1 {
			t.Fatalf("request %d: %+v %v", i, again, err)
		}
	}
	if plans.saves != 1 || len(plans.plans[m.ID]) != 1 || len(plans.plans[m.ID][0].Steps) != 6 {
		t.Fatalf("one set of steps, whatever the number of requests: saves=%d plans=%d", plans.saves, len(plans.plans[m.ID]))
	}
	if got, _ := uc.GetPlan(context.Background(), "alice", m.ID, 0); got.Plan.ID != first.Plan.ID || len(got.Versions) != 1 {
		t.Fatalf("GetPlan = %+v", got)
	}
}

func TestPlanning_IdempotencyKeyReplaysTheStoredPlan(t *testing.T) {
	uc, _, plans := plannedUseCases()
	m := newMission(t, uc, "alice", "Voyage à Nairobi pendant 5 jours")
	first, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{IdempotencyKey: "k-1"})
	if err != nil || !first.Created || first.Plan.IdempotencyKey != "k-1" {
		t.Fatalf("first: %+v %v", first, err)
	}
	again, err := uc.Plan(context.Background(), "alice", m.ID, PlanOptions{IdempotencyKey: "k-1"})
	if err != nil || again.Created || again.Plan.ID != first.Plan.ID || plans.saves != 1 {
		t.Fatalf("replay: %+v %v saves=%d", again, err, plans.saves)
	}
}

func TestPlanning_ReplacingAPlanIsAskedForAndKeepsHistory(t *testing.T) {
	uc, _, plans := plannedUseCases()
	ctx := context.Background()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	first, _ := uc.Plan(ctx, "alice", m.ID, PlanOptions{})

	// The kind of the mission changes: planning again is a different plan, which is never stored silently.
	travel := "TRAVEL"
	if _, err := uc.Update(ctx, "alice", m.ID, MissionPatch{Type: &travel}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{}); !errors.Is(err, domain.ErrMissionPlanExists) {
		t.Fatalf("a different plan without replace = %v, want ErrMissionPlanExists", err)
	}
	if cur, _ := uc.GetPlan(ctx, "alice", m.ID, 0); cur.Plan.ID != first.Plan.ID || len(cur.Versions) != 1 {
		t.Fatal("a refused replacement changes nothing")
	}

	second, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{Replace: true})
	if err != nil || !second.Created || second.Plan.Version != 2 || second.Plan.MissionType != domain.MissionTypeTravel {
		t.Fatalf("replace: %+v %v", second, err)
	}
	if got := stepTypesOf(second.Plan); got[0] != "FIND_TRANSPORT" || len(got) != 4 {
		t.Fatalf("new steps %v", got)
	}
	if second.Plan.Steps[0].PlanVersion != 2 || second.Plan.Steps[0].Position != 1 {
		t.Fatalf("the new version has its own steps: %+v", second.Plan.Steps[0])
	}
	if len(second.Versions) != 2 || len(plans.plans[m.ID]) != 2 {
		t.Fatalf("both versions are kept: %v", second.Versions)
	}
	old, err := uc.GetPlan(ctx, "alice", m.ID, 1)
	if err != nil || old.Plan.Status != domain.MissionPlanSuperseded || old.Plan.SupersededAt == 0 || len(old.Plan.Steps) != 6 {
		t.Fatalf("the old version stays traceable: %+v %v", old, err)
	}
	if cur, _ := uc.GetPlan(ctx, "alice", m.ID, 0); cur.Plan.Version != 2 || cur.Plan.Status != domain.MissionPlanCurrent {
		t.Fatalf("current = %+v", cur.Plan)
	}
	if _, err := uc.GetPlan(ctx, "alice", m.ID, 9); !errors.Is(err, domain.ErrMissionPlanNotFound) {
		t.Fatalf("an unknown version = %v", err)
	}
}

func TestPlanning_AStartedPlanIsNotReplaced(t *testing.T) {
	uc, _, plans := plannedUseCases()
	ctx := context.Background()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	_, _ = uc.Plan(ctx, "alice", m.ID, PlanOptions{})
	plans.current(m.ID).Steps[0].Status = domain.StepInProgress
	stay := "STAY"
	_, _ = uc.Update(ctx, "alice", m.ID, MissionPatch{Type: &stay})
	if _, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{Replace: true}); !errors.Is(err, domain.ErrMissionPlanInProgress) {
		t.Fatalf("replace with a step under way = %v, want ErrMissionPlanInProgress", err)
	}
}

func TestPlanning_ConcurrentRequestsMakeOnePlan(t *testing.T) {
	uc, _, plans := plannedUseCases()
	ctx := context.Background()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	// Another request stores the same plan between this one's read and its write.
	plans.raceOnce = func() {
		other, _ := uc.planner.Plan(ctx, m)
		p, _ := domain.NewMissionPlan(m, nil, other, "", missionClock.Unix())
		plans.plans[m.ID] = append(plans.plans[m.ID], p)
	}
	res, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{})
	if err != nil || res.Created || res.Plan.Version != 1 || len(plans.plans[m.ID]) != 1 {
		t.Fatalf("the loser is answered with the winner's plan: %+v %v plans=%d", res, err, len(plans.plans[m.ID]))
	}
}

func TestPlanning_LifecycleIsRespected(t *testing.T) {
	uc, repo, _ := plannedUseCases()
	ctx := context.Background()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	if _, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{}); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.Get(ctx, m.ID); got.Status != domain.MissionDraft {
		t.Fatalf("a planned mission is not activated by planning: %s", got.Status)
	}
	paused := domain.MissionActive
	_, _ = uc.Update(ctx, "alice", m.ID, MissionPatch{Status: &paused})
	cancelled := domain.MissionCancelled
	_, _ = uc.Update(ctx, "alice", m.ID, MissionPatch{Status: &cancelled})
	closed := newMission(t, uc, "alice", "Un voyage à Goma")
	_, _ = uc.Update(ctx, "alice", closed.ID, MissionPatch{Status: &cancelled})
	if _, err := uc.Plan(ctx, "alice", closed.ID, PlanOptions{}); !errors.Is(err, domain.ErrMissionClosed) {
		t.Fatalf("a cancelled mission is not planned = %v", err)
	}
	if res, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{IdempotencyKey: "never-used"}); !errors.Is(err, domain.ErrMissionClosed) {
		t.Fatalf("closed, new key = %v %v", res, err)
	}
	if res, err := uc.GetPlan(ctx, "alice", m.ID, 0); err != nil || res.Plan == nil {
		t.Fatalf("a closed mission's plan can still be read: %v", err)
	}
}

func TestPlanning_OnlyTheOwnerPlansOrReads(t *testing.T) {
	uc, _, plans := plannedUseCases()
	ctx := context.Background()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	if res, err := uc.Plan(ctx, "alice", m.ID, PlanOptions{}); err != nil || res.Plan == nil {
		t.Fatalf("owner: %v", err)
	}
	if _, err := uc.Plan(ctx, "bob", m.ID, PlanOptions{}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("another's mission = %v, want ErrMissionForbidden", err)
	}
	if _, err := uc.Plan(ctx, "bob", m.ID, PlanOptions{IdempotencyKey: "k"}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("another's mission with a key = %v", err)
	}
	if _, err := uc.GetPlan(ctx, "bob", m.ID, 0); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("reading another's plan = %v", err)
	}
	if _, err := uc.GetPlan(ctx, "bob", m.ID, 1); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("reading another's plan version = %v", err)
	}
	if _, err := uc.Plan(ctx, "", m.ID, PlanOptions{}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("no actor = %v", err)
	}
	if _, err := uc.Plan(ctx, "alice", "missing", PlanOptions{}); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
	if plans.saves != 1 {
		t.Fatalf("a refused request stores nothing: %d", plans.saves)
	}
}

func TestPlanning_GetPlanBeforeAnyPlanIsAStateNotAnError(t *testing.T) {
	uc, _, _ := plannedUseCases()
	m := newMission(t, uc, "alice", "Anniversaire pour 40 personnes à Kinshasa")
	res, err := uc.GetPlan(context.Background(), "alice", m.ID, 0)
	if err != nil || res.Plan != nil || !res.Plannable || len(res.Versions) != 0 {
		t.Fatalf("no plan yet: %+v %v", res, err)
	}
}

func TestPlanning_NeedsAPlanner(t *testing.T) {
	uc, _ := newMissionUseCases()
	if _, err := uc.Plan(context.Background(), "alice", "x", PlanOptions{}); !errors.Is(err, domain.ErrPlanningUnavailable) {
		t.Fatalf("no planner = %v", err)
	}
	if _, err := uc.GetPlan(context.Background(), "alice", "x", 0); !errors.Is(err, domain.ErrPlanningUnavailable) {
		t.Fatalf("no planner = %v", err)
	}
}
