package integration

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lib/pq"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Multi-step orchestration on the real schema (migration 000030): the dependency graph and its constraints, the run
 * lock, the orchestrator over HTTP with the REAL Commerce Action Runtime, and ten simultaneous orchestrations.
 */

func (f *execFixture) stay(user string, withDates bool) (string, map[string]string) {
	f.t.Helper()
	body := `{"objective":"Un séjour à Kinshasa pour 2 personnes","type":"STAY","location":"Kinshasa","participants":2`
	if withDates {
		body += `,"start_date":"2026-11-10","end_date":"2026-11-13"`
	}
	code, m := f.call(user, "POST", "/api/v1/missions", body+`}`)
	if code != http.StatusCreated {
		f.t.Fatalf("create = %d %v", code, m)
	}
	id := m["id"].(string)
	code, p := f.call(user, "POST", "/api/v1/missions/"+id+"/plan", "")
	if code != http.StatusCreated {
		f.t.Fatalf("plan = %d %v", code, p)
	}
	steps := map[string]string{}
	for _, raw := range p["steps"].([]interface{}) {
		s := raw.(map[string]interface{})
		steps[s["type"].(string)] = s["id"].(string)
	}
	return id, steps
}

func (f *execFixture) run(user, mission string, path string, body string) (int, map[string]interface{}) {
	return f.call(user, "POST", "/api/v1/missions/"+mission+"/"+path, body)
}

func stepStates(body map[string]interface{}) map[string]map[string]interface{} {
	out := map[string]map[string]interface{}{}
	for _, raw := range body["steps"].([]interface{}) {
		s := raw.(map[string]interface{})
		out[s["type"].(string)] = s
	}
	return out
}

func sqlState(err error) string {
	var pe *pq.Error
	if errors.As(err, &pe) {
		return string(pe.Code)
	}
	return ""
}

func TestOrchestration_WholeMissionOnTheRealSchema(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	search, check := steps["FIND_ACCOMMODATION"], steps["CHECK_AVAILABILITY"]

	// The plan stored its graph: the check waits for the search, and the steps list says so.
	if f.n(`SELECT COUNT(*) FROM mission_step_dependencies WHERE mission_id = $1 AND step_id = $2 AND depends_on_step_id = $3 AND plan_version = 1`, mission, check, search) != 1 {
		t.Fatal("the dependency is stored with the plan")
	}
	code, before := f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/execution", "")
	if code != http.StatusOK || before["outcome"] != "ready" || before["busy"] != false {
		t.Fatalf("before = %d %v", code, before)
	}
	st := stepStates(before)
	if st["FIND_ACCOMMODATION"]["state"] != "ready" || st["CHECK_AVAILABILITY"]["state"] != "pending" || st["CHECK_AVAILABILITY"]["reason"] != "waiting" ||
		st["CHECK_AVAILABILITY"]["depends_on"].([]interface{})[0] != search {
		t.Fatalf("before: %v", st)
	}
	if f.search.count() != 0 {
		t.Fatal("a read runs nothing")
	}

	code, res := f.run(xAlice, mission, "run", "")
	if code != http.StatusOK || res["outcome"] != "completed" || res["mission_status"] != "completed" || res["busy"] != false {
		t.Fatalf("run = %d %v", code, res)
	}
	if p := res["progress"].(map[string]interface{}); p["completed"] != float64(2) || p["total"] != float64(2) {
		t.Fatalf("progress %v", p)
	}
	if ran := res["ran"].([]interface{}); len(ran) != 2 || ran[0].(map[string]interface{})["step_id"] != search || ran[1].(map[string]interface{})["step_id"] != check {
		t.Fatalf("the search ran first, then the check: %v", res["ran"])
	}
	st = stepStates(res)
	if st["FIND_ACCOMMODATION"]["state"] != "completed" || st["CHECK_AVAILABILITY"]["state"] != "completed" {
		t.Fatalf("states %v", st)
	}
	// What the check asked the real runtime: the first listing the search found, the mission's dates and head count.
	if f.avail.count() != 1 {
		t.Fatalf("availability asked %d times", f.avail.count())
	}
	a := f.avail.asked[0]
	if a["listing_id"] != "l-1" || a["party_size"] != 2 || a["end_at"] == a["start_at"] {
		t.Fatalf("asked %v", a)
	}
	if res := st["CHECK_AVAILABILITY"]["execution"].(map[string]interface{})["result"].(map[string]interface{}); res["available"] != true || res["reference"] != "l-1" || res["listing_id"] != "l-1" {
		t.Fatalf("the check keeps a summary and the listing as reference: %v", res)
	}
	var payload string
	if err := f.db.DB.QueryRowContext(f.ctx, `SELECT result_payload::text FROM mission_step_executions WHERE step_id = $1`, check).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, `"listing_id": "l-1"`) || strings.Contains(payload, "Title") {
		t.Fatalf("durable payload %s", payload)
	}
	if f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'completed'`, mission) != 1 || f.n(`SELECT COUNT(*) FROM mission_run_locks WHERE mission_id = $1`, mission) != 0 {
		t.Fatal("the mission is completed and the lock released")
	}
	// Credits: the runtime charged the metered search once; nothing else touched them.
	if f.credits.reserved != 1 || f.credits.consumed != 1 || f.credits.released != 0 || f.credits.balance != 98 {
		t.Fatalf("credits reserved=%d consumed=%d released=%d balance=%d", f.credits.reserved, f.credits.consumed, f.credits.released, f.credits.balance)
	}

	// Repeating the orchestration runs nothing, charges nothing, and is not an error.
	for i := 0; i < 3; i++ {
		for _, path := range []string{"run", "resume"} {
			code, again := f.run(xAlice, mission, path, "")
			if code != http.StatusOK || again["outcome"] != "completed" || len(again["ran"].([]interface{})) != 0 {
				t.Fatalf("%s again = %d %v", path, code, again)
			}
		}
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE mission_id = $1`, mission) != 2 || f.search.count() != 1 || f.avail.count() != 1 || f.credits.reserved != 1 {
		t.Fatal("a completed step is never run again")
	}
}

