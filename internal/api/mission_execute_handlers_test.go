package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/domain"
)

// fakeStepExecutor scripts the executor, recording what the handler gave it.
type fakeStepExecutor struct {
	gotActor, gotMission, gotStep, gotKey string
	calls                                 int
	do                                    func() (*domain.StepExecutionResult, error)
}

func (f *fakeStepExecutor) Execute(_ context.Context, actorID, missionID, stepID, key string) (*domain.StepExecutionResult, error) {
	f.calls++
	f.gotActor, f.gotMission, f.gotStep, f.gotKey = actorID, missionID, stepID, key
	return f.do()
}

// fakeAnnotator says every step can be run and that the first one has a completed attempt.
type fakeAnnotator struct{}

func (fakeAnnotator) Annotate(_ context.Context, _ *domain.Mission, steps []domain.MissionStep) (map[string]domain.MissionStepState, error) {
	out := map[string]domain.MissionStepState{}
	for i, s := range steps {
		st := domain.MissionStepState{Executable: true, CanExecute: true}
		if i == 0 {
			st.CanExecute = false
			st.Execution = &domain.MissionStepExecution{ID: "e-1", StepID: s.ID, Attempt: 1, Status: domain.ExecutionCompleted, ActionType: domain.ActionSearchListings,
				ResultReference: "e-1", ResultPayload: map[string]interface{}{"total": 3, "listing_ids": []string{"l1", "l2", "l3"}}, StartedAt: 1000, CompletedAt: 1001, RuntimeStatus: "SUCCESS"}
		}
		out[s.ID] = st
	}
	return out, nil
}

func executeServer(exec StepExecutor) http.Handler {
	repo := &memMissions{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}, plans: map[string][]*domain.MissionPlan{}}
	uc := application.NewMissionUseCases(repo).
		WithPlanning(missionplanning.NewRuleBased(func(domain.CommerceActionType) bool { return true }), repo).
		WithStepStates(fakeAnnotator{})
	h := NewMissionHandler(uc).WithExecution(exec)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions", h.CreateH)
	mux.HandleFunc("GET /api/v1/missions/{id}/steps", h.StepsH)
	mux.HandleFunc("POST /api/v1/missions/{id}/plan", h.PlanH)
	mux.HandleFunc("GET /api/v1/missions/{id}/plan", h.GetPlanH)
	mux.HandleFunc("POST /api/v1/missions/{id}/steps/{step_id}/execute", h.ExecuteStepH)
	return mux
}

const (
	execMission = "5b1f3c52-0a7d-4c1e-9f6e-3b2d4a8e9c10"
	execStep    = "6c2a4d63-1b8e-4d2f-8a7f-4c3e5b9f0d21"
	execURL     = "/api/v1/missions/" + execMission + "/steps/" + execStep + "/execute"
)

func completedResult() *domain.StepExecutionResult {
	exec := &domain.MissionStepExecution{
		ID: "7d3b5e74-2c9f-4e30-9b80-5d4f6ca01e32", StepID: execStep, MissionID: execMission, Attempt: 1, Status: domain.ExecutionCompleted,
		ActionType: domain.ActionSearchListings, ResultReference: "7d3b5e74-2c9f-4e30-9b80-5d4f6ca01e32", RuntimeStatus: "SUCCESS",
		ResultPayload: map[string]interface{}{"total": 3, "listing_ids": []string{"l1", "l2", "l3"}, "has_more": false, "vertical": "HOTEL"},
		StartedAt:     1000, CompletedAt: 1002,
	}
	return &domain.StepExecutionResult{
		Mission:   &domain.Mission{ID: execMission, Status: domain.MissionDraft},
		Step:      &domain.MissionStep{ID: execStep, MissionID: execMission, Type: "FIND_ACCOMMODATION", Title: "Find accommodation", Status: domain.StepCompleted, Position: 2, ActionType: domain.ActionSearchListings, PlanVersion: 1, ResultRef: exec.ID},
		Execution: exec, State: domain.MissionStepState{Executable: true, CanExecute: false, Execution: exec},
	}
}

func TestExecuteAPI_NeedsASignedInUser(t *testing.T) {
	f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return completedResult(), nil }}
	if code, _ := callMission(t, executeServer(f), "", "POST", execURL, ""); code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d, want 401", code)
	}
	if f.calls != 0 {
		t.Fatal("nothing is executed for an anonymous request")
	}
}

