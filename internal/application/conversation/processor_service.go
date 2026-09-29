package conversation

import (
	"context"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ProcessConversationRequest is the request for processing a conversation message.
type ProcessConversationRequest struct {
	Channel          domain.Channel
	SenderID         string
	SenderExternalID string
	MessageText      string
	// MessageIdempotencyKey deduplicates client retries / webhook redeliveries.
	MessageIdempotencyKey string
	Locale                string
	TenantID              string
	Actor                 domain.CommerceActor
}

// ProcessConversationResponse is the response from processing a conversation message.
type ProcessConversationResponse struct {
	ConversationID    string                       `json:"conversation_id"`
	Response          *domain.ConversationResponse `json:"response"`
	RequiresUserInput bool                         `json:"requires_user_input"`
	IsConfirmation    bool                         `json:"is_confirmation"`
	IsClarification   bool                         `json:"is_clarification"`
	MissingParams     []domain.MissingParameter    `json:"missing_params,omitempty"`
}

// ProcessorService adapts ConversationProcessor to the ConversationUseCases interface.
// It provides a clean API for the HTTP layer to invoke the processor.
type ProcessorService struct {
	processor *ConversationProcessor
}

// NewProcessorService creates a new ProcessorService.
func NewProcessorService(processor *ConversationProcessor) *ProcessorService {
	return &ProcessorService{processor: processor}
}

// ProcessMessage processes an inbound message through the conversation processor.
func (s *ProcessorService) ProcessMessage(ctx context.Context, req *ProcessConversationRequest) (*ProcessConversationResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	actorType := req.Actor.Type
	if actorType == "" {
		actorType = domain.ActorTypeCustomer
	}

	processReq := &ProcessRequest{
		Channel:               req.Channel,
		SenderID:              req.SenderID,
		SenderExternalID:      req.SenderExternalID,
		MessageText:           req.MessageText,
		MessageIdempotencyKey: req.MessageIdempotencyKey,
		Locale:                req.Locale,
		TenantID:              req.TenantID,
		Actor:                 req.Actor,
	}

	result, err := s.processor.ProcessMessage(ctx, processReq)
	if err != nil {
		return nil, err
	}

	return &ProcessConversationResponse{
		ConversationID:    result.ConversationID,
		Response:          result.Response,
		RequiresUserInput: result.RequiresUserInput,
		IsConfirmation:    result.IsConfirmation,
		IsClarification:   result.IsClarification,
		MissingParams:     result.MissingParams,
	}, nil
}

// ConfirmAction confirms a pending action.
func (s *ProcessorService) ConfirmAction(ctx context.Context, convID, token string, actor domain.CommerceActor) (*ProcessConversationResponse, error) {
	result, err := s.processor.ConfirmAction(ctx, convID, token, actor)
	if err != nil {
		return nil, err
	}
	return s.toResponse(result), nil
}

// RejectAction rejects a pending confirmation.
func (s *ProcessorService) RejectAction(ctx context.Context, convID, token string, actor domain.CommerceActor) (*ProcessConversationResponse, error) {
	result, err := s.processor.RejectAction(ctx, convID, token, actor)
	if err != nil {
		return nil, err
	}
	return s.toResponse(result), nil
}

func (s *ProcessorService) toResponse(result *ProcessResult) *ProcessConversationResponse {
	if result == nil {
		return nil
	}
	return &ProcessConversationResponse{
		ConversationID:    result.ConversationID,
		Response:          result.Response,
		RequiresUserInput: result.RequiresUserInput,
		IsConfirmation:    result.IsConfirmation,
		IsClarification:   result.IsClarification,
		MissingParams:     result.MissingParams,
	}
}
