package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var missionNow = time.Date(2026, 10, 6, 9, 0, 0, 0, time.UTC)

func newTestMission(t *testing.T) *Mission {
	t.Helper()
	m, err := NewMission("user-1", MissionTypeEvent, "Anniversaire", "Organiser un anniversaire pour 40 personnes à Kinshasa", missionNow)
	if err != nil {
		t.Fatalf("NewMission: %v", err)
	}
	return m
}

func TestNewMission_IsADraftOwnedByItsCreator(t *testing.T) {
	m := newTestMission(t)
	if m.Status != MissionDraft {
		t.Fatalf("status = %s, want draft", m.Status)
	}
	if m.ID == "" || m.OwnerID != "user-1" || m.Type != MissionTypeEvent {
		t.Fatalf("unexpected mission %+v", m)
	}
	if m.CreatedAt != missionNow.Unix() || m.UpdatedAt != missionNow.Unix() {
		t.Fatalf("timestamps not set: %d %d", m.CreatedAt, m.UpdatedAt)
	}
	if !m.OwnedBy("user-1") || m.OwnedBy("user-2") || m.OwnedBy("") {
		t.Fatal("ownership must hold for the creator only")
	}
}

func TestMission_Validation(t *testing.T) {
	long := strings.Repeat("x", MissionObjectiveMax+1)
	cases := []struct {
		name   string
		mutate func(*Mission)
		want   string
	}{
		{"no owner", func(m *Mission) { m.OwnerID = " " }, "owner"},
		{"unknown type", func(m *Mission) { m.Type = "WEDDING_PLANNER" }, "type must be"},
		{"short title", func(m *Mission) { m.Title = "A" }, "title"},
		{"long objective", func(m *Mission) { m.Objective = long }, "objective"},
		{"short objective", func(m *Mission) { m.Objective = "ab" }, "objective"},
		{"end before start", func(m *Mission) { m.StartDate, m.EndDate = 2000, 1000 }, "end date"},
		{"negative participants", func(m *Mission) { m.Participants = -1 }, "participants"},
		{"too many participants", func(m *Mission) { m.Participants = MissionMaxParticipant + 1 }, "participants"},
		{"negative budget", func(m *Mission) { m.BudgetMinor = -5 }, "budget"},
		{"budget without currency", func(m *Mission) { m.BudgetMinor = 5000 }, "currency"},
		{"unknown status", func(m *Mission) { m.Status = "frozen" }, "status"},
		{"empty preference name", func(m *Mission) { m.Preferences = map[string]string{" ": "x"} }, "preference"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestMission(t)
			c.mutate(m)
			err := m.Validate()
			if !errors.Is(err, ErrMissionInvalid) || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Validate() = %v, want ErrMissionInvalid mentioning %q", err, c.want)
			}
		})
	}
	m := newTestMission(t)
	m.BudgetMinor, m.Currency, m.Participants, m.StartDate, m.EndDate = 500000, "USD", 40, 1000, 1000
	if err := m.Validate(); err != nil {
		t.Fatalf("a complete mission must validate: %v", err)
	}
}

func TestMissionStatus_Lifecycle(t *testing.T) {
	allowed := map[[2]MissionStatus]bool{
		{MissionDraft, MissionActive}: true, {MissionDraft, MissionCancelled}: true,
		{MissionActive, MissionPaused}: true, {MissionActive, MissionCompleted}: true, {MissionActive, MissionCancelled}: true,
		{MissionPaused, MissionActive}: true, {MissionPaused, MissionCancelled}: true,
	}
	all := []MissionStatus{MissionDraft, MissionActive, MissionPaused, MissionCompleted, MissionCancelled}
	for _, from := range all {
		for _, to := range all {
			if got, want := from.CanTransitionTo(to), allowed[[2]MissionStatus{from, to}]; got != want {
				t.Errorf("%s -> %s: got %v, want %v", from, to, got, want)
			}
		}
	}
	if !MissionCompleted.IsTerminal() || !MissionCancelled.IsTerminal() || MissionPaused.IsTerminal() {
		t.Fatal("completed and cancelled are terminal, paused is not")
	}
}

