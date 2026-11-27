package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

// WebhookHandler processes incoming payment provider webhook callbacks.
// It implements signature verification, idempotency, and reconciliation
// between provider-reported status and internal payment state.
type WebhookHandler struct {
	orders            domain.OrderRepository
	payments          domain.PaymentRepository
	refunds           domain.RefundRepository
	accounting        domain.PaymentAccounting
	locker            Locker
	events            domain.PaymentWebhookEventStore
	providers         domain.ProviderRepository
	signatureVerifier domain.WebhookSignatureVerifier
	normalizers       map[string]WebhookNormalizer
	logger            zerolog.Logger
}

// WebhookNormalizer verifies a provider's own signature scheme and maps its
// raw event onto WebhookPayload. Returning (nil, nil) acknowledges an event
// type that needs no processing.
type WebhookNormalizer interface {
	Normalize(orgID, signature string, body []byte) (*WebhookPayload, error)
}

type WebhookHandlerConfig struct {
	Payments domain.PaymentRepository
	Refunds  domain.RefundRepository
	// Accounting writes captures and refunds with their ledger entries, atomically. Required.
	Accounting domain.PaymentAccounting
	// Locker serializes a payment's changes with the API's operations on it. Production wiring must set it.
	Locker            Locker
	Events            domain.PaymentWebhookEventStore
	Providers         domain.ProviderRepository
	SignatureVerifier domain.WebhookSignatureVerifier
	// Orders, when set, keeps the order lifecycle in step with provider
	// callbacks (captured -> order paid, fully refunded -> order refunded).
	Orders domain.OrderRepository
	// Normalizers maps provider codes (e.g. "stripe") to provider-specific
	// signature verification + payload mapping. Providers without one use
	// SignatureVerifier and the generic WebhookPayload JSON body.
	Normalizers map[string]WebhookNormalizer
	Logger      zerolog.Logger
}