func TestOrchestration_FailureBlocksDependentsAndResumeCompletesWithoutRepeatingWhatCompleted(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	search, check := steps["FIND_ACCOMMODATION"], steps["CHECK_AVAILABILITY"]

	f.search.set(errors.New("opensearch: connection refused (10.1.2.3:9200)"))
	code, res := f.run(xAlice, mission, "run", "")
	if code != http.StatusOK || res["outcome"] != "needs_attention" || res["mission_status"] != "active" {
		t.Fatalf("run = %d %v", code, res)
	}
	st := stepStates(res)
	if st["FIND_ACCOMMODATION"]["state"] != "failed" || st["CHECK_AVAILABILITY"]["state"] != "blocked" || st["CHECK_AVAILABILITY"]["reason"] != "dependency_failed" ||
		st["CHECK_AVAILABILITY"]["blocked_by"].([]interface{})[0] != search {
		t.Fatalf("states %v", st)
	}
	if e := st["FIND_ACCOMMODATION"]["execution"].(map[string]interface{})["error"].(map[string]interface{}); e["code"] != "ACTION_FAILED" {
		t.Fatalf("a stable code: %v", e)
	}
	if strings.Contains(strings.ToLower(toJSON(res)), "10.1.2.3") {
		t.Fatal("the backend's message never reaches the client")
	}
	if f.avail.count() != 0 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, check) != 0 {
		t.Fatal("a blocked step never reaches the runtime")
	}
	if f.credits.reserved != 1 || f.credits.released != 1 || f.credits.consumed != 0 || f.credits.balance != 100 {
		t.Fatalf("a failed action keeps the credits: %d %d %d %d", f.credits.reserved, f.credits.released, f.credits.consumed, f.credits.balance)
	}
	// A plain run does not retry the failed step.
	if code, again := f.run(xAlice, mission, "run", ""); code != http.StatusOK || again["outcome"] != "needs_attention" || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, search) != 1 {
		t.Fatalf("run retried: %d %v", code, again)
	}

	// The cause is gone: a resume retries the failed step once, then the step that was waiting for it.
	f.search.set(nil)
	code, res = f.run(xAlice, mission, "resume", "")
	if code != http.StatusOK || res["outcome"] != "completed" || res["mission_status"] != "completed" {
		t.Fatalf("resume = %d %v", code, res)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, search) != 2 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, check) != 1 {
		t.Fatal("a retry is the next attempt, and the dependent ran once")
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1 AND status = 'failed' AND attempt = 1`, search) != 1 {
		t.Fatal("the failed attempt stays as history")
	}
	if f.credits.reserved != 2 || f.credits.released != 1 || f.credits.consumed != 1 || f.credits.balance != 98 {
		t.Fatalf("charged for the success only: %d %d %d %d", f.credits.reserved, f.credits.released, f.credits.consumed, f.credits.balance)
	}
}

func TestOrchestration_AMissingInputBlocksUntilThePersonProvidesIt(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, false) // no dates
	check := steps["CHECK_AVAILABILITY"]
	code, res := f.run(xAlice, mission, "run", "")
	st := stepStates(res)
	if code != http.StatusOK || res["outcome"] != "needs_attention" || res["mission_status"] != "active" ||
		st["FIND_ACCOMMODATION"]["state"] != "completed" || st["CHECK_AVAILABILITY"]["state"] != "blocked" ||
		st["CHECK_AVAILABILITY"]["reason"] != "needs_input" || st["CHECK_AVAILABILITY"]["detail"] != "dates" {
		t.Fatalf("no dates = %d %v", code, st)
	}
	if f.avail.count() != 0 {
		t.Fatal("a date is never made up")
	}
	// Running the step by hand says the same thing.
	if code, body := f.exec(xAlice, mission, check); code != http.StatusConflict || xErr(body) != "STEP_NEEDS_INPUT" {
		t.Fatalf("execute by hand = %d %v", code, body)
	}
	// The person gives the dates; a run finishes the mission without searching again.
	if code, body := f.call(xAlice, "PATCH", "/api/v1/missions/"+mission, `{"start_date":"2026-11-10","end_date":"2026-11-12"}`); code != http.StatusOK {
		t.Fatalf("patch = %d %v", code, body)
	}
	code, res = f.run(xAlice, mission, "run", "")
	if code != http.StatusOK || res["outcome"] != "completed" || f.search.count() != 1 || f.avail.count() != 1 {
		t.Fatalf("after the dates = %d %v searches=%d", code, res["outcome"], f.search.count())
	}
}

func TestOrchestration_StepsAndOrchestrationAgreeOnWaiting(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	// Running the dependent by hand before its search is refused by the executor itself, not only by the orchestrator.
	if code, body := f.exec(xAlice, mission, steps["CHECK_AVAILABILITY"]); code != http.StatusConflict || xErr(body) != "STEP_WAITING" {
		t.Fatalf("execute out of order = %d %v", code, body)
	}
	if f.search.count() != 0 || f.avail.count() != 0 {
		t.Fatal("nothing reached the runtime")
	}
	// The steps list carries the same states.
	_, list := f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", "")
	for _, raw := range list["items"].([]interface{}) {
		s := raw.(map[string]interface{})
		if s["type"] == "CHECK_AVAILABILITY" && (s["state"] != "pending" || s["can_execute"] != false) {
			t.Fatalf("the list says %v", s)
		}
	}
	// Run the search by hand, then the dependent: the same graph is respected.
	if code, body := f.exec(xAlice, mission, steps["FIND_ACCOMMODATION"]); code != http.StatusCreated {
		t.Fatalf("search = %d %v", code, body)
	}
	if code, body := f.exec(xAlice, mission, steps["CHECK_AVAILABILITY"]); code != http.StatusCreated || body["status"] != "completed" {
		t.Fatalf("check = %d %v", code, body)
	}
	// All steps were completed one by one while the mission was a draft: a run only completes it.
	code, res := f.run(xAlice, mission, "run", "")
	if code != http.StatusOK || res["mission_status"] != "completed" || len(res["ran"].([]interface{})) != 0 || f.search.count() != 1 {
		t.Fatalf("run = %d %v", code, res)
	}
}

func TestOrchestration_OwnershipAndErrorsOnTheRealSchema(t *testing.T) {
	f := newExecFixture(t)
	mission, _ := f.stay(xAlice, true)
	for _, c := range [][2]string{{"POST", "run"}, {"POST", "resume"}, {"GET", "execution"}} {
		path := "/api/v1/missions/" + mission + "/" + c[1]
		if code, body := f.call(xBob, c[0], path, ""); code != http.StatusForbidden || xErr(body) != "FORBIDDEN" {
			t.Errorf("User B %s = %d %v", c[1], code, body)
		}
		if code, _ := f.call("", c[0], path, ""); code != http.StatusUnauthorized {
			t.Errorf("anonymous %s = %d", c[1], code)
		}
		if code, body := f.call(xAlice, c[0], "/api/v1/missions/6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f0d21/"+c[1], ""); code != http.StatusNotFound || xErr(body) != "NOT_FOUND" {
			t.Errorf("unknown mission %s = %d %v", c[1], code, body)
		}
	}
	if f.search.count() != 0 || f.n(`SELECT COUNT(*) FROM mission_run_locks`) != 0 || f.n(`SELECT COUNT(*) FROM mission_step_executions`) != 0 {
		t.Fatal("a refused request ran nothing and locked nothing")
	}
	// A mission without a plan.
	_, m := f.call(xAlice, "POST", "/api/v1/missions", `{"objective":"Un séjour à Goma","type":"STAY"}`)
	if code, body := f.run(xAlice, m["id"].(string), "run", ""); code != http.StatusConflict || xErr(body) != "PLAN_REQUIRED" {
		t.Fatalf("no plan = %d %v", code, body)
	}
	// Paused and cancelled missions are not run; a completed one answers with its state.
	for _, status := range []string{"paused", "cancelled"} {
		id, _ := f.stay(xAlice, true)
		f.setStatus(id, status)
		if code, body := f.run(xAlice, id, "run", ""); code != http.StatusConflict || xErr(body) != "MISSION_NOT_RUNNABLE" {
			t.Errorf("%s mission = %d %v", status, code, body)
		}
	}
	if f.search.count() != 0 {
		t.Fatal("nothing ran for a paused or cancelled mission")
	}
}

func (f *execFixture) setStatus(mission, status string) {
	f.t.Helper()
	if _, err := f.db.DB.ExecContext(f.ctx, `UPDATE missions SET status = $2 WHERE id = $1`, mission, status); err != nil {
		f.t.Fatal(err)
	}
}

func TestOrchestration_ACircularGraphIsRefusedBeforeAnythingRuns(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	search, check := steps["FIND_ACCOMMODATION"], steps["CHECK_AVAILABILITY"]
	// The database cannot exclude a cycle by a constraint, so one inserted behind the application's back is detected when
	// the plan is read, and the mission is not run.
	if _, err := f.db.DB.ExecContext(f.ctx, `INSERT INTO mission_step_dependencies (mission_id, plan_version, step_id, depends_on_step_id) VALUES ($1, 1, $2, $3)`, mission, search, check); err != nil {
		t.Fatalf("the cycle is storable: %v", err)
	}
	for _, c := range [][2]string{{"POST", "run"}, {"POST", "resume"}, {"GET", "execution"}} {
		code, body := f.call(xAlice, c[0], "/api/v1/missions/"+mission+"/"+c[1], "")
		if code != http.StatusConflict || xErr(body) != "MISSION_GRAPH_INVALID" {
			t.Errorf("%s with a cycle = %d %v", c[1], code, body)
		}
	}
	if f.search.count() != 0 || f.n(`SELECT COUNT(*) FROM mission_step_executions`) != 0 || f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'draft'`, mission) != 1 {
		t.Fatal("nothing ran from a graph that cannot be ordered")
	}
	// Executing a step by hand does not bypass the graph either: the listing of steps still works.
	if code, body := f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", ""); code != http.StatusOK || len(body["items"].([]interface{})) != 2 {
		t.Fatalf("the steps are still listed: %d %v", code, body)
	}
}

