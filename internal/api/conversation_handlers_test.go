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

func (m *mockConversationUseCases) ConfirmAction(ctx context.Context, convID string, token string) (*conversation.ProcessConversationResponse, error) {
	m.confirmCalled = true
	return m.resp, m.err
}

func (m *mockConversationUseCases) RejectAction(ctx context.Context, convID string, token string) (*conversation.ProcessConversationResponse, error) {
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

	body := `{"channel":"web","message_text":"find iPhone","actor":{"id":"user-1","type":"customer"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/message", bytes.NewBufferString(body))
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
	if useCases.lastReq.Actor.ID != "user-1" {
		t.Errorf("expected actor ID user-1, got %s", useCases.lastReq.Actor.ID)
	}

	var resp conversation.ProcessConversationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ConversationID != "conv-1" {
		t.Errorf("expected conv-1, got %s", resp.ConversationID)
	}
	if resp.Response.Text != "Here are some results" {
		t.Errorf("unexpected text: %s", resp.Response.Text)
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
	w := httptest.NewRecorder()

	handler.HandleConfirmAction(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !useCases.confirmCalled {
		t.Error("expected ConfirmAction to be called")
	}
}

func TestConversationHandler_GenerateToken(t *testing.T) {
	handler := NewConversationHandler(&mockConversationUseCases{}, zerolog.Nop())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/conv-1/token", nil)
	w := httptest.NewRecorder()

	handler.HandleGenerateToken(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["token"] == nil || resp["token"] == "" {
		t.Error("expected non-empty token")
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

func TestGenerateToken(t *testing.T) {
	token := generateToken()
	if token == "" {
		t.Error("expected non-empty token")
	}
	if len(token) != 32 {
		t.Errorf("expected 32-char hex token, got %d chars", len(token))
	}

	token2 := generateToken()
	if token == token2 {
		t.Error("expected different tokens")
	}
}
