package whatsapp

import (
	"context"
	"fmt"
	"strings"

	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
)

type Service struct {
	store              domain.ConversationStore
	userRepo           domain.UserRepository
	channelAccountRepo domain.ChannelAccountRepository
	messageStore       domain.MessageStore
	webhookEventStore  domain.WebhookEventStore
	runtime            *commerce.CommerceActionRuntime
	adapter            *whatsappinfra.Adapter
	presenter          *whatsappinfra.Presenter
}

func NewService(
	store domain.ConversationStore,
	userRepo domain.UserRepository,
	channelAccountRepo domain.ChannelAccountRepository,
	messageStore domain.MessageStore,
	webhookEventStore domain.WebhookEventStore,
	listingUseCases commerce.ListingUseCases,
	searchUseCases commerce.SearchUseCases,
	reservationUseCases commerce.ReservationUseCases,
	availabilityUseCases commerce.AvailabilityUseCases,
	runtime *commerce.CommerceActionRuntime,
	adapter *whatsappinfra.Adapter,
	presenter *whatsappinfra.Presenter,
) *Service {
	if adapter == nil {
		adapter = whatsappinfra.NewAdapter(whatsappinfra.Config{})
	}
	if presenter == nil {
		presenter = whatsappinfra.NewPresenter()
	}

	if runtime != nil {
		if listingUseCases != nil {
			runtime.RegisterHandler(commerce.NewCreateListingHandler(listingUseCases))
			runtime.RegisterHandler(commerce.NewPublishListingHandler(listingUseCases))
			runtime.RegisterHandler(commerce.NewPauseListingHandler(listingUseCases))
			runtime.RegisterHandler(commerce.NewResumeListingHandler(listingUseCases))
		}
		if searchUseCases != nil {
			runtime.RegisterHandler(commerce.NewSuggestHandler(searchUseCases))
		}
		if reservationUseCases != nil {
			runtime.RegisterHandler(commerce.NewCreateReservationHandler(reservationUseCases))
			runtime.RegisterHandler(commerce.NewGetReservationHandler(reservationUseCases))
			runtime.RegisterHandler(commerce.NewCancelReservationHandler(reservationUseCases))
		}
		if availabilityUseCases != nil {
			runtime.RegisterHandler(commerce.NewCheckAvailabilityHandler(availabilityUseCases))
		}
	}

	return &Service{
		store:              store,
		userRepo:           userRepo,
		channelAccountRepo: channelAccountRepo,
		messageStore:       messageStore,
		webhookEventStore:  webhookEventStore,
		runtime:            runtime,
		adapter:            adapter,
		presenter:          presenter,
	}
}

func (s *Service) ResolveTenant(ctx context.Context, channel domain.Channel, wabaID string) (string, error) {
	if wabaID == "" {
		return "", fmt.Errorf("WhatsApp Business Account ID required for tenant resolution")
	}
	account, err := s.channelAccountRepo.FindByChannelExternalID(ctx, channel, wabaID)
	if err != nil {
		return "", err
	}
	if account == nil {
		return "", fmt.Errorf("channel account not configured for WABA ID: %s", wabaID)
	}
	return account.OrganizationID, nil
}

func (s *Service) VerifyWebhook(body []byte, signature string) error {
	if s.adapter == nil {
		return fmt.Errorf("webhook adapter not configured")
	}
	return s.adapter.VerifyWebhookSignature(body, signature)
}

