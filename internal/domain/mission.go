package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

/*
 * Organiser: a Mission is a goal a person delegates to Trezutil ("organise a birthday for 40 people in Kinshasa")
 * that may need several commerce actions. It is an orchestration layer, not a second commerce engine: a step only
 * REFERENCES a commerce action (ActionType, ActionRef) and its result (ResultRef); the Commerce Action Runtime
 * stays the one place where actions run, and credits stay priced per action by the usage policies. Nothing in a
 * Mission is priced or metered.
 *
 * Phase 1 holds the mission and its ordered steps. Steps are recorded, not executed.
 */

// MissionType is what a mission is about. Codes are open like verticals: a new kind of mission is a new code,
// not a schema change.
type MissionType string

const (
	MissionTypeTravel       MissionType = "TRAVEL"
	MissionTypeEvent        MissionType = "EVENT"
	MissionTypeMove         MissionType = "MOVE"
	MissionTypeStay         MissionType = "STAY"
	MissionTypeTransport    MissionType = "TRANSPORT"
	MissionTypeBusinessTrip MissionType = "BUSINESS_TRIP"
	MissionTypeProject      MissionType = "PROJECT"
	MissionTypeOther        MissionType = "OTHER"
)

// MissionTypes lists the mission types in display order.
func MissionTypes() []MissionType {
	return []MissionType{MissionTypeEvent, MissionTypeTravel, MissionTypeStay, MissionTypeMove, MissionTypeTransport, MissionTypeBusinessTrip, MissionTypeProject, MissionTypeOther}
}

// IsValid reports whether t is a known mission type.
func (t MissionType) IsValid() bool {
	for _, k := range MissionTypes() {
		if t == k {
			return true
		}
	}
	return false
}

// NormalizeMissionType turns input ("event", "business trip") into a type code. "accommodation" and "moving" are
// accepted for STAY and MOVE, the names the planning templates are described with.
func NormalizeMissionType(raw string) MissionType {
	s := strings.ToUpper(strings.TrimSpace(raw))
	t := MissionType(strings.NewReplacer("-", "_", " ", "_").Replace(s))
	switch t {
	case "ACCOMMODATION":
		return MissionTypeStay
	case "MOVING":
		return MissionTypeMove
	}
	return t
}

// MissionStatus is where a mission is in its life. Values are lower case like order and reservation statuses.
type MissionStatus string

const (
	MissionDraft     MissionStatus = "draft"
	MissionActive    MissionStatus = "active"
	MissionPaused    MissionStatus = "paused"
	MissionCompleted MissionStatus = "completed"
	MissionCancelled MissionStatus = "cancelled"
)

var missionTransitions = map[MissionStatus][]MissionStatus{
	MissionDraft:     {MissionActive, MissionCancelled},
	MissionActive:    {MissionPaused, MissionCompleted, MissionCancelled},
	MissionPaused:    {MissionActive, MissionCancelled},
	MissionCompleted: nil,
	MissionCancelled: nil,
}

// IsValid reports whether s is a known mission status.
func (s MissionStatus) IsValid() bool {
	_, ok := missionTransitions[s]
	return ok
}

// IsTerminal reports whether a mission in this status can no longer change status or take steps.
func (s MissionStatus) IsTerminal() bool { return s == MissionCompleted || s == MissionCancelled }

