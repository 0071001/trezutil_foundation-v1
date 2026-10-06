package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/missionorch"
	"github.com/trezutil/trezutil/internal/domain"
)

// fakeOrchestrator scripts the orchestrator, recording what the handler gave it.
type fakeOrchestrator struct {
	gotActor, gotMission string
	gotOpts              missionorch.Options
	runs, reads          int
	run                  func() (*missionorch.State, error)
}

func (f *fakeOrchestrator) Run(_ context.Context, actorID, missionID string, opts missionorch.Options) (*missionorch.State, error) {
	f.runs++
	f.gotActor, f.gotMission, f.gotOpts = actorID, missionID, opts
	return f.run()
}

func (f *fakeOrchestrator) State(_ context.Context, actorID, missionID string) (*missionorch.State, error) {
	f.reads++
	f.gotActor, f.gotMission = actorID, missionID
	return f.run()
}

const (
	orchMission  = "5b1f3c52-0a7d-4c1e-9f6e-3b2d4a8e9c10"
	runURL       = "/api/v1/missions/" + orchMission + "/run"
	resumeURL    = "/api/v1/missions/" + orchMission + "/resume"
	execStateURL = "/api/v1/missions/" + orchMission + "/execution"
)

func orchServer(o MissionOrchestrator) http.Handler {
	repo := &memMissions{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}, plans: map[string][]*domain.MissionPlan{}}
	h := NewMissionHandler(application.NewMissionUseCases(repo)).WithOrchestration(o)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions/{id}/run", h.RunH)
	mux.HandleFunc("POST /api/v1/missions/{id}/resume", h.ResumeH)
	mux.HandleFunc("GET /api/v1/missions/{id}/execution", h.ExecutionH)
	return mux
}

// partialState: a transport search completed, the accommodation search failed and the check that depends on it is blocked.
func partialState(busy bool) *missionorch.State {
	exec := &domain.MissionStepExecution{ID: "e1", Attempt: 1, Status: domain.ExecutionCompleted, ActionType: domain.ActionSearchListings,
		ResultReference: "e1", ResultPayload: map[string]interface{}{"total": 2, "listing_ids": []string{"l1", "l2"}}, StartedAt: 1000, CompletedAt: 1001}
	bad := &domain.MissionStepExecution{ID: "e2", Attempt: 1, Status: domain.ExecutionFailed, ActionType: domain.ActionSearchListings, ErrorCode: "ACTION_FAILED", RuntimeStatus: "FAILED", StartedAt: 1002, CompletedAt: 1003}
	steps := []domain.FlowStep{
		{Step: domain.MissionStep{ID: "s1", MissionID: orchMission, Type: "FIND_TRANSPORT", Status: domain.StepCompleted, Position: 1, PlanVersion: 2, ActionType: domain.ActionSearchListings},
			State: domain.MissionStepState{Flow: domain.FlowCompleted, Executable: true, Execution: exec}},
		{Step: domain.MissionStep{ID: "s2", MissionID: orchMission, Type: "FIND_ACCOMMODATION", Status: domain.StepFailed, Position: 2, PlanVersion: 2, ActionType: domain.ActionSearchListings},
			State: domain.MissionStepState{Flow: domain.FlowFailed, Executable: true, CanExecute: true, Execution: bad}},
		{Step: domain.MissionStep{ID: "s3", MissionID: orchMission, Type: "CHECK_AVAILABILITY", Status: domain.StepPending, Position: 3, PlanVersion: 2, ActionType: domain.ActionCheckAvailability},
			State: domain.MissionStepState{Flow: domain.FlowBlocked, Executable: true, DependsOn: []string{"s2"}, BlockedBy: []string{"s2"}, Reason: domain.ReasonDependencyFailed}},
		{Step: domain.MissionStep{ID: "s4", MissionID: orchMission, Type: "CHECK_AVAILABILITY", Status: domain.StepPending, Position: 4, PlanVersion: 2, ActionType: domain.ActionCheckAvailability},
			State: domain.MissionStepState{Flow: domain.FlowBlocked, Executable: true, DependsOn: []string{"s1"}, Reason: domain.ReasonNeedsInput, Detail: "dates"}},
	}
	flow := &domain.MissionFlow{PlanVersion: 2, Steps: steps}
	return &missionorch.State{
		Mission: &domain.Mission{ID: orchMission, Status: domain.MissionActive}, Flow: flow, Outcome: flow.Outcome(busy), Progress: flow.Progress(), Busy: busy,
		Ran: []missionorch.Ran{
			{StepID: "s1", StepType: "FIND_TRANSPORT", ActionType: domain.ActionSearchListings, ExecutionID: "e1", Attempt: 1, Status: domain.ExecutionCompleted},
			{StepID: "s2", StepType: "FIND_ACCOMMODATION", ActionType: domain.ActionSearchListings, ExecutionID: "e2", Attempt: 1, Status: domain.ExecutionFailed, ErrorCode: "ACTION_FAILED"},
		},
	}
}