func TestExecuteAPI_RunsOneStepAndAnswersWithTheExecution(t *testing.T) {
	f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return completedResult(), nil }}
	code, body := callMission(t, executeServer(f), aliceID, "POST", execURL, "", "Idempotency-Key", "k-1")
	if code != http.StatusCreated {
		t.Fatalf("execute = %d %v", code, body)
	}
	if f.gotActor != aliceID || f.gotMission != execMission || f.gotStep != execStep || f.gotKey != "k-1" || f.calls != 1 {
		t.Fatalf("the handler passes the signed-in person, the ids and the key: %+v", f)
	}
	if body["mission_id"] != execMission || body["step_id"] != execStep || body["execution_id"] != "7d3b5e74-2c9f-4e30-9b80-5d4f6ca01e32" ||
		body["status"] != "completed" || body["action_type"] != "SEARCH_LISTINGS" || body["attempt"] != float64(1) || body["replayed"] != false || body["step_status"] != "completed" {
		t.Fatalf("body %v", body)
	}
	result, _ := body["result"].(map[string]interface{})
	if result["reference"] != "7d3b5e74-2c9f-4e30-9b80-5d4f6ca01e32" || result["total"] != float64(3) || len(result["listing_ids"].([]interface{})) != 3 {
		t.Fatalf("result %v", result)
	}
	if _, has := body["error"]; has {
		t.Fatal("a completed execution has no error")
	}
	if _, leaked := body["runtime_status"]; leaked {
		t.Fatal("the runtime's internals are not exposed")
	}
	step, _ := body["step"].(map[string]interface{})
	if step["status"] != "completed" || step["executable"] != true || step["can_execute"] != false || step["result_ref"] == "" {
		t.Fatalf("step %v", step)
	}
	if exec, _ := step["execution"].(map[string]interface{}); exec["status"] != "completed" {
		t.Fatalf("step.execution %v", step["execution"])
	}
}

func TestExecuteAPI_ReplayAnswers200AndAFailureIsARecordedOutcome(t *testing.T) {
	f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) {
		r := completedResult()
		r.Replayed = true
		return r, nil
	}}
	if code, body := callMission(t, executeServer(f), aliceID, "POST", execURL, "", "Idempotency-Key", "k-1"); code != http.StatusOK || body["replayed"] != true {
		t.Fatalf("a replay answers 200 replayed: %d %v", code, body)
	}
	f.do = func() (*domain.StepExecutionResult, error) {
		r := completedResult()
		r.Execution.Status, r.Execution.ErrorCode, r.Execution.ResultReference, r.Execution.ResultPayload = domain.ExecutionFailed, "INSUFFICIENT_CREDITS", "", map[string]interface{}{}
		r.Step.Status, r.Step.ResultRef = domain.StepFailed, ""
		r.State = domain.MissionStepState{Executable: true, CanExecute: true, Execution: r.Execution}
		return r, nil
	}
	code, body := callMission(t, executeServer(f), aliceID, "POST", execURL, "")
	if code != http.StatusCreated || body["status"] != "failed" || body["step_status"] != "failed" {
		t.Fatalf("failure = %d %v", code, body)
	}
	if e, _ := body["error"].(map[string]interface{}); e["code"] != "INSUFFICIENT_CREDITS" || len(e) != 1 {
		t.Fatalf("a stable code, nothing else: %v", body["error"])
	}
	if _, has := body["result"]; has {
		t.Fatal("a failed execution has no result")
	}
	if step, _ := body["step"].(map[string]interface{}); step["can_execute"] != true {
		t.Fatalf("a failed step can be retried: %v", body["step"])
	}
}

func TestExecuteAPI_ErrorsMapToControlledStatuses(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"another person's mission", domain.ErrMissionForbidden, http.StatusForbidden, "FORBIDDEN"},
		{"unknown mission", domain.ErrMissionNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"unknown step, or one of another mission", domain.ErrMissionStepNotFound, http.StatusNotFound, "NOT_FOUND"},
		{"already running", domain.ErrStepInProgress, http.StatusConflict, "STEP_IN_PROGRESS"},
		{"completed or skipped", domain.ErrStepNotRunnable, http.StatusConflict, "STEP_NOT_RUNNABLE"},
		{"mission paused or closed", domain.ErrMissionNotRunnable, http.StatusConflict, "MISSION_NOT_RUNNABLE"},
		{"given up on", domain.ErrExecutionSuperseded, http.StatusConflict, "EXECUTION_SUPERSEDED"},
		{"unsupported action", domain.ErrStepNotExecutable, http.StatusUnprocessableEntity, "STEP_NOT_EXECUTABLE"},
	}
	for _, c := range cases {
		f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return nil, c.err }}
		code, body := callMission(t, executeServer(f), aliceID, "POST", execURL, "")
		if code != c.code || errCode(body) != c.want {
			t.Errorf("%s = %d %v, want %d %s", c.name, code, body, c.code, c.want)
		}
	}
	// An unexpected failure is a 500 that says nothing of what went wrong.
	f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return nil, context.DeadlineExceeded }}
	code, body := callMission(t, executeServer(f), aliceID, "POST", execURL, "")
	if code != http.StatusInternalServerError || strings.Contains(strings.ToLower(body["error"].(map[string]interface{})["message"].(string)), "deadline") {
		t.Fatalf("internal error = %d %v", code, body)
	}
}

