package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/observability"
)

// PaymentService is the application-use-case layer for the Payment Core.
// It orchestrates payment operations across domain aggregates, provider
// integration (via ProviderOrchestrator), and the ledger. All mutations
// flow through the domain aggregates — no direct SQL updates.
type PaymentService struct {
	payments     domain.PaymentRepository
	refunds      domain.RefundRepository
	accounting   domain.PaymentAccounting
	orders       domain.OrderRepository
	orchestrator *provider.ProviderOrchestrator
	logger       zerolog.Logger
	locker       Locker
}

type PaymentServiceConfig struct {
	Payments domain.PaymentRepository
	Refunds  domain.RefundRepository
	// Accounting writes captures and refunds together with their ledger entries, atomically. Required.
	Accounting   domain.PaymentAccounting
	Orders       domain.OrderRepository
	Orchestrator *provider.ProviderOrchestrator
	Logger       zerolog.Logger
	// Locker serializes money-moving operations per payment across all
	// instances. Production wiring must set it (PostgreSQL advisory locks).
	Locker Locker
}

func NewPaymentService(config PaymentServiceConfig) *PaymentService {
	return &PaymentService{
		payments:     config.Payments,
		refunds:      config.Refunds,
		accounting:   config.Accounting,
		orders:       config.Orders,
		orchestrator: config.Orchestrator,
		logger:       config.Logger,
		locker:       config.Locker,
	}
}

// paymentProviderAdapter type-asserts a domain.ProviderAdapter to a
// PaymentProviderAdapter. The ProviderOrchestrator passes the base
// ProviderAdapter interface into the perform callback; payment operations
// require the extended PaymentProviderAdapter interface.
func paymentProviderAdapter(adapter domain.ProviderAdapter) (domain.PaymentProviderAdapter, error) {
	ppa, ok := adapter.(domain.PaymentProviderAdapter)
	if !ok {
		return nil, domain.ErrPaymentOperationNotSupported
	}
	return ppa, nil
}

