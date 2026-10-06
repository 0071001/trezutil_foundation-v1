package application

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
)

/*
 * Missions (Organiser). The use cases own who may do what; the repository only stores. A mission belongs to the
 * person who created it and to nobody else: every read and write checks the owner, and a mission of somebody
 * else answers ErrMissionForbidden. Creating a mission starts nothing: it records the goal as a draft with what
 * could be understood from it. No commerce action runs and no credit is used here; steps are recorded for a
 * later phase to connect to the Commerce Action Runtime.
 */

// MissionUseCases are the mission operations.
type MissionUseCases struct {
	repo    domain.MissionRepository
	planner domain.MissionPlanner
	plans   domain.MissionPlanRepository
	// annotator tells, for each step, whether it can be run and how its latest attempt went (the step executor).
	annotator domain.StepAnnotator
	now       func() time.Time
}

// NewMissionUseCases creates MissionUseCases.
func NewMissionUseCases(repo domain.MissionRepository) *MissionUseCases {
	return &MissionUseCases{repo: repo, now: time.Now}
}

// WithClock sets the clock (tests).
func (u *MissionUseCases) WithClock(now func() time.Time) *MissionUseCases {
	u.now = now
	return u
}

// CreateMissionInput is what a person says about a mission. Only Objective is required: the rest is understood
// from it (see UnderstandObjective) unless given.
type CreateMissionInput struct {
	Objective    string
	Type         string
	Title        string
	Location     string
	Participants int
	StartDate    int64
	EndDate      int64
	BudgetMinor  int64
	Currency     string
	Preferences  map[string]string
}

// MissionPatch changes a mission: nil leaves a field as it is.
type MissionPatch struct {
	Type         *string
	Title        *string
	Objective    *string
	Location     *string
	Participants *int
	StartDate    *int64
	EndDate      *int64
	BudgetMinor  *int64
	Currency     *string
	Preferences  map[string]string
	Status       *domain.MissionStatus
}

// StepInput is a new step.
type StepInput struct {
	Type        string
	Title       string
	Description string
	ActionType  string
}

// Create records a draft mission for the actor. With an idempotency key a retry returns the mission the first call
// made; the same key with another objective is domain.ErrIdempotencyConflict.
func (u *MissionUseCases) Create(ctx context.Context, actorID string, in CreateMissionInput, idempotencyKey string) (*domain.Mission, bool, error) {
	if actorID == "" {
		return nil, false, domain.ErrMissionForbidden
	}
	objective := strings.TrimSpace(in.Objective)
	if idempotencyKey != "" {
		if existing, err := u.repo.FindByIdempotencyKey(ctx, actorID, idempotencyKey); err == nil {
			if existing.Objective != objective {
				return nil, false, domain.ErrIdempotencyConflict
			}
			return existing, true, nil
		} else if !errors.Is(err, domain.ErrMissionNotFound) {
			return nil, false, err
		}
	}
	brief := UnderstandObjective(objective)
	typ := brief.Type
	if strings.TrimSpace(in.Type) != "" {
		typ = domain.NormalizeMissionType(in.Type)
	}
	title := firstNonEmpty(in.Title, brief.Title)
	m, err := domain.NewMission(actorID, typ, title, objective, u.now())
	if err != nil {
		return nil, false, err
	}
	m.Location = strings.TrimSpace(firstNonEmpty(in.Location, brief.Location))
	m.Participants = in.Participants
	if m.Participants == 0 {
		m.Participants = brief.Participants
	}
	m.StartDate, m.EndDate = in.StartDate, in.EndDate
	m.BudgetMinor, m.Currency = in.BudgetMinor, strings.ToUpper(strings.TrimSpace(in.Currency))
	if in.Preferences != nil {
		m.Preferences = in.Preferences
	}
	m.IdempotencyKey = idempotencyKey
	if err := m.Validate(); err != nil {
		return nil, false, err
	}
	if err := u.repo.Create(ctx, m); err != nil {
		if idempotencyKey != "" && errors.Is(err, domain.ErrIdempotencyConflict) { // a concurrent retry won
			if existing, ferr := u.repo.FindByIdempotencyKey(ctx, actorID, idempotencyKey); ferr == nil && existing.Objective == objective {
				return existing, true, nil
			}
		}
		return nil, false, err
	}
	return m, false, nil
}

// owned loads a mission and checks that the actor owns it.
func (u *MissionUseCases) owned(ctx context.Context, actorID, id string) (*domain.Mission, error) {
	if actorID == "" {
		return nil, domain.ErrMissionForbidden
	}
	m, err := u.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !m.OwnedBy(actorID) {
		return nil, domain.ErrMissionForbidden
	}
	return m, nil
}

// Get returns one of the actor's missions.
func (u *MissionUseCases) Get(ctx context.Context, actorID, id string) (*domain.Mission, error) {
	return u.owned(ctx, actorID, id)
}

