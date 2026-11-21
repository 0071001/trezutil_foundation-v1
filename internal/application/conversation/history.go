package conversation

import (
	"context"
	"errors"
	"regexp"

	"github.com/trezutil/trezutil/internal/domain"
)

// ErrConversationNotFound: no conversation has that id (or the id is not one).
var ErrConversationNotFound = errors.New("conversation not found")

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// HistoryService reads back a person's own conversations: the list, and the transcript of one. It never changes
// anything and never runs an action. Ownership is the one the processor enforces: a conversation belongs to the
// customer recorded on it; a guest conversation (no customer yet) is readable by nobody through this service.
type HistoryService struct {
	lister        domain.ConversationHistoryReader
	conversations domain.ConversationStore
	messages      domain.MessageStore
}

// NewHistoryService builds the service from the stores the conversation service already uses.
func NewHistoryService(lister domain.ConversationHistoryReader, conversations domain.ConversationStore, messages domain.MessageStore) *HistoryService {
	return &HistoryService{lister: lister, conversations: conversations, messages: messages}
}

// List returns the caller's conversations on a channel, most recent activity first.
func (s *HistoryService) List(ctx context.Context, userID string, channel domain.Channel, limit int) ([]domain.ConversationHistoryItem, error) {
	if userID == "" {
		return nil, domain.ErrConversationForbidden
	}
	return s.lister.ListByCustomer(ctx, userID, channel, limit)
}

// Transcript returns the messages of one of the caller's conversations, oldest first.
func (s *HistoryService) Transcript(ctx context.Context, userID, conversationID string) (*domain.Conversation, []*domain.Message, error) {
	if userID == "" {
		return nil, nil, domain.ErrConversationForbidden
	}
	if !uuidPattern.MatchString(conversationID) {
		return nil, nil, ErrConversationNotFound
	}
	conv, err := s.conversations.FindByID(ctx, conversationID)
	if err != nil {
		return nil, nil, err
	}
	if conv == nil {
		return nil, nil, ErrConversationNotFound
	}
	if conv.CustomerID == "" || conv.CustomerID != userID {
		return nil, nil, domain.ErrConversationForbidden
	}
	msgs, err := s.messages.FindByConversationID(ctx, conv.ID)
	if err != nil {
		return nil, nil, err
	}
	return conv, msgs, nil
}
