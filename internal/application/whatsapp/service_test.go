package whatsapp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
)

type mockStore struct {
	conversations map[string]*domain.Conversation
	created       []*domain.Conversation
	findErr       error
	createErr     error
}

func (m *mockStore) Create(ctx context.Context, conversation *domain.Conversation) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, conversation)
	m.conversations[conversation.ExternalID] = conversation
	return nil
}

func (m *mockStore) FindByID(ctx context.Context, id string) (*domain.Conversation, error) {
	return nil, nil
}

func (m *mockStore) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.Conversation, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	c, ok := m.conversations[externalID]
	if !ok {
		return nil, nil
	}
	return c, nil
}

func (m *mockStore) Update(ctx context.Context, conversation *domain.Conversation) error {
	return nil
}

type mockUserRepo struct {
	user *domain.User
	err  error
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error) {
	return nil, nil
}
func (m *mockUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.user, nil
}
func (m *mockUserRepo) Create(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) Update(ctx context.Context, user *domain.User) error { return nil }
func (m *mockUserRepo) AssignRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *mockUserRepo) RemoveRole(ctx context.Context, userID string, role domain.Role) error {
	return nil
}
func (m *mockUserRepo) GetUserRoles(ctx context.Context, userID string) ([]domain.Role, error) {
	return []domain.Role{}, nil
}
func (m *mockUserRepo) HasRole(ctx context.Context, userID string, role domain.Role) (bool, error) {
	return false, nil
}

type mockChannelAccountRepo struct {
	account *domain.ChannelAccount
	err     error
}

func (m *mockChannelAccountRepo) Create(ctx context.Context, account *domain.ChannelAccount) error {
	return nil
}
func (m *mockChannelAccountRepo) FindByID(ctx context.Context, id string) (*domain.ChannelAccount, error) {
	return nil, nil
}
func (m *mockChannelAccountRepo) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.ChannelAccount, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.account != nil && m.account.ExternalID == externalID && m.account.Channel == channel {
		return m.account, nil
	}
	return nil, nil
}
func (m *mockChannelAccountRepo) Update(ctx context.Context, account *domain.ChannelAccount) error {
	return nil
}

type mockMessageStore struct {
	messages map[string]*domain.Message
	err      error
}

func (m *mockMessageStore) Create(ctx context.Context, message *domain.Message) error {
	if m.err != nil {
		return m.err
	}
	m.messages[message.IdempotencyKey] = message
	return nil
}
func (m *mockMessageStore) FindByID(ctx context.Context, id string) (*domain.Message, error) {
	return nil, nil
}
func (m *mockMessageStore) FindByConversationID(ctx context.Context, conversationID string) ([]*domain.Message, error) {
	return nil, nil
}
func (m *mockMessageStore) FindByExternalRef(ctx context.Context, channel domain.Channel, externalRef string) (*domain.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, msg := range m.messages {
		if msg.ExternalRef == externalRef {
			return msg, nil
		}
	}
	return nil, nil
}

type mockWebhookEventStore struct {
	claimed bool
	err     error
}

func (m *mockWebhookEventStore) TryClaim(ctx context.Context, channel domain.Channel, idempotencyKey string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	if m.claimed {
		return false, nil
	}
	m.claimed = true
	return true, nil
}
func (m *mockWebhookEventStore) MarkProcessed(ctx context.Context, channel domain.Channel, idempotencyKey string) error {
	return nil
}
func (m *mockWebhookEventStore) MarkFailed(ctx context.Context, channel domain.Channel, idempotencyKey, errorMsg string) error {
	return nil
}

type mockListingUseCases struct{}

func (m *mockListingUseCases) CreateListing(ctx context.Context, actorID string, listing *domain.Listing, attributes []*domain.AttributeValue, media []*domain.ListingMedia, availability *domain.ListingAvailability, locations []*domain.ListingLocation) error {
	return nil
}

func (m *mockListingUseCases) GetListing(ctx context.Context, actorID string, listingID string) (*domain.Listing, error) {
	return nil, nil
}

func (m *mockListingUseCases) PublishListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

func (m *mockListingUseCases) PauseListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

