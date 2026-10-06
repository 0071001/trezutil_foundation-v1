package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/trezutil/trezutil/internal/application/missionorch"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// MissionOrchestrator runs a whole mission (missionorch.Orchestrator).
type MissionOrchestrator interface {
	Run(ctx context.Context, actorID, missionID string, opts missionorch.Options) (*missionorch.State, error)
	State(ctx context.Context, actorID, missionID string) (*missionorch.State, error)
}

// WithOrchestration enables running a whole mission.
func (h *MissionHandler) WithOrchestration(o MissionOrchestrator) *MissionHandler {
	h.orch = o
	return h
}

// ranJSON is a step this request ran.
func ranJSON(r missionorch.Ran) map[string]interface{} {
	out := map[string]interface{}{
		"step_id": r.StepID, "step_type": r.StepType, "action_type": r.ActionType,
		"execution_id": r.ExecutionID, "attempt": r.Attempt, "status": r.Status,
	}
	if r.ErrorCode != "" {
		out["error_code"] = r.ErrorCode
	}
	return out
}

// orchestrationJSON is the execution state of a mission as clients read it: the mission's outcome and progress, what
// this request ran and every step of the current plan with its state. Nothing of the runtime's internals is exposed.
func orchestrationJSON(st *missionorch.State) map[string]interface{} {
	steps := make([]map[string]interface{}, 0, len(st.Flow.Steps))
	for _, s := range st.Flow.Steps {
		steps = append(steps, stepWithState(s.Step, s.State))
	}
	ran := make([]map[string]interface{}, 0, len(st.Ran))
	for _, r := range st.Ran {
		ran = append(ran, ranJSON(r))
	}
	return map[string]interface{}{
		"mission_id": st.Mission.ID, "plan_version": st.Flow.PlanVersion, "mission_status": st.Mission.Status,
		"outcome": st.Outcome, "busy": st.Busy,
		"progress": map[string]int{"completed": st.Progress.Completed, "skipped": st.Progress.Skipped, "total": st.Progress.Total},
		"ran":      ran, "steps": steps,
	}
}

// runBody is the optional body of a run or a resume.
type runBody struct {
	OnFailure string `json:"on_failure"`
}

// RunH handles POST /api/v1/missions/{id}/run: it orchestrates the mission, running in dependency order the steps that
// are ready (each one through the Commerce Action Runtime, which alone checks, reserves and consumes credits). The body
// is optional: {"on_failure": "stop"} (the default: stop at the first failure) or "continue" (carry on with the steps
// that do not depend on the failed one). A run never retries a failed step (that is a resume), never touches a
// confirmation (such a step is blocked for the person), and runs nothing that is already completed: repeating the
// request is safe. 200 with the state; 202 with the state when another run holds the mission (nothing was run by this
// request); 409 PLAN_REQUIRED, MISSION_GRAPH_INVALID, MISSION_NOT_RUNNABLE.
func (h *MissionHandler) RunH(w http.ResponseWriter, r *http.Request) { h.orchestrate(w, r, false) }

// ResumeH handles POST /api/v1/missions/{id}/resume: a run that also runs, once each, the steps whose latest attempt
// failed or went silent. Steps that completed are never run again.
func (h *MissionHandler) ResumeH(w http.ResponseWriter, r *http.Request) { h.orchestrate(w, r, true) }

func (h *MissionHandler) orchestrate(w http.ResponseWriter, r *http.Request, retry bool) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	var body runBody
	if r.Body != nil && r.ContentLength != 0 {
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMissionBody))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
			return
		}
	}
	policy := missionorch.OnFailure(body.OnFailure)
	if policy == "" {
		policy = missionorch.StopOnFailure
	}
	if !policy.IsValid() {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: `on_failure is "stop" or "continue"`}}, http.StatusBadRequest)
		return
	}
	if h.orch == nil {
		missionError(w, domain.ErrStepNotExecutable)
		return
	}
	st, err := h.orch.Run(r.Context(), actorID, id, missionorch.Options{RetryFailed: retry, OnFailure: policy})
	if err != nil {
		missionError(w, err)
		return
	}
	status := http.StatusOK
	if st.Busy {
		status = http.StatusAccepted
	}
	writeJSON(w, orchestrationJSON(st), status)
}

// ExecutionH handles GET /api/v1/missions/{id}/execution: the execution state of the mission's current plan. It runs
// and changes nothing. 409 PLAN_REQUIRED when the mission has no plan, MISSION_GRAPH_INVALID when its stored graph
// cannot be ordered.
func (h *MissionHandler) ExecutionH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actorID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	id, ok := missionID(w, r)
	if !ok {
		return
	}
	if h.orch == nil {
		missionError(w, domain.ErrNoPlanToRun)
		return
	}
	st, err := h.orch.State(r.Context(), actorID, id)
	if err != nil {
		missionError(w, err)
		return
	}
	writeJSON(w, orchestrationJSON(st), http.StatusOK)
}