// CreatePayment creates a payment intent and initiates it with the provider.
// If the provider is unavailable, the payment remains in CREATED status for retry.
func (s *PaymentService) CreatePayment(
	ctx context.Context,
	orgID, actorID, orderID string,
	amount domain.Money,
	methodType domain.PaymentMethodType,
	methodID string,
	idempotencyKey string,
	metadata map[string]interface{},
) (*domain.Payment, error) {
	if orgID == "" {
		return nil, errors.New("organization_id is required")
	}
	if orderID == "" {
		return nil, errors.New("order_id is required")
	}
	if amount.AmountMinor <= 0 {
		return nil, errors.New("amount must be positive")
	}
	if methodID == "" {
		return nil, errors.New("payment method id is required")
	}
	if idempotencyKey == "" {
		return nil, errors.New("idempotency_key is required")
	}

	now := unixNow()

	existing, err := s.payments.FindByIdempotencyKey(ctx, orgID, idempotencyKey)
	if err == nil {
		// Idempotency hit: verify the existing payment matches the requested order,
		// amount, and currency. A request with the same key but different financial
		// parameters is a conflict — never silently reuse.
		if existing.OrderID != orderID {
			return nil, domain.ErrIdempotencyConflict
		}
		if existing.Amount.AmountMinor != amount.AmountMinor {
			return nil, domain.ErrIdempotencyConflict
		}
		if existing.Amount.Currency != amount.Currency {
			return nil, domain.ErrIdempotencyConflict
		}
		return s.resumePayment(ctx, orgID, actorID, existing, idempotencyKey), nil
	}
	if !errors.Is(err, domain.ErrPaymentNotFound) {
		return nil, fmt.Errorf("failed to check existing payment: %w", err)
	}

	// Validate order ownership and pricing before creating payment.
	// The order's TotalAmount + Currency are authoritative — client-supplied
	// values must match exactly.
	order, err := s.orders.FindByIDAndOrg(ctx, orgID, orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to look up order %s: %w", orderID, err)
	}
	if order == nil {
		return nil, fmt.Errorf("order %s not found", orderID)
	}
	if order.OrganizationID != orgID {
		return nil, fmt.Errorf("order %s does not belong to organization %s", orderID, orgID)
	}
	if order.TotalAmount.AmountMinor != amount.AmountMinor {
		return nil, fmt.Errorf("payment amount %d does not match order total %d", amount.AmountMinor, order.TotalAmount.AmountMinor)
	}
	if order.Currency != amount.Currency {
		return nil, fmt.Errorf("payment currency %s does not match order currency %s", amount.Currency, order.Currency)
	}
	// Only orders awaiting payment can be charged; a cancelled, expired or
	// already-paid order must never accept a new payment.
	if order.Status != domain.OrderStatusDraft && order.Status != domain.OrderStatusPendingPayment {
		return nil, domain.ErrOrderNotPayable
	}

	providerEntry, err := s.orchestrator.SelectProvider(ctx, orgID, domain.CapabilityPayment, nil)
	if err != nil {
		return nil, fmt.Errorf("no payment provider available: %w", err)
	}

	p := domain.NewPayment(orgID, orderID, amount, methodType, methodID, providerEntry.Provider.ID, idempotencyKey, now)
	p.Metadata = metadata
	if err := p.ValidateInvariants(); err != nil {
		return nil, fmt.Errorf("invalid payment: %w", err)
	}
	if err := s.payments.Create(ctx, p); err != nil {
		if errors.Is(err, domain.ErrPaymentAlreadyExists) {
			existing, err := s.payments.FindByIdempotencyKey(ctx, orgID, idempotencyKey)
			if err != nil {
				return nil, fmt.Errorf("idempotent payment exists but could not be loaded: %w", err)
			}
			return existing, nil
		}
		if errors.Is(err, domain.ErrOrderNotPayable) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// Initiating a payment submits the order (draft -> pending_payment).
	if err := submitOrderForPayment(ctx, s.orders, orgID, orderID); err != nil {
		s.logger.Warn().Err(err).Str("order", orderID).Msg("failed to submit order for payment")
	}

	return s.drivePayment(ctx, orgID, actorID, p, idempotencyKey), nil
}

// resumePayment answers an idempotent replay. A payment that never reached
// the provider (e.g. timeout) is initiated again with the same idempotency
// key (the provider returns the original intent, never a second charge); a
// payment awaiting the payer gets a fresh client action.
func (s *PaymentService) resumePayment(ctx context.Context, orgID, actorID string, p *domain.Payment, idempotencyKey string) *domain.Payment {
	switch {
	case p.Status == domain.PaymentStatusCreated && p.ProviderPaymentID == "":
		if !resendable(p.CreatedAt) {
			s.logger.Warn().Str("payment", p.ID).Msg("payment too old to send again safely; check it with the provider")
			return p
		}
		return s.drivePayment(ctx, orgID, actorID, p, idempotencyKey)
	case (p.Status == domain.PaymentStatusCreated || p.Status == domain.PaymentStatusPending) && p.ProviderPaymentID != "":
		result, err := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
			Capability:     domain.CapabilityPayment,
			Operation:      string(domain.PaymentProviderOpGet),
			IdempotencyKey: idempotencyKey,
		}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
			ppa, err := paymentProviderAdapter(adapter)
			if err != nil {
				return domain.ProviderResult{}, err
			}
			resp, rerr := ppa.GetPayment(ctx, p.ProviderPaymentID)
			if rerr != nil {
				return domain.ProviderResult{Success: false, ErrorMessage: rerr.Error()}, rerr
			}
			return domain.ProviderResult{Success: true, ExternalReference: resp.ProviderPaymentID, Data: withStatus(resp)}, nil
		})
		if err == nil && result != nil && result.Success {
			p.ClientAction = clientAction(result.Data)
			// Apply the provider's current state (e.g. the payer confirmed
			// in the browser and the intent is now authorized/captured).
			if statusStr, _ := result.Data["status"].(string); statusStr != "" {
				if aerr := s.applyProviderResult(ctx, p, statusStr, result.ExternalReference); aerr != nil {
					s.logger.Error().Err(aerr).Str("payment", p.ID).Msg("provider state not recorded; it will be retried")
				} else if p.IsCaptured() {
					syncOrderWithPayment(ctx, s.orders, p, s.logger)
				}
			}
		}
	}
	return p
}

// drivePayment initiates a persisted payment with the provider and applies
// the outcome. Transient failures leave it CREATED so a replay with the same
// idempotency key can resume it; definitive declines fail it.
func (s *PaymentService) drivePayment(ctx context.Context, orgID, actorID string, p *domain.Payment, idempotencyKey string) *domain.Payment {
	providerResult, execErr := s.initiateWithProvider(ctx, orgID, actorID, p, idempotencyKey)
	if execErr != nil {
		if errors.Is(execErr, domain.ErrProviderDeclined) {
			s.recordFailure(ctx, p, execErr.Error())
			return p
		}
		s.logger.Warn().Err(execErr).Str("payment", p.ID).Msg("payment initiation failed; payment left in CREATED for retry")
		return p
	}

	if !providerResult.Success {
		s.recordFailure(ctx, p, providerResult.ErrorMessage)
		return p
	}
	p.ClientAction = clientAction(providerResult.Data)

	statusStr, _ := providerResult.Data["status"].(string)
	providerPaymentID := providerResult.ExternalReference
	if providerPaymentID == "" {
		if id, ok := providerResult.Data["provider_payment_id"].(string); ok {
			providerPaymentID = id
		}
	}

	if err := s.applyProviderResult(ctx, p, statusStr, providerPaymentID); err != nil {
		s.logger.Error().Err(err).Str("payment", p.ID).Msg("provider result not recorded; the webhook or a status read will retry it")
		return p
	}
	if p.IsCaptured() {
		syncOrderWithPayment(ctx, s.orders, p, s.logger)
	}
	return p
}