// CanTransitionTo reports whether a mission may move from s to next.
func (s MissionStatus) CanTransitionTo(next MissionStatus) bool {
	for _, allowed := range missionTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// Mission errors.
var (
	// ErrMissionNotFound: no mission with that id.
	ErrMissionNotFound = fmt.Errorf("mission not found")
	// ErrMissionForbidden: the mission belongs to somebody else.
	ErrMissionForbidden = fmt.Errorf("this mission belongs to another account")
	// ErrMissionInvalid: a field failed validation; the wrapped message says which.
	ErrMissionInvalid = fmt.Errorf("invalid mission")
	// ErrInvalidMissionTransition: the mission cannot move to that status from where it is.
	ErrInvalidMissionTransition = fmt.Errorf("the mission cannot move to that status")
	// ErrMissionClosed: a completed or cancelled mission takes no more steps and no more edits.
	ErrMissionClosed = fmt.Errorf("the mission is closed")
	// ErrMissionStepLimit: the mission already has the most steps allowed.
	ErrMissionStepLimit = fmt.Errorf("the mission has the most steps allowed")
	// ErrInvalidStepTransition: the step cannot move to that status from where it is.
	ErrInvalidStepTransition = fmt.Errorf("the step cannot move to that status")
)

func missionInvalid(format string, args ...interface{}) error {
	return fmt.Errorf("%w: %s", ErrMissionInvalid, fmt.Sprintf(format, args...))
}

// Limits of a mission's free text and shape.
const (
	MissionTitleMin       = 2
	MissionTitleMax       = 120
	MissionObjectiveMin   = 3
	MissionObjectiveMax   = 2000
	MissionLocationMax    = 120
	MissionMaxParticipant = 100000
	MissionMaxPreferences = 20
	MissionMaxSteps       = 30
	MissionStepTitleMax   = 120
	MissionStepDescMax    = 1000
)

var (
	currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	stepTypePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,31}$`)
	actionPattern   = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
)

// Mission is a goal delegated to Trezutil. Dates are unix seconds at 00:00 UTC of the day, 0 when not set;
// money is in minor units like every other amount.
type Mission struct {
	ID             string
	OwnerID        string
	Type           MissionType
	Title          string
	Objective      string
	Status         MissionStatus
	Location       string
	StartDate      int64
	EndDate        int64
	Participants   int
	BudgetMinor    int64
	Currency       string
	Preferences    map[string]string
	IdempotencyKey string
	CreatedAt      int64
	UpdatedAt      int64
}

// NewMission creates a draft mission owned by ownerID and validates it.
func NewMission(ownerID string, typ MissionType, title, objective string, now time.Time) (*Mission, error) {
	m := &Mission{
		ID: NewID(), OwnerID: ownerID, Type: typ, Title: strings.TrimSpace(title), Objective: strings.TrimSpace(objective),
		Status: MissionDraft, Preferences: map[string]string{}, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// Validate checks every field of the mission.
func (m *Mission) Validate() error {
	if strings.TrimSpace(m.OwnerID) == "" {
		return missionInvalid("an owner is required")
	}
	if !m.Type.IsValid() {
		return missionInvalid("type must be one of %s", joinMissionTypes())
	}
	if n := utf8.RuneCountInString(m.Title); n < MissionTitleMin || n > MissionTitleMax {
		return missionInvalid("title must be %d to %d characters", MissionTitleMin, MissionTitleMax)
	}
	if n := utf8.RuneCountInString(m.Objective); n < MissionObjectiveMin || n > MissionObjectiveMax {
		return missionInvalid("objective must be %d to %d characters", MissionObjectiveMin, MissionObjectiveMax)
	}
	if !m.Status.IsValid() {
		return missionInvalid("unknown status")
	}
	if utf8.RuneCountInString(m.Location) > MissionLocationMax {
		return missionInvalid("location must be at most %d characters", MissionLocationMax)
	}
	if m.StartDate < 0 || m.EndDate < 0 || (m.StartDate != 0 && m.EndDate != 0 && m.EndDate < m.StartDate) {
		return missionInvalid("the end date cannot be before the start date")
	}
	if m.Participants < 0 || m.Participants > MissionMaxParticipant {
		return missionInvalid("participants must be between 0 and %d", MissionMaxParticipant)
	}
	if m.BudgetMinor < 0 {
		return missionInvalid("budget cannot be negative")
	}
	if m.BudgetMinor > 0 && !currencyPattern.MatchString(m.Currency) {
		return missionInvalid("a budget needs a three-letter currency")
	}
	if len(m.Preferences) > MissionMaxPreferences {
		return missionInvalid("at most %d preferences", MissionMaxPreferences)
	}
	for k, v := range m.Preferences {
		if k = strings.TrimSpace(k); k == "" || utf8.RuneCountInString(k) > 40 || utf8.RuneCountInString(v) > 200 {
			return missionInvalid("a preference needs a name of at most 40 characters and a value of at most 200")
		}
	}
	return nil
}

func joinMissionTypes() string {
	var parts []string
	for _, t := range MissionTypes() {
		parts = append(parts, string(t))
	}
	return strings.Join(parts, ", ")
}

// OwnedBy reports whether the mission belongs to userID.
func (m *Mission) OwnedBy(userID string) bool { return userID != "" && m.OwnerID == userID }

// TransitionTo moves the mission to next if the lifecycle allows it.
func (m *Mission) TransitionTo(next MissionStatus, now time.Time) error {
	if !next.IsValid() || !m.Status.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidMissionTransition, m.Status, next)
	}
	m.Status, m.UpdatedAt = next, now.Unix()
	return nil
}

// AcceptsSteps reports whether steps may still be added.
func (m *Mission) AcceptsSteps() bool { return !m.Status.IsTerminal() }

// NewStep builds a pending step for the mission. Its Position is set by the repository when it is stored, so
// concurrent additions never share one.
func (m *Mission) NewStep(typ, title, description string, now time.Time) (*MissionStep, error) {
	if !m.AcceptsSteps() {
		return nil, ErrMissionClosed
	}
	s := &MissionStep{
		ID: NewID(), MissionID: m.ID, Type: strings.ToUpper(strings.TrimSpace(typ)), Title: strings.TrimSpace(title),
		Description: strings.TrimSpace(description), Status: StepPending, CreatedAt: now.Unix(), UpdatedAt: now.Unix(),
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// MissionStepStatus is where one step stands.
type MissionStepStatus string

const (
	StepPending    MissionStepStatus = "pending"
	StepInProgress MissionStepStatus = "in_progress"
	StepCompleted  MissionStepStatus = "completed"
	StepSkipped    MissionStepStatus = "skipped"
	StepFailed     MissionStepStatus = "failed"
)

var stepTransitions = map[MissionStepStatus][]MissionStepStatus{
	StepPending:    {StepInProgress, StepSkipped},
	StepInProgress: {StepCompleted, StepFailed, StepSkipped},
	StepFailed:     {StepPending, StepInProgress, StepSkipped}, // a failed step is run again directly: pending is only for resetting it
	StepCompleted:  nil,
	StepSkipped:    {StepPending},
}

// IsValid reports whether s is a known step status.
func (s MissionStepStatus) IsValid() bool {
	_, ok := stepTransitions[s]
	return ok
}

// CanTransitionTo reports whether a step may move from s to next.
func (s MissionStepStatus) CanTransitionTo(next MissionStepStatus) bool {
	for _, allowed := range stepTransitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

// MissionStep is one independent, ordered piece of a mission ("find a venue"). ActionType names the commerce
// action the step will run through the Commerce Action Runtime, ActionRef the action it ran as and ResultRef what
// it produced (an order, a reservation, a search); all three stay empty until a later phase connects steps.
type MissionStep struct {
	ID          string
	MissionID   string
	Type        string
	Title       string
	Description string
	Status      MissionStepStatus
	Position    int
	ActionType  CommerceActionType
	// ActionSupport says whether the Commerce OS can run ActionType today. It is set for planned steps; a step
	// added by hand leaves it empty.
	ActionSupport StepActionSupport
	// PlanVersion is the plan the step belongs to; 0 for a step added by hand.
	PlanVersion int
	ActionRef   string
	ResultRef   string
	CreatedAt   int64
	UpdatedAt   int64
}

// Validate checks the step's fields.
func (s *MissionStep) Validate() error {
	if !stepTypePattern.MatchString(s.Type) {
		return missionInvalid("a step type is a code such as VENUE or CATERING")
	}
	if n := utf8.RuneCountInString(s.Title); n < MissionTitleMin || n > MissionStepTitleMax {
		return missionInvalid("step title must be %d to %d characters", MissionTitleMin, MissionStepTitleMax)
	}
	if utf8.RuneCountInString(s.Description) > MissionStepDescMax {
		return missionInvalid("step description must be at most %d characters", MissionStepDescMax)
	}
	if !s.Status.IsValid() {
		return missionInvalid("unknown step status")
	}
	if s.ActionType != "" && !actionPattern.MatchString(string(s.ActionType)) {
		return missionInvalid("action type must be a commerce action code")
	}
	if !s.ActionSupport.IsValid() {
		return missionInvalid("unknown action support")
	}
	if s.PlanVersion < 0 {
		return missionInvalid("plan version cannot be negative")
	}
	return nil
}

// TransitionTo moves the step to next if its lifecycle allows it.
func (s *MissionStep) TransitionTo(next MissionStepStatus, now time.Time) error {
	if !next.IsValid() || !s.Status.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s to %s", ErrInvalidStepTransition, s.Status, next)
	}
	s.Status, s.UpdatedAt = next, now.Unix()
	return nil
}

// MissionRepository stores missions and their steps.
type MissionRepository interface {
	// Create stores a new mission.
	Create(ctx context.Context, m *Mission) error
	// FindByIdempotencyKey returns the owner's mission created with that key, or ErrMissionNotFound.
	FindByIdempotencyKey(ctx context.Context, ownerID, key string) (*Mission, error)
	// Get returns a mission by id, or ErrMissionNotFound.
	Get(ctx context.Context, id string) (*Mission, error)
	// ListByOwner returns the owner's missions, most recently changed first, optionally of one status, with the
	// total count.
	ListByOwner(ctx context.Context, ownerID string, status MissionStatus, page Page) ([]Mission, int, error)
	// Update stores the mission's changed fields and status; ErrMissionNotFound when it does not exist.
	Update(ctx context.Context, m *Mission) error
	// AddStep stores the step at the next free position of its mission (set on the step); it returns
	// ErrMissionNotFound for an unknown mission and ErrMissionStepLimit at maxSteps.
	AddStep(ctx context.Context, s *MissionStep, maxSteps int) error
	// Steps lists the mission's steps in order.
	Steps(ctx context.Context, missionID string) ([]MissionStep, error)
}