func (m *mockListingUseCases) ResumeListing(ctx context.Context, actorID string, listingID string) error {
	return nil
}

type mockSearchUseCases struct{}

func (m *mockSearchUseCases) ExecuteSearch(ctx context.Context, actorID string, query domain.DiscoveryQuery) (domain.SearchResultPage, error) {
	return domain.SearchResultPage{}, nil
}

func (m *mockSearchUseCases) Suggest(ctx context.Context, actorID string, req domain.SuggestRequest) (domain.SuggestResponse, error) {
	return domain.SuggestResponse{}, nil
}

type mockReservationUseCases struct{}

func (m *mockReservationUseCases) CreateReservation(ctx context.Context, actorID string, listingID string, startAt int64, endAt int64, quantity int, partySize int, idempotencyKey string) (*domain.Reservation, error) {
	return nil, nil
}

func (m *mockReservationUseCases) GetReservation(ctx context.Context, actorID string, reservationID string) (*domain.Reservation, error) {
	return nil, nil
}

func (m *mockReservationUseCases) CancelReservation(ctx context.Context, actorID string, reservationID string) error {
	return nil
}

type mockAvailabilityUseCases struct{}

func (m *mockAvailabilityUseCases) CheckAvailability(ctx context.Context, actorID string, query domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	return domain.AvailabilityResult{}, nil
}

func newTestService(t *testing.T) (*Service, *mockStore, *mockUserRepo, *mockChannelAccountRepo, *mockMessageStore, *mockWebhookEventStore, *mockListingUseCases) {
	t.Helper()
	store := &mockStore{conversations: make(map[string]*domain.Conversation)}
	userRepo := &mockUserRepo{}
	channelAccountRepo := &mockChannelAccountRepo{account: &domain.ChannelAccount{
		ID:             "chan-1",
		OrganizationID: "org-1",
		Channel:        domain.ChannelWhatsApp,
		ExternalID:     "waba-123",
		Status:         domain.ChannelAccountStatusActive,
	}}
	messageStore := &mockMessageStore{messages: make(map[string]*domain.Message)}
	webhookEventStore := &mockWebhookEventStore{}
	listingUseCases := &mockListingUseCases{}
	searchUseCases := &mockSearchUseCases{}
	reservationUseCases := &mockReservationUseCases{}
	availabilityUseCases := &mockAvailabilityUseCases{}
	registry := commerce.NewHandlerRegistry()
	runtime := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    registry,
		IdempotencyStore:   commerce.NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: commerce.NewDefaultConfirmationPolicy(),
		ParameterValidator: commerce.NewDefaultParameterValidator(),
	})
	adapter := whatsappinfra.NewAdapter(whatsappinfra.Config{})
	presenter := whatsappinfra.NewPresenter()
	svc := NewService(store, userRepo, channelAccountRepo, messageStore, webhookEventStore, listingUseCases, searchUseCases, reservationUseCases, availabilityUseCases, runtime, adapter, presenter)
	return svc, store, userRepo, channelAccountRepo, messageStore, webhookEventStore, listingUseCases
}

