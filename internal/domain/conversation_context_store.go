package domain

import "context"

// ConversationContextStore defines the interface for conversation context persistence.
type ConversationContextStore interface {
	Save(ctx context.Context, conversationID string, context ConversationContext) error
	Load(ctx context.Context, conversationID string) (*ConversationContext, error)
	Delete(ctx context.Context, conversationID string) error
}
