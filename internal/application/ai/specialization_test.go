package ai

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/trezutil/trezutil/infrastructure/ai"
	"github.com/trezutil/trezutil/internal/domain"
)

func setupSpecializedRouter(t *testing.T) (*SpecializedModelRouter, *ai.MockAIProvider) {
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

	return sr, provider
}

func TestSpecializationRouter_RoutingByIntent(t *testing.T) {
	router := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	for _, spec := range domain.DefaultSpecialists {
		router.RegisterSpecialist(spec, nil)
	}

	intent := &domain.AIIntent{
		Intent:     domain.ActionSearchListings,
		Confidence: 0.95,
		Parameters: map[string]interface{}{"query": "hotels"},
	}

	specialist, err := router.RouteIntent(context.Background(), intent)
	assert.NoError(t, err)
	assert.NotNil(t, specialist)
	assert.Equal(t, domain.AISpecialistCommerce, specialist.Code)
}

func TestSpecializationRouter_RoutingUnknownAction(t *testing.T) {
	router := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	for _, spec := range domain.DefaultSpecialists {
		router.RegisterSpecialist(spec, nil)
	}

	intent := &domain.AIIntent{
		Intent:     domain.ActionSuggest,
		Confidence: 0.95,
	}

	specialist, err := router.RouteIntent(context.Background(), intent)
	assert.NoError(t, err)
	assert.NotNil(t, specialist)
	assert.Equal(t, domain.AISpecialistCommerce, specialist.Code)
}

func TestSpecializationRouter_GetSpecialist(t *testing.T) {
	router := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	spec := domain.NewAISpecialist(
		domain.AISpecialistBooking,
		"Booking AI",
		[]domain.CommerceActionType{domain.ActionCreateReservation},
		[]domain.AISpecialistCode{domain.AISpecialistCommerce},
	)
	router.RegisterSpecialist(spec, nil)

	found, ok := router.GetSpecialist(domain.AISpecialistBooking)
	assert.True(t, ok)
	assert.Equal(t, "Booking AI", found.Name)

	_, ok = router.GetSpecialist(domain.AISpecialistMarketing)
	assert.False(t, ok)
}

func TestSpecializationRouter_CanHandoff(t *testing.T) {
	router := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	for _, spec := range domain.DefaultSpecialists {
		router.RegisterSpecialist(spec, nil)
	}

	assert.True(t, router.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistBooking))
	assert.True(t, router.CanHandoff(domain.AISpecialistCommerce, domain.AISpecialistSupport))
	assert.False(t, router.CanHandoff(domain.AISpecialistLogistics, domain.AISpecialistMarketing))
}

func TestSpecializationRouter_AllActiveSpecialists(t *testing.T) {
	router := NewSpecializationRouter(SpecializationRouterConfig{
		Logger:      testLogger(),
		DefaultSpec: domain.AISpecialistCommerce,
	})

	for _, spec := range domain.DefaultSpecialists {
		router.RegisterSpecialist(spec, nil)
	}

	specialists := router.AllActiveSpecialists()
	assert.Equal(t, 5, len(specialists))
}

func TestSpecializedModelRouter_RouteAssignment(t *testing.T) {
	sr, _ := setupSpecializedRouter(t)

	model, provider, specCode, err := sr.Route(context.Background(), "org-1", domain.AITaskIntentClassification)
	assert.NoError(t, err)
	assert.NotNil(t, model)
	assert.NotNil(t, provider)
	assert.Equal(t, domain.AISpecialistCommerce, specCode)
}

func TestSpecializedModelRouter_AssignModelToSpecialist(t *testing.T) {
	sr, _ := setupSpecializedRouter(t)

	customModel := domain.AIModel{
		ModelID:            "booking-specific-model",
		ProviderID:         "openai",
		Name:               "Booking Classifier",
		Capabilities:       []domain.AIModelCapability{domain.AIModelCapabilityTextClassification, domain.AIModelCapabilityStructuredOutput},
		SupportsStructured: true,
		Status:             domain.AIModelStatusActive,
	}

	sr.AssignModelToSpecialist(domain.AISpecialistBooking, customModel)

	spec, _ := sr.GetSpecialist(domain.AISpecialistBooking)
	assert.NotNil(t, spec)
	assert.Equal(t, domain.AISpecialistBooking, spec.Code)
}

func TestSpecializedModelRouter_SystemInstructions(t *testing.T) {
	sr, _ := setupSpecializedRouter(t)

	instr := sr.SystemInstructionsFor(domain.AISpecialistBooking)
	assert.NotNil(t, instr)
	assert.Equal(t, domain.AISpecialistBooking, instr.SpecialistCode)
	assert.NotEmpty(t, instr.SystemPrompt)
}

