package conversation

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/application/vertical"
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

	// 1. Find or create conversation (enforces ownership)
	conv, err := p.findOrCreateConversation(ctx, req)
	if err != nil {
		if errors.Is(err, domain.ErrConversationForbidden) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to resolve conversation: %w", err)
	}

	// Guests act under a conversation-scoped pseudo identity: they can browse
	// but never reach use cases that require a real account.
	if !req.Actor.IsAuthenticated() {
		req.Actor = domain.NewCommerceActor("guest:"+conv.ID, domain.ActorTypeGuest)
	}

	// 2. Persist inbound message; a redelivered message (same idempotency key)
	// is answered from the stored reply and never re-executes its action.
	messageKey := req.MessageIdempotencyKey
	if messageKey == "" {
		messageKey = "in:" + domain.NewID()
	}
	if p.messageStore != nil {
		inbound := domain.NewMessage(conv.ID, domain.MessageDirectionInbound, domain.MessageTypeText, req.SenderID, &domain.MessageContent{
			Text: req.MessageText,
			Data: map[string]interface{}{"locale": req.Locale},
		})
		inbound.ExternalRef = req.MessageExternalRef
		inbound.IdempotencyKey = messageKey
		inbound.CreatedAt = time.Now().Unix()
		if err := p.messageStore.Create(ctx, inbound); err != nil {
			if errors.Is(err, domain.ErrDuplicateMessage) {
				return p.replayReply(ctx, conv, messageKey), nil
			}
			p.logger.Warn().Err(err).Str("conversation_id", conv.ID).Msg("Failed to persist inbound message")
		}
	}

	result, err := p.dispatch(ctx, conv, req)
	if err != nil {
		return nil, err
	}
	p.persistReply(ctx, conv, messageKey, result)
	return result, nil
}

// dispatch routes the message according to the persisted conversation context.
func (p *ConversationProcessor) dispatch(ctx context.Context, conv *domain.Conversation, req *ProcessRequest) (*ProcessResult, error) {
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

	// 4. Check for pending confirmation (expired confirmations are dropped)
	if cx.HasPendingConfirmation() {
		if p.confirmationExpired(cx) {
			cx.ConfirmationState = domain.ConfirmationFlowState{}
			cx.PendingAction = nil
			_ = p.saveContext(ctx, conv.ID, cx)
		} else {
			return p.handlePendingConfirmation(ctx, conv, cx, req)
		}
	}

	// 5. Check for missing parameters
	if cx.HasMissingParameters() {
		return p.handleMissingParameterReply(ctx, conv, cx, req)
	}

	// 6. Normal flow: interpret intent → resolve → execute
	return p.handleNewMessage(ctx, conv, cx, req)
}

// replayReply returns the reply stored for an already-processed message.
func (p *ConversationProcessor) replayReply(ctx context.Context, conv *domain.Conversation, messageKey string) *ProcessResult {
	text := "I already received that message."
	if p.messageStore != nil {
		if history, err := p.messageStore.FindByConversationID(ctx, conv.ID); err == nil {
			for _, m := range history {
				if m != nil && m.IdempotencyKey == "reply:"+messageKey && m.Content != nil {
					text = m.Content.Text
				}
			}
		}
	}
	resp := domain.NewConversationResponse(text, conv.Language)
	resp.Metadata["duplicate"] = true
	return &ProcessResult{ConversationID: conv.ID, Response: resp}
}

// persistReply stores the outbound reply so the transcript is complete and
// redeliveries can be answered without re-executing.
func (p *ConversationProcessor) persistReply(ctx context.Context, conv *domain.Conversation, messageKey string, result *ProcessResult) {
	if p.messageStore == nil || result == nil || result.Response == nil {
		return
	}
	out := domain.NewMessage(conv.ID, domain.MessageDirectionOutbound, domain.MessageTypeText, "trezutil", &domain.MessageContent{
		Text: result.Response.Text,
	})
	out.IdempotencyKey = "reply:" + messageKey
	out.Language = result.Response.Language
	out.CreatedAt = time.Now().Unix()
	if err := p.messageStore.Create(ctx, out); err != nil && !errors.Is(err, domain.ErrDuplicateMessage) {
		p.logger.Warn().Err(err).Str("conversation_id", conv.ID).Msg("Failed to persist reply")
	}
}

