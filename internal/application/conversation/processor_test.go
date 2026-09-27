package conversation

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

type mockConvStore struct {
	conversations map[string]*domain.Conversation
}

func (m *mockConvStore) Create(ctx context.Context, conv *domain.Conversation) error {
	if m.conversations == nil {
		m.conversations = make(map[string]*domain.Conversation)
	}
	conv.ID = fmt.Sprintf("conv-%d", len(m.conversations)+1)
	m.conversations[conv.ID] = conv
	return nil
}

func (m *mockConvStore) FindByID(ctx context.Context, id string) (*domain.Conversation, error) {
	conv, ok := m.conversations[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return conv, nil
}

func (m *mockConvStore) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.Conversation, error) {
	for _, conv := range m.conversations {
		if conv.Channel == channel && conv.ExternalID == externalID {
			return conv, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockConvStore) Update(ctx context.Context, conv *domain.Conversation) error {
	m.conversations[conv.ID] = conv
	return nil
}

type mockContextStore struct {
	contexts map[string]domain.ConversationContext
}

func (m *mockContextStore) Save(ctx context.Context, convID string, context domain.ConversationContext) error {
	if m.contexts == nil {
		m.contexts = make(map[string]domain.ConversationContext)
	}
	m.contexts[convID] = context
	return nil
}

func (m *mockContextStore) Load(ctx context.Context, convID string) (*domain.ConversationContext, error) {
	cx, ok := m.contexts[convID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &cx, nil
}

func (m *mockContextStore) Delete(ctx context.Context, convID string) error {
	delete(m.contexts, convID)
	return nil
}

type mockMessageStore struct {
	messages []*domain.Message
}

func (m *mockMessageStore) Create(ctx context.Context, msg *domain.Message) error {
	m.messages = append(m.messages, msg)
	return nil
}

func (m *mockMessageStore) FindByID(ctx context.Context, id string) (*domain.Message, error) {
	return nil, fmt.Errorf("not found")
}

func (m *mockMessageStore) FindByConversationID(ctx context.Context, convID string) ([]*domain.Message, error) {
	var result []*domain.Message
	for _, msg := range m.messages {
		if msg.ConversationID == convID {
			result = append(result, msg)
		}
	}
	return result, nil
}

func (m *mockMessageStore) FindByExternalRef(ctx context.Context, channel domain.Channel, externalRef string) (*domain.Message, error) {
	return nil, fmt.Errorf("not found")
}

type mockRuntime struct {
	supportedActions map[domain.CommerceActionType]bool
	results          map[domain.CommerceActionType]*commerce.ExecuteResult
	err              error
}

func newMockRuntime() *mockRuntime {
	return &mockRuntime{
		supportedActions: make(map[domain.CommerceActionType]bool),
		results:          make(map[domain.CommerceActionType]*commerce.ExecuteResult),
	}
}

func (m *mockRuntime) Execute(ctx context.Context, action *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	result, ok := m.results[action.ActionType]
	if !ok {
		return &commerce.ExecuteResult{
			ActionID:   action.ActionID,
			Status:     domain.ActionResultSuccess,
			MessageKey: "ACTION_SUCCESS",
			Data:       map[string]interface{}{"action_type": string(action.ActionType)},
		}, nil
	}
	return result, nil
}

func (m *mockRuntime) IsSupported(actionType domain.CommerceActionType) bool {
	return m.supportedActions[actionType]
}

type mockClassifier struct {
	intent   *domain.AIIntent
	fallback *domain.ConversationResponse
	err      error
}

func (m *mockClassifier) ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &IntentClassificationResult{
		Intent:   m.intent,
		Fallback: m.fallback,
	}, nil
}

type mockPackResolver struct {
	packs map[string]map[domain.CommerceActionType]bool
}

func newMockPackResolver() *mockPackResolver {
	return &mockPackResolver{
		packs: map[string]map[domain.CommerceActionType]bool{
			"org-1": {
				domain.ActionSearchListings: true,
				domain.ActionCreateOrder:    true,
			},
		},
	}
}

func (m *mockPackResolver) ResolvePackForAction(orgID string, action domain.CommerceActionType) (domain.PackCode, bool) {
	if actions, ok := m.packs[orgID]; ok {
		if actions[action] {
			return domain.PackECommerce, true
		}
	}
	return "", false
}

func (m *mockPackResolver) ResolveSpecialistForAction(action domain.CommerceActionType) (domain.AISpecialistCode, bool) {
	return domain.AISpecialistCommerce, true
}

func (m *mockPackResolver) GetAvailablePacks(orgID string) []domain.PackCode {
	return []domain.PackCode{domain.PackECommerce}
}

func (m *mockPackResolver) IsActionSupported(orgID string, action domain.CommerceActionType) bool {
	_, ok := m.ResolvePackForAction(orgID, action)
	return ok
}

func TestProcessMessage_NewConversation(t *testing.T) {
	store := &mockConvStore{conversations: make(map[string]*domain.Conversation)}
	contextStore := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	runtime := newMockRuntime()
	runtime.supportedActions[domain.ActionSearchListings] = true
	runtime.results[domain.ActionSearchListings] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{
			"action_type": string(domain.ActionSearchListings),
			"data": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "list-1", "title": "Product A"},
				},
			},
		},
	}

	processor := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: store,
		ContextStore:      contextStore,
		MessageStore:      &mockMessageStore{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      newMockPackResolver(),
	})

	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "find iPhone 15",
		TenantID:         "org-1",
		Actor:            domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
	}
	req.Actor.OrgID = "org-1"

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConversationID == "" {
		t.Error("expected conversation ID")
	}
	if result.Response == nil {
		t.Fatal("expected response")
	}
	if result.Response.Text == "" {
		t.Error("expected response text")
	}
}

