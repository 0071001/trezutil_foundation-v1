package payment

import (
	"context"
	"errors"
	"fmt"
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
	ledger       domain.LedgerRepository
	orders       domain.OrderRepository
	orchestrator *provider.ProviderOrchestrator
	logger       zerolog.Logger
	locker       Locker
}

type PaymentServiceConfig struct {
	Payments     domain.PaymentRepository
	Refunds      domain.RefundRepository
	Ledger       domain.LedgerRepository
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
		ledger:       config.Ledger,
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
				if target, perr := parseProviderStatus(statusStr); perr == nil && target != p.Status {
					if aerr := s.applyProviderResult(ctx, p, statusStr, result.ExternalReference); aerr == nil && p.IsCaptured() {
						if lerr := s.postLedgerForCapture(ctx, p); lerr != nil {
							s.logger.Warn().Err(lerr).Str("payment", p.ID).Msg("ledger posting for capture failed")
						}
						syncOrderWithPayment(ctx, s.orders, p, s.logger)
					}
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
			_ = s.failPayment(ctx, p, execErr.Error())
			return p
		}
		s.logger.Warn().Err(execErr).Str("payment", p.ID).Msg("payment initiation failed; payment left in CREATED for retry")
		return p
	}

	if !providerResult.Success {
		_ = s.failPayment(ctx, p, providerResult.ErrorMessage)
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
		s.logger.Warn().Err(err).Str("payment", p.ID).Msg("failed to apply provider result")
		return p
	}

	if p.IsCaptured() {
		if lerr := s.postLedgerForCapture(ctx, p); lerr != nil {
			s.logger.Warn().Err(lerr).Str("payment", p.ID).Msg("ledger posting for capture failed")
		}
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
	for _, k := range []string{"client_secret", "publishable_key", "provider_code", "checkout_url", "instructions"} {
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

// applyProviderResult maps a provider status string to a domain transition.
func (s *PaymentService) applyProviderResult(ctx context.Context, p *domain.Payment, statusStr, providerPaymentID string) error {
	target, err := parseProviderStatus(statusStr)
	if err != nil {
		return fmt.Errorf("unknown provider status %q: %w", statusStr, err)
	}

	now := unixNow()
	if providerPaymentID != "" {
		p.ProviderPaymentID = providerPaymentID
	}

	switch target {
	case domain.PaymentStatusAuthorized:
		err = p.Authorize(providerPaymentID, now, now)
	case domain.PaymentStatusCaptured:
		err = p.Capture(providerPaymentID, now, now)
	case domain.PaymentStatusFailed:
		err = p.Fail("provider declined payment", now)
		if err == nil {
			observability.PaymentFailed(ctx, p.ID, p.OrderID, "provider_decline", "provider declined payment")
		}
	default:
		if terr := p.ValidateTransition(target); terr == nil {
			err = p.TransitionTo(target, now)
		} else {
			err = fmt.Errorf("cannot map provider status %q to a valid payment state", statusStr)
		}
	}

	if err != nil {
		return err
	}
	return s.payments.Update(ctx, p)
}

// failPayment transitions the payment to FAILED and persists.
func (s *PaymentService) failPayment(ctx context.Context, p *domain.Payment, reason string) error {
	now := unixNow()
	if err := p.Fail(reason, now); err != nil {
		return err
	}
	observability.PaymentFailed(ctx, p.ID, p.OrderID, "failed", reason)
	return s.payments.Update(ctx, p)
}

// postLedgerForCapture posts DEBIT and CREDIT entries for a captured payment.
// Idempotency key is unique per payment, so retries are no-ops at the DB level.
func (s *PaymentService) postLedgerForCapture(ctx context.Context, p *domain.Payment) error {
	if !p.IsCaptured() {
		return nil
	}
	now := unixNow()
	entries := []*domain.LedgerEntry{
		domain.NewLedgerEntry(
			p.TenantID, &p.ID, nil,
			domain.LedgerAccountCash, domain.LedgerEntryTypeDebit,
			p.Amount, "ledger_capture_"+p.ID, now,
		),
		domain.NewLedgerEntry(
			p.TenantID, &p.ID, nil,
			domain.LedgerAccountRevenue, domain.LedgerEntryTypeCredit,
			p.Amount, "ledger_capture_rev_"+p.ID, now,
		),
	}
	for _, e := range entries {
		if err := e.Post(now); err != nil {
			return fmt.Errorf("ledger entry post failed: %w", err)
		}
		if err := e.ValidateInvariants(); err != nil {
			return fmt.Errorf("ledger invariant violated: %w", err)
		}
		if _, err := s.ledger.Post(ctx, e); err != nil {
			return fmt.Errorf("ledger persist failed: %w", err)
		}
	}
	return nil
}

// postLedgerForRefund posts a DEBIT entry against the refund account.
func (s *PaymentService) postLedgerForRefund(ctx context.Context, r *domain.Refund) error {
	now := unixNow()
	e := domain.NewLedgerEntry(
		r.TenantID, nil, &r.ID,
		domain.LedgerAccountRefund, domain.LedgerEntryTypeDebit,
		r.Amount, "ledger_refund_"+r.ID, now,
	)
	if err := e.Post(now); err != nil {
		return fmt.Errorf("ledger entry post failed: %w", err)
	}
	if err := e.ValidateInvariants(); err != nil {
		return fmt.Errorf("ledger invariant violated: %w", err)
	}
	if _, err := s.ledger.Post(ctx, e); err != nil {
		return fmt.Errorf("ledger persist failed: %w", err)
	}
	return nil
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
		return nil, err
	}

	if p.IsCaptured() {
		if lerr := s.postLedgerForCapture(ctx, p); lerr != nil {
			s.logger.Warn().Err(lerr).Str("payment", p.ID).Msg("ledger posting for capture failed")
		}
		syncOrderWithPayment(ctx, s.orders, p, s.logger)
	}

	return p, nil
}

// RefundPayment refunds a captured payment (fully or partially).
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

	// Check idempotency before business logic — allows retry of a previously
	// completed refund even if the payment status has changed since.
	if _, err := s.refunds.FindByIdempotencyKey(ctx, orgID, idempotencyKey); err == nil {
		return p, nil, domain.ErrRefundAlreadyExists
	}

	if !p.CanRefund() {
		return nil, nil, fmt.Errorf("payment %s cannot be refunded from status %s", p.ID, p.Status)
	}

	var refundAmount domain.Money
	if amount != nil {
		refundAmount = *amount
	} else {
		refundAmount = p.Amount
	}
	if refundAmount.AmountMinor > p.RemainingRefundable().AmountMinor {
		return nil, nil, fmt.Errorf("%w: refund amount %d exceeds remaining refundable %d", domain.ErrRefundExceedsRemaining,
			refundAmount.AmountMinor, p.RemainingRefundable().AmountMinor)
	}

	now := unixNow()
	refund := domain.NewRefund(orgID, p.ID, refundAmount, idempotencyKey, now)
	if err := refund.ValidateInvariants(); err != nil {
		return nil, nil, fmt.Errorf("invalid refund: %w", err)
	}

	result, execErr := s.orchestrator.ExecuteCapability(ctx, orgID, actorID, provider.ExecuteOptions{
		Capability:     domain.CapabilityPayment,
		Operation:      string(domain.PaymentProviderOpRefund),
		IdempotencyKey: idempotencyKey,
	}, func(ctx context.Context, execCtx *domain.ProviderExecutionContext, adapter domain.ProviderAdapter) (domain.ProviderResult, error) {
		ppa, err := paymentProviderAdapter(adapter)
		if err != nil {
			return domain.ProviderResult{}, err
		}
		resp, rerr := ppa.RefundPayment(domain.WithProviderIdempotencyKey(ctx, "refund:"+idempotencyKey), p.ProviderPaymentID, &refundAmount)
		if rerr != nil {
			return domain.ProviderResult{Success: false, ErrorMessage: rerr.Error()}, rerr
		}
		return domain.ProviderResult{
			Success:           true,
			ExternalReference: resp.ProviderRefundID,
			Data:              map[string]interface{}{"status": resp.Status},
		}, nil
	})

	if execErr != nil || !result.Success {
		var failReason string
		if execErr != nil {
			failReason = execErr.Error()
		} else {
			failReason = result.ErrorMessage
		}
		_ = refund.Fail(failReason, now)
		_ = s.refunds.Create(ctx, refund)

		if execErr != nil {
			return p, refund, fmt.Errorf("provider refund failed: %w", execErr)
		}
		return p, refund, fmt.Errorf("provider refund failed: %s", result.ErrorMessage)
	}

	if err := s.refunds.Create(ctx, refund); err != nil {
		return p, refund, fmt.Errorf("failed to create refund: %w", err)
	}

	providerRefundStatus, _ := result.Data["status"].(string)
	if err := finalizeRefund(ctx, refund, p, result.ExternalReference, providerRefundStatus, now); err != nil {
		return p, refund, err
	}
	if err := s.refunds.Update(ctx, refund); err != nil {
		return p, refund, fmt.Errorf("failed to update refund: %w", err)
	}
	if refund.Status != domain.RefundStatusSucceeded {
		// Pending (asynchronous) or failed refunds leave the payment, ledger
		// and order untouched; the provider webhook completes pending ones.
		return p, refund, nil
	}
	if err := s.payments.Update(ctx, p); err != nil {
		return p, refund, fmt.Errorf("failed to update payment: %w", err)
	}

	if lerr := s.postLedgerForRefund(ctx, refund); lerr != nil {
		s.logger.Warn().Err(lerr).Str("refund", refund.ID).Msg("ledger posting for refund failed")
	}
	syncOrderWithPayment(ctx, s.orders, p, s.logger)

	return p, refund, nil
}