func TestOrchestrationAPI_NeedsASignedInUser(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return partialState(false), nil }}
	srv := orchServer(f)
	for _, c := range [][2]string{{"POST", runURL}, {"POST", resumeURL}, {"GET", execStateURL}} {
		if code, _ := callMission(t, srv, "", c[0], c[1], ""); code != http.StatusUnauthorized {
			t.Errorf("%s %s unauthenticated = %d, want 401", c[0], c[1], code)
		}
	}
	if f.runs+f.reads != 0 {
		t.Fatal("nothing is run for an anonymous request")
	}
}

func TestOrchestrationAPI_RunAnswersWithTheStateOfEveryStep(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return partialState(false), nil }}
	code, body := callMission(t, orchServer(f), aliceID, "POST", runURL, "")
	if code != http.StatusOK {
		t.Fatalf("run = %d %v", code, body)
	}
	if f.gotActor != aliceID || f.gotMission != orchMission || f.gotOpts.RetryFailed || f.gotOpts.OnFailure != missionorch.StopOnFailure {
		t.Fatalf("the signed-in person, the id and the safe default policy: %+v", f)
	}
	if body["mission_id"] != orchMission || body["plan_version"] != float64(2) || body["mission_status"] != "active" || body["outcome"] != "needs_attention" || body["busy"] != false {
		t.Fatalf("body %v", body)
	}
	if p, _ := body["progress"].(map[string]interface{}); p["completed"] != float64(1) || p["total"] != float64(4) || p["skipped"] != float64(0) {
		t.Fatalf("progress %v", body["progress"])
	}
	ran, _ := body["ran"].([]interface{})
	if len(ran) != 2 || ran[1].(map[string]interface{})["error_code"] != "ACTION_FAILED" || ran[0].(map[string]interface{})["step_id"] != "s1" {
		t.Fatalf("ran %v", ran)
	}
	steps, _ := body["steps"].([]interface{})
	if len(steps) != 4 {
		t.Fatalf("steps %v", steps)
	}
	by := map[string]map[string]interface{}{}
	for _, s := range steps {
		m := s.(map[string]interface{})
		by[m["id"].(string)] = m
	}
	if by["s1"]["state"] != "completed" || by["s2"]["state"] != "failed" || by["s3"]["state"] != "blocked" {
		t.Fatalf("states %v", by)
	}
	if by["s3"]["reason"] != "dependency_failed" || by["s3"]["blocked_by"].([]interface{})[0] != "s2" || by["s3"]["depends_on"].([]interface{})[0] != "s2" {
		t.Fatalf("a blocked step says by what and why: %v", by["s3"])
	}
	if by["s4"]["reason"] != "needs_input" || by["s4"]["detail"] != "dates" {
		t.Fatalf("a missing input is named: %v", by["s4"])
	}
	if _, has := by["s1"]["reason"]; has {
		t.Fatal("a completed step has no reason")
	}
	if e, _ := by["s2"]["execution"].(map[string]interface{}); e["status"] != "failed" || e["error"].(map[string]interface{})["code"] != "ACTION_FAILED" {
		t.Fatalf("the failed attempt: %v", by["s2"]["execution"])
	}
	for k := range by["s2"] {
		if strings.Contains(k, "runtime") {
			t.Fatalf("the runtime's internals are not exposed: %s", k)
		}
	}
}

