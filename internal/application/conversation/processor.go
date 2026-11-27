package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"github.com/trezutil/trezutil/internal/application/vertical"
	"strconv"
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
	// Context summarises the conversation so far (last search, numbered results,
	// the listing in focus, today's date) for the interpreter.
	Context string
	// Memory is what the signed-in person asked Trezutil to remember ("brand: Dell"), as hints. It is data the person
	// wrote, never instructions; empty for a guest or when the memory is off.
	Memory []string
}

// MemoryHints gives the assistant a person's commercial memory (memory.Service).
type MemoryHints interface {
	Hints(ctx context.Context, userID string) []string
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
	memory             MemoryHints
	replyWriter        ReplyWriter
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
	// Memory, when set, gives the interpreter the signed-in person's commercial memory.
	Memory MemoryHints
	// ReplyWriter, when set, has the model word the answers that report what was found (stream.go).
	ReplyWriter ReplyWriter
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
		memory:             config.Memory,
		replyWriter:        config.ReplyWriter,
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
			// Without the stored message a redelivery would run the action again: stop here.
			p.logger.Error().Err(err).Str("conversation_id", conv.ID).Msg("Failed to persist inbound message")
			return nil, fmt.Errorf("%w: %v", domain.ErrMessageNotStored, err)
		}
	}

	result, err := p.dispatch(ctx, conv, req)
	if err != nil {
		return nil, err
	}
	p.writeReply(ctx, conv, req, result)
	// The answer is stored even when the person stopped waiting (closed the stream): the action ran, and asking
	// again must give back this answer, not run it a second time.
	p.persistReply(context.WithoutCancel(ctx), conv, messageKey, result)
	return result, nil
}

