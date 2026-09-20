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