func TestOrchestration_DependencyConstraintsOnTheRealSchema(t *testing.T) {
	f := newExecFixture(t)
	missionA, stepsA := f.stay(xAlice, true)
	missionB, stepsB := f.stay(xBob, true)
	a1, a2 := stepsA["FIND_ACCOMMODATION"], stepsA["CHECK_AVAILABILITY"]
	b1 := stepsB["FIND_ACCOMMODATION"]
	insert := func(mission string, version int, step, on string) error {
		_, err := f.db.DB.ExecContext(f.ctx, `INSERT INTO mission_step_dependencies (mission_id, plan_version, step_id, depends_on_step_id) VALUES ($1, $2, $3, $4)`, mission, version, step, on)
		return err
	}
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"a step cannot depend on itself":              {insert(missionA, 1, a1, a1), "23514"},
		"the same dependency is stored once":          {insert(missionA, 1, a2, a1), "23505"},
		"a dependency on a step of another mission":   {insert(missionA, 1, a1, b1), "23503"},
		"a dependency of a step of another mission":   {insert(missionB, 1, a1, b1), "23503"},
		"a dependency on a step that does not exist":  {insert(missionA, 1, a1, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f0d21"), "23503"},
		"a dependency on another plan version":        {insert(missionA, 2, a1, a2), "23503"},
		"a hand-added step (plan version 0) has none": {insert(missionA, 0, a1, a2), "23514"},
		"a mission that does not exist":               {insert("6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f0d21", 1, a1, a2), "23503"},
	} {
		if got := sqlState(c.err); got != c.want {
			t.Errorf("%s: SQLSTATE %q (%v), want %s", name, got, c.err, c.want)
		}
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_dependencies`) != 2 {
		t.Fatal("only the two plans' own dependencies are stored")
	}
	// The graph follows its steps and its mission.
	if _, err := f.db.DB.ExecContext(f.ctx, `DELETE FROM missions WHERE id = $1`, missionA); err != nil {
		t.Fatal(err)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_dependencies WHERE mission_id = $1`, missionA) != 0 || f.n(`SELECT COUNT(*) FROM mission_step_dependencies WHERE mission_id = $1`, missionB) != 1 {
		t.Fatal("deleting a mission removes its dependencies, and only its own")
	}
	// The identity constraint the foreign keys rest on.
	if f.n(`SELECT COUNT(*) FROM pg_constraint WHERE conname = 'uq_mission_steps_identity'`) != 1 {
		t.Fatal("uq_mission_steps_identity")
	}
}

func TestOrchestration_PlansKeepTheirOwnGraphAcrossVersions(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	ctx := context.Background()
	m, err := f.repo.Get(ctx, mission)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := f.repo.CurrentPlan(ctx, mission)
	if err != nil || v1.Version != 1 || len(v1.Dependencies) != 1 || v1.Dependencies[0].StepID != steps["CHECK_AVAILABILITY"] || v1.Dependencies[0].DependsOnID != steps["FIND_ACCOMMODATION"] {
		t.Fatalf("the loaded plan carries its dependencies: %+v %v", v1, err)
	}
	// A new version (built by the domain, as a planner's proposal would be) has its own steps and its own graph.
	v2, err := domain.NewMissionPlan(m, v1, &domain.PlanProposal{Planner: "rules", PlannerVersion: "3", Steps: []domain.PlannedStep{
		{Type: "FIND_ACCOMMODATION", Title: "Find accommodation", ActionType: domain.ActionSearchListings, ActionSupport: domain.ActionSupported},
		{Type: "CHECK_AVAILABILITY", Title: "Check availability", ActionType: domain.ActionCheckAvailability, ActionSupport: domain.ActionSupported, DependsOn: []string{"FIND_ACCOMMODATION"}},
	}}, "", time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.repo.SavePlan(ctx, v2, 1); err != nil {
		t.Fatal(err)
	}
	cur, err := f.repo.CurrentPlan(ctx, mission)
	if err != nil || cur.Version != 2 || len(cur.Dependencies) != 1 || cur.Dependencies[0].StepID == v1.Dependencies[0].StepID {
		t.Fatalf("version 2 has its own graph: %+v %v", cur, err)
	}
	old, err := f.repo.PlanVersion(ctx, mission, 1)
	if err != nil || len(old.Dependencies) != 1 || old.Dependencies[0] != v1.Dependencies[0] {
		t.Fatalf("version 1 keeps its graph as history: %+v %v", old, err)
	}
	deps, err := f.repo.PlanDependencies(ctx, mission, 2)
	if err != nil || len(deps) != 1 || deps[0] != cur.Dependencies[0] {
		t.Fatalf("PlanDependencies: %+v %v", deps, err)
	}
	up, err := f.repo.Upstream(ctx, cur.Dependencies[0].StepID)
	if err != nil || len(up) != 1 || up[0].Step.ID != cur.Dependencies[0].DependsOnID || up[0].Execution != nil {
		t.Fatalf("Upstream: %+v %v", up, err)
	}
	// Only the current plan is run.
	code, res := f.run(xAlice, mission, "run", "")
	if code != http.StatusOK || res["plan_version"] != float64(2) || res["outcome"] != "completed" {
		t.Fatalf("run = %d %v", code, res)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions e JOIN mission_steps s ON s.id = e.step_id WHERE s.mission_id = $1 AND s.plan_version = 1`, mission) != 0 {
		t.Fatal("the replaced plan's steps are history: never run")
	}
}

func TestOrchestration_TenSimultaneousRunsExecuteEachStepOnce(t *testing.T) {
	f := newExecFixture(t)
	f.search.mu.Lock()
	f.search.delay = 60 * time.Millisecond
	f.search.mu.Unlock()
	mission, _ := f.stay(xAlice, true)

	const n = 10
	codes := make([]int, n)
	bodies := make([]map[string]interface{}, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			codes[i], bodies[i] = f.run(xAlice, mission, "run", "")
		}(i)
	}
	close(start)
	wg.Wait()

	accepted, ran := 0, 0
	for i := range codes {
		switch codes[i] {
		case http.StatusOK:
			ran += len(bodies[i]["ran"].([]interface{}))
		case http.StatusAccepted:
			accepted++
			// Busy means another orchestration holds the mission and this request ran nothing. The outcome is that of the steps
			// as read: "in_progress" while there is work left, "completed" if the holder finished before this was read.
			if bodies[i]["busy"] != true || len(bodies[i]["ran"].([]interface{})) != 0 || (bodies[i]["outcome"] != "in_progress" && bodies[i]["outcome"] != "completed") {
				t.Fatalf("a busy answer runs nothing: %v", bodies[i])
			}
		default:
			t.Fatalf("orchestration %d = %d %v", i, codes[i], bodies[i])
		}
	}
	if accepted == 0 {
		t.Fatal("the others were told the mission is being run")
	}
	if ran != 2 || f.search.count() != 1 || f.avail.count() != 1 {
		t.Fatalf("reported %d steps, %d searches, %d checks: each step runs once", ran, f.search.count(), f.avail.count())
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE mission_id = $1`, mission) != 2 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE mission_id = $1 AND status = 'completed'`, mission) != 2 {
		t.Fatal("two executions, both completed")
	}
	if f.credits.reserved != 1 || f.credits.consumed != 1 || f.credits.balance != 98 {
		t.Fatalf("one charge: reserved=%d consumed=%d balance=%d", f.credits.reserved, f.credits.consumed, f.credits.balance)
	}
	if f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'completed'`, mission) != 1 || f.n(`SELECT COUNT(*) FROM mission_run_locks`) != 0 {
		t.Fatal("completed once, lock released")
	}
}

func TestOrchestration_TenSimultaneousRunsOfTenMissionsDoNotInterfere(t *testing.T) {
	f := newExecFixture(t)
	f.search.mu.Lock()
	f.search.delay = 20 * time.Millisecond
	f.search.mu.Unlock()
	ids := make([]string, 10)
	for i := range ids {
		ids[i], _ = f.stay(xAlice, true)
	}
	var wg sync.WaitGroup
	for _, id := range ids {
		for k := 0; k < 2; k++ { // two orchestrations per mission, all at once
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				if code, body := f.run(xAlice, id, "run", ""); code != http.StatusOK && code != http.StatusAccepted {
					t.Errorf("run = %d %v", code, body)
				}
			}(id)
		}
	}
	wg.Wait()
	if f.n(`SELECT COUNT(*) FROM missions WHERE status = 'completed'`) != 10 || f.n(`SELECT COUNT(*) FROM mission_step_executions`) != 20 || f.search.count() != 10 || f.avail.count() != 10 {
		t.Fatalf("each of the ten missions ran its two steps once: executions=%d searches=%d checks=%d", f.n(`SELECT COUNT(*) FROM mission_step_executions`), f.search.count(), f.avail.count())
	}
}

