package api

import (
	"bytes"
	"context"
	"crypto/subtle"
	"io"
	"net/http"
	"os"

	"github.com/trezutil/trezutil/internal/domain"
	whatsappinfra "github.com/trezutil/trezutil/internal/infrastructure/whatsapp"
)

type WebhookService interface {
	VerifyWebhook(body []byte, signature string) error
	ProcessWebhook(ctx context.Context, channel domain.Channel, senderPhone string, conversationExternalID string, wabaID string, messages []domain.Message) (*domain.ConversationResponse, error)
}

type WhatsappHandler struct {
	service WebhookService
	adapter *whatsappinfra.Adapter
}

func NewWhatsappHandler(service WebhookService, adapters ...*whatsappinfra.Adapter) *WhatsappHandler {
	var adapter *whatsappinfra.Adapter
	if len(adapters) > 0 && adapters[0] != nil {
		adapter = adapters[0]
	} else {
		secret := os.Getenv("WHATSAPP_WEBHOOK_SECRET")
		if secret == "" {
			secret = os.Getenv("WHATSAPP_APP_SECRET")
		}
		adapter = whatsappinfra.NewAdapter(whatsappinfra.Config{WebhookSecret: secret})
	}

	return &WhatsappHandler{
		service: service,
		adapter: adapter,
	}
}

type WhatsAppHandler = WhatsappHandler

func NewWhatsAppHandler(service WebhookService, adapters ...*whatsappinfra.Adapter) *WhatsAppHandler {
	return NewWhatsappHandler(service, adapters...)
}

func (h *WhatsappHandler) WebhookH(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{
			Code:    "WEBHOOK_READ_FAILED",
			Message: "Failed to read webhook body",
		}}, http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	if h.service != nil {
		if err := h.service.VerifyWebhook(body, signature); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{
				Code:    "WEBHOOK_VERIFICATION_FAILED",
				Message: "Invalid webhook signature",
			}}, http.StatusUnauthorized)
			return
		}
	} else if h.adapter != nil {
		verificationRequest := r.Clone(r.Context())
		verificationRequest.Body = io.NopCloser(bytes.NewReader(body))
		verificationRequest.Header.Set("X-Hub-Signature-256", signature)
		if err := h.adapter.VerifyWebhook(verificationRequest); err != nil {
			writeJSON(w, ErrorResponse{Error: Error{
				Code:    "WEBHOOK_VERIFICATION_FAILED",
				Message: "Invalid webhook signature",
			}}, http.StatusUnauthorized)
			return
		}
	}

	if h.adapter == nil {
		writeJSON(w, ErrorResponse{Error: Error{
			Code:    "WEBHOOK_ADAPTER_UNAVAILABLE",
			Message: "WhatsApp adapter is not configured",
		}}, http.StatusInternalServerError)
		return
	}

	payload, err := h.adapter.ParsePayload(body)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{
			Code:    "WEBHOOK_PARSE_FAILED",
			Message: "Invalid webhook payload",
		}}, http.StatusBadRequest)
		return
	}

	if h.service == nil {
		writeJSON(w, ErrorResponse{Error: Error{
			Code:    "WEBHOOK_SERVICE_UNAVAILABLE",
			Message: "WhatsApp service is not configured",
		}}, http.StatusInternalServerError)
		return
	}

	responses := make([]*domain.ConversationResponse, 0)
	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			group := &whatsappinfra.ProviderPayload{
				Entry: []whatsappinfra.ProviderEntry{
					{Changes: []whatsappinfra.ProviderChange{change}},
				},
			}
			channel, senderPhone, conversationExternalID, wabaID, messages, err := h.adapter.ParseInbound(group)
			if err != nil {
				writeJSON(w, ErrorResponse{Error: Error{
					Code:    "WEBHOOK_PARSE_FAILED",
					Message: "Invalid webhook message",
				}}, http.StatusBadRequest)
				return
			}
			if len(messages) == 0 {
				continue
			}

			response, err := h.service.ProcessWebhook(r.Context(), channel, senderPhone, conversationExternalID, wabaID, messages)
			if err != nil {
				writeJSON(w, ErrorResponse{Error: Error{
					Code:    "WEBHOOK_PROCESSING_FAILED",
					Message: "Webhook processing failed",
				}}, http.StatusInternalServerError)
				return
			}
			if response != nil {
				responses = append(responses, response)
			}
		}
	}

	if len(responses) == 0 {
		writeJSON(w, domain.NewConversationResponse("", "en"), http.StatusOK)
		return
	}

	writeJSON(w, aggregateConversationResponses(responses), http.StatusOK)
}

func aggregateConversationResponses(responses []*domain.ConversationResponse) *domain.ConversationResponse {
	if len(responses) == 1 {
		return responses[0]
	}

	aggregate := domain.NewConversationResponse("", responses[0].Language)
	for _, response := range responses {
		if aggregate.Language == "" && response.Language != "" {
			aggregate.Language = response.Language
		}
		if response.Text != "" {
			if aggregate.Text == "" {
				aggregate.Text = response.Text
			} else {
				aggregate.Text += " " + response.Text
			}
		}
		if response.StructuredContent != nil {
			aggregate.StructuredContent = response.StructuredContent
		}
		aggregate.Cards = append(aggregate.Cards, response.Cards...)
		aggregate.Actions = append(aggregate.Actions, response.Actions...)
		if aggregate.Confirmation == nil && response.Confirmation != nil {
			confirmation := *response.Confirmation
			aggregate.Confirmation = &confirmation
		}
		for key, value := range response.Metadata {
			if aggregate.Metadata == nil {
				aggregate.Metadata = make(map[string]interface{})
			}
			aggregate.Metadata[key] = value
		}
	}

	return aggregate
}

// VerifySubscriptionH answers Meta's webhook subscription handshake
// (GET ?hub.mode=subscribe&hub.verify_token=...&hub.challenge=...).
func VerifySubscriptionH(verifyToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if verifyToken == "" || q.Get("hub.mode") != "subscribe" ||
			subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(verifyToken)) != 1 {
			writeJSON(w, ErrorResponse{Error: Error{Code: "FORBIDDEN", Message: "Invalid verification request"}}, http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(q.Get("hub.challenge")))
	}
}
