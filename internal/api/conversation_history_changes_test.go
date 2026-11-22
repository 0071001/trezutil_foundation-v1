package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// historyWriter is an in-memory ConversationHistoryWriter scoped by customer, as the PostgreSQL one is.
type historyWriter struct {
	owner    map[string]string // conversation id → customer id
	titles   map[string]string
	archived map[string]bool
	deleted  map[string]bool
}

func newHistoryWriter() *historyWriter {
	return &historyWriter{
		owner:  map[string]string{ownConv: "user-1", otherConv: "user-2", guestConv: ""},
		titles: map[string]string{}, archived: map[string]bool{}, deleted: map[string]bool{},
	}
}

func (s *historyWriter) owned(customerID, id string) bool {
	return customerID != "" && s.owner[id] == customerID && !s.deleted[id]
}
func (s *historyWriter) Rename(_ context.Context, customerID, id, title string) error {
	if !s.owned(customerID, id) {
		return domain.ErrConversationNotOwned
	}
	s.titles[id] = title
	return nil
}
func (s *historyWriter) SetArchived(_ context.Context, customerID, id string, archived bool) error {
	if !s.owned(customerID, id) {
		return domain.ErrConversationNotOwned
	}
	s.archived[id] = archived
	return nil
}
func (s *historyWriter) Delete(_ context.Context, customerID, id string) error {
	if !s.owned(customerID, id) {
		return domain.ErrConversationNotOwned
	}
	s.deleted[id] = true
	return nil
}

func historyHandlerWithWriter(w *historyWriter) *ConversationHandler {
	base := historyHandler()
	svc := base.history.(*conversation.HistoryService)
	return NewConversationHandler(nil, zerolog.Nop()).WithHistory(svc.WithWriter(w))
}

func historyChange(method, path, user, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	return req
}

func TestConversationRenameAndArchiveByTheOwner(t *testing.T) {
	w := newHistoryWriter()
	h := historyHandlerWithWriter(w)
	rec := httptest.NewRecorder()
	HandleConversationRoutes(h, rec, historyChange(http.MethodPatch, "/api/v1/conversations/"+ownConv, "user-1",
		`{"title":"  Achat   ordinateurs  bureau ","archived":true}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	if w.titles[ownConv] != "Achat ordinateurs bureau" || !w.archived[ownConv] {
		t.Fatalf("title %q archived %v", w.titles[ownConv], w.archived[ownConv])
	}
	var body map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["title"] != "Achat ordinateurs bureau" || body["archived"] != true {
		t.Fatalf("unexpected answer %v", body)
	}
}

func TestConversationTitleIsCut(t *testing.T) {
	w := newHistoryWriter()
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandlerWithWriter(w), rec, historyChange(http.MethodPatch, "/api/v1/conversations/"+ownConv, "user-1",
		`{"title":"`+strings.Repeat("é", 200)+`"}`))
	if rec.Code != http.StatusOK || len([]rune(w.titles[ownConv])) != conversation.MaxTitleLength {
		t.Fatalf("got %d, title of %d characters", rec.Code, len([]rune(w.titles[ownConv])))
	}
}

func TestConversationChangesAreOwnerOnly(t *testing.T) {
	for _, id := range []string{otherConv, guestConv, "not-a-uuid", "44444444-4444-4444-8444-444444444444"} {
		w := newHistoryWriter()
		h := historyHandlerWithWriter(w)
		for _, req := range []*http.Request{
			historyChange(http.MethodPatch, "/api/v1/conversations/"+id, "user-1", `{"archived":true}`),
			historyChange(http.MethodDelete, "/api/v1/conversations/"+id, "user-1", ``),
		} {
			rec := httptest.NewRecorder()
			HandleConversationRoutes(h, rec, req)
			// Somebody else's conversation answers exactly like one that does not exist.
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s %s: want 404, got %d", req.Method, id, rec.Code)
			}
		}
		if w.archived[id] || w.deleted[id] {
			t.Fatalf("%s was changed by someone who does not own it", id)
		}
	}
}

func TestConversationChangesNeedASignedInUser(t *testing.T) {
	h := historyHandlerWithWriter(newHistoryWriter())
	for _, req := range []*http.Request{
		historyChange(http.MethodPatch, "/api/v1/conversations/"+ownConv, "", `{"archived":true}`),
		historyChange(http.MethodDelete, "/api/v1/conversations/"+ownConv, "", ``),
	} {
		rec := httptest.NewRecorder()
		HandleConversationRoutes(h, rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s as a guest: want 401, got %d", req.Method, rec.Code)
		}
	}
}

func TestConversationDeleteByTheOwner(t *testing.T) {
	w := newHistoryWriter()
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandlerWithWriter(w), rec, historyChange(http.MethodDelete, "/api/v1/conversations/"+ownConv, "user-1", ``))
	if rec.Code != http.StatusNoContent || !w.deleted[ownConv] {
		t.Fatalf("want 204 and deleted, got %d deleted=%v", rec.Code, w.deleted[ownConv])
	}
}

func TestConversationPatchNeedsSomethingToChange(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandlerWithWriter(newHistoryWriter()), rec, historyChange(http.MethodPatch, "/api/v1/conversations/"+ownConv, "user-1", `{}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestConversationChangesWithoutWriterAreUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandler(), rec, historyChange(http.MethodDelete, "/api/v1/conversations/"+ownConv, "user-1", ``))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rec.Code)
	}
}

func TestConversationTranscriptCarriesStoredCards(t *testing.T) {
	// The record a live answer stores, with one key that must not be read back.
	stored := map[string]interface{}{conversation.ReplyRecordKey: map[string]interface{}{
		"cards":    []interface{}{map[string]interface{}{"id": "lst-1", "type": "listing", "title": "Dell Latitude"}},
		"metadata": map[string]interface{}{"action_type": "SEARCH_LISTINGS", "action_id": "should-not-leak"},
	}}
	if got := conversation.StoredReply(stored); got == nil {
		t.Fatal("a stored record must be read back")
	} else {
		meta := got["metadata"].(map[string]interface{})
		if _, leaked := meta["action_id"]; leaked {
			t.Fatal("only the kept metadata keys may be read back")
		}
		if len(got["cards"].([]interface{})) != 1 {
			t.Fatalf("cards lost: %v", got)
		}
	}
}
