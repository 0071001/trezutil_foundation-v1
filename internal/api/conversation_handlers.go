package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
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

// ConversationHistory reads back a person's own conversations (conversation.HistoryService).
type ConversationHistory interface {
	List(ctx context.Context, userID string, channel domain.Channel, limit int) ([]domain.ConversationHistoryItem, error)
	Transcript(ctx context.Context, userID, conversationID string) (*domain.Conversation, []*domain.Message, error)
	Rename(ctx context.Context, userID, conversationID, title string) error
	SetArchived(ctx context.Context, userID, conversationID string, archived bool) error
	Delete(ctx context.Context, userID, conversationID string) error
}

// ConversationHandler handles conversation HTTP requests.
type ConversationHandler struct {
	useCases ConversationUseCases
	history  ConversationHistory
	logger   zerolog.Logger
}

// WithHistory enables GET /api/v1/conversations and GET /api/v1/conversations/{id}/messages.
func (h *ConversationHandler) WithHistory(history ConversationHistory) *ConversationHandler {
	h.history = history
	return h
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
		writeJSON(w, ErrorResponse{Error: Error{Code: "CONFIRMATION_FAILED", Message: "The confirmation could not be applied"}}, http.StatusBadRequest)
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
		writeJSON(w, ErrorResponse{Error: Error{Code: "REJECTION_FAILED", Message: "The rejection could not be applied"}}, http.StatusBadRequest)
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
	case path == "/api/v1/conversations" && method == "GET":
		handler.HandleListConversations(w, r)
		return
	case strings.HasPrefix(path, "/api/v1/conversations/") && strings.HasSuffix(path, "/messages") && method == "GET":
		handler.HandleConversationMessages(w, r)
		return
	case conversationIDFromPath(path) != "" && method == "PATCH":
		handler.HandleUpdateConversation(w, r)
		return
	case conversationIDFromPath(path) != "" && method == "DELETE":
		handler.HandleDeleteConversation(w, r)
		return
	}

	writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: fmt.Sprintf("Route not found: %s %s", method, path)}}, http.StatusNotFound)
}

// conversationSummaryJSON is one conversation of GET /api/v1/conversations.
type conversationSummaryJSON struct {
	ID             string `json:"id"`
	Channel        string `json:"channel"`
	ExternalID     string `json:"external_id"`
	Title          string `json:"title"`
	MessageCount   int    `json:"message_count"`
	CreatedAt      int64  `json:"created_at"`
	LastActivityAt int64  `json:"last_activity_at"`
	Renamed        bool   `json:"renamed"`
	Archived       bool   `json:"archived"`
}

// transcriptMessageJSON is one message of GET /api/v1/conversations/{id}/messages: its text and, for an answer, the
// cards and results it showed (conversation.ReplyRecordKey). Never the other data the processor keeps with a message.
type transcriptMessageJSON struct {
	ID        string                 `json:"id"`
	Direction string                 `json:"direction"`
	Text      string                 `json:"text"`
	CreatedAt int64                  `json:"created_at"`
	Response  map[string]interface{} `json:"response,omitempty"`
}

// conversationIDFromPath returns {id} of /api/v1/conversations/{id} (nothing after it), or "".
func conversationIDFromPath(path string) string {
	id := strings.TrimPrefix(path, "/api/v1/conversations/")
	if id == path || id == "" || strings.Contains(id, "/") || id == "message" {
		return ""
	}
	return id
}

// writeHistoryChangeError answers a failed rename, archive or delete.
func (h *ConversationHandler) writeHistoryChangeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, conversation.ErrConversationNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Conversation not found"}}, http.StatusNotFound)
	case errors.Is(err, conversation.ErrHistoryReadOnly):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Conversations cannot be changed here"}}, http.StatusServiceUnavailable)
	default:
		h.logger.Error().Err(err).Msg("Failed to change a conversation")
		writeJSON(w, ErrorResponse{Error: Error{Code: "HISTORY_FAILED", Message: "The conversation could not be changed"}}, http.StatusInternalServerError)
	}
}

