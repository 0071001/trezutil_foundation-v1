package whatsapp

import (
	"github.com/trezutil/trezutil/internal/domain"
)

// Presenter transforms ConversationResponse into WhatsApp-compatible outbound payloads.
type Presenter struct{}

// NewPresenter creates a new WhatsApp presenter.
func NewPresenter() *Presenter {
	return &Presenter{}
}

// ToOutboundMessages converts a channel-neutral ConversationResponse into WhatsApp outbound messages.
func (p *Presenter) ToOutboundMessages(response *domain.ConversationResponse) ([]OutboundMessage, error) {
	adapter := NewAdapter(Config{})
	return adapter.BuildOutbound(response)
}

// MapTextResponse creates a simple text response for WhatsApp.
func (p *Presenter) MapTextResponse(text string) []OutboundMessage {
	return []OutboundMessage{{
		Type: "text",
		Text: text,
	}}
}

// MapError creates a customer-friendly error response for WhatsApp.
func (p *Presenter) MapError(code string) []OutboundMessage {
	errorMessages := map[string]string{
		"INSUFFICIENT_CREDITS":   "You don't have enough credits for this action. Please top up your account.",
		"ACTION_NOT_IMPLEMENTED": "This feature is coming soon. Stay tuned!",
		"ORDER_NOT_FOUND":        "Order not found. Please check the order ID and try again.",
		"LISTING_NOT_FOUND":      "Listing not found. Please try another search.",
		"VALIDATION_FAILED":      "Some required information is missing. Please try again.",
		"UNAUTHORIZED":           "You don't have permission to perform this action.",
	}
	if msg, ok := errorMessages[code]; ok {
		return p.MapTextResponse(msg)
	}
	return p.MapTextResponse("Sorry, something went wrong. Please try again.")
}