// List returns the actor's missions, optionally of one status, with the total.
func (u *MissionUseCases) List(ctx context.Context, actorID string, status domain.MissionStatus, page domain.Page) ([]domain.Mission, int, error) {
	if actorID == "" {
		return nil, 0, domain.ErrMissionForbidden
	}
	if status != "" && !status.IsValid() {
		return nil, 0, fmt.Errorf("%w: unknown status", domain.ErrMissionInvalid)
	}
	return u.repo.ListByOwner(ctx, actorID, status, page)
}

// Update changes one of the actor's missions. A closed mission is read-only; a status change follows the lifecycle.
func (u *MissionUseCases) Update(ctx context.Context, actorID, id string, p MissionPatch) (*domain.Mission, error) {
	m, err := u.owned(ctx, actorID, id)
	if err != nil {
		return nil, err
	}
	if m.Status.IsTerminal() {
		return nil, domain.ErrMissionClosed
	}
	now := u.now()
	if p.Type != nil {
		m.Type = domain.NormalizeMissionType(*p.Type)
	}
	if p.Title != nil {
		m.Title = strings.TrimSpace(*p.Title)
	}
	if p.Objective != nil {
		m.Objective = strings.TrimSpace(*p.Objective)
	}
	if p.Location != nil {
		m.Location = strings.TrimSpace(*p.Location)
	}
	if p.Participants != nil {
		m.Participants = *p.Participants
	}
	if p.StartDate != nil {
		m.StartDate = *p.StartDate
	}
	if p.EndDate != nil {
		m.EndDate = *p.EndDate
	}
	if p.BudgetMinor != nil {
		m.BudgetMinor = *p.BudgetMinor
	}
	if p.Currency != nil {
		m.Currency = strings.ToUpper(strings.TrimSpace(*p.Currency))
	}
	if p.Preferences != nil {
		m.Preferences = p.Preferences
	}
	m.UpdatedAt = now.Unix()
	if err := m.Validate(); err != nil {
		return nil, err
	}
	if p.Status != nil && *p.Status != m.Status {
		if err := m.TransitionTo(*p.Status, now); err != nil {
			return nil, err
		}
	}
	if err := u.repo.Update(ctx, m); err != nil {
		return nil, err
	}
	return m, nil
}

// AddStep appends a step to one of the actor's missions. The step is recorded as pending: nothing runs.
func (u *MissionUseCases) AddStep(ctx context.Context, actorID, missionID string, in StepInput) (*domain.MissionStep, error) {
	m, err := u.owned(ctx, actorID, missionID)
	if err != nil {
		return nil, err
	}
	s, err := m.NewStep(in.Type, in.Title, in.Description, u.now())
	if err != nil {
		return nil, err
	}
	s.ActionType = domain.CommerceActionType(strings.ToUpper(strings.TrimSpace(in.ActionType)))
	if err := s.Validate(); err != nil {
		return nil, err
	}
	if err := u.repo.AddStep(ctx, s, domain.MissionMaxSteps); err != nil {
		return nil, err
	}
	return s, nil
}