func TestMission_TransitionTo(t *testing.T) {
	m := newTestMission(t)
	later := missionNow.Add(time.Hour)
	if err := m.TransitionTo(MissionCompleted, later); !errors.Is(err, ErrInvalidMissionTransition) {
		t.Fatalf("draft -> completed = %v, want ErrInvalidMissionTransition", err)
	}
	if m.Status != MissionDraft || m.UpdatedAt != missionNow.Unix() {
		t.Fatal("a refused transition must change nothing")
	}
	if err := m.TransitionTo(MissionActive, later); err != nil || m.Status != MissionActive || m.UpdatedAt != later.Unix() {
		t.Fatalf("draft -> active failed: %v %+v", err, m)
	}
	if err := m.TransitionTo("unknown", later); !errors.Is(err, ErrInvalidMissionTransition) {
		t.Fatalf("unknown status = %v", err)
	}
}

func TestMission_NewStep_BelongsToTheMission(t *testing.T) {
	m := newTestMission(t)
	s, err := m.NewStep("venue", "Trouver une salle", "Pour 40 personnes", missionNow)
	if err != nil {
		t.Fatalf("NewStep: %v", err)
	}
	if s.MissionID != m.ID || s.Type != "VENUE" || s.Status != StepPending || s.ID == "" {
		t.Fatalf("unexpected step %+v", s)
	}
	if s.Position != 0 || s.ActionType != "" || s.ActionRef != "" || s.ResultRef != "" {
		t.Fatal("position is the repository's and a new step references no action yet")
	}
	other, _ := m.NewStep("CATERING", "Traiteur", "", missionNow)
	if other.ID == s.ID {
		t.Fatal("steps need distinct ids")
	}
}

func TestMission_NewStep_Validation(t *testing.T) {
	m := newTestMission(t)
	for name, c := range map[string][3]string{
		"type not a code":  {"not a code", "Salle", ""},
		"type too short":   {"V", "Salle", ""},
		"short title":      {"VENUE", "S", ""},
		"long description": {"VENUE", "Salle", strings.Repeat("d", MissionStepDescMax+1)},
	} {
		if _, err := m.NewStep(c[0], c[1], c[2], missionNow); !errors.Is(err, ErrMissionInvalid) {
			t.Errorf("%s: err = %v, want ErrMissionInvalid", name, err)
		}
	}
}

func TestMission_ClosedMissionTakesNoSteps(t *testing.T) {
	for _, closing := range []MissionStatus{MissionCancelled, MissionCompleted} {
		m := newTestMission(t)
		m.Status = closing
		if _, err := m.NewStep("VENUE", "Salle", "", missionNow); !errors.Is(err, ErrMissionClosed) {
			t.Errorf("%s: err = %v, want ErrMissionClosed", closing, err)
		}
	}
	m := newTestMission(t)
	m.Status = MissionPaused
	if _, err := m.NewStep("VENUE", "Salle", "", missionNow); err != nil {
		t.Fatalf("a paused mission still takes steps: %v", err)
	}
}

func TestMissionStep_Lifecycle(t *testing.T) {
	m := newTestMission(t)
	s, _ := m.NewStep("VENUE", "Salle", "", missionNow)
	if err := s.TransitionTo(StepCompleted, missionNow); !errors.Is(err, ErrInvalidStepTransition) {
		t.Fatalf("pending -> completed = %v", err)
	}
	for _, next := range []MissionStepStatus{StepInProgress, StepFailed, StepPending, StepSkipped, StepPending, StepInProgress, StepCompleted} {
		if err := s.TransitionTo(next, missionNow); err != nil {
			t.Fatalf("-> %s: %v", next, err)
		}
	}
	if err := s.TransitionTo(StepPending, missionNow); !errors.Is(err, ErrInvalidStepTransition) {
		t.Fatal("a completed step is final")
	}
}

func TestNormalizeMissionType(t *testing.T) {
	for in, want := range map[string]MissionType{"event": MissionTypeEvent, " Business trip ": MissionTypeBusinessTrip, "business-trip": MissionTypeBusinessTrip} {
		if got := NormalizeMissionType(in); got != want || !got.IsValid() {
			t.Errorf("NormalizeMissionType(%q) = %q, want %q", in, got, want)
		}
	}
	if NormalizeMissionType("wedding").IsValid() {
		t.Fatal("an unlisted type is not valid")
	}
}