func NewWebhookHandler(config WebhookHandlerConfig) *WebhookHandler {
	return &WebhookHandler{
		orders:            config.Orders,
		payments:          config.Payments,
		refunds:           config.Refunds,
		accounting:        config.Accounting,
		locker:            config.Locker,
		events:            config.Events,
		providers:         config.Providers,
		signatureVerifier: config.SignatureVerifier,
		normalizers:       config.Normalizers,
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
	// receivedAt is when this event was first received.
	receivedAt int64
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
	var payload WebhookPayload
	if normalizer, ok := h.normalizers[providerCode]; ok {
		normalized, err := normalizer.Normalize(orgID, signature, body)
		if err != nil {
			if errors.Is(err, domain.ErrWebhookSignatureInvalid) {
				h.logger.Warn().Str("provider", providerCode).Msg("webhook signature verification failed")
				return domain.ErrWebhookSignatureInvalid
			}
			return err
		}
		if normalized == nil {
			return nil // event type not relevant to Payment Core
		}
		payload = *normalized
	} else {
		if h.signatureVerifier != nil {
			if err := h.signatureVerifier.Verify(providerCode, body, signature); err != nil {
				h.logger.Warn().Err(err).Str("provider", providerCode).Msg("webhook signature verification failed")
				return domain.ErrWebhookSignatureInvalid
			}
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("failed to parse webhook payload: %w", err)
		}
	}
	if payload.ProviderMessageID == "" {
		return fmt.Errorf("provider_message_id is required in webhook payload")
	}

	event := domain.NewPaymentWebhookEvent(providerCode, payload.ProviderMessageID, orgID,
		domain.PaymentWebhookEventType(payload.EventType), body, unixNow())
	// An event that cannot be kept is refused, so the provider sends it again.
	if err := h.events.PersistEvent(ctx, event); err != nil {
		return fmt.Errorf("failed to persist webhook event: %w", err)
	}

	claimed, err := h.events.TryClaim(ctx, providerCode, payload.ProviderMessageID)
	if err != nil {
		return fmt.Errorf("failed to claim webhook event: %w", err)
	}
	if !claimed {
		stored, err := h.events.FindByProviderMessageID(ctx, providerCode, payload.ProviderMessageID)
		if err != nil {
			return fmt.Errorf("failed to read webhook event: %w", err)
		}
		if stored.Status == domain.PaymentWebhookStatusProcessed || stored.Status == domain.PaymentWebhookStatusReview {
			h.logger.Info().Str("provider", providerCode).Str("msg_id", payload.ProviderMessageID).
				Msg("webhook event already handled")
			return nil
		}
		// Being processed elsewhere: if that processor stops, only another delivery will complete it.
		return ErrWebhookInProgress
	}

	payload.ProviderCode = providerCode
	payload.OrgID = orgID
	stored, _ := h.events.FindByProviderMessageID(ctx, providerCode, payload.ProviderMessageID)
	if stored != nil {
		payload.receivedAt = stored.CreatedAt
	}

	perr := h.processEvent(ctx, &payload)
	var review *reviewError
	switch {
	case errors.As(perr, &review):
		h.logger.Error().Str("provider", providerCode).Str("msg_id", payload.ProviderMessageID).
			Str("reason", review.reason).Msg("webhook event kept for review: nothing changed")
		if merr := h.events.MarkReview(ctx, providerCode, payload.ProviderMessageID, review.reason); merr != nil {
			return fmt.Errorf("webhook event marking failed: %w", merr)
		}
		return nil
	case perr != nil:
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

// ErrWebhookInProgress: another delivery of this event is being processed; the provider should send it again.
var ErrWebhookInProgress = errors.New("this webhook event is being processed")

// unknownGrace is how long an event about something not recorded yet is retried (the record may still be on its way)
// before it is kept for review.
const unknownGrace = 60 * 60 // seconds

// reviewError keeps an event for a person: it contradicts what is recorded, or refers to nothing recorded.
type reviewError struct{ reason string }

func (e *reviewError) Error() string { return "review: " + e.reason }

func needsReview(format string, args ...interface{}) error {
	return &reviewError{reason: fmt.Sprintf(format, args...)}
}

func unknownReference(payload *WebhookPayload, what string) error {
	if payload.receivedAt > 0 && unixNow()-payload.receivedAt > unknownGrace {
		return needsReview("unknown %s", what)
	}
	return fmt.Errorf("unknown %s (will be retried)", what)
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

// paymentOf finds the payment an event is about. A platform provider (Trezutil's own account) collects for every
// organisation: its payments belong to the order's organisation, so they are found by the provider's reference alone.
func (h *WebhookHandler) paymentOf(ctx context.Context, payload *WebhookPayload) (*domain.Payment, error) {
	providerEntry, err := h.providers.FindByCode(ctx, payload.OrgID, payload.ProviderCode)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve provider: %w", err)
	}
	lookupOrg := payload.OrgID
	if providerEntry.Configuration.Settings["scope"] == "platform" {
		lookupOrg = ""
	}
	p, err := h.payments.FindByProviderPaymentID(ctx, lookupOrg, providerEntry.ID, payload.ProviderPaymentID)
	if errors.Is(err, domain.ErrPaymentNotFound) {
		return nil, unknownReference(payload, "payment "+payload.ProviderPaymentID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find payment: %w", err)
	}
	return p, nil
}

// locked runs fn on the payment as currently stored, holding its lock.
func (h *WebhookHandler) locked(ctx context.Context, payload *WebhookPayload, fn func(context.Context, *domain.Payment) error) error {
	found, err := h.paymentOf(ctx, payload)
	if err != nil {
		return err
	}
	run := func(ctx context.Context) error {
		p, err := h.payments.FindByID(ctx, found.TenantID, found.ID)
		if err != nil {
			return fmt.Errorf("failed to load payment: %w", err)
		}
		return fn(ctx, p)
	}
	if h.locker == nil {
		return run(ctx)
	}
	return h.locker.WithLock(ctx, "payment:"+found.ID, run)
}

// reconcilePayment updates payment status based on a payment-level webhook.
func (h *WebhookHandler) reconcilePayment(ctx context.Context, payload *WebhookPayload) error {
	target, err := parseProviderStatus(payload.Status)
	if err != nil {
		return fmt.Errorf("invalid provider status in webhook: %w", err)
	}
	return h.locked(ctx, payload, func(ctx context.Context, p *domain.Payment) error {
		if target == domain.PaymentStatusRefunded || target == domain.PaymentStatusPartiallyRefunded {
			return needsReview("payment %s reported %s without a refund event", p.ID, target)
		}
		// Replays and late events: providers deliver at least once and out of order.
		if p.IsCaptured() {
			switch target {
			case domain.PaymentStatusCaptured, domain.PaymentStatusAuthorized, domain.PaymentStatusPending, domain.PaymentStatusCreated:
				if err := h.ensureEntries(ctx, p); err != nil {
					return err
				}
				syncOrderWithPayment(ctx, h.orders, p, h.logger)
				return nil
			}
			return needsReview("payment %s is captured; the provider reports %s", p.ID, target)
		}
		if p.IsTerminal() {
			if target == domain.PaymentStatusCaptured {
				return needsReview("the provider captured payment %s recorded as %s: money to return or to record", p.ID, p.Status)
			}
			syncOrderWithPayment(ctx, h.orders, p, h.logger)
			return nil
		}
		if p.Status == target || (target == domain.PaymentStatusAuthorized && p.IsProcessingOrLater() && p.Status != domain.PaymentStatusAuthorized) {
			syncOrderWithPayment(ctx, h.orders, p, h.logger)
			return nil
		}

		now := unixNow()
		if p.ProviderPaymentID == "" {
			p.ProviderPaymentID = payload.ProviderPaymentID
		}
		var err error
		switch target {
		case domain.PaymentStatusAuthorized:
			err = p.Authorize(payload.ProviderPaymentID, now, now)
		case domain.PaymentStatusCaptured:
			err = p.Capture(payload.ProviderPaymentID, now, now)
		case domain.PaymentStatusFailed:
			err = p.Fail("provider webhook: "+payload.Status, now)
		case domain.PaymentStatusCancelled:
			err = p.Cancel(now)
		case domain.PaymentStatusExpired:
			err = p.Expire(now)
		case domain.PaymentStatusCreated:
			return nil // an older state
		default:
			err = p.TransitionTo(target, now)
		}
		if err != nil {
			return needsReview("payment %s cannot move from %s to %s", p.ID, p.Status, target)
		}

		if p.Status == domain.PaymentStatusCaptured {
			entries, err := domain.CaptureEntries(p, now)
			if err != nil {
				return err
			}
			// The capture exists only with its accounting: on failure nothing changes and the event is retried.
			if err := h.accounting.CommitCapture(ctx, p, entries); err != nil {
				return fmt.Errorf("failed to record capture: %w", err)
			}
		} else if err := h.payments.Update(ctx, p); err != nil {
			return fmt.Errorf("failed to persist payment after webhook: %w", err)
		}
		if p.Status == domain.PaymentStatusFailed {
			observability.PaymentFailed(ctx, p.ID, p.OrderID, "webhook", "provider webhook: "+payload.Status)
		}
		syncOrderWithPayment(ctx, h.orders, p, h.logger)
		return nil
	})
}

func (h *WebhookHandler) ensureEntries(ctx context.Context, p *domain.Payment) error {
	entries, err := domain.CaptureEntries(p, unixNow())
	if err != nil {
		return err
	}
	if _, err := h.accounting.EnsureEntries(ctx, entries); err != nil {
		return fmt.Errorf("failed to post missing capture entries: %w", err)
	}
	return nil
}

// reconcileRefund updates refund status based on a refund webhook.
func (h *WebhookHandler) reconcileRefund(ctx context.Context, payload *WebhookPayload) error {
	outcome := classifyRefundStatus(payload.Status)
	return h.locked(ctx, payload, func(ctx context.Context, p *domain.Payment) error {
		refunds, err := h.refunds.FindByPaymentID(ctx, p.TenantID, p.ID, 0, 100)
		if err != nil {
			return fmt.Errorf("failed to find refunds: %w", err)
		}
		var refund *domain.Refund
		for _, r := range refunds {
			if payload.ProviderRefundID != "" && r.ProviderRefundID == payload.ProviderRefundID {
				refund = r
				break
			}
		}
		if refund == nil {
			// Not recorded (yet): a refund made on the provider's dashboard ends up in review.
			return unknownReference(payload, "refund "+payload.ProviderRefundID)
		}
		if outcome == refundUnknown {
			return needsReview("unknown refund status %q for refund %s", payload.Status, refund.ID)
		}
		if outcome == refundPending {
			return nil
		}
		now := unixNow()
		if refund.Status.IsTerminal() {
			switch {
			case refund.Status == domain.RefundStatusSucceeded && outcome == refundSucceeded:
				entries, err := domain.RefundEntries(refund, now)
				if err != nil {
					return err
				}
				if _, err := h.accounting.EnsureEntries(ctx, entries); err != nil {
					return fmt.Errorf("failed to post missing refund entries: %w", err)
				}
				syncOrderWithPayment(ctx, h.orders, p, h.logger)
				return nil
			case refund.Status == domain.RefundStatusFailed && outcome == refundFailed:
				return nil
			}
			return needsReview("refund %s is %s; the provider reports %s", refund.ID, refund.Status, payload.Status)
		}

		if outcome == refundFailed {
			if err := refund.Fail("provider webhook: "+payload.Status, now); err != nil {
				return fmt.Errorf("failed to fail refund: %w", err)
			}
			if err := h.refunds.Update(ctx, refund); err != nil {
				return fmt.Errorf("failed to persist refund after webhook: %w", err)
			}
			return nil
		}

		if err := refund.Succeed(payload.ProviderRefundID, now, now); err != nil {
			return fmt.Errorf("failed to succeed refund: %w", err)
		}
		// The refund was pending, so the payment's refunded amount was not updated yet.
		if err := p.MarkPartiallyRefunded(refund.Amount, now); err != nil {
			return needsReview("refund %s succeeded but payment %s cannot take it: %v", refund.ID, p.ID, err)
		}
		entries, err := domain.RefundEntries(refund, now)
		if err != nil {
			return err
		}
		if err := h.accounting.CommitRefund(ctx, refund, p, entries); err != nil {
			return fmt.Errorf("failed to record refund: %w", err)
		}
		syncOrderWithPayment(ctx, h.orders, p, h.logger)
		return nil
	})
}
