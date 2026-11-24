package ai

import (
	"context"
	"time"

	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
)

// IntentClassifierAdapter adapts AIOrchestrator to the conversation.IntentClassifier interface.
// This bridges the AI gateway's intent classification to the ConversationProcessor.
type IntentClassifierAdapter struct {
	orchestrator *AIOrchestrator
	categories   *categoryCache
	rate         *rateCache
}

// AdapterOption configures an IntentClassifierAdapter.
type AdapterOption func(*IntentClassifierAdapter)

// WithCategories lets the interpreter name the category a person means.
func WithCategories(list CategoryLister) AdapterOption {
	return func(a *IntentClassifierAdapter) { a.categories = newCategoryCache(list) }
}

// WithExchangeRate lets the interpreter read a budget given in Congolese francs, at the rate an administrator set.
func WithExchangeRate(read RateReader) AdapterOption {
	return func(a *IntentClassifierAdapter) { a.rate = newRateCache(read) }
}

// NewIntentClassifierAdapter creates an adapter wrapping the AIOrchestrator.
func NewIntentClassifierAdapter(orch *AIOrchestrator, opts ...AdapterOption) *IntentClassifierAdapter {
	a := &IntentClassifierAdapter{orchestrator: orch}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// ClassifyIntent performs AI-only intent classification without executing the action.
func (a *IntentClassifierAdapter) ClassifyIntent(ctx context.Context, req *conversation.IntentClassificationRequest) (*conversation.IntentClassificationResult, error) {
	startTime := time.Now()

	result := &conversation.IntentClassificationResult{}

	aiReq := &AIInterpretRequest{
		ConversationID:     req.ConversationID,
		OrganizationID:     req.OrganizationID,
		TenantID:           req.TenantID,
		AuthenticatedActor: req.Actor,
		Messages:           req.Messages,
		Locale:             req.Locale,
		Channel:            req.Channel,
	}

	// Build sanitized message context
	sanitized, requestID, traceID := a.orchestrator.buildSanitizedContext(aiReq)

	// Model calls are metered when an AI_INTERPRETATION usage policy is
	// active. Without credits the conversation degrades to keyword routing.
	settle, ok := a.meter(ctx, req, requestID)
	if !ok {
		result.Fallback = a.orchestrator.buildFallbackResponse("You don't have enough credits to use AI features.")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	aiReq2 := domain.AIRequest{
		Task:             domain.AITaskIntentClassification,
		Messages:         sanitized.messages,
		SystemPrompt:     a.requestPrompt(ctx, req.Context, req.Memory...),
		StructuredOutput: true,
		OutputSchema:     intentOutputSchema(),
		RequestID:        requestID,
		TraceID:          traceID,
		MaxTokens:        4000,
		Temperature:      0.3,
		// Timeout left to the gateway (AI_TIMEOUT_SECONDS).
	}

	_ = req.Locale

	aiResp, err := a.orchestrator.gateway.Generate(ctx, req.OrganizationID, aiReq2)
	// The model call is billed once it produced an answer, even if the answer
	// is later rejected by the allowlist or the confidence threshold.
	settle(err == nil)
	if err != nil {
		result.Fallback = a.orchestrator.buildFallbackResponse("I'm having trouble understanding your request. Could you rephrase?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	result.ModelUsed = aiResp.ModelID
	result.ProviderUsed = aiResp.ProviderID
	result.TokensUsed = aiResp.TokensUsed

	intent, err := a.orchestrator.parseIntent(aiResp)
	if err != nil {
		result.Fallback = a.orchestrator.buildFallbackResponse("I couldn't understand the action you requested. Could you be more specific?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	if !intent.IsAllowlisted() || !intent.ConfidenceOK(a.orchestrator.confidenceThreshold) {
		result.Fallback = a.orchestrator.buildFallbackResponse("I'm not confident enough about what you'd like to do. Could you clarify your request?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	result.Intent = intent
	result.LatencyMs = time.Since(startTime).Milliseconds()
	return result, nil
}

// meter reserves AI credits for an authenticated actor when a usage policy
// applies. It returns a settle function (consume on true, release on false)
// and false when the actor cannot afford the call.
func (a *IntentClassifierAdapter) meter(ctx context.Context, req *conversation.IntentClassificationRequest, requestID string) (func(bool), bool) {
	return meterUsage(ctx, a.orchestrator.usageService, "AI_INTERPRETATION", req.Channel, req.Actor, req.TenantID, requestID)
}

// Ensure interface compliance
var _ conversation.IntentClassifier = (*IntentClassifierAdapter)(nil)
