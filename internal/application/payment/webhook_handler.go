package payment

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// WebhookHandler processes incoming payment provider webhook callbacks.
// It implements signature verification, idempotency, and reconciliation
// between provider-reported status and internal payment state.
type WebhookHandler struct {
	payments          domain.PaymentRepository
	refunds           domain.RefundRepository
	ledger            domain.LedgerRepository
	events            domain.PaymentWebhookEventStore
	providers         domain.ProviderRepository
	signatureVerifier domain.WebhookSignatureVerifier
	logger            zerolog.Logger
}

type WebhookHandlerConfig struct {
	Payments          domain.PaymentRepository
	Refunds           domain.RefundRepository
	Ledger            domain.LedgerRepository
	Events            domain.PaymentWebhookEventStore
	Providers         domain.ProviderRepository
	SignatureVerifier domain.WebhookSignatureVerifier
	Logger            zerolog.Logger
}

func NewWebhookHandler(config WebhookHandlerConfig) *WebhookHandler {
	return &WebhookHandler{
		payments:          config.Payments,
		refunds:           config.Refunds,
		ledger:            config.Ledger,
		events:            config.Events,
		providers:         config.Providers,
		signatureVerifier: config.SignatureVerifier,
		logger:            config.Logger,
	}
}

// WebhookPayload is the standard shape of a payment provider webhook body.
// Provider-specific adapters map their raw payload into this structure.
type WebhookPayload struct {
	EventType         string          `json:"event_type"`
	ProviderCode      string          `json:"provider_code"`
	OrgID             string          `json:"-"`
	ProviderMessageID string          `json:"provider_message_id"`
	ProviderPaymentID string          `json:"provider_payment_id,omitempty"`
	ProviderRefundID  string          `json:"provider_refund_id,omitempty"`
	Status            string          `json:"status"`
	Amount            json.RawMessage `json:"amount,omitempty"`
	Currency          string          `json:"currency,omitempty"`
	Timestamp         int64           `json:"timestamp,omitempty"`
}

// ProcessWebhook handles a single webhook invocation: verifies signature,
// persists the event, claims it (idempotent), and reconciles provider status
// to internal state.
func (h *WebhookHandler) ProcessWebhook(
	ctx context.Context,
	orgID, providerCode string,
	signature string,
	body []byte,
) error {
	if h.signatureVerifier != nil {
		if err := h.signatureVerifier.Verify(providerCode, body, signature); err != nil {
			h.logger.Warn().Err(err).Str("provider", providerCode).Msg("webhook signature verification failed")
			return domain.ErrWebhookSignatureInvalid
		}
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}
	if payload.ProviderMessageID == "" {
		return fmt.Errorf("provider_message_id is required in webhook payload")
	}

	event := domain.NewPaymentWebhookEvent(providerCode, payload.ProviderMessageID, orgID,
		domain.PaymentWebhookEventType(payload.EventType), body, unixNow())
	if err := h.events.PersistEvent(ctx, event); err != nil {
		h.logger.Warn().Err(err).Msg("failed to persist webhook event (may be duplicate)")
	}

	claimed, err := h.events.TryClaim(ctx, providerCode, payload.ProviderMessageID)
	if err != nil {
		return fmt.Errorf("failed to claim webhook event: %w", err)
	}
	if !claimed {
		h.logger.Info().Str("provider", providerCode).
			Str("msg_id", payload.ProviderMessageID).
			Msg("webhook event already processed or being processed")
		return nil
	}

	payload.ProviderCode = providerCode
	payload.OrgID = orgID

	if perr := h.processEvent(ctx, &payload); perr != nil {
		if merr := h.events.MarkFailed(ctx, providerCode, payload.ProviderMessageID, perr.Error()); merr != nil {
			h.logger.Warn().Err(merr).Msg("failed to mark webhook event as failed")
		}
		return fmt.Errorf("webhook processing failed: %w", perr)
	}

	if merr := h.events.MarkProcessed(ctx, providerCode, payload.ProviderMessageID); merr != nil {
		h.logger.Warn().Err(merr).Str("provider", providerCode).
			Str("msg_id", payload.ProviderMessageID).
			Msg("failed to mark webhook event as processed")
		return fmt.Errorf("webhook event marking failed: %w", merr)
	}

	return nil
}

