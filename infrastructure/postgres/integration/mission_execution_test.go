package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/api"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/missionexec"
	"github.com/trezutil/trezutil/internal/application/missionorch"
	"github.com/trezutil/trezutil/internal/application/missionplanning"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

/*
 * Running a mission step end to end on the real schema (migration 000029): HTTP request, handler, step executor,
 * the REAL Commerce Action Runtime (its validation, idempotency, usage check and search handler) and PostgreSQL.
 * Only the catalogue search and the credit store behind the runtime are faked, to count what the runtime does with
 * credits.
 */

const (
	xAlice = "d0e50000-0000-4000-8000-0000000000c1"
	xBob   = "d0e50000-0000-4000-8000-0000000000c2"
)

type execSearch struct {
	mu      sync.Mutex
	queries []domain.DiscoveryQuery
	err     error
	delay   time.Duration
}

func (s *execSearch) ExecuteSearch(_ context.Context, _ string, q domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	s.mu.Lock()
	s.queries = append(s.queries, q)
	err, delay := s.err, s.delay
	s.mu.Unlock()
	time.Sleep(delay)
	if err != nil {
		return domain.SearchResultPage{}, err
	}
	return domain.SearchResultPage{Total: 3, Items: []domain.SearchResult{{ListingID: "l-1"}, {ListingID: "l-2"}, {ListingID: "l-3"}}}, nil
}
func (s *execSearch) Suggest(context.Context, string, domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{}, nil
}
func (s *execSearch) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.queries)
}
func (s *execSearch) set(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

type execCredits struct {
	mu                           sync.Mutex
	cost, balance                int64
	reserved, consumed, released int
	open                         map[string]int64
}

func (c *execCredits) GetBalance(context.Context, string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{AccountID: "acc", Granted: c.balance}, nil
}
func (c *execCredits) ResolvePolicy(_ context.Context, a domain.CommerceActionType, _ domain.Channel, _ domain.ActorType) (*domain.UsagePolicy, error) {
	if a == domain.ActionSearchListings {
		return &domain.UsagePolicy{ActionType: a, Cost: c.cost, Active: true}, nil
	}
	return nil, domain.ErrUsagePolicyNotFound
}
func (c *execCredits) Check(_ context.Context, _ string, cost int64) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.balance >= cost, nil
}
func (c *execCredits) Reserve(_ context.Context, _ string, _ domain.CommerceActionType, cost int64, actionID, _ string) (*domain.UsageReservation, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reserved++
	c.balance -= cost
	id := "res-" + actionID
	c.open[id] = cost
	return &domain.UsageReservation{ReservationID: id, Amount: cost}, nil
}
func (c *execCredits) Consume(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.open[id]; !ok {
		return domain.ErrUsageReservationNotFound
	}
	delete(c.open, id)
	c.consumed++
	return nil
}
func (c *execCredits) Release(_ context.Context, id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cost, ok := c.open[id]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	delete(c.open, id)
	c.balance += cost
	c.released++
	return nil
}
func (c *execCredits) Grant(context.Context, string, int64, string) error { return nil }
func (c *execCredits) FindOrCreateCreditAccountByOwner(_ context.Context, _ domain.ActorType, ownerID, _, _, _ string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{AccountID: "acc-" + ownerID, Granted: c.balance}, nil
}

// execAvail is the availability handler behind the runtime: it answers without a catalogue and records what it was asked.
type execAvail struct {
	mu    sync.Mutex
	asked []map[string]interface{}
}

func (a *execAvail) Handle(_ context.Context, _ domain.CommerceActor, p map[string]interface{}) (interface{}, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asked = append(a.asked, p)
	return map[string]interface{}{"available": true, "capacity": 4, "remaining": 4, "reason": "", "start_at": p["start_at"], "end_at": p["end_at"]}, nil
}
func (a *execAvail) ActionType() domain.CommerceActionType { return domain.ActionCheckAvailability }
func (a *execAvail) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.asked)
}

type execFixture struct {
	t       *testing.T
	db      *TestDB
	ctx     context.Context
	srv     http.Handler
	search  *execSearch
	avail   *execAvail
	credits *execCredits
	repo    *postgres.MissionRepository
	uc      *application.MissionUseCases
}