func (s *Service) ProcessWebhook(
	ctx context.Context,
	channel domain.Channel,
	senderPhone string,
	conversationExternalID string,
	wabaID string,
	messages []domain.Message,
) (*domain.ConversationResponse, error) {
	tenantID, err := s.ResolveTenant(ctx, channel, wabaID)
	if err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByPhone(ctx, senderPhone)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("unknown sender")
	}

	if s.webhookEventStore != nil {
		for _, msg := range messages {
			if msg.IdempotencyKey == "" {
				continue
			}
			claimed, err := s.webhookEventStore.TryClaim(ctx, channel, msg.IdempotencyKey)
			if err != nil {
				return nil, err
			}
			if !claimed {
				return s.duplicateResponse(), nil
			}
		}
	}

	conv, err := s.store.FindByChannelExternalID(ctx, channel, conversationExternalID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		conv = domain.NewConversation(channel, conversationExternalID, user.ID, tenantID)
		if err := s.store.Create(ctx, conv); err != nil {
			return nil, err
		}
	} else if conv.OrganizationID != tenantID {
		return nil, fmt.Errorf("conversation tenant mismatch: expected %s, got %s", tenantID, conv.OrganizationID)
	}

	if s.messageStore != nil {
		for _, msg := range messages {
			msg.ConversationID = conv.ID
			existing, err := s.messageStore.FindByExternalRef(ctx, channel, msg.ExternalRef)
			if err != nil {
				return nil, err
			}
			if existing == nil {
				if err := s.messageStore.Create(ctx, &msg); err != nil {
					return nil, err
				}
			}
		}
	}

	response := domain.NewConversationResponse("", conv.Language)
	response.Language = conv.Language
	if response.Language == "" {
		response.Language = "en"
	}

	for _, msg := range messages {
		if msg.Type != domain.MessageTypeText {
			continue
		}

		actionType := s.resolveActionType(msg.Content.Text)
		if actionType == "" {
			response.Text = msg.Content.Text
			continue
		}

		actor := domain.NewCommerceActor(user.ID, domain.ActorTypeCustomer)
		actor.OrgID = tenantID
		action := domain.NewCommerceAction(actionType, actor, conv.ID)
		action.WithTenant(tenantID)
		action.WithSource("USER")
		action.WithParameters(msg.Content.Data)

		result, err := s.runtime.Execute(ctx, action, actor)
		if err != nil {
			if s.webhookEventStore != nil {
				_ = s.webhookEventStore.MarkFailed(ctx, channel, msg.IdempotencyKey, err.Error())
			}
			return nil, err
		}

		response = s.convertExecuteResult(result)
		break
	}

	if s.webhookEventStore != nil {
		for _, msg := range messages {
			if msg.IdempotencyKey != "" {
				_ = s.webhookEventStore.MarkProcessed(ctx, channel, msg.IdempotencyKey)
			}
		}
	}

	return response, nil
}

func (s *Service) duplicateResponse() *domain.ConversationResponse {
	resp := domain.NewConversationResponse("", "en")
	resp.Text = "Already processed"
	return resp
}

func (s *Service) resolveActionType(text string) domain.CommerceActionType {
	lower := strings.ToLower(text)

	if strings.Contains(lower, "search") || strings.Contains(lower, "find") ||
		strings.Contains(lower, "look") || strings.Contains(lower, "browse") ||
		strings.Contains(lower, "catalog") || strings.Contains(lower, "list") ||
		strings.Contains(lower, "suggest") || strings.Contains(lower, "recommend") ||
		strings.Contains(lower, "option") || strings.Contains(lower, "alternative") {
		return domain.ActionSearchListings
	}

	if strings.Contains(lower, "available") || strings.Contains(lower, "check") ||
		strings.Contains(lower, "disponible") || strings.Contains(lower, "availability") {
		return domain.ActionCheckAvailability
	}

	if strings.Contains(lower, "reserve") || strings.Contains(lower, "book") ||
		strings.Contains(lower, "reserver") || strings.Contains(lower, "booker") {
		return domain.ActionCreateReservation
	}

	if strings.Contains(lower, "my reservation") || strings.Contains(lower, "ma reservation") ||
		strings.Contains(lower, "view booking") || strings.Contains(lower, "voir reservation") {
		return domain.ActionGetReservation
	}

	if strings.Contains(lower, "cancel") || strings.Contains(lower, "annuler") {
		return domain.ActionCancelReservation
	}

	if strings.Contains(lower, "buy") || strings.Contains(lower, "order") ||
		strings.Contains(lower, "purchase") || strings.Contains(lower, "cart") ||
		strings.Contains(lower, "checkout") || strings.Contains(lower, "pay") {
		return domain.ActionCreateOrder
	}

	if strings.Contains(lower, "create listing") || strings.Contains(lower, "new listing") ||
		strings.Contains(lower, "add product") || strings.Contains(lower, "sell") ||
		strings.Contains(lower, "publish") {
		return domain.ActionCreateListing
	}

	return ""
}

func (s *Service) convertExecuteResult(result *commerce.ExecuteResult) *domain.ConversationResponse {
	response := domain.NewConversationResponse("", "")

	if result.MessageKey != "" {
		response.Text = result.MessageKey
	} else {
		response.Text = "OK"
	}

	if len(result.Data) > 0 {
		response.Metadata = result.Data
	}

	if result.RequiresConfirmation {
		response.SetConfirmation(domain.ConversationConfirmation{
			ActionID: result.ActionID,
			Summary:  result.MessageKey,
		})
	}

	if result.NextAction != nil {
		response.AddAction(result.NextAction.ActionType, result.NextAction.Description)
	}

	for _, e := range result.Errors {
		if e.Message != "" {
			response.Text = e.Message
			break
		}
	}

	return response
}
