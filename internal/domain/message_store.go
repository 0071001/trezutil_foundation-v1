package domain

import "context"

// MessageStore defines the interface for message persistence.
type MessageStore interface {
	Create(ctx context.Context, message *Message) error
	FindByID(ctx context.Context, id string) (*Message, error)
	FindByConversationID(ctx context.Context, conversationID string) ([]*Message, error)
	FindByExternalRef(ctx context.Context, channel Channel, externalRef string) (*Message, error)
}
