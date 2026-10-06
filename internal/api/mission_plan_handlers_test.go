package api

import (
	"context"
	"net/http"
	"sort"
	"testing"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/domain"
)

// The plan side of the in-memory repository.
func (r *memMissions) currentPlan(id string) *domain.MissionPlan {
	for _, p := range r.plans[id] {
		if p.Status == domain.MissionPlanCurrent {
			return p
		}
	}
	return nil
}
func copyOfPlan(p *domain.MissionPlan) *domain.MissionPlan {
	c := *p
	c.Steps = append([]domain.MissionStep{}, p.Steps...)
	return &c
}
func (r *memMissions) CurrentPlan(_ context.Context, id string) (*domain.MissionPlan, error) {
	if p := r.currentPlan(id); p != nil {
		return copyOfPlan(p), nil
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (r *memMissions) PlanVersion(_ context.Context, id string, v int) (*domain.MissionPlan, error) {
	for _, p := range r.plans[id] {
		if p.Version == v {
			return copyOfPlan(p), nil
		}
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (r *memMissions) PlanVersions(_ context.Context, id string) ([]int, error) {
	out := []int{}
	for _, p := range r.plans[id] {
		out = append(out, p.Version)
	}
	sort.Ints(out)
	return out, nil
}
func (r *memMissions) PlanByIdempotencyKey(_ context.Context, id, key string) (*domain.MissionPlan, error) {
	for _, p := range r.plans[id] {
		if p.IdempotencyKey == key {
			return copyOfPlan(p), nil
		}
	}
	return nil, domain.ErrMissionPlanNotFound
}
func (r *memMissions) SavePlan(_ context.Context, plan *domain.MissionPlan, expected int) error {
	cur := 0
	if p := r.currentPlan(plan.MissionID); p != nil {
		cur = p.Version
	}
	if cur != expected {
		return domain.ErrMissionPlanConflict
	}
	if p := r.currentPlan(plan.MissionID); p != nil {
		p.Status, p.SupersededAt = domain.MissionPlanSuperseded, plan.CreatedAt
	}
	r.plans[plan.MissionID] = append(r.plans[plan.MissionID], copyOfPlan(plan))
	return nil
}

func plannedServer(withPlanning bool) http.Handler {
	repo := &memMissions{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}, plans: map[string][]*domain.MissionPlan{}}
	uc := application.NewMissionUseCases(repo)
	if withPlanning {
		uc.WithPlanning(missionplanning.NewRuleBased(func(a domain.CommerceActionType) bool {
			return a == domain.ActionSearchListings || a == domain.ActionCheckAvailability
		}), repo)
	}
	h := NewMissionHandler(uc)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions", h.CreateH)
	mux.HandleFunc("GET /api/v1/missions/{id}", h.GetH)
	mux.HandleFunc("PATCH /api/v1/missions/{id}", h.UpdateH)
	mux.HandleFunc("GET /api/v1/missions/{id}/steps", h.StepsH)
	mux.HandleFunc("POST /api/v1/missions/{id}/plan", h.PlanH)
	mux.HandleFunc("GET /api/v1/missions/{id}/plan", h.GetPlanH)
	return mux
}

func createFrom(t *testing.T, srv http.Handler, user, sentence string) string {
	t.Helper()
	code, body := callMission(t, srv, user, "POST", "/api/v1/missions", `{"objective":"`+sentence+`"}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, body)
	}
	return body["id"].(string)
}

func stepIDs(plan map[string]interface{}) []string {
	var out []string
	for _, s := range plan["steps"].([]interface{}) {
		out = append(out, s.(map[string]interface{})["id"].(string))
	}
	return out
}

func TestPlanAPI_NeedsASignedInUser(t *testing.T) {
	srv := plannedServer(true)
	id := createBirthday(t, srv, aliceID)
	for _, c := range [][2]string{{"POST", "/api/v1/missions/" + id + "/plan"}, {"GET", "/api/v1/missions/" + id + "/plan"}, {"GET", "/api/v1/missions/" + id + "/plan?version=1"}} {
		if code, _ := callMission(t, srv, "", c[0], c[1], ""); code != http.StatusUnauthorized {
			t.Errorf("%s %s without a user = %d, want 401", c[0], c[1], code)
		}
	}
}

func TestPlanAPI_PlansAnEventAsPendingIntentions(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	code, plan := callMission(t, srv, aliceID, "POST", "/api/v1/missions/"+id+"/plan", "")
	if code != http.StatusCreated {
		t.Fatalf("plan = %d %v", code, plan)
	}
	if plan["mission_id"] != id || plan["mission_type"] != "EVENT" || plan["plan_version"] != float64(1) || plan["status"] != "current" || plan["plannable"] != true {
		t.Fatalf("plan header %v", plan)
	}
	if plan["planner"] != "rules" || plan["created_at"] == "" {
		t.Fatalf("planner %v", plan)
	}
	steps := plan["steps"].([]interface{})
	if len(steps) != 6 {
		t.Fatalf("an event has six steps, got %d", len(steps))
	}
	for i, raw := range steps {
		s := raw.(map[string]interface{})
		if s["position"] != float64(i+1) || s["status"] != "pending" || s["plan_version"] != float64(1) || s["id"] == "" || s["type"] == "" || s["title"] == "" {
			t.Fatalf("step %d: %v", i, s)
		}
		if s["action_type"] != "SEARCH_LISTINGS" || s["action_support"] != "supported" {
			t.Fatalf("step %d names its action: %v", i, s)
		}
		if s["action_ref"] != "" || s["result_ref"] != "" {
			t.Fatalf("a plan holds intentions, never results: %v", s)
		}
	}
	if first := steps[0].(map[string]interface{}); first["type"] != "FIND_VENUE" || first["title"] != "Find a venue" {
		t.Fatalf("first step %v", first)
	}
	// Planning leaves the mission as it was: a draft, nothing added.
	if _, m := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id, ""); m["status"] != "draft" || m["location"] != "Kinshasa" {
		t.Fatalf("mission after planning %v", m)
	}
	// The steps endpoint now lists the plan's steps too.
	if _, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id+"/steps", ""); list["total"] != float64(6) {
		t.Fatalf("steps = %v", list)
	}
}

func TestPlanAPI_TravelMovingAndUnknown(t *testing.T) {
	srv := plannedServer(true)
	types := func(plan map[string]interface{}) []string {
		var out []string
		for _, s := range plan["steps"].([]interface{}) {
			out = append(out, s.(map[string]interface{})["type"].(string))
		}
		return out
	}
	for _, c := range []struct {
		sentence string
		want     []string
	}{
		{"Voyage à Nairobi pendant 5 jours", []string{"FIND_TRANSPORT", "FIND_ACCOMMODATION", "FIND_LOCAL_TRANSPORT", "FIND_ACTIVITIES"}},
		{"Je veux organiser mon déménagement", []string{"FIND_ACCOMMODATION", "FIND_MOVING_TRANSPORT", "FIND_MOVERS", "FIND_STORAGE"}},
	} {
		id := createFrom(t, srv, aliceID, c.sentence)
		code, plan := callMission(t, srv, aliceID, "POST", "/api/v1/missions/"+id+"/plan", "")
		got := types(plan)
		if code != http.StatusCreated || len(got) != len(c.want) {
			t.Fatalf("%q = %d %v", c.sentence, code, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%q steps %v, want %v", c.sentence, got, c.want)
			}
		}
	}

	id := createFrom(t, srv, aliceID, "Je veux organiser quelque chose")
	code, plan := callMission(t, srv, aliceID, "POST", "/api/v1/missions/"+id+"/plan", "")
	if code != http.StatusOK || plan["status"] != "needs_information" || plan["needs_information"] != true || plan["reason"] != "no_template" || plan["plannable"] != false || plan["plan_version"] != float64(0) {
		t.Fatalf("an unknown goal asks for more information = %d %v", code, plan)
	}
	if len(plan["steps"].([]interface{})) != 0 {
		t.Fatalf("no step is made up: %v", plan["steps"])
	}
	if _, m := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id, ""); m["type"] != "OTHER" || m["location"] != "" || m["participants"] != float64(0) || m["status"] != "draft" {
		t.Fatalf("nothing is invented: %v", m)
	}
}

func TestPlanAPI_TheSameRequestGivesTheSamePlan(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	path := "/api/v1/missions/" + id + "/plan"
	c1, first := callMission(t, srv, aliceID, "POST", path, "", "Idempotency-Key", "k-1")
	c2, again := callMission(t, srv, aliceID, "POST", path, "", "Idempotency-Key", "k-1")
	c3, noKey := callMission(t, srv, aliceID, "POST", path, `{}`)
	if c1 != http.StatusCreated || c2 != http.StatusOK || c3 != http.StatusOK {
		t.Fatalf("statuses %d %d %d", c1, c2, c3)
	}
	for _, p := range []map[string]interface{}{again, noKey} {
		a, b := stepIDs(first), stepIDs(p)
		if len(a) != 6 || len(a) != len(b) {
			t.Fatalf("%v vs %v", a, b)
		}
		for i := range a {
			if a[i] != b[i] {
				t.Fatalf("the same plan means the same steps: %v vs %v", a, b)
			}
		}
		if p["plan_version"] != float64(1) {
			t.Fatalf("version %v", p["plan_version"])
		}
	}
	if _, got := callMission(t, srv, aliceID, "GET", path, ""); got["versions"].([]interface{})[0] != float64(1) || len(got["versions"].([]interface{})) != 1 {
		t.Fatalf("one version only: %v", got["versions"])
	}
	if _, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id+"/steps", ""); list["total"] != float64(6) {
		t.Fatalf("one set of steps, not %v", list["total"])
	}
}

func TestPlanAPI_GetPlan(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	path := "/api/v1/missions/" + id + "/plan"
	code, none := callMission(t, srv, aliceID, "GET", path, "")
	if code != http.StatusOK || none["status"] != "none" || none["plan_version"] != float64(0) || none["plannable"] != true || len(none["steps"].([]interface{})) != 0 {
		t.Fatalf("no plan yet is a state, not an error: %d %v", code, none)
	}
	callMission(t, srv, aliceID, "POST", path, "")
	if code, p := callMission(t, srv, aliceID, "GET", path, ""); code != 200 || p["status"] != "current" || len(p["steps"].([]interface{})) != 6 {
		t.Fatalf("current plan = %d %v", code, p)
	}
	if code, p := callMission(t, srv, aliceID, "GET", path+"?version=1", ""); code != 200 || p["plan_version"] != float64(1) {
		t.Fatalf("version 1 = %d %v", code, p)
	}
	for _, v := range []string{"0", "-1", "abc", "1.5"} {
		if code, _ := callMission(t, srv, aliceID, "GET", path+"?version="+v, ""); code != http.StatusBadRequest {
			t.Errorf("version=%s = %d, want 400", v, code)
		}
	}
	if code, b := callMission(t, srv, aliceID, "GET", path+"?version=9", ""); code != http.StatusNotFound || errCode(b) != "NOT_FOUND" {
		t.Fatalf("unknown version = %d %v", code, b)
	}
}

func TestPlanAPI_ReplacingAPlanIsAskedForAndKeepsHistory(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	path := "/api/v1/missions/" + id + "/plan"
	callMission(t, srv, aliceID, "POST", path, "")
	if code, m := callMission(t, srv, aliceID, "PATCH", "/api/v1/missions/"+id, `{"type":"travel"}`); code != 200 || m["type"] != "TRAVEL" {
		t.Fatalf("retype = %d %v", code, m)
	}
	if code, b := callMission(t, srv, aliceID, "POST", path, ""); code != http.StatusConflict || errCode(b) != "PLAN_EXISTS" {
		t.Fatalf("a different plan is not stored silently = %d %v", code, b)
	}
	code, second := callMission(t, srv, aliceID, "POST", path, `{"replace": true}`)
	if code != http.StatusCreated || second["plan_version"] != float64(2) || second["plan_mission_type"] != "TRAVEL" || len(second["steps"].([]interface{})) != 4 {
		t.Fatalf("replace = %d %v", code, second)
	}
	if len(second["versions"].([]interface{})) != 2 {
		t.Fatalf("versions %v", second["versions"])
	}
	_, old := callMission(t, srv, aliceID, "GET", path+"?version=1", "")
	if old["status"] != "superseded" || old["superseded_at"] == "" || len(old["steps"].([]interface{})) != 6 || old["plan_mission_type"] != "EVENT" {
		t.Fatalf("the old plan stays: %v", old)
	}
	if _, cur := callMission(t, srv, aliceID, "GET", path, ""); cur["plan_version"] != float64(2) || cur["status"] != "current" {
		t.Fatalf("current %v", cur)
	}
	if _, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id+"/steps", ""); list["total"] != float64(4) {
		t.Fatalf("the steps list shows the current plan only: %v", list["total"])
	}
}

func TestPlanAPI_StrictRequestValidation(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	path := "/api/v1/missions/" + id + "/plan"
	for name, body := range map[string]string{
		"unknown field":   `{"steps": []}`,
		"wrong type":      `{"replace": "yes"}`,
		"not json":        `replace=true`,
		"owner smuggling": `{"owner_id": "` + bobID + `"}`,
	} {
		if code, b := callMission(t, srv, aliceID, "POST", path, body); code != http.StatusBadRequest || errCode(b) != "INVALID_REQUEST" {
			t.Errorf("%s = %d %v, want 400", name, code, b)
		}
	}
	long := make([]byte, 101)
	for i := range long {
		long[i] = 'k'
	}
	if code, _ := callMission(t, srv, aliceID, "POST", path, "", "Idempotency-Key", string(long)); code != http.StatusBadRequest {
		t.Fatalf("an over-long key = %d", code)
	}
	if _, p := callMission(t, srv, aliceID, "GET", path, ""); p["status"] != "none" {
		t.Fatalf("a refused request stores nothing: %v", p["status"])
	}
}

func TestPlanAPI_OnlyTheOwnerPlansOrReads(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	path := "/api/v1/missions/" + id + "/plan"
	if code, _ := callMission(t, srv, aliceID, "POST", path, ""); code != http.StatusCreated {
		t.Fatalf("User A, Mission A = %d, want 201", code)
	}
	if code, _ := callMission(t, srv, aliceID, "GET", path, ""); code != http.StatusOK {
		t.Fatalf("User A reads Mission A = %d", code)
	}
	for _, c := range [][3]string{
		{"POST", path, ""}, {"POST", path, `{"replace": true}`}, {"GET", path, ""}, {"GET", path + "?version=1", ""},
	} {
		if code, b := callMission(t, srv, bobID, c[0], c[1], c[2]); code != http.StatusForbidden || errCode(b) != "FORBIDDEN" {
			t.Errorf("User B %s %s = %d %v, want 403", c[0], c[1], code, b)
		}
	}
	if code, _ := callMission(t, srv, bobID, "POST", path, "", "Idempotency-Key", "k"); code != http.StatusForbidden {
		t.Fatalf("User B with a key = %d, want 403", code)
	}
	// B's own empty mission is not touched by A's id, and A's plan is still the only one.
	other := createFrom(t, srv, bobID, "Voyage à Nairobi pendant 5 jours")
	if _, p := callMission(t, srv, bobID, "GET", "/api/v1/missions/"+other+"/plan", ""); p["status"] != "none" {
		t.Fatalf("B's mission has no plan of its own yet: %v", p["status"])
	}
	if _, a := callMission(t, srv, aliceID, "GET", path, ""); len(a["versions"].([]interface{})) != 1 {
		t.Fatalf("A's plan untouched: %v", a["versions"])
	}
	for _, p := range []string{"/api/v1/missions/44444444-4444-4444-8444-444444444444/plan", "/api/v1/missions/not-an-id/plan"} {
		for _, m := range []string{"GET", "POST"} {
			if code, b := callMission(t, srv, aliceID, m, p, ""); code != http.StatusNotFound || errCode(b) != "NOT_FOUND" {
				t.Errorf("%s %s = %d %v, want 404", m, p, code, b)
			}
		}
	}
}

func TestPlanAPI_ClosedMissionAndMissingPlanner(t *testing.T) {
	srv := plannedServer(true)
	id := createFrom(t, srv, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	callMission(t, srv, aliceID, "PATCH", "/api/v1/missions/"+id, `{"status":"cancelled"}`)
	if code, b := callMission(t, srv, aliceID, "POST", "/api/v1/missions/"+id+"/plan", ""); code != http.StatusConflict || errCode(b) != "MISSION_CLOSED" {
		t.Fatalf("a cancelled mission is not planned = %d %v", code, b)
	}
	bare := plannedServer(false)
	id2 := createFrom(t, bare, aliceID, "Anniversaire pour 40 personnes à Kinshasa")
	code, b := callMission(t, bare, aliceID, "POST", "/api/v1/missions/"+id2+"/plan", "")
	if code != http.StatusServiceUnavailable || errCode(b) != "PLANNING_UNAVAILABLE" {
		t.Fatalf("no planner configured = %d %v", code, b)
	}
}