// withStatus flattens a provider response into ProviderResult data.
func withStatus(resp domain.PaymentProviderResponse) map[string]interface{} {
	data := map[string]interface{}{"status": resp.Status, "provider_payment_id": resp.ProviderPaymentID}
	for k, v := range resp.Data {
		data[k] = v
	}
	return data
}

// clientAction extracts payer-facing completion data (never persisted).
func clientAction(data map[string]interface{}) map[string]string {
	out := map[string]string{}
	for _, k := range []string{"client_secret", "publishable_key", "provider_code", "checkout_url", "instructions", "charged_amount", "fee_amount"} {
		if v, ok := data[k].(string); ok && v != "" {
			out[k] = v
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// initiateWithProvider calls the provider orchestrator to create a payment.
func (s *PaymentService) initiateWithProvider(
	ctx context.Context, orgID, actorID string, p *domain.Payment,
	idempotencyKey string,
) (*provider.ExecuteResult, error) {
	req := domain.PaymentProviderRequest{
		Amount:            p.Amount,
		PaymentMethodType: p.PaymentMethodType,
		PaymentMethodID:   p.PaymentMethodID,
		IdempotencyKey:    idempotencyKey,
		Metadata:          p.Metadata,
	}

	return s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpCreate),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.CreatePayment(domain.WithProviderIdempotencyKey(ctx, idempotencyKey), req)
		if rerr != nil {
			return domain.ProviderResult{
				Success:           false,
				ExternalReference: resp.ProviderPaymentID,
				ErrorCode:         resp.ErrorCode,
				ErrorMessage:      rerr.Error(),
			}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderPaymentID,
			Data:              withStatus(resp),
		}, nil
	})
}

// errContradiction: the provider reports a state that cannot follow the recorded one. Nothing is changed; the
// reconciliation report and a person decide.
var errContradiction = errors.New("provider state contradicts the recorded payment")

// applyProviderResult records the provider's state of a payment. A capture is recorded only together with its ledger
// entries (one transaction): if that fails nothing changes, p is left as stored, and the error is returned so it is
// retried (webhook, status read) instead of being shown as done. The provider's reference is saved first, so the
// retry can find the payment.
func (s *PaymentService) applyProviderResult(ctx context.Context, p *domain.Payment, statusStr, providerPaymentID string) error {
	target, err := parseProviderStatus(statusStr)
	if err != nil {
		return fmt.Errorf("unknown provider status %q: %w", statusStr, err)
	}
	// A refund is recorded from a refund (with its amount and accounting), never from a payment status.
	if target == domain.PaymentStatusRefunded || target == domain.PaymentStatusPartiallyRefunded {
		return fmt.Errorf("%w: %s without a refund record", errContradiction, target)
	}
	if p.IsCaptured() {
		switch target {
		case domain.PaymentStatusCaptured, domain.PaymentStatusAuthorized, domain.PaymentStatusPending, domain.PaymentStatusCreated:
			return s.ensureCaptureEntries(ctx, p)
		}
		return fmt.Errorf("%w: %s after a capture", errContradiction, target)
	}
	if p.IsTerminal() {
		if target == p.Status {
			return nil
		}
		return fmt.Errorf("%w: %s for a payment recorded as %s", errContradiction, target, p.Status)
	}
	if target == domain.PaymentStatusCreated && p.Status != domain.PaymentStatusCreated {
		return nil // an older state
	}

	now := unixNow()
	if providerPaymentID != "" && p.ProviderPaymentID == "" && target == domain.PaymentStatusCaptured && p.Status == domain.PaymentStatusCreated {
		saved := *p
		p.ProviderPaymentID = providerPaymentID
		if err := p.TransitionTo(domain.PaymentStatusPending, now); err != nil {
			*p = saved
			return err
		}
		if err := s.payments.Update(ctx, p); err != nil {
			*p = saved
			return fmt.Errorf("provider reference not saved: %w", err)
		}
	}

	before := *p
	if providerPaymentID != "" {
		p.ProviderPaymentID = providerPaymentID
	}
	switch target {
	case domain.PaymentStatusAuthorized:
		err = p.Authorize(p.ProviderPaymentID, now, now)
	case domain.PaymentStatusCaptured:
		err = p.Capture(p.ProviderPaymentID, now, now)
	case domain.PaymentStatusFailed:
		err = p.Fail("provider declined payment", now)
	default:
		if terr := p.ValidateTransition(target); terr == nil {
			err = p.TransitionTo(target, now)
		} else {
			err = fmt.Errorf("cannot map provider status %q to a valid payment state", statusStr)
		}
	}
	if err == nil {
		err = s.record(ctx, p)
	}
	if err != nil {
		*p = before
		return err
	}
	if p.Status == domain.PaymentStatusFailed {
		observability.PaymentFailed(ctx, p.ID, p.OrderID, "provider_decline", "provider declined payment")
	}
	return nil
}

