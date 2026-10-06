package integration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Organiser's missions and steps on the real schema (migration 000027): ownership, idempotency, ordered step
 * positions under concurrency, the lifecycle and the database's own constraints.
 */

const (
	mAlice = "d0e50000-0000-4000-8000-0000000000a1"
	mBob   = "d0e50000-0000-4000-8000-0000000000a2"
)

func TestMissions_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	t.Cleanup(db.Close)
	ctx := context.Background()
	sellerUser(t, db, ctx, mAlice, "+243810000071")
	sellerUser(t, db, ctx, mBob, "+243810000072")
	repo := postgres.NewMissionRepository(db.DB)
	uc := application.NewMissionUseCases(repo)

	// A draft is created with what was understood from the sentence, and read back intact.
	m, replay, err := uc.Create(ctx, mAlice, application.CreateMissionInput{
		Objective: "Je veux organiser un anniversaire pour 40 personnes à Kinshasa", StartDate: time.Date(2027, 3, 5, 0, 0, 0, 0, time.UTC).Unix(),
		BudgetMinor: 250000, Currency: "usd", Preferences: map[string]string{"musique": "rumba"},
	}, "key-1")
	if err != nil || replay {
		t.Fatalf("create: %v replay=%v", err, replay)
	}
	got, err := uc.Get(ctx, mAlice, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.MissionDraft || got.Type != domain.MissionTypeEvent || got.Title != "Anniversaire" || got.Location != "Kinshasa" ||
		got.Participants != 40 || got.BudgetMinor != 250000 || got.Currency != "USD" || got.Preferences["musique"] != "rumba" {
		t.Fatalf("read back %+v", got)
	}
	if got.StartDate != time.Date(2027, 3, 5, 0, 0, 0, 0, time.UTC).Unix() || got.EndDate != 0 {
		t.Fatalf("dates: start %d end %d", got.StartDate, got.EndDate)
	}

	// Ownership: Bob never reaches Alice's mission; an unknown or malformed id is not found.
	if _, err := uc.Get(ctx, mBob, m.ID); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Bob reading Alice's = %v", err)
	}
	for _, id := range []string{"d0e50000-0000-4000-8000-0000000000ff", "not-a-uuid"} {
		if _, err := uc.Get(ctx, mAlice, id); !errors.Is(err, domain.ErrMissionNotFound) {
			t.Fatalf("Get(%q) = %v, want not found", id, err)
		}
	}

	// Idempotency: the same key returns the first mission, once, even when two retries race.
	again, replay, err := uc.Create(ctx, mAlice, application.CreateMissionInput{Objective: "Je veux organiser un anniversaire pour 40 personnes à Kinshasa"}, "key-1")
	if err != nil || !replay || again.ID != m.ID {
		t.Fatalf("retry: %v replay=%v", err, replay)
	}
	if _, _, err := uc.Create(ctx, mAlice, application.CreateMissionInput{Objective: "Un voyage à Goma"}, "key-1"); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("same key, other objective = %v", err)
	}
	var wg sync.WaitGroup
	ids := make([]string, 6)
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, _, err := uc.Create(ctx, mAlice, application.CreateMissionInput{Objective: "Un voyage à Goma"}, "race")
			if err != nil {
				t.Errorf("racing create %d: %v", i, err)
				return
			}
			ids[i] = r.ID
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("racing retries must all return one mission: %v", ids)
		}
	}
	if _, _, err := uc.Create(ctx, mBob, application.CreateMissionInput{Objective: "Un voyage à Goma"}, "race"); err != nil {
		t.Fatalf("keys are per owner: %v", err)
	}

	// Listing: own missions only, newest change first, filtered by status, with the total.
	items, total, err := uc.List(ctx, mAlice, "", domain.Page{Limit: 10})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("Alice lists %d of %d: %v", len(items), total, err)
	}
	for _, it := range items {
		if it.OwnerID != mAlice {
			t.Fatalf("a foreign mission in Alice's list: %+v", it)
		}
	}
	active := domain.MissionActive
	if _, err := uc.Update(ctx, mAlice, m.ID, application.MissionPatch{Status: &active, Location: ptrStr("Gombe")}); err != nil {
		t.Fatal(err)
	}
	if items, total, _ := uc.List(ctx, mAlice, domain.MissionActive, domain.Page{Limit: 10}); total != 1 || len(items) != 1 || items[0].Location != "Gombe" {
		t.Fatalf("active list: %d %+v", total, items)
	}
	if items, total, _ := uc.List(ctx, mAlice, "", domain.Page{Limit: 1, Offset: 1}); total != 2 || len(items) != 1 {
		t.Fatalf("paging: %d of %d", len(items), total)
	}

	// Steps: positions run 1..n with no gaps or repeats, even when added at the same time.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := uc.AddStep(ctx, mAlice, m.ID, application.StepInput{Type: "VENUE", Title: "Une étape"}); err != nil {
				t.Errorf("add step: %v", err)
			}
		}()
	}
	wg.Wait()
	steps, err := uc.Steps(ctx, mAlice, m.ID)
	if err != nil || len(steps) != 4 {
		t.Fatalf("steps = %d, %v", len(steps), err)
	}
	for i, s := range steps {
		if s.Position != i+1 || s.Status != domain.StepPending {
			t.Fatalf("step %d: %+v", i, s)
		}
	}
	first, err := uc.AddStep(ctx, mAlice, m.ID, application.StepInput{Type: "CATERING", Title: "Traiteur", Description: "Buffet", ActionType: "search_listings"})
	if err != nil || first.Position != 5 || first.ActionType != domain.ActionSearchListings {
		t.Fatalf("fifth step: %v %+v", err, first)
	}
	for i := len(steps) + 1; i < domain.MissionMaxSteps; i++ {
		if _, err := uc.AddStep(ctx, mAlice, m.ID, application.StepInput{Type: "OTHER", Title: "Encore"}); err != nil {
			t.Fatalf("step %d: %v", i+1, err)
		}
	}
	if _, err := uc.AddStep(ctx, mAlice, m.ID, application.StepInput{Type: "OTHER", Title: "De trop"}); !errors.Is(err, domain.ErrMissionStepLimit) {
		t.Fatalf("over the limit = %v", err)
	}
	if err := repo.AddStep(ctx, &domain.MissionStep{ID: domain.NewID(), MissionID: "d0e50000-0000-4000-8000-0000000000ff", Type: "VENUE", Title: "Salle", Status: domain.StepPending}, 30); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("a step of an unknown mission = %v", err)
	}

	// The database refuses what the domain would: it is the last line of defence.
	for name, q := range map[string]string{
		"status":   `INSERT INTO missions (owner_id, mission_type, title, objective, status) VALUES ('` + mAlice + `', 'EVENT', 'T', 'O', 'frozen')`,
		"type":     `INSERT INTO missions (owner_id, mission_type, title, objective) VALUES ('` + mAlice + `', 'not a code', 'T', 'O')`,
		"budget":   `INSERT INTO missions (owner_id, mission_type, title, objective, budget_minor) VALUES ('` + mAlice + `', 'EVENT', 'T', 'O', 100)`,
		"dates":    `INSERT INTO missions (owner_id, mission_type, title, objective, start_date, end_date) VALUES ('` + mAlice + `', 'EVENT', 'T', 'O', '2027-02-01', '2027-01-01')`,
		"position": `INSERT INTO mission_steps (mission_id, step_type, title, position) VALUES ('` + m.ID + `', 'VENUE', 'T', 0)`,
		"repeat":   `INSERT INTO mission_steps (mission_id, step_type, title, position) VALUES ('` + m.ID + `', 'VENUE', 'T', 1)`,
		"owner":    `INSERT INTO missions (owner_id, mission_type, title, objective) VALUES ('d0e50000-0000-4000-8000-0000000000ee', 'EVENT', 'T', 'O')`,
	} {
		if _, err := db.DB.ExecContext(ctx, q); err == nil {
			t.Errorf("the database accepted an invalid %s", name)
		}
	}

	// A mission goes with its owner, and its steps with it.
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, mAlice); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := db.DB.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM missions WHERE owner_id = $1) + (SELECT COUNT(*) FROM mission_steps WHERE mission_id = $2)`, mAlice, m.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("missions and steps outlive their owner: %d, %v", left, err)
	}
}

func ptrStr(s string) *string { return &s }