// processEvent performs the reconciliation for a claimed webhook event.
func (h *WebhookHandler) processEvent(ctx context.Context, payload *WebhookPayload) error {
	eventType := domain.PaymentWebhookEventType(payload.EventType)

	switch eventType {
	case domain.WebhookEventPaymentSucceeded, domain.WebhookEventPaymentFailed,
		domain.WebhookEventPaymentCanceled, domain.WebhookEventPaymentExpired:
		return h.reconcilePayment(ctx, payload)

	case domain.WebhookEventRefundSucceeded, domain.WebhookEventRefundFailed,
		domain.WebhookEventChargeRefunded:
		return h.reconcileRefund(ctx, payload)

	default:
		h.logger.Warn().Str("event_type", payload.EventType).
			Str("provider", payload.ProviderCode).
			Msg("unhandled webhook event type")
		return nil
	}
}

// reconcilePayment updates payment status based on a payment-level webhook.
func (h *WebhookHandler) reconcilePayment(ctx context.Context, payload *WebhookPayload) error {
	providerEntry, err := h.providers.FindByCode(ctx, payload.OrgID, payload.ProviderCode)
	if err != nil {
		return fmt.Errorf("failed to resolve provider: %w", err)
	}

	p, err := h.payments.FindByProviderPaymentID(ctx, payload.OrgID, providerEntry.ID, payload.ProviderPaymentID)
	if err != nil {
		if err == domain.ErrPaymentNotFound {
			h.logger.Warn().
				Str("provider", payload.ProviderCode).
				Str("provider_payment_id", payload.ProviderPaymentID).
				Msg("webhook references unknown payment")
			return nil
		}
		return fmt.Errorf("failed to find payment: %w", err)
	}

	now := unixNow()
	target, err := parseProviderStatus(payload.Status)
	if err != nil {
		return fmt.Errorf("invalid provider status in webhook: %w", err)
	}

	if p.ProviderPaymentID == "" {
		p.ProviderPaymentID = payload.ProviderPaymentID
	}

	switch target {
	case domain.PaymentStatusAuthorized:
		if err := p.Authorize(payload.ProviderPaymentID, now, now); err != nil {
			return fmt.Errorf("failed to authorize payment: %w", err)
		}
	case domain.PaymentStatusCaptured:
		if err := p.Capture(payload.ProviderPaymentID, now, now); err != nil {
			return fmt.Errorf("failed to capture payment: %w", err)
		}
		if err := h.postLedgerForCapture(ctx, p); err != nil {
			h.logger.Warn().Err(err).Str("payment", p.ID).Msg("ledger posting for webhook capture failed")
		}
	case domain.PaymentStatusFailed:
		if err := p.Fail("provider webhook: "+payload.Status, now); err != nil {
			return fmt.Errorf("failed to fail payment: %w", err)
		}
	case domain.PaymentStatusCancelled:
		if err := p.Cancel(now); err != nil {
			return fmt.Errorf("failed to cancel payment: %w", err)
		}
	case domain.PaymentStatusExpired:
		if err := p.Expire(now); err != nil {
			return fmt.Errorf("failed to expire payment: %w", err)
		}
	default:
		if err := p.TransitionTo(target, now); err != nil {
			return fmt.Errorf("failed to transition payment to %s: %w", target, err)
		}
	}

	if err := h.payments.Update(ctx, p); err != nil {
		return fmt.Errorf("failed to persist payment after webhook: %w", err)
	}

	return nil
}