// record persists a payment's new state; a capture goes with its ledger entries.
func (s *PaymentService) record(ctx context.Context, p *domain.Payment) error {
	if p.Status != domain.PaymentStatusCaptured {
		return s.payments.Update(ctx, p)
	}
	entries, err := domain.CaptureEntries(p, unixNow())
	if err != nil {
		return err
	}
	return s.accounting.CommitCapture(ctx, p, entries)
}

// ensureCaptureEntries posts a captured payment's entries if any is missing (a payment captured by an earlier version
// whose posting failed); it never posts a line twice.
func (s *PaymentService) ensureCaptureEntries(ctx context.Context, p *domain.Payment) error {
	entries, err := domain.CaptureEntries(p, unixNow())
	if err != nil {
		return err
	}
	posted, err := s.accounting.EnsureEntries(ctx, entries)
	if posted > 0 {
		s.logger.Warn().Str("payment", p.ID).Int("entries", posted).Msg("missing capture entries posted")
	}
	return err
}

// recordFailure marks a payment failed after a definitive provider refusal; if that cannot be saved, the payment is
// left as stored and the error is logged.
func (s *PaymentService) recordFailure(ctx context.Context, p *domain.Payment, reason string) {
	before := *p
	err := p.Fail(reason, unixNow())
	if err == nil {
		err = s.payments.Update(ctx, p)
	}
	if err != nil {
		*p = before
		s.logger.Error().Err(err).Str("payment", p.ID).Msg("payment refusal not recorded")
		return
	}
	observability.PaymentFailed(ctx, p.ID, p.OrderID, "failed", reason)
}

// CancelPayment cancels a payment that is eligible (not captured).
func (s *PaymentService) cancelPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error) {
	p, err := s.payments.FindByIDForUpdate(ctx, orgID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load payment: %w", err)
	}

	if !p.CanCancel() {
		return nil, fmt.Errorf("payment %s cannot be cancelled from status %s", p.ID, p.Status)
	}

	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpCancel),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.CancelPayment(domain.WithProviderIdempotencyKey(ctx, keyOr(idempotencyKey, "cancel:"+p.ID)), p.ProviderPaymentID)
		if rerr != nil {
			return domain.ProviderResult{Success: false, ErrorMessage: rerr.Error()}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderPaymentID,
			Data:              map[string]interface{}{"status": resp.Status},
		}, nil
	})

	if execErr != nil || !result.Success {
		if execErr != nil {
			return nil, fmt.Errorf("provider cancel failed: %w", execErr)
		}
		return nil, fmt.Errorf("provider cancel failed: %s", result.ErrorMessage)
	}

	if err := p.Cancel(unixNow()); err != nil {
		return nil, fmt.Errorf("failed to cancel payment: %w", err)
	}
	if err := s.payments.Update(ctx, p); err != nil {
		return nil, fmt.Errorf("failed to persist cancelled payment: %w", err)
	}

	return p, nil
}

// CapturePayment captures a previously authorized payment.
func (s *PaymentService) capturePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string, amount *domain.Money) (*domain.Payment, error) {
	p, err := s.payments.FindByIDForUpdate(ctx, orgID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load payment: %w", err)
	}

	if p.Status != domain.PaymentStatusAuthorized {
		return nil, fmt.Errorf("payment %s must be AUTHORIZED to capture (status=%s)", p.ID, p.Status)
	}

	var captureAmount domain.Money
	if amount != nil {
		captureAmount = *amount
	} else {
		captureAmount = p.Amount
	}

	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpCapture),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.CapturePayment(domain.WithProviderIdempotencyKey(ctx, keyOr(idempotencyKey, "capture:"+p.ID)), p.ProviderPaymentID, &captureAmount)
		if rerr != nil {
			return domain.ProviderResult{Success: false, ErrorMessage: rerr.Error()}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderPaymentID,
			Data:              map[string]interface{}{"status": resp.Status},
		}, nil
	})
	if execErr != nil {
		return nil, fmt.Errorf("provider capture failed: %w", execErr)
	}
	if !result.Success {
		return nil, fmt.Errorf("provider capture failed: %s", result.ErrorMessage)
	}

	statusStr, _ := result.Data["status"].(string)
	if err := s.applyProviderResult(ctx, p, statusStr, result.ExternalReference); err != nil {
		// The provider may have captured: the webhook or a status read records it; nothing claims it is done now.
		return nil, fmt.Errorf("capture not recorded: %w", err)
	}
	if p.IsCaptured() {
		syncOrderWithPayment(ctx, s.orders, p, s.logger)
	}

	return p, nil
}