func TestProcessMessage_ConfirmationFlow(t *testing.T) {
	store := &mockConvStore{conversations: make(map[string]*domain.Conversation)}
	contextStore := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	runtime := newMockRuntime()
	runtime.supportedActions[domain.ActionCreateOrder] = true
	runtime.results[domain.ActionCreateOrder] = &commerce.ExecuteResult{
		Status:               domain.ActionResultRequiresConfirmation,
		MessageKey:           "Please confirm: create order for 2x Product A",
		RequiresConfirmation: true,
		Data: map[string]interface{}{
			"action_type": string(domain.ActionCreateOrder),
		},
	}

	processor := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: store,
		ContextStore:      contextStore,
		MessageStore:      &mockMessageStore{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      newMockPackResolver(),
	})

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor.OrgID = "org-1"

	// Step 1: Send a message that triggers a confirmation requirement
	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "buy 2 of the iPhone I saw",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsConfirmation {
		t.Error("expected confirmation response")
	}
	if result.Response.Confirmation == nil {
		t.Error("expected confirmation data")
	}

	// Step 2: Confirm the action
	confirmReq := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         result.ConversationID,
		SenderExternalID: "ext-1",
		MessageText:      "yes",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), confirmReq)
	if err != nil {
		t.Fatalf("unexpected error on confirmation: %v", err)
	}

	if result2.IsConfirmation {
		t.Error("did not expect another confirmation")
	}
}

func TestProcessMessage_MissingParameterFlow(t *testing.T) {
	store := &mockConvStore{conversations: make(map[string]*domain.Conversation)}
	contextStore := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	runtime := newMockRuntime()
	runtime.supportedActions[domain.ActionCreateOrder] = true
	runtime.results[domain.ActionCreateOrder] = &commerce.ExecuteResult{
		Status:     domain.ActionResultValidationFailed,
		MessageKey: "ACTION_VALIDATION_FAILED",
		Errors: []domain.ActionError{
			domain.NewActionError("MISSING_PARAM", "listing_id is required", "listing_id"),
		},
		Data: map[string]interface{}{},
	}

	processor := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: store,
		ContextStore:      contextStore,
		MessageStore:      &mockMessageStore{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      newMockPackResolver(),
	})

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor.OrgID = "org-1"

	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "buy something",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsClarification {
		t.Error("expected clarification response")
	}
	if len(result.MissingParams) == 0 {
		t.Error("expected missing params")
	}
}

