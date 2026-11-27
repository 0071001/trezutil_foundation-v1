package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type paymentWebhookEventRow struct {
	ID             string
	ProviderCode   string
	ProviderMsgID  string
	OrganizationID string
	EventType      string
	Status         string
	Payload        []byte
	ProcessedAt    sql.NullTime
	ErrorMessage   sql.NullString
	CreatedAt      time.Time
}

type PaymentWebhookEventStore struct {
	db *sql.DB
}

func NewPaymentWebhookEventStore(db *sql.DB) *PaymentWebhookEventStore {
	return &PaymentWebhookEventStore{db: db}
}

const paymentWebhookInsertQuery = `INSERT INTO payment_webhook_events (
	id, provider_code, provider_msg_id, organization_id, event_type, payload, status, processed_at, error_message, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
ON CONFLICT (provider_code, provider_msg_id) DO NOTHING`

func (s *PaymentWebhookEventStore) TryClaim(ctx context.Context, providerCode, providerMsgID string) (bool, error) {
	row := s.db.QueryRowContext(ctx, `
		UPDATE payment_webhook_events
		SET status = 'processing', claimed_at = NOW(), attempts = attempts + 1
		WHERE provider_code = $1 AND provider_msg_id = $2
		  AND (status IN ('pending', 'failed')
		       OR (status = 'processing' AND COALESCE(claimed_at, created_at) < NOW() - make_interval(secs => $3)))
		RETURNING 1
	`, providerCode, providerMsgID, domain.PaymentWebhookClaimTimeout)

	var one int
	err := row.Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to claim payment webhook event: %w", err)
	}
	return true, nil
}

func (s *PaymentWebhookEventStore) MarkProcessed(ctx context.Context, providerCode, providerMsgID string) error {
	query := `UPDATE payment_webhook_events
		SET status = 'processed', processed_at = NOW()
		WHERE provider_code = $1 AND provider_msg_id = $2`

	result, err := s.db.ExecContext(ctx, query, providerCode, providerMsgID)
	if err != nil {
		return fmt.Errorf("failed to mark payment webhook event processed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrWebhookEventDuplicate
	}
	return nil
}

func (s *PaymentWebhookEventStore) MarkFailed(ctx context.Context, providerCode, providerMsgID string, errorMsg string) error {
	query := `UPDATE payment_webhook_events
		SET status = 'failed', error_message = $3
		WHERE provider_code = $1 AND provider_msg_id = $2 AND status = 'processing'`

	result, err := s.db.ExecContext(ctx, query, providerCode, providerMsgID, errorMsg)
	if err != nil {
		return fmt.Errorf("failed to mark payment webhook event failed: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrWebhookEventDuplicate
	}
	return nil
}

// MarkReview keeps a claimed event for a person.
func (s *PaymentWebhookEventStore) MarkReview(ctx context.Context, providerCode, providerMsgID string, reason string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE payment_webhook_events
		SET status = 'review', error_message = $3, processed_at = NOW()
		WHERE provider_code = $1 AND provider_msg_id = $2 AND status = 'processing'`, providerCode, providerMsgID, reason)
	if err != nil {
		return fmt.Errorf("failed to mark payment webhook event for review: %w", err)
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return domain.ErrWebhookEventDuplicate
	}
	return nil
}

func (s *PaymentWebhookEventStore) FindByProviderMessageID(ctx context.Context, providerCode, providerMsgID string) (*domain.PaymentWebhookEvent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, provider_code, provider_msg_id, organization_id, event_type, status, payload, processed_at, error_message, created_at
		FROM payment_webhook_events
		WHERE provider_code = $1 AND provider_msg_id = $2
	`, providerCode, providerMsgID)
	return s.scan(row)
}

func (s *PaymentWebhookEventStore) PersistEvent(ctx context.Context, event *domain.PaymentWebhookEvent) error {
	if event.ID == "" {
		query := `INSERT INTO payment_webhook_events (
			provider_code, provider_msg_id, organization_id, event_type, payload, status, processed_at, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (provider_code, provider_msg_id) DO NOTHING
		RETURNING id`
		var id string
		err := s.db.QueryRowContext(ctx, query,
			event.ProviderCode, event.ProviderMsgID, event.TenantID,
			string(event.EventType), event.Payload, string(event.Status),
			nullTimeFromPtr(event.ProcessedAt),
			event.ErrorMessage,
		).Scan(&id)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to persist payment webhook event: %w", err)
		}
		event.ID = id
	} else {
		query := `INSERT INTO payment_webhook_events (
			id, provider_code, provider_msg_id, organization_id, event_type, payload, status, processed_at, error_message, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (provider_code, provider_msg_id) DO NOTHING`
		_, err := s.db.ExecContext(ctx, query,
			event.ID, event.ProviderCode, event.ProviderMsgID, event.TenantID,
			string(event.EventType), event.Payload, string(event.Status),
			nullTimeFromPtr(event.ProcessedAt),
			event.ErrorMessage,
		)
		if err != nil {
			return fmt.Errorf("failed to persist payment webhook event: %w", err)
		}
	}
	return nil
}

func (s *PaymentWebhookEventStore) scan(row *sql.Row) (*domain.PaymentWebhookEvent, error) {
	var e paymentWebhookEventRow
	err := row.Scan(
		&e.ID, &e.ProviderCode, &e.ProviderMsgID, &e.OrganizationID,
		&e.EventType, &e.Status, &e.Payload, &e.ProcessedAt, &e.ErrorMessage, &e.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrWebhookEventDuplicate
		}
		return nil, fmt.Errorf("failed to scan payment webhook event: %w", err)
	}

	event := &domain.PaymentWebhookEvent{
		ID:            e.ID,
		ProviderCode:  e.ProviderCode,
		ProviderMsgID: e.ProviderMsgID,
		TenantID:      e.OrganizationID,
		EventType:     domain.PaymentWebhookEventType(e.EventType),
		Status:        domain.PaymentWebhookStatus(e.Status),
		Payload:       e.Payload,
		ErrorMessage:  e.ErrorMessage.String,
		CreatedAt:     toUnix(e.CreatedAt),
	}
	if e.ProcessedAt.Valid {
		ts := toUnix(e.ProcessedAt.Time)
		event.ProcessedAt = &ts
	}
	return event, nil
}