// RefundPayment refunds a captured payment (fully or partially).
//
// The refund is recorded as pending before the provider is asked, so an interruption never leaves money returned
// without a trace. Asking again with the same key resumes it (the provider receives the same idempotency key). A
// refund succeeds only together with the reduced payment and its ledger entries. An outcome that cannot be known (a
// timeout, an unknown status) stays pending until the provider's webhook or a person settles it.
func (s *PaymentService) refundPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string, amount *domain.Money) (*domain.Payment, *domain.Refund, error) {
	if idempotencyKey == "" {
		// Without a caller key, still make orchestrator retries of this one
		// request idempotent at the provider.
		idempotencyKey = domain.NewID()
	}
	p, err := s.payments.FindByIDForUpdate(ctx, orgID, paymentID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load payment: %w", err)
	}

	// The same key again: an interrupted request is resumed; anything else was already handled.
	existing, err := s.refunds.FindByIdempotencyKey(ctx, orgID, idempotencyKey)
	switch {
	case err == nil:
		if existing.PaymentID != p.ID || (amount != nil && (amount.AmountMinor != existing.Amount.AmountMinor || amount.Currency != existing.Amount.Currency)) {
			return p, nil, domain.ErrIdempotencyConflict
		}
		if existing.Status == domain.RefundStatusPending && existing.ProviderRefundID == "" {
			if !resendable(existing.CreatedAt) {
				return p, existing, fmt.Errorf("%w: the request is too old to send again safely; check it with the provider", ErrRefundOutcomeUnknown)
			}
			return s.submitRefund(ctx, orgID, actorID, p, existing)
		}
		return p, nil, domain.ErrRefundAlreadyExists
	case !errors.Is(err, domain.ErrRefundNotFound):
		return p, nil, fmt.Errorf("failed to check existing refund: %w", err)
	}

	if !p.CanRefund() {
		return nil, nil, fmt.Errorf("payment %s cannot be refunded from status %s", p.ID, p.Status)
	}

	var refundAmount domain.Money
	if amount != nil {
		refundAmount = *amount
	} else {
		refundAmount = p.RemainingRefundable()
	}
	// Refunds still pending count against what remains: two of them must not return more than was captured.
	reserved, err := s.pendingRefunds(ctx, p)
	if err != nil {
		return p, nil, err
	}
	available := p.RemainingRefundable().AmountMinor - reserved
	if refundAmount.AmountMinor <= 0 || refundAmount.AmountMinor > available {
		return nil, nil, fmt.Errorf("%w: refund amount %d exceeds remaining refundable %d", domain.ErrRefundExceedsRemaining,
			refundAmount.AmountMinor, available)
	}

	refund := domain.NewRefund(orgID, p.ID, refundAmount, idempotencyKey, unixNow())
	if err := refund.ValidateInvariants(); err != nil {
		return nil, nil, fmt.Errorf("invalid refund: %w", err)
	}
	if err := s.refunds.Create(ctx, refund); err != nil {
		return p, nil, fmt.Errorf("failed to record the refund request: %w", err)
	}
	return s.submitRefund(ctx, orgID, actorID, p, refund)
}

func (s *PaymentService) pendingRefunds(ctx context.Context, p *domain.Payment) (int64, error) {
	list, err := s.refunds.FindByPaymentID(ctx, p.TenantID, p.ID, 0, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to read pending refunds: %w", err)
	}
	var total int64
	for _, r := range list {
		if r.Status == domain.RefundStatusPending {
			total += r.Amount.AmountMinor
		}
	}
	return total, nil
}

// providerKeyWindow: providers forget idempotency keys after a while (Stripe keeps them at least 24 hours). A request
// older than this is never sent again on its own: it could be executed a second time.
const providerKeyWindow = 23 * 60 * 60

func resendable(createdAt int64) bool { return unixNow()-createdAt < providerKeyWindow }

// ErrRefundRefused: the provider certainly did not return the money (declined, or it offers no refund).
var ErrRefundRefused = errors.New("provider refund failed")

// ErrRefundOutcomeUnknown: the provider may or may not have returned the money. The refund stays pending; asking again
// with the same idempotency key, the provider's webhook, or a person settles it.
var ErrRefundOutcomeUnknown = errors.New("the refund's outcome is not known yet")

