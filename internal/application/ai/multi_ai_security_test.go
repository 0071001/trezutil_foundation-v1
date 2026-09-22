package ai

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/trezutil/trezutil/infrastructure/ai"
	"github.com/trezutil/trezutil/internal/domain"
)

func setupSecurityTestEnv(t *testing.T) (*SpecializedModelRouter, *ai.MockAIProvider, *MockRuntime) {
	t.Helper()
	router := NewModelRouter(testLogger())
	provider := ai.NewMockAIProvider()
	router.RegisterProvider(provider)

	specRouter := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	for _, spec := range domain.DefaultSpecialists {
		gateway := NewAIGateway(router, AIGatewayConfig{
			Logger:  testLogger(),
			Timeout: 5 * time.Second,
		})
		specRouter.RegisterSpecialist(spec, gateway)
	}

	sr := NewSpecializedModelRouter(SpecializedModelRouterConfig{
		Logger:      testLogger(),
		ModelRouter: router,
		SpecRouter:  specRouter,
	})

	runtime := NewMockRuntime()
	return sr, provider, runtime
}

func makeSecurityRequest(userContent string) domain.AIRequest {
	return domain.AIRequest{
		Task: domain.AITaskIntentClassification,
		Messages: []domain.AIMessage{
			{Role: "system", Content: "Classify intent"},
			{Role: "user", Content: userContent},
		},
	}
}

func TestSecurity_AI_CannotInjectTenantID(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels", "tenant_id": "evil-tenant"}}`
	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("find hotels")
	resp, _, err := sr.Generate(context.Background(), "real-secret-org", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.True(t, intent.HasUntrustedFields(), "intent should contain untrusted sensitive fields")
}

func TestSecurity_AI_CannotInjectActorID(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels", "actor_id": "platform_admin"}}`
	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("find hotels")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.True(t, intent.HasUntrustedFields(), "intent should reject actor_id")
}

func TestSecurity_AI_CannotInjectRole(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels", "role": "admin"}}`
	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("find hotels")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.True(t, intent.HasUntrustedFields(), "intent should reject role field")
}

func TestSecurity_AI_CannotInjectPermissions(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "GET_ORDER", "confidence": 0.95, "parameters": {"order_id": "ORD-1", "permissions": ["admin", "superuser"]}}`
	provider.SetResponse("show my order", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("show my order")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.True(t, intent.HasUntrustedFields(), "intent should reject permissions field")
}

func TestSecurity_AI_ConfirmationStateInjection(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "GET_ORDER", "confidence": 0.95, "parameters": {"order_id": "ORD-1", "confirmation_state": "approved"}}`
	provider.SetResponse("show my order", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("show my order")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	if intent == nil {
		t.Fatal("intent should be parsed")
	}
	assert.True(t, intent.HasUntrustedFields(), "intent should reject confirmation_state")
}

func TestSecurity_AI_SecretRefInjection(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "test", "secret_ref": "my-api-key"}}`
	provider.SetResponse("test query", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("test query")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.True(t, intent.HasUntrustedFields(), "intent should reject secret_ref")
}

func TestSecurity_AI_NoSensitiveFieldsInCleanResponse(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`
	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("find hotels")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.False(t, intent.HasUntrustedFields(), "clean intent should not have untrusted fields")
}