func newExecFixture(t *testing.T) *execFixture {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	ctx := context.Background()
	sellerUser(t, db, ctx, xAlice, "+243810000091")
	sellerUser(t, db, ctx, xBob, "+243810000092")
	repo := postgres.NewMissionRepository(db.DB)
	search := &execSearch{}
	credits := &execCredits{cost: 2, balance: 100, open: map[string]int64{}}
	registry := commerce.NewHandlerRegistry()
	registry.Register(commerce.NewSearchHandler(search))
	avail := &execAvail{}
	registry.Register(avail)
	runtime := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(), ParameterValidator: commerce.NewDefaultParameterValidator(), UsageService: credits,
	})
	executor := missionexec.New(repo, repo, repo, repo, runtime, zerolog.Nop())
	orchestrator := missionorch.New(repo, repo, executor, zerolog.Nop())
	uc := application.NewMissionUseCases(repo).WithPlanning(missionplanning.NewRuleBased(registry.Has), repo).WithStepStates(executor)
	h := api.NewMissionHandler(uc).WithExecution(executor).WithOrchestration(orchestrator)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/missions", h.CreateH)
	mux.HandleFunc("PATCH /api/v1/missions/{id}", h.UpdateH)
	mux.HandleFunc("GET /api/v1/missions/{id}/steps", h.StepsH)
	mux.HandleFunc("POST /api/v1/missions/{id}/plan", h.PlanH)
	mux.HandleFunc("POST /api/v1/missions/{id}/steps/{step_id}/execute", h.ExecuteStepH)
	mux.HandleFunc("POST /api/v1/missions/{id}/run", h.RunH)
	mux.HandleFunc("POST /api/v1/missions/{id}/resume", h.ResumeH)
	mux.HandleFunc("GET /api/v1/missions/{id}/execution", h.ExecutionH)
	return &execFixture{t: t, db: db, ctx: ctx, srv: mux, search: search, avail: avail, credits: credits, repo: repo, uc: uc}
}