func (p *ConversationProcessor) confirmationExpired(cx *domain.ConversationContext) bool {
	requested := cx.ConfirmationState.RequestedAt
	return requested > 0 && time.Now().Unix()-requested > int64(p.confirmationExpiry/time.Second)
}

func (p *ConversationProcessor) findOrCreateConversation(ctx context.Context, req *ProcessRequest) (*domain.Conversation, error) {
	if req.SenderExternalID != "" {
		conv, err := p.convStore.FindByChannelExternalID(ctx, req.Channel, req.SenderExternalID)
		if err == nil && conv != nil {
			switch {
			case conv.CustomerID == "" && req.Actor.IsAuthenticated():
				// A guest conversation is claimed when its visitor signs in.
				conv.CustomerID = req.Actor.ID
				conv.UpdatedAt = time.Now().Unix()
				if err := p.convStore.Update(ctx, conv); err != nil {
					return nil, err
				}
			case conv.CustomerID != "" && conv.CustomerID != req.Actor.ID:
				// Never let one caller continue (or confirm actions in) another
				// person's conversation by presenting its external ID.
				return nil, domain.ErrConversationForbidden
			}
			return conv, nil
		}
	}

	customerID := ""
	if req.Actor.IsAuthenticated() {
		customerID = req.Actor.ID
	}
	conv := domain.NewConversation(req.Channel, req.SenderExternalID, customerID, req.TenantID)
	if conv.ExternalID == "" {
		conv.ExternalID = conv.ID
	}
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

		return p.executePendingAction(ctx, conv, cx, req.Actor)

	case isNegative(text):
		flow.State = domain.ConfirmationRejected
		cx.ConfirmationState = flow
		cx.PendingAction = nil
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
	resolvedValue := strings.TrimSpace(req.MessageText)
	if strings.HasSuffix(param.Name, "_id") {
		// "the second one" -> the ID of the second listed result.
		resolvedValue, _ = domain.ResolveParameterValue(param.Name, req.MessageText, cx)
	}

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
	applySearchCriteria(cx, action)
	if listingScopedActions[action.ActionType] {
		if v, ok := action.Parameters["listing_id"]; !ok || v == "" {
			if id := currentListingID(cx); id != "" {
				action.Parameters["listing_id"] = id
			}
		}
	}
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
	// A short reply pointing at a previous result ("2", "the second one")
	// opens that listing.
	if id := referencedResultID(req.MessageText, cx); id != "" {
		return &domain.ConversationIntent{
			Name:       "RESULT_REFERENCE",
			Confidence: 1.0,
			Source:     "SYSTEM",
			ActionType: domain.ActionGetListing,
			Entities:   []domain.ConversationEntity{{Name: "listing_id", Value: id, NormalizedValue: id, Source: "CONTEXT"}},
		}, nil
	}

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
		if err != nil || classResult == nil || classResult.Fallback != nil || classResult.Intent == nil {
			// The model is unavailable or unsure: degrade to deterministic
			// keyword routing instead of failing the conversation.
			if err != nil {
				p.logger.Warn().Err(err).Msg("AI classification failed; using keyword routing")
			}
			return keywordIntent(p, ctx, conv, req), nil
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
	return keywordIntent(p, ctx, conv, req), nil
}

// keywordIntent is the deterministic, AI-free intent resolver. For searches
// the user's own words are the query text.
func keywordIntent(p *ConversationProcessor, ctx context.Context, conv *domain.Conversation, req *ProcessRequest) *domain.ConversationIntent {
	actionType := resolveActionTypeKeyword(req.MessageText)
	if actionType == "" {
		return nil
	}

	specialist := p.resolveSpecialist(actionType)
	packCode, _ := p.resolvePack(ctx, conv.OrganizationID, actionType)

	intent := &domain.ConversationIntent{
		Name:                 "KEYWORD_CLASSIFIED",
		Confidence:           1.0,
		Source:               "SYSTEM",
		ActionType:           actionType,
		Specialist:           specialist,
		VerticalPack:         packCode,
		RequiresConfirmation: domain.AIActionRequiresConfirmation(actionType),
	}
	if actionType == domain.ActionSearchListings {
		intent.Entities = []domain.ConversationEntity{{Name: "text", Value: req.MessageText, NormalizedValue: req.MessageText, Source: "USER"}}
	}
	return intent
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
	if !actor.IsAuthenticated() && !guestAllowedActions[action.ActionType] {
		resp := domain.NewConversationResponse("Please sign in with your phone number to continue — browsing is open to everyone, but orders, bookings and payments need an account.", cx.Language)
		resp.Metadata["requires_authentication"] = true
		resp.Metadata["action_type"] = string(action.ActionType)
		_ = p.saveContext(ctx, conv.ID, cx)
		return &ProcessResult{ConversationID: conv.ID, Response: resp, RequiresUserInput: true}, nil
	}

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

		// Unguessable, single-use token; confirming also requires being the
		// conversation owner, so disclosing it to that owner is safe.
		token := "confirm_" + domain.NewID()
		if result.Response.Confirmation != nil {
			result.Response.Confirmation.Details["confirmation_token"] = token
		}
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

	case domain.ActionResultNotImplemented:
		result.Response = domain.NewConversationResponse("I can't do that from the chat yet. You can do it from the listing page in the Trezutil app.", cx.Language)
		cx.PendingAction = nil

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
	actor domain.CommerceActor,
) (*ProcessResult, error) {
	if cx.PendingAction == nil {
		return nil, fmt.Errorf("no pending action found for confirmation")
	}

	action := cx.PendingAction
	// Only the person who requested the action may confirm it, and it always
	// executes as the currently authenticated actor.
	if action.Actor.ID != actor.ID {
		return nil, domain.ErrConversationForbidden
	}
	action.ConfirmationState = domain.ConfirmationApproved

	execResult, err := p.runtime.Execute(ctx, action, actor)
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
	data := result.Data["data"]
	resp := domain.NewConversationResponse(successText(action.ActionType, data), language)
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "conversation_processor"
	resp.Metadata["action_id"] = action.ActionID
	if data != nil {
		resp.Metadata["result_data"] = data
	}
	for _, item := range extractResultItems(data) {
		if len(resp.Cards) >= 10 {
			break
		}
		if item.ID == "" {
			continue
		}
		resp.AddCard(domain.ConversationCard{
			ID:          item.ID,
			Type:        item.RefType,
			Title:       item.Title,
			Description: item.Subtitle,
			Data:        map[string]interface{}{"listing_id": item.ID, "position": item.Index + 1},
		})
	}
	return resp
}

// successText renders a channel-neutral, human reply (WhatsApp shows only text).
func successText(actionType domain.CommerceActionType, data interface{}) string {
	switch actionType {
	case domain.ActionSearchListings:
		items := extractResultItems(data)
		if len(items) == 0 {
			return "I couldn't find anything matching that yet. Try other words, a neighbourhood, or a category."
		}
		var b strings.Builder
		heading := ""
		if m, ok := data.(map[string]interface{}); ok {
			if _, relaxed := m["relaxed"]; relaxed {
				if c, ok := m["criteria"].(map[string]interface{}); ok && c["location"] != nil {
					fmt.Fprintf(&b, "Nothing is listed in %v yet, so here are the closest matches.\n", c["location"])
				}
			}
			heading = criteriaHeading(m["criteria"])
		}
		if heading != "" {
			fmt.Fprintf(&b, "%s — %d found:", heading, len(items))
		} else {
			fmt.Fprintf(&b, "Here is what I found (%d):", len(items))
		}
		for i, it := range items {
			if i == 5 {
				b.WriteString("\n…and more.")
				break
			}
			fmt.Fprintf(&b, "\n%d. %s", i+1, it.Title)
			if it.Subtitle != "" {
				fmt.Fprintf(&b, " — %s", it.Subtitle)
			}
		}
		b.WriteString("\nReply with the number of the one you like.")
		return b.String()
	case domain.ActionGetListing:
		if l, ok := data.(*domain.Listing); ok && l != nil {
			text := l.Title
			if price := formatPrice(l.PriceAmount, l.PriceCurrency); price != "" {
				text += " — " + price
			}
			if l.Description != nil && *l.Description != "" {
				text += "\n" + *l.Description
			}
			return text + "\nSay \"book it\" to reserve dates or \"buy it\" to order."
		}
	case domain.ActionPauseListing:
		return "Done — the listing is paused and hidden from search."
	case domain.ActionResumeListing, domain.ActionPublishListing:
		return "Done — the listing is live and searchable."
	case domain.ActionCreateOrder:
		if m, ok := data.(map[string]interface{}); ok {
			return fmt.Sprintf("Your order is created (ref %v). You can pay for it now.", m["order_id"])
		}
		return "Your order is created."
	case domain.ActionCreateReservation:
		return "You're booked! Your reservation is confirmed."
	case domain.ActionCancelReservation, domain.ActionCancelOrder:
		return "Done — it's cancelled."
	case domain.ActionCheckAvailability:
		if m, ok := data.(map[string]interface{}); ok {
			if avail, _ := m["available"].(bool); avail {
				return "Good news: it's available for those dates."
			}
			return "Sorry, it's not available for those dates."
		}
	}
	return "I've completed that for you."
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
	if prompt, ok := friendlyParamPrompts[param.Name]; ok {
		return prompt
	}
	prompt := fmt.Sprintf("I need a bit more information: %s. What would you like to use?", param.Description)
	if len(param.Examples) > 0 && len(param.Examples) <= 3 {
		prompt += fmt.Sprintf(" (e.g., %s)", strings.Join(param.Examples, ", "))
	}
	return prompt
}

var friendlyParamPrompts = map[string]string{
	"text":           "What are you looking for? (e.g. \"a hotel in Gombe\" or \"iPhone 13\")",
	"listing_id":     "Which one? You can reply with its number from the list (e.g. \"2\" or \"the second one\").",
	"start_at":       "When should it start? (e.g. 2026-10-01 14:00)",
	"end_at":         "And when should it end? (e.g. 2026-10-03 11:00)",
	"quantity":       "How many would you like?",
	"vertical":       "What kind of place are you looking for? (e.g. a hotel, a restaurant, an apartment)",
	"location":       "Where? (e.g. Gombe)",
	"nights":         "For how many nights?",
	"party_size":     "For how many people?",
	"order_id":       "Which order? Please share its reference.",
	"reservation_id": "Which reservation? Please share its reference.",
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

	if l, ok := data.(*domain.Listing); ok && l != nil {
		item.ID, item.Title, item.Subtitle = l.ID, l.Title, formatPrice(l.PriceAmount, l.PriceCurrency)
		item.Data = nil
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

	// Typed search results (from the in-process search use case).
	if typed, ok := result["items"].([]domain.SearchResult); ok {
		items := make([]domain.ResultItemRef, 0, len(typed))
		for i, r := range typed {
			subtitle := formatPrice(r.PriceAmount, r.PriceCurrency)
			if r.Location != nil && r.Location.Neighborhood != nil && *r.Location.Neighborhood != "" {
				subtitle += " · " + *r.Location.Neighborhood
			}
			items = append(items, domain.ResultItemRef{
				RefType:  "listing",
				ID:       r.ListingID,
				Index:    i,
				Title:    r.Title,
				Subtitle: subtitle,
			})
		}
		return items
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
		item := domain.ResultItemRef{Index: i, RefType: "listing"}
		if m, ok := itemData.(map[string]interface{}); ok {
			if id, ok := m["id"].(string); ok {
				item.ID = id
			} else if id, ok := m["listing_id"].(string); ok {
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

func formatPrice(amount float64, currency string) string {
	if currency == "" {
		return ""
	}
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%d %s", int64(amount), currency)
	}
	return fmt.Sprintf("%.2f %s", amount, currency)
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

	if n := len(messages); n == 0 || messages[n-1].Content == nil || messages[n-1].Content.Text != req.MessageText {
		inbound := domain.NewMessage(convID, domain.MessageDirectionInbound, domain.MessageTypeText, req.SenderID, &domain.MessageContent{
			Text: req.MessageText,
		})
		messages = append(messages, *inbound)
	}
	// Bound the history handed to the classifier.
	if len(messages) > 20 {
		messages = messages[len(messages)-20:]
	}

	return messages, nil
}

// ConfirmAction processes a confirmation response using a token. The actor
// must own the conversation and be the one who requested the action.
func (p *ConversationProcessor) ConfirmAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*ProcessResult, error) {
	conv, cx, err := p.loadOwnedPendingConfirmation(ctx, convID, token, actor)
	if err != nil {
		return nil, err
	}
	result, err := p.executePendingAction(ctx, conv, cx, actor)
	if err != nil {
		return nil, err
	}
	p.persistReply(ctx, conv, "confirm:"+token, result)
	return result, nil
}

// RejectAction processes a rejection of a confirmation.
func (p *ConversationProcessor) RejectAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*ProcessResult, error) {
	_, cx, err := p.loadOwnedPendingConfirmation(ctx, convID, token, actor)
	if err != nil {
		return nil, err
	}

	cx.ConfirmationState.State = domain.ConfirmationRejected
	cx.PendingAction = nil
	_ = p.saveContext(ctx, convID, cx)

	resp := domain.NewConversationResponse("I've cancelled that action.", cx.Language)
	return &ProcessResult{ConversationID: convID, Response: resp}, nil
}

func (p *ConversationProcessor) loadOwnedPendingConfirmation(ctx context.Context, convID, token string, actor domain.CommerceActor) (*domain.Conversation, *domain.ConversationContext, error) {
	conv, err := p.convStore.FindByID(ctx, convID)
	if err != nil || conv == nil {
		return nil, nil, fmt.Errorf("invalid or expired confirmation token")
	}
	if !actor.IsAuthenticated() || conv.CustomerID != actor.ID {
		return nil, nil, domain.ErrConversationForbidden
	}
	cx, err := p.loadContext(ctx, convID)
	if err != nil {
		return nil, nil, err
	}
	if token == "" || cx.ConfirmationState.ConfirmationToken != token ||
		cx.ConfirmationState.State != domain.ConfirmationRequested || p.confirmationExpired(cx) {
		return nil, nil, fmt.Errorf("invalid or expired confirmation token")
	}
	return conv, cx, nil
}

// listingScopedActions act on one listing; "book it", "buy it" or "pause this
// listing" refer to the listing the user is currently looking at.
// criteriaHeading states the structured criteria of a search:
// "Hotels & accommodation in Gombe · 2 nights · 2 guests".
func criteriaHeading(raw interface{}) string {
	c, ok := raw.(map[string]interface{})
	if !ok || len(c) == 0 {
		return ""
	}
	var parts []string
	head := "Results"
	if v, ok := c["vertical"].(string); ok {
		if def, found := vertical.DefaultCatalog().Get(domain.Vertical(v)); found {
			head = def.Name
		}
	}
	if loc, ok := c["location"].(string); ok && loc != "" {
		head += " in " + loc
	}
	parts = append(parts, head)
	if n := toInt(c["nights"]); n > 0 {
		parts = append(parts, pluralize(n, "night"))
	}
	if n := toInt(c["party_size"]); n > 0 {
		parts = append(parts, pluralize(n, "guest"))
	}
	return strings.Join(parts, " · ")
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

func pluralize(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// applySearchCriteria remembers the structured criteria of a search and lends
// them to a later availability check or booking that doesn't restate them
// ("a hotel in Gombe for two nights for two people" … "book it").
func applySearchCriteria(cx *domain.ConversationContext, action *domain.CommerceAction) {
	if action.Parameters == nil {
		action.Parameters = map[string]interface{}{}
	}
	str := func(k string) string {
		if v, ok := action.Parameters[k]; ok && v != nil {
			return strings.TrimSpace(fmt.Sprintf("%v", v))
		}
		return ""
	}
	num := func(k string) int {
		n, _ := strconv.Atoi(str(k))
		return n
	}
	switch action.ActionType {
	case domain.ActionSearchListings:
		c := domain.SearchCriteria{
			Vertical:  domain.NormalizeVertical(str("vertical")),
			Location:  str("location"),
			Nights:    num("nights"),
			PartySize: num("party_size"),
			StartAt:   str("start_at"),
		}
		if c.IsZero() {
			cx.SearchCriteria = nil
		} else {
			cx.SearchCriteria = &c
		}
	case domain.ActionCheckAvailability, domain.ActionCreateReservation:
		c := cx.SearchCriteria
		if c == nil {
			return
		}
		if str("nights") == "" && c.Nights > 0 {
			action.Parameters["nights"] = c.Nights
		}
		if str("party_size") == "" && c.PartySize > 0 {
			action.Parameters["party_size"] = c.PartySize
		}
		if str("start_at") == "" && c.StartAt != "" {
			action.Parameters["start_at"] = c.StartAt
		}
	}
}

var listingScopedActions = map[domain.CommerceActionType]bool{
	domain.ActionGetListing:        true,
	domain.ActionCheckAvailability: true,
	domain.ActionCreateReservation: true,
	domain.ActionCreateOrder:       true,
	domain.ActionPublishListing:    true,
	domain.ActionPauseListing:      true,
	domain.ActionResumeListing:     true,
}

// currentListingID returns the listing in focus: the single listing last
// shown (e.g. after opening a result).
func currentListingID(cx *domain.ConversationContext) string {
	if cx == nil || len(cx.ResultItems) != 1 {
		return ""
	}
	if item := cx.ResultItems[0]; item.RefType == "listing" {
		return item.ID
	}
	return ""
}

// guestAllowedActions are read-only discovery actions available without an account.
var guestAllowedActions = map[domain.CommerceActionType]bool{
	domain.ActionSearchListings:  true,
	domain.ActionGetListing:      true,
	domain.ActionSuggest:         true,
	domain.ActionCompareListings: true,
}

func resolveActionTypeKeyword(text string) domain.CommerceActionType {
	lower := strings.ToLower(text)

	// Listing lifecycle (seller operations) first: "listing" contains "list".
	if strings.Contains(lower, "pause") || strings.Contains(lower, "suspend") {
		return domain.ActionPauseListing
	}
	if strings.Contains(lower, "resume") || strings.Contains(lower, "reactivate") || strings.Contains(lower, "unpause") {
		return domain.ActionResumeListing
	}
	if strings.Contains(lower, "publish") {
		return domain.ActionPublishListing
	}

	if strings.Contains(lower, "search") || strings.Contains(lower, "find") || strings.Contains(lower, "cherch") ||
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
		strings.Contains(lower, "add product") || strings.Contains(lower, "sell") {
		return domain.ActionCreateListing
	}

	return ""
}

func isAffirmative(text string) bool {
	words := normalizedWords(text)
	if len(words) == 0 {
		return false
	}
	joined := strings.Join(words, " ")
	for _, phrase := range []string{"go ahead", "please do", "do it"} {
		if strings.Contains(" "+joined+" ", " "+phrase+" ") {
			return true
		}
	}
	affirmatives := map[string]bool{"yes": true, "y": true, "yeah": true, "yep": true, "sure": true, "ok": true,
		"okay": true, "confirm": true, "confirmed": true, "proceed": true, "oui": true, "d'accord": true, "daccord": true}
	// Short replies only: a longer sentence that merely contains "ok"
	// (e.g. "can you book another?") must not confirm a payment.
	if len(words) > 4 {
		return false
	}
	for _, w := range words {
		if affirmatives[w] {
			return !isNegative(text)
		}
	}
	return false
}

func isNegative(text string) bool {
	words := normalizedWords(text)
	joined := " " + strings.Join(words, " ") + " "
	for _, phrase := range []string{"no thanks", "not now"} {
		if strings.Contains(joined, " "+phrase+" ") {
			return true
		}
	}
	negatives := map[string]bool{"no": true, "n": true, "nope": true, "cancel": true, "never": true, "stop": true, "abort": true, "non": true, "annuler": true}
	for _, w := range words {
		if negatives[w] {
			return true
		}
	}
	return false
}

// normalizedWords lowercases text and splits it into words, dropping punctuation.
func normalizedWords(text string) []string {
	return strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '\''
	})
}

// referencedResultID resolves a short reply to one of the last listed results.
func referencedResultID(text string, cx *domain.ConversationContext) string {
	if cx == nil || len(cx.ResultItems) == 0 || len(normalizedWords(text)) > 4 {
		return ""
	}
	trimmed := strings.TrimSpace(text)
	if n, err := strconv.Atoi(trimmed); err == nil {
		if n >= 1 && n <= len(cx.ResultItems) {
			return cx.ResultItems[n-1].ID
		}
		return ""
	}
	if ref := domain.ResolveContextualReference(trimmed, cx); ref != nil && ref.Strategy == domain.StrategyOrdinal {
		return ref.ResolvedItemID
	}
	return ""
}