func TestSecurity_AI_AllowlistedActionsOnly(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	for _, action := range domain.AIAcceptedActions {
		provider.SetResponse(string(action), domain.AIResponse{
			Content:    `{"intent": "` + string(action) + `", "confidence": 0.95, "parameters": {"test": "value"}}`,
			TokensUsed: 50,
		})

		req := domain.AIRequest{
			Task: domain.AITaskIntentClassification,
			Messages: []domain.AIMessage{
				{Role: "system", Content: "Classify intent"},
				{Role: "user", Content: string(action)},
			},
		}

		resp, _, err := sr.Generate(context.Background(), "org-1", req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		intent, perr := parseIntentResponse(resp.Content)
		assert.NoError(t, perr)
		if intent == nil {
			t.Fatalf("intent should be parsed for action %s", action)
		}
		assert.True(t, intent.IsAllowlisted(), "action %s should be in allowlist", action)
	}
}

func TestSecurity_AI_NonAllowlistedActionRejected(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	// CREATE_LISTING is NOT in AIAcceptedActions
	intentJSON := `{"intent": "CREATE_LISTING", "confidence": 0.95, "parameters": {"listing": "test"}}`
	provider.SetResponse("create listing", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := makeSecurityRequest("create listing")
	resp, _, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// Since CREATE_LISTING is not in the allowlist, parseIntentResponse will return nil intent
	intent, perr := parseIntentResponse(resp.Content)
	assert.NoError(t, perr)
	assert.Nil(t, intent, "CREATE_LISTING should not produce a valid intent")
}

func TestSecurity_SpecialistHandoffCannotEscalate(t *testing.T) {
	sr, _, _ := setupSecurityTestEnv(t)

	assert.False(t, sr.CanHandoff(domain.AISpecialistLogistics, domain.AISpecialistMarketing),
		"Logistics should not be able to handoff to Marketing")

	assert.True(t, sr.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistBooking),
		"Commerce should be able to handoff to Booking")
}

func TestSecurity_SystemPromptContainsSensitiveFieldWarning(t *testing.T) {
	sr, _ := setupSpecializedRouter(t)

	instr := sr.SystemInstructionsFor(domain.AISpecialistCommerce)
	assert.NotNil(t, instr)
	assert.NotEmpty(t, instr.SystemPrompt)
	assert.Contains(t, instr.SystemPrompt, "tenant_id")
	assert.Contains(t, instr.SystemPrompt, "actor_id")
}

func TestSecurity_SpecializedModelRouter_OpIDMasking(t *testing.T) {
	sr, provider := setupSpecializedRouter(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "test"}}`
	provider.SetResponse("test", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	originalOrgID := "real-secret-org"

	req := domain.AIRequest{
		Task: domain.AITaskIntentClassification,
		Messages: []domain.AIMessage{
			{Role: "system", Content: "Classify intent"},
			{Role: "user", Content: "test"},
		},
		TraceID: originalOrgID,
	}

	resp, _, err := sr.Generate(context.Background(), originalOrgID, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotContains(t, resp.ProviderID, originalOrgID)
}

func TestSecurity_MultiSpecialist_TenantIsolation(t *testing.T) {
	sr, provider, _ := setupSecurityTestEnv(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`
	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	for _, orgID := range []string{"org-a", "org-b", "org-c"} {
		req := makeSecurityRequest("find hotels")
		resp, specCode, err := sr.Generate(context.Background(), orgID, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, domain.AISpecialistCommerce, specCode)
		assert.NotContains(t, resp.ProviderID, orgID,
			"org ID should not be leaked in provider ID")
	}
}

func TestSecurity_MultiSpecialist_HandoffPreservesContext(t *testing.T) {
	sr, _, _ := setupSecurityTestEnv(t)

	ok := sr.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistBooking)
	assert.True(t, ok, "Commerce should be able to handoff to Booking")

	ok = sr.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistMarketing)
	assert.True(t, ok, "Commerce should be able to handoff to Marketing")

	ok = sr.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistSupport)
	assert.True(t, ok, "Commerce should be able to handoff to Support")
}

func TestSecurity_MultiSpecialist_NoPrivilegeEscalation(t *testing.T) {
	sr, _, _ := setupSecurityTestEnv(t)

	ok := sr.CanHandoff(domain.AISpecialistLogistics, domain.AISpecialistMarketing)
	assert.False(t, ok, "Logistics should NOT be able to handoff to Marketing")

	ok = sr.CanHandoff(domain.AISpecialistMarketing, domain.AISpecialistBooking)
	assert.False(t, ok, "Marketing should NOT be able to handoff to Booking")
}

func parseIntentResponse(content string) (*domain.AIIntent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, err
	}

	intent := domain.NewAIIntent()
	intentStr, _ := raw["intent"].(string)
	for _, a := range domain.AIAcceptedActions {
		if string(a) == intentStr {
			intent.Intent = a
			break
		}
	}
	if intent.Intent == "" {
		return nil, nil
	}

	intent.Confidence, _ = raw["confidence"].(float64)
	intent.Parameters, _ = raw["parameters"].(map[string]interface{})
	intent.Explanation, _ = raw["explanation"].(string)

	return intent, nil
}
