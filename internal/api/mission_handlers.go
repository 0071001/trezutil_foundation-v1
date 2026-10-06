package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

/*
 * Organiser's missions, under /api/v1/missions. Every route needs a signed-in user and only ever touches that
 * user's own missions: a mission of somebody else answers 403, an unknown one 404.
 *
 *	POST  /missions                  {objective, type?, title?, location?, participants?, start_date?, end_date?,
 *	                                  budget_minor?, currency?, preferences?}   a draft; Idempotency-Key honoured
 *	GET   /missions?status=&page=&page_size=                                   the caller's missions
 *	GET   /missions/{id}
 *	PATCH /missions/{id}             any of the fields above, and status        lifecycle-checked
 *	POST  /missions/{id}/steps       {type, title, description?, action_type?}  recorded as pending, never run
 *	GET   /missions/{id}/steps       the steps added by hand and those of the current plan
 *	POST  /missions/{id}/plan        {replace?}  prepares the plan; Idempotency-Key honoured, the same request gives the
 *	                                 same plan; a different existing plan is replaced only when asked (replace: true)
 *	GET   /missions/{id}/plan?version=   the current plan, or one version of its history
 *	POST  /missions/{id}/steps/{step_id}/execute   runs ONE step through the Commerce Action Runtime; Idempotency-Key
 *	                                 honoured (the same step and key is the same execution); no body
 *
 *	POST  /missions/{id}/run         {on_failure?: "stop"|"continue"}  orchestrates the mission: runs, in dependency order,
 *	                                 the steps that are ready. 200 with the state; 202 when another run holds the mission
 *	POST  /missions/{id}/resume      the same, and also runs once the steps that failed or went silent
 *	GET   /missions/{id}/execution   the state of the steps and the mission's outcome (changes nothing)
 *
 * Planning proposes steps (intentions to act). Running a step, or a whole mission, is explicit, on request: the runtime
 * is the only place where the action runs and where credits are checked, reserved, consumed or released.
 */

// MissionService is what the mission routes need (application.MissionUseCases).
type MissionService interface {
	Create(ctx context.Context, actorID string, in application.CreateMissionInput, idempotencyKey string) (*domain.Mission, bool, error)
	Get(ctx context.Context, actorID, id string) (*domain.Mission, error)
	List(ctx context.Context, actorID string, status domain.MissionStatus, page domain.Page) ([]domain.Mission, int, error)
	Update(ctx context.Context, actorID, id string, p application.MissionPatch) (*domain.Mission, error)
	AddStep(ctx context.Context, actorID, missionID string, in application.StepInput) (*domain.MissionStep, error)
	Steps(ctx context.Context, actorID, missionID string) ([]domain.MissionStep, error)
	Plan(ctx context.Context, actorID, missionID string, opts application.PlanOptions) (*application.PlanResult, error)
	GetPlan(ctx context.Context, actorID, missionID string, version int) (*application.PlanResult, error)
	StepsView(ctx context.Context, actorID, missionID string) (*application.StepsResult, error)
}

// StepExecutor runs one mission step (missionexec.Executor).
type StepExecutor interface {
	Execute(ctx context.Context, actorID, missionID, stepID, idempotencyKey string) (*domain.StepExecutionResult, error)
}

// MissionHandler serves the mission routes.
type MissionHandler struct {
	svc  MissionService
	exec StepExecutor
	orch MissionOrchestrator
}

// NewMissionHandler creates a MissionHandler.
func NewMissionHandler(svc MissionService) *MissionHandler { return &MissionHandler{svc: svc} }

// WithExecution enables running a step.
func (h *MissionHandler) WithExecution(exec StepExecutor) *MissionHandler {
	h.exec = exec
	return h
}

const (
	maxMissionBody = 64 << 10
	missionDateFmt = "2006-01-02"
)

func missionJSON(m *domain.Mission) map[string]interface{} {
	date := func(unix int64) interface{} {
		if unix == 0 {
			return nil
		}
		return time.Unix(unix, 0).UTC().Format(missionDateFmt)
	}
	prefs := m.Preferences
	if prefs == nil {
		prefs = map[string]string{}
	}
	return map[string]interface{}{
		"id": m.ID, "type": m.Type, "title": m.Title, "objective": m.Objective, "status": m.Status,
		"location": m.Location, "start_date": date(m.StartDate), "end_date": date(m.EndDate),
		"participants": m.Participants, "budget_minor": m.BudgetMinor, "currency": m.Currency, "preferences": prefs,
		"created_at": isoTime(m.CreatedAt), "updated_at": isoTime(m.UpdatedAt),
	}
}

