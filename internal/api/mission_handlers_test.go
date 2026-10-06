package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// memMissions is the smallest MissionRepository the routes need.
type memMissions struct {
	missions map[string]*domain.Mission
	steps    map[string][]domain.MissionStep
	plans    map[string][]*domain.MissionPlan // plan versions by mission, oldest first
}

func (r *memMissions) Create(_ context.Context, m *domain.Mission) error {
	for _, e := range r.missions {
		if m.IdempotencyKey != "" && e.OwnerID == m.OwnerID && e.IdempotencyKey == m.IdempotencyKey {
			return domain.ErrIdempotencyConflict
		}
	}
	c := *m
	r.missions[m.ID] = &c
	return nil
}
func (r *memMissions) FindByIdempotencyKey(_ context.Context, owner, key string) (*domain.Mission, error) {
	for _, m := range r.missions {
		if m.OwnerID == owner && m.IdempotencyKey == key {
			c := *m
			return &c, nil
		}
	}
	return nil, domain.ErrMissionNotFound
}
func (r *memMissions) Get(_ context.Context, id string) (*domain.Mission, error) {
	if m, ok := r.missions[id]; ok {
		c := *m
		return &c, nil
	}
	return nil, domain.ErrMissionNotFound
}
func (r *memMissions) ListByOwner(_ context.Context, owner string, status domain.MissionStatus, p domain.Page) ([]domain.Mission, int, error) {
	out := []domain.Mission{}
	for _, m := range r.missions {
		if m.OwnerID == owner && (status == "" || m.Status == status) {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, len(out), nil
}
func (r *memMissions) Update(_ context.Context, m *domain.Mission) error {
	c := *m
	r.missions[m.ID] = &c
	return nil
}
func (r *memMissions) AddStep(_ context.Context, s *domain.MissionStep, max int) error {
	if len(r.steps[s.MissionID]) >= max {
		return domain.ErrMissionStepLimit
	}
	s.Position = len(r.steps[s.MissionID]) + 1
	r.steps[s.MissionID] = append(r.steps[s.MissionID], *s)
	return nil
}
func (r *memMissions) Steps(_ context.Context, id string) ([]domain.MissionStep, error) {
	out := append([]domain.MissionStep{}, r.steps[id]...)
	if p := r.currentPlan(id); p != nil {
		out = append(out, p.Steps...)
	}
	return out, nil
}

func missionServer() http.Handler {
	h := NewMissionHandler(application.NewMissionUseCases(&memMissions{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}}))
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions", h.CreateH)
	mux.HandleFunc("GET /api/v1/missions", h.ListH)
	mux.HandleFunc("GET /api/v1/missions/{id}", h.GetH)
	mux.HandleFunc("PATCH /api/v1/missions/{id}", h.UpdateH)
	mux.HandleFunc("POST /api/v1/missions/{id}/steps", h.AddStepH)
	mux.HandleFunc("GET /api/v1/missions/{id}/steps", h.StepsH)
	return mux
}

func callMission(t *testing.T, srv http.Handler, user, method, path, body string, headers ...string) (int, map[string]interface{}) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	out := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func errCode(body map[string]interface{}) string {
	e, _ := body["error"].(map[string]interface{})
	c, _ := e["code"].(string)
	return c
}

const aliceID = "11111111-1111-4111-8111-111111111111"
const bobID = "22222222-2222-4222-8222-222222222222"

func createBirthday(t *testing.T, srv http.Handler, user string) string {
	t.Helper()
	code, body := callMission(t, srv, user, "POST", "/api/v1/missions", `{"objective":"Je veux organiser un anniversaire pour 40 personnes à Kinshasa"}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, body)
	}
	return body["id"].(string)
}

func TestMissionAPI_NeedsASignedInUser(t *testing.T) {
	srv := missionServer()
	id := createBirthday(t, srv, aliceID)
	for _, c := range [][3]string{
		{"POST", "/api/v1/missions", `{"objective":"Un voyage à Goma"}`},
		{"GET", "/api/v1/missions", ""},
		{"GET", "/api/v1/missions/" + id, ""},
		{"PATCH", "/api/v1/missions/" + id, `{"title":"Autre"}`},
		{"POST", "/api/v1/missions/" + id + "/steps", `{"type":"VENUE","title":"Salle"}`},
		{"GET", "/api/v1/missions/" + id + "/steps", ""},
	} {
		if code, _ := callMission(t, srv, "", c[0], c[1], c[2]); code != http.StatusUnauthorized {
			t.Errorf("%s %s without a user = %d, want 401", c[0], c[1], code)
		}
	}
}

func TestMissionAPI_CreateIsADraftWithWhatWasUnderstood(t *testing.T) {
	srv := missionServer()
	code, m := callMission(t, srv, aliceID, "POST", "/api/v1/missions", `{"objective":"Je veux organiser un anniversaire pour 40 personnes à Kinshasa"}`)
	if code != http.StatusCreated {
		t.Fatalf("create = %d %v", code, m)
	}
	if m["status"] != "draft" || m["type"] != "EVENT" || m["title"] != "Anniversaire" || m["location"] != "Kinshasa" || m["participants"] != float64(40) {
		t.Fatalf("unexpected mission %v", m)
	}
	if m["start_date"] != nil || m["created_at"] == "" || m["preferences"] == nil {
		t.Fatalf("dates stay empty, timestamps and preferences are present: %v", m)
	}
	if _, leaked := m["owner_id"]; leaked {
		t.Fatal("the owner is implied by the token, not echoed")
	}
}

func TestMissionAPI_CreateWithGivenFields(t *testing.T) {
	srv := missionServer()
	code, m := callMission(t, srv, aliceID, "POST", "/api/v1/missions",
		`{"objective":"Un voyage à Goma","type":"travel","title":"Goma 2027","participants":2,"start_date":"2027-01-10","end_date":"2027-01-17","budget_minor":150000,"currency":"usd","preferences":{"hotel":"calme"}}`)
	if code != http.StatusCreated || m["type"] != "TRAVEL" || m["start_date"] != "2027-01-10" || m["end_date"] != "2027-01-17" || m["currency"] != "USD" {
		t.Fatalf("create = %d %v", code, m)
	}
	if prefs, _ := m["preferences"].(map[string]interface{}); prefs["hotel"] != "calme" {
		t.Fatalf("preferences lost: %v", m["preferences"])
	}
}

func TestMissionAPI_CreateValidation(t *testing.T) {
	srv := missionServer()
	for name, body := range map[string]string{
		"no objective":  `{}`,
		"short":         `{"objective":"ab"}`,
		"bad type":      `{"objective":"Un voyage à Goma","type":"wedding"}`,
		"bad date":      `{"objective":"Un voyage à Goma","start_date":"10/01/2027"}`,
		"dates swapped": `{"objective":"Un voyage à Goma","start_date":"2027-02-01","end_date":"2027-01-01"}`,
		"budget only":   `{"objective":"Un voyage à Goma","budget_minor":500}`,
		"status given":  `{"objective":"Un voyage à Goma","status":"active"}`,
		"unknown field": `{"objective":"Un voyage à Goma","owner_id":"` + bobID + `"}`,
		"not json":      `objective=voyage`,
	} {
		code, body := callMission(t, srv, aliceID, "POST", "/api/v1/missions", body)
		if code != http.StatusBadRequest {
			t.Errorf("%s = %d %v, want 400", name, code, body)
		}
	}
	if code, body := callMission(t, srv, aliceID, "POST", "/api/v1/missions", `{"objective":"ab"}`); code != 400 || errCode(body) != "INVALID_MISSION" {
		t.Fatalf("a validation failure says why: %d %v", code, body)
	}
}

func TestMissionAPI_IdempotentCreate(t *testing.T) {
	srv := missionServer()
	body := `{"objective":"Je veux organiser un anniversaire pour 40 personnes à Kinshasa"}`
	code1, first := callMission(t, srv, aliceID, "POST", "/api/v1/missions", body, "Idempotency-Key", "k1")
	code2, again := callMission(t, srv, aliceID, "POST", "/api/v1/missions", body, "Idempotency-Key", "k1")
	if code1 != http.StatusCreated || code2 != http.StatusOK || first["id"] != again["id"] {
		t.Fatalf("a retry returns the first mission: %d %d", code1, code2)
	}
	if code, b := callMission(t, srv, aliceID, "POST", "/api/v1/missions", `{"objective":"Un voyage à Goma"}`, "Idempotency-Key", "k1"); code != http.StatusConflict || errCode(b) != "IDEMPOTENCY_CONFLICT" {
		t.Fatalf("same key, other mission = %d %v", code, b)
	}
	if code, _ := callMission(t, srv, aliceID, "GET", "/api/v1/missions", ""); code != 200 {
		t.Fatal("list failed")
	}
	_, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions", "")
	if list["total"] != float64(1) {
		t.Fatalf("a retried creation makes one mission, got %v", list["total"])
	}
}

func TestMissionAPI_ListShowsOnlyYourOwn(t *testing.T) {
	srv := missionServer()
	createBirthday(t, srv, aliceID)
	createBirthday(t, srv, aliceID)
	createBirthday(t, srv, bobID)
	_, list := callMission(t, srv, aliceID, "GET", "/api/v1/missions", "")
	items, _ := list["items"].([]interface{})
	if list["total"] != float64(2) || len(items) != 2 || list["page"] != float64(1) {
		t.Fatalf("alice sees her two: %v", list)
	}
	if code, body := callMission(t, srv, aliceID, "GET", "/api/v1/missions?status=active", ""); code != 200 || body["total"] != float64(0) {
		t.Fatalf("no active mission yet: %d %v", code, body)
	}
	if code, body := callMission(t, srv, aliceID, "GET", "/api/v1/missions?status=frozen", ""); code != http.StatusBadRequest {
		t.Fatalf("unknown status = %d %v", code, body)
	}
	_, empty := callMission(t, srv, "33333333-3333-4333-8333-333333333333", "GET", "/api/v1/missions", "")
	if items, ok := empty["items"].([]interface{}); !ok || len(items) != 0 {
		t.Fatalf("an empty list is [] not null: %v", empty)
	}
}

func TestMissionAPI_GetOwnForbiddenOthersAndNotFound(t *testing.T) {
	srv := missionServer()
	id := createBirthday(t, srv, aliceID)
	if code, m := callMission(t, srv, aliceID, "GET", "/api/v1/missions/"+id, ""); code != 200 || m["id"] != id {
		t.Fatalf("own mission = %d %v", code, m)
	}
	if code, b := callMission(t, srv, bobID, "GET", "/api/v1/missions/"+id, ""); code != http.StatusForbidden || errCode(b) != "FORBIDDEN" {
		t.Fatalf("another's mission = %d %v, want 403", code, b)
	}
	for _, path := range []string{"/api/v1/missions/44444444-4444-4444-8444-444444444444", "/api/v1/missions/not-an-id"} {
		if code, b := callMission(t, srv, aliceID, "GET", path, ""); code != http.StatusNotFound || errCode(b) != "NOT_FOUND" {
			t.Fatalf("%s = %d %v, want 404", path, code, b)
		}
	}
}

func TestMissionAPI_PatchFollowsTheLifecycle(t *testing.T) {
	srv := missionServer()
	id := createBirthday(t, srv, aliceID)
	path := "/api/v1/missions/" + id
	if code, m := callMission(t, srv, aliceID, "PATCH", path, `{"location":"Gombe","participants":55,"start_date":"2027-03-05","status":"active"}`); code != 200 ||
		m["location"] != "Gombe" || m["participants"] != float64(55) || m["start_date"] != "2027-03-05" || m["status"] != "active" {
		t.Fatalf("edit = %d %v", code, m)
	}
	if code, b := callMission(t, srv, aliceID, "PATCH", path, `{"status":"draft"}`); code != http.StatusConflict || errCode(b) != "INVALID_TRANSITION" {
		t.Fatalf("active -> draft = %d %v", code, b)
	}
	if code, _ := callMission(t, srv, aliceID, "PATCH", path, `{"start_date":"soon"}`); code != http.StatusBadRequest {
		t.Fatalf("bad date = %d", code)
	}
	if code, _ := callMission(t, srv, aliceID, "PATCH", path, `{"participants":-3}`); code != http.StatusBadRequest {
		t.Fatalf("negative participants = %d", code)
	}
	if code, b := callMission(t, srv, bobID, "PATCH", path, `{"title":"Volé"}`); code != http.StatusForbidden {
		t.Fatalf("another's mission = %d %v", code, b)
	}
	callMission(t, srv, aliceID, "PATCH", path, `{"status":"cancelled"}`)
	if code, b := callMission(t, srv, aliceID, "PATCH", path, `{"title":"Encore"}`); code != http.StatusConflict || errCode(b) != "MISSION_CLOSED" {
		t.Fatalf("a cancelled mission = %d %v", code, b)
	}
}

func TestMissionAPI_Steps(t *testing.T) {
	srv := missionServer()
	id := createBirthday(t, srv, aliceID)
	base := "/api/v1/missions/" + id + "/steps"
	code, first := callMission(t, srv, aliceID, "POST", base, `{"type":"venue","title":"Trouver une salle","description":"Pour 40 personnes"}`)
	if code != http.StatusCreated || first["position"] != float64(1) || first["status"] != "pending" || first["type"] != "VENUE" || first["mission_id"] != id {
		t.Fatalf("add step = %d %v", code, first)
	}
	if first["action_ref"] != "" || first["result_ref"] != "" {
		t.Fatalf("a recorded step has not run: %v", first)
	}
	callMission(t, srv, aliceID, "POST", base, `{"type":"CATERING","title":"Traiteur"}`)
	code, list := callMission(t, srv, aliceID, "GET", base, "")
	items, _ := list["items"].([]interface{})
	if code != 200 || len(items) != 2 || items[1].(map[string]interface{})["position"] != float64(2) {
		t.Fatalf("list steps = %d %v", code, list)
	}
	if code, _ := callMission(t, srv, aliceID, "POST", base, `{"type":"x","title":"Salle"}`); code != http.StatusBadRequest {
		t.Fatalf("bad step type = %d", code)
	}
	if code, _ := callMission(t, srv, aliceID, "POST", base, `{"type":"VENUE","title":"Salle","position":1}`); code != http.StatusBadRequest {
		t.Fatalf("a client cannot choose the position = %d", code)
	}
	if code, _ := callMission(t, srv, bobID, "POST", base, `{"type":"VENUE","title":"Salle"}`); code != http.StatusForbidden {
		t.Fatalf("adding to another's mission = %d", code)
	}
	if code, _ := callMission(t, srv, bobID, "GET", base, ""); code != http.StatusForbidden {
		t.Fatalf("reading another's steps = %d", code)
	}
	if code, _ := callMission(t, srv, aliceID, "GET", "/api/v1/missions/44444444-4444-4444-8444-444444444444/steps", ""); code != http.StatusNotFound {
		t.Fatalf("steps of an unknown mission = %d", code)
	}
	callMission(t, srv, aliceID, "PATCH", "/api/v1/missions/"+id, `{"status":"cancelled"}`)
	if code, b := callMission(t, srv, aliceID, "POST", base, `{"type":"VENUE","title":"Salle"}`); code != http.StatusConflict || errCode(b) != "MISSION_CLOSED" {
		t.Fatalf("step on a cancelled mission = %d %v", code, b)
	}
}
