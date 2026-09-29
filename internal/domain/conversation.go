package domain

import (
	"fmt"
	"time"
)

// ConversationState represents the lifecycle state of a conversation.
type ConversationState string

const (
	ConversationStateActive   ConversationState = "active"
	ConversationStatePaused   ConversationState = "paused"
	ConversationStateClosed   ConversationState = "closed"
	ConversationStateArchived ConversationState = "archived"
)

// Conversation represents a channel-independent conversation with a user.
type Conversation struct {
	ID             string
	OrganizationID string
	CustomerID     string
	SellerID       string
	StoreID        string
	Channel        Channel
	ExternalID     string
	Language       string
	State          ConversationState
	Metadata       map[string]interface{}
	CreatedAt      int64
	UpdatedAt      int64
}

// NewConversation creates a new Conversation.
func NewConversation(channel Channel, externalID string, customerID string, organizationID string) *Conversation {
	now := time.Now().Unix()
	return &Conversation{
		ID:             NewID(),
		OrganizationID: organizationID,
		CustomerID:     customerID,
		Channel:        channel,
		ExternalID:     externalID,
		State:          ConversationStateActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		Metadata:       make(map[string]interface{}),
	}
}

// Pause transitions the conversation to paused state.
func (c *Conversation) Pause() error {
	if c.State != ConversationStateActive {
		return fmt.Errorf("cannot pause conversation in state %s", c.State)
	}
	c.State = ConversationStatePaused
	c.UpdatedAt = time.Now().Unix()
	return nil
}

// Resume transitions the conversation from paused to active.
func (c *Conversation) Resume() error {
	if c.State != ConversationStatePaused {
		return fmt.Errorf("cannot resume conversation in state %s", c.State)
	}
	c.State = ConversationStateActive
	c.UpdatedAt = time.Now().Unix()
	return nil
}

// Close transitions the conversation to closed state.
func (c *Conversation) Close() error {
	if c.State == ConversationStateClosed || c.State == ConversationStateArchived {
		return fmt.Errorf("cannot close conversation in terminal state %s", c.State)
	}
	c.State = ConversationStateClosed
	c.UpdatedAt = time.Now().Unix()
	return nil
}

// Archive transitions the conversation to archived state.
func (c *Conversation) Archive() error {
	if c.State == ConversationStateArchived {
		return fmt.Errorf("cannot archive conversation in terminal state %s", c.State)
	}
	c.State = ConversationStateArchived
	c.UpdatedAt = time.Now().Unix()
	return nil
}

// IsTerminal returns true if the conversation is in a terminal state.
func (c *Conversation) IsTerminal() bool {
	return c.State == ConversationStateClosed || c.State == ConversationStateArchived
}

// SetSellerContext sets seller/store context on the conversation.
func (c *Conversation) SetSellerContext(sellerID, storeID string) {
	c.SellerID = sellerID
	c.StoreID = storeID
}

// SetLanguage sets the conversation language.
func (c *Conversation) SetLanguage(language string) {
	c.Language = language
}

// AddMetadata adds metadata to the conversation.
func (c *Conversation) AddMetadata(key string, value interface{}) {
	c.Metadata[key] = value
}