// reconcileRefund updates refund status based on a refund webhook.
func (h *WebhookHandler) reconcileRefund(ctx context.Context, payload *WebhookPayload) error {
	providerEntry, err := h.providers.FindByCode(ctx, payload.OrgID, payload.ProviderCode)
	if err != nil {
		return fmt.Errorf("failed to resolve provider: %w", err)
	}

	p, err := h.payments.FindByProviderPaymentID(ctx, payload.OrgID, providerEntry.ID, payload.ProviderPaymentID)
	if err != nil {
		if err == domain.ErrPaymentNotFound {
			h.logger.Warn().
				Str("provider", payload.ProviderCode).
				Str("provider_refund_id", payload.ProviderRefundID).
				Msg("webhook references unknown payment")
			return nil
		}
		return fmt.Errorf("failed to find payment for refund webhook: %w", err)
	}

	refunds, err := h.refunds.FindByPaymentID(ctx, p.TenantID, p.ID, 0, 100)
	if err != nil {
		return fmt.Errorf("failed to find refunds: %w", err)
	}

	var targetRefund *domain.Refund
	for _, r := range refunds {
		if r.ProviderRefundID == payload.ProviderRefundID {
			targetRefund = r
			break
		}
	}
	if targetRefund == nil {
		h.logger.Warn().
			Str("provider", payload.ProviderCode).
			Str("provider_refund_id", payload.ProviderRefundID).
			Msg("webhook references unknown refund")
		return nil
	}

	now := unixNow()
	targetStatus, _ := parseProviderStatus(payload.Status)

	if targetStatus == domain.PaymentStatusFailed {
		if err := targetRefund.Fail("provider webhook: "+payload.Status, now); err != nil {
			return fmt.Errorf("failed to fail refund: %w", err)
		}
	} else {
		if err := targetRefund.Succeed(payload.ProviderRefundID, now, now); err != nil {
			return fmt.Errorf("failed to succeed refund: %w", err)
		}
		if err := h.postLedgerForRefund(ctx, targetRefund); err != nil {
			h.logger.Warn().Err(err).Str("refund", targetRefund.ID).
				Msg("ledger posting for webhook refund failed")
		}
	}

	if err := h.refunds.Update(ctx, targetRefund); err != nil {
		return fmt.Errorf("failed to persist refund after webhook: %w", err)
	}

	return nil
}

func (h *WebhookHandler) postLedgerForCapture(ctx context.Context, p *domain.Payment) error {
	now := unixNow()
	entries := []*domain.LedgerEntry{
		domain.NewLedgerEntry(
			p.TenantID, &p.ID, nil,
			domain.LedgerAccountCash, domain.LedgerEntryTypeDebit,
			p.Amount, "ledger_webhook_capture_"+p.ID, now,
		),
		domain.NewLedgerEntry(
			p.TenantID, &p.ID, nil,
			domain.LedgerAccountRevenue, domain.LedgerEntryTypeCredit,
			p.Amount, "ledger_webhook_capture_rev_"+p.ID, now,
		),
	}
	for _, e := range entries {
		if err := e.Post(now); err != nil {
			return fmt.Errorf("ledger entry post failed: %w", err)
		}
		if _, err := h.ledger.Post(ctx, e); err != nil {
			return fmt.Errorf("ledger persist failed: %w", err)
		}
	}
	return nil
}

func (h *WebhookHandler) postLedgerForRefund(ctx context.Context, r *domain.Refund) error {
	now := unixNow()
	e := domain.NewLedgerEntry(
		r.TenantID, nil, &r.ID,
		domain.LedgerAccountRefund, domain.LedgerEntryTypeDebit,
		r.Amount, "ledger_webhook_refund_"+r.ID, now,
	)
	if err := e.Post(now); err != nil {
		return err
	}
	if _, err := h.ledger.Post(ctx, e); err != nil {
		return err
	}
	return nil
}
