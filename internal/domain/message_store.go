package domain

import (
	"context"
	"errors"
)

// MessageStore defines the interface for message persistence.
type MessageStore interface {
	Create(ctx context.Context, message *Message) error
	FindByID(ctx context.Context, id string) (*Message, error)
	FindByConversationID(ctx context.Context, conversationID string) ([]*Message, error)
	FindByExternalRef(ctx context.Context, channel Channel, externalRef string) (*Message, error)
}

// ErrMessageNotStored: a request could not be recorded, so nothing was done; it can be sent again.
var ErrMessageNotStored = errors.New("the message could not be stored")
