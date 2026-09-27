package conversation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

// IntentClassifier does AI-only intent classification (no execution).
// This returns the classified intent without executing it, so the
// ConversationProcessor can manage confirmation, missing params, and context.
type IntentClassifier interface {
	ClassifyIntent(ctx context.Context, req *IntentClassificationRequest) (*IntentClassificationResult, error)
}

// IntentClassificationRequest is the request for pure intent classification.
type IntentClassificationRequest struct {
	ConversationID string
	OrganizationID string
	TenantID       string
	Actor          domain.CommerceActor
	Messages       []domain.Message
	Locale         string
	Channel        domain.Channel
}

// IntentClassificationResult is the result of intent classification.
type IntentClassificationResult struct {
	Intent       *domain.AIIntent
	ModelUsed    string
	ProviderUsed string
	TokensUsed   int
	LatencyMs    int64
	Fallback     *domain.ConversationResponse
}

// ActionRuntime handles commerce action execution.
type ActionRuntime interface {
	Execute(ctx context.Context, action *domain.CommerceAction, actor domain.CommerceActor) (*commerce.ExecuteResult, error)
	IsSupported(actionType domain.CommerceActionType) bool
}

// ProcessRequest represents an inbound message from any channel.
type ProcessRequest struct {
	Channel               domain.Channel
	SenderID              string
	SenderExternalID      string
	MessageText           string
	MessageExternalRef    string
	MessageIdempotencyKey string
	Locale                string
	TenantID              string
	Actor                 domain.CommerceActor
}

// ProcessResult represents the result of processing an inbound message.
type ProcessResult struct {
	ConversationID    string
	Response          *domain.ConversationResponse
	RequiresUserInput bool
	IsConfirmation    bool
	IsClarification   bool
	MissingParams     []domain.MissingParameter
}

// ConversationProcessor orchestrates the end-to-end conversational commerce flow.
type ConversationProcessor struct {
	logger             zerolog.Logger
	convStore          domain.ConversationStore
	contextStore       domain.ConversationContextStore
	messageStore       domain.MessageStore
	webhookEventStore  domain.WebhookEventStore
	classifier         IntentClassifier
	actionResolver     *ActionResolver
	runtime            ActionRuntime
	packResolver       domain.PackResolver
	confirmationExpiry time.Duration
}

// ProcessorConfig configures the ConversationProcessor.
type ProcessorConfig struct {
	Logger            zerolog.Logger
	ConversationStore domain.ConversationStore
	ContextStore      domain.ConversationContextStore
	MessageStore      domain.MessageStore
	WebhookEventStore domain.WebhookEventStore
	IntentClassifier  IntentClassifier
	ActionResolver    *ActionResolver
	Runtime           ActionRuntime
	PackResolver      domain.PackResolver
}

// NewConversationProcessor creates a new ConversationProcessor.
func NewConversationProcessor(config ProcessorConfig) *ConversationProcessor {
	logger := config.Logger
	if logger.GetLevel() == 0 {
		logger = zerolog.Nop()
	}

	return &ConversationProcessor{
		logger:             logger,
		convStore:          config.ConversationStore,
		contextStore:       config.ContextStore,
		messageStore:       config.MessageStore,
		webhookEventStore:  config.WebhookEventStore,
		classifier:         config.IntentClassifier,
		actionResolver:     config.ActionResolver,
		runtime:            config.Runtime,
		packResolver:       config.PackResolver,
		confirmationExpiry: 10 * time.Minute,
	}
}