// HandleUpdateConversation handles PATCH /api/v1/conversations/{id} with {"title": "…"} and/or {"archived": true}:
// the owner renames or archives their conversation. An empty title gives it back its first message as name.
// 404 for an unknown id and for somebody else's conversation alike.
func (h *ConversationHandler) HandleUpdateConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	if h.history == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Conversation history is not available"}}, http.StatusServiceUnavailable)
		return
	}
	var req struct {
		Title    *string `json:"title"`
		Archived *bool   `json:"archived"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	if req.Title == nil && req.Archived == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOTHING_TO_CHANGE", Message: "Send title and/or archived"}}, http.StatusBadRequest)
		return
	}
	if req.Title != nil && len([]rune(*req.Title)) > 4*conversation.MaxTitleLength {
		writeJSON(w, ErrorResponse{Error: Error{Code: "TITLE_TOO_LONG", Message: "title is too long"}}, http.StatusBadRequest)
		return
	}
	id := conversationIDFromPath(r.URL.Path)
	if req.Title != nil {
		if err := h.history.Rename(r.Context(), userID, id, *req.Title); err != nil {
			h.writeHistoryChangeError(w, err)
			return
		}
	}
	if req.Archived != nil {
		if err := h.history.SetArchived(r.Context(), userID, id, *req.Archived); err != nil {
			h.writeHistoryChangeError(w, err)
			return
		}
	}
	out := map[string]interface{}{"id": id}
	if req.Title != nil {
		out["title"] = conversation.CleanTitle(*req.Title)
	}
	if req.Archived != nil {
		out["archived"] = *req.Archived
	}
	writeJSON(w, out, http.StatusOK)
}

// HandleDeleteConversation handles DELETE /api/v1/conversations/{id}: the owner deletes their conversation and
// everything stored with it. Orders and reservations it created are not deleted. 204 on success.
func (h *ConversationHandler) HandleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	if h.history == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Conversation history is not available"}}, http.StatusServiceUnavailable)
		return
	}
	if err := h.history.Delete(r.Context(), userID, conversationIDFromPath(r.URL.Path)); err != nil {
		h.writeHistoryChangeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// HandleListConversations handles GET /api/v1/conversations?channel=web&limit=50: the caller's own conversations,
// the most recently active first. Signed-in users only.
func (h *ConversationHandler) HandleListConversations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	if h.history == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Conversation history is not available"}}, http.StatusServiceUnavailable)
		return
	}
	channel := domain.Channel(strings.ToLower(r.URL.Query().Get("channel")))
	if channel == "" {
		channel = domain.ChannelWeb
	}
	if !channel.IsValid() {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_CHANNEL", Message: "Invalid channel"}}, http.StatusBadRequest)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_LIMIT", Message: "limit must be between 1 and 100"}}, http.StatusBadRequest)
			return
		}
		limit = n
	}
	list, err := h.history.List(r.Context(), userID, channel, limit)
	if err != nil {
		h.logger.Error().Err(err).Msg("Failed to list conversations")
		writeJSON(w, ErrorResponse{Error: Error{Code: "HISTORY_FAILED", Message: "The conversations could not be read"}}, http.StatusInternalServerError)
		return
	}
	items := make([]conversationSummaryJSON, 0, len(list))
	for _, c := range list {
		items = append(items, conversationSummaryJSON{
			ID: c.ID, Channel: string(c.Channel), ExternalID: c.ExternalID, Title: c.Title,
			MessageCount: c.MessageCount, CreatedAt: c.CreatedAt, LastActivityAt: c.LastActivityAt,
			Renamed: c.Renamed, Archived: c.Archived,
		})
	}
	writeJSON(w, map[string]interface{}{"items": items}, http.StatusOK)
}

// HandleConversationMessages handles GET /api/v1/conversations/{id}/messages: the transcript of one of the caller's
// conversations, oldest first. 404 for an unknown id, 403 for somebody else's conversation.
func (h *ConversationHandler) HandleConversationMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	if h.history == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_AVAILABLE", Message: "Conversation history is not available"}}, http.StatusServiceUnavailable)
		return
	}
	convID := extractIDFromPath(r.URL.Path, "/api/v1/conversations/", "/messages")
	conv, msgs, err := h.history.Transcript(r.Context(), userID, convID)
	switch {
	case errors.Is(err, domain.ErrConversationForbidden):
		writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "This conversation belongs to another user"}}, http.StatusForbidden)
		return
	case errors.Is(err, conversation.ErrConversationNotFound):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_FOUND", Message: "Conversation not found"}}, http.StatusNotFound)
		return
	case err != nil:
		h.logger.Error().Err(err).Msg("Failed to read a conversation")
		writeJSON(w, ErrorResponse{Error: Error{Code: "HISTORY_FAILED", Message: "The conversation could not be read"}}, http.StatusInternalServerError)
		return
	}
	items := make([]transcriptMessageJSON, 0, len(msgs))
	for _, m := range orderTranscript(msgs) {
		if m == nil || m.Content == nil || strings.TrimSpace(m.Content.Text) == "" {
			continue
		}
		item := transcriptMessageJSON{ID: m.ID, Direction: string(m.Direction), Text: m.Content.Text, CreatedAt: m.CreatedAt}
		if m.Direction == domain.MessageDirectionOutbound {
			item.Response = conversation.StoredReply(m.Content.Data)
		}
		items = append(items, item)
	}
	writeJSON(w, map[string]interface{}{"conversation_id": conv.ID, "external_id": conv.ExternalID, "items": items}, http.StatusOK)
}

// orderTranscript puts each answer right after the question it answers. Messages are stored to the second, so two
// exchanges in the same second cannot be ordered by time alone; the processor stores every answer under the key
// "reply:" + the question's key, which is the real link. An answer without its question (a confirmation's reply,
// "reply:confirm:…") keeps its place in time.
func orderTranscript(msgs []*domain.Message) []*domain.Message {
	ordered := make([]*domain.Message, 0, len(msgs))
	sorted := append([]*domain.Message(nil), msgs...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].CreatedAt < sorted[j].CreatedAt })
	replies := map[string]*domain.Message{}
	questions := map[string]bool{}
	for _, m := range sorted {
		if m == nil {
			continue
		}
		if m.Direction == domain.MessageDirectionInbound && m.IdempotencyKey != "" {
			questions[m.IdempotencyKey] = true
		}
		if m.Direction == domain.MessageDirectionOutbound && strings.HasPrefix(m.IdempotencyKey, "reply:") {
			replies[strings.TrimPrefix(m.IdempotencyKey, "reply:")] = m
		}
	}
	for _, m := range sorted {
		if m == nil {
			continue
		}
		if m.Direction == domain.MessageDirectionOutbound && strings.HasPrefix(m.IdempotencyKey, "reply:") && questions[strings.TrimPrefix(m.IdempotencyKey, "reply:")] {
			continue // placed after its question
		}
		ordered = append(ordered, m)
		if m.Direction == domain.MessageDirectionInbound {
			if reply := replies[m.IdempotencyKey]; reply != nil && m.IdempotencyKey != "" {
				ordered = append(ordered, reply)
			}
		}
	}
	return ordered
}