func stepJSON(s domain.MissionStep) map[string]interface{} {
	return map[string]interface{}{
		"id": s.ID, "mission_id": s.MissionID, "type": s.Type, "title": s.Title, "description": s.Description,
		"status": s.Status, "position": s.Position, "action_type": s.ActionType, "action_support": s.ActionSupport,
		"plan_version": s.PlanVersion, "action_ref": s.ActionRef, "result_ref": s.ResultRef, "created_at": isoTime(s.CreatedAt), "updated_at": isoTime(s.UpdatedAt),
	}
}

// executionJSON is one attempt to run a step as clients read it: a stable status and error code, and the result as a
// small summary with references. Nothing of the runtime's internals is exposed.
func executionJSON(e *domain.MissionStepExecution) map[string]interface{} {
	out := map[string]interface{}{
		"id": e.ID, "attempt": e.Attempt, "status": e.Status, "action_type": e.ActionType, "started_at": isoTime(e.StartedAt),
	}
	if e.CompletedAt != 0 {
		out["completed_at"] = isoTime(e.CompletedAt)
	}
	switch e.Status {
	case domain.ExecutionCompleted:
		result := map[string]interface{}{"reference": e.ResultReference}
		for k, v := range e.ResultPayload {
			result[k] = v
		}
		out["result"] = result
	case domain.ExecutionFailed:
		out["error"] = map[string]interface{}{"code": e.ErrorCode}
	}
	return out
}

// stepWithState is a step with what a client needs to run it: whether it can be run at all and now, and its latest
// attempt.
func stepWithState(s domain.MissionStep, st domain.MissionStepState) map[string]interface{} {
	out := stepJSON(s)
	out["executable"], out["can_execute"], out["stale"] = st.Executable, st.CanExecute, st.Stale
	if st.Execution != nil {
		out["execution"] = executionJSON(st.Execution)
	}
	// A step of the current plan also says where it stands in the plan's graph: its state (pending, ready, running,
	// completed, failed, blocked, skipped), what it waits for and, when blocked, what blocks it and why.
	if st.Flow != "" {
		out["state"] = st.Flow
		out["depends_on"] = nonNilStrings(st.DependsOn)
		out["blocked_by"] = nonNilStrings(st.BlockedBy)
		if st.Reason != "" {
			out["reason"] = st.Reason
		}
		if st.Detail != "" {
			out["detail"] = st.Detail
		}
	}
	return out
}