func TestExecuteAPI_RequestValidation(t *testing.T) {
	f := &fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return completedResult(), nil }}
	srv := executeServer(f)
	for _, path := range []string{
		"/api/v1/missions/not-an-id/steps/" + execStep + "/execute",
		"/api/v1/missions/" + execMission + "/steps/not-an-id/execute",
	} {
		if code, b := callMission(t, srv, aliceID, "POST", path, ""); code != http.StatusNotFound || errCode(b) != "NOT_FOUND" {
			t.Errorf("%s = %d %v, want 404", path, code, b)
		}
	}
	for name, body := range map[string]string{
		"parameters from the client": `{"parameters": {"vertical": "HOTEL"}}`,
		"an action chosen by client": `{"action_type": "CREATE_ORDER"}`,
		"not json":                   `run it`,
	} {
		if code, b := callMission(t, srv, aliceID, "POST", execURL, body); code != http.StatusBadRequest || errCode(b) != "INVALID_REQUEST" {
			t.Errorf("%s = %d %v, want 400", name, code, b)
		}
	}
	if code, _ := callMission(t, srv, aliceID, "POST", execURL, "", "Idempotency-Key", strings.Repeat("k", 101)); code != http.StatusBadRequest {
		t.Errorf("an over-long key = %d", code)
	}
	if code, _ := callMission(t, srv, aliceID, "POST", execURL, `{}`); code != http.StatusCreated {
		t.Errorf("an empty JSON object is accepted = %d", code)
	}
	if f.calls != 1 {
		t.Fatalf("only the valid request reached the executor: %d", f.calls)
	}
}

func TestExecuteAPI_WithoutAnExecutorTheStepIsNotExecutable(t *testing.T) {
	repo := &memMissions{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}, plans: map[string][]*domain.MissionPlan{}}
	h := NewMissionHandler(application.NewMissionUseCases(repo))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions/{id}/steps/{step_id}/execute", h.ExecuteStepH)
	if code, b := callMission(t, mux, aliceID, "POST", execURL, ""); code != http.StatusUnprocessableEntity || errCode(b) != "STEP_NOT_EXECUTABLE" {
		t.Fatalf("no executor = %d %v", code, b)
	}
}

// The plan and the steps list tell a client which steps can be run and how their latest attempt went.
func TestExecuteAPI_PlanAndStepsShowWhatCanBeRun(t *testing.T) {
	srv := executeServer(&fakeStepExecutor{do: func() (*domain.StepExecutionResult, error) { return completedResult(), nil }})
	id := createFrom(t, srv, aliceID, "Un séjour de 3 jours")
	code, plan := callMission(t, srv, aliceID, "POST", "/api/v1/missions/"+id+"/plan", "")
	if code != http.StatusCreated {
		t.Fatalf("plan = %d %v", code, plan)
	}
	steps := plan["steps"].([]interface{})
	first, second := steps[0].(map[string]interface{}), steps[1].(map[string]interface{})
	if first["executable"] != true || first["can_execute"] != false || second["can_execute"] != true {
		t.Fatalf("states in the plan: %v %v", first, second)
	}
	if exec, _ := first["execution"].(map[string]interface{}); exec["status"] != "completed" || exec["result"].(map[string]interface{})["total"] != float64(3) {
		t.Fatalf("latest attempt in the plan: %v", first["execution"])
	}
	if _, has := second["execution"]; has {
		t.Fatal("a step never run has no execution")
	}
	_, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id+"/steps", "")
	items := list["items"].([]interface{})
	if len(items) != 2 || items[1].(map[string]interface{})["can_execute"] != true {
		t.Fatalf("states in the steps list: %v", list)
	}
	// Reading is the owner's too.
	if code, _ := callMission(t, srv, bobID, "GET", "/api/v1/missions/"+id+"/steps", ""); code != http.StatusForbidden {
		t.Fatalf("another person reading the steps = %d", code)
	}
}
