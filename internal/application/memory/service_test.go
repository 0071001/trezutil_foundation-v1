package memory

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// fakeStore is an in-memory domain.MemoryStore scoped by user, like the PostgreSQL one.
type fakeStore struct {
	items    []domain.MemoryItem
	disabled map[string]bool
	next     int
	failList bool
}

func newFakeStore() *fakeStore { return &fakeStore{disabled: map[string]bool{}} }

func (s *fakeStore) List(_ context.Context, userID string) ([]domain.MemoryItem, error) {
	if s.failList {
		return nil, errors.New("down")
	}
	out := []domain.MemoryItem{}
	for i := len(s.items) - 1; i >= 0; i-- { // most recent first
		if s.items[i].UserID == userID {
			out = append(out, s.items[i])
		}
	}
	return out, nil
}
func (s *fakeStore) Count(ctx context.Context, userID string) (int, error) {
	l, _ := s.List(ctx, userID)
	return len(l), nil
}
func (s *fakeStore) Add(_ context.Context, item *domain.MemoryItem) error {
	s.next++
	item.ID = fmt.Sprintf("00000000-0000-4000-8000-%012d", s.next)
	s.items = append(s.items, *item)
	return nil
}
func (s *fakeStore) UpdateText(_ context.Context, userID, id, text string) (*domain.MemoryItem, error) {
	for i := range s.items {
		if s.items[i].ID == id && s.items[i].UserID == userID {
			s.items[i].Text = text
			return &s.items[i], nil
		}
	}
	return nil, domain.ErrMemoryItemNotFound
}
func (s *fakeStore) Delete(_ context.Context, userID, id string) error {
	for i := range s.items {
		if s.items[i].ID == id && s.items[i].UserID == userID {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrMemoryItemNotFound
}
func (s *fakeStore) DeleteAll(_ context.Context, userID string) error {
	kept := []domain.MemoryItem{}
	for _, it := range s.items {
		if it.UserID != userID {
			kept = append(kept, it)
		}
	}
	s.items = kept
	return nil
}
func (s *fakeStore) Enabled(_ context.Context, userID string) (bool, error) { return !s.disabled[userID], nil }
func (s *fakeStore) SetEnabled(_ context.Context, userID string, enabled bool) error {
	s.disabled[userID] = !enabled
	return nil
}

func TestAddCleansAndRefusesSecrets(t *testing.T) {
	svc := NewService(newFakeStore())
	ctx := context.Background()
	item, err := svc.Add(ctx, "u1", domain.MemoryBrand, "  Dell   ou  HP  ")
	if err != nil || item.Text != "Dell ou HP" {
		t.Fatalf("got %+v %v", item, err)
	}
	for _, text := range []string{"Ma carte 4970 1012 3456 7890", "mot de passe: abc", "IBAN CD12 3456"} {
		_, err := svc.Add(ctx, "u1", domain.MemoryPreference, text)
		var sensitive *SensitiveError
		if !errors.As(err, &sensitive) {
			t.Fatalf("%q must be refused as sensitive, got %v", text, err)
		}
	}
	if _, err := svc.Add(ctx, "u1", "colour", "bleu"); !errors.Is(err, ErrInvalidKind) {
		t.Fatalf("an unknown kind must be refused, got %v", err)
	}
	if _, err := svc.Add(ctx, "u1", domain.MemoryBrand, "   "); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("an empty text must be refused, got %v", err)
	}
	if _, err := svc.Add(ctx, "", domain.MemoryBrand, "Dell"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("a guest has no memory, got %v", err)
	}
	long, _ := svc.Add(ctx, "u1", domain.MemoryProject, strings.Repeat("a", 600))
	if long == nil || len([]rune(long.Text)) != MaxTextLength {
		t.Fatalf("a long text is cut to %d characters", MaxTextLength)
	}
}

func TestMemoryIsLimited(t *testing.T) {
	svc := NewService(newFakeStore())
	for i := 0; i < MaxItems; i++ {
		if _, err := svc.Add(context.Background(), "u1", domain.MemoryProject, fmt.Sprintf("projet %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.Add(context.Background(), "u1", domain.MemoryProject, "un de trop"); !errors.Is(err, ErrTooMany) {
		t.Fatalf("want ErrTooMany, got %v", err)
	}
}

func TestChangesAreScopedToTheOwner(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	ctx := context.Background()
	mine, _ := svc.Add(ctx, "u1", domain.MemoryBrand, "Dell")
	if _, err := svc.Update(ctx, "u2", mine.ID, "HP"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("someone else cannot change it, got %v", err)
	}
	if err := svc.Delete(ctx, "u2", mine.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("someone else cannot delete it, got %v", err)
	}
	if err := svc.Delete(ctx, "u1", "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a malformed id is not found, got %v", err)
	}
	if updated, err := svc.Update(ctx, "u1", mine.ID, "Lenovo"); err != nil || updated.Text != "Lenovo" {
		t.Fatalf("the owner changes it: %+v %v", updated, err)
	}
}

func TestHintsRespectTheSwitchAndTheLimits(t *testing.T) {
	store := newFakeStore()
	svc := NewService(store)
	ctx := context.Background()
	_, _ = svc.Add(ctx, "u1", domain.MemoryBrand, "Dell")
	_, _ = svc.Add(ctx, "u1", domain.MemoryBudget, "Moins de 800 USD\npar poste <b>")
	_, _ = svc.Add(ctx, "u2", domain.MemoryBrand, "Apple")

	hints := svc.Hints(ctx, "u1")
	if len(hints) != 2 || hints[0] != "budget: Moins de 800 USD par poste  b " || hints[1] != "brand: Dell" {
		t.Fatalf("unexpected hints %q", hints)
	}
	if svc.Hints(ctx, "") != nil {
		t.Fatal("a guest has no hints")
	}
	_ = svc.SetEnabled(ctx, "u1", false)
	if svc.Hints(ctx, "u1") != nil {
		t.Fatal("a memory switched off is never given to the assistant")
	}
	_ = svc.SetEnabled(ctx, "u1", true)
	store.failList = true
	if svc.Hints(ctx, "u1") != nil {
		t.Fatal("an unreadable memory gives no hints (the conversation goes on)")
	}
	store.failList = false
	for i := 0; i < 50; i++ {
		_, _ = svc.Add(ctx, "u3", domain.MemoryProject, strings.Repeat("x", 100))
	}
	got, total := svc.Hints(ctx, "u3"), 0
	for _, h := range got {
		total += len(h)
	}
	if len(got) > MaxHintItems || total > MaxHintChars {
		t.Fatalf("hints exceed the limits: %d items, %d chars", len(got), total)
	}
}
