package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestConversationStore_CreateAndFindByChannelExternalID(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)

	store := postgres.NewConversationStore(db.DB)

	conv := domain.NewConversation(domain.ChannelWhatsApp, "conv-external-1", userID, orgID)
	conv.ID = "123e4567-e89b-12d3-a456-426614174050"
	conv.Language = "en"
	conv.State = domain.ConversationStateActive
	conv.CreatedAt = time.Now().Unix()
	conv.UpdatedAt = time.Now().Unix()

	err := store.Create(ctx, conv)
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	found, err := store.FindByChannelExternalID(ctx, domain.ChannelWhatsApp, "conv-external-1")
	if err != nil {
		t.Fatalf("failed to find conversation: %v", err)
	}
	if found == nil {
		t.Fatal("expected conversation, got nil")
	}
	if found.ID != conv.ID {
		t.Errorf("expected ID %s, got %s", conv.ID, found.ID)
	}
	if found.OrganizationID != orgID {
		t.Errorf("expected org %s, got %s", orgID, found.OrganizationID)
	}
	if found.Channel != domain.ChannelWhatsApp {
		t.Errorf("expected channel whatsapp, got %s", found.Channel)
	}
	if found.ExternalID != "conv-external-1" {
		t.Errorf("expected external ID 'conv-external-1', got '%s'", found.ExternalID)
	}
}

func TestConversationStore_FindByChannelExternalID_NotFound(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	store := postgres.NewConversationStore(db.DB)
	ctx := context.Background()

	found, err := store.FindByChannelExternalID(ctx, domain.ChannelWhatsApp, "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Error("expected nil conversation, got one")
	}
}

func TestConversationStore_FindByID(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)

	store := postgres.NewConversationStore(db.DB)

	conv := domain.NewConversation(domain.ChannelWhatsApp, "conv-external-2", userID, orgID)
	conv.ID = "123e4567-e89b-12d3-a456-426614174051"
	conv.CreatedAt = time.Now().Unix()
	conv.UpdatedAt = time.Now().Unix()

	if err := store.Create(ctx, conv); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	found, err := store.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("failed to find conversation: %v", err)
	}
	if found == nil {
		t.Fatal("expected conversation, got nil")
	}
	if found.ExternalID != "conv-external-2" {
		t.Errorf("expected ExternalID 'conv-external-2', got '%s'", found.ExternalID)
	}
}

func TestConversationStore_FindByID_NotFound(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	store := postgres.NewConversationStore(db.DB)
	ctx := context.Background()

	found, err := store.FindByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Error("expected nil conversation, got one")
	}
}

func TestConversationStore_Update(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)

	store := postgres.NewConversationStore(db.DB)

	conv := domain.NewConversation(domain.ChannelWhatsApp, "conv-external-3", userID, orgID)
	conv.ID = "123e4567-e89b-12d3-a456-426614174052"
	conv.Language = "en"
	conv.State = domain.ConversationStateActive
	conv.CreatedAt = time.Now().Unix()
	conv.UpdatedAt = time.Now().Unix()

	if err := store.Create(ctx, conv); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	conv.Language = "fr"
	conv.State = domain.ConversationStatePaused
	conv.UpdatedAt = time.Now().Unix()

	if err := store.Update(ctx, conv); err != nil {
		t.Fatalf("failed to update conversation: %v", err)
	}

	found, err := store.FindByID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("failed to find conversation: %v", err)
	}
	if found == nil {
		t.Fatal("expected conversation, got nil")
	}
	if found.Language != "fr" {
		t.Errorf("expected Language 'fr', got '%s'", found.Language)
	}
	if found.State != domain.ConversationStatePaused {
		t.Errorf("expected State 'paused', got '%s'", found.State)
	}
}

func TestConversationStore_CreateDuplicateExternalID(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	orgID := "123e4567-e89b-12d3-a456-426614174020"
	userID := "123e4567-e89b-12d3-a456-426614174023"

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, userID)

	store := postgres.NewConversationStore(db.DB)

	conv1 := domain.NewConversation(domain.ChannelWhatsApp, "conv-external-4", userID, orgID)
	conv1.ID = "123e4567-e89b-12d3-a456-426614174053"
	conv1.CreatedAt = time.Now().Unix()
	conv1.UpdatedAt = time.Now().Unix()

	if err := store.Create(ctx, conv1); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	conv2 := domain.NewConversation(domain.ChannelWhatsApp, "conv-external-4", userID, orgID)
	conv2.ID = "123e4567-e89b-12d3-a456-426614174054"
	conv2.CreatedAt = time.Now().Unix()
	conv2.UpdatedAt = time.Now().Unix()

	err := store.Create(ctx, conv2)
	if err == nil {
		t.Fatal("expected error for duplicate external ID, got nil")
	}
}