// ProcessMessage processes an inbound message and returns a channel-neutral response.
func (p *ConversationProcessor) ProcessMessage(ctx context.Context, req *ProcessRequest) (*ProcessResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	if req.MessageText == "" {
		return nil, fmt.Errorf("message text is required")
	}
	if req.Channel == "" || !req.Channel.IsValid() {
		return nil, fmt.Errorf("invalid channel")
	}

	p.logger.Info().
		Str("channel", string(req.Channel)).
		Str("sender", req.SenderID).
		Msg("Processing inbound message")

	// 1. Find or create conversation
	conv, err := p.findOrCreateConversation(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve conversation: %w", err)
	}

	// 2. Persist inbound message
	if p.messageStore != nil && req.MessageIdempotencyKey != "" {
		inbound := domain.NewMessage(conv.ID, domain.MessageDirectionInbound, domain.MessageTypeText, req.SenderID, &domain.MessageContent{
			Text: req.MessageText,
			Data: map[string]interface{}{"locale": req.Locale},
		})
		inbound.ExternalRef = req.MessageExternalRef
		inbound.IdempotencyKey = req.MessageIdempotencyKey
		_ = p.messageStore.Create(ctx, inbound)
	}

	// 3. Load conversation context
	cx, err := p.loadContext(ctx, conv.ID)
	if err != nil {
		p.logger.Warn().Err(err).Str("conversation_id", conv.ID).Msg("Failed to load context, starting fresh")
		cx = &domain.ConversationContext{}
	}
	if cx.Language == "" {
		cx.Language = conv.Language
		if cx.Language == "" {
			cx.Language = "en"
		}
	}

	// 4. Check for pending confirmation
	if cx.HasPendingConfirmation() {
		return p.handlePendingConfirmation(ctx, conv, cx, req)
	}

	// 5. Check for missing parameters
	if cx.HasMissingParameters() {
		return p.handleMissingParameterReply(ctx, conv, cx, req)
	}

	// 6. Normal flow: interpret intent → resolve → execute
	return p.handleNewMessage(ctx, conv, cx, req)
}

func (p *ConversationProcessor) findOrCreateConversation(ctx context.Context, req *ProcessRequest) (*domain.Conversation, error) {
	if req.SenderExternalID != "" {
		conv, err := p.convStore.FindByChannelExternalID(ctx, req.Channel, req.SenderExternalID)
		if err == nil && conv != nil {
			return conv, nil
		}
	}

	conv := domain.NewConversation(req.Channel, req.SenderExternalID, req.Actor.ID, req.TenantID)
	if err := p.convStore.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return conv, nil
}

func (p *ConversationProcessor) loadContext(ctx context.Context, convID string) (*domain.ConversationContext, error) {
	if p.contextStore == nil {
		return &domain.ConversationContext{}, nil
	}
	return p.contextStore.Load(ctx, convID)
}

func (p *ConversationProcessor) saveContext(ctx context.Context, convID string, cx *domain.ConversationContext) error {
	if p.contextStore == nil {
		return nil
	}
	cx.LastActivityAt = time.Now().Unix()
	return p.contextStore.Save(ctx, convID, *cx)
}

// handlePendingConfirmation processes a user's response to a confirmation prompt.
func (p *ConversationProcessor) handlePendingConfirmation(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	req *ProcessRequest,
) (*ProcessResult, error) {
	flow := cx.ConfirmationState
	text := strings.ToLower(strings.TrimSpace(req.MessageText))

	switch {
	case isAffirmative(text):
		flow.State = domain.ConfirmationApproved
		cx.ConfirmationState = flow
		_ = p.saveContext(ctx, conv.ID, cx)

		return p.executePendingAction(ctx, conv, cx, flow.ActionID, true)

	case isNegative(text):
		flow.State = domain.ConfirmationRejected
		cx.ConfirmationState = flow
		_ = p.saveContext(ctx, conv.ID, cx)

		resp := domain.NewConversationResponse("I've cancelled that action.", cx.Language)
		return &ProcessResult{
			ConversationID: conv.ID,
			Response:       resp,
		}, nil

	default:
		resp := domain.NewConversationResponse("Please reply with yes or no to confirm or cancel this action.", cx.Language)
		resp.SetConfirmation(domain.ConversationConfirmation{
			ActionID: flow.ActionID,
			Summary:  flow.Summary,
			Details:  flow.Details,
		})
		return &ProcessResult{
			ConversationID:    conv.ID,
			Response:          resp,
			RequiresUserInput: true,
			IsConfirmation:    true,
		}, nil
	}
}

// handleMissingParameterReply processes a user's response to a clarification question.
func (p *ConversationProcessor) handleMissingParameterReply(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	req *ProcessRequest,
) (*ProcessResult, error) {
	missing := cx.MissingParameters
	if len(missing) == 0 {
		return p.handleNewMessage(ctx, conv, cx, req)
	}

	param := missing[0]
	resolvedValue, _ := domain.ResolveParameterValue(param.Name, req.MessageText, cx)

	if cx.PendingAction != nil {
		cx.PendingAction.Parameters[param.Name] = resolvedValue
	}

	cx.MissingParameters = cx.MissingParameters[1:]

	if len(cx.MissingParameters) > 0 {
		_ = p.saveContext(ctx, conv.ID, cx)
		resp := p.buildClarificationResponse(cx.Language, cx.MissingParameters, cx.PendingAction)
		return &ProcessResult{
			ConversationID:    conv.ID,
			Response:          resp,
			RequiresUserInput: true,
			IsClarification:   true,
			MissingParams:     cx.MissingParameters,
		}, nil
	}

	action := cx.PendingAction
	cx.PendingAction = nil
	_ = p.saveContext(ctx, conv.ID, cx)

	return p.executeAction(ctx, conv, cx, action, req.Actor)
}