// Steps lists the steps of one of the actor's missions in order.
func (u *MissionUseCases) Steps(ctx context.Context, actorID, missionID string) ([]domain.MissionStep, error) {
	if _, err := u.owned(ctx, actorID, missionID); err != nil {
		return nil, err
	}
	return u.repo.Steps(ctx, missionID)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// MissionBrief is what could be understood from a sentence, without any model: a kind, a title, a place and a
// head count. Anything not found is left empty; the person can correct it.
type MissionBrief struct {
	Type         domain.MissionType
	Title        string
	Location     string
	Participants int
}

type briefKind struct {
	words []string // accent-free, lower case; matched on word starts
	typ   domain.MissionType
	title string
}

// Ordered: the first kind with a matching word wins, so the specific ones come before the general ones.
var briefKinds = []briefKind{
	{[]string{"deplacement professionnel", "voyage d'affaires", "business trip"}, domain.MissionTypeBusinessTrip, "Déplacement professionnel"},
	{[]string{"anniversaire"}, domain.MissionTypeEvent, "Anniversaire"},
	{[]string{"birthday"}, domain.MissionTypeEvent, "Birthday"},
	{[]string{"mariage", "noces"}, domain.MissionTypeEvent, "Mariage"},
	{[]string{"wedding"}, domain.MissionTypeEvent, "Wedding"},
	{[]string{"fete", "soiree", "reception"}, domain.MissionTypeEvent, "Fête"},
	{[]string{"party"}, domain.MissionTypeEvent, "Party"},
	{[]string{"evenement", "conference", "seminaire", "cocktail"}, domain.MissionTypeEvent, "Événement"},
	{[]string{"event", "conference", "seminar"}, domain.MissionTypeEvent, "Event"},
	{[]string{"demenag"}, domain.MissionTypeMove, "Déménagement"},
	{[]string{"moving house", "move house", "relocation", "relocate"}, domain.MissionTypeMove, "Move"},
	{[]string{"sejour", "week-end", "weekend"}, domain.MissionTypeStay, "Séjour"},
	{[]string{"stay"}, domain.MissionTypeStay, "Stay"},
	{[]string{"voyage", "vacances"}, domain.MissionTypeTravel, "Voyage"},
	{[]string{"trip", "travel", "vacation", "holiday"}, domain.MissionTypeTravel, "Trip"},
	{[]string{"transfert", "navette", "airport pickup"}, domain.MissionTypeTransport, "Transport"},
	{[]string{"transport"}, domain.MissionTypeTransport, "Transport"},
	{[]string{"projet"}, domain.MissionTypeProject, "Projet"},
	{[]string{"project"}, domain.MissionTypeProject, "Project"},
}

var (
	participantsPattern = regexp.MustCompile(`(?i)\b(\d{1,6})\s*(?:personnes?|persons?|people|invit[ée]s?|guests?|pax|convives?|participants?|adultes?)\b`)
	locationPattern     = regexp.MustCompile(`(?:^|[\s,])(?:à|a|in|at|en|vers|near|près de|pres de)\s+(\p{Lu}[\p{L}'’-]*(?:\s+\p{Lu}[\p{L}'’-]*){0,2})`)
	monthWords          = map[string]bool{"janvier": true, "fevrier": true, "mars": true, "avril": true, "mai": true, "juin": true, "juillet": true, "aout": true, "septembre": true, "octobre": true, "novembre": true, "decembre": true, "january": true, "february": true, "march": true, "april": true, "may": true, "june": true, "july": true, "august": true, "september": true, "october": true, "november": true, "december": true}
)

// UnderstandObjective reads a sentence such as "Je veux organiser un anniversaire pour 40 personnes à Kinshasa"
// and returns what it says: an EVENT titled "Anniversaire", 40 people, Kinshasa. It is a plain keyword reading
// (French and English), not a model: it never invents, and an unclear sentence becomes an OTHER mission titled
// with its first words.
func UnderstandObjective(text string) MissionBrief {
	text = strings.TrimSpace(text)
	folded := foldAccents(strings.ToLower(text))
	brief := MissionBrief{Type: domain.MissionTypeOther}
	for _, k := range briefKinds {
		if containsKeyword(folded, k.words) {
			brief.Type, brief.Title = k.typ, k.title
			break
		}
	}
	if brief.Title == "" {
		brief.Title = titleFromWords(text)
	}
	if m := participantsPattern.FindStringSubmatch(text); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 && n <= domain.MissionMaxParticipant {
			brief.Participants = n
		}
	}
	for _, m := range locationPattern.FindAllStringSubmatch(text, -1) {
		place := strings.TrimSpace(m[1])
		if !monthWords[foldAccents(strings.ToLower(strings.Fields(place)[0]))] {
			brief.Location = place
			break
		}
	}
	return brief
}

func containsKeyword(folded string, words []string) bool {
	for _, w := range words {
		for i := 0; ; {
			j := strings.Index(folded[i:], w)
			if j < 0 {
				break
			}
			start := i + j
			if start == 0 || !isLetter(rune(folded[start-1])) { // a word start, so "fete" does not match "profete"
				return true
			}
			i = start + len(w)
		}
	}
	return false
}

func isLetter(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

var accentFolder = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a", "á", "a", "ç", "c", "é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i", "í", "i", "ô", "o", "ö", "o", "ó", "o", "ù", "u", "û", "u", "ü", "u", "ú", "u", "ÿ", "y", "œ", "oe", "’", "'",
)

// foldAccents lower-cases nothing: it maps the accented letters of French and Portuguese to plain ones.
func foldAccents(s string) string { return accentFolder.Replace(s) }

// titleFromWords makes a title of the first words of a sentence, without its leading "I want to organise".
func titleFromWords(text string) string {
	words := strings.Fields(text)
	for len(words) > 2 && leadingWords[foldAccents(strings.ToLower(strings.Trim(words[0], ".,;:!?")))] {
		words = words[1:]
	}
	title := strings.Join(words, " ")
	if utf8.RuneCountInString(title) > 60 {
		runes := []rune(title)[:60]
		title = strings.TrimSpace(string(runes))
		if i := strings.LastIndex(title, " "); i > 20 {
			title = title[:i]
		}
	}
	title = strings.Trim(title, " .,;:!?")
	if title == "" {
		return "Mission"
	}
	r := []rune(title)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

var leadingWords = map[string]bool{
	"je": true, "veux": true, "voudrais": true, "aimerais": true, "souhaite": true, "organiser": true, "preparer": true, "planifier": true,
	"i": true, "want": true, "would": true, "like": true, "to": true, "organise": true, "organize": true, "plan": true, "prepare": true,
	"un": true, "une": true, "le": true, "la": true, "a": true, "an": true, "the": true, "m'": true, "nous": true, "on": true,
}