// submitRefund asks the provider for a pending refund and records the answer.
func (s *PaymentService) submitRefund(ctx context.Context, orgID, actorID string, p *domain.Payment, refund *domain.Refund) (*domain.Payment, *domain.Refund, error) {
	key := refund.IdempotencyKey
	amount := refund.Amount
	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpRefund),
		IdempotencyKey: key,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.RefundPayment(domain.WithProviderIdempotencyKey(ctx, "refund:"+key), p.ProviderPaymentID, &amount)
		if rerr != nil {
			return domain.ProviderResult{Success: false, ErrorCode: resp.ErrorCode, ErrorMessage: rerr.Error()}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderRefundID,
			Data:              map[string]interface{}{"status": resp.Status},
		}, nil
	})

	now := unixNow()
	if execErr != nil || result == nil || !result.Success {
		reason, code := "provider refund failed", domain.ProviderErrorCode("")
		if result != nil {
			reason, code = result.ErrorMessage, result.ErrorCode
		}
		if execErr != nil {
			reason = execErr.Error()
		}
		if !refusedForCertain(execErr, code) {
			s.logger.Error().Str("refund", refund.ID).Str("payment", p.ID).Msg("refund outcome unknown; left pending")
			return p, refund, fmt.Errorf("%w: %s", ErrRefundOutcomeUnknown, reason)
		}
		before := *refund
		if err := refund.Fail(reason, now); err != nil {
			return p, refund, err
		}
		if err := s.refunds.Update(ctx, refund); err != nil {
			*refund = before
			return p, refund, fmt.Errorf("refund refused by the provider but not recorded: %w", err)
		}
		return p, refund, fmt.Errorf("%w: %s", ErrRefundRefused, reason)
	}

	status, _ := result.Data["status"].(string)
	switch classifyRefundStatus(status) {
	case refundSucceeded:
		beforeRefund, beforePayment := *refund, *p
		err := s.settleRefund(ctx, refund, p, result.ExternalReference, now)
		if err != nil {
			*refund, *p = beforeRefund, beforePayment
			// Keep the provider's reference so its webhook finds the refund and completes it.
			s.saveReference(ctx, refund, result.ExternalReference, now)
			return p, refund, fmt.Errorf("refund accepted by the provider but not recorded yet: %w", err)
		}
		syncOrderWithPayment(ctx, s.orders, p, s.logger)
		return p, refund, nil
	case refundPending:
		// Asynchronous refund: it completes (or fails) via the provider webhook.
		if err := s.saveReference(ctx, refund, result.ExternalReference, now); err != nil {
			return p, refund, fmt.Errorf("failed to update refund: %w", err)
		}
		return p, refund, nil
	case refundFailed:
		before := *refund
		if err := refund.Fail("provider reported refund status: "+status, now); err != nil {
			return p, refund, err
		}
		if err := s.refunds.Update(ctx, refund); err != nil {
			*refund = before
			return p, refund, fmt.Errorf("failed to update refund: %w", err)
		}
		return p, refund, nil
	default:
		_ = s.saveReference(ctx, refund, result.ExternalReference, now)
		s.logger.Error().Str("refund", refund.ID).Str("status", status).Msg("unknown refund status; left pending")
		return p, refund, fmt.Errorf("%w: unknown provider status %q", ErrRefundOutcomeUnknown, status)
	}
}

// settleRefund records a succeeded refund with the reduced payment and the ledger entries, atomically.
func (s *PaymentService) settleRefund(ctx context.Context, refund *domain.Refund, p *domain.Payment, providerRefundID string, now int64) error {
	if err := refund.Succeed(providerRefundID, now, now); err != nil {
		return fmt.Errorf("failed to mark refund succeeded: %w", err)
	}
	if err := p.MarkPartiallyRefunded(refund.Amount, now); err != nil {
		return fmt.Errorf("failed to update payment for refund: %w", err)
	}
	entries, err := domain.RefundEntries(refund, now)
	if err != nil {
		return err
	}
	return s.accounting.CommitRefund(ctx, refund, p, entries)
}

func (s *PaymentService) saveReference(ctx context.Context, refund *domain.Refund, providerRefundID string, now int64) error {
	if providerRefundID == "" {
		return nil
	}
	before := *refund
	refund.MarkSubmitted(providerRefundID, now)
	if err := s.refunds.Update(ctx, refund); err != nil {
		*refund = before
		s.logger.Error().Err(err).Str("refund", refund.ID).Msg("provider refund reference not saved")
		return err
	}
	return nil
}

// refusedForCertain reports whether the provider certainly did not act. A timeout, an unavailable provider or an
// unclassified error leave the outcome unknown.
func refusedForCertain(err error, code domain.ProviderErrorCode) bool {
	if errors.Is(err, domain.ErrProviderDeclined) || errors.Is(err, domain.ErrPaymentOperationNotSupported) ||
		errors.Is(err, domain.ErrNoProvidersAvailable) {
		return true
	}
	switch code {
	case domain.ErrCodeInvalidRequest, domain.ErrCodeProviderRejected, domain.ErrCodeProviderAuthFailed,
		domain.ErrCodeUnsupportedCapability, domain.ErrCodeResourceNotFound:
		return true
	}
	return false
}

