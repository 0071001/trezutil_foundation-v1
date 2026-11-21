package domain

import (
	"context"
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
}

// ConversationHistoryReader lists a customer's conversations. It is a read-only companion of ConversationStore,
// kept separate so the stores and fakes that only process messages do not have to implement it.
type ConversationHistoryReader interface {
	ListByCustomer(ctx context.Context, customerID string, channel Channel, limit int) ([]ConversationHistoryItem, error)
}
