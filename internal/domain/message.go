package domain

import (
	"fmt"
	"time"
)

// MessageDirection represents the direction of a message.
type MessageDirection string

const (
	MessageDirectionInbound  MessageDirection = "inbound"
	MessageDirectionOutbound MessageDirection = "outbound"
)

// MessageType represents the type of a message.
type MessageType string

const (
	MessageTypeText       MessageType = "text"
	MessageTypeImage      MessageType = "image"
	MessageTypeVideo      MessageType = "video"
	MessageTypeAudio      MessageType = "audio"
	MessageTypeDocument   MessageType = "document"
	MessageTypeLocation   MessageType = "location"
	MessageTypeStructured MessageType = "structured"
	MessageTypeSystem     MessageType = "system"
)

// IsValid returns true if the message type is recognized.
func (m MessageType) IsValid() bool {
	switch m {
	case MessageTypeText, MessageTypeImage, MessageTypeVideo, MessageTypeAudio,
		MessageTypeDocument, MessageTypeLocation, MessageTypeStructured, MessageTypeSystem:
		return true
	default:
		return false
	}
}

// Message represents an inbound/outbound message in a conversation.
type Message struct {
	ID             string
	ConversationID string
	Direction      MessageDirection
	Type           MessageType
	SenderID       string
	RecipientID    string
	Content        *MessageContent
	ExternalRef    string
	IdempotencyKey string
	Language       string
	CreatedAt      int64
}

// MessageContent represents the payload of a message.
type MessageContent struct {
	Text      string
	Data      map[string]interface{}
	MediaURLs []string
	Location  *GeoPoint
}

// NewMessage creates a new Message.
func NewMessage(conversationID string, direction MessageDirection, msgType MessageType, senderID string, content *MessageContent) *Message {
	return &Message{
		ID:             "",
		ConversationID: conversationID,
		Direction:      direction,
		Type:           msgType,
		SenderID:       senderID,
		Content:        content,
		CreatedAt:      time.Now().Unix(),
	}
}

// Validate returns an error if the message is invalid.
func (m *Message) Validate() error {
	if m.ConversationID == "" {
		return fmt.Errorf("message must have a conversation ID")
	}
	if m.Direction != MessageDirectionInbound && m.Direction != MessageDirectionOutbound {
		return fmt.Errorf("invalid message direction: %s", m.Direction)
	}
	if !m.Type.IsValid() {
		return fmt.Errorf("invalid message type: %s", m.Type)
	}
	if m.SenderID == "" {
		return fmt.Errorf("message must have a sender ID")
	}
	if m.Content == nil {
		return fmt.Errorf("message must have content")
	}
	if m.IdempotencyKey == "" && m.Direction == MessageDirectionInbound {
		return fmt.Errorf("inbound messages require an idempotency key")
	}
	return nil
}

// IsInbound returns true if the message is inbound.
func (m *Message) IsInbound() bool {
	return m.Direction == MessageDirectionInbound
}

// IsOutbound returns true if the message is outbound.
func (m *Message) IsOutbound() bool {
	return m.Direction == MessageDirectionOutbound
}

// IdempotencyReference returns the channel + external ID combination for idempotency.
func (m *Message) IdempotencyReference(channel Channel) string {
	return string(channel) + ":" + m.IdempotencyKey
}
