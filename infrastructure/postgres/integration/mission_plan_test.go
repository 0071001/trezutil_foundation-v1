package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Mission plans on the real schema (migration 000028): one current plan per mission, versions kept as history,
 * the same request stored once even when it races, steps per plan version, and the database's own constraints.
 */

const (
	pAlice = "d0e50000-0000-4000-8000-0000000000b1"
	pBob   = "d0e50000-0000-4000-8000-0000000000b2"
)

func planningUseCases(db *TestDB) (*application.MissionUseCases, *postgres.MissionRepository) {
	repo := postgres.NewMissionRepository(db.DB)
	supports := func(a domain.CommerceActionType) bool {
		return a == domain.ActionSearchListings || a == domain.ActionCheckAvailability
	}
	return application.NewMissionUseCases(repo).WithPlanning(missionplanning.NewRuleBased(supports), repo), repo
}

func missionRows(t *testing.T, db *TestDB, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMissionPlans_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	ctx := context.Background()
	sellerUser(t, db, ctx, pAlice, "+243810000081")
	sellerUser(t, db, ctx, pBob, "+243810000082")
	uc, repo := planningUseCases(db)

	m, _, err := uc.Create(ctx, pAlice, application.CreateMissionInput{Objective: "Anniversaire pour 40 personnes à Kinshasa"}, "")
	if err != nil {
		t.Fatal(err)
	}

	// No plan yet is a state.
	if res, err := uc.GetPlan(ctx, pAlice, m.ID, 0); err != nil || res.Plan != nil || !res.Plannable {
		t.Fatalf("before planning: %+v %v", res, err)
	}

	// The same request, made by many at once, stores one plan with one set of steps.
	var wg sync.WaitGroup
	created := make([]bool, 8)
	ids := make([]string, 8)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := uc.Plan(ctx, pAlice, m.ID, application.PlanOptions{})
			if err != nil {
				t.Errorf("racing plan %d: %v", i, err)
				return
			}
			created[i], ids[i] = res.Created, res.Plan.ID
		}(i)
	}
	wg.Wait()
	made := 0
	for i, c := range created {
		if c {
			made++
		}
		if ids[i] != ids[0] {
			t.Fatalf("every request is answered with the same plan: %v", ids)
		}
	}
	if made != 1 {
		t.Fatalf("exactly one request stores the plan, %d did", made)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_plans WHERE mission_id = $1`, m.ID); n != 1 {
		t.Fatalf("one plan row, got %d", n)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_steps WHERE mission_id = $1`, m.ID); n != 6 {
		t.Fatalf("one set of six steps, got %d", n)
	}

	plan, err := uc.GetPlan(ctx, pAlice, m.ID, 0)
	if err != nil || plan.Plan.Version != 1 || plan.Plan.Status != domain.MissionPlanCurrent || plan.Plan.MissionType != domain.MissionTypeEvent ||
		plan.Plan.Planner != "rules" || plan.Plan.Fingerprint == "" || len(plan.Plan.Steps) != 6 {
		t.Fatalf("stored plan %+v %v", plan.Plan, err)
	}
	for i, s := range plan.Plan.Steps {
		if s.Position != i+1 || s.PlanVersion != 1 || s.Status != domain.StepPending || s.ActionType != domain.ActionSearchListings ||
			s.ActionSupport != domain.ActionSupported || s.ActionRef != "" || s.ResultRef != "" {
			t.Fatalf("step %d %+v", i, s)
		}
	}
	if got, _ := repo.Get(ctx, m.ID); got.Status != domain.MissionDraft {
		t.Fatalf("planning leaves the mission a draft: %s", got.Status)
	}

	// An idempotency key replays the plan it stored, also when it races.
	travel, _, _ := uc.Create(ctx, pAlice, application.CreateMissionInput{Objective: "Voyage à Nairobi pendant 5 jours"}, "")
	keyed := make([]string, 6)
	for i := range keyed {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, err := uc.Plan(ctx, pAlice, travel.ID, application.PlanOptions{IdempotencyKey: "plan-key"})
			if err != nil {
				t.Errorf("keyed plan %d: %v", i, err)
				return
			}
			keyed[i] = res.Plan.ID
		}(i)
	}
	wg.Wait()
	for _, id := range keyed {
		if id != keyed[0] || id == "" {
			t.Fatalf("racing retries with one key share one plan: %v", keyed)
		}
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_steps WHERE mission_id = $1`, travel.ID); n != 4 {
		t.Fatalf("four travel steps, got %d", n)
	}

	// Replacing: asked for, the old version stays as history, positions restart in the new one.
	moving := "MOVING"
	if _, err := uc.Update(ctx, pAlice, m.ID, application.MissionPatch{Type: &moving}); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Plan(ctx, pAlice, m.ID, application.PlanOptions{}); !errors.Is(err, domain.ErrMissionPlanExists) {
		t.Fatalf("a different plan is not stored silently: %v", err)
	}
	second, err := uc.Plan(ctx, pAlice, m.ID, application.PlanOptions{Replace: true})
	if err != nil || !second.Created || second.Plan.Version != 2 || len(second.Plan.Steps) != 4 || second.Plan.Steps[0].Position != 1 {
		t.Fatalf("replace: %+v %v", second, err)
	}
	if len(second.Versions) != 2 || second.Versions[0] != 1 || second.Versions[1] != 2 {
		t.Fatalf("versions %v", second.Versions)
	}
	old, err := uc.GetPlan(ctx, pAlice, m.ID, 1)
	if err != nil || old.Plan.Status != domain.MissionPlanSuperseded || old.Plan.SupersededAt == 0 || len(old.Plan.Steps) != 6 || old.Plan.MissionType != domain.MissionTypeEvent {
		t.Fatalf("the old version is kept: %+v %v", old, err)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_plans WHERE mission_id = $1 AND status = 'current'`, m.ID); n != 1 {
		t.Fatalf("one current plan, got %d", n)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_steps WHERE mission_id = $1`, m.ID); n != 10 {
		t.Fatalf("6 + 4 steps are kept, got %d", n)
	}
	steps, err := uc.Steps(ctx, pAlice, m.ID)
	if err != nil || len(steps) != 4 || steps[0].PlanVersion != 2 {
		t.Fatalf("the steps list shows the current plan only: %d %v", len(steps), err)
	}
	// Planning the replaced plan again is the same plan, not a third version.
	again, err := uc.Plan(ctx, pAlice, m.ID, application.PlanOptions{Replace: true})
	if err != nil || again.Created || again.Plan.Version != 2 || len(again.Versions) != 2 {
		t.Fatalf("again: %+v %v", again, err)
	}

	// A step added by hand stays apart from the plans: its own limit and positions.
	hand, err := uc.AddStep(ctx, pAlice, m.ID, application.StepInput{Type: "OTHER", Title: "Une étape à moi"})
	if err != nil || hand.Position != 1 || hand.PlanVersion != 0 || hand.ActionSupport != "" {
		t.Fatalf("hand-added step %+v %v", hand, err)
	}
	for i := 1; i < domain.MissionMaxSteps; i++ {
		if _, err := uc.AddStep(ctx, pAlice, m.ID, application.StepInput{Type: "OTHER", Title: "Encore"}); err != nil {
			t.Fatalf("hand-added step %d: %v", i+1, err)
		}
	}
	if _, err := uc.AddStep(ctx, pAlice, m.ID, application.StepInput{Type: "OTHER", Title: "De trop"}); !errors.Is(err, domain.ErrMissionStepLimit) {
		t.Fatalf("the limit counts hand-added steps only, and holds: %v", err)
	}
	if all, _ := uc.Steps(ctx, pAlice, m.ID); len(all) != domain.MissionMaxSteps+4 {
		t.Fatalf("hand-added then current plan steps: %d", len(all))
	}

	// A goal with no template stores nothing.
	other, _, _ := uc.Create(ctx, pAlice, application.CreateMissionInput{Objective: "Je veux organiser quelque chose"}, "")
	res, err := uc.Plan(ctx, pAlice, other.ID, application.PlanOptions{})
	if err != nil || !res.NeedsInformation || res.Plan != nil {
		t.Fatalf("unknown goal: %+v %v", res, err)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM mission_plans WHERE mission_id = $1`, other.ID); n != 0 {
		t.Fatalf("nothing stored for an unplannable goal: %d", n)
	}

	// Ownership.
	if _, err := uc.Plan(ctx, pBob, m.ID, application.PlanOptions{}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Bob planning Alice's mission = %v", err)
	}
	if _, err := uc.GetPlan(ctx, pBob, m.ID, 1); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Bob reading Alice's plan = %v", err)
	}
	if _, err := uc.GetPlan(ctx, pAlice, "d0e50000-0000-4000-8000-0000000000ff", 0); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
	if _, err := repo.PlanVersion(ctx, m.ID, 9); !errors.Is(err, domain.ErrMissionPlanNotFound) {
		t.Fatalf("unknown version = %v", err)
	}
	if _, err := repo.CurrentPlan(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrMissionPlanNotFound) {
		t.Fatalf("malformed id = %v", err)
	}

	// The database refuses what the domain would.
	rawPlan := `INSERT INTO mission_plans (mission_id, version, mission_type, planner, planner_version, status, fingerprint, superseded_at) VALUES ('` + m.ID + `', %V%, 'EVENT', 'rules', '1', '%S%', 'f', %T%)`
	for name, q := range map[string]string{
		"second current plan": strings.NewReplacer("%V%", "3", "%S%", "current", "%T%", "NULL").Replace(rawPlan),
		"repeated version":    strings.NewReplacer("%V%", "1", "%S%", "superseded", "%T%", "NOW()").Replace(rawPlan),
		"version zero":        strings.NewReplacer("%V%", "0", "%S%", "superseded", "%T%", "NOW()").Replace(rawPlan),
		"superseded undated":  strings.NewReplacer("%V%", "4", "%S%", "superseded", "%T%", "NULL").Replace(rawPlan),
		"unknown status":      strings.NewReplacer("%V%", "5", "%S%", "frozen", "%T%", "NULL").Replace(rawPlan),
		"bad support":         `INSERT INTO mission_steps (mission_id, step_type, title, position, plan_version, action_support) VALUES ('` + m.ID + `', 'VENUE', 'T', 99, 2, 'maybe')`,
		"repeated position":   `INSERT INTO mission_steps (mission_id, step_type, title, position, plan_version) VALUES ('` + m.ID + `', 'VENUE', 'T', 1, 2)`,
		"negative version":    `INSERT INTO mission_steps (mission_id, step_type, title, position, plan_version) VALUES ('` + m.ID + `', 'VENUE', 'T', 98, -1)`,
		"unknown mission":     `INSERT INTO mission_plans (mission_id, version, mission_type, planner, planner_version, fingerprint) VALUES ('d0e50000-0000-4000-8000-0000000000ee', 1, 'EVENT', 'rules', '1', 'f')`,
	} {
		if _, err := db.DB.ExecContext(ctx, q); err == nil {
			t.Errorf("the database accepted: %s", name)
		}
	}
	// The same position in another plan version is fine.
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO mission_steps (mission_id, step_type, title, position, plan_version) VALUES ('`+m.ID+`', 'VENUE', 'T', 1, 7)`); err != nil {
		t.Fatalf("positions are per plan version: %v", err)
	}

	// Plans and their steps go with the mission's owner.
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, pAlice); err != nil {
		t.Fatal(err)
	}
	if n := missionRows(t, db, `SELECT (SELECT COUNT(*) FROM mission_plans) + (SELECT COUNT(*) FROM mission_steps)`); n != 0 {
		t.Fatalf("plans and steps outlive their owner: %d", n)
	}
}

// The Down section of the migration undoes it and the Up section can be applied again.
func TestMissionPlans_MigrationDownAndUp(t *testing.T) {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	ctx := context.Background()
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "000028_mission_plans.sql"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	down := content[strings.Index(content, "-- +goose Down")+len("-- +goose Down"):]
	if _, err := db.DB.ExecContext(ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mission_plans'`); n != 0 {
		t.Fatal("down must drop mission_plans")
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'mission_steps' AND column_name IN ('plan_version', 'action_support')`); n != 0 {
		t.Fatal("down must drop the step columns")
	}
	if _, err := db.DB.ExecContext(ctx, extractUpSection(content)); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if n := missionRows(t, db, `SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mission_plans'`); n != 1 {
		t.Fatal("up must create mission_plans again")
	}
}
