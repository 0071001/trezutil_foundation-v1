package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeMissionRepo struct {
	missions map[string]*domain.Mission
	steps    map[string][]domain.MissionStep
}

func newFakeMissionRepo() *fakeMissionRepo {
	return &fakeMissionRepo{missions: map[string]*domain.Mission{}, steps: map[string][]domain.MissionStep{}}
}

func (f *fakeMissionRepo) Create(_ context.Context, m *domain.Mission) error {
	for _, e := range f.missions {
		if m.IdempotencyKey != "" && e.OwnerID == m.OwnerID && e.IdempotencyKey == m.IdempotencyKey {
			return domain.ErrIdempotencyConflict
		}
	}
	c := *m
	f.missions[m.ID] = &c
	return nil
}
func (f *fakeMissionRepo) FindByIdempotencyKey(_ context.Context, ownerID, key string) (*domain.Mission, error) {
	for _, m := range f.missions {
		if m.OwnerID == ownerID && m.IdempotencyKey == key {
			c := *m
			return &c, nil
		}
	}
	return nil, domain.ErrMissionNotFound
}
func (f *fakeMissionRepo) Get(_ context.Context, id string) (*domain.Mission, error) {
	if m, ok := f.missions[id]; ok {
		c := *m
		return &c, nil
	}
	return nil, domain.ErrMissionNotFound
}
func (f *fakeMissionRepo) ListByOwner(_ context.Context, ownerID string, status domain.MissionStatus, page domain.Page) ([]domain.Mission, int, error) {
	var out []domain.Mission
	for _, m := range f.missions {
		if m.OwnerID == ownerID && (status == "" || m.Status == status) {
			out = append(out, *m)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	if page.Offset >= len(out) {
		return []domain.Mission{}, total, nil
	}
	out = out[page.Offset:]
	if page.Limit > 0 && page.Limit < len(out) {
		out = out[:page.Limit]
	}
	return out, total, nil
}
func (f *fakeMissionRepo) Update(_ context.Context, m *domain.Mission) error {
	if _, ok := f.missions[m.ID]; !ok {
		return domain.ErrMissionNotFound
	}
	c := *m
	f.missions[m.ID] = &c
	return nil
}
func (f *fakeMissionRepo) AddStep(_ context.Context, s *domain.MissionStep, maxSteps int) error {
	if _, ok := f.missions[s.MissionID]; !ok {
		return domain.ErrMissionNotFound
	}
	if maxSteps > 0 && len(f.steps[s.MissionID]) >= maxSteps {
		return domain.ErrMissionStepLimit
	}
	s.Position = len(f.steps[s.MissionID]) + 1
	f.steps[s.MissionID] = append(f.steps[s.MissionID], *s)
	return nil
}
func (f *fakeMissionRepo) Steps(_ context.Context, missionID string) ([]domain.MissionStep, error) {
	return append([]domain.MissionStep{}, f.steps[missionID]...), nil
}

var missionClock = time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

func newMissionUseCases() (*MissionUseCases, *fakeMissionRepo) {
	repo := newFakeMissionRepo()
	return NewMissionUseCases(repo).WithClock(func() time.Time { return missionClock }), repo
}

const birthday = "Je veux organiser un anniversaire pour 40 personnes à Kinshasa."

func TestMissions_CreateUnderstandsTheSentenceAndStaysADraft(t *testing.T) {
	uc, _ := newMissionUseCases()
	m, replay, err := uc.Create(context.Background(), "alice", CreateMissionInput{Objective: birthday}, "")
	if err != nil || replay {
		t.Fatalf("Create = %v, replay %v", err, replay)
	}
	if m.Type != domain.MissionTypeEvent || m.Title != "Anniversaire" || m.Participants != 40 || m.Location != "Kinshasa" {
		t.Fatalf("understood %+v", m)
	}
	if m.Status != domain.MissionDraft || m.OwnerID != "alice" || m.Objective != strings.TrimSpace(birthday) {
		t.Fatalf("a created mission is a draft of its owner: %+v", m)
	}
}

func TestMissions_GivenFieldsWinOverTheReading(t *testing.T) {
	uc, _ := newMissionUseCases()
	m, _, err := uc.Create(context.Background(), "alice", CreateMissionInput{
		Objective: birthday, Type: "travel", Title: "Escapade", Location: "Lubumbashi", Participants: 12, BudgetMinor: 250000, Currency: "usd",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Type != domain.MissionTypeTravel || m.Title != "Escapade" || m.Location != "Lubumbashi" || m.Participants != 12 || m.Currency != "USD" {
		t.Fatalf("given fields must win: %+v", m)
	}
}

func TestMissions_CreateRejectsWhatIsInvalid(t *testing.T) {
	uc, repo := newMissionUseCases()
	for name, in := range map[string]CreateMissionInput{
		"empty objective": {Objective: "  "},
		"unknown type":    {Objective: birthday, Type: "wedding"},
		"budget no money": {Objective: birthday, BudgetMinor: 100},
	} {
		if _, _, err := uc.Create(context.Background(), "alice", in, ""); !errors.Is(err, domain.ErrMissionInvalid) {
			t.Errorf("%s: err = %v, want ErrMissionInvalid", name, err)
		}
	}
	if _, _, err := uc.Create(context.Background(), "", CreateMissionInput{Objective: birthday}, ""); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Errorf("no actor: err = %v", err)
	}
	if len(repo.missions) != 0 {
		t.Fatal("nothing invalid may be stored")
	}
}

func TestMissions_IdempotentCreate(t *testing.T) {
	uc, repo := newMissionUseCases()
	first, replay, err := uc.Create(context.Background(), "alice", CreateMissionInput{Objective: birthday}, "key-1")
	if err != nil || replay {
		t.Fatalf("first: %v %v", err, replay)
	}
	again, replay, err := uc.Create(context.Background(), "alice", CreateMissionInput{Objective: birthday}, "key-1")
	if err != nil || !replay || again.ID != first.ID || len(repo.missions) != 1 {
		t.Fatalf("a retry returns the first mission: %v replay=%v", err, replay)
	}
	if _, _, err := uc.Create(context.Background(), "alice", CreateMissionInput{Objective: "Un voyage à Goma"}, "key-1"); !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Fatalf("same key, other objective = %v, want ErrIdempotencyConflict", err)
	}
	other, replay, err := uc.Create(context.Background(), "bob", CreateMissionInput{Objective: birthday}, "key-1")
	if err != nil || replay || other.ID == first.ID {
		t.Fatalf("keys are per owner: %v replay=%v", err, replay)
	}
}

func TestMissions_OwnerReadsHisOwnAndNobodyElses(t *testing.T) {
	uc, _ := newMissionUseCases()
	m, _, _ := uc.Create(context.Background(), "alice", CreateMissionInput{Objective: birthday}, "")
	if got, err := uc.Get(context.Background(), "alice", m.ID); err != nil || got.ID != m.ID {
		t.Fatalf("Get own = %v", err)
	}
	if _, err := uc.Get(context.Background(), "bob", m.ID); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Get other's = %v, want ErrMissionForbidden", err)
	}
	if _, err := uc.Get(context.Background(), "alice", "missing"); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("Get unknown = %v, want ErrMissionNotFound", err)
	}
	if _, err := uc.Update(context.Background(), "bob", m.ID, MissionPatch{Title: ptr("Mine now")}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Update other's = %v", err)
	}
	if _, err := uc.AddStep(context.Background(), "bob", m.ID, StepInput{Type: "VENUE", Title: "Salle"}); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("AddStep other's = %v", err)
	}
	if _, err := uc.Steps(context.Background(), "bob", m.ID); !errors.Is(err, domain.ErrMissionForbidden) {
		t.Fatalf("Steps other's = %v", err)
	}
	items, total, err := uc.List(context.Background(), "bob", "", domain.Page{Limit: 20})
	if err != nil || total != 0 || len(items) != 0 {
		t.Fatalf("List for bob = %v %d %d", err, total, len(items))
	}
	if _, _, err := uc.List(context.Background(), "alice", "frozen", domain.Page{Limit: 20}); !errors.Is(err, domain.ErrMissionInvalid) {
		t.Fatalf("unknown status filter = %v", err)
	}
}

