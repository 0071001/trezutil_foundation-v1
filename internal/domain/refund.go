package domain

import (
	"fmt"
)

// RefundStatus represents the lifecycle state of a refund.
type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "pending"
	RefundStatusSucceeded RefundStatus = "succeeded"
	RefundStatusFailed    RefundStatus = "failed"
	RefundStatusCancelled RefundStatus = "cancelled"
)

// IsTerminal returns true if the refund status is terminal.
func (s RefundStatus) IsTerminal() bool {
	switch s {
	case RefundStatusSucceeded, RefundStatusFailed, RefundStatusCancelled:
		return true
	default:
		return false
	}
}

// IsTerminal returns true if the refund is in a terminal state.
func (r *Refund) IsTerminal() bool {
	return r.Status.IsTerminal()
}

// Refund represents a refund against a captured payment.
//
// Design rules:
//   - A refund is always tied to exactly one payment (PaymentID).
//   - The refund Amount and Currency are immutable after creation.
//   - total refunded amount across all refunds for a payment must not exceed
//     the captured amount — enforced at the Payment aggregate level.
//   - Historical payment amounts are never mutated to represent refunds.
type Refund struct {
	ID               string
	TenantID         string
	PaymentID        string
	Amount           Money
	ProviderRefundID string
	Status           RefundStatus
	IdempotencyKey   string
	RequestedAt      int64
	ProcessedAt      *int64
	FailedAt         *int64
	FailureReason    string
	CreatedAt        int64
	UpdatedAt        int64
	Version          int64
	Metadata         map[string]interface{}
}

// NewRefund creates a new Refund in PENDING status.
func NewRefund(
	tenantID, paymentID string,
	amount Money,
	idempotencyKey string,
	now int64,
) *Refund {
	return &Refund{
		ID:             "",
		TenantID:       tenantID,
		PaymentID:      paymentID,
		Amount:         amount,
		Status:         RefundStatusPending,
		IdempotencyKey: idempotencyKey,
		RequestedAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
		Version:        1,
		Metadata:       make(map[string]interface{}),
	}
}

// Succeed transitions the refund to SUCCEEDED.
func (r *Refund) Succeed(providerRefundID string, at int64, now int64) error {
	if err := r.ValidateTransition(RefundStatusSucceeded); err != nil {
		return err
	}
	r.ProviderRefundID = providerRefundID
	r.Status = RefundStatusSucceeded
	r.ProcessedAt = &at
	r.UpdatedAt = now
	r.Version++
	return nil
}

// Fail transitions the refund to FAILED.
// MarkSubmitted records the provider's reference for a refund that the
// provider is still processing asynchronously (status stays pending).
func (r *Refund) MarkSubmitted(providerRefundID string, now int64) {
	r.ProviderRefundID = providerRefundID
	r.UpdatedAt = now
	r.Version++
}

func (r *Refund) Fail(reason string, now int64) error {
	if err := r.ValidateTransition(RefundStatusFailed); err != nil {
		return err
	}
	r.Status = RefundStatusFailed
	r.FailedAt = &now
	r.FailureReason = reason
	r.UpdatedAt = now
	r.Version++
	return nil
}

// Cancel transitions the refund to CANCELLED.
func (r *Refund) Cancel(now int64) error {
	if err := r.ValidateTransition(RefundStatusCancelled); err != nil {
		return err
	}
	r.Status = RefundStatusCancelled
	r.UpdatedAt = now
	r.Version++
	return nil
}

// RefundValidTransitions defines the allowed status transitions for a Refund.
var RefundValidTransitions = map[RefundStatus][]RefundStatus{
	RefundStatusPending: {
		RefundStatusSucceeded,
		RefundStatusFailed,
		RefundStatusCancelled,
	},
}

// ValidateTransition validates a status transition without applying it.
func (r *Refund) ValidateTransition(target RefundStatus) error {
	if r.Status == target {
		return nil
	}

	if r.IsTerminal() {
		return fmt.Errorf("cannot transition from terminal status %s", r.Status)
	}

	valid, ok := RefundValidTransitions[r.Status]
	if !ok {
		return fmt.Errorf("no transitions defined from status %s", r.Status)
	}

	for _, t := range valid {
		if t == target {
			return nil
		}
	}

	return fmt.Errorf("invalid transition from %s to %s", r.Status, target)
}

// ValidateInvariants checks all domain invariants for a Refund.
func (r *Refund) ValidateInvariants() error {
	if r.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	if r.PaymentID == "" {
		return fmt.Errorf("payment_id is required")
	}
	if r.Amount.AmountMinor <= 0 {
		return fmt.Errorf("refund amount must be positive")
	}
	if r.Amount.Currency == "" {
		return fmt.Errorf("currency is required")
	}
	if r.IdempotencyKey == "" {
		return fmt.Errorf("idempotency_key is required")
	}
	if r.Version < 1 {
		return fmt.Errorf("version must be >= 1")
	}
	return nil
}