func nonNilStrings(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// missionError answers the HTTP status a mission error maps to.
func missionError(w http.ResponseWriter, err error) {
	code, status, msg := "INTERNAL_ERROR", http.StatusInternalServerError, InternalErrorMessage
	switch {
	case errors.Is(err, domain.ErrMissionNotFound):
		code, status, msg = "NOT_FOUND", http.StatusNotFound, "Mission not found"
	case errors.Is(err, domain.ErrMissionForbidden):
		code, status, msg = "FORBIDDEN", http.StatusForbidden, "This mission belongs to another account"
	case errors.Is(err, domain.ErrMissionInvalid):
		code, status, msg = "INVALID_MISSION", http.StatusBadRequest, strings.TrimPrefix(err.Error(), domain.ErrMissionInvalid.Error()+": ")
	case errors.Is(err, domain.ErrInvalidMissionTransition):
		code, status, msg = "INVALID_TRANSITION", http.StatusConflict, "The mission cannot move to that status from where it is"
	case errors.Is(err, domain.ErrMissionClosed):
		code, status, msg = "MISSION_CLOSED", http.StatusConflict, "The mission is closed"
	case errors.Is(err, domain.ErrMissionStepLimit):
		code, status, msg = "STEP_LIMIT", http.StatusConflict, "The mission already has the most steps allowed"
	case errors.Is(err, domain.ErrIdempotencyConflict):
		code, status, msg = "IDEMPOTENCY_CONFLICT", http.StatusConflict, "Idempotency-Key was already used for a different mission"
	case errors.Is(err, domain.ErrMissionPlanNotFound):
		code, status, msg = "NOT_FOUND", http.StatusNotFound, "Plan not found"
	case errors.Is(err, domain.ErrMissionPlanExists):
		code, status, msg = "PLAN_EXISTS", http.StatusConflict, "The mission already has a different plan; ask to replace it to prepare a new one"
	case errors.Is(err, domain.ErrMissionPlanInProgress):
		code, status, msg = "PLAN_IN_PROGRESS", http.StatusConflict, "The plan has steps under way and cannot be replaced"
	case errors.Is(err, domain.ErrMissionPlanConflict):
		code, status, msg = "PLAN_CONFLICT", http.StatusConflict, "The plan changed meanwhile; read it again"
	case errors.Is(err, domain.ErrMissionStepNotFound), errors.Is(err, domain.ErrMissionExecutionNotFound):
		code, status, msg = "NOT_FOUND", http.StatusNotFound, "Step not found"
	case errors.Is(err, domain.ErrStepNotExecutable):
		code, status, msg = "STEP_NOT_EXECUTABLE", http.StatusUnprocessableEntity, "This step cannot be executed yet"
	case errors.Is(err, domain.ErrStepInProgress):
		code, status, msg = "STEP_IN_PROGRESS", http.StatusConflict, "The step is already being executed"
	case errors.Is(err, domain.ErrStepNotRunnable):
		code, status, msg = "STEP_NOT_RUNNABLE", http.StatusConflict, "The step cannot be executed in its current state"
	case errors.Is(err, domain.ErrMissionNotRunnable):
		code, status, msg = "MISSION_NOT_RUNNABLE", http.StatusConflict, "The mission does not allow running steps now"
	case errors.Is(err, domain.ErrStepWaiting):
		code, status, msg = "STEP_WAITING", http.StatusConflict, "The step waits for steps it depends on that have not completed"
	case errors.Is(err, domain.ErrStepInputMissing):
		code, status, msg = "STEP_NEEDS_INPUT", http.StatusConflict, "The step is missing something it needs"
		if detail := strings.TrimPrefix(err.Error(), domain.ErrStepInputMissing.Error()+": "); detail != err.Error() && len(detail) <= 40 {
			msg += ": " + detail // a stable word: dates, listing
		}
	case errors.Is(err, domain.ErrNoPlanToRun):
		code, status, msg = "PLAN_REQUIRED", http.StatusConflict, "The mission has no plan to run; prepare one first"
	case errors.Is(err, domain.ErrDependencyCycle), errors.Is(err, domain.ErrDependencyInvalid):
		code, status, msg = "MISSION_GRAPH_INVALID", http.StatusConflict, "The steps of the plan depend on each other in a way that cannot be run"
	case errors.Is(err, domain.ErrMissionBusy):
		code, status, msg = "MISSION_BUSY", http.StatusConflict, "The mission is being run"
	case errors.Is(err, domain.ErrExecutionSuperseded):
		code, status, msg = "EXECUTION_SUPERSEDED", http.StatusConflict, "The execution was superseded"
	case errors.Is(err, domain.ErrPlanningUnavailable):
		code, status, msg = "PLANNING_UNAVAILABLE", http.StatusServiceUnavailable, InternalErrorMessage
	}
	writeJSON(w, ErrorResponse{Error: Error{Code: code, Message: msg}}, status)
}

// decodeMission reads a strict JSON body: unknown fields are refused so a misspelt field is not silently lost.
func decodeMission(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMissionBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return false
	}
	return true
}

func parseMissionDate(raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	d, err := time.Parse(missionDateFmt, raw)
	if err != nil {
		return 0, false
	}
	return d.UTC().Unix(), true
}

func badMissionDate(w http.ResponseWriter) {
	writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_MISSION", Message: "Dates are written YYYY-MM-DD"}}, http.StatusBadRequest)
}

// missionID reads {id} of the path; a malformed one cannot exist.
func missionID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("id")
	if !orgIDPattern.MatchString(id) {
		missionError(w, domain.ErrMissionNotFound)
		return "", false
	}
	return id, true
}

// stepID reads {step_id} of the path; a malformed one cannot exist.
func stepID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("step_id")
	if !orgIDPattern.MatchString(id) {
		missionError(w, domain.ErrMissionStepNotFound)
		return "", false
	}
	return id, true
}