// handleNewMessage processes a message that starts a new intent flow.
func (p *ConversationProcessor) handleNewMessage(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	req *ProcessRequest,
) (*ProcessResult, error) {
	// Build messages for AI interpretation
	messages, err := p.buildMessageHistory(ctx, conv.ID, req)
	if err != nil {
		return nil, err
	}

	// Interpret via AI
	intent, err := p.interpretIntent(ctx, conv, cx, req, messages)
	if err != nil {
		resp := domain.NewConversationResponse("I'm having trouble understanding your request. Could you rephrase?", cx.Language)
		return &ProcessResult{ConversationID: conv.ID, Response: resp}, nil
	}
	if intent == nil {
		resp := domain.NewConversationResponse("I can help you search for listings, check availability, and view your orders and reservations.", cx.Language)
		return &ProcessResult{ConversationID: conv.ID, Response: resp}, nil
	}

	// Update context with the resolved intent
	cx.CurrentIntent = intent

	// Resolve pack and specialist
	if cx.PackCode == "" {
		if pack, ok := p.resolvePack(ctx, conv.OrganizationID, intent.ActionType); ok {
			cx.PackCode = pack
		}
	}
	if cx.SpecialistCode == "" {
		if spec := p.resolveSpecialist(intent.ActionType); spec != "" {
			cx.SpecialistCode = spec
		}
	}

	// Build and execute the action
	action := intent.ToAction(conv.ID, req.Actor, req.TenantID)
	if action.IdempotencyKey == "" {
		action.IdempotencyKey = fmt.Sprintf("action_%s_%d", conv.ID, time.Now().UnixNano())
	}

	return p.executeAction(ctx, conv, cx, action, req.Actor)
}

func (p *ConversationProcessor) interpretIntent(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	req *ProcessRequest,
	messages []domain.Message,
) (*domain.ConversationIntent, error) {
	// Try AI classification if available
	if p.classifier != nil {
		classReq := &IntentClassificationRequest{
			ConversationID: conv.ID,
			OrganizationID: conv.OrganizationID,
			TenantID:       req.TenantID,
			Actor:          req.Actor,
			Messages:       messages,
			Locale:         req.Locale,
			Channel:        req.Channel,
		}

		classResult, err := p.classifier.ClassifyIntent(ctx, classReq)
		if err != nil {
			return nil, err
		}

		if classResult.Fallback != nil {
			return nil, nil
		}

		if classResult.Intent == nil {
			return nil, nil
		}

		// Resolve specialist and pack
		if len(classResult.Intent.Parameters) == 0 {
			classResult.Intent.Parameters = make(map[string]interface{})
		}

		specialist := p.resolveSpecialist(classResult.Intent.Intent)
		packCode, _ := p.resolvePack(ctx, conv.OrganizationID, classResult.Intent.Intent)

		return &domain.ConversationIntent{
			Name:                 "AI_CLASSIFIED",
			Confidence:           classResult.Intent.Confidence,
			Source:               "AI",
			ActionType:           classResult.Intent.Intent,
			Specialist:           specialist,
			VerticalPack:         packCode,
			RequiresConfirmation: domain.AIActionRequiresConfirmation(classResult.Intent.Intent),
			Entities:             domain.EntitiesFromParams(classResult.Intent.Parameters),
			Explanation:          classResult.Intent.Explanation,
		}, nil
	}

	// Fall back to keyword-based resolution
	actionType := resolveActionTypeKeyword(req.MessageText)
	if actionType == "" {
		return nil, nil
	}

	specialist := p.resolveSpecialist(actionType)
	packCode, _ := p.resolvePack(ctx, conv.OrganizationID, actionType)

	return &domain.ConversationIntent{
		Name:                 "KEYWORD_CLASSIFIED",
		Confidence:           1.0,
		Source:               "SYSTEM",
		ActionType:           actionType,
		Specialist:           specialist,
		VerticalPack:         packCode,
		RequiresConfirmation: domain.AIActionRequiresConfirmation(actionType),
	}, nil
}

func (p *ConversationProcessor) resolvePack(ctx context.Context, orgID string, action domain.CommerceActionType) (domain.PackCode, bool) {
	if p.packResolver == nil {
		return "", false
	}
	return p.packResolver.ResolvePackForAction(orgID, action)
}

