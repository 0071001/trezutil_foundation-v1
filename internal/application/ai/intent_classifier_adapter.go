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
}

// NewIntentClassifierAdapter creates an adapter wrapping the AIOrchestrator.
func NewIntentClassifierAdapter(orch *AIOrchestrator) *IntentClassifierAdapter {
	return &IntentClassifierAdapter{orchestrator: orch}
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
	sanitized, _, _ := a.orchestrator.buildSanitizedContext(aiReq)

	aiReq2 := domain.AIRequest{
		Task:             domain.AITaskIntentClassification,
		Messages:         sanitized.messages,
		SystemPrompt:     a.orchestrator.buildSystemPrompt(),
		StructuredOutput: true,
		OutputSchema:     intentOutputSchema(),
		MaxTokens:        4000,
		Temperature:      0.3,
		Timeout:          30 * time.Second,
	}

	_ = req.Locale

	aiResp, err := a.orchestrator.gateway.Generate(ctx, req.OrganizationID, aiReq2)
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

// Ensure interface compliance
var _ conversation.IntentClassifier = (*IntentClassifierAdapter)(nil)
