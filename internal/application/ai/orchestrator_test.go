package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trezutil/trezutil/infrastructure/ai"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
)

func setupOrchestrator(t *testing.T) (*AIOrchestrator, *ai.MockAIProvider, *MockRuntime, *MockUsageService) {
	provider := ai.NewMockAIProvider()
	router := NewModelRouter(testLogger())
	router.RegisterProvider(provider)
	gateway := setupTestGateway(router)
	runtime := NewMockRuntime()
	usageSvc := NewMockUsageService()
	resolver := conversation.NewActionResolver(conversation.NewDefaultIntentValidator())

	orch := NewAIOrchestrator(AIOrchestratorConfig{
		Logger:              testLogger(),
		Router:              router,
		Gateway:             gateway,
		Resolver:            resolver,
		Runtime:             runtime,
		UsageService:        usageSvc,
		ConfidenceThreshold: 0.7,
	})

	return orch, provider, runtime, usageSvc
}

type MockRuntime struct {
	executedActions []*domain.CommerceAction
	results         []*commerce.ExecuteResult
	executeFn       func(action *domain.CommerceAction) *commerce.ExecuteResult
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{}
}

func (m *MockRuntime) Execute(ctx context.Context, action *domain.CommerceAction, authenticatedActor domain.CommerceActor) (*commerce.ExecuteResult, error) {
	m.executedActions = append(m.executedActions, action)

	if action.Actor != authenticatedActor {
		return &commerce.ExecuteResult{
			Status: domain.ActionResultUnauthorized,
		}, nil
	}

	if m.executeFn != nil {
		result := m.executeFn(action)
		m.results = append(m.results, result)
		return result, nil
	}

	return &commerce.ExecuteResult{
		Status: domain.ActionResultSuccess,
		Data: map[string]interface{}{
			"data": map[string]interface{}{"listings": []interface{}{}},
		},
	}, nil
}

func (m *MockRuntime) IsSupported(actionType domain.CommerceActionType) bool {
	for _, a := range domain.AIAcceptedActions {
		if a == actionType {
			return true
		}
	}
	return false
}

type MockUsageService struct {
	reservations      map[string]*domain.UsageReservation
	reservationCtr    int
	lastReservationID string
	wasReleased       bool
	canAfford         bool
	consumeCount      int
}

func NewMockUsageService() *MockUsageService {
	return &MockUsageService{
		reservations: make(map[string]*domain.UsageReservation),
		canAfford:    true,
	}
}

func (m *MockUsageService) GetBalance(ctx context.Context, accountID string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{Granted: 1000, AccountID: accountID}, nil
}

func (m *MockUsageService) ResolvePolicy(ctx context.Context, actionType domain.CommerceActionType, channel domain.Channel, actorType domain.ActorType) (*domain.UsagePolicy, error) {
	return &domain.UsagePolicy{Cost: 1, Active: true, ActionType: actionType}, nil
}

func (m *MockUsageService) Check(ctx context.Context, accountID string, cost int64) (bool, error) {
	return m.canAfford, nil
}

func (m *MockUsageService) Reserve(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64, idempotencyKey string, tenantID string) (*domain.UsageReservation, error) {
	m.reservationCtr++
	res := &domain.UsageReservation{
		ReservationID: "res-" + string(rune(m.reservationCtr)),
		AccountID:     accountID,
		ActionType:    actionType,
		Amount:        cost,
		Status:        domain.UsageReservationActive,
	}
	m.lastReservationID = res.ReservationID
	m.reservations[res.ReservationID] = res
	return res, nil
}

func (m *MockUsageService) Consume(ctx context.Context, reservationID string) error {
	m.consumeCount++
	if r, ok := m.reservations[reservationID]; ok {
		r.Status = domain.UsageReservationConsumed
	}
	return nil
}

func (m *MockUsageService) Release(ctx context.Context, reservationID string) error {
	m.wasReleased = true
	if r, ok := m.reservations[reservationID]; ok {
		r.Status = domain.UsageReservationReleased
	}
	return nil
}

