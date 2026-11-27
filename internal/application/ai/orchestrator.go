package ai

import (
	"context"
	"errors"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"github.com/trezutil/trezutil/internal/domain"
)

type CommerceActionExecutor interface {
	Execute(ctx context.Context, action *domain.CommerceAction, authenticatedActor domain.CommerceActor) (*commerce.ExecuteResult, error)
	IsSupported(actionType domain.CommerceActionType) bool
}

const (
	DefaultConfidenceThreshold = 0.7
	DefaultMaxMessages         = 20
	DefaultMaxTokensPerMessage = 500
	DefaultTokenBudget         = 16000
	SensitiveFieldToken        = "token"
)

type AIOrchestratorConfig struct {
	Logger              zerolog.Logger
	Router              *ModelRouter
	Gateway             *AIGateway
	Resolver            *conversation.ActionResolver
	Runtime             CommerceActionExecutor
	UsageService        domain.UsageService
	ConfidenceThreshold float64
	MaxMessages         int
	TokenBudget         int
}

type AIOrchestrator struct {
	logger              zerolog.Logger
	router              *ModelRouter
	gateway             *AIGateway
	resolver            *conversation.ActionResolver
	runtime             CommerceActionExecutor
	usageService        domain.UsageService
	confidenceThreshold float64
	maxMessages         int
	tokenBudget         int
}

func NewAIOrchestrator(config AIOrchestratorConfig) *AIOrchestrator {
	threshold := config.ConfidenceThreshold
	if threshold <= 0 {
		threshold = DefaultConfidenceThreshold
	}
	maxMessages := config.MaxMessages
	if maxMessages <= 0 {
		maxMessages = DefaultMaxMessages
	}
	tokenBudget := config.TokenBudget
	if tokenBudget <= 0 {
		tokenBudget = DefaultTokenBudget
	}

	return &AIOrchestrator{
		logger:              config.Logger,
		router:              config.Router,
		gateway:             config.Gateway,
		resolver:            config.Resolver,
		runtime:             config.Runtime,
		usageService:        config.UsageService,
		confidenceThreshold: threshold,
		maxMessages:         maxMessages,
		tokenBudget:         tokenBudget,
	}
}

type AIOrchestratedResult struct {
	Intent            *domain.AIIntent
	Action            *domain.CommerceAction
	ActionResult      *commerce.ExecuteResult
	Response          *domain.ConversationResponse
	NeedsConfirmation bool
	Confirmation      *domain.ConfirmationData
	ModelUsed         string
	ProviderUsed      string
	TokensUsed        int
	LatencyMs         int64
}

type AIInterpretRequest struct {
	ConversationID     string
	OrganizationID     string
	TenantID           string
	AuthenticatedActor domain.CommerceActor
	Messages           []domain.Message
	Locale             string
	Channel            domain.Channel
}