// The step claim holds even if two orchestrations somehow both run (the lock is the first line of defence only).
func TestOrchestration_TheStepClaimHoldsWithoutTheLock(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.stay(xAlice, true)
	f.search.mu.Lock()
	f.search.delay = 50 * time.Millisecond
	f.search.mu.Unlock()
	var wg sync.WaitGroup
	codes := make([]int, 8)
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes[i], _ = f.exec(xAlice, mission, steps["FIND_ACCOMMODATION"], "Idempotency-Key", "k-"+string(rune('a'+i)))
		}(i)
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		if c == http.StatusCreated {
			created++
		} else if c != http.StatusConflict {
			t.Errorf("status %d", c)
		}
	}
	if created != 1 || f.search.count() != 1 || f.credits.reserved != 1 {
		t.Fatalf("one attempt wins: created=%d searches=%d reserved=%d", created, f.search.count(), f.credits.reserved)
	}
}

func TestRunLock_OnPostgres(t *testing.T) {
	f := newExecFixture(t)
	mission, _ := f.stay(xAlice, true)
	ctx := context.Background()
	now := time.Now()

	// Of twenty simultaneous attempts, one takes the lock.
	var won int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ok, err := f.repo.AcquireRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f00"+string(rune('0'+i/10))+string(rune('0'+i%10)), now, time.Minute)
			if err != nil {
				t.Errorf("acquire: %v", err)
			}
			if ok {
				mu.Lock()
				won++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if won != 1 {
		t.Fatalf("%d orchestrations took the lock", won)
	}
	if held, _ := f.repo.RunInProgress(ctx, mission, now); !held {
		t.Fatal("held")
	}
	if held, _ := f.repo.RunInProgress(ctx, mission, now.Add(2*time.Minute)); held {
		t.Fatal("a lease that ran out is not a hold")
	}
	var holder string
	if err := f.db.DB.QueryRowContext(ctx, `SELECT token::text FROM mission_run_locks WHERE mission_id = $1`, mission).Scan(&holder); err != nil {
		t.Fatal(err)
	}
	// Only the holder extends or releases; another holder's release does nothing.
	if err := f.repo.ExtendRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9fffff", now, time.Minute); !errors.Is(err, domain.ErrMissionBusy) {
		t.Fatalf("a stranger extends = %v", err)
	}
	if err := f.repo.ReleaseRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9fffff"); err != nil || f.n(`SELECT COUNT(*) FROM mission_run_locks`) != 1 {
		t.Fatalf("a stranger releases = %v", err)
	}
	if err := f.repo.ExtendRun(ctx, mission, holder, now, 10*time.Minute); err != nil {
		t.Fatalf("the holder extends: %v", err)
	}
	if ok, _ := f.repo.AcquireRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9fffff", now.Add(5*time.Minute), time.Minute); ok {
		t.Fatal("the extended lease still holds")
	}
	// The holder died: after the lease another takes over, and the old holder can no longer extend.
	if ok, err := f.repo.AcquireRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9fffff", now.Add(11*time.Minute), time.Minute); err != nil || !ok {
		t.Fatalf("takeover = %v %v", ok, err)
	}
	if err := f.repo.ExtendRun(ctx, mission, holder, now.Add(11*time.Minute), time.Minute); !errors.Is(err, domain.ErrMissionBusy) {
		t.Fatalf("the old holder extends = %v", err)
	}
	if err := f.repo.ReleaseRun(ctx, mission, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9fffff"); err != nil || f.n(`SELECT COUNT(*) FROM mission_run_locks`) != 0 {
		t.Fatalf("release = %v", err)
	}
	// A lock row is tied to its mission, and a lock cannot end before it starts.
	if _, err := f.db.DB.ExecContext(ctx, `INSERT INTO mission_run_locks (mission_id, token, acquired_at, expires_at) VALUES ($1, $2, NOW(), NOW() - INTERVAL '1 minute')`, mission, holder); sqlState(err) != "23514" {
		t.Fatalf("expires before acquired = %v", err)
	}
	if _, err := f.db.DB.ExecContext(ctx, `INSERT INTO mission_run_locks (mission_id, token, acquired_at, expires_at) VALUES ($1, $2, NOW(), NOW() + INTERVAL '1 minute')`, "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f0d21", holder); sqlState(err) != "23503" {
		t.Fatalf("a lock on no mission = %v", err)
	}
	// A malformed mission id is not a lock.
	if ok, err := f.repo.AcquireRun(ctx, "not-a-uuid", holder, now, time.Minute); ok || !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("malformed id = %v %v", ok, err)
	}
}