type refundOutcome int

const (
	refundUnknown refundOutcome = iota
	refundSucceeded
	refundPending
	refundFailed
)

// classifyRefundStatus reads a provider's refund status; anything not recognised is unknown, never success.
func classifyRefundStatus(status string) refundOutcome {
	switch strings.ToLower(status) {
	case "succeeded", "refunded":
		return refundSucceeded
	case "pending", "processing", "requires_action":
		return refundPending
	case "failed", "declined", "canceled", "cancelled":
		return refundFailed
	}
	return refundUnknown
}

// AuthorizePayment authorizes a payment that is in CREATED or PENDING status.
func (s *PaymentService) authorizePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error) {
	p, err := s.payments.FindByIDForUpdate(ctx, orgID, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load payment: %w", err)
	}

	if p.Status != domain.PaymentStatusCreated && p.Status != domain.PaymentStatusPending {
		return nil, fmt.Errorf("payment %s must be CREATED or PENDING to authorize (status=%s)", p.ID, p.Status)
	}

	if p.ProviderPaymentID != "" {
		// The provider already holds this payment: refresh its state instead
		// of creating a second provider payment.
		refreshed := s.resumePayment(ctx, orgID, actorID, p, idempotencyKey)
		return refreshed, nil
	}

	providerEntry, err := s.orchestrator.SelectProvider(ctx, orgID, domain.CapabilityPayment, nil)
	if err != nil {
		return nil, fmt.Errorf("no payment provider available: %w", err)
	}
	providerID := providerEntry.GetProvider().ID

	req := domain.PaymentProviderRequest{
		Amount:            p.Amount,
		PaymentMethodType: p.PaymentMethodType,
		PaymentMethodID:   p.PaymentMethodID,
		IdempotencyKey:    idempotencyKey,
		Metadata:          p.Metadata,
	}

	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpCreate),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		if ppa.ProviderID() != providerID {
			return domain.ProviderResult{}, fmt.Errorf("provider mismatch: expected %s, got %s", providerID, ppa.ProviderID())
		}
		resp, rerr := ppa.CreatePayment(domain.WithProviderIdempotencyKey(ctx, idempotencyKey), req)
		if rerr != nil {
			return domain.ProviderResult{
				Success:           false,
				ExternalReference: resp.ProviderPaymentID,
				ErrorCode:         resp.ErrorCode,
				ErrorMessage:      rerr.Error(),
			}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderPaymentID,
			Data: map[string]interface{}{
				"status":              resp.Status,
				"provider_payment_id": resp.ProviderPaymentID,
			},
		}, nil
	})

	if execErr != nil {
		s.logger.Warn().Err(execErr).Str("payment", p.ID).Msg("payment authorization failed")
		observability.PaymentFailed(ctx, p.ID, p.OrderID, "provider_error", execErr.Error())
		return p, nil
	}

	if !result.Success {
		s.recordFailure(ctx, p, result.ErrorMessage)
		return p, nil
	}

	statusStr, _ := result.Data["status"].(string)
	providerPaymentID := result.ExternalReference
	if providerPaymentID == "" {
		if id, ok := result.Data["provider_payment_id"].(string); ok {
			providerPaymentID = id
		}
	}

	if err := s.applyProviderResult(ctx, p, statusStr, providerPaymentID); err != nil {
		s.logger.Warn().Err(err).Str("payment", p.ID).Msg("failed to apply provider result for authorization")
		return p, nil
	}

	return p, nil
}

// GetPayment returns a payment by ID.
func (s *PaymentService) GetPayment(ctx context.Context, orgID, paymentID string) (*domain.Payment, error) {
	return s.payments.FindByID(ctx, orgID, paymentID)
}

// ListRefunds returns refunds for a payment.
func (s *PaymentService) ListRefunds(ctx context.Context, orgID, paymentID string, offset, limit int) ([]*domain.Refund, error) {
	p, err := s.payments.FindByID(ctx, orgID, paymentID)
	if err != nil {
		return nil, err
	}
	return s.refunds.FindByPaymentID(ctx, p.TenantID, p.ID, offset, limit)
}

// GetPaymentStatus syncs a payment with the provider's state, under the payment's lock.
func (s *PaymentService) GetPaymentStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (p *domain.Payment, err error) {
	lockErr := s.withPaymentLock(ctx, paymentID, func(ctx context.Context) error {
		p, err = s.syncStatus(ctx, orgID, actorID, paymentID, idempotencyKey)
		return err
	})
	if err == nil && lockErr != nil {
		err = lockErr
	}
	return p, err
}

