// Package memory is a person's commercial memory: what they chose to let Trezutil remember, kept on the server so
// every device sees it and the assistant can use it as hints when it interprets a request.
package memory

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
)

// Limits. The assistant receives at most MaxHintItems items and MaxHintChars characters in all.
const (
	MaxTextLength = 400
	MaxItems      = 200
	MaxHintItems  = 30
	MaxHintChars  = 2000
)

var (
	ErrEmptyText   = errors.New("memory text is empty")
	ErrInvalidKind = errors.New("memory kind is not one of the six kinds")
	ErrTooMany     = errors.New("memory is full")
	ErrNotFound    = domain.ErrMemoryItemNotFound
	ErrForbidden   = errors.New("sign in to use the commercial memory")
)

// SensitiveError says why a text was refused (payment data, a code or secret, bank details).
type SensitiveError struct{ Reason string }

func (e *SensitiveError) Error() string { return "memory refuses sensitive data: " + e.Reason }

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// Service applies the rules of commercial memory over a domain.MemoryStore.
type Service struct {
	store domain.MemoryStore
}

// NewService builds the service.
func NewService(store domain.MemoryStore) *Service {
	return &Service{store: store}
}

// Snapshot is a person's memory as they see it.
type Snapshot struct {
	Enabled bool
	Items   []domain.MemoryItem
}

// Get returns the caller's memory.
func (s *Service) Get(ctx context.Context, userID string) (*Snapshot, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	enabled, err := s.store.Enabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &Snapshot{Enabled: enabled, Items: items}, nil
}

// Add remembers one more thing for the caller.
func (s *Service) Add(ctx context.Context, userID string, kind domain.MemoryKind, text string) (*domain.MemoryItem, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	if !kind.IsValid() {
		return nil, ErrInvalidKind
	}
	clean, err := cleanText(text)
	if err != nil {
		return nil, err
	}
	n, err := s.store.Count(ctx, userID)
	if err != nil {
		return nil, err
	}
	if n >= MaxItems {
		return nil, ErrTooMany
	}
	item := &domain.MemoryItem{UserID: userID, Kind: kind, Text: clean}
	if err := s.store.Add(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

// Update changes the text of one of the caller's items.
func (s *Service) Update(ctx context.Context, userID, itemID, text string) (*domain.MemoryItem, error) {
	if userID == "" {
		return nil, ErrForbidden
	}
	if !uuidPattern.MatchString(itemID) {
		return nil, ErrNotFound
	}
	clean, err := cleanText(text)
	if err != nil {
		return nil, err
	}
	return s.store.UpdateText(ctx, userID, itemID, clean)
}

// Delete forgets one of the caller's items.
func (s *Service) Delete(ctx context.Context, userID, itemID string) error {
	if userID == "" {
		return ErrForbidden
	}
	if !uuidPattern.MatchString(itemID) {
		return ErrNotFound
	}
	return s.store.Delete(ctx, userID, itemID)
}

// Clear forgets everything the caller asked to remember.
func (s *Service) Clear(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrForbidden
	}
	return s.store.DeleteAll(ctx, userID)
}

// SetEnabled switches the caller's memory on or off. Off, it is kept but the assistant never reads it.
func (s *Service) SetEnabled(ctx context.Context, userID string, enabled bool) error {
	if userID == "" {
		return ErrForbidden
	}
	return s.store.SetEnabled(ctx, userID, enabled)
}

// Hints returns what the assistant may use for this person: one line per item ("brand: Dell"), the most recent
// first, within MaxHintItems and MaxHintChars. Nothing for a guest, a person who switched the memory off, or when
// the memory cannot be read (the conversation goes on without it).
func (s *Service) Hints(ctx context.Context, userID string) []string {
	if s == nil || userID == "" {
		return nil
	}
	enabled, err := s.store.Enabled(ctx, userID)
	if err != nil || !enabled {
		return nil
	}
	items, err := s.store.List(ctx, userID)
	if err != nil {
		return nil
	}
	out, total := []string{}, 0
	for _, item := range items {
		if len(out) == MaxHintItems {
			break
		}
		line := string(item.Kind) + ": " + oneLine(item.Text)
		if total+len(line) > MaxHintChars {
			break
		}
		out = append(out, line)
		total += len(line)
	}
	return out
}

func cleanText(text string) (string, error) {
	clean := strings.Join(strings.Fields(text), " ")
	if clean == "" {
		return "", ErrEmptyText
	}
	if utf8.RuneCountInString(clean) > MaxTextLength {
		clean = strings.TrimSpace(string([]rune(clean)[:MaxTextLength]))
	}
	if reason := domain.MemorySensitiveReason(clean); reason != "" {
		return "", &SensitiveError{Reason: reason}
	}
	return clean, nil
}

// oneLine keeps a hint on one line and free of characters that could fake a section of the prompt.
func oneLine(text string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', '\t', '<', '>', '`':
			return ' '
		}
		return r
	}, text)
}