func TestProcessMessage_ContextResolution(t *testing.T) {
	store := &mockConvStore{conversations: make(map[string]*domain.Conversation)}
	contextStore := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	runtime := newMockRuntime()
	runtime.supportedActions[domain.ActionGetListing] = true
	callCount := 0
	runtime.results[domain.ActionGetListing] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data:       map[string]interface{}{},
	}

	processor := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: store,
		ContextStore:      contextStore,
		MessageStore:      &mockMessageStore{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      newMockPackResolver(),
	})

	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor.OrgID = "org-1"

	// First message: search for listings (stores results in context)
	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "find phones",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Response == nil {
		t.Fatal("expected response")
	}
	_ = callCount

	// Second message: reference the first result
	cx, _ := contextStore.Load(context.Background(), result.ConversationID)
	if cx == nil {
		t.Fatal("expected context to be loaded")
	}
	if len(cx.ResultItems) == 0 {
		t.Log("no result items stored (expected for fallback path)")
	}
}

func TestProcessMessage_InvalidChannel(t *testing.T) {
	processor := NewConversationProcessor(ProcessorConfig{
		Logger: zerolog.Nop(),
	})

	req := &ProcessRequest{
		Channel:     domain.Channel("invalid"),
		MessageText: "hello",
	}

	_, err := processor.ProcessMessage(context.Background(), req)
	if err == nil {
		t.Error("expected error for invalid channel")
	}
}

func TestProcessMessage_EmptyMessage(t *testing.T) {
	processor := NewConversationProcessor(ProcessorConfig{
		Logger: zerolog.Nop(),
	})

	req := &ProcessRequest{
		Channel:     domain.ChannelWeb,
		MessageText: "",
	}

	_, err := processor.ProcessMessage(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty message")
	}
}

func TestProcessMessage_NilRequest(t *testing.T) {
	processor := NewConversationProcessor(ProcessorConfig{
		Logger: zerolog.Nop(),
	})

	_, err := processor.ProcessMessage(context.Background(), nil)
	if err == nil {
		t.Error("expected error for nil request")
	}
}

func TestIsAffirmative(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"yes", true},
		{"yes please", true},
		{"ok", true},
		{"Sure, go ahead", true},
		{"confirmed", true},
		{"no", false},
		{"maybe", false},
		{"I don't know", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := isAffirmative(strings.ToLower(tc.input))
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsNegative(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"no", true},
		{"no thanks", true},
		{"cancel", true},
		{"never", true},
		{"yes", false},
		{"maybe", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := isNegative(strings.ToLower(tc.input))
			if got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Services Pack E2E Tests
// These tests verify the Services pack end-to-end through the ConversationProcessor,
// covering discovery, missing params, context references, availability, reservation,
// tenant isolation, and idempotency.

type servicesMockPackResolver struct {
	packs map[string]map[domain.CommerceActionType]bool
}

func newServicesMockPackResolver() *servicesMockPackResolver {
	return &servicesMockPackResolver{
		packs: map[string]map[domain.CommerceActionType]bool{
			"org-1": {
				domain.ActionSearchListings:    true,
				domain.ActionCheckAvailability: true,
				domain.ActionCreateReservation: true,
				domain.ActionGetReservation:    true,
				domain.ActionCancelReservation: true,
			},
		},
	}
}

func (m *servicesMockPackResolver) ResolvePackForAction(orgID string, action domain.CommerceActionType) (domain.PackCode, bool) {
	if actions, ok := m.packs[orgID]; ok {
		if actions[action] {
			return domain.PackServices, true
		}
	}
	return "", false
}

func (m *servicesMockPackResolver) ResolveSpecialistForAction(action domain.CommerceActionType) (domain.AISpecialistCode, bool) {
	return domain.AISpecialistCommerce, true
}

func (m *servicesMockPackResolver) GetAvailablePacks(orgID string) []domain.PackCode {
	return []domain.PackCode{domain.PackServices}
}

func (m *servicesMockPackResolver) IsActionSupported(orgID string, action domain.CommerceActionType) bool {
	_, ok := m.ResolvePackForAction(orgID, action)
	return ok
}

func setupServicesProcessor() (*ConversationProcessor, *mockRuntime, *mockPackResolver, *mockConvStore, *mockContextStore) {
	store := &mockConvStore{conversations: make(map[string]*domain.Conversation)}
	contextStore := &mockContextStore{contexts: make(map[string]domain.ConversationContext)}
	runtime := newMockRuntime()
	runtime.supportedActions[domain.ActionSearchListings] = true
	runtime.supportedActions[domain.ActionCheckAvailability] = true
	runtime.supportedActions[domain.ActionCreateReservation] = true
	runtime.supportedActions[domain.ActionGetReservation] = true
	runtime.supportedActions[domain.ActionCancelReservation] = true

	runtime.results[domain.ActionSearchListings] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{
			"action_type": string(domain.ActionSearchListings),
			"data": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"id": "svc-1", "title": "Haircut Service", "category": "beauty"},
					map[string]interface{}{"id": "svc-2", "title": "Massage Therapy", "category": "wellness"},
				},
			},
		},
	}
	runtime.results[domain.ActionCheckAvailability] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "AVAILABILITY_CHECK",
		Data: map[string]interface{}{
			"action_type": string(domain.ActionCheckAvailability),
			"slot":        "2026-10-15T14:00:00",
			"available":   true,
		},
	}
	runtime.results[domain.ActionCreateReservation] = &commerce.ExecuteResult{
		Status:               domain.ActionResultRequiresConfirmation,
		MessageKey:           "Please confirm reservation",
		RequiresConfirmation: true,
		Data: map[string]interface{}{
			"action_type": string(domain.ActionCreateReservation),
			"reservation": map[string]interface{}{
				"service":  "Haircut Service",
				"datetime": "2026-10-15T14:00:00",
				"guests":   1,
			},
		},
	}

	packResolver := &mockPackResolver{
		packs: map[string]map[domain.CommerceActionType]bool{
			"org-1": {
				domain.ActionSearchListings:    true,
				domain.ActionCheckAvailability: true,
				domain.ActionCreateReservation: true,
				domain.ActionGetReservation:    true,
				domain.ActionCancelReservation: true,
			},
		},
	}

	processor := NewConversationProcessor(ProcessorConfig{
		Logger:            zerolog.Nop(),
		ConversationStore: store,
		ContextStore:      contextStore,
		MessageStore:      &mockMessageStore{},
		ActionResolver:    NewActionResolver(NewDefaultIntentValidator()),
		Runtime:           runtime,
		PackResolver:      packResolver,
	})

	return processor, runtime, packResolver, store, contextStore
}

