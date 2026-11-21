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

// historyConvs is an in-memory ConversationStore + ConversationHistoryReader; historyMsgs a MessageStore.
type historyConvs struct {
	byID map[string]*domain.Conversation
}

func (s *historyConvs) Create(context.Context, *domain.Conversation) error { return nil }
func (s *historyConvs) FindByID(_ context.Context, id string) (*domain.Conversation, error) {
	return s.byID[id], nil
}
func (s *historyConvs) FindByChannelExternalID(context.Context, domain.Channel, string) (*domain.Conversation, error) {
	return nil, nil
}
func (s *historyConvs) Update(context.Context, *domain.Conversation) error { return nil }
func (s *historyConvs) ListByCustomer(_ context.Context, customerID string, channel domain.Channel, _ int) ([]domain.ConversationHistoryItem, error) {
	out := []domain.ConversationHistoryItem{}
	for _, c := range s.byID {
		if c.CustomerID == customerID && c.Channel == channel {
			out = append(out, domain.ConversationHistoryItem{ID: c.ID, Channel: c.Channel, ExternalID: c.ExternalID, Title: "Je cherche 50 ordinateurs", MessageCount: 2})
		}
	}
	return out, nil
}

type historyMsgs struct{ byConv map[string][]*domain.Message }

func (s *historyMsgs) Create(context.Context, *domain.Message) error             { return nil }
func (s *historyMsgs) FindByID(context.Context, string) (*domain.Message, error) { return nil, nil }
func (s *historyMsgs) FindByConversationID(_ context.Context, id string) ([]*domain.Message, error) {
	return s.byConv[id], nil
}
func (s *historyMsgs) FindByExternalRef(context.Context, domain.Channel, string) (*domain.Message, error) {
	return nil, nil
}

const (
	ownConv   = "11111111-1111-4111-8111-111111111111"
	otherConv = "22222222-2222-4222-8222-222222222222"
	guestConv = "33333333-3333-4333-8333-333333333333"
)

func historyHandler() *ConversationHandler {
	convs := &historyConvs{byID: map[string]*domain.Conversation{
		ownConv:   {ID: ownConv, CustomerID: "user-1", Channel: domain.ChannelWeb, ExternalID: "web-a"},
		otherConv: {ID: otherConv, CustomerID: "user-2", Channel: domain.ChannelWeb, ExternalID: "web-b"},
		guestConv: {ID: guestConv, CustomerID: "", Channel: domain.ChannelWeb, ExternalID: "web-c"},
	}}
	msgs := &historyMsgs{byConv: map[string][]*domain.Message{
		ownConv: {
			// Two exchanges stored in the same second, in the order the database returned them: each answer must
			// follow its own question (the processor keys an answer "reply:" + the question's key).
			{ID: "m1", Direction: domain.MessageDirectionInbound, IdempotencyKey: "k1", Content: &domain.MessageContent{Text: "Je cherche 50 ordinateurs", Data: map[string]interface{}{"locale": "fr"}}, CreatedAt: 100},
			{ID: "m3", Direction: domain.MessageDirectionInbound, IdempotencyKey: "k2", Content: &domain.MessageContent{Text: "Plutôt des Dell"}, CreatedAt: 100},
			{ID: "m2", Direction: domain.MessageDirectionOutbound, IdempotencyKey: "reply:k1", Content: &domain.MessageContent{Text: "J’ai trouvé 2 offres."}, CreatedAt: 100},
			{ID: "m4", Direction: domain.MessageDirectionOutbound, IdempotencyKey: "reply:k2", Content: &domain.MessageContent{Text: "Voici les Dell."}, CreatedAt: 100},
			{ID: "m5", Direction: domain.MessageDirectionOutbound, IdempotencyKey: "reply:confirm:tok", Content: &domain.MessageContent{Text: "Commande créée."}, CreatedAt: 101},
		},
	}}
	return NewConversationHandler(nil, zerolog.Nop()).WithHistory(conversation.NewHistoryService(convs, convs, msgs))
}

func historyRequest(path, user string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	return req
}

func TestConversationHistoryRequiresASignedInUser(t *testing.T) {
	h := historyHandler()
	for _, path := range []string{"/api/v1/conversations", "/api/v1/conversations/" + ownConv + "/messages"} {
		rec := httptest.NewRecorder()
		HandleConversationRoutes(h, rec, historyRequest(path, ""))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s as a guest: want 401, got %d", path, rec.Code)
		}
	}
}

func TestConversationHistoryListsOnlyTheCallersConversations(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandler(), rec, historyRequest("/api/v1/conversations", "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct{ Items []conversationSummaryJSON }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].ID != ownConv || body.Items[0].ExternalID != "web-a" {
		t.Fatalf("want only the caller's conversation, got %+v", body.Items)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("a personal list must not be cached")
	}
}

func TestConversationHistoryRejectsABadLimit(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandler(), rec, historyRequest("/api/v1/conversations?limit=500", "user-1"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestConversationTranscriptOrderAndContent(t *testing.T) {
	rec := httptest.NewRecorder()
	HandleConversationRoutes(historyHandler(), rec, historyRequest("/api/v1/conversations/"+ownConv+"/messages", "user-1"))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body)
	}
	var body struct {
		ConversationID string `json:"conversation_id"`
		Items          []transcriptMessageJSON
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.ConversationID != ownConv || len(body.Items) != 5 {
		t.Fatalf("unexpected transcript: %+v", body)
	}
	got := []string{}
	for _, m := range body.Items {
		got = append(got, m.ID)
	}
	if strings.Join(got, ",") != "m1,m2,m3,m4,m5" {
		t.Fatalf("each answer must follow its question: got %v", got)
	}
	if strings.Contains(rec.Body.String(), "locale") {
		t.Fatal("the transcript must carry the text only, not the processor's internal data")
	}
}

func TestConversationTranscriptOwnership(t *testing.T) {
	cases := map[string]int{
		otherConv:                              http.StatusForbidden, // somebody else's
		guestConv:                              http.StatusForbidden, // never claimed by an account
		"not-a-uuid":                           http.StatusNotFound,
		"44444444-4444-4444-8444-444444444444": http.StatusNotFound,
	}
	for id, want := range cases {
		rec := httptest.NewRecorder()
		HandleConversationRoutes(historyHandler(), rec, historyRequest("/api/v1/conversations/"+id+"/messages", "user-1"))
		if rec.Code != want {
			t.Fatalf("%s: want %d, got %d", id, want, rec.Code)
		}
	}
}