// writeReply has the model word an answer that reports what was found, streaming it as it is written. The drafted
// reply stays when there is no writer, when the answer must keep the server's exact words (writingEligible), when
// the model fails or is stopped, or when the written text states a number the server did not establish.
func (p *ConversationProcessor) writeReply(ctx context.Context, conv *domain.Conversation, req *ProcessRequest, result *ProcessResult) {
	if p.replyWriter == nil || !writingEligible(result) || ctx.Err() != nil {
		return
	}
	// A writer that knows it cannot write now (paused after a provider failure) is not asked, so the stream does not
	// announce a "writing" step that will not happen.
	if r, ok := p.replyWriter.(interface{ Ready() bool }); ok && !r.Ready() {
		return
	}
	sink := SinkFrom(ctx)
	sink.Stage("writing", "")
	draft := result.Response.Text
	facts := replyFacts(result.Response)
	actionType, _ := result.Response.Metadata["action_type"].(string)
	sent := false
	text, err := p.replyWriter.WriteReply(ctx, ReplyWritingInput{
		OrganizationID: conv.OrganizationID, Channel: req.Channel, Actor: req.Actor,
		Language: result.Response.Language, CustomerMessage: req.MessageText, ActionType: actionType,
		Draft: draft, Facts: facts,
	}, func(piece string) {
		if piece != "" {
			sent = true
			sink.Delta(piece)
		}
	})
	text = strings.TrimSpace(text)
	if err != nil || text == "" || !numbersAreGrounded(text, draft, facts, req.MessageText) {
		if err == nil && text != "" {
			p.logger.Warn().Str("conversation_id", conv.ID).Msg("Written answer dropped: it states a number the server did not establish")
		}
		if sent {
			sink.Reset()
		}
		return
	}
	result.Response.Text = text
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
	}
	cx.Language = resolveLanguage(req.Locale, cx.Language)

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
	lang := resolveLanguage("", conv.Language)
	var stored *domain.Message
	if p.messageStore != nil {
		if history, err := p.messageStore.FindByConversationID(ctx, conv.ID); err == nil {
			for _, m := range history {
				if m != nil && m.IdempotencyKey == "reply:"+messageKey && m.Content != nil {
					stored = m
				}
			}
		}
	}
	if stored == nil {
		// The first delivery is still being answered (or its answer was lost): say so, so the client asks again
		// in a moment instead of showing this as the answer.
		resp := domain.NewConversationResponse(tr(lang, "I'm still working on that message — one moment.", "Je traite encore ce message — un instant."), lang)
		resp.Metadata["duplicate"] = true
		resp.Metadata["in_progress"] = true
		return &ProcessResult{ConversationID: conv.ID, Response: resp}
	}
	resp := domain.NewConversationResponse(stored.Content.Text, lang)
	if stored.Language != "" {
		resp.Language = stored.Language
	}
	// The cards and results the answer showed come back with it (never a confirmation: see ReplyRecordKey).
	if record := StoredReply(stored.Content.Data); record != nil {
		if raw, err := json.Marshal(record["cards"]); err == nil {
			_ = json.Unmarshal(raw, &resp.Cards)
		}
		if meta, ok := record["metadata"].(map[string]interface{}); ok {
			for k, v := range meta {
				resp.Metadata[k] = v
			}
		}
	}
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
		Data: replyRecord(result.Response),
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

		resp := domain.NewConversationResponse(tr(cx.Language, "No problem — I've cancelled that. Nothing was changed.", "Pas de souci — j’ai annulé. Rien n’a été modifié."), cx.Language)
		return &ProcessResult{
			ConversationID: conv.ID,
			Response:       resp,
		}, nil

	default:
		resp := domain.NewConversationResponse(tr(cx.Language, "Just a quick yes or no, please — shall I go ahead?", "Juste un oui ou un non, s’il vous plaît — je continue ?"), cx.Language)
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
	// The person changed the subject instead of answering: drop the pending
	// question and read the message as a new request.
	if !answersMissingParam(param.Name, req.MessageText) {
		cx.MissingParameters = nil
		cx.PendingAction = nil
		_ = p.saveContext(ctx, conv.ID, cx)
		return p.handleNewMessage(ctx, conv, cx, req)
	}
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
		resp := domain.NewConversationResponse(tr(cx.Language, "Sorry, I didn't quite catch that. Could you say it another way? For example: “a hotel in Gombe this weekend”.", "Désolé, je n’ai pas bien compris. Pouvez-vous le dire autrement ? Par exemple : « un hôtel à Gombe ce week-end »."), cx.Language)
		return &ProcessResult{ConversationID: conv.ID, Response: resp}, nil
	}
	if intent == nil {
		resp := domain.NewConversationResponse(tr(cx.Language, "Happy to help! I can find products, services, restaurants, places to stay and homes — then book or order them with you. What are you looking for?", "Avec plaisir ! Je peux trouver des produits, des services, des restaurants, des hébergements et des logements — puis les réserver ou les commander avec vous. Que cherchez-vous ?"), cx.Language)
		resp.Metadata["suggestions"] = welcomeSuggestions(cx.Language)
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
	completeFromMessage(action, req.MessageText, cx)
	if listingScopedActions[action.ActionType] {
		if v, ok := action.Parameters["listing_id"]; !ok || v == "" {
			if id := currentListingID(cx); id != "" {
				action.Parameters["listing_id"] = id
			}
		}
	}
	if action.IdempotencyKey == "" {
		action.IdempotencyKey = fmt.Sprintf("action_%s_%s", conv.ID, domain.NewID())
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
	// A reply pointing at a previous result ("2", "le deuxième",
	// "Parle-moi du numéro 1") opens that listing, or orders or books it.
	// Longer messages and requests to act go to the model first, which
	// also reads dates and quantities; the keyword fallback resolves them too.
	if intent := resultReferenceIntent(req.MessageText, cx); intent != nil && intent.ActionType == domain.ActionGetListing && isShortReference(req.MessageText) {
		return intent, nil
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
			Context:        classifierContext(cx, req.Actor.IsAuthenticated(), cx.Language, time.Now()),
		}
		if p.memory != nil && req.Actor.IsAuthenticated() {
			classReq.Memory = p.memory.Hints(ctx, req.Actor.ID)
		}

		SinkFrom(ctx).Stage("interpreting", "")
		classResult, err := p.classifier.ClassifyIntent(ctx, classReq)
		if err != nil || classResult == nil || classResult.Fallback != nil || classResult.Intent == nil {
			// The model is unavailable or unsure: degrade to deterministic
			// keyword routing instead of failing the conversation.
			if err != nil {
				p.logger.Warn().Err(err).Msg("AI classification failed; using keyword routing")
			}
			return keywordIntent(p, ctx, conv, cx, req), nil
		}

		if classResult.Intent.Intent == domain.ActionSearchListings {
			classResult.Intent.Parameters = enrichSearchParams(classResult.Intent.Parameters, req.MessageText, cx.SearchCriteria)
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
	return keywordIntent(p, ctx, conv, cx, req), nil
}

// keywordIntent is the deterministic, AI-free intent resolver. For searches
// the user's own words are the query text.
func keywordIntent(p *ConversationProcessor, ctx context.Context, conv *domain.Conversation, cx *domain.ConversationContext, req *ProcessRequest) *domain.ConversationIntent {
	if intent := resultReferenceIntent(req.MessageText, cx); intent != nil {
		return intent
	}
	// A follow-up that only adjusts the last search ("moins cher", "et à Limete").
	if cx != nil {
		if merged, ok := refineSearch(cx.SearchCriteria, req.MessageText); ok {
			intent := &domain.ConversationIntent{
				Name:        "KEYWORD_REFINEMENT",
				Confidence:  1.0,
				Source:      "SYSTEM",
				ActionType:  domain.ActionSearchListings,
				Specialist:  p.resolveSpecialist(domain.ActionSearchListings),
				Entities:    domain.EntitiesFromParams(merged),
				Explanation: "refines the previous search",
			}
			intent.VerticalPack, _ = p.resolvePack(ctx, conv.OrganizationID, domain.ActionSearchListings)
			return intent
		}
	}
	actionType := resolveActionTypeKeyword(req.MessageText)
	// Naming a kind of place or offer ("un plombier disponible aujourd'hui",
	// "un resto à Limete") is a search, whatever other keyword is in the text.
	if v, _ := vertical.DefaultCatalog().DetectVertical(req.MessageText); v != "" && (actionType == "" || actionType == domain.ActionCheckAvailability) {
		actionType = domain.ActionSearchListings
	}
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
		intent.Entities = domain.EntitiesFromParams(keywordSearchParams(req.MessageText))
		if len(intent.Entities) == 0 {
			intent.Entities = []domain.ConversationEntity{{Name: "text", Value: req.MessageText, NormalizedValue: req.MessageText, Source: "USER"}}
		}
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
		resp := domain.NewConversationResponse(tr(cx.Language, "To go ahead I'll need you signed in — it only takes your phone number. Browsing is open to everyone, but orders, bookings and payments need an account.", "Pour continuer, il faut que vous soyez connecté — il suffit de votre numéro de téléphone. Naviguer est ouvert à tous, mais les commandes, réservations et paiements nécessitent un compte."), cx.Language)
		resp.Metadata["requires_authentication"] = true
		resp.Metadata["action_type"] = string(action.ActionType)
		_ = p.saveContext(ctx, conv.ID, cx)
		return &ProcessResult{ConversationID: conv.ID, Response: resp, RequiresUserInput: true}, nil
	}

	SinkFrom(ctx).Stage("acting", string(action.ActionType))
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
		result.Response = domain.NewConversationResponse(tr(cx.Language, "I'm sorry, you don't have permission to do that.", "Désolé, vous n’avez pas la permission de faire cela."), cx.Language)

	case domain.ActionResultNotFound:
		result.Response = domain.NewConversationResponse(tr(cx.Language, "I couldn't find that one. It may have been removed — could you check the details, or tell me what you're after and I'll search again?", "Je n’ai pas trouvé celui-ci. Il a peut-être été supprimé — pouvez-vous vérifier les détails, ou me dire ce que vous cherchez pour que je relance la recherche ?"), cx.Language)

	case domain.ActionResultNotImplemented:
		result.Response = domain.NewConversationResponse(tr(cx.Language, "I can't do that from the chat just yet, but you can do it from the listing page in the Trezutil app.", "Je ne peux pas encore faire cela depuis la conversation, mais vous pouvez le faire depuis la page de l’annonce dans l’application Trezutil."), cx.Language)
		cx.PendingAction = nil

	default:
		result.Response = domain.NewConversationResponse(tr(cx.Language, "Hmm, something went wrong on my side. Could you try that again in a moment?", "Hmm, quelque chose s’est mal passé de mon côté. Pouvez-vous réessayer dans un instant ?"), cx.Language)
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
		result.Response = domain.NewConversationResponse(tr(cx.Language, "I wasn't able to finish that. Could you try again in a moment?", "Je n’ai pas pu terminer. Pouvez-vous réessayer dans un instant ?"), cx.Language)
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
	resp := domain.NewConversationResponse(successText(language, action.ActionType, data), language)
	resp.Metadata["action_type"] = string(action.ActionType)
	resp.Metadata["source"] = "conversation_processor"
	resp.Metadata["action_id"] = action.ActionID
	if data != nil {
		resp.Metadata["result_data"] = data
	}
	if sugg := suggestionsFor(language, action.ActionType, data); len(sugg) > 0 {
		resp.Metadata["suggestions"] = sugg
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
func successText(lang string, actionType domain.CommerceActionType, data interface{}) string {
	switch actionType {
	case domain.ActionSearchListings:
		return searchText(lang, data, extractResultItems(data))
	case domain.ActionGetListing:
		if l, ok := data.(*domain.Listing); ok && l != nil {
			return listingText(lang, l)
		}
	case domain.ActionPauseListing:
		return tr(lang, "Done — the listing is paused and hidden from search. You can bring it back whenever you like.", "C’est fait — l’annonce est en pause et masquée de la recherche. Vous pouvez la remettre en ligne quand vous voulez.")
	case domain.ActionResumeListing, domain.ActionPublishListing:
		return tr(lang, "Done — the listing is live and searchable again.", "C’est fait — l’annonce est de nouveau en ligne et consultable.")
	case domain.ActionCreateOrder:
		if m, ok := data.(map[string]interface{}); ok {
			return fmt.Sprintf(tr(lang, "All set! Your order is created (ref %v). You can pay for it whenever you're ready.", "C’est bon ! Votre commande est créée (réf. %v). Vous pourrez la payer quand vous serez prêt."), m["order_id"])
		}
		return tr(lang, "All set! Your order is created. You can pay for it whenever you're ready.", "C’est bon ! Votre commande est créée. Vous pourrez la payer quand vous serez prêt.")
	case domain.ActionCreateReservation:
		return tr(lang, "You're all set — your booking is confirmed, and I've saved it to your reservations.", "Tout est bon — votre réservation est confirmée, et je l’ai enregistrée dans vos réservations.")
	case domain.ActionCancelReservation, domain.ActionCancelOrder:
		return tr(lang, "Done — that's cancelled. Anything else I can help with?", "C’est fait — c’est annulé. Puis-je vous aider avec autre chose ?")
	case domain.ActionCheckAvailability:
		if m, ok := data.(map[string]interface{}); ok {
			if avail, _ := m["available"].(bool); avail {
				return tr(lang, "Good news — it's available for those dates. Want me to book it?", "Bonne nouvelle — c’est disponible à ces dates. Voulez-vous que je le réserve ?")
			}
			return tr(lang, "Sorry, it's not available for those dates. Would you like to try other dates, or see something similar?", "Désolé, ce n’est pas disponible à ces dates. Voulez-vous essayer d’autres dates, ou voir quelque chose de similaire ?")
		}
	}
	return tr(lang, "Done — that's taken care of.", "C’est fait — c’est réglé.")
}

func (p *ConversationProcessor) buildConfirmationResponse(
	action *domain.CommerceAction,
	result *commerce.ExecuteResult,
	language string,
) *domain.ConversationResponse {
	resp := domain.NewConversationResponse(tr(language, "Almost there — I just need your go-ahead before I do this.", "Presque terminé — il me faut juste votre accord avant de continuer."), language)
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
		return domain.NewConversationResponse(tr(language, "Could you tell me a little more about what you need?", "Pouvez-vous m’en dire un peu plus sur ce dont vous avez besoin ?"), language)
	}

	param := missing[0]
	resp := domain.NewConversationResponse(p.missingParamPrompt(language, param), language)
	resp.Metadata["source"] = "clarification_requested"
	resp.Metadata["missing_param"] = param.Name
	resp.Metadata["missing_param_description"] = param.Description

	if len(param.Examples) > 0 {
		resp.Metadata["examples"] = param.Examples
	}
	if sugg := clarificationSuggestions(language, param.Name); len(sugg) > 0 {
		resp.Metadata["suggestions"] = sugg
	}

	if action != nil {
		resp.Metadata["action_type"] = string(action.ActionType)
	}

	return resp
}

func (p *ConversationProcessor) missingParamPrompt(lang string, param domain.MissingParameter) string {
	if isFrench(lang) {
		if prompt, ok := friendlyParamPromptsFR[param.Name]; ok {
			return prompt
		}
	}
	if prompt, ok := friendlyParamPrompts[param.Name]; ok {
		return prompt
	}
	prompt := fmt.Sprintf("Just one more thing — %s. What would you like to use?", lowerFirst(strings.TrimSuffix(strings.TrimSpace(param.Description), ".")))
	if len(param.Examples) > 0 && len(param.Examples) <= 3 {
		prompt += fmt.Sprintf(" (e.g., %s)", strings.Join(param.Examples, ", "))
	}
	return prompt
}

var friendlyParamPrompts = map[string]string{
	"text":           "Sure — what are you looking for? You can say things like “a hotel in Gombe” or “an iPhone 13”.",
	"listing_id":     "Which one do you mean? Reply with its number from the list (like “2” or “the second one”).",
	"start_at":       "When would you like it to start? (for example 2026-10-01 14:00)",
	"end_at":         "And when should it end? (for example 2026-10-03 11:00)",
	"quantity":       "How many would you like?",
	"vertical":       "What kind of place are you after — a hotel, a restaurant, an apartment?",
	"location":       "Which area should I look in? (for example Gombe)",
	"nights":         "For how many nights?",
	"party_size":     "How many people will it be?",
	"order_id":       "Which order do you mean? Please share its reference.",
	"reservation_id": "Which reservation do you mean? Please share its reference.",
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
			p.storeOpenedListing(ctx, conv.ID, cx, data)
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
	cx.CurrentListing = nil
}

// storeOpenedListing remembers the offer just opened ("achète-le" then means
// it). An offer opened from the listed results keeps that list, so "et le
// deuxième ?" still points into it.
func (p *ConversationProcessor) storeOpenedListing(ctx context.Context, convID string, cx *domain.ConversationContext, data interface{}) {
	l, ok := data.(*domain.Listing)
	if !ok || l == nil {
		p.storeSingleResult(ctx, convID, cx, "listing", data)
		return
	}
	cx.CurrentListing = &domain.ListingRef{ID: l.ID, Title: l.Title, OrganizationID: l.OrganizationID, Status: string(l.Status)}
	if len(cx.ResultItems) > 1 {
		for _, item := range cx.ResultItems {
			if item.ID == l.ID {
				return
			}
		}
	}
	p.storeSingleResult(ctx, convID, cx, "listing", data)
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

	resp := domain.NewConversationResponse(tr(cx.Language, "I've cancelled that action.", "J’ai annulé cette action."), cx.Language)
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
			Vertical:   domain.NormalizeVertical(str("vertical")),
			Location:   str("location"),
			Nights:     num("nights"),
			PartySize:  num("party_size"),
			StartAt:    str("start_at"),
			Text:       str("text"),
			Sort:       str("sort"),
			CategoryID: str("category_id"),
		}
		c.PriceMin, _ = strconv.ParseFloat(str("price_min"), 64)
		c.PriceMax, _ = strconv.ParseFloat(str("price_max"), 64)
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
	if cx != nil && cx.CurrentListing != nil && cx.CurrentListing.ID != "" {
		return cx.CurrentListing.ID
	}
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
	// Asking whether something is free is browsing; booking it still needs an account.
	domain.ActionCheckAvailability: true,
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
		strings.Contains(lower, "trouv") || strings.Contains(lower, "montre") || strings.Contains(lower, "affich") || strings.Contains(lower, "recommand") ||
		strings.Contains(lower, "look") || strings.Contains(lower, "browse") ||
		strings.Contains(lower, "catalog") || strings.Contains(lower, "list") ||
		strings.Contains(lower, "suggest") || strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "option") || strings.Contains(lower, "alternative") {
		return domain.ActionSearchListings
	}

	if strings.Contains(lower, "available") || strings.Contains(lower, "check") || strings.Contains(lower, "disponib") ||
		strings.Contains(lower, "availability") {
		return domain.ActionCheckAvailability
	}

	if strings.Contains(lower, "reserve") || strings.Contains(lower, "book") || strings.Contains(lower, "réserv") {
		return domain.ActionCreateReservation
	}

	if strings.Contains(lower, "my reservation") || strings.Contains(lower, "view booking") {
		return domain.ActionGetReservation
	}

	if strings.Contains(lower, "cancel") || strings.Contains(lower, "annuler") {
		return domain.ActionCancelReservation
	}

	if strings.Contains(lower, "buy") || strings.Contains(lower, "order") || strings.Contains(lower, "achet") || strings.Contains(lower, "achèt") || strings.Contains(lower, "achat") || strings.Contains(lower, "command") || strings.Contains(lower, "payer") ||
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

// A number introduced as a position in the list: "numéro 1", "n° 2",
// "#3", "l'offre 2", "option 1", "result 2".
var resultNumberRef = regexp.MustCompile(`(?i)(?:\bnum[ée]ro|\bn[°º]|#|\boffres?|\boptions?|\br[ée]sultats?|\bresults?|\bnumber)\s*(\d{1,2})\b`)

// A position said as a word: "le premier", "la 2e", "the second one", "le dernier".
var resultOrdinalRef = regexp.MustCompile(`(?i)(?:^|[\s'’])(premi[eè]re?|1(?:er|re|ère)|deuxi[eè]me|2(?:e|ème|eme|nd)|seconde?|troisi[eè]me|3(?:e|ème|eme|rd)|quatri[eè]me|4(?:e|ème|eme|th)|cinqui[eè]me|5(?:e|ème|eme|th)|derni[eè]re?|first|third|fourth|fifth|last)(?:$|[\s.!?,])`)

var resultOrdinals = map[string]int{
	"premier": 1, "première": 1, "premiere": 1, "1er": 1, "1re": 1, "1ère": 1, "first": 1,
	"deuxième": 2, "deuxieme": 2, "2e": 2, "2ème": 2, "2eme": 2, "2nd": 2, "second": 2, "seconde": 2,
	"troisième": 3, "troisieme": 3, "3e": 3, "3ème": 3, "3eme": 3, "3rd": 3, "third": 3,
	"quatrième": 4, "quatrieme": 4, "4e": 4, "4ème": 4, "4eme": 4, "4th": 4, "fourth": 4,
	"cinquième": 5, "cinquieme": 5, "5e": 5, "5ème": 5, "5eme": 5, "5th": 5, "fifth": 5,
	"dernier": -1, "dernière": -1, "derniere": -1, "last": -1,
}

// referencedResultID resolves a reply pointing at one of the last listed
// results: a bare number ("2"), a numbered reference anywhere in the message
// ("Parle-moi du numéro 1", "l’offre 2 : Collier en or") or, in a short
// reply, an ordinal word ("le deuxième", "the last one").
func referencedResultID(text string, cx *domain.ConversationContext) string {
	if cx == nil || len(cx.ResultItems) == 0 {
		return ""
	}
	items := cx.ResultItems
	at := func(n int) string {
		if n == -1 {
			return items[len(items)-1].ID
		}
		if n >= 1 && n <= len(items) {
			return items[n-1].ID
		}
		return ""
	}
	trimmed := strings.TrimSpace(strings.TrimRight(strings.TrimSpace(text), ".!?"))
	if n, err := strconv.Atoi(trimmed); err == nil {
		return at(n)
	}
	if m := resultNumberRef.FindStringSubmatch(text); m != nil {
		n, _ := strconv.Atoi(m[1])
		return at(n)
	}
	// Ordinal words also appear in ordinary requests ("le premier prix",
	// "first class"), so only a short reply counts as pointing at the list.
	if len(strings.Fields(text)) > 6 {
		return ""
	}
	if m := resultOrdinalRef.FindStringSubmatch(strings.ToLower(text)); m != nil {
		if n, ok := resultOrdinals[m[1]]; ok {
			return at(n)
		}
	}
	return ""
}

// isShortReference reports whether the message is little more than the
// reference itself: "2", "le deuxième", "Parle-moi de l’offre 1 : <titre>".
func isShortReference(text string) bool {
	if i := strings.Index(text, ":"); i >= 0 {
		text = text[:i]
	}
	return len(strings.Fields(text)) <= 6
}

// resultReferenceIntent opens the referenced result, or acts on it when the
// message asks to buy, book or check it ("je prends le numéro 2").
func resultReferenceIntent(text string, cx *domain.ConversationContext) *domain.ConversationIntent {
	id := referencedResultID(text, cx)
	if id == "" {
		return nil
	}
	action := domain.ActionGetListing
	// The words after a colon are the offer's own title ("l’offre 2 : Commande
	// groupée de pagnes"): only the user's part says what to do.
	ask := text
	if i := strings.IndexAny(ask, ":"); i >= 0 {
		ask = ask[:i]
	}
	switch a := resolveActionTypeKeyword(ask); a {
	case domain.ActionCreateOrder, domain.ActionCreateReservation, domain.ActionCheckAvailability:
		action = a
	}
	return &domain.ConversationIntent{
		Name:                 "RESULT_REFERENCE",
		Confidence:           1.0,
		Source:               "SYSTEM",
		ActionType:           action,
		RequiresConfirmation: domain.AIActionRequiresConfirmation(action),
		Entities:             []domain.ConversationEntity{{Name: "listing_id", Value: id, NormalizedValue: id, Source: "CONTEXT"}},
	}
}