func (s *PaymentService) syncStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error) {
	p, err := s.payments.FindByID(ctx, orgID, paymentID)
	if err != nil {
		return nil, err
	}
	if p.ProviderPaymentID == "" {
		return p, nil
	}

	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpGet),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.GetPayment(ctx, p.ProviderPaymentID)
		if rerr != nil {
			return domain.ProviderResult{Success: false, ErrorMessage: rerr.Error()}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderPaymentID,
			Data:              map[string]interface{}{"status": resp.Status},
		}, nil
	})

	if execErr != nil || !result.Success {
		return p, nil
	}

	// A payment confirmed by reading it back (the payer approved on their phone, the webhook is late or lost) settles
	// the order exactly as the webhook would.
	statusStr, _ := result.Data["status"].(string)
	if statusStr == "" {
		return p, nil
	}
	if err := s.applyProviderResult(ctx, p, statusStr, result.ExternalReference); err != nil {
		s.logger.Error().Err(err).Str("payment", p.ID).Msg("provider status not recorded")
		return p, nil
	}
	if p.IsCaptured() {
		syncOrderWithPayment(ctx, s.orders, p, s.logger)
	}
	return p, nil
}

func parseProviderStatus(s string) (domain.PaymentStatus, error) {
	switch s {
	case "created", "CREATED", "":
		return domain.PaymentStatusCreated, nil
	case "pending", "PENDING", "requires_action":
		return domain.PaymentStatusPending, nil
	case "authorized", "AUTHORIZED", "requires_capture":
		return domain.PaymentStatusAuthorized, nil
	case "captured", "CAPTURED":
		return domain.PaymentStatusCaptured, nil
	case "succeeded", "SUCCEEDED":
		return domain.PaymentStatusCaptured, nil
	case "failed", "FAILED", "declined", "DECLINED":
		return domain.PaymentStatusFailed, nil
	case "cancelled", "CANCELLED", "voided", "VOIDED":
		return domain.PaymentStatusCancelled, nil
	case "expired", "EXPIRED":
		return domain.PaymentStatusExpired, nil
	case "refunded", "REFUNDED":
		return domain.PaymentStatusRefunded, nil
	case "partially_refunded", "PARTIALLY_REFUNDED":
		return domain.PaymentStatusPartiallyRefunded, nil
	default:
		return "", fmt.Errorf("unrecognized provider status: %s", s)
	}
}

func unixNow() int64 {
	return time.Now().Unix()
}

// withPaymentLock runs fn while holding the per-payment lock, so the
// read-check-provider-persist sequence of a money-moving operation can never
// interleave with another one on the same payment (e.g. two refunds both
// passing the "remaining refundable" check and both refunding at the provider).
func (s *PaymentService) withPaymentLock(ctx context.Context, paymentID string, fn func(context.Context) error) error {
	if s.locker == nil {
		return fn(ctx)
	}
	return s.locker.WithLock(ctx, "payment:"+paymentID, fn)
}

// CancelPayment cancels a payment that is eligible (not captured).
func (s *PaymentService) CancelPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (p *domain.Payment, err error) {
	lockErr := s.withPaymentLock(ctx, paymentID, func(ctx context.Context) error {
		p, err = s.cancelPayment(ctx, orgID, actorID, paymentID, idempotencyKey)
		return err
	})
	if err == nil && lockErr != nil {
		err = lockErr
	}
	return p, err
}

// CapturePayment captures a previously authorized payment.
func (s *PaymentService) CapturePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string, amount *domain.Money) (p *domain.Payment, err error) {
	lockErr := s.withPaymentLock(ctx, paymentID, func(ctx context.Context) error {
		p, err = s.capturePayment(ctx, orgID, actorID, paymentID, idempotencyKey, amount)
		return err
	})
	if err == nil && lockErr != nil {
		err = lockErr
	}
	return p, err
}

// RefundPayment refunds a captured payment (fully or partially).
func (s *PaymentService) RefundPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string, amount *domain.Money) (p *domain.Payment, r *domain.Refund, err error) {
	lockErr := s.withPaymentLock(ctx, paymentID, func(ctx context.Context) error {
		p, r, err = s.refundPayment(ctx, orgID, actorID, paymentID, idempotencyKey, amount)
		return err
	})
	if err == nil && lockErr != nil {
		err = lockErr
	}
	return p, r, err
}

// AuthorizePayment authorizes a payment that is in CREATED or PENDING status.
func (s *PaymentService) AuthorizePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (p *domain.Payment, err error) {
	lockErr := s.withPaymentLock(ctx, paymentID, func(ctx context.Context) error {
		p, err = s.authorizePayment(ctx, orgID, actorID, paymentID, idempotencyKey)
		return err
	})
	if err == nil && lockErr != nil {
		err = lockErr
	}
	return p, err
}

func keyOr(key, def string) string {
	if key != "" {
		return key
	}
	return def
}
