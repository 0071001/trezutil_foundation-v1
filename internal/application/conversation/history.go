package conversation

import (
	"context"
	"errors"
	"regexp"
	"strings"

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
	writer        domain.ConversationHistoryWriter
}

// NewHistoryService builds the service from the stores the conversation service already uses.
func NewHistoryService(lister domain.ConversationHistoryReader, conversations domain.ConversationStore, messages domain.MessageStore) *HistoryService {
	return &HistoryService{lister: lister, conversations: conversations, messages: messages}
}

// WithWriter lets the owner rename, archive and delete their conversations.
func (s *HistoryService) WithWriter(writer domain.ConversationHistoryWriter) *HistoryService {
	s.writer = writer
	return s
}

// ErrHistoryReadOnly: the service was built without a writer.
var ErrHistoryReadOnly = errors.New("conversation history is read-only")

// MaxTitleLength bounds a conversation's name (the database enforces the same limit).
const MaxTitleLength = 120

// CleanTitle collapses whitespace and cuts a name to MaxTitleLength characters.
func CleanTitle(title string) string {
	clean := strings.Join(strings.Fields(title), " ")
	if r := []rune(clean); len(r) > MaxTitleLength {
		clean = strings.TrimSpace(string(r[:MaxTitleLength]))
	}
	return clean
}

// Rename names one of the caller's conversations; an empty title gives it back its first message as name.
func (s *HistoryService) Rename(ctx context.Context, userID, conversationID, title string) error {
	return s.change(userID, conversationID, func() error {
		return s.writer.Rename(ctx, userID, conversationID, CleanTitle(title))
	})
}

// SetArchived archives or restores one of the caller's conversations.
func (s *HistoryService) SetArchived(ctx context.Context, userID, conversationID string, archived bool) error {
	return s.change(userID, conversationID, func() error {
		return s.writer.SetArchived(ctx, userID, conversationID, archived)
	})
}

// Delete removes one of the caller's conversations and everything stored with it.
func (s *HistoryService) Delete(ctx context.Context, userID, conversationID string) error {
	return s.change(userID, conversationID, func() error {
		return s.writer.Delete(ctx, userID, conversationID)
	})
}

// change runs an owner-scoped change. Someone else's conversation and an unknown one answer the same
// ErrConversationNotFound, so the answer never tells whether a conversation exists.
func (s *HistoryService) change(userID, conversationID string, run func() error) error {
	if userID == "" {
		return domain.ErrConversationForbidden
	}
	if s.writer == nil {
		return ErrHistoryReadOnly
	}
	if !uuidPattern.MatchString(conversationID) {
		return ErrConversationNotFound
	}
	if err := run(); err != nil {
		if errors.Is(err, domain.ErrConversationNotOwned) {
			return ErrConversationNotFound
		}
		return err
	}
	return nil
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