func (f *execFixture) call(user, method, path, body string, headers ...string) (int, map[string]interface{}) {
	f.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	f.srv.ServeHTTP(rec, req)
	out := map[string]interface{}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// plan creates a mission from a sentence, plans it and returns its id and its steps by type.
func (f *execFixture) plan(user, sentence string) (string, map[string]string) {
	f.t.Helper()
	code, m := f.call(user, "POST", "/api/v1/missions", `{"objective":"`+sentence+`"}`)
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

func (f *execFixture) exec(user, mission, step string, headers ...string) (int, map[string]interface{}) {
	return f.call(user, "POST", "/api/v1/missions/"+mission+"/steps/"+step+"/execute", "", headers...)
}

func (f *execFixture) n(query string, args ...interface{}) int {
	f.t.Helper()
	var n int
	if err := f.db.DB.QueryRowContext(f.ctx, query, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func xErr(body map[string]interface{}) string {
	e, _ := body["error"].(map[string]interface{})
	c, _ := e["code"].(string)
	return c
}

func TestMissionExecution_EndToEndOnTheRealSchema(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.plan(xAlice, "Voyage à Nairobi pendant 5 jours")
	stay, transport, activities := steps["FIND_ACCOMMODATION"], steps["FIND_TRANSPORT"], steps["FIND_ACTIVITIES"]

	// The steps list says which can be run.
	_, list := f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", "")
	can := map[string]bool{}
	for _, raw := range list["items"].([]interface{}) {
		s := raw.(map[string]interface{})
		can[s["type"].(string)] = s["can_execute"].(bool)
		if s["execution"] != nil {
			t.Fatalf("nothing has run yet: %v", s)
		}
	}
	if !can["FIND_ACCOMMODATION"] || !can["FIND_TRANSPORT"] || can["FIND_ACTIVITIES"] {
		t.Fatalf("can_execute %v", can)
	}

	// Happy path: step -> executor -> runtime -> existing search -> durable result -> COMPLETED.
	code1, res := f.exec(xAlice, mission, stay, "Idempotency-Key", "k-1")
	if code1 != http.StatusCreated || res["status"] != "completed" || res["step_status"] != "completed" || res["replayed"] != false || res["action_type"] != "SEARCH_LISTINGS" {
		t.Fatalf("execute = %d %v", code1, res)
	}
	result := res["result"].(map[string]interface{})
	execID := res["execution_id"].(string)
	if result["reference"] != execID || result["total"] != float64(3) || len(result["listing_ids"].([]interface{})) != 3 || result["vertical"] != "HOTEL" {
		t.Fatalf("result %v", result)
	}
	if q := f.search.queries[0]; q.Vertical != domain.VerticalHotel || q.Place != "Nairobi" {
		t.Fatalf("the existing search handler ran with the step's vertical and the mission's place: %+v", q)
	}
	var status, action, payload, runtimeStatus, errCode string
	var attempt int
	if err := f.db.DB.QueryRowContext(f.ctx, `SELECT status, action_type, result_payload::text, runtime_status, error_code, attempt FROM mission_step_executions WHERE id = $1`, execID).
		Scan(&status, &action, &payload, &runtimeStatus, &errCode, &attempt); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || action != "SEARCH_LISTINGS" || runtimeStatus != "SUCCESS" || errCode != "" || attempt != 1 || !strings.Contains(payload, `"l-1"`) || strings.Contains(payload, "Title") {
		t.Fatalf("durable record: %s %s %s %s attempt=%d %s", status, action, runtimeStatus, errCode, attempt, payload)
	}
	var stepStatus, actionRef, resultRef string
	if err := f.db.DB.QueryRowContext(f.ctx, `SELECT status, action_ref, result_ref FROM mission_steps WHERE id = $1`, stay).Scan(&stepStatus, &actionRef, &resultRef); err != nil {
		t.Fatal(err)
	}
	if stepStatus != "completed" || actionRef != execID || resultRef != execID {
		t.Fatalf("step %s %s %s", stepStatus, actionRef, resultRef)
	}
	if mstatus := f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'draft'`, mission); mstatus != 1 {
		t.Fatal("running a step neither activates nor completes the mission")
	}
	if f.n(`SELECT COUNT(*) FROM mission_steps WHERE mission_id = $1 AND status <> 'pending'`, mission) != 1 {
		t.Fatal("only the requested step ran")
	}
	// Credits: the runtime reserved and consumed once; the mission layer added nothing.
	if f.credits.reserved != 1 || f.credits.consumed != 1 || f.credits.released != 0 || f.credits.balance != 98 {
		t.Fatalf("credits reserved=%d consumed=%d released=%d balance=%d", f.credits.reserved, f.credits.consumed, f.credits.released, f.credits.balance)
	}

	// Idempotency: the same step and key is the same execution, nothing runs again.
	code2, again := f.exec(xAlice, mission, stay, "Idempotency-Key", "k-1")
	if code2 != http.StatusOK || again["replayed"] != true || again["execution_id"] != execID {
		t.Fatalf("replay = %d %v", code2, again)
	}
	if f.search.count() != 1 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, stay) != 1 || f.credits.reserved != 1 {
		t.Fatal("one actual execution, one charge")
	}
	// A completed step is never run again.
	if c, b := f.exec(xAlice, mission, stay, "Idempotency-Key", "k-2"); c != http.StatusConflict || xErr(b) != "STEP_NOT_RUNNABLE" {
		t.Fatalf("completed step = %d %v", c, b)
	}
	if c, b := f.exec(xAlice, mission, stay); c != http.StatusConflict || xErr(b) != "STEP_NOT_RUNNABLE" {
		t.Fatalf("completed step, no key = %d %v", c, b)
	}

	// The steps list now shows the result.
	_, list = f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", "")
	for _, raw := range list["items"].([]interface{}) {
		s := raw.(map[string]interface{})
		if s["id"] == stay {
			e := s["execution"].(map[string]interface{})
			if s["status"] != "completed" || s["can_execute"] != false || e["status"] != "completed" || e["result"].(map[string]interface{})["total"] != float64(3) {
				t.Fatalf("completed step in the list: %v", s)
			}
		}
	}

	// Security.
	if c, b := f.exec(xBob, mission, transport); c != http.StatusForbidden || xErr(b) != "FORBIDDEN" {
		t.Fatalf("User B on User A's step = %d %v", c, b)
	}
	if c, _ := f.exec("", mission, transport); c != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d", c)
	}
	bobMission, _ := f.plan(xBob, "Voyage à Goma")
	if c, b := f.exec(xBob, bobMission, transport); c != http.StatusNotFound || xErr(b) != "NOT_FOUND" {
		t.Fatalf("a step of another mission = %d %v", c, b)
	}
	const unknown = "d0e50000-0000-4000-8000-0000000000ff"
	if c, _ := f.exec(xAlice, unknown, transport); c != http.StatusNotFound {
		t.Fatalf("unknown mission = %d", c)
	}
	if c, _ := f.exec(xAlice, mission, unknown); c != http.StatusNotFound {
		t.Fatalf("unknown step = %d", c)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, transport) != 0 || f.search.count() != 1 {
		t.Fatal("a refused request runs and records nothing")
	}

	// A step with no executable action behind it is a controlled refusal, not a guess.
	if c, b := f.exec(xAlice, mission, activities); c != http.StatusUnprocessableEntity || xErr(b) != "STEP_NOT_EXECUTABLE" {
		t.Fatalf("not executable = %d %v", c, b)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, activities) != 0 || f.n(`SELECT COUNT(*) FROM mission_steps WHERE id = $1 AND status = 'pending'`, activities) != 1 {
		t.Fatal("it left the step as it was")
	}

	// Failure is recorded; the step is not stuck; the credits are released.
	f.search.set(errors.New("opensearch: connection refused 10.0.0.9:9200 password=s3cret"))
	c3, fail := f.exec(xAlice, mission, transport, "Idempotency-Key", "t-1")
	if c3 != http.StatusCreated || fail["status"] != "failed" || fail["step_status"] != "failed" || fail["error"].(map[string]interface{})["code"] != "ACTION_FAILED" {
		t.Fatalf("failure = %d %v", c3, fail)
	}
	var failCode, failRuntime, failPayload string
	if err := f.db.DB.QueryRowContext(f.ctx, `SELECT error_code, runtime_status, result_payload::text FROM mission_step_executions WHERE id = $1`, fail["execution_id"]).Scan(&failCode, &failRuntime, &failPayload); err != nil {
		t.Fatal(err)
	}
	if failCode != "ACTION_FAILED" || failRuntime != "FAILED" || strings.Contains(failPayload, "s3cret") {
		t.Fatalf("failure record %s %s %s", failCode, failRuntime, failPayload)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE error_code LIKE '%s3cret%' OR error_code LIKE '%10.0.0.9%'`) != 0 {
		t.Fatal("no internal detail is kept")
	}
	if f.credits.reserved != 2 || f.credits.released != 1 || f.credits.consumed != 1 || f.credits.balance != 98 {
		t.Fatalf("a failed action keeps the credits: reserved=%d released=%d consumed=%d balance=%d", f.credits.reserved, f.credits.released, f.credits.consumed, f.credits.balance)
	}
	// The same key replays the failure.
	if c, b := f.exec(xAlice, mission, transport, "Idempotency-Key", "t-1"); c != http.StatusOK || b["replayed"] != true || b["status"] != "failed" {
		t.Fatalf("replayed failure = %d %v", c, b)
	}
	// Retry once the search works: a second attempt, completed; charged once for the success.
	f.search.set(nil)
	c4, retry := f.exec(xAlice, mission, transport, "Idempotency-Key", "t-2")
	if c4 != http.StatusCreated || retry["status"] != "completed" || retry["attempt"] != float64(2) || retry["step_status"] != "completed" {
		t.Fatalf("retry = %d %v", c4, retry)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, transport) != 2 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1 AND status = 'failed'`, transport) != 1 {
		t.Fatal("the failed attempt stays as history")
	}
	if f.credits.reserved != 3 || f.credits.consumed != 2 || f.credits.released != 1 || f.credits.balance != 96 || len(f.credits.open) != 0 {
		t.Fatalf("no double charge: reserved=%d consumed=%d released=%d balance=%d", f.credits.reserved, f.credits.consumed, f.credits.released, f.credits.balance)
	}
	// The latest attempt is the one the list shows.
	_, list = f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", "")
	for _, raw := range list["items"].([]interface{}) {
		if s := raw.(map[string]interface{}); s["id"] == transport && s["execution"].(map[string]interface{})["attempt"] != float64(2) {
			t.Fatalf("latest attempt in the list: %v", s["execution"])
		}
	}

	// A step that has run cannot be replaced away with its plan.
	if _, m := f.call(xAlice, "PATCH", "/api/v1/missions/"+mission, `{"type":"event"}`); m["type"] != "EVENT" {
		t.Fatalf("retype %v", m)
	}
	if c, b := f.call(xAlice, "POST", "/api/v1/missions/"+mission+"/plan", `{"replace": true}`); c != http.StatusConflict || xErr(b) != "PLAN_IN_PROGRESS" {
		t.Fatalf("replacing a plan with finished steps = %d %v", c, b)
	}

	// A paused mission runs nothing.
	pausedMission, pausedSteps := f.plan(xAlice, "Un séjour de 3 jours")
	f.call(xAlice, "PATCH", "/api/v1/missions/"+pausedMission, `{"status":"active"}`)
	f.call(xAlice, "PATCH", "/api/v1/missions/"+pausedMission, `{"status":"paused"}`)
	if c, b := f.exec(xAlice, pausedMission, pausedSteps["FIND_ACCOMMODATION"]); c != http.StatusConflict || xErr(b) != "MISSION_NOT_RUNNABLE" {
		t.Fatalf("paused mission = %d %v", c, b)
	}
	f.call(xAlice, "PATCH", "/api/v1/missions/"+pausedMission, `{"status":"active"}`)
	if c, b := f.exec(xAlice, pausedMission, pausedSteps["FIND_ACCOMMODATION"]); c != http.StatusCreated || b["status"] != "completed" {
		t.Fatalf("an active mission runs its steps = %d %v", c, b)
	}
	if f.n(`SELECT COUNT(*) FROM missions WHERE id = $1 AND status = 'active'`, pausedMission) != 1 {
		t.Fatal("the mission is not completed when its steps are")
	}
}

func TestMissionExecution_ConcurrentRequestsRunOnce(t *testing.T) {
	f := newExecFixture(t)
	f.search.delay = 80 * time.Millisecond
	mission, steps := f.plan(xAlice, "Je veux organiser mon déménagement")
	step := steps["FIND_ACCOMMODATION"]

	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses := map[int]int{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _ := f.exec(xAlice, mission, step, "Idempotency-Key", fmt.Sprintf("key-%d", i))
			mu.Lock()
			statuses[c]++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	if statuses[http.StatusCreated] != 1 || statuses[http.StatusCreated]+statuses[http.StatusConflict] != 10 {
		t.Fatalf("one run, the others refused: %v", statuses)
	}
	if f.search.count() != 1 || f.n(`SELECT COUNT(*) FROM mission_step_executions WHERE step_id = $1`, step) != 1 || f.credits.reserved != 1 || f.credits.consumed != 1 {
		t.Fatalf("one search, one execution, one charge: searches=%d reserved=%d", f.search.count(), f.credits.reserved)
	}
	if q := f.search.queries[0]; q.Vertical != domain.VerticalRealEstate {
		t.Fatalf("moving house looks for a place to live: %+v", q)
	}
}

func TestMissionExecution_RacingRetriesWithOneKeyShareOneExecution(t *testing.T) {
	f := newExecFixture(t)
	f.search.delay = 60 * time.Millisecond
	mission, steps := f.plan(xAlice, "Un séjour de 3 jours")
	step := steps["FIND_ACCOMMODATION"]
	var wg sync.WaitGroup
	var mu sync.Mutex
	statuses, ids := map[int]int{}, map[string]bool{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, b := f.exec(xAlice, mission, step, "Idempotency-Key", "same-key")
			mu.Lock()
			statuses[c]++
			if id, ok := b["execution_id"].(string); ok {
				ids[id] = true
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	if statuses[http.StatusCreated] != 1 || statuses[http.StatusCreated]+statuses[http.StatusOK] != 8 || len(ids) != 1 {
		t.Fatalf("one creation, the rest replays of it: %v %v", statuses, ids)
	}
	if f.search.count() != 1 || f.credits.reserved != 1 {
		t.Fatalf("searches=%d reserved=%d", f.search.count(), f.credits.reserved)
	}
}

func TestMissionExecution_AnAbandonedAttemptIsGivenUpOn(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.plan(xAlice, "Un séjour de 3 jours")
	step := steps["FIND_ACCOMMODATION"]
	// An attempt that began ten minutes ago and never ended (the process died).
	if _, err := f.db.DB.ExecContext(f.ctx, `UPDATE mission_steps SET status = 'in_progress' WHERE id = $1`, step); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.DB.ExecContext(f.ctx, `INSERT INTO mission_step_executions (step_id, mission_id, attempt, status, action_type, started_at) VALUES ($1, $2, 1, 'in_progress', 'SEARCH_LISTINGS', NOW() - INTERVAL '10 minutes')`, step, mission); err != nil {
		t.Fatal(err)
	}
	_, list := f.call(xAlice, "GET", "/api/v1/missions/"+mission+"/steps", "")
	for _, raw := range list["items"].([]interface{}) {
		if s := raw.(map[string]interface{}); s["id"] == step && (s["stale"] != true || s["can_execute"] != true) {
			t.Fatalf("a silent attempt shows as stale and runnable: %v", s)
		}
	}
	c, res := f.exec(xAlice, mission, step)
	if c != http.StatusCreated || res["status"] != "completed" || res["attempt"] != float64(2) {
		t.Fatalf("run again = %d %v", c, res)
	}
	var oldStatus, oldCode string
	if err := f.db.DB.QueryRowContext(f.ctx, `SELECT status, error_code FROM mission_step_executions WHERE step_id = $1 AND attempt = 1`, step).Scan(&oldStatus, &oldCode); err != nil {
		t.Fatal(err)
	}
	if oldStatus != "failed" || oldCode != "ABANDONED" {
		t.Fatalf("the silent attempt is recorded as abandoned: %s %s", oldStatus, oldCode)
	}
	// A recent attempt in progress is respected.
	other, otherSteps := f.plan(xAlice, "Voyage à Goma")
	if _, err := f.db.DB.ExecContext(f.ctx, `UPDATE mission_steps SET status = 'in_progress' WHERE id = $1`, otherSteps["FIND_ACCOMMODATION"]); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.DB.ExecContext(f.ctx, `INSERT INTO mission_step_executions (step_id, mission_id, attempt, status, action_type) VALUES ($1, $2, 1, 'in_progress', 'SEARCH_LISTINGS')`, otherSteps["FIND_ACCOMMODATION"], other); err != nil {
		t.Fatal(err)
	}
	if c, b := f.exec(xAlice, other, otherSteps["FIND_ACCOMMODATION"]); c != http.StatusConflict || xErr(b) != "STEP_IN_PROGRESS" {
		t.Fatalf("a running attempt = %d %v", c, b)
	}
}

func TestMissionExecution_TheDatabaseRefusesWhatTheDomainWould(t *testing.T) {
	f := newExecFixture(t)
	mission, steps := f.plan(xAlice, "Un séjour de 3 jours")
	step := steps["FIND_ACCOMMODATION"]
	if c, _ := f.exec(xAlice, mission, step, "Idempotency-Key", "k-1"); c != http.StatusCreated {
		t.Fatal("setup")
	}
	ins := func(cols, vals string) string {
		return `INSERT INTO mission_step_executions (step_id, mission_id, action_type, ` + cols + `) VALUES ('` + step + `', '` + mission + `', 'SEARCH_LISTINGS', ` + vals + `)`
	}
	for name, q := range map[string]string{
		"attempt zero":              ins("attempt", "0"),
		"repeated attempt":          ins("attempt", "1"),
		"unknown status":            ins("attempt, status", "5, 'done'"),
		"failed without a code":     ins("attempt, status, completed_at", "6, 'failed', NOW()"),
		"in progress but finished":  ins("attempt, status, completed_at", "7, 'in_progress', NOW()"),
		"finished but not complete": ins("attempt, status", "8, 'completed'"),
		"repeated key":              ins("attempt, idempotency_key, status, completed_at", "9, 'k-1', 'completed', NOW()"),
		"unknown step":              `INSERT INTO mission_step_executions (step_id, mission_id, attempt, action_type) VALUES ('d0e50000-0000-4000-8000-0000000000ee', '` + mission + `', 1, 'X')`,
	} {
		if _, err := f.db.DB.ExecContext(f.ctx, q); err == nil {
			t.Errorf("the database accepted: %s", name)
		}
	}
	// Two attempts in progress for one step cannot exist.
	if _, err := f.db.DB.ExecContext(f.ctx, ins("attempt", "2")); err != nil {
		t.Fatalf("one attempt in progress is fine: %v", err)
	}
	if _, err := f.db.DB.ExecContext(f.ctx, ins("attempt", "3")); err == nil {
		t.Error("a second attempt in progress for the same step was accepted")
	}
	// Executions go with their step's mission, and with its owner.
	if _, err := f.db.DB.ExecContext(f.ctx, `DELETE FROM users WHERE id = $1`, xAlice); err != nil {
		t.Fatal(err)
	}
	if f.n(`SELECT COUNT(*) FROM mission_step_executions`) != 0 {
		t.Fatal("executions outlive their owner")
	}
}

func TestMissionExecution_MigrationDownAndUp(t *testing.T) {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "000029_mission_step_executions.sql"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	down := content[strings.Index(content, "-- +goose Down")+len("-- +goose Down"):]
	if _, err := db.DB.Exec(down); err != nil {
		t.Fatalf("down: %v", err)
	}
	var n int
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mission_step_executions'`).Scan(&n)
	if n != 0 {
		t.Fatal("down must drop the table")
	}
	if _, err := db.DB.Exec(extractUpSection(content)); err != nil {
		t.Fatalf("up again: %v", err)
	}
	_ = db.DB.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'mission_step_executions'`).Scan(&n)
	if n != 1 {
		t.Fatal("up must create the table again")
	}
}