func (p *ConversationProcessor) resolveSpecialist(action domain.CommerceActionType) domain.AISpecialistCode {
	if specialists := domain.GetSpecialistsForAction(action); len(specialists) > 0 {
		return specialists[0]
	}
	return ""
}

func (p *ConversationProcessor) executeAction(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	action *domain.CommerceAction,
	actor domain.CommerceActor,
) (*ProcessResult, error) {
	execResult, err := p.runtime.Execute(ctx, action, actor)
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	cx.LastActionID = action.ActionID
	cx.PendingAction = action

	result := &ProcessResult{
		ConversationID: conv.ID,
	}

	switch execResult.Status {
	case domain.ActionResultSuccess:
		result.Response = p.buildSuccessResponse(action, execResult, cx.Language)
		p.updateContextFromResult(ctx, conv, cx, action, execResult)
		cx.ConfirmationState = domain.ConfirmationFlowState{}
		cx.PendingAction = nil
		cx.MissingParameters = nil

	case domain.ActionResultRequiresConfirmation:
		result.IsConfirmation = true
		result.RequiresUserInput = true
		result.Response = p.buildConfirmationResponse(action, execResult, cx.Language)

		token := fmt.Sprintf("confirm_%s_%d", action.ActionID, time.Now().UnixNano())
		cx.ConfirmationState = domain.ConfirmationFlowState{
			ActionID:          action.ActionID,
			State:             domain.ConfirmationRequested,
			Summary:           execResult.MessageKey,
			Details:           map[string]interface{}{"action_type": string(action.ActionType)},
			RequestedAt:       time.Now().Unix(),
			ConfirmationToken: token,
		}

	case domain.ActionResultValidationFailed:
		result.IsClarification = true
		result.RequiresUserInput = true
		result.MissingParams = p.extractMissingParams(execResult)
		cx.MissingParameters = result.MissingParams
		result.Response = p.buildClarificationResponse(cx.Language, result.MissingParams, action)
		cx.PendingAction = action

	case domain.ActionResultUnauthorized:
		result.Response = domain.NewConversationResponse("You don't have permission to perform this action.", cx.Language)

	case domain.ActionResultNotFound:
		result.Response = domain.NewConversationResponse("I couldn't find what you're looking for. Please check the details and try again.", cx.Language)

	default:
		result.Response = domain.NewConversationResponse("Something went wrong processing your request. Please try again.", cx.Language)
	}

	_ = p.saveContext(ctx, conv.ID, cx)
	return result, nil
}

func (p *ConversationProcessor) executePendingAction(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	actionID string,
	confirmed bool,
) (*ProcessResult, error) {
	if cx.PendingAction == nil {
		return nil, fmt.Errorf("no pending action found for confirmation")
	}

	action := cx.PendingAction
	if confirmed {
		action.ConfirmationState = domain.ConfirmationApproved
	}

	execResult, err := p.runtime.Execute(ctx, action, action.Actor)
	if err != nil {
		return nil, fmt.Errorf("action execution failed: %w", err)
	}

	result := &ProcessResult{
		ConversationID: conv.ID,
	}

	if execResult.Status == domain.ActionResultSuccess {
		result.Response = p.buildSuccessResponse(action, execResult, cx.Language)
		p.updateContextFromResult(ctx, conv, cx, action, execResult)
	} else {
		result.Response = domain.NewConversationResponse("I wasn't able to complete that action. Please try again.", cx.Language)
	}

	cx.ConfirmationState = domain.ConfirmationFlowState{}
	cx.PendingAction = nil
	_ = p.saveContext(ctx, conv.ID, cx)

	return result, nil
}

func (p *ConversationProcessor) buildSuccessResponse(
	action *domain.CommerceAction,
	result *commerce.ExecuteResult,
	language string,
) *domain.ConversationResponse {
	resp := domain.NewConversationResponse("I've completed that for you.", language)
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "conversation_processor"
	resp.Metadata["action_id"] = action.ActionID

	if data, ok := result.Data["data"]; ok {
		resp.Metadata["result_data"] = data
	}

	return resp
}

func (p *ConversationProcessor) buildConfirmationResponse(
	action *domain.CommerceAction,
	result *commerce.ExecuteResult,
	language string,
) *domain.ConversationResponse {
	resp := domain.NewConversationResponse("I need your confirmation before I proceed.", language)
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "confirmation_requested"
	resp.SetConfirmation(domain.ConversationConfirmation{
		ActionID: action.ActionID,
		Summary:  result.MessageKey,
		Details:  map[string]interface{}{"action_type": string(action.ActionType)},
	})
	return resp
}