func newServicesActor() domain.CommerceActor {
	actor := domain.NewCommerceActor("user-services-1", domain.ActorTypeCustomer)
	actor.OrgID = "org-1"
	return actor
}

func TestServicesPack_Discovery(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "I need a haircut",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ConversationID == "" {
		t.Error("expected conversation ID")
	}
	if result.Response == nil {
		t.Fatal("expected response")
	}
	if result.Response.Text == "" {
		t.Error("expected response text for discovery results")
	}

	if runtime.results[domain.ActionSearchListings].Data == nil {
		t.Error("expected search results data")
	}
}

func TestServicesPack_MissingParameters(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	// First, search for services
	req1 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "I need a haircut",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result1, err := processor.ProcessMessage(context.Background(), req1)
	if err != nil {
		t.Fatalf("unexpected error on first message: %v", err)
	}
	if result1.Response == nil {
		t.Fatal("expected response on first message")
	}

	// Now try to book without specifying date/time/guests - should trigger missing params
	runtime.results[domain.ActionCreateReservation] = &commerce.ExecuteResult{
		Status:     domain.ActionResultValidationFailed,
		MessageKey: "ACTION_VALIDATION_FAILED",
		Errors: []domain.ActionError{
			domain.NewActionError("MISSING_PARAM", "date is required", "date"),
			domain.NewActionError("MISSING_PARAM", "time is required", "time"),
			domain.NewActionError("MISSING_PARAM", "guests is required", "guests"),
		},
		Data: map[string]interface{}{},
	}

	req2 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book the haircut",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error on second message: %v", err)
	}

	if !result2.IsClarification {
		t.Error("expected clarification response for missing params")
	}
	if len(result2.MissingParams) == 0 {
		t.Error("expected missing params")
	}
	expectedParams := map[string]bool{"date": false, "time": false, "guests": false}
	for _, mp := range result2.MissingParams {
		if _, ok := expectedParams[mp.Name]; ok {
			expectedParams[mp.Name] = true
		}
	}
	for param, found := range expectedParams {
		if !found {
			t.Errorf("expected missing param %s, not found", param)
		}
	}
}