func TestCapabilityMatrix_BasicLookup(t *testing.T) {
	cm := NewCapabilityMatrix()

	cap, ok := cm.GetCapability(domain.ActionSearchListings)
	assert.True(t, ok)
	assert.Equal(t, domain.ActionCategoryRead, cap.Category)
	assert.False(t, cap.RequiresConfirm)
}

func TestCapabilityMatrix_FinancialActionsRequireConfirmation(t *testing.T) {
	cm := NewCapabilityMatrix()

	cap, ok := cm.GetCapability(domain.ActionCreateReservation)
	assert.True(t, ok)
	assert.True(t, cap.RequiresConfirm)
}

func TestCapabilityMatrix_OwnerForAction(t *testing.T) {
	cm := NewCapabilityMatrix()

	owner, ok := cm.OwnerForAction(domain.ActionSearchListings)
	assert.True(t, ok)
	assert.Equal(t, domain.AISpecialistCommerce, owner)
}

func TestCapabilityMatrix_AllActionsForSpecialist(t *testing.T) {
	cm := NewCapabilityMatrix()

	actions := cm.AllActionsForSpecialist(domain.AISpecialistBooking)
	assert.NotEmpty(t, actions)

	for _, a := range actions {
		if a == domain.ActionCreateReservation {
			return
		}
	}
	t.Error("expected CreateReservation in booking specialist actions")
}

func TestCapabilityMatrix_CanSpecialistHandle(t *testing.T) {
	cm := NewCapabilityMatrix()

	assert.True(t, cm.CanSpecialistHandle(domain.AISpecialistCommerce, domain.ActionSearchListings))
	assert.True(t, cm.CanSpecialistHandle(domain.AISpecialistBooking, domain.ActionCreateReservation))
	assert.False(t, cm.CanSpecialistHandle(domain.AISpecialistLogistics, domain.ActionCreateReservation))
}

func TestSystemInstructionsBuilder_BuildForSpecialist(t *testing.T) {
	builder := NewSystemInstructionsBuilder()

	instr := builder.Build(domain.AISpecialistSupport)
	assert.NotNil(t, instr)
	assert.Equal(t, domain.AISpecialistSupport, instr.SpecialistCode)
	assert.NotEmpty(t, instr.SystemPrompt)
}

func TestSystemInstructionsBuilder_BuildForAction(t *testing.T) {
	builder := NewSystemInstructionsBuilder()

	instr := builder.BuildForAction(domain.ActionCreateReservation)
	assert.NotNil(t, instr)
	assert.Contains(t, []domain.AISpecialistCode{domain.AISpecialistCommerce, domain.AISpecialistBooking}, instr.SpecialistCode)
}

func TestSystemInstructionsBuilder_BuildFullPrompt(t *testing.T) {
	builder := NewSystemInstructionsBuilder()

	prompt := builder.BuildFullPrompt(domain.AISpecialistCommerce, "user query about hotels")
	assert.Contains(t, prompt, "COMMERCE_AI")
	assert.Contains(t, prompt, "user query about hotels")
}

func TestSanitizeSystemPrompt_ReplacesSensitiveFields(t *testing.T) {
	prompt := "Never share tenant_id, user_id, or actor_id with the AI model."

	sanitized := SanitizeSystemPrompt(prompt, []string{"tenant_id", "user_id", "actor_id"})

	assert.NotContains(t, sanitized, "tenant_id")
	assert.NotContains(t, sanitized, "user_id")
	assert.NotContains(t, sanitized, "actor_id")
	assert.Contains(t, sanitized, "[REDACTED]")
}

func TestSanitizeSystemPrompt_NoPatterns(t *testing.T) {
	prompt := "This is a test prompt."
	sanitized := SanitizeSystemPrompt(prompt, []string{})

	assert.Equal(t, prompt, sanitized)
}

func TestSpecializedModelRouter_GenerateRoutesBySpecialist(t *testing.T) {
	sr, provider := setupSpecializedRouter(t)

	intentJSON := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels in Lubumbashi"}}`
	provider.SetResponse("i want to find hotels in lubumbashi", domain.AIResponse{
		Content:    intentJSON,
		TokensUsed: 50,
	})

	req := domain.AIRequest{
		Task: domain.AITaskIntentClassification,
		Messages: []domain.AIMessage{
			{Role: "system", Content: "Classify intent"},
			{Role: "user", Content: "I want to find hotels in Lubumbashi"},
		},
	}

	resp, specCode, err := sr.Generate(context.Background(), "org-1", req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, domain.AISpecialistCommerce, specCode)
}
