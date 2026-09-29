package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type mockConversationUseCases struct {
	lastReq       *conversation.ProcessConversationRequest
	resp          *conversation.ProcessConversationResponse
	err           error
	confirmCalled bool
	rejectCalled  bool
}

func (m *mockConversationUseCases) ProcessMessage(ctx context.Context, req *conversation.ProcessConversationRequest) (*conversation.ProcessConversationResponse, error) {
	m.lastReq = req
	return m.resp, m.err
}

func (m *mockConversationUseCases) ConfirmAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*conversation.ProcessConversationResponse, error) {
	m.confirmCalled = true
	return m.resp, m.err
}

func (m *mockConversationUseCases) RejectAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*conversation.ProcessConversationResponse, error) {
	m.rejectCalled = true
	return m.resp, m.err
}

func TestConversationHandler_ProcessMessage(t *testing.T) {
	useCases := &mockConversationUseCases{
		resp: &conversation.ProcessConversationResponse{
			ConversationID: "conv-1",
			Response: &domain.ConversationResponse{
				Text:     "Here are some results",
				Language: "en",
			},
		},
	}

	handler := NewConversationHandler(useCases, zerolog.Nop())

	// The body claims to be a platform operator; only the verified principal counts.
	body := `{"channel":"web","message_text":"find iPhone","actor":{"id":"attacker","type":"platform_operator"},"organization_id":"org-x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/message", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: "user-1"}))
	w := httptest.NewRecorder()

	handler.HandleProcessMessage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	if useCases.lastReq == nil {
		t.Fatal("expected ProcessMessage to be called")
	}
	if useCases.lastReq.Channel != domain.ChannelWeb {
		t.Errorf("expected channel web, got %s", useCases.lastReq.Channel)
	}
	if useCases.lastReq.MessageText != "find iPhone" {
		t.Errorf("expected 'find iPhone', got %s", useCases.lastReq.MessageText)
	}
	if useCases.lastReq.Actor.ID != "user-1" || useCases.lastReq.Actor.Type != domain.ActorTypeCustomer {
		t.Errorf("expected verified customer user-1, got %s/%s", useCases.lastReq.Actor.ID, useCases.lastReq.Actor.Type)
	}
	if useCases.lastReq.TenantID != "" || useCases.lastReq.Actor.OrgID != "" {
		t.Errorf("client-supplied organization must be ignored, got tenant=%q org=%q", useCases.lastReq.TenantID, useCases.lastReq.Actor.OrgID)
	}

	responseBody := w.Body.Bytes()
	var resp conversation.ProcessConversationResponse
	if err := json.Unmarshal(responseBody, &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ConversationID != "conv-1" {
		t.Errorf("expected conv-1, got %s", resp.ConversationID)
	}
	if resp.Response.Text != "Here are some results" {
		t.Errorf("unexpected text: %s", resp.Response.Text)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		t.Fatalf("failed to decode JSON contract: %v", err)
	}
	if _, ok := payload["conversation_id"]; !ok {
		t.Fatal("response is missing conversation_id")
	}
	var responsePayload map[string]json.RawMessage
	if err := json.Unmarshal(payload["response"], &responsePayload); err != nil {
		t.Fatalf("failed to decode conversation response: %v", err)
	}
	if _, ok := responsePayload["text"]; !ok {
		t.Fatal("conversation response is missing text")
	}
}

func TestConversationHandler_ProcessMessage_MissingText(t *testing.T) {
	handler := NewConversationHandler(&mockConversationUseCases{}, zerolog.Nop())

	body := `{"channel":"web"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.HandleProcessMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConversationHandler_ProcessMessage_InvalidChannel(t *testing.T) {
	handler := NewConversationHandler(&mockConversationUseCases{}, zerolog.Nop())

	body := `{"channel":"invalid","message_text":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/message", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.HandleProcessMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestConversationHandler_ConfirmAction(t *testing.T) {
	useCases := &mockConversationUseCases{
		resp: &conversation.ProcessConversationResponse{
			ConversationID: "conv-1",
			Response: &domain.ConversationResponse{
				Text: "Order confirmed!",
			},
		},
	}

	handler := NewConversationHandler(useCases, zerolog.Nop())

	body := `{"token":"test-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/conv-1/confirm", bytes.NewBufferString(body))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: "user-1"}))
	w := httptest.NewRecorder()

	handler.HandleConfirmAction(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !useCases.confirmCalled {
		t.Error("expected ConfirmAction to be called")
	}
}

func TestConversationHandler_ConfirmRequiresAuthentication(t *testing.T) {
	useCases := &mockConversationUseCases{}
	handler := NewConversationHandler(useCases, zerolog.Nop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/conv-1/confirm", bytes.NewBufferString(`{"token":"t"}`))
	w := httptest.NewRecorder()
	handler.HandleConfirmAction(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for anonymous confirmation, got %d", w.Code)
	}
	if useCases.confirmCalled {
		t.Error("ConfirmAction must not run for anonymous callers")
	}
}

func TestHandleConversationRoutes_NotFound(t *testing.T) {
	handler := NewConversationHandler(&mockConversationUseCases{}, zerolog.Nop())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/unknown", nil)
	w := httptest.NewRecorder()

	HandleConversationRoutes(handler, w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestExtractIDFromPath(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		suffix string
		want   string
	}{
		{"/api/v1/conversations/conv-123/confirm", "/api/v1/conversations/", "/confirm", "conv-123"},
		{"/api/v1/conversations/conv-123/reject", "/api/v1/conversations/", "/reject", "conv-123"},
		{"/api/v1/unknown", "/api/v1/conversations/", "/confirm", ""},
	}

	for _, tc := range tests {
		got := extractIDFromPath(tc.path, tc.prefix, tc.suffix)
		if got != tc.want {
			t.Errorf("extractIDFromPath(%q, %q, %q) = %q, want %q", tc.path, tc.prefix, tc.suffix, got, tc.want)
		}
	}
}
