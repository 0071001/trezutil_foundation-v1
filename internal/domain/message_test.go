package domain

import (
	"testing"
)

func TestNewMessage(t *testing.T) {
	content := &MessageContent{Text: "Hello"}
	message := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", content)

	if message.ConversationID != "conv-1" {
		t.Errorf("expected conv-1, got %s", message.ConversationID)
	}
	if message.Direction != MessageDirectionInbound {
		t.Errorf("expected inbound, got %s", message.Direction)
	}
	if message.Type != MessageTypeText {
		t.Errorf("expected text, got %s", message.Type)
	}
	if message.SenderID != "user-1" {
		t.Errorf("expected user-1, got %s", message.SenderID)
	}
	if message.Content == nil {
		t.Error("expected content to be set")
	}
	if message.CreatedAt == 0 {
		t.Error("expected created_at to be set")
	}
}

func TestMessageValidate(t *testing.T) {
	tests := []struct {
		name    string
		message *Message
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid inbound text",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1",
					&MessageContent{Text: "Hello"})
				m.IdempotencyKey = "msg-1"
				return m
			}(),
			wantErr: false,
		},
		{
			name: "valid outbound structured",
			message: NewMessage("conv-1", MessageDirectionOutbound, MessageTypeStructured, "bot-1",
				&MessageContent{Data: map[string]interface{}{"key": "value"}}),
			wantErr: false,
		},
		{
			name: "missing conversation ID",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", &MessageContent{Text: "Hi"})
				m.ConversationID = ""
				return m
			}(),
			wantErr: true,
			errMsg:  "conversation ID",
		},
		{
			name: "invalid direction",
			message: func() *Message {
				m := NewMessage("conv-1", "unknown", MessageTypeText, "user-1", &MessageContent{Text: "Hi"})
				return m
			}(),
			wantErr: true,
			errMsg:  "invalid message direction",
		},
		{
			name: "invalid type",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageType("unknown"), "user-1", &MessageContent{Text: "Hi"})
				return m
			}(),
			wantErr: true,
			errMsg:  "invalid message type",
		},
		{
			name: "missing sender ID",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "", &MessageContent{Text: "Hi"})
				return m
			}(),
			wantErr: true,
			errMsg:  "sender ID",
		},
		{
			name: "missing content",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", nil)
				return m
			}(),
			wantErr: true,
			errMsg:  "content",
		},
		{
			name: "inbound missing idempotency key",
			message: func() *Message {
				m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", &MessageContent{Text: "Hi"})
				m.IdempotencyKey = ""
				return m
			}(),
			wantErr: true,
			errMsg:  "idempotency key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				if tt.errMsg != "" && err != nil && err.Error() != tt.errMsg {
					// Check if the error message contains the expected substring
					if !contains(err.Error(), tt.errMsg) {
						t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestMessageIsInbound(t *testing.T) {
	m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", &MessageContent{Text: "Hi"})
	if !m.IsInbound() {
		t.Error("expected inbound")
	}
	if m.IsOutbound() {
		t.Error("expected not outbound")
	}
}

func TestMessageIsOutbound(t *testing.T) {
	m := NewMessage("conv-1", MessageDirectionOutbound, MessageTypeText, "bot-1", &MessageContent{Text: "Hi"})
	if !m.IsOutbound() {
		t.Error("expected outbound")
	}
	if m.IsInbound() {
		t.Error("expected not inbound")
	}
}

func TestMessageIdempotencyReference(t *testing.T) {
	m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeText, "user-1", &MessageContent{Text: "Hi"})
	m.IdempotencyKey = "msg-123"

	ref := m.IdempotencyReference(ChannelWhatsApp)
	if ref != "whatsapp:msg-123" {
		t.Errorf("expected whatsapp:msg-123, got %s", ref)
	}
}

func TestMessageTypeIsValid(t *testing.T) {
	tests := []struct {
		msgType  MessageType
		expected bool
	}{
		{MessageTypeText, true},
		{MessageTypeImage, true},
		{MessageTypeVideo, true},
		{MessageTypeAudio, true},
		{MessageTypeDocument, true},
		{MessageTypeLocation, true},
		{MessageTypeStructured, true},
		{MessageTypeSystem, true},
		{MessageType("unknown"), false},
		{"", false},
	}

	for _, tt := range tests {
		if tt.msgType.IsValid() != tt.expected {
			t.Errorf("IsValid() for %q: expected %v", tt.msgType, tt.expected)
		}
	}
}

func TestMessageContentWithLocation(t *testing.T) {
	content := &MessageContent{
		Text:     "Location",
		Location: &GeoPoint{Latitude: -4.44, Longitude: 15.27},
	}
	m := NewMessage("conv-1", MessageDirectionInbound, MessageTypeLocation, "user-1", content)

	if m.Content.Location == nil {
		t.Fatal("expected location to be set")
	}
	if m.Content.Location.Latitude != -4.44 {
		t.Errorf("expected latitude -4.44, got %f", m.Content.Location.Latitude)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