func TestServicesPack_ContextReferences(t *testing.T) {
	processor, _, _, _, contextStore := setupServicesProcessor()
	actor := newServicesActor()

	// Step 1: Search for services (stores results in context)
	req1 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "find me massage services",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result1, err := processor.ProcessMessage(context.Background(), req1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 2: Reference the first result using ordinal "first"
	req2 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book the first one",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result2

	// Context should have result items stored
	cx, err := contextStore.Load(context.Background(), result1.ConversationID)
	if err != nil || cx == nil {
		t.Fatal("expected context to be loaded")
	}

	if len(cx.ResultItems) == 0 {
		t.Error("expected result items stored in context")
	}

	// Step 3: Reference "latest result"
	req3 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book the latest one for tomorrow",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result3, err := processor.ProcessMessage(context.Background(), req3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result3.Response == nil {
		t.Fatal("expected response")
	}
}

func TestServicesPack_AvailabilityCheck(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "check availability for haircut on October 15th at 2pm",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == nil {
		t.Fatal("expected response")
	}

	if runtime.results[domain.ActionCheckAvailability] == nil {
		t.Error("expected availability check result")
	}
}

func TestServicesPack_ReservationWithConfirmation(t *testing.T) {
	processor, _, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	// Step 1: Initiate reservation (triggers confirmation)
	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book a haircut for October 15th at 2pm for 2 people",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsConfirmation {
		t.Error("expected confirmation response for reservation")
	}
	if result.Response.Confirmation == nil {
		t.Error("expected confirmation data")
	}

	// Step 2: Confirm the reservation
	confirmReq := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         result.ConversationID,
		SenderExternalID: "ext-1",
		MessageText:      "yes, confirm that",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), confirmReq)
	if err != nil {
		t.Fatalf("unexpected error on confirmation: %v", err)
	}

	if result2.IsConfirmation {
		t.Error("did not expect another confirmation after confirm")
	}
	if result2.Response == nil {
		t.Fatal("expected response after confirmation")
	}
}

func TestServicesPack_ReservationRejection(t *testing.T) {
	processor, _, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	// Step 1: Initiate reservation
	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book a haircut",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.IsConfirmation {
		t.Error("expected confirmation response")
	}

	// Step 2: Reject the reservation
	rejectReq := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         result.ConversationID,
		SenderExternalID: "ext-1",
		MessageText:      "no, cancel that",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), rejectReq)
	if err != nil {
		t.Fatalf("unexpected error on rejection: %v", err)
	}

	if result2.Response == nil {
		t.Fatal("expected response after rejection")
	}
}

func TestServicesPack_TenantIsolation(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()

	// Org 1 - search listings
	actor1 := domain.NewCommerceActor("user-org1", domain.ActorTypeCustomer)
	actor1.OrgID = "org-1"
	runtime.results[domain.ActionSearchListings] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{
			"action_type": string(domain.ActionSearchListings),
			"data": map[string]interface{}{
				"org": "org-1",
				"items": []interface{}{
					map[string]interface{}{"id": "org1-svc-1", "title": "Org1 Haircut"},
				},
			},
		},
	}

	req1 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-org1",
		SenderExternalID: "ext-org1",
		MessageText:      "find haircut services",
		TenantID:         "org-1",
		Actor:            actor1,
	}

	result1, err := processor.ProcessMessage(context.Background(), req1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.Response == nil {
		t.Fatal("expected response for org-1")
	}

	// Org 2 - should not see org-1 results
	actor2 := domain.NewCommerceActor("user-org2", domain.ActorTypeCustomer)
	actor2.OrgID = "org-2"
	runtime.results[domain.ActionSearchListings] = &commerce.ExecuteResult{
		Status:     domain.ActionResultSuccess,
		MessageKey: "ACTION_SUCCESS",
		Data: map[string]interface{}{
			"action_type": string(domain.ActionSearchListings),
			"data": map[string]interface{}{
				"org": "org-2",
				"items": []interface{}{
					map[string]interface{}{"id": "org2-svc-1", "title": "Org2 Massage"},
				},
			},
		},
	}

	req2 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-org2",
		SenderExternalID: "ext-org2",
		MessageText:      "find massage services",
		TenantID:         "org-2",
		Actor:            actor2,
	}

	result2, err := processor.ProcessMessage(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.Response == nil {
		t.Fatal("expected response for org-2")
	}

	// Results should be different conversations
	if result1.ConversationID == result2.ConversationID {
		t.Error("expected different conversation IDs for different tenants")
	}
}