func TestTransitionMission_IsACompareAndSet(t *testing.T) {
	f := newExecFixture(t)
	mission, _ := f.stay(xAlice, true)
	ctx := context.Background()
	var changed int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := f.repo.TransitionMission(ctx, mission, domain.MissionDraft, domain.MissionActive, time.Now())
			if err != nil {
				t.Errorf("transition: %v", err)
			}
			if ok {
				mu.Lock()
				changed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if changed != 1 {
		t.Fatalf("%d transitions applied", changed)
	}
	if ok, _ := f.repo.TransitionMission(ctx, mission, domain.MissionDraft, domain.MissionCompleted, time.Now()); ok {
		t.Fatal("a stale read cannot overwrite a change made meanwhile")
	}
	if f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'active'`, mission) != 1 {
		t.Fatal("active")
	}
}

// The migration goes down and up again, leaving the earlier tables as they were.
func TestMigration000030_DownAndUp(t *testing.T) {
	f := newExecFixture(t)
	mission, _ := f.stay(xAlice, true)
	content, err := os.ReadFile(filepath.Join(migrationsDir, "000030_mission_orchestration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	up := extractUpSection(string(content))
	down := string(content[strings.Index(string(content), "-- +goose Down")+len("-- +goose Down"):])

	exists := func(name string) bool {
		var reg sql.NullString
		if err := f.db.DB.QueryRowContext(f.ctx, `SELECT to_regclass($1)::text`, name).Scan(&reg); err != nil {
			t.Fatal(err)
		}
		return reg.Valid
	}
	if !exists("mission_step_dependencies") || !exists("mission_run_locks") {
		t.Fatal("the migration created its tables")
	}
	if _, err := f.db.DB.ExecContext(f.ctx, down); err != nil {
		t.Fatalf("down: %v", err)
	}
	if exists("mission_step_dependencies") || exists("mission_run_locks") || f.n(`SELECT COUNT(*) FROM pg_constraint WHERE conname = 'uq_mission_steps_identity'`) != 0 {
		t.Fatal("down removed what up added")
	}
	if f.n(`SELECT COUNT(*) FROM mission_steps WHERE mission_id = $1`, mission) != 2 || f.n(`SELECT COUNT(*) FROM missions WHERE id = $1`, mission) != 1 {
		t.Fatal("down leaves the missions and their steps")
	}
	if _, err := f.db.DB.ExecContext(f.ctx, up); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if !exists("mission_step_dependencies") || !exists("mission_run_locks") {
		t.Fatal("up again")
	}
	// And down is repeatable.
	if _, err := f.db.DB.ExecContext(f.ctx, down); err != nil {
		t.Fatalf("down again: %v", err)
	}
	if _, err := f.db.DB.ExecContext(f.ctx, down); err != nil {
		t.Fatalf("down is idempotent: %v", err)
	}
}

func toJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