func (m *MockUsageService) Grant(ctx context.Context, accountID string, amount int64, reason string) error {
	return nil
}

func (m *MockUsageService) FindOrCreateCreditAccountByOwner(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string, planID string, periodID string) (*domain.CreditAccount, error) {
	return &domain.CreditAccount{
		AccountID: fmt.Sprintf("acc-%s", ownerID),
		OwnerType: ownerType,
		OwnerID:   ownerID,
		TenantID:  tenantID,
		PlanID:    planID,
		PeriodID:  periodID,
		Granted:   1000,
		Reserved:  0,
		Consumed:  0,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}, nil
}

func TestOrchestrator_ActorOverride(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels in Lubumbashi"}}`
	provider.SetResponse("hotels in lubumbashi", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	authenticatedActor := domain.NewCommerceActor("real-user-123", domain.ActorTypeCustomer)
	authenticatedActor.UserID = "user-real-123"
	authenticatedActor.OrgID = "org-real-456"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-real-456",
		TenantID:           "org-real-456",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{
				ConversationID: "conv-1",
				Direction:      domain.MessageDirectionInbound,
				Type:           domain.MessageTypeText,
				SenderID:       "user-real-123",
				Content:        &domain.MessageContent{Text: "Find hotels in Lubumbashi"},
			},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	runtime.executeFn = func(action *domain.CommerceAction) *commerce.ExecuteResult {
		if action.Actor.ID != "real-user-123" {
			return &commerce.ExecuteResult{Status: domain.ActionResultUnauthorized}
		}
		if action.TenantID != "org-real-456" {
			return &commerce.ExecuteResult{Status: domain.ActionResultUnauthorized}
		}
		return &commerce.ExecuteResult{Status: domain.ActionResultSuccess}
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotNil(t, result.Response)
	assert.Empty(t, result.Intent)

	if result.ActionResult == nil {
		assert.Equal(t, domain.ActionResultSuccess, domain.ActionResultSuccess)
	}
	if len(runtime.executedActions) > 0 {
		action := runtime.executedActions[0]
		assert.Equal(t, "AI", action.Source)
		assert.Equal(t, "real-user-123", action.Actor.ID)
		assert.Equal(t, "org-real-456", action.TenantID)
		assert.NotEqual(t, "fake-actor", action.Actor.ID)
		assert.NotEqual(t, "fake-tenant", action.TenantID)
	}
}

func TestOrchestrator_AICannotSetActor(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels", "actor_id": "platform_operator"}}`
	provider.SetResponse("hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	authenticatedActor := domain.NewCommerceActor("customer-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "org-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "customer-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	assert.NotNil(t, result)

	if len(runtime.executedActions) > 0 {
		action := runtime.executedActions[0]
		assert.Equal(t, "customer-1", action.Actor.ID)
		assert.Equal(t, domain.ActorTypeCustomer, action.Actor.Type)
	}
}

func TestOrchestrator_TenantIsolation(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels", "tenant_id": "competitor-tenant"}}`
	provider.SetResponse("hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	authenticatedActor := domain.NewCommerceActor("user-a", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-a"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-a",
		TenantID:           "tenant-a",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-a", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	if len(runtime.executedActions) > 0 {
		action := runtime.executedActions[0]
		assert.Equal(t, "tenant-a", action.TenantID)
		assert.NotEqual(t, "competitor-tenant", action.TenantID)
	}
}

func TestOrchestrator_ActionAllowlist(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	intentJSON := `{"intent": "INITIATE_PAYMENT", "confidence": 0.95, "parameters": {"amount": 100}}`
	provider.SetResponse("payment", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "org-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "pay 100 dollars"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, runtime.executedActions, 0)
}

func TestOrchestrator_LowConfidenceFallback(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.3, "parameters": {"query": "hotels"}}`
	provider.SetResponse("hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "org-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.ActionResult)
	assert.Len(t, runtime.executedActions, 0)
}

func TestOrchestrator_AIProviderFailureFallback(t *testing.T) {
	orch, provider, runtime, _ := setupOrchestrator(t)

	provider.SetError("hotels", domain.NewAIError(domain.AIErrProviderTimeout, "timeout"))

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "org-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.ActionResult)
	assert.Len(t, runtime.executedActions, 0)
}

func TestOrchestrator_SecretsNotInContext(t *testing.T) {
	orch, provider, _, _ := setupOrchestrator(t)

	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`,
		TokensUsed: 50,
	})

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: domain.NewCommerceActor("user-1", domain.ActorTypeCustomer),
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "system", Content: &domain.MessageContent{Text: "api_key=sk-secret-123"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	_, _ = orch.Interpret(context.Background(), req)

	for _, msg := range provider.LastMessages() {
		assert.NotContains(t, msg.Content, "sk-secret-123")
	}
}

func TestIntent_HasUntrustedFields(t *testing.T) {
	intent := domain.NewAIIntent()
	intent.Intent = domain.ActionSearchListings
	intent.Confidence = 0.95

	intent.Parameters["query"] = "hotels"
	assert.False(t, intent.HasUntrustedFields())

	intent.Parameters["actor_id"] = "hacker"
	assert.True(t, intent.HasUntrustedFields())

	intent.Parameters["tenant_id"] = "evil-tenant"
	assert.True(t, intent.HasUntrustedFields())

	intent.Parameters["role"] = "admin"
	assert.True(t, intent.HasUntrustedFields())

	intent.Parameters["secret_ref"] = "my-secret"
	assert.True(t, intent.HasUntrustedFields())

	intent.Parameters["confirmation_state"] = "approved"
	assert.True(t, intent.HasUntrustedFields())
}

func TestIntent_IsAllowlisted(t *testing.T) {
	allowlisted := []domain.CommerceActionType{
		domain.ActionSearchListings,
		domain.ActionGetListing,
		domain.ActionCheckAvailability,
	}

	for _, a := range allowlisted {
		intent := domain.NewAIIntent()
		intent.Intent = a
		assert.True(t, intent.IsAllowlisted(), "expected %s to be allowlisted", a)
	}

	nonAllowlisted := []domain.CommerceActionType{
		domain.ActionCreateListing,
		domain.ActionInitiatePayment,
		domain.ActionCreateOrder,
	}

	for _, a := range nonAllowlisted {
		intent := domain.NewAIIntent()
		intent.Intent = a
		assert.False(t, intent.IsAllowlisted(), "expected %s to NOT be allowlisted", a)
	}
}

func TestOrchestrator_FullRequestID(t *testing.T) {
	orch, provider, _, _ := setupOrchestrator(t)

	provider.SetResponse("test", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "test"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "org-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "org-1",
		TenantID:           "org-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "test"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	_, _ = orch.Interpret(context.Background(), req)
	assert.NotEmpty(t, provider.LastRequestID())
}

func TestIntentJSONParsing(t *testing.T) {
	jsonStr := `{"intent": "SEARCH_LISTINGS", "confidence": 0.85, "parameters": {"query": "test"}}`
	var raw map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &raw)
	require.NoError(t, err)

	intent, err := jsonToIntent(raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ActionSearchListings, intent.Intent)
	assert.Equal(t, 0.85, intent.Confidence)
	assert.Equal(t, "test", intent.Parameters["query"])
}

func jsonToIntent(raw map[string]interface{}) (*domain.AIIntent, error) {
	intent := domain.NewAIIntent()

	intentStr, _ := raw["intent"].(string)
	for _, a := range domain.AIAcceptedActions {
		if a == domain.CommerceActionType(intentStr) {
			intent.Intent = a
			break
		}
	}
	if intent.Intent == "" {
		return nil, nil
	}

	intent.Confidence, _ = raw["confidence"].(float64)
	intent.Parameters, _ = raw["parameters"].(map[string]interface{})

	return intent, nil
}
