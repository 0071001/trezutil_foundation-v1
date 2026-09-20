package conversation

import (
	"context"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// Service handles conversation-related operations.
type Service struct {
	store domain.ConversationStore
}

// NewService creates a new conversation service.
func NewService(store domain.ConversationStore) *Service {
	return &Service{
		store: store,
	}
}

// CreateConversation creates a new conversation.
func (s *Service) CreateConversation(ctx context.Context, channel domain.Channel, externalID string, customerID string, organizationID string) (*domain.Conversation, error) {
	if !channel.IsValid() {
		return nil, fmt.Errorf("invalid channel: %s", channel)
	}
	if externalID == "" {
		return nil, fmt.Errorf("external ID is required")
	}
	if customerID == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	conversation := domain.NewConversation(channel, externalID, customerID, organizationID)

	if err := s.store.Create(ctx, conversation); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	return conversation, nil
}

// GetConversation retrieves a conversation by ID.
func (s *Service) GetConversation(ctx context.Context, id string) (*domain.Conversation, error) {
	if id == "" {
		return nil, fmt.Errorf("conversation ID is required")
	}
	return s.store.FindByID(ctx, id)
}

// AddMessage adds a message to a conversation and updates the last activity timestamp.
func (s *Service) AddMessage(ctx context.Context, conversationID string, message *domain.Message) error {
	if err := message.Validate(); err != nil {
		return fmt.Errorf("invalid message: %w", err)
	}

	conversation, err := s.store.FindByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	message.ConversationID = conversationID
	message.CreatedAt = time.Now().Unix()

	conversation.UpdatedAt = time.Now().Unix()
	if err := s.store.Update(ctx, conversation); err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

// UpdateContext updates the conversation context (e.g., current intent, entities).
func (s *Service) UpdateContext(ctx context.Context, conversationID string, context domain.ConversationContext) error {
	conversation, err := s.store.FindByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	if context.Language != "" {
		conversation.SetLanguage(context.Language)
	}
	conversation.UpdatedAt = time.Now().Unix()

	if err := s.store.Update(ctx, conversation); err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	return nil
}

// CloseConversation closes a conversation.
func (s *Service) CloseConversation(ctx context.Context, conversationID string) error {
	conversation, err := s.store.FindByID(ctx, conversationID)
	if err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}
	if err := conversation.Close(); err != nil {
		return err
	}
	return s.store.Update(ctx, conversation)
}