func ptr[T any](v T) *T { return &v }

func TestMissions_UpdateFollowsTheLifecycle(t *testing.T) {
	uc, _ := newMissionUseCases()
	ctx := context.Background()
	m, _, _ := uc.Create(ctx, "alice", CreateMissionInput{Objective: birthday}, "")

	got, err := uc.Update(ctx, "alice", m.ID, MissionPatch{Location: ptr("Gombe"), Participants: ptr(55), StartDate: ptr(int64(86400))})
	if err != nil || got.Location != "Gombe" || got.Participants != 55 || got.StartDate != 86400 {
		t.Fatalf("edit: %v %+v", err, got)
	}
	if _, err := uc.Update(ctx, "alice", m.ID, MissionPatch{EndDate: ptr(int64(100)), StartDate: ptr(int64(200))}); !errors.Is(err, domain.ErrMissionInvalid) {
		t.Fatalf("end before start = %v", err)
	}
	if _, err := uc.Update(ctx, "alice", m.ID, MissionPatch{Status: ptr(domain.MissionCompleted)}); !errors.Is(err, domain.ErrInvalidMissionTransition) {
		t.Fatalf("draft -> completed = %v", err)
	}
	if got, err := uc.Get(ctx, "alice", m.ID); err != nil || got.Status != domain.MissionDraft {
		t.Fatalf("a refused transition changes nothing: %v %v", err, got.Status)
	}
	for _, next := range []domain.MissionStatus{domain.MissionActive, domain.MissionPaused, domain.MissionActive, domain.MissionCancelled} {
		if got, err := uc.Update(ctx, "alice", m.ID, MissionPatch{Status: ptr(next)}); err != nil || got.Status != next {
			t.Fatalf("-> %s: %v", next, err)
		}
	}
	if _, err := uc.Update(ctx, "alice", m.ID, MissionPatch{Title: ptr("Encore")}); !errors.Is(err, domain.ErrMissionClosed) {
		t.Fatalf("a cancelled mission is read-only = %v", err)
	}
}

