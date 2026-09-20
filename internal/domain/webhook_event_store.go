package domain

import "context"

// WebhookEventStore defines the interface for durable webhook event idempotency.
type WebhookEventStore interface {
	// TryClaim attempts to atomically claim a webhook event for processing.
	// Returns (claimed, error). If already claimed/processed, returns false, nil.
	TryClaim(ctx context.Context, channel Channel, idempotencyKey string) (bool, error)
	// MarkProcessed marks a claimed event as successfully processed.
	MarkProcessed(ctx context.Context, channel Channel, idempotencyKey string) error
	// MarkFailed marks a claimed event as failed for retry.
	MarkFailed(ctx context.Context, channel Channel, idempotencyKey, errorMsg string) error
}

// WebhookEvent represents a webhook event record.
type WebhookEvent struct {
	ID             string
	Channel        Channel
	ProviderMsgID  string
	IdempotencyKey string
	Status         string
	Payload        []byte
	ErrorMessage   string
	ProcessedAt    *int64
	CreatedAt      int64
}
