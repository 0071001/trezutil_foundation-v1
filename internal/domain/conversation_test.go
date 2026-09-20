package domain

import (
	"testing"
	"time"
)

func TestChannelValidation(t *testing.T) {
	tests := []struct {
		channel  Channel
		expected bool
	}{
		{ChannelWhatsApp, true},
		{ChannelWeb, true},
		{ChannelMobile, true},
		{ChannelUSSD, true},
		{ChannelSMS, true},
		{ChannelVoice, true},
		{ChannelOther, true},
		{Channel("unknown"), false},
		{"", false},
	}

	for _, tt := range tests {
		if tt.channel.IsValid() != tt.expected {
			t.Errorf("IsValid() for %q: expected %v", tt.channel, tt.expected)
		}
	}
}

func TestChannelIsMetaChannel(t *testing.T) {
	if !ChannelWhatsApp.IsMetaChannel() {
		t.Error("expected WhatsApp to be a meta channel")
	}
	if ChannelWeb.IsMetaChannel() {
		t.Error("expected Web to not be a meta channel")
	}
}

func TestNewConversation(t *testing.T) {
	now := time.Now().Unix()
	channel := ChannelWhatsApp
	conversation := NewConversation(channel, "ext-1", "customer-1", "org-1")

	if conversation.Channel != channel {
		t.Errorf("expected %s, got %s", channel, conversation.Channel)
	}
	if conversation.ExternalID != "ext-1" {
		t.Errorf("expected ext-1, got %s", conversation.ExternalID)
	}
	if conversation.CustomerID != "customer-1" {
		t.Errorf("expected customer-1, got %s", conversation.CustomerID)
	}
	if conversation.OrganizationID != "org-1" {
		t.Errorf("expected org-1, got %s", conversation.OrganizationID)
	}
	if conversation.State != ConversationStateActive {
		t.Errorf("expected active, got %s", conversation.State)
	}
	if conversation.CreatedAt != now {
		t.Errorf("expected created_at %d, got %d", now, conversation.CreatedAt)
	}
	if conversation.UpdatedAt != now {
		t.Errorf("expected updated_at %d, got %d", now, conversation.UpdatedAt)
	}
	if conversation.Metadata == nil {
		t.Error("expected metadata map to be initialized")
	}
}

func TestConversationPause(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")

	if err := conversation.Pause(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conversation.State != ConversationStatePaused {
		t.Errorf("expected paused, got %s", conversation.State)
	}
}

func TestConversationPauseFromPaused(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.Pause()

	if err := conversation.Pause(); err == nil {
		t.Error("expected error when pausing paused conversation")
	}
}

func TestConversationPauseFromClosed(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.State = ConversationStateClosed

	if err := conversation.Pause(); err == nil {
		t.Error("expected error when pausing closed conversation")
	}
}

func TestConversationResume(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.Pause()

	if err := conversation.Resume(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conversation.State != ConversationStateActive {
		t.Errorf("expected active, got %s", conversation.State)
	}
}

func TestConversationResumeFromActive(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")

	if err := conversation.Resume(); err == nil {
		t.Error("expected error when resuming active conversation")
	}
}

func TestConversationClose(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")

	if err := conversation.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conversation.State != ConversationStateClosed {
		t.Errorf("expected closed, got %s", conversation.State)
	}
}

func TestConversationCloseFromClosed(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.Close()

	if err := conversation.Close(); err == nil {
		t.Error("expected error when closing closed conversation")
	}
}

func TestConversationArchive(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")

	if err := conversation.Archive(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if conversation.State != ConversationStateArchived {
		t.Errorf("expected archived, got %s", conversation.State)
	}
}

func TestConversationIsTerminal(t *testing.T) {
	tests := []struct {
		state    ConversationState
		expected bool
	}{
		{ConversationStateActive, false},
		{ConversationStatePaused, false},
		{ConversationStateClosed, true},
		{ConversationStateArchived, true},
	}

	for _, tt := range tests {
		conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
		conversation.State = tt.state

		if conversation.IsTerminal() != tt.expected {
			t.Errorf("IsTerminal() for %s: expected %v", tt.state, tt.expected)
		}
	}
}

func TestConversationSetSellerContext(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.SetSellerContext("seller-1", "store-1")

	if conversation.SellerID != "seller-1" {
		t.Errorf("expected seller-1, got %s", conversation.SellerID)
	}
	if conversation.StoreID != "store-1" {
		t.Errorf("expected store-1, got %s", conversation.StoreID)
	}
}

func TestConversationSetLanguage(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.SetLanguage("fr")

	if conversation.Language != "fr" {
		t.Errorf("expected fr, got %s", conversation.Language)
	}
}

func TestConversationAddMetadata(t *testing.T) {
	conversation := NewConversation(ChannelWhatsApp, "ext-1", "customer-1", "org-1")
	conversation.AddMetadata("key", "value")

	if conversation.Metadata["key"] != "value" {
		t.Errorf("expected value, got %v", conversation.Metadata["key"])
	}
}