// finalizeRefund marks the refund as succeeded or failed and updates the
// payment's refund tracking via MarkPartiallyRefunded.
func finalizeRefund(ctx context.Context, refund *domain.Refund, payment *domain.Payment,
	providerRefundID, statusStr string, now int64,
) error {
	switch statusStr {
	case "", "succeeded":
	case "pending":
		// Asynchronous refund: it completes (or fails) via the provider
		// webhook, which then updates the payment, ledger and order.
		refund.MarkSubmitted(providerRefundID, now)
		return nil
	default:
		if err := refund.Fail("provider reported refund status: "+statusStr, now); err != nil {
			return err
		}
		return nil
	}

	if err := refund.Succeed(providerRefundID, now, now); err != nil {
		return fmt.Errorf("failed to mark refund succeeded: %w", err)
	}
	if err := payment.MarkPartiallyRefunded(refund.Amount, now); err != nil {
		return fmt.Errorf("failed to update payment for refund: %w", err)
	}
	return nil
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

	now := unixNow()
	if execErr != nil {
		s.logger.Warn().Err(execErr).Str("payment", p.ID).Msg("payment authorization failed")
		observability.PaymentFailed(ctx, p.ID, p.OrderID, "provider_error", execErr.Error())
		return p, nil
	}

	if !result.Success {
		_ = p.Fail(result.ErrorMessage, now)
		observability.PaymentFailed(ctx, p.ID, p.OrderID, "provider_decline", result.ErrorMessage)
		_ = s.payments.Update(ctx, p)
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

// GetPaymentStatus optionally syncs status from the provider.
func (s *PaymentService) GetPaymentStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error) {
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

	statusStr, _ := result.Data["status"].(string)
	before := p.Status
	if err := s.applyProviderResult(ctx, p, statusStr, result.ExternalReference); err != nil {
		s.logger.Debug().Err(err).Str("payment", p.ID).Msg("provider status sync failed")
		return p, nil
	}
	// A payment confirmed by reading it back (the payer approved on their phone, the webhook is late or lost) settles
	// the order exactly as the webhook would.
	if p.Status != before {
		if p.Status == domain.PaymentStatusCaptured {
			if lerr := s.postLedgerForCapture(ctx, p); lerr != nil {
				s.logger.Warn().Err(lerr).Str("payment", p.ID).Msg("ledger posting for polled capture failed")
			}
		}
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