func (o *AIOrchestrator) Interpret(ctx context.Context, req *AIInterpretRequest) (*AIOrchestratedResult, error) {
	startTime := time.Now()

	result := &AIOrchestratedResult{}

	sanitized, requestID, traceID := o.buildSanitizedContext(req)

	var reservationID string
	if o.usageService != nil {
		policy, err := o.usageService.ResolvePolicy(ctx, "AI_INTERPRETATION", req.Channel, req.AuthenticatedActor.Type)
		if err != nil && !errors.Is(err, domain.ErrUsagePolicyNotFound) {
			result.Response = o.buildFallbackResponse("AI features are unavailable right now. Please try again.")
			result.LatencyMs = time.Since(startTime).Milliseconds()
			return result, nil
		}
		if err == nil && policy.IsMetered() {
			ok, _ := o.usageService.Check(ctx, req.AuthenticatedActor.ID, policy.Cost)
			if !ok {
				result.Response = o.buildFallbackResponse("You don't have enough credits to use AI features.")
				result.LatencyMs = time.Since(startTime).Milliseconds()
				return result, nil
			}
			res, rerr := o.usageService.Reserve(ctx, req.AuthenticatedActor.ID, "AI_INTERPRETATION", policy.Cost, requestID, req.TenantID)
			if rerr == nil {
				reservationID = res.ReservationID
			}
		}
	}

	releaseCredits := func() {
		if reservationID != "" && o.usageService != nil {
			_ = o.usageService.Release(ctx, reservationID)
		}
	}

	consumeCredits := func() {
		if reservationID != "" && o.usageService != nil {
			_ = o.usageService.MarkAction(context.WithoutCancel(ctx), reservationID, domain.UsageActionSucceeded)
			_ = o.usageService.Consume(context.WithoutCancel(ctx), reservationID)
		}
	}

	aiReq := domain.AIRequest{
		Task:             domain.AITaskIntentClassification,
		Messages:         sanitized.messages,
		SystemPrompt:     o.buildSystemPrompt(),
		StructuredOutput: true,
		OutputSchema:     intentOutputSchema(),
		RequestID:        requestID,
		TraceID:          traceID,
		MaxTokens:        4000,
		Temperature:      0.3,
		Timeout:          30 * time.Second,
	}

	aiResp, err := o.gateway.Generate(ctx, req.OrganizationID, aiReq)
	if err != nil {
		releaseCredits()
		result.Response = o.buildFallbackResponse("I'm having trouble understanding your request. Could you rephrase?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	result.ModelUsed = aiResp.ModelID
	result.ProviderUsed = aiResp.ProviderID
	result.TokensUsed = aiResp.TokensUsed

	intent, err := o.parseIntent(aiResp)
	if err != nil {
		releaseCredits()
		result.Response = o.buildFallbackResponse("I couldn't understand the action you requested. Could you be more specific?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	result.Intent = intent

	if !intent.IsAllowlisted() {
		releaseCredits()
		result.Response = o.buildFallbackResponse("I can help you search for listings, check availability, and view your orders and reservations.")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	if !intent.ConfidenceOK(o.confidenceThreshold) {
		releaseCredits()
		result.Response = o.buildFallbackResponse("I'm not confident enough about what you'd like to do. Could you clarify your request?")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	action, err := o.resolver.Resolve(ctx,
		domain.ConversationIntent{
			Name:       "AI_CLASSIFIED_INTENT",
			ActionType: intent.Intent,
			Confidence: intent.Confidence,
			Source:     "AI",
			Entities:   o.entitiesFromParams(intent.Parameters),
		},
		req.AuthenticatedActor,
		req.ConversationID,
		req.TenantID,
	)
	if err != nil {
		result.Response = o.buildFallbackResponse("I couldn't build the requested action. Please try again.")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}
	action.WithSource("AI")

	result.Action = action

	execResult, err := o.runtime.Execute(ctx, action, req.AuthenticatedActor)
	if err != nil {
		releaseCredits()
		result.Response = o.buildFallbackResponse("Something went wrong processing your request.")
		result.LatencyMs = time.Since(startTime).Milliseconds()
		return result, nil
	}

	if execResult.Status == domain.ActionResultSuccess {
		consumeCredits()
	} else {
		releaseCredits()
	}

	result.ActionResult = execResult

	if execResult.RequiresConfirmation {
		result.NeedsConfirmation = true
		result.Confirmation = &domain.ConfirmationData{
			Summary: "Please confirm this action",
			Details: map[string]interface{}{
				"action_type": string(action.ActionType),
			},
		}
		result.Response = o.buildConfirmationResponse(action, execResult)
	} else if execResult.Status == domain.ActionResultSuccess {
		result.Response = o.buildSuccessResponse(action, execResult)
	} else if execResult.Status == domain.ActionResultRequiresConfirmation {
		result.NeedsConfirmation = true
		result.Confirmation = &domain.ConfirmationData{
			Summary: action.ConfirmationData.Summary,
		}
		result.Response = o.buildConfirmationResponse(action, execResult)
	} else {
		result.Response = o.buildErrorResponse(execResult)
	}

	result.LatencyMs = time.Since(startTime).Milliseconds()
	return result, nil
}

type sanitizedContext struct {
	messages []domain.AIMessage
}

func (o *AIOrchestrator) buildSanitizedContext(req *AIInterpretRequest) (*sanitizedContext, string, string) {
	ctx := &sanitizedContext{}
	ctx.messages = make([]domain.AIMessage, 0, o.maxMessages)

	requestID := fmt.Sprintf("ai_%d", time.Now().UnixNano())
	traceID := requestID + "_trace"

	tokenCount := 0
	systemMsg := o.buildSystemPrompt()
	tokens := estimateTokens(systemMsg)
	tokenCount += tokens

	count := 0
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if count >= o.maxMessages {
			break
		}

		msg := req.Messages[i]
		if msg.Content == nil || strings.TrimSpace(msg.Content.Text) == "" {
			continue
		}

		msgTokens := estimateTokens(msg.Content.Text)
		if tokenCount+msgTokens > o.tokenBudget {
			break
		}

		role := "user"
		if msg.IsOutbound() {
			role = "assistant"
		}

		ctx.messages = append([]domain.AIMessage{{Role: role, Content: stripSensitive(msg.Content.Text)}}, ctx.messages...)
		tokenCount += msgTokens
		count++
	}

	ctx.messages = append([]domain.AIMessage{{Role: "system", Content: systemMsg}}, ctx.messages...)

	return ctx, requestID, traceID
}

func (o *AIOrchestrator) buildSystemPrompt() string {
	return systemPrompt
}

// systemPrompt is the static part of the interpreter instructions (see prompt.go).
var systemPrompt = buildStaticPrompt()

// verticalHelp lists the catalogue's verticals with the words customers use
// for them. The model only names a vertical; which sellers or providers serve
// it is decided server-side.
func verticalHelp() string {
	var b strings.Builder
	for _, def := range vertical.DefaultCatalog().All() {
		b.WriteString("  " + string(def.Code) + " (" + def.Name)
		if len(def.Keywords) > 0 {
			b.WriteString(": " + strings.Join(def.Keywords, ", "))
		}
		b.WriteString(")\n")
	}
	return b.String()
}

var intentHelp = map[domain.CommerceActionType]string{
	domain.ActionSearchListings:       "find listings matching a description",
	domain.ActionGetListing:           "show one specific listing",
	domain.ActionCompareListings:      "compare listings",
	domain.ActionSuggest:              "suggest listings or search terms",
	domain.ActionCheckAvailability:    "check whether a listing is available for dates",
	domain.ActionGetOrder:             "show an order",
	domain.ActionGetReservation:       "show a reservation",
	domain.ActionGetFulfillmentStatus: "fulfillment status of an order",
	domain.ActionGetDeliveryStatus:    "delivery status of an order",
	domain.ActionCheckInventory:       "stock level of a listing",
	domain.ActionGetPaymentStatus:     "payment status of an order",
}

func (o *AIOrchestrator) parseIntent(resp *domain.AIResponse) (*domain.AIIntent, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content), &raw); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	intentStr, _ := raw["intent"].(string)
	var actionType domain.CommerceActionType
	for _, a := range domain.AIAcceptedActions {
		if strings.EqualFold(string(a), intentStr) {
			actionType = a
			break
		}
	}
	if actionType == "" {
		return nil, fmt.Errorf("intent '%s' is not in the allowlist", intentStr)
	}

	confidence, _ := raw["confidence"].(float64)
	if confidence < 0 || confidence > 1 {
		return nil, fmt.Errorf("confidence must be between 0 and 1")
	}

	params, _ := raw["parameters"].(map[string]interface{})

	intent := domain.NewAIIntent()
	intent.Intent = actionType
	intent.Confidence = confidence
	intent.Parameters = params
	intent.Explanation, _ = raw["explanation"].(string)

	if intent.HasUntrustedFields() {
		return nil, fmt.Errorf("AI response contains untrusted sensitive fields")
	}

	return intent, nil
}

func (o *AIOrchestrator) entitiesFromParams(params map[string]interface{}) []domain.ConversationEntity {
	entities := make([]domain.ConversationEntity, 0, len(params))
	for k, v := range params {
		entities = append(entities, domain.ConversationEntity{
			Name:   k,
			Value:  fmt.Sprintf("%v", v),
			Source: "AI",
		})
	}
	return entities
}

func (o *AIOrchestrator) buildFallbackResponse(text string) *domain.ConversationResponse {
	resp := domain.NewConversationResponse(text, "en")
	resp.Metadata["source"] = "AI_FALLBACK"
	return resp
}

func (o *AIOrchestrator) buildSuccessResponse(action *domain.CommerceAction, execResult *commerce.ExecuteResult) *domain.ConversationResponse {
	text := "I've completed that for you."
	resp := domain.NewConversationResponse(text, "en")
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "AI_SUCCESS"
	return resp
}

func (o *AIOrchestrator) buildErrorResponse(execResult *commerce.ExecuteResult) *domain.ConversationResponse {
	text := "I wasn't able to complete that action. Please try again or contact support."
	resp := domain.NewConversationResponse(text, "en")
	resp.Metadata["error"] = true
	resp.Metadata["source"] = "AI_ERROR"
	return resp
}

func (o *AIOrchestrator) buildConfirmationResponse(action *domain.CommerceAction, execResult *commerce.ExecuteResult) *domain.ConversationResponse {
	text := "I can help with that, but I need your confirmation first."
	resp := domain.NewConversationResponse(text, "en")
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "AI_CONFIRMATION"
	resp.SetConfirmation(domain.ConversationConfirmation{
		ActionID: action.ActionID,
		Summary:  action.ConfirmationData.Summary,
		Details:  action.ConfirmationData.Details,
	})
	return resp
}

// sensitiveWords are redacted from text sent to a model. Matching ignores
// case; the rest of the message keeps its casing (names, places, IDs).
var sensitiveWords = regexp.MustCompile(`(?i)token|secret|password|credential|api_key`)

func stripSensitive(text string) string {
	return sensitiveWords.ReplaceAllString(text, "[REDACTED]")
}

func estimateTokens(text string) int {
	words := len(strings.Fields(text))
	return words * 13 / 10
}

func intentOutputSchema() *domain.JSONSchema {
	intents := make([]string, 0, len(domain.AIAcceptedActions)+1)
	for _, a := range domain.AIAcceptedActions {
		intents = append(intents, string(a))
	}
	intents = append(intents, "NONE")
	str := func(d string) domain.JSONSchemaProperty {
		return domain.JSONSchemaProperty{Type: "string", Description: d}
	}
	verticals := make([]string, 0, 10)
	for _, def := range vertical.DefaultCatalog().All() {
		verticals = append(verticals, string(def.Code))
	}
	return &domain.JSONSchema{
		Type: "object",
		Properties: map[string]domain.JSONSchemaProperty{
			"intent": {
				Type:        "string",
				Description: "The classified commerce action type",
				Enum:        intents,
			},
			"confidence": {
				Type:        "number",
				Description: "Confidence score between 0 and 1",
			},
			"parameters": {
				Type:        "object",
				Description: "Extracted parameters",
				Properties: map[string]domain.JSONSchemaProperty{
					"text":           str("Search words"),
					"listing_id":     str("Listing identifier"),
					"order_id":       str("Order identifier"),
					"reservation_id": str("Reservation identifier"),
					"start_at":       str("ISO 8601 start date"),
					"end_at":         str("ISO 8601 end date"),
					"quantity":       {Type: "integer", Description: "Number of units"},
					"vertical":       {Type: "string", Description: "Commerce vertical named by the user", Enum: verticals},
					"location":       str("Place name, e.g. a neighbourhood or city"),
					"nights":         {Type: "integer", Description: "Number of nights of a stay"},
					"price_max":      {Type: "number", Description: "Budget ceiling in US dollars"},
					"price_min":      {Type: "number", Description: "Budget floor in US dollars"},
					"sort":           {Type: "string", Description: "Ordering wanted", Enum: []string{"price_asc", "price_desc", "newest"}},
					"category_id":    str("Category identifier from the category list"),
					"party_size":     {Type: "integer", Description: "Number of people"},
				},
			},
			"explanation": {
				Type:        "string",
				Description: "Brief explanation of the classification",
			},
		},
		Required: []string{"intent", "confidence", "parameters", "explanation"},
	}
}