func TestMissions_StepsAreOrderedAndRecordedNotRun(t *testing.T) {
	uc, _ := newMissionUseCases()
	ctx := context.Background()
	m, _, _ := uc.Create(ctx, "alice", CreateMissionInput{Objective: birthday}, "")
	for i, title := range []string{"Trouver une salle", "Choisir un traiteur", "Décoration"} {
		s, err := uc.AddStep(ctx, "alice", m.ID, StepInput{Type: "venue", Title: title, ActionType: "search_listings"})
		if err != nil || s.Position != i+1 || s.Status != domain.StepPending || s.ActionType != domain.ActionSearchListings {
			t.Fatalf("step %d: %v %+v", i, err, s)
		}
		if s.ActionRef != "" || s.ResultRef != "" {
			t.Fatal("a recorded step has not run: no action, no result")
		}
	}
	steps, err := uc.Steps(ctx, "alice", m.ID)
	if err != nil || len(steps) != 3 || steps[0].Title != "Trouver une salle" || steps[2].Position != 3 {
		t.Fatalf("Steps = %v %+v", err, steps)
	}
	if _, err := uc.AddStep(ctx, "alice", m.ID, StepInput{Type: "VENUE", Title: "Salle", ActionType: "do something"}); !errors.Is(err, domain.ErrMissionInvalid) {
		t.Fatalf("a malformed action type = %v", err)
	}
	if _, err := uc.AddStep(ctx, "alice", "missing", StepInput{Type: "VENUE", Title: "Salle"}); !errors.Is(err, domain.ErrMissionNotFound) {
		t.Fatalf("unknown mission = %v", err)
	}
}

func TestMissions_StepLimitAndClosedMission(t *testing.T) {
	uc, _ := newMissionUseCases()
	ctx := context.Background()
	m, _, _ := uc.Create(ctx, "alice", CreateMissionInput{Objective: birthday}, "")
	for i := 0; i < domain.MissionMaxSteps; i++ {
		if _, err := uc.AddStep(ctx, "alice", m.ID, StepInput{Type: "OTHER", Title: "Une étape"}); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if _, err := uc.AddStep(ctx, "alice", m.ID, StepInput{Type: "OTHER", Title: "De trop"}); !errors.Is(err, domain.ErrMissionStepLimit) {
		t.Fatalf("over the limit = %v", err)
	}
	closed, _, _ := uc.Create(ctx, "alice", CreateMissionInput{Objective: "Un voyage à Goma"}, "")
	_, _ = uc.Update(ctx, "alice", closed.ID, MissionPatch{Status: ptr(domain.MissionCancelled)})
	if _, err := uc.AddStep(ctx, "alice", closed.ID, StepInput{Type: "VENUE", Title: "Salle"}); !errors.Is(err, domain.ErrMissionClosed) {
		t.Fatalf("closed mission = %v", err)
	}
}

func TestUnderstandObjective(t *testing.T) {
	cases := []struct {
		text         string
		typ          domain.MissionType
		title, place string
		people       int
	}{
		{"Je veux organiser un anniversaire pour 40 personnes à Kinshasa.", domain.MissionTypeEvent, "Anniversaire", "Kinshasa", 40},
		{"I want to organise a birthday for 25 guests in Lubumbashi", domain.MissionTypeEvent, "Birthday", "Lubumbashi", 25},
		{"Organiser un mariage à Goma pour 120 invités", domain.MissionTypeEvent, "Mariage", "Goma", 120},
		{"Je prépare mon déménagement vers Matadi", domain.MissionTypeMove, "Déménagement", "Matadi", 0},
		{"Un séjour de 3 jours à Kisangani", domain.MissionTypeStay, "Séjour", "Kisangani", 0},
		{"Je pars en voyage à Dubaï avec 2 personnes", domain.MissionTypeTravel, "Voyage", "Dubaï", 2},
		{"Un déplacement professionnel à Paris", domain.MissionTypeBusinessTrip, "Déplacement professionnel", "Paris", 0},
		{"Organiser une fête en Octobre", domain.MissionTypeEvent, "Fête", "", 0},
		{"Je veux réorganiser mon bureau", domain.MissionTypeOther, "Réorganiser mon bureau", "", 0},
	}
	for _, c := range cases {
		got := UnderstandObjective(c.text)
		if got.Type != c.typ || got.Title != c.title || got.Location != c.place || got.Participants != c.people {
			t.Errorf("%q\n got  %+v\n want type=%s title=%q place=%q people=%d", c.text, got, c.typ, c.title, c.place, c.people)
		}
	}
	long := UnderstandObjective("Quelque chose de très particulier que personne ne sait ranger dans une catégorie connue de tout le monde ici")
	if long.Type != domain.MissionTypeOther || len([]rune(long.Title)) > 60 {
		t.Fatalf("an unclear sentence is OTHER with a short title: %+v", long)
	}
	if got := UnderstandObjective("   "); got.Title != "Mission" {
		t.Fatalf("empty input = %+v", got)
	}
}
