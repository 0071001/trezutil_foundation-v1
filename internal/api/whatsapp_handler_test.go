package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
)

type handlerTestService struct {
	verifyErr   error
	processResp *domain.ConversationResponse
	processErr  error
}

func (f *handlerTestService) VerifyWebhook(body []byte, signature string) error {
	return f.verifyErr
}

func (f *handlerTestService) ProcessWebhook(ctx context.Context, channel domain.Channel, senderPhone string, conversationExternalID string, wabaID string, messages []domain.Message) (*domain.ConversationResponse, error) {
	return f.processResp, f.processErr
}

type storeForHandler struct {
	conversations map[string]*domain.Conversation
	created       []*domain.Conversation
	findErr       error
	createErr     error
}

func (m *storeForHandler) Create(ctx context.Context, conversation *domain.Conversation) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, conversation)
	m.conversations[conversation.ExternalID] = conversation
	return nil
}

func (m *storeForHandler) FindByID(ctx context.Context, id string) (*domain.Conversation, error) {
	return nil, nil
}

func (m *storeForHandler) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.Conversation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	c, ok := m.conversations[externalID]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (m *storeForHandler) Update(ctx context.Context, conversation *domain.Conversation) error {
	return nil
}

type userRepoForHandler struct {
	user *domain.User
	err  error
}

func (m *userRepoForHandler) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (m *userRepoForHandler) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}
func (m *userRepoForHandler) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *userRepoForHandler) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *userRepoForHandler) AssignRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *userRepoForHandler) RemoveRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *userRepoForHandler) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	return []domain.Role{}, nil
}
func (m *userRepoForHandler) HasRole(ctx context.Context, userID string, role domain.Role) (bool, error) {
	return false, nil
}

func makeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func newHandlerTest(t *testing.T) (*WhatsappHandler, *domain.ConversationResponse) {
	t.Helper()
	resp := domain.NewConversationResponse("", "en")
	svc := &handlerTestService{
		processResp: resp,
	}
	adapter := whatsappinfra.NewAdapter(whatsappinfra.Config{WebhookSecret: "test-secret"})
	h := NewWhatsappHandler(svc, adapter)
	return h, resp
}

func TestWebhookH_VerificationFails(t *testing.T) {
	svc := &handlerTestService{verifyErr: errors.New("invalid signature")}
	adapter := whatsappinfra.NewAdapter(whatsappinfra.Config{})
	h := NewWhatsappHandler(svc, adapter)

	body := []byte(`{"entry":[{"changes":[{"value":{"messages":[],"contacts":[],"conversations":[]}}]}]}`)
	wrongSig := makeHMAC("wrong-secret", body)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", wrongSig)
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestWebhookH_InvalidPayload(t *testing.T) {
	h, _ := newHandlerTest(t)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader([]byte("not json")))
	req.Header.Set("X-Hub-Signature-256", "sha256=valid")
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestWebhookH_ProcessingError(t *testing.T) {
	svc := &handlerTestService{verifyErr: nil, processErr: errors.New("processing failed")}
	adapter := whatsappinfra.NewAdapter(whatsappinfra.Config{WebhookSecret: "test-secret"})
	h := NewWhatsappHandler(svc, adapter)

	payload := whatsappinfra.ProviderPayload{
		Entry: []whatsappinfra.ProviderEntry{
			{Changes: []whatsappinfra.ProviderChange{
				{Value: whatsappinfra.ProviderValue{
					WaID: "waba-123",
					Messages: []whatsappinfra.ProviderMessage{
						{ID: "msg-1", From: "+243000000000", Timestamp: "2024-01-01T00:00:00Z", Type: "text", Text: &whatsappinfra.ProviderText{Body: "hello"}},
					},
				}},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	sig := makeHMAC("test-secret", body)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestWebhookH_Success(t *testing.T) {
	h, _ := newHandlerTest(t)

	payload := whatsappinfra.ProviderPayload{
		Entry: []whatsappinfra.ProviderEntry{
			{Changes: []whatsappinfra.ProviderChange{
				{Value: whatsappinfra.ProviderValue{
					WaID: "waba-123",
					Messages: []whatsappinfra.ProviderMessage{
						{ID: "msg-1", From: "+243000000000", Timestamp: "2024-01-01T00:00:00Z", Type: "text", Text: &whatsappinfra.ProviderText{Body: "hello"}},
					},
				}},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	sig := makeHMAC("test-secret", body)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWebhookH_EmptyMessages(t *testing.T) {
	h, _ := newHandlerTest(t)

	payload := whatsappinfra.ProviderPayload{
		Entry: []whatsappinfra.ProviderEntry{
			{Changes: []whatsappinfra.ProviderChange{
				{Value: whatsappinfra.ProviderValue{
					WaID:     "waba-123",
					Messages: []whatsappinfra.ProviderMessage{},
				}},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	sig := makeHMAC("test-secret", body)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookH_MultipleConversations(t *testing.T) {
	h, _ := newHandlerTest(t)

	payload := whatsappinfra.ProviderPayload{
		Entry: []whatsappinfra.ProviderEntry{
			{Changes: []whatsappinfra.ProviderChange{
				{Value: whatsappinfra.ProviderValue{
					WaID: "waba-123",
					Messages: []whatsappinfra.ProviderMessage{
						{ID: "msg-1", From: "+243000000000", Timestamp: "2024-01-01T00:00:00Z", Type: "text", Text: &whatsappinfra.ProviderText{Body: "hello"}},
					},
				}},
			}},
			{Changes: []whatsappinfra.ProviderChange{
				{Value: whatsappinfra.ProviderValue{
					WaID: "waba-123",
					Messages: []whatsappinfra.ProviderMessage{
						{ID: "msg-2", From: "+243000000001", Timestamp: "2024-01-01T00:00:01Z", Type: "text", Text: &whatsappinfra.ProviderText{Body: "hi"}},
					},
				}},
			}},
		},
	}
	body, _ := json.Marshal(payload)
	sig := makeHMAC("test-secret", body)

	req := httptest.NewRequest("POST", "/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()

	h.WebhookH(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAggregateConversationResponses_Single(t *testing.T) {
	r := domain.NewConversationResponse("hello", "en")
	got := aggregateConversationResponses([]*domain.ConversationResponse{r})
	if got.Text != "hello" {
		t.Errorf("expected 'hello', got '%s'", got.Text)
	}
}

func TestAggregateConversationResponses_Multiple(t *testing.T) {
	r1 := domain.NewConversationResponse("hello", "en")
	r2 := domain.NewConversationResponse("world", "fr")
	r2.Metadata = map[string]interface{}{"key": "value"}
	got := aggregateConversationResponses([]*domain.ConversationResponse{r1, r2})
	if got.Text != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", got.Text)
	}
	if got.Language != "en" {
		t.Errorf("expected 'en', got '%s'", got.Language)
	}
	if got.Metadata == nil || got.Metadata["key"] != "value" {
		t.Error("expected metadata from second response")
	}
}
