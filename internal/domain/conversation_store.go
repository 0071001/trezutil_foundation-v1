package domain

import (
	"context"
	"errors"
	"time"
)

// ConversationStore defines the interface for conversation persistence.
type ConversationStore interface {
	Create(ctx context.Context, conversation *Conversation) error
	FindByID(ctx context.Context, id string) (*Conversation, error)
	FindByChannelExternalID(ctx context.Context, channel Channel, externalID string) (*Conversation, error)
	Update(ctx context.Context, conversation *Conversation) error
}

// SystemClock returns the current Unix timestamp.
func SystemClock() int64 {
	return time.Now().Unix()
}

// ConversationHistoryItem is one conversation as its owner sees it in a list: what it is about (the first message they
// wrote) and when it last moved. It carries no commerce data.
type ConversationHistoryItem struct {
	ID             string
	Channel        Channel
	ExternalID     string
	Title          string
	MessageCount   int
	CreatedAt      int64
	LastActivityAt int64
	// Renamed: Title is the name the owner gave it, not their first message.
	Renamed  bool
	Archived bool
}

// ConversationHistoryReader lists a customer's conversations. It is a read-only companion of ConversationStore,
// kept separate so the stores and fakes that only process messages do not have to implement it.
type ConversationHistoryReader interface {
	ListByCustomer(ctx context.Context, customerID string, channel Channel, limit int) ([]ConversationHistoryItem, error)
}

// ConversationHistoryWriter applies an owner's own changes to their conversation. Every method is scoped by the
// customer: it changes nothing and returns ErrConversationNotOwned when the conversation is not theirs or is gone.
type ConversationHistoryWriter interface {
	// Rename sets the conversation's name; an empty title gives it back its first message as name.
	Rename(ctx context.Context, customerID, conversationID, title string) error
	SetArchived(ctx context.Context, customerID, conversationID string, archived bool) error
	// Delete removes the conversation and everything stored with it (messages, context, confirmations).
	Delete(ctx context.Context, customerID, conversationID string) error
}

// ErrConversationNotOwned: no conversation with that id belongs to that customer (it may not exist at all).
var ErrConversationNotOwned = errors.New("conversation not found for this customer")