func TestOrchestrationAPI_ResumeAsksForRetriesAndThePolicyIsChosenInTheBody(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return partialState(false), nil }}
	srv := orchServer(f)
	if code, _ := callMission(t, srv, aliceID, "POST", resumeURL, ""); code != http.StatusOK || !f.gotOpts.RetryFailed || f.gotOpts.OnFailure != missionorch.StopOnFailure {
		t.Fatalf("resume = %d %+v", code, f.gotOpts)
	}
	if code, _ := callMission(t, srv, aliceID, "POST", runURL, `{"on_failure":"continue"}`); code != http.StatusOK || f.gotOpts.RetryFailed || f.gotOpts.OnFailure != missionorch.ContinueIndependent {
		t.Fatalf("continue = %d %+v", code, f.gotOpts)
	}
	runs := f.runs
	for name, body := range map[string]string{
		"unknown policy": `{"on_failure":"always"}`, "unknown field": `{"on_failure":"stop","owner_id":"` + bobID + `"}`, "not json": `nope`,
	} {
		if code, b := callMission(t, srv, aliceID, "POST", runURL, body); code != http.StatusBadRequest || errCode(b) != "INVALID_REQUEST" {
			t.Errorf("%s = %d %v", name, code, b)
		}
	}
	if f.runs != runs {
		t.Fatal("a bad request runs nothing")
	}
}

func TestOrchestrationAPI_ABusyMissionAnswers202WithItsState(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) {
		s := partialState(true)
		s.Ran = nil
		return s, nil
	}}
	code, body := callMission(t, orchServer(f), aliceID, "POST", runURL, "")
	if code != http.StatusAccepted || body["busy"] != true || body["outcome"] != "in_progress" || len(body["ran"].([]interface{})) != 0 {
		t.Fatalf("busy = %d %v", code, body)
	}
}

func TestOrchestrationAPI_ExecutionIsARead(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return partialState(false), nil }}
	code, body := callMission(t, orchServer(f), aliceID, "GET", execStateURL, "")
	if code != http.StatusOK || f.reads != 1 || f.runs != 0 || body["outcome"] != "needs_attention" {
		t.Fatalf("read = %d runs=%d reads=%d %v", code, f.runs, f.reads, body)
	}
}

func TestOrchestrationAPI_ErrorsMapToStableCodes(t *testing.T) {
	for _, c := range []struct {
		err    error
		status int
		code   string
	}{
		{domain.ErrMissionNotFound, 404, "NOT_FOUND"},
		{domain.ErrMissionForbidden, 403, "FORBIDDEN"},
		{domain.ErrNoPlanToRun, 409, "PLAN_REQUIRED"},
		{domain.ErrDependencyCycle, 409, "MISSION_GRAPH_INVALID"},
		{domain.ErrDependencyInvalid, 409, "MISSION_GRAPH_INVALID"},
		{domain.ErrMissionNotRunnable, 409, "MISSION_NOT_RUNNABLE"},
		{domain.ErrMissionBusy, 409, "MISSION_BUSY"},
		{domain.ErrStepWaiting, 409, "STEP_WAITING"},
		{domain.ErrStepInputMissing, 409, "STEP_NEEDS_INPUT"},
	} {
		f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return nil, c.err }}
		for _, call := range [][2]string{{"POST", runURL}, {"POST", resumeURL}, {"GET", execStateURL}} {
			code, body := callMission(t, orchServer(f), aliceID, call[0], call[1], "")
			if code != c.status || errCode(body) != c.code {
				t.Errorf("%s %v = %d %s, want %d %s", call[1], c.err, code, errCode(body), c.status, c.code)
			}
		}
	}
	// An unexpected error is the generic one: nothing of it reaches the client.
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return nil, context.DeadlineExceeded }}
	if code, body := callMission(t, orchServer(f), aliceID, "POST", runURL, ""); code != 500 || strings.Contains(fmt.Sprint(body), "deadline") {
		t.Fatalf("unexpected = %d %v", code, body)
	}
}

func TestOrchestrationAPI_MalformedIdsAreNotFoundAndNothingRuns(t *testing.T) {
	f := &fakeOrchestrator{run: func() (*missionorch.State, error) { return partialState(false), nil }}
	for _, id := range []string{"not-a-uuid", "1", "5b1f3c52-0a7d-4c1e-9f6e-3b2d4a8e9c1g", "%27%3B--"} {
		if code, _ := callMission(t, orchServer(f), aliceID, "POST", "/api/v1/missions/"+id+"/run", ""); code != http.StatusNotFound {
			t.Errorf("%q = %d", id, code)
		}
	}
	if f.runs != 0 {
		t.Fatal("a malformed id runs nothing")
	}
}

func TestOrchestrationAPI_WithoutAnOrchestratorNothingRuns(t *testing.T) {
	srv := orchServer(nil)
	if code, body := callMission(t, srv, aliceID, "POST", runURL, ""); code != http.StatusUnprocessableEntity || errCode(body) != "STEP_NOT_EXECUTABLE" {
		t.Fatalf("no orchestrator = %d %v", code, body)
	}
}
