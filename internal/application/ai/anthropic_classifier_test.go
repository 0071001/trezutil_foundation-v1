package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/ai/anthropic"
	"github.com/trezutil/trezutil/internal/infrastructure/ai/anthropic/anthropicfake"
)

// These tests run the real Anthropic provider (HTTP, SDK, schema, error
// mapping) against an emulated Messages API, through the same router,
// gateway, allowlist and usage metering as production.

func anthropicClassifier(t *testing.T, respond anthropicfake.Responder) (*IntentClassifierAdapter, *anthropicfake.Server, *MockUsageService) {
	t.Helper()
	srv := anthropicfake.New("sk-ant-test", respond)
	t.Cleanup(srv.Close)
	provider, err := anthropic.New(anthropic.Config{APIKey: "sk-ant-test", BaseURL: srv.URL, MaxRetries: 0})
	require.NoError(t, err)
	router := NewModelRouter(testLogger())
	router.RegisterProvider(provider)
	usage := NewMockUsageService()
	orch := NewAIOrchestrator(AIOrchestratorConfig{
		Logger:       testLogger(),
		Router:       router,
		Gateway:      NewAIGateway(router, AIGatewayConfig{Logger: testLogger(), Timeout: 2 * time.Second}),
		UsageService: usage,
	})
	return NewIntentClassifierAdapter(orch), srv, usage
}

func classificationRequest(text string) *conversation.IntentClassificationRequest {
	return &conversation.IntentClassificationRequest{
		ConversationID: "conv-1",
		OrganizationID: "org-1",
		TenantID:       "org-1",
		Actor:          domain.CommerceActor{ID: "user-1", Type: domain.ActorTypeCustomer},
		Channel:        domain.ChannelWeb,
		Messages:       []domain.Message{{Direction: domain.MessageDirectionInbound, Content: &domain.MessageContent{Text: text}}},
	}
}

func structuredReply(v map[string]interface{}) anthropicfake.Responder {
	return func(map[string]interface{}) anthropicfake.Reply {
		b, _ := json.Marshal(v)
		return anthropicfake.Reply{Text: string(b)}
	}
}

func TestAnthropicClassifier_NaturalLanguageSearch(t *testing.T) {
	classifier, srv, usage := anthropicClassifier(t, nil)

	res, err := classifier.ClassifyIntent(context.Background(), classificationRequest("I'm looking for a 2 bedroom apartment in Kinshasa"))
	require.NoError(t, err)
	require.Nil(t, res.Fallback)
	require.NotNil(t, res.Intent)
	assert.Equal(t, domain.ActionSearchListings, res.Intent.Intent)
	assert.Equal(t, "2 bedroom apartment Kinshasa", res.Intent.Parameters["text"])
	assert.Equal(t, anthropic.ProviderID, res.ProviderUsed)
	assert.Equal(t, 150, res.TokensUsed)
	assert.Equal(t, 1, usage.consumeCount, "a successful model call is metered")

	// The model is told which intents exist and gets no identity data.
	req := srv.Requests()[0]
	body, _ := json.Marshal(req)
	assert.Contains(t, string(body), "SEARCH_LISTINGS")
	assert.NotContains(t, string(body), "user-1")
	assert.NotContains(t, string(body), "org-1")
}

func TestAnthropicClassifier_RejectsWhatTheAllowlistForbids(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"write action":       {"intent": "CREATE_ORDER", "confidence": 0.99, "parameters": map[string]interface{}{}, "explanation": ""},
		"none":               {"intent": "NONE", "confidence": 0.9, "parameters": map[string]interface{}{}, "explanation": ""},
		"low confidence":     {"intent": "SEARCH_LISTINGS", "confidence": 0.2, "parameters": map[string]interface{}{"text": "x"}, "explanation": ""},
		"identity injection": {"intent": "GET_ORDER", "confidence": 0.99, "parameters": map[string]interface{}{"order_id": "o1", "tenant_id": "other-org"}, "explanation": ""},
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			classifier, _, _ := anthropicClassifier(t, structuredReply(reply))
			res, err := classifier.ClassifyIntent(context.Background(), classificationRequest("do it"))
			require.NoError(t, err)
			assert.Nil(t, res.Intent)
			assert.NotNil(t, res.Fallback, "the processor falls back to deterministic routing")
		})
	}
}

func TestAnthropicClassifier_ProviderFailureReleasesCredits(t *testing.T) {
	classifier, _, usage := anthropicClassifier(t, func(map[string]interface{}) anthropicfake.Reply {
		return anthropicfake.Error(http.StatusUnauthorized, "authentication_error", "invalid x-api-key")
	})
	res, err := classifier.ClassifyIntent(context.Background(), classificationRequest("find a car"))
	require.NoError(t, err)
	assert.NotNil(t, res.Fallback)
	assert.True(t, usage.wasReleased)
	assert.Equal(t, 0, usage.consumeCount)
}

func TestAnthropicClassifier_NoCreditsNoModelCall(t *testing.T) {
	classifier, srv, usage := anthropicClassifier(t, nil)
	usage.canAfford = false
	res, err := classifier.ClassifyIntent(context.Background(), classificationRequest("find a car"))
	require.NoError(t, err)
	assert.NotNil(t, res.Fallback)
	assert.Empty(t, srv.Requests(), "the provider must not be called without credits")
}

func TestAnthropicClassifier_GuestsAreNotMetered(t *testing.T) {
	classifier, _, usage := anthropicClassifier(t, nil)
	req := classificationRequest("find a car")
	req.Actor = domain.CommerceActor{Type: domain.ActorTypeGuest}
	res, err := classifier.ClassifyIntent(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res.Intent)
	assert.Equal(t, 0, usage.reservationCtr)
}
