package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// ConversationUseCases interface for conversation operations.
// Implemented by conversation.ProcessorService.
type ConversationUseCases interface {
	ProcessMessage(ctx context.Context, req *conversation.ProcessConversationRequest) (*conversation.ProcessConversationResponse, error)
	ConfirmAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*conversation.ProcessConversationResponse, error)
	RejectAction(ctx context.Context, convID string, token string, actor domain.CommerceActor) (*conversation.ProcessConversationResponse, error)
}

// conversationMessageRequest is the JSON payload for a conversation message.
// Actor and OrganizationID are accepted for backward compatibility but are
// ignored: identity comes only from the verified access token.
type conversationMessageRequest struct {
	Channel               string                 `json:"channel"`
	SenderID              string                 `json:"sender_id"`
	SenderExternalID      string                 `json:"sender_external_id"`
	MessageText           string                 `json:"message_text"`
	MessageExternalRef    string                 `json:"message_external_ref,omitempty"`
	MessageIdempotencyKey string                 `json:"message_idempotency_key,omitempty"`
	Locale                string                 `json:"locale,omitempty"`
	OrganizationID        string                 `json:"organization_id,omitempty"`
	Actor                 conversationActorJSON  `json:"actor"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
}

type conversationActorJSON struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ConversationHandler handles conversation HTTP requests.
type ConversationHandler struct {
	useCases ConversationUseCases
	logger   zerolog.Logger
}

func NewConversationHandler(useCases ConversationUseCases, logger zerolog.Logger) *ConversationHandler {
	if logger.GetLevel() == 0 {
		logger = zerolog.Nop()
	}
	return &ConversationHandler{
		useCases: useCases,
		logger:   logger,
	}
}

// HandleProcessMessage handles POST /api/v1/conversations/message
func (h *ConversationHandler) HandleProcessMessage(w http.ResponseWriter, r *http.Request) {
	var httpReq conversationMessageRequest
	if err := readJSONBody(r, &httpReq); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if httpReq.MessageText == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_MESSAGE_TEXT", Message: "message_text is required"}}, http.StatusBadRequest)
		return
	}

	channel := domain.Channel(strings.ToLower(httpReq.Channel))
	if channel == "" {
		channel = domain.ChannelWeb
	}
	if !channel.IsValid() {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_CHANNEL", Message: "Invalid channel"}}, http.StatusBadRequest)
		return
	}

	if len(httpReq.MessageText) > maxMessageTextLength {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MESSAGE_TOO_LONG", Message: "message_text is too long"}}, http.StatusRequestEntityTooLarge)
		return
	}

	req := &conversation.ProcessConversationRequest{
		Channel:               channel,
		SenderID:              httpReq.SenderID,
		SenderExternalID:      httpReq.SenderExternalID,
		MessageText:           httpReq.MessageText,
		MessageIdempotencyKey: httpReq.MessageIdempotencyKey,
		Locale:                httpReq.Locale,
		Actor:                 principalActor(r),
	}

	resp, err := h.useCases.ProcessMessage(r.Context(), req)
	if err != nil {
		if errors.Is(err, domain.ErrConversationForbidden) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "This conversation belongs to another user"}}, http.StatusForbidden)
			return
		}
		h.logger.Error().Err(err).Msg("Failed to process message")
		writeJSON(w, ErrorResponse{Error: Error{Code: "PROCESSING_FAILED", Message: "The assistant could not process this message"}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, resp, http.StatusOK)
}

// HandleConfirmAction handles POST /api/v1/conversations/{id}/confirm
func (h *ConversationHandler) HandleConfirmAction(w http.ResponseWriter, r *http.Request) {
	convID := extractIDFromPath(r.URL.Path, "/api/v1/conversations/", "/confirm")
	if convID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_CONVERSATION_ID", Message: "Conversation ID required in path"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TOKEN", Message: "Confirmation token is required"}}, http.StatusBadRequest)
		return
	}

	actor := principalActor(r)
	if !actor.IsAuthenticated() {
		auth.WriteUnauthorized(w, "AUTHENTICATION_REQUIRED", "Sign in to confirm this action")
		return
	}
	resp, err := h.useCases.ConfirmAction(r.Context(), convID, req.Token, actor)
	if err != nil {
		if errors.Is(err, domain.ErrConversationForbidden) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "This conversation belongs to another user"}}, http.StatusForbidden)
			return
		}
		writeJSON(w, ErrorResponse{Error: Error{Code: "CONFIRMATION_FAILED", Message: err.Error()}}, http.StatusBadRequest)
		return
	}

	writeJSON(w, resp, http.StatusOK)
}

// HandleRejectAction handles POST /api/v1/conversations/{id}/reject
func (h *ConversationHandler) HandleRejectAction(w http.ResponseWriter, r *http.Request) {
	convID := extractIDFromPath(r.URL.Path, "/api/v1/conversations/", "/reject")
	if convID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_CONVERSATION_ID", Message: "Conversation ID required in path"}}, http.StatusBadRequest)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_TOKEN", Message: "Confirmation token is required"}}, http.StatusBadRequest)
		return
	}

	actor := principalActor(r)
	if !actor.IsAuthenticated() {
		auth.WriteUnauthorized(w, "AUTHENTICATION_REQUIRED", "Sign in to reject this action")
		return
	}
	resp, err := h.useCases.RejectAction(r.Context(), convID, req.Token, actor)
	if err != nil {
		if errors.Is(err, domain.ErrConversationForbidden) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "This conversation belongs to another user"}}, http.StatusForbidden)
			return
		}
		writeJSON(w, ErrorResponse{Error: Error{Code: "REJECTION_FAILED", Message: err.Error()}}, http.StatusBadRequest)
		return
	}

	writeJSON(w, resp, http.StatusOK)
}

func extractIDFromPath(path, prefix, suffix string) string {
	if strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix) {
		return path[len(prefix) : len(path)-len(suffix)]
	}
	return ""
}

// maxMessageTextLength bounds a single chat message.
const maxMessageTextLength = 4000

// principalActor returns the verified caller as a commerce actor, or a guest.
func principalActor(r *http.Request) domain.CommerceActor {
	if id := auth.UserID(r.Context()); id != "" {
		return domain.NewCommerceActor(id, domain.ActorTypeCustomer)
	}
	return domain.NewCommerceActor("", domain.ActorTypeGuest)
}

// HandleConversationRoutes routes conversation-related API requests.
func HandleConversationRoutes(handler *ConversationHandler, w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	switch {
	case path == "/api/v1/conversations/message" && method == "POST":
		handler.HandleProcessMessage(w, r)
		return
	case strings.HasPrefix(path, "/api/v1/conversations/") && strings.HasSuffix(path, "/confirm") && method == "POST":
		handler.HandleConfirmAction(w, r)
		return
	case strings.HasPrefix(path, "/api/v1/conversations/") && strings.HasSuffix(path, "/reject") && method == "POST":
		handler.HandleRejectAction(w, r)
		return
	case path == "/api/v1/conversations" && method == "POST":
		handler.HandleProcessMessage(w, r)
		return
	}

	writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: fmt.Sprintf("Route not found: %s %s", method, path)}}, http.StatusNotFound)
}
