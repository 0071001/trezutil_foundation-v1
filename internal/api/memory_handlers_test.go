package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/memory"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type apiMemoryStore struct {
	items    []domain.MemoryItem
	disabled map[string]bool
}

func (s *apiMemoryStore) List(_ context.Context, userID string) ([]domain.MemoryItem, error) {
	out := []domain.MemoryItem{}
	for _, it := range s.items {
		if it.UserID == userID {
			out = append(out, it)
		}
	}
	return out, nil
}
func (s *apiMemoryStore) Count(ctx context.Context, userID string) (int, error) {
	l, _ := s.List(ctx, userID)
	return len(l), nil
}
func (s *apiMemoryStore) Add(_ context.Context, item *domain.MemoryItem) error {
	item.ID = fmt.Sprintf("00000000-0000-4000-8000-%012d", len(s.items)+1)
	s.items = append(s.items, *item)
	return nil
}
func (s *apiMemoryStore) UpdateText(_ context.Context, userID, id, text string) (*domain.MemoryItem, error) {
	for i := range s.items {
		if s.items[i].ID == id && s.items[i].UserID == userID {
			s.items[i].Text = text
			return &s.items[i], nil
		}
	}
	return nil, domain.ErrMemoryItemNotFound
}
func (s *apiMemoryStore) Delete(_ context.Context, userID, id string) error {
	for i := range s.items {
		if s.items[i].ID == id && s.items[i].UserID == userID {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrMemoryItemNotFound
}
func (s *apiMemoryStore) DeleteAll(_ context.Context, userID string) error {
	kept := []domain.MemoryItem{}
	for _, it := range s.items {
		if it.UserID != userID {
			kept = append(kept, it)
		}
	}
	s.items = kept
	return nil
}
func (s *apiMemoryStore) Enabled(_ context.Context, userID string) (bool, error) {
	return !s.disabled[userID], nil
}
func (s *apiMemoryStore) SetEnabled(_ context.Context, userID string, enabled bool) error {
	s.disabled[userID] = !enabled
	return nil
}

func memoryCall(h http.Handler, method, path, user, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestMemoryRoutes(t *testing.T) {
	store := &apiMemoryStore{disabled: map[string]bool{}}
	h := NewMemoryHandler(memory.NewService(store), zerolog.Nop())

	if rec := memoryCall(h, http.MethodGet, "/api/v1/memory", "", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("a guest: want 401, got %d", rec.Code)
	}

	rec := memoryCall(h, http.MethodPost, "/api/v1/memory/items", "u1", `{"kind":"brand","text":"Dell"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add: want 201, got %d: %s", rec.Code, rec.Body)
	}
	var added memoryItemJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &added)

	if rec := memoryCall(h, http.MethodPost, "/api/v1/memory/items", "u1", `{"kind":"preference","text":"code pin 1234"}`); rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), `"reason":"secret"`) {
		t.Fatalf("a secret: want 422 with its reason, got %d: %s", rec.Code, rec.Body)
	}
	if rec := memoryCall(h, http.MethodPost, "/api/v1/memory/items", "u1", `{"kind":"x","text":"y"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown kind: want 400, got %d", rec.Code)
	}

	rec = memoryCall(h, http.MethodGet, "/api/v1/memory", "u1", "")
	var snap struct {
		Enabled bool
		Items   []memoryItemJSON
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &snap)
	if rec.Code != http.StatusOK || !snap.Enabled || len(snap.Items) != 1 || snap.Items[0].Text != "Dell" {
		t.Fatalf("get: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("a personal memory must not be cached")
	}

	// Someone else neither sees nor changes it.
	rec = memoryCall(h, http.MethodGet, "/api/v1/memory", "u2", "")
	if strings.Contains(rec.Body.String(), "Dell") {
		t.Fatal("another user sees this memory")
	}
	if rec := memoryCall(h, http.MethodPatch, "/api/v1/memory/items/"+added.ID, "u2", `{"text":"HP"}`); rec.Code != http.StatusNotFound {
		t.Fatalf("another user's change: want 404, got %d", rec.Code)
	}
	if rec := memoryCall(h, http.MethodDelete, "/api/v1/memory/items/"+added.ID, "u2", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("another user's delete: want 404, got %d", rec.Code)
	}

	if rec := memoryCall(h, http.MethodPatch, "/api/v1/memory/items/"+added.ID, "u1", `{"text":"Lenovo"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Lenovo") {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	if rec := memoryCall(h, http.MethodPut, "/api/v1/memory/settings", "u1", `{"enabled":false}`); rec.Code != http.StatusOK || !store.disabled["u1"] {
		t.Fatalf("switch off: %d", rec.Code)
	}
	if rec := memoryCall(h, http.MethodPut, "/api/v1/memory/settings", "u1", `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("settings without enabled: want 400, got %d", rec.Code)
	}
	if rec := memoryCall(h, http.MethodDelete, "/api/v1/memory/items/"+added.ID, "u1", ""); rec.Code != http.StatusNoContent || len(store.items) != 0 {
		t.Fatalf("delete: %d, %d left", rec.Code, len(store.items))
	}
	_ = memoryCall(h, http.MethodPost, "/api/v1/memory/items", "u1", `{"kind":"brand","text":"Dell"}`)
	_ = memoryCall(h, http.MethodPost, "/api/v1/memory/items", "u2", `{"kind":"brand","text":"HP"}`)
	if rec := memoryCall(h, http.MethodDelete, "/api/v1/memory/items", "u1", ""); rec.Code != http.StatusNoContent || len(store.items) != 1 || store.items[0].UserID != "u2" {
		t.Fatalf("clear must remove only the caller's items: %d %+v", rec.Code, store.items)
	}
}