func makeHMAC(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestService_VerifyWebhook_Success(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	secret := "test-secret"
	svc.adapter = whatsappinfra.NewAdapter(whatsappinfra.Config{WebhookSecret: secret})

	body := []byte(`{"test": true}`)
	mac := makeHMAC(secret, body)

	if err := svc.VerifyWebhook(body, mac); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestService_VerifyWebhook_BadSignature(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	svc.adapter = whatsappinfra.NewAdapter(whatsappinfra.Config{WebhookSecret: "test-secret"})

	body := []byte(`{"test": true}`)
	if err := svc.VerifyWebhook(body, "invalid"); err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestService_VerifyWebhook_NoSecret(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	body := []byte(`{"test": true}`)
	if err := svc.VerifyWebhook(body, ""); err == nil {
		t.Fatal("expected error when no secret configured")
	}
}

func TestService_ResolveTenant_Success(t *testing.T) {
	svc, _, _, car, _, _, _ := newTestService(t)
	car.account = &domain.ChannelAccount{
		ID:             "chan-1",
		OrganizationID: "org-1",
		Channel:        domain.ChannelWhatsApp,
		ExternalID:     "waba-123",
		Status:         domain.ChannelAccountStatusActive,
	}

	tenantID, err := svc.ResolveTenant(context.Background(), domain.ChannelWhatsApp, "waba-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tenantID != "org-1" {
		t.Errorf("expected tenant org-1, got %s", tenantID)
	}
}

func TestService_ResolveTenant_NotFound(t *testing.T) {
	svc, _, _, car, _, _, _ := newTestService(t)
	car.account = nil

	_, err := svc.ResolveTenant(context.Background(), domain.ChannelWhatsApp, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown channel account")
	}
}

func TestService_ProcessWebhook_UnknownSender(t *testing.T) {
	svc, _, userRepo, _, _, _, _ := newTestService(t)
	userRepo.user = nil

	_, err := svc.ProcessWebhook(context.Background(), domain.ChannelWhatsApp, "+243000000000", "conv-1", "waba-123", nil)
	if err == nil {
		t.Fatal("expected error for unknown sender")
	}
}

func TestService_ProcessWebhook_CreatesConversation(t *testing.T) {
	svc, store, userRepo, _, _, _, _ := newTestService(t)
	userRepo.user = &domain.User{ID: "user-1", Phone: "+243000000000", DisplayName: "Test", Status: domain.UserStatusActive}

	messages := []domain.Message{
		{
			ID:             "msg-1",
			Type:           domain.MessageTypeText,
			Direction:      domain.MessageDirectionInbound,
			SenderID:       "+243000000000",
			Content:        &domain.MessageContent{Text: "hello", Data: make(map[string]interface{})},
			ExternalRef:    "msg-1",
			IdempotencyKey: "msg-1",
			CreatedAt:      1700000000,
		},
	}

	resp, err := svc.ProcessWebhook(context.Background(), domain.ChannelWhatsApp, "+243000000000", "conv-1", "waba-123", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if len(store.created) != 1 {
		t.Fatalf("expected 1 created conversation, got %d", len(store.created))
	}
	if store.created[0].CustomerID != "user-1" {
		t.Errorf("expected customer ID 'user-1', got '%s'", store.created[0].CustomerID)
	}
	if store.created[0].OrganizationID != "org-1" {
		t.Errorf("expected organization ID 'org-1', got '%s'", store.created[0].OrganizationID)
	}
}

func TestService_ProcessWebhook_DuplicateIdempotency(t *testing.T) {
	svc, _, userRepo, _, _, wes, _ := newTestService(t)
	userRepo.user = &domain.User{ID: "user-1", Phone: "+243000000000", DisplayName: "Test", Status: domain.UserStatusActive}
	wes.claimed = true

	messages := []domain.Message{
		{
			ID:             "msg-1",
			Type:           domain.MessageTypeText,
			Direction:      domain.MessageDirectionInbound,
			SenderID:       "+243000000000",
			Content:        &domain.MessageContent{Text: "hello", Data: make(map[string]interface{})},
			ExternalRef:    "msg-1",
			IdempotencyKey: "msg-1",
			CreatedAt:      1700000000,
		},
	}

	resp, err := svc.ProcessWebhook(context.Background(), domain.ChannelWhatsApp, "+243000000000", "conv-1", "waba-123", messages)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.Text != "Already processed" {
		t.Errorf("expected 'Already processed', got '%s'", resp.Text)
	}
}

func TestService_ResolveActionType_Search(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	if got := svc.resolveActionType("search for products"); got != domain.ActionSearchListings {
		t.Errorf("expected ActionSearchListings, got %s", got)
	}
}

func TestService_ResolveActionType_Order(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	if got := svc.resolveActionType("buy now"); got != domain.ActionCreateOrder {
		t.Errorf("expected ActionCreateOrder, got %s", got)
	}
}

func TestService_ResolveActionType_Empty(t *testing.T) {
	svc, _, _, _, _, _, _ := newTestService(t)
	if got := svc.resolveActionType("random text"); got != "" {
		t.Errorf("expected empty action type, got %s", got)
	}
}