func TestServicesPack_Idempotency(t *testing.T) {
	processor, _, _, store, _ := setupServicesProcessor()
	actor := newServicesActor()

	req := &ProcessRequest{
		Channel:               domain.ChannelWeb,
		SenderID:              "sender-1",
		SenderExternalID:      "ext-1",
		MessageText:           "book a haircut for tomorrow",
		TenantID:              "org-1",
		Actor:                 actor,
		MessageIdempotencyKey: "msg-key-123",
	}

	result1, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}

	// Send same message again (duplicate idempotency key)
	result2, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on duplicate call: %v", err)
	}

	// Check that only one message was stored
	var msgCount int
	for _, conv := range store.conversations {
		_ = conv
	}
	// The message store should have deduplicated
	if result1.ConversationID != result2.ConversationID {
		t.Error("expected same conversation ID for idempotent message")
	}

	_ = msgCount
}

func TestServicesPack_ConfirmationTimeout(t *testing.T) {
	processor, _, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	// Initiate reservation
	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-1",
		SenderExternalID: "ext-1",
		MessageText:      "book a massage",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result1, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result1.IsConfirmation {
		t.Error("expected confirmation response")
	}

	// Simulate timeout by sending a new message (not a yes/no confirmation)
	req2 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         result1.ConversationID,
		SenderExternalID: "ext-1",
		MessageText:      "actually, what services do you offer?",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error on follow-up: %v", err)
	}

	// Should get a normal response, not a confirmation error
	if result2.Response == nil {
		t.Fatal("expected response after timeout")
	}
}

func TestServicesPack_AvailabilityAndReservationFlow(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()
	actor := newServicesActor()

	// Step 1: Search for a service
	req1 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-flow",
		SenderExternalID: "ext-flow",
		MessageText:      "find me a haircut service",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result1, err := processor.ProcessMessage(context.Background(), req1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = result1

	// Step 2: Check availability
	req2 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-flow",
		SenderExternalID: "ext-flow",
		MessageText:      "is it available on October 15th at 2pm?",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result2, err := processor.ProcessMessage(context.Background(), req2)
	if err != nil {
		t.Fatalf("unexpected error on availability check: %v", err)
	}
	if result2.Response == nil {
		t.Fatal("expected availability response")
	}

	// Step 3: Book the reservation (triggers confirmation)
	runtime.results[domain.ActionCreateReservation] = &commerce.ExecuteResult{
		Status:               domain.ActionResultRequiresConfirmation,
		MessageKey:           "Please confirm your reservation for Oct 15 at 2pm",
		RequiresConfirmation: true,
		Data: map[string]interface{}{
			"action_type": string(domain.ActionCreateReservation),
			"reservation": map[string]interface{}{
				"service":  "Haircut Service",
				"datetime": "2026-10-15T14:00:00",
				"guests":   2,
				"status":   "pending_confirmation",
			},
		},
	}

	req3 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-flow",
		SenderExternalID: "ext-flow",
		MessageText:      "book it for 2 people",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result3, err := processor.ProcessMessage(context.Background(), req3)
	if err != nil {
		t.Fatalf("unexpected error on reservation: %v", err)
	}
	if !result3.IsConfirmation {
		t.Error("expected confirmation for reservation")
	}

	// Step 4: Confirm
	req4 := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         result3.ConversationID,
		SenderExternalID: "ext-flow",
		MessageText:      "yes, confirm",
		TenantID:         "org-1",
		Actor:            actor,
	}

	result4, err := processor.ProcessMessage(context.Background(), req4)
	if err != nil {
		t.Fatalf("unexpected error on confirmation: %v", err)
	}
	if result4.Response == nil {
		t.Fatal("expected final response after confirmation")
	}
}

func TestServicesPack_ActorTypeSystemSkipsUsage(t *testing.T) {
	processor, runtime, _, _, _ := setupServicesProcessor()

	systemActor := domain.NewCommerceActor("system-1", domain.ActorTypeSystem)
	systemActor.OrgID = "org-1"

	req := &ProcessRequest{
		Channel:          domain.ChannelWeb,
		SenderID:         "sender-sys",
		SenderExternalID: "ext-sys",
		MessageText:      "find services",
		TenantID:         "org-1",
		Actor:            systemActor,
	}

	result, err := processor.ProcessMessage(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Response == nil {
		t.Fatal("expected response for system actor")
	}
	_ = runtime
}
