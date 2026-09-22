package ai

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/ai"
	"github.com/trezutil/trezutil/internal/domain"
)

func testLogger() zerolog.Logger {
	return zerolog.Nop()
}

func setupTestRouter() *ModelRouter {
	router := NewModelRouter(testLogger())
	provider := ai.NewMockAIProvider()
	router.RegisterProvider(provider)
	return router
}

func setupTestGateway(router *ModelRouter) *AIGateway {
	return NewAIGateway(router, AIGatewayConfig{
		Logger:  testLogger(),
		Timeout: 5 * time.Second,
	})
}

func TestModelRouter_SelectsCorrectModel(t *testing.T) {
	router := setupTestRouter()

	model, provider, err := router.Route(context.Background(), "org1", domain.AITaskIntentClassification)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if model.ModelID != "mock-intent-classifier" {
		t.Fatalf("expected model mock-intent-classifier, got: %s", model.ModelID)
	}
	if provider.ID() != "mock" {
		t.Fatalf("expected provider mock, got: %s", provider.ID())
	}
}

func TestModelRouter_UnsupportedCapability(t *testing.T) {
	router := setupTestRouter()

	_, _, err := router.Route(context.Background(), "org1", domain.AITaskType("VISION_ANALYSIS"))
	if err == nil {
		t.Fatal("expected error for unsupported task, got nil")
	}
}

func TestModelRouter_InactiveProviderExcluded(t *testing.T) {
	router := setupTestRouter()
	router.UnregisterProvider("mock")

	_, _, err := router.Route(context.Background(), "org1", domain.AITaskIntentClassification)
	if err == nil {
		t.Fatal("expected error for no providers, got nil")
	}
}

func TestAIGateway_RequestValidation(t *testing.T) {
	router := setupTestRouter()
	gateway := setupTestGateway(router)

	_, err := gateway.Generate(context.Background(), "org1", domain.AIRequest{
		Task: "",
	})
	if err == nil {
		t.Fatal("expected error for empty task, got nil")
	}
}

func TestAIGateway_Success(t *testing.T) {
	router := setupTestRouter()
	provider := ai.NewMockAIProvider()

	intentResponse := `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotel in Lubumbashi"}}`
	provider.SetResponse("i want to find a hotel in lubumbashi", domain.AIResponse{
		Content:    intentResponse,
		TokensUsed: 150,
	})

	router.RegisterProvider(provider)
	gateway := setupTestGateway(router)

	resp, err := gateway.Generate(context.Background(), "org1", domain.AIRequest{
		Task: domain.AITaskIntentClassification,
		Messages: []domain.AIMessage{
			{Role: "system", Content: "Classify intent"},
			{Role: "user", Content: "I want to find a hotel in Lubumbashi"},
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.ProviderID != "mock" {
		t.Fatalf("expected provider mock, got: %s", resp.ProviderID)
	}
}
