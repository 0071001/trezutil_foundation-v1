package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestUsage_CreditsReservedBeforeAI(t *testing.T) {
	orch, provider, runtime, usageSvc := setupOrchestrator(t)

	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	_, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)

	assert.NotNil(t, usageSvc.lastReservationID)
	assert.Len(t, runtime.executedActions, 1)
}

func TestUsage_FailureReleasesCredits(t *testing.T) {
	orch, provider, runtime, usageSvc := setupOrchestrator(t)

	provider.SetError("hotels", domain.NewAIError(domain.AIErrProviderTimeout, "timeout"))

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Empty(t, result.ActionResult)
	assert.Len(t, runtime.executedActions, 0)
	assert.True(t, usageSvc.wasReleased)
}

func TestUsage_InsufficientCredits(t *testing.T) {
	orch, provider, runtime, usageSvc := setupOrchestrator(t)

	usageSvc.canAfford = false

	provider.SetResponse("hotels", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Len(t, runtime.executedActions, 0)
}

func TestConfirmation_ReadOnlyActionNoConfirmation(t *testing.T) {
	orch, provider, _, _ := setupOrchestrator(t)

	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.NeedsConfirmation)
	assert.NotNil(t, result.ActionResult)
	assert.Equal(t, "SUCCESS", string(result.ActionResult.Status))
}

func TestConfirmation_MutationRequiresConfirmation(t *testing.T) {
	orch, provider, _, _ := setupOrchestrator(t)

	provider.SetResponse("create order now", domain.AIResponse{
		Content:    `{"intent": "CREATE_ORDER", "confidence": 0.95, "parameters": {"listing_id": "abc"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "create order now"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Nil(t, result.Intent)
}

func TestUsage_DoubleConsumptionPrevention(t *testing.T) {
	orch, provider, _, usageSvc := setupOrchestrator(t)

	provider.SetResponse("find hotels", domain.AIResponse{
		Content:    `{"intent": "SEARCH_LISTINGS", "confidence": 0.95, "parameters": {"query": "hotels"}}`,
		TokensUsed: 10,
	})

	authenticatedActor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticatedActor.OrgID = "tenant-1"

	req := &AIInterpretRequest{
		ConversationID:     "conv-1",
		OrganizationID:     "tenant-1",
		TenantID:           "tenant-1",
		AuthenticatedActor: authenticatedActor,
		Messages: []domain.Message{
			{ConversationID: "conv-1", Direction: domain.MessageDirectionInbound, Type: domain.MessageTypeText, SenderID: "user-1", Content: &domain.MessageContent{Text: "find hotels"}},
		},
		Locale:  "en",
		Channel: domain.ChannelWeb,
	}

	result1, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result1)

	consumedCount := usageSvc.consumeCount
	assert.Equal(t, 1, consumedCount)

	result2, err := orch.Interpret(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.Equal(t, 1, consumedCount)
}
