package domain

import (
	"context"
	"fmt"
)

// PaymentWebhookStatus represents the processing status of a payment webhook event.
type PaymentWebhookStatus string

const (
	PaymentWebhookStatusPending    PaymentWebhookStatus = "pending"
	PaymentWebhookStatusProcessing PaymentWebhookStatus = "processing"
	PaymentWebhookStatusProcessed  PaymentWebhookStatus = "processed"
	PaymentWebhookStatusFailed     PaymentWebhookStatus = "failed"
)

// PaymentWebhookEventType represents the type of a payment provider webhook event.
type PaymentWebhookEventType string

const (
	WebhookEventPaymentSucceeded PaymentWebhookEventType = "payment.succeeded"
	WebhookEventPaymentFailed    PaymentWebhookEventType = "payment.failed"
	WebhookEventPaymentCanceled  PaymentWebhookEventType = "payment.canceled"
	WebhookEventPaymentExpired   PaymentWebhookEventType = "payment.expired"
	WebhookEventRefundSucceeded  PaymentWebhookEventType = "refund.succeeded"
	WebhookEventRefundFailed     PaymentWebhookEventType = "refund.failed"
	WebhookEventChargeRefunded   PaymentWebhookEventType = "charge.refunded"
	WebhookEventChargeDispute    PaymentWebhookEventType = "charge.dispute"
)

// PaymentWebhookEvent represents a webhook event from a payment provider.
type PaymentWebhookEvent struct {
	ID            string
	ProviderCode  string
	ProviderMsgID string
	TenantID      string
	EventType     PaymentWebhookEventType
	Payload       []byte
	Status        PaymentWebhookStatus
	ProcessedAt   *int64
	ErrorMessage  string
	CreatedAt     int64
}

// NewPaymentWebhookEvent creates a new payment webhook event in PENDING status.
func NewPaymentWebhookEvent(
	providerCode, providerMsgID, tenantID string,
	eventType PaymentWebhookEventType,
	payload []byte,
	now int64,
) *PaymentWebhookEvent {
	return &PaymentWebhookEvent{
		ProviderCode:  providerCode,
		ProviderMsgID: providerMsgID,
		TenantID:      tenantID,
		EventType:     eventType,
		Payload:       payload,
		Status:        PaymentWebhookStatusPending,
		CreatedAt:     now,
	}
}

// ValidateInvariants checks invariants for a PaymentWebhookEvent.
func (e *PaymentWebhookEvent) ValidateInvariants() error {
	if e.ProviderCode == "" {
		return fmt.Errorf("provider_code is required")
	}
	if e.ProviderMsgID == "" {
		return fmt.Errorf("provider_message_id is required")
	}
	if e.TenantID == "" {
		return fmt.Errorf("tenant_id is required")
	}
	return nil
}

// PaymentWebhookEventStore defines the interface for durable payment webhook
// event idempotency.
//
// This is separate from the channel-level WebhookEventStore (used by WhatsApp)
// because payment webhooks require provider-specific tenant resolution,
// signature verification, and reconciliation semantics.
type PaymentWebhookEventStore interface {
	// TryClaim atomically claims a webhook event for processing.
	// Returns (claimed, error). If already claimed/processed, returns false, nil.
	TryClaim(ctx context.Context, providerCode, providerMsgID string) (bool, error)
	// MarkProcessed marks a claimed event as successfully processed.
	MarkProcessed(ctx context.Context, providerCode, providerMsgID string) error
	// MarkFailed marks a claimed event as failed for retry.
	MarkFailed(ctx context.Context, providerCode, providerMsgID string, errorMsg string) error
	// FindByProviderMessageID retrieves an event by provider and message ID.
	FindByProviderMessageID(ctx context.Context, providerCode, providerMsgID string) (*PaymentWebhookEvent, error)
	// PersistEvent inserts a webhook event record (idempotent via ON CONFLICT).
	PersistEvent(ctx context.Context, event *PaymentWebhookEvent) error
}