type missionBody struct {
	Objective    *string               `json:"objective"`
	Type         *string               `json:"type"`
	Title        *string               `json:"title"`
	Location     *string               `json:"location"`
	Participants *int                  `json:"participants"`
	StartDate    *string               `json:"start_date"`
	EndDate      *string               `json:"end_date"`
	BudgetMinor  *int64                `json:"budget_minor"`
	Currency     *string               `json:"currency"`
	Preferences  map[string]string     `json:"preferences"`
	Status       *domain.MissionStatus `json:"status"`
}

func str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// CreateH handles POST /api/v1/missions.
func (h *MissionHandler) CreateH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	var body missionBody
	if !decodeMission(w, r, &body) {
		return
	}
	if body.Status != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_MISSION", Message: "A new mission is always a draft"}}, http.StatusBadRequest)
		return
	}
	start, ok1 := parseMissionDate(str(body.StartDate))
	end, ok2 := parseMissionDate(str(body.EndDate))
	if !ok1 || !ok2 {
		badMissionDate(w)
		return
	}
	in := application.CreateMissionInput{
		Objective: str(body.Objective), Type: str(body.Type), Title: str(body.Title), Location: str(body.Location),
		StartDate: start, EndDate: end, Currency: str(body.Currency), Preferences: body.Preferences,
	}
	if body.Participants != nil {
		in.Participants = *body.Participants
	}
	if body.BudgetMinor != nil {
		in.BudgetMinor = *body.BudgetMinor
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 100 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Idempotency-Key is at most 100 characters"}}, http.StatusBadRequest)
		return
	}
	m, replay, err := h.svc.Create(r.Context(), actorID, in, key)
	if err != nil {
		missionError(w, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	writeJSON(w, missionJSON(m), status)
}

// ListH handles GET /api/v1/missions.
func (h *MissionHandler) ListH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	page, pageNo := consolePage(r, 20)
	items, total, err := h.svc.List(r.Context(), actorID, domain.MissionStatus(r.URL.Query().Get("status")), page)
	if err != nil {
		missionError(w, err)
		return
	}
	out := make([]map[string]interface{}, 0, len(items))
	for i := range items {
		out = append(out, missionJSON(&items[i]))
	}
	writeJSON(w, paged(out, total, pageNo, page), http.StatusOK)
}

// GetH handles GET /api/v1/missions/{id}.
func (h *MissionHandler) GetH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	m, err := h.svc.Get(r.Context(), actorID, id)
	if err != nil {
		missionError(w, err)
		return
	}
	writeJSON(w, missionJSON(m), http.StatusOK)
}

// UpdateH handles PATCH /api/v1/missions/{id}.
func (h *MissionHandler) UpdateH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	var body missionBody
	if !decodeMission(w, r, &body) {
		return
	}
	patch := application.MissionPatch{
		Type: body.Type, Title: body.Title, Objective: body.Objective, Location: body.Location, Participants: body.Participants,
		BudgetMinor: body.BudgetMinor, Currency: body.Currency, Preferences: body.Preferences, Status: body.Status,
	}
	if body.StartDate != nil {
		d, ok := parseMissionDate(*body.StartDate)
		if !ok {
			badMissionDate(w)
			return
		}
		patch.StartDate = &d
	}
	if body.EndDate != nil {
		d, ok := parseMissionDate(*body.EndDate)
		if !ok {
			badMissionDate(w)
			return
		}
		patch.EndDate = &d
	}
	m, err := h.svc.Update(r.Context(), actorID, id, patch)
	if err != nil {
		missionError(w, err)
		return
	}
	writeJSON(w, missionJSON(m), http.StatusOK)
}

// AddStepH handles POST /api/v1/missions/{id}/steps.
func (h *MissionHandler) AddStepH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	var body struct {
		Type        string `json:"type"`
		Title       string `json:"title"`
		Description string `json:"description"`
		ActionType  string `json:"action_type"`
	}
	if !decodeMission(w, r, &body) {
		return
	}
	s, err := h.svc.AddStep(r.Context(), actorID, id, application.StepInput{Type: body.Type, Title: body.Title, Description: body.Description, ActionType: body.ActionType})
	if err != nil {
		missionError(w, err)
		return
	}
	writeJSON(w, stepJSON(*s), http.StatusCreated)
}

// StepsH handles GET /api/v1/missions/{id}/steps: the steps added by hand, then those of the current plan, each with
// whether it can be run and how its latest attempt went.
func (h *MissionHandler) StepsH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	view, err := h.svc.StepsView(r.Context(), actorID, id)
	if err != nil {
		missionError(w, err)
		return
	}
	out := make([]map[string]interface{}, 0, len(view.Steps))
	for _, s := range view.Steps {
		out = append(out, stepWithState(s, view.States[s.ID]))
	}
	writeJSON(w, map[string]interface{}{"items": out, "total": len(out)}, http.StatusOK)
}

