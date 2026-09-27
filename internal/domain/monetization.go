package domain

import (
	"context"
	"fmt"
	"time"
)

// SubscriptionStatus represents the lifecycle status of a tenant subscription.
type SubscriptionStatus string

const (
	SubscriptionStatusActive   SubscriptionStatus = "active"
	SubscriptionStatusPending  SubscriptionStatus = "pending"
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
	SubscriptionStatusPastDue  SubscriptionStatus = "past_due"
)

// BillingPeriodStatus represents the open/closed state of a billing period.
type BillingPeriodStatus string

const (
	BillingPeriodStatusOpen   BillingPeriodStatus = "open"
	BillingPeriodStatusClosed BillingPeriodStatus = "closed"
)

// CreditGrantSource describes where a credit grant originated.
type CreditGrantSource string

const (
	CreditGrantSourceAllocation CreditGrantSource = "allocation" // subscription free-tier / quota reset
	CreditGrantSourcePurchase   CreditGrantSource = "purchase"   // paid via Payment
	CreditGrantSourceManual     CreditGrantSource = "manual"     // admin adjustment (never AI)
)

// CreditPurchaseStatus represents the lifecycle of a credit purchase.
type CreditPurchaseStatus string

const (
	CreditPurchaseStatusPending  CreditPurchaseStatus = "pending"
	CreditPurchaseStatusCaptured CreditPurchaseStatus = "captured"
	CreditPurchaseStatusFailed   CreditPurchaseStatus = "failed"
	CreditPurchaseStatusCanceled CreditPurchaseStatus = "canceled"
)

var (
	ErrMonetizationPlanNotFound      = fmt.Errorf("monetization plan not found")
	ErrSubscriptionNotFound          = fmt.Errorf("subscription not found")
	ErrSubscriptionNotActive         = fmt.Errorf("subscription is not active")
	ErrBillingPeriodNotFound         = fmt.Errorf("billing period not found")
	ErrBillingPeriodAlreadyClosed    = fmt.Errorf("billing period is already closed")
	ErrCreditGrantDuplicate          = fmt.Errorf("credit grant already exists for idempotency key")
	ErrCreditPurchaseNotFound        = fmt.Errorf("credit purchase not found")
	ErrCreditPurchaseNotPending      = fmt.Errorf("credit purchase is not in a pending state")
	ErrCreditPurchaseAlreadyLinked   = fmt.Errorf("credit purchase already captured")
	ErrCreditPurchasePaymentMismatch = fmt.Errorf("credit purchase payment id mismatch")
)

// NewErrInsufficientCreditsForGrant returns an error for attempted negative-balance grants.
func NewErrInsufficientCreditsForGrant(accountID string, available, requested int64) error {
	return fmt.Errorf("credit grant would overdraw account %s (available=%d requested=%d)", accountID, available, requested)
}

// Subscription represents a tenant-scoped entitlement to a Plan.
// A subscription owns a sequence of BillingPeriods; exactly one period is open at a time.
type Subscription struct {
	ID                string             `json:"id"`
	TenantID          string             `json:"tenant_id"`
	PlanID            string             `json:"plan_id"`
	OwnerType         string             `json:"owner_type"`
	OwnerID           string             `json:"owner_id"`
	Status            SubscriptionStatus `json:"status"`
	CurrentPeriodID   string             `json:"current_period_id"`
	CancelAtPeriodEnd bool               `json:"cancel_at_period_end"`
	StartedAt         int64              `json:"started_at"`
	CreatedAt         int64              `json:"created_at"`
	UpdatedAt         int64              `json:"updated_at"`
}