func (p *ConversationProcessor) buildClarificationResponse(
	language string,
	missing []domain.MissingParameter,
	action *domain.CommerceAction,
) *domain.ConversationResponse {
	if len(missing) == 0 {
		return domain.NewConversationResponse("Could you provide more details?", language)
	}

	param := missing[0]
	resp := domain.NewConversationResponse(p.missingParamPrompt(param), language)
	resp.Metadata["source"] = "clarification_requested"
	resp.Metadata["missing_param"] = param.Name
	resp.Metadata["missing_param_description"] = param.Description

	if len(param.Examples) > 0 {
		resp.Metadata["examples"] = param.Examples
	}

	if action != nil {
		resp.Metadata["action_type"] = string(action.ActionType)
	}

	return resp
}

func (p *ConversationProcessor) missingParamPrompt(param domain.MissingParameter) string {
	prompt := fmt.Sprintf("I need a bit more information: %s. What would you like to use?", param.Description)
	if len(param.Examples) > 0 && len(param.Examples) <= 3 {
		prompt += fmt.Sprintf(" (e.g., %s)", strings.Join(param.Examples, ", "))
	}
	return prompt
}

func (p *ConversationProcessor) extractMissingParams(result *commerce.ExecuteResult) []domain.MissingParameter {
	var params []domain.MissingParameter
	for _, err := range result.Errors {
		params = append(params, domain.MissingParameter{
			Name:        err.Field,
			Description: err.Message,
		})
	}
	if len(params) == 0 {
		params = append(params, domain.MissingParameter{
			Name:        "details",
			Description: "additional details needed to complete your request",
		})
	}
	return params
}

// updateContextFromResult updates the conversation context after a successful action.
func (p *ConversationProcessor) updateContextFromResult(
	ctx context.Context,
	conv *domain.Conversation,
	cx *domain.ConversationContext,
	action *domain.CommerceAction,
	result *commerce.ExecuteResult,
) {
	data := result.Data["data"]
	switch action.ActionType {
	case domain.ActionSearchListings:
		if data != nil {
			p.storeSearchResult(ctx, conv.ID, cx, data)
		}
	case domain.ActionGetListing:
		if data != nil {
			p.storeSingleResult(ctx, conv.ID, cx, "listing", data)
		}
	case domain.ActionGetOrder:
		if data != nil {
			p.storeSingleResult(ctx, conv.ID, cx, "order", data)
		}
	case domain.ActionCreateOrder:
		if data != nil {
			p.storeSingleResult(ctx, conv.ID, cx, "order", data)
		}
	case domain.ActionGetReservation:
		if data != nil {
			p.storeSingleResult(ctx, conv.ID, cx, "reservation", data)
		}
	case domain.ActionCreateReservation:
		if data != nil {
			p.storeSingleResult(ctx, conv.ID, cx, "reservation", data)
		}
	}
}

func (p *ConversationProcessor) storeSearchResult(ctx context.Context, convID string, cx *domain.ConversationContext, data interface{}) {
	items := extractResultItems(data)
	cx.ResultItems = items
}

func (p *ConversationProcessor) storeSingleResult(ctx context.Context, convID string, cx *domain.ConversationContext, refType string, data interface{}) {
	item := domain.ResultItemRef{
		RefType: refType,
		Index:   0,
		Data:    data,
	}

	if m, ok := data.(map[string]interface{}); ok {
		if id, ok := m["id"].(string); ok {
			item.ID = id
		}
		if title, ok := m["title"].(string); ok {
			item.Title = title
		} else if name, ok := m["name"].(string); ok {
			item.Title = name
		}
		if status, ok := m["status"].(string); ok {
			item.Subtitle = status
		}
	}

	cx.ResultItems = []domain.ResultItemRef{item}
}

func extractResultItems(data interface{}) []domain.ResultItemRef {
	if data == nil {
		return nil
	}

	result, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	itemsList, ok := result["items"].([]interface{})
	if !ok {
		itemsList, ok = result["listings"].([]interface{})
	}
	if !ok {
		return nil
	}

	items := make([]domain.ResultItemRef, 0, len(itemsList))
	for i, itemData := range itemsList {
		item := domain.ResultItemRef{Index: i}
		if m, ok := itemData.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				item.ID = id
			}
			if title, ok := m["title"].(string); ok {
				item.Title = title
			} else if name, ok := m["name"].(string); ok {
				item.Title = name
			}
			if status, ok := m["status"].(string); ok {
				item.Subtitle = status
			}
			item.Data = itemData
		}
		items = append(items, item)
	}
	return items
}

