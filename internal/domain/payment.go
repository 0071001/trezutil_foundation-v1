package domain

import (
	"fmt"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	PaymentStatusCreated           PaymentStatus = "created"
	PaymentStatusPending           PaymentStatus = "pending"
	PaymentStatusAuthorized        PaymentStatus = "authorized"
	PaymentStatusCaptured          PaymentStatus = "captured"
	PaymentStatusFailed            PaymentStatus = "failed"
	PaymentStatusCancelled         PaymentStatus = "cancelled"
	PaymentStatusExpired           PaymentStatus = "expired"
	PaymentStatusRefunded          PaymentStatus = "refunded"
	PaymentStatusPartiallyRefunded PaymentStatus = "partially_refunded"
)

// IsTerminal returns true if the status is a terminal state.
// Note: CAPTURED and PARTIALLY_REFUNDED are NOT terminal — they allow
// further refunds. Only FINAL states where no transitions are possible
// are terminal.
func (s PaymentStatus) IsTerminal() bool {
	switch s {
	case PaymentStatusFailed, PaymentStatusCancelled, PaymentStatusExpired,
		PaymentStatusRefunded:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the payment is in a terminal state.
func (p *Payment) IsTerminal() bool {
	return p.Status.IsTerminal()
}

// PaymentMethodType represents the type of payment method used.
type PaymentMethodType string

const (
	PaymentMethodMobileMoney  PaymentMethodType = "MOBILE_MONEY"
	PaymentMethodCard         PaymentMethodType = "CARD"
	PaymentMethodBankTransfer PaymentMethodType = "BANK_TRANSFER"
	PaymentMethodCash         PaymentMethodType = "CASH"
	PaymentMethodWallet       PaymentMethodType = "WALLET"
	PaymentMethodOther        PaymentMethodType = "OTHER"
)

// IsValid returns true if the payment method type is recognized.
func (m PaymentMethodType) IsValid() bool {
	switch m {
	case PaymentMethodMobileMoney, PaymentMethodCard, PaymentMethodBankTransfer,
		PaymentMethodCash, PaymentMethodWallet, PaymentMethodOther:
		return true
	default:
		return false
	}
}

// PaymentProviderOperation represents an operation a payment provider can perform.
type PaymentProviderOperation string

const (
	PaymentProviderOpCreate  PaymentProviderOperation = "create"
	PaymentProviderOpGet     PaymentProviderOperation = "get"
	PaymentProviderOpCancel  PaymentProviderOperation = "cancel"
	PaymentProviderOpCapture PaymentProviderOperation = "capture"
	PaymentProviderOpRefund  PaymentProviderOperation = "refund"
)

// Payment represents an attempt to move money through an external payment provider.
//
// The aggregate enforces:
//   - Explicit state transitions (see validTransitions)
//   - Immutable financial fields (Amount, Currency, OrderID, ProviderID) after
//     processing has started (i.e. once status leaves CREATED).
//   - No sensitive credentials are stored — only tokenized references.
type Payment struct {
	// ClientAction carries transient, payer-facing data returned by the
	// provider for completing the payment in the browser (e.g. a Stripe
	// client_secret). It is never persisted.
	ClientAction      map[string]string
	ID                string
	TenantID          string
	OrderID           string
	Amount            Money
	PaymentMethodType PaymentMethodType
	PaymentMethodID   string
	ProviderID        string
	ProviderPaymentID string
	Status            PaymentStatus
	IdempotencyKey    string
	AuthorizedAt      *int64
	CapturedAt        *int64
	CancelledAt       *int64
	FailedAt          *int64
	FailureReason     string
	RefundedAmount    Money
	CreatedAt         int64
	UpdatedAt         int64
	Version           int64
	Metadata          map[string]interface{}
}

// NewPayment creates a new Payment in CREATED status.
func NewPayment(
	tenantID, orderID string,
	amount Money,
	methodType PaymentMethodType,
	methodID, providerID, idempotencyKey string,
	now int64,
) *Payment {
	return &Payment{
		ID:                "",
		TenantID:          tenantID,
		OrderID:           orderID,
		Amount:            amount,
		PaymentMethodType: methodType,
		PaymentMethodID:   methodID,
		ProviderID:        providerID,
		ProviderPaymentID: "",
		Status:            PaymentStatusCreated,
		IdempotencyKey:    idempotencyKey,
		RefundedAmount:    Money{Currency: amount.Currency},
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
		Metadata:          make(map[string]interface{}),
	}
}

// ValidTransitions defines the allowed status transitions for a Payment.
var PaymentValidTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentStatusCreated: {
		PaymentStatusPending,
		PaymentStatusAuthorized,
		PaymentStatusCaptured,
		PaymentStatusFailed,
		PaymentStatusCancelled,
	},
	PaymentStatusPending: {
		PaymentStatusAuthorized,
		PaymentStatusCaptured,
		PaymentStatusFailed,
		PaymentStatusCancelled,
		PaymentStatusExpired,
	},
	PaymentStatusAuthorized: {
		PaymentStatusCaptured,
		PaymentStatusCancelled,
		PaymentStatusFailed,
		PaymentStatusExpired,
	},
	PaymentStatusCaptured: {
		PaymentStatusRefunded,
		PaymentStatusPartiallyRefunded,
		PaymentStatusFailed,
	},
	PaymentStatusPartiallyRefunded: {
		PaymentStatusRefunded,
		PaymentStatusFailed,
	},
}

// ValidateTransition validates a status transition without applying it.
func (p *Payment) ValidateTransition(target PaymentStatus) error {
	if p.Status == target {
		return nil
	}

	if p.IsTerminal() {
		return fmt.Errorf("cannot transition from terminal status %s", p.Status)
	}

	valid, ok := PaymentValidTransitions[p.Status]
	if !ok {
		return fmt.Errorf("no transitions defined from status %s", p.Status)
	}

	for _, t := range valid {
		if t == target {
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %s to %s", p.Status, target)
}

// TransitionTo transitions the payment to a new status.
func (p *Payment) TransitionTo(target PaymentStatus, now int64) error {
	if err := p.ValidateTransition(target); err != nil {
		return err
	}
	p.Status = target
	p.UpdatedAt = now
	p.Version++
	return nil
}

// Authorize transitions the payment to AUTHORIZED (from PENDING or CREATED).
func (p *Payment) Authorize(providerPaymentID string, at int64, now int64) error {
	if err := p.ValidateTransition(PaymentStatusAuthorized); err != nil {
		return err
	}
	p.ProviderPaymentID = providerPaymentID
	p.Status = PaymentStatusAuthorized
	p.AuthorizedAt = &at
	p.UpdatedAt = now
	p.Version++
	return nil
}

// Capture transitions the payment to CAPTURED (from AUTHORIZED, PENDING, or CREATED).
func (p *Payment) Capture(providerPaymentID string, at int64, now int64) error {
	if err := p.ValidateTransition(PaymentStatusCaptured); err != nil {
		return err
	}
	if providerPaymentID != "" {
		p.ProviderPaymentID = providerPaymentID
	}
	p.Status = PaymentStatusCaptured
	p.CapturedAt = &at
	p.UpdatedAt = now
	p.Version++
	return nil
}

// Fail transitions the payment to FAILED.
func (p *Payment) Fail(reason string, now int64) error {
	if err := p.ValidateTransition(PaymentStatusFailed); err != nil {
		return err
	}
	p.Status = PaymentStatusFailed
	p.FailedAt = &now
	p.FailureReason = reason
	p.UpdatedAt = now
	p.Version++
	return nil
}

// Cancel transitions the payment to CANCELLED.
func (p *Payment) Cancel(now int64) error {
	if err := p.ValidateTransition(PaymentStatusCancelled); err != nil {
		return err
	}
	p.Status = PaymentStatusCancelled
	p.CancelledAt = &now
	p.UpdatedAt = now
	p.Version++
	return nil
}

// Expire transitions the payment to EXPIRED.
func (p *Payment) Expire(now int64) error {
	if err := p.ValidateTransition(PaymentStatusExpired); err != nil {
		return err
	}
	p.Status = PaymentStatusExpired
	p.UpdatedAt = now
	p.Version++
	return nil
}

// MarkPartiallyRefunded records a partial refund.
// The refund amount is accumulated; status moves to PARTIALLY_REFUNDED or REFUNDED.
func (p *Payment) MarkPartiallyRefunded(amount Money, now int64) error {
	if p.Status != PaymentStatusCaptured && p.Status != PaymentStatusPartiallyRefunded {
		return fmt.Errorf("cannot refund payment from status %s: only captured or partially_refunded payments can be refunded", p.Status)
	}
	if err := p.validateRefundAmount(amount); err != nil {
		return err
	}

	totalRefunded := p.RefundedAmount.AmountMinor + amount.AmountMinor
	if totalRefunded > p.Amount.AmountMinor {
		return fmt.Errorf("total refunded (%d) exceeds captured amount (%d)", totalRefunded, p.Amount.AmountMinor)
	}

	if totalRefunded == p.Amount.AmountMinor {
		p.Status = PaymentStatusRefunded
	} else {
		if err := p.ValidateTransition(PaymentStatusPartiallyRefunded); err != nil {
			return err
		}
		p.Status = PaymentStatusPartiallyRefunded
	}

	p.RefundedAmount = Money{
		AmountMinor: totalRefunded,
		Currency:    p.Amount.Currency,
	}
	p.UpdatedAt = now
	p.Version++
	return nil
}

// validateRefundAmount checks the refund amount is valid for this payment.
func (p *Payment) validateRefundAmount(amount Money) error {
	if amount.AmountMinor <= 0 {
		return fmt.Errorf("refund amount must be positive")
	}
	if amount.Currency != p.Amount.Currency {
		return fmt.Errorf("refund currency %s does not match payment currency %s", amount.Currency, p.Amount.Currency)
	}
	return nil
}

// RemainingRefundable returns the amount still available to refund.
func (p *Payment) RemainingRefundable() Money {
	return Money{
		AmountMinor: p.Amount.AmountMinor - p.RefundedAmount.AmountMinor,
		Currency:    p.Amount.Currency,
	}
}

// CanRefund returns true if the payment is in a state that allows refunds.
func (p *Payment) CanRefund() bool {
	return p.Status == PaymentStatusCaptured || p.Status == PaymentStatusPartiallyRefunded
}

// CanCancel returns true if the payment can be cancelled (voided before capture).
func (p *Payment) CanCancel() bool {
	switch p.Status {
	case PaymentStatusCreated, PaymentStatusPending, PaymentStatusAuthorized:
		return true
	default:
		return false
	}
}

// IsCaptured returns true if the payment has been captured.
func (p *Payment) IsCaptured() bool {
	return p.Status == PaymentStatusCaptured || p.Status == PaymentStatusRefunded ||
		p.Status == PaymentStatusPartiallyRefunded
}

// IsProcessingOrLater returns true if payment has moved past CREATED/PENDING.
func (p *Payment) IsProcessingOrLater() bool {
	return p.Status != PaymentStatusCreated && p.Status != PaymentStatusPending
}

// ValidateInvariants checks all domain invariants for a Payment.
func (p *Payment) ValidateInvariants() error {
	if p.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if p.OrderID == "" {
		return fmt.Errorf("order_id is required")
	}
	if p.Amount.AmountMinor <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if p.Amount.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if !p.PaymentMethodType.IsValid() {
		return fmt.Errorf("invalid payment method type: %s", p.PaymentMethodType)
	}
	if p.ProviderID == "" {
		return fmt.Errorf("provider_id is required")
	}
	if p.IdempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}
	if !p.IsTerminal() && !isValidNonTerminalStatus(p.Status) {
		return fmt.Errorf("invalid payment status: %s", p.Status)
	}
	if p.RefundedAmount.Currency != "" && p.RefundedAmount.Currency != p.Amount.Currency {
		return fmt.Errorf("refunded_amount currency mismatch: %s != %s", p.RefundedAmount.Currency, p.Amount.Currency)
	}
	if p.Version < 1 {
		return fmt.Errorf("version must be >= 1")
	}
	return nil
}

// isValidNonTerminalStatus checks against the known set of statuses.
func isValidNonTerminalStatus(s PaymentStatus) bool {
	switch s {
	case PaymentStatusCreated, PaymentStatusPending, PaymentStatusAuthorized,
		PaymentStatusCaptured, PaymentStatusFailed, PaymentStatusCancelled,
		PaymentStatusExpired, PaymentStatusRefunded, PaymentStatusPartiallyRefunded:
		return true
	default:
		return false
	}
}
