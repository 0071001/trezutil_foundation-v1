package whatsapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/conversation"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
)

// InboundProcessor is the channel-neutral conversation engine.
type InboundProcessor interface {
	ProcessMessage(ctx context.Context, req *conversation.ProcessRequest) (*conversation.ProcessResult, error)
}

// OutboundSender delivers replies to a WhatsApp user.
type OutboundSender interface {
	SendOutbound(ctx context.Context, to string, messages []whatsappinfra.OutboundMessage) error
}

// Bridge connects the WhatsApp webhook to the ConversationProcessor, so
// WhatsApp is only a channel: intent resolution, confirmations, context,
// idempotency and commerce execution are shared with the web.
//
//	Webhook -> HMAC -> channel account (tenant) -> user (auto-provisioned from
//	the Meta-verified sender phone) -> ConversationProcessor -> reply sender
type Bridge struct {
	adapter         *whatsappinfra.Adapter
	channelAccounts domain.ChannelAccountRepository
	users           domain.UserRepository
	events          domain.WebhookEventStore
	processor       InboundProcessor
	sender          OutboundSender
	logger          zerolog.Logger
}

// BridgeConfig configures a Bridge.
type BridgeConfig struct {
	Adapter         *whatsappinfra.Adapter
	ChannelAccounts domain.ChannelAccountRepository
	Users           domain.UserRepository
	Events          domain.WebhookEventStore
	Processor       InboundProcessor
	// Sender is optional; without it replies are only returned in the HTTP
	// response (useful for tests, not delivered to users).
	Sender OutboundSender
	Logger zerolog.Logger
}

// NewBridge creates a Bridge.
func NewBridge(cfg BridgeConfig) *Bridge {
	return &Bridge{
		adapter:         cfg.Adapter,
		channelAccounts: cfg.ChannelAccounts,
		users:           cfg.Users,
		events:          cfg.Events,
		processor:       cfg.Processor,
		sender:          cfg.Sender,
		logger:          cfg.Logger,
	}
}

// VerifyWebhook checks the X-Hub-Signature-256 HMAC.
func (b *Bridge) VerifyWebhook(body []byte, signature string) error {
	if b.adapter == nil {
		return fmt.Errorf("webhook adapter not configured")
	}
	return b.adapter.VerifyWebhookSignature(body, signature)
}

// ProcessWebhook handles the messages of one webhook change.
func (b *Bridge) ProcessWebhook(
	ctx context.Context,
	channel domain.Channel,
	senderPhone string,
	conversationExternalID string,
	wabaID string,
	messages []domain.Message,
) (*domain.ConversationResponse, error) {
	if wabaID == "" {
		return nil, fmt.Errorf("WhatsApp Business Account ID required for tenant resolution")
	}
	account, err := b.channelAccounts.FindByChannelExternalID(ctx, channel, wabaID)
	if err != nil || account == nil {
		return nil, fmt.Errorf("channel account not configured for WABA ID %s", wabaID)
	}

	// wa_id is the sender's number without "+"; Meta has verified it.
	phone := senderPhone
	if phone != "" && !strings.HasPrefix(phone, "+") {
		phone = "+" + phone
	}
	user, err := identity.FindOrCreateVerifiedUser(ctx, b.users, phone, time.Now().Unix())
	if err != nil {
		return nil, fmt.Errorf("resolve sender: %w", err)
	}

	var last *domain.ConversationResponse
	for _, msg := range messages {
		if msg.IdempotencyKey != "" && b.events != nil {
			claimed, err := b.events.TryClaim(ctx, channel, msg.IdempotencyKey)
			if err != nil {
				return nil, err
			}
			if !claimed {
				continue // redelivery of an already processed message
			}
		}

		var response *domain.ConversationResponse
		if msg.Type != domain.MessageTypeText || msg.Content == nil || strings.TrimSpace(msg.Content.Text) == "" {
			response = domain.NewConversationResponse("For now I can read text messages only. Tell me what you're looking for!", "en")
		} else {
			result, err := b.processor.ProcessMessage(ctx, &conversation.ProcessRequest{
				Channel:               channel,
				SenderID:              user.ID,
				SenderExternalID:      wabaID + ":" + user.Phone,
				MessageText:           msg.Content.Text,
				MessageExternalRef:    msg.ExternalRef,
				MessageIdempotencyKey: msg.IdempotencyKey,
				Locale:                msg.Language,
				TenantID:              account.OrganizationID,
				Actor:                 domain.NewCommerceActor(user.ID, domain.ActorTypeCustomer),
			})
			if err != nil {
				if msg.IdempotencyKey != "" && b.events != nil {
					_ = b.events.MarkFailed(ctx, channel, msg.IdempotencyKey, err.Error())
				}
				return nil, err
			}
			response = result.Response
		}

		if msg.IdempotencyKey != "" && b.events != nil {
			_ = b.events.MarkProcessed(ctx, channel, msg.IdempotencyKey)
		}
		if b.sender != nil && response != nil {
			outbound, _ := b.adapter.BuildOutbound(response)
			if err := b.sender.SendOutbound(ctx, user.Phone, outbound); err != nil {
				// The message is processed; a delivery failure must not cause
				// the provider to redeliver and re-execute it.
				b.logger.Warn().Err(err).Str("user_id", user.ID).Msg("WhatsApp reply delivery failed")
			}
		}
		last = response
	}
	return last, nil
}