func (p *ConversationProcessor) buildMessageHistory(ctx context.Context, convID string, req *ProcessRequest) ([]domain.Message, error) {
	messages := []domain.Message{}
	if p.messageStore != nil {
		history, _ := p.messageStore.FindByConversationID(ctx, convID)
		for _, m := range history {
			if m != nil {
				messages = append(messages, *m)
			}
		}
	}

	inbound := domain.NewMessage(convID, domain.MessageDirectionInbound, domain.MessageTypeText, req.SenderID, &domain.MessageContent{
		Text: req.MessageText,
	})
	messages = append(messages, *inbound)

	return messages, nil
}

// ConfirmAction processes a confirmation response using a token.
func (p *ConversationProcessor) ConfirmAction(ctx context.Context, convID string, token string) (*ProcessResult, error) {
	cx, err := p.loadContext(ctx, convID)
	if err != nil {
		return nil, err
	}

	if cx.ConfirmationState.ConfirmationToken != token || cx.ConfirmationState.State != domain.ConfirmationRequested {
		return nil, fmt.Errorf("invalid or expired confirmation token")
	}

	conv, err := p.convStore.FindByID(ctx, convID)
	if err != nil {
		return nil, err
	}

	return p.executePendingAction(ctx, conv, cx, cx.ConfirmationState.ActionID, true)
}

// RejectAction processes a rejection of a confirmation.
func (p *ConversationProcessor) RejectAction(ctx context.Context, convID string, token string) (*ProcessResult, error) {
	cx, err := p.loadContext(ctx, convID)
	if err != nil {
		return nil, err
	}

	if cx.ConfirmationState.ConfirmationToken != token || cx.ConfirmationState.State != domain.ConfirmationRequested {
		return nil, fmt.Errorf("invalid or expired confirmation token")
	}

	cx.ConfirmationState.State = domain.ConfirmationRejected
	_ = p.saveContext(ctx, convID, cx)

	resp := domain.NewConversationResponse("I've cancelled that action.", cx.Language)
	return &ProcessResult{ConversationID: convID, Response: resp}, nil
}

func resolveActionTypeKeyword(text string) domain.CommerceActionType {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "search") || strings.Contains(lower, "find") ||
		strings.Contains(lower, "look") || strings.Contains(lower, "browse") ||
		strings.Contains(lower, "catalog") || strings.Contains(lower, "list") ||
		strings.Contains(lower, "suggest") || strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "option") || strings.Contains(lower, "alternative") {
		return domain.ActionSearchListings
	}

	if strings.Contains(lower, "available") || strings.Contains(lower, "check") ||
		strings.Contains(lower, "availability") {
		return domain.ActionCheckAvailability
	}

	if strings.Contains(lower, "reserve") || strings.Contains(lower, "book") {
		return domain.ActionCreateReservation
	}

	if strings.Contains(lower, "my reservation") || strings.Contains(lower, "view booking") {
		return domain.ActionGetReservation
	}

	if strings.Contains(lower, "cancel") || strings.Contains(lower, "annuler") {
		return domain.ActionCancelReservation
	}

	if strings.Contains(lower, "buy") || strings.Contains(lower, "order") ||
		strings.Contains(lower, "purchase") || strings.Contains(lower, "cart") ||
		strings.Contains(lower, "checkout") || strings.Contains(lower, "pay") {
		return domain.ActionCreateOrder
	}

	if strings.Contains(lower, "create listing") || strings.Contains(lower, "new listing") ||
		strings.Contains(lower, "add product") || strings.Contains(lower, "sell") ||
		strings.Contains(lower, "publish") {
		return domain.ActionCreateListing
	}

	return ""
}

func isAffirmative(text string) bool {
	affirmatives := []string{"yes", "yeah", "yep", "sure", "ok", "okay", "confirm", "confirmed", "proceed", "go ahead", "please do"}
	for _, a := range affirmatives {
		if strings.Contains(text, a) {
			return true
		}
	}
	// Single-letter "y" must be exact match
	trimmed := strings.TrimSpace(text)
	return trimmed == "y"
}

func isNegative(text string) bool {
	negatives := []string{"no", "nope", "cancel", "never", "stop", "abort", "no thanks", "not now"}
	for _, n := range negatives {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}
