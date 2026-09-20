package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type webhookEventRow struct {
	ID             string
	Channel        string
	ProviderMsgID  string
	IdempotencyKey string
	Status         string
	Payload        []byte
	ErrorMessage   sql.NullString
	ProcessedAt    sql.NullTime
	CreatedAt      time.Time
}

// WebhookEventStore implements domain.WebhookEventStore using PostgreSQL.
type WebhookEventStore struct {
	db *sql.DB
}

// NewWebhookEventStore creates a new WebhookEventStore.
func NewWebhookEventStore(db *sql.DB) *WebhookEventStore {
	return &WebhookEventStore{db: db}
}

// TryClaim atomically claims a webhook event for processing.
// Uses INSERT ON CONFLICT + UPDATE pattern for race-free claiming.
func (s *WebhookEventStore) TryClaim(ctx context.Context, channel domain.Channel, idempotencyKey string) (bool, error) {
	// Try to insert new event with 'pending' status
	queryInsert := `INSERT INTO webhook_events (channel, provider_message_id, idempotency_key, status, payload)
	                VALUES ($1, $2, $3, 'pending', '{}')
	                ON CONFLICT (channel, idempotency_key) DO NOTHING
	                RETURNING id`

	var id string
	err := s.db.QueryRowContext(ctx, queryInsert, string(channel), "", idempotencyKey).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			// Event exists, try to claim it by updating from 'pending' to 'processing'
			return s.claimExisting(ctx, channel, idempotencyKey)
		}
		return false, fmt.Errorf("failed to insert webhook event: %w", err)
	}
	// Successfully inserted and claimed
	return true, nil
}

func (s *WebhookEventStore) claimExisting(ctx context.Context, channel domain.Channel, idempotencyKey string) (bool, error) {
	query := `UPDATE webhook_events
	          SET status = 'processing', processed_at = NOW()
	          WHERE channel = $1 AND idempotency_key = $2 AND status = 'pending'
	          RETURNING status`

	var status string
	err := s.db.QueryRowContext(ctx, query, string(channel), idempotencyKey).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			// No row with status='pending' - already claimed or processed
			return false, nil
		}
		return false, fmt.Errorf("failed to claim webhook event: %w", err)
	}
	// Successfully claimed existing pending event
	return true, nil
}

// MarkProcessed marks a claimed event as successfully processed.
func (s *WebhookEventStore) MarkProcessed(ctx context.Context, channel domain.Channel, idempotencyKey string) error {
	query := `UPDATE webhook_events
	          SET status = 'processed', processed_at = NOW()
	          WHERE channel = $1 AND idempotency_key = $2 AND status = 'processing'`

	result, err := s.db.ExecContext(ctx, query, string(channel), idempotencyKey)
	if err != nil {
		return fmt.Errorf("failed to mark webhook event processed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("webhook event not found or not in processing state")
	}
	return nil
}

// MarkFailed marks a claimed event as failed for retry.
func (s *WebhookEventStore) MarkFailed(ctx context.Context, channel domain.Channel, idempotencyKey, errorMsg string) error {
	query := `UPDATE webhook_events
	          SET status = 'failed', error_message = $3, processed_at = NOW()
	          WHERE channel = $1 AND idempotency_key = $2 AND status = 'processing'`

	result, err := s.db.ExecContext(ctx, query, string(channel), idempotencyKey, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark webhook event failed: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("webhook event not found or not in processing state")
	}
	return nil
}