// planJSON is a plan as the clients read it: its steps are intentions (pending, with the action each will run and
// whether the Commerce OS can run it today), never results.
func planJSON(res *application.PlanResult) map[string]interface{} {
	steps := []map[string]interface{}{}
	out := map[string]interface{}{
		"mission_id": res.Mission.ID, "mission_type": res.Mission.Type, "plannable": res.Plannable,
		"versions": res.Versions, "steps": steps, "plan_version": 0, "status": "none",
	}
	if res.NeedsInformation {
		out["status"], out["needs_information"], out["reason"] = "needs_information", true, res.Reason
	}
	if p := res.Plan; p != nil {
		for _, st := range p.Steps {
			steps = append(steps, stepWithState(st, res.States[st.ID]))
		}
		out["steps"], out["plan_version"], out["status"] = steps, p.Version, string(p.Status)
		out["plan_mission_type"], out["planner"], out["planner_version"] = p.MissionType, p.Planner, p.PlannerVersion
		out["created_at"] = isoTime(p.CreatedAt)
		if p.SupersededAt != 0 {
			out["superseded_at"] = isoTime(p.SupersededAt)
		}
	}
	return out
}

// PlanH handles POST /api/v1/missions/{id}/plan. The body is optional: {"replace": true} asks for a new version when
// the mission already has a different plan. 201 when a plan was stored, 200 for the plan that already exists (the
// same request twice gives the same plan) or when the mission cannot be planned as it is (needs_information).
func (h *MissionHandler) PlanH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	var body struct {
		Replace bool `json:"replace"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMissionBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
			return
		}
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 100 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Idempotency-Key is at most 100 characters"}}, http.StatusBadRequest)
		return
	}
	res, err := h.svc.Plan(r.Context(), actorID, id, application.PlanOptions{Replace: body.Replace, IdempotencyKey: key})
	if err != nil {
		missionError(w, err)
		return
	}
	status := http.StatusOK
	if res.Created {
		status = http.StatusCreated
	}
	writeJSON(w, planJSON(res), status)
}

// GetPlanH handles GET /api/v1/missions/{id}/plan?version=N: the current plan, or one version of its history. A
// mission without a plan answers 200 with status "none", which is a state, not an error.
func (h *MissionHandler) GetPlanH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	version := 0
	if raw := r.URL.Query().Get("version"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "version must be a whole number from 1"}}, http.StatusBadRequest)
			return
		}
		version = n
	}
	res, err := h.svc.GetPlan(r.Context(), actorID, id, version)
	if err != nil {
		missionError(w, err)
		return
	}
	writeJSON(w, planJSON(res), http.StatusOK)
}

// ExecuteStepH handles POST /api/v1/missions/{id}/steps/{step_id}/execute: it runs ONE step through the Commerce
// Action Runtime. No body: what the step asks of the runtime is decided server-side from the mission and the step,
// never by the client. 201 when an attempt was run (completed or failed: a failure is a recorded outcome, readable
// in `status` and `error.code`), 200 when the request repeated an Idempotency-Key and returned that attempt. 409 when
// the step is already running, already completed or the mission does not allow running steps; 422 when the step has
// no executable action yet.
func (h *MissionHandler) ExecuteStepH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	sid, ok := stepID(w, r)
	if !ok {
		return
	}
	if r.ContentLength > 0 {
		var body struct{}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMissionBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "This request takes no body"}}, http.StatusBadRequest)
			return
		}
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 100 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Idempotency-Key is at most 100 characters"}}, http.StatusBadRequest)
		return
	}
	if h.exec == nil {
		missionError(w, domain.ErrStepNotExecutable)
		return
	}
	res, err := h.exec.Execute(r.Context(), actorID, id, sid, key)
	if err != nil {
		missionError(w, err)
		return
	}
	out := executionJSON(res.Execution)
	out["mission_id"], out["step_id"], out["execution_id"], out["replayed"] = res.Mission.ID, res.Step.ID, res.Execution.ID, res.Replayed
	out["step_status"] = res.Step.Status
	out["step"] = stepWithState(*res.Step, res.State)
	status := http.StatusCreated
	if res.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, out, status)
}