// NewSubscription creates an active subscription for the given tenant/owner/plan.
func NewSubscription(tenantID, planID, ownerType, ownerID string, periodID string, now int64) *Subscription {
	sub := &Subscription{
		ID:                "",
		TenantID:          tenantID,
		PlanID:            planID,
		OwnerType:         ownerType,
		OwnerID:           ownerID,
		Status:            SubscriptionStatusActive,
		CurrentPeriodID:   periodID,
		CancelAtPeriodEnd: false,
		StartedAt:         now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return sub
}

// Cancel marks the subscription to cancel at the end of the current period.
func (s *Subscription) Cancel(now int64) {
	if s.Status != SubscriptionStatusActive {
		return
	}
	s.CancelAtPeriodEnd = true
	s.UpdatedAt = now
}

// Reactivate re-activates a canceled subscription.
func (s *Subscription) Reactivate(now int64) {
	if s.Status == SubscriptionStatusCanceled {
		s.Status = SubscriptionStatusActive
		s.CancelAtPeriodEnd = false
		s.UpdatedAt = now
	}
}

// IsActive returns true if the subscription can accrue entitlements.
func (s Subscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive
}

// CurrentPeriodID returns the active billing period identifier.
func (s Subscription) CurrentPeriod() string {
	return s.CurrentPeriodID
}

// BillingPeriod represents a single billing window owned by a Subscription.
// The period_id doubles as a credit-account period partition key.
type BillingPeriod struct {
	ID             string              `json:"id"` // e.g. "2026-01"
	TenantID       string              `json:"tenant_id"`
	SubscriptionID string              `json:"subscription_id"`
	PlanID         string              `json:"plan_id"`
	StartsAt       int64               `json:"starts_at"`
	EndsAt         int64               `json:"ends_at"`
	Status         BillingPeriodStatus `json:"status"`
	CreatedAt      int64               `json:"created_at"`
}

// NewBillingPeriod creates an open billing period.
func NewBillingPeriod(id, tenantID, subscriptionID, planID string, startsAt, endsAt, now int64) *BillingPeriod {
	return &BillingPeriod{
		ID:             id,
		TenantID:       tenantID,
		SubscriptionID: subscriptionID,
		PlanID:         planID,
		StartsAt:       startsAt,
		EndsAt:         endsAt,
		Status:         BillingPeriodStatusOpen,
		CreatedAt:      now,
	}
}

// IsOpen returns true if the period is open and currently active.
func (p BillingPeriod) IsOpen() bool {
	return p.Status == BillingPeriodStatusOpen
}

// Contains returns true if now falls within the period window.
func (p BillingPeriod) Contains(now int64) bool {
	return now >= p.StartsAt && now <= p.EndsAt
}

// Close closes the billing period (idempotent).
func (p *BillingPeriod) Close(now int64) error {
	if p.Status == BillingPeriodStatusClosed {
		return ErrBillingPeriodAlreadyClosed
	}
	p.Status = BillingPeriodStatusClosed
	return nil
}

// CreditGrant represents an idempotent credit mutation on a CreditAccount.
// Idempotency is enforced by the DB unique constraint on IdempotencyKey.
type CreditGrant struct {
	ID             string            `json:"id"`
	AccountID      string            `json:"account_id"`
	IdempotencyKey string            `json:"idempotency_key"`
	Source         CreditGrantSource `json:"source"`
	Amount         int64             `json:"amount"`
	TenantID       string            `json:"tenant_id"`
	PeriodID       string            `json:"period_id"`
	PaymentID      *string           `json:"payment_id,omitempty"` // set only for purchase grants
	CreatedAt      int64             `json:"created_at"`
}

// NewCreditGrant creates a credit grant. Amount must be positive.
func NewCreditGrant(accountID, idempotencyKey string, source CreditGrantSource, amount int64, tenantID, periodID string, paymentID *string, now int64) (*CreditGrant, error) {
	if amount <= 0 {
		return nil, NewErrInsufficientCreditsForGrant(accountID, 0, amount)
	}
	if accountID == "" {
		return nil, fmt.Errorf("account_id is required")
	}
	if idempotencyKey == "" {
		return nil, fmt.Errorf("idempotency_key is required")
	}
	return &CreditGrant{
		AccountID:      accountID,
		IdempotencyKey: idempotencyKey,
		Source:         source,
		Amount:         amount,
		TenantID:       tenantID,
		PeriodID:       periodID,
		PaymentID:      paymentID,
		CreatedAt:      now,
	}, nil
}

// CreditPurchase represents a request to purchase credits, gated on a Payment.
// Credits are only granted on the payment Captured transition (idempotent).
type CreditPurchase struct {
	ID            string               `json:"id"`
	TenantID      string               `json:"tenant_id"`
	OwnerType     string               `json:"owner_type"`
	OwnerID       string               `json:"owner_id"`
	PlanID        string               `json:"plan_id"` // credit-pack plan
	AmountCredits int64                `json:"amount_credits"`
	PaymentID     string               `json:"payment_id"`
	Status        CreditPurchaseStatus `json:"status"`
	CreatedAt     int64                `json:"created_at"`
	UpdatedAt     int64                `json:"updated_at"`
}

// NewCreditPurchase creates a pending credit purchase backed by a Payment.
func NewCreditPurchase(tenantID, ownerType, ownerID, planID string, amountCredits int64, paymentID string, now int64) (*CreditPurchase, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant_id is required")
	}
	if ownerID == "" {
		return nil, fmt.Errorf("owner_id is required")
	}
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id is required")
	}
	if amountCredits <= 0 {
		return nil, fmt.Errorf("amount_credits must be positive")
	}
	return &CreditPurchase{
		TenantID:      tenantID,
		OwnerType:     ownerType,
		OwnerID:       ownerID,
		PlanID:        planID,
		AmountCredits: amountCredits,
		PaymentID:     paymentID,
		Status:        CreditPurchaseStatusPending,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

// MarkCaptured transitions a pending purchase to captured, returning the grant key + amount.
// Idempotent: a second call with the same paymentID is a no-op.
func (p *CreditPurchase) MarkCaptured(paymentID string, now int64) (*CreditGrantSpec, error) {
	if p.Status == CreditPurchaseStatusCaptured {
		return nil, ErrCreditPurchaseAlreadyLinked
	}
	if p.Status != CreditPurchaseStatusPending {
		return nil, ErrCreditPurchaseNotPending
	}
	if p.PaymentID != paymentID {
		return nil, ErrCreditPurchasePaymentMismatch
	}
	p.Status = CreditPurchaseStatusCaptured
	p.UpdatedAt = now
	return &CreditGrantSpec{
		Amount:         p.AmountCredits,
		IdempotencyKey: "purchase:" + p.PaymentID,
	}, nil
}

// MarkFailed transitions a pending purchase to failed.
func (p *CreditPurchase) MarkFailed(now int64) {
	if p.Status == CreditPurchaseStatusPending {
		p.Status = CreditPurchaseStatusFailed
		p.UpdatedAt = now
	}
}

// MarkCanceled transitions a pending purchase to canceled.
func (p *CreditPurchase) MarkCanceled(now int64) {
	if p.Status == CreditPurchaseStatusPending {
		p.Status = CreditPurchaseStatusCanceled
		p.UpdatedAt = now
	}
}

// CreditGrantSpec is the output of a successful capture: the credits to grant and the
// idempotency key that must be used when recording the grant.
type CreditGrantSpec struct {
	Amount         int64
	IdempotencyKey string
}

// SubscriptionRepository defines persistence for subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, sub *Subscription) error
	FindByID(ctx context.Context, id string) (*Subscription, error)
	FindByOwner(ctx context.Context, tenantID, ownerType, ownerID string) (*Subscription, error)
	Update(ctx context.Context, sub *Subscription) error
}

// BillingPeriodRepository defines persistence for billing periods.
type BillingPeriodRepository interface {
	Create(ctx context.Context, period *BillingPeriod) error
	FindByID(ctx context.Context, id, tenantID string) (*BillingPeriod, error)
	FindBySubscription(ctx context.Context, subscriptionID string) ([]*BillingPeriod, error)
	Update(ctx context.Context, period *BillingPeriod) error
}

// CreditPurchaseRepository defines persistence for credit purchases.
type CreditPurchaseRepository interface {
	Create(ctx context.Context, purchase *CreditPurchase) error
	FindByID(ctx context.Context, id string) (*CreditPurchase, error)
	FindByPaymentID(ctx context.Context, paymentID string) (*CreditPurchase, error)
	Update(ctx context.Context, purchase *CreditPurchase) error
	// MarkCaptured upserts a credit purchase row and marks it captured. Idempotent:
	// a replay with the same payment_id is a no-op (ON CONFLICT).
	MarkCaptured(ctx context.Context, paymentID, tenantID, planID, ownerType, ownerID string, amountCredits, now int64) error
}

// time import is reserved for future timestamp helpers; kept here so monetization
// packages can evolve without restructuring.
var _ = time.Time{}
