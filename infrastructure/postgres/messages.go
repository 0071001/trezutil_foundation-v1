package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type messageRow struct {
	ID             string
	ConversationID string
	Direction      string
	Type           string
	SenderID       string
	RecipientID    sql.NullString
	ContentJSON    []byte
	ExternalRef    string
	IdempotencyKey string
	Language       string
	CreatedAt      time.Time
}

// MessageStore implements domain.MessageStore using PostgreSQL.
type MessageStore struct {
	db *sql.DB
}

// NewMessageStore creates a new MessageStore.
func NewMessageStore(db *sql.DB) *MessageStore {
	return &MessageStore{db: db}
}

// Create inserts a new message.
func (s *MessageStore) Create(ctx context.Context, message *domain.Message) error {
	contentJSON, err := json.Marshal(message.Content)
	if err != nil {
		return fmt.Errorf("failed to marshal message content: %w", err)
	}

	query := `INSERT INTO messages (id, conversation_id, direction, type, sender_id, recipient_id, content, external_ref, idempotency_key, language, created_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	var recipientID sql.NullString
	if message.RecipientID != "" {
		recipientID = sql.NullString{String: message.RecipientID, Valid: true}
	}

	_, err = s.db.ExecContext(ctx, query,
		message.ID, message.ConversationID, string(message.Direction), string(message.Type),
		message.SenderID, recipientID, contentJSON, message.ExternalRef, message.IdempotencyKey,
		message.Language, toTime(message.CreatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

// FindByID retrieves a message by ID.
func (s *MessageStore) FindByID(ctx context.Context, id string) (*domain.Message, error) {
	query := `SELECT id, conversation_id, direction, type, sender_id, recipient_id, content, external_ref, idempotency_key, language, created_at
	          FROM messages WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	return scanMessage(row)
}

// FindByConversationID retrieves all messages for a conversation.
func (s *MessageStore) FindByConversationID(ctx context.Context, conversationID string) ([]*domain.Message, error) {
	query := `SELECT id, conversation_id, direction, type, sender_id, recipient_id, content, external_ref, idempotency_key, language, created_at
	          FROM messages WHERE conversation_id = $1 ORDER BY created_at`
	rows, err := s.db.QueryContext(ctx, query, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}
	defer rows.Close()

	var messages []*domain.Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}
	if messages == nil {
		messages = []*domain.Message{}
	}
	return messages, nil
}

// FindByExternalRef retrieves a message by channel and external reference.
func (s *MessageStore) FindByExternalRef(ctx context.Context, channel domain.Channel, externalRef string) (*domain.Message, error) {
	query := `SELECT m.id, m.conversation_id, m.direction, m.type, m.sender_id, m.recipient_id, m.content, m.external_ref, m.idempotency_key, m.language, m.created_at
	          FROM messages m
	          INNER JOIN conversations c ON m.conversation_id = c.id
	          WHERE c.channel = $1 AND m.external_ref = $2`
	row := s.db.QueryRowContext(ctx, query, string(channel), externalRef)
	return scanMessage(row)
}

func scanMessage(row scanner) (*domain.Message, error) {
	var u messageRow
	var recipientID sql.NullString
	err := row.Scan(&u.ID, &u.ConversationID, &u.Direction, &u.Type, &u.SenderID, &recipientID,
		&u.ContentJSON, &u.ExternalRef, &u.IdempotencyKey, &u.Language, &u.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find message: %w", err)
	}

	content := &domain.MessageContent{Data: make(map[string]interface{})}
	if len(u.ContentJSON) > 0 && string(u.ContentJSON) != "null" {
		if err := json.Unmarshal(u.ContentJSON, content); err != nil {
			return nil, fmt.Errorf("failed to unmarshal message content: %w", err)
		}
	}

	message := &domain.Message{
		ID:             u.ID,
		ConversationID: u.ConversationID,
		Direction:      domain.MessageDirection(u.Direction),
		Type:           domain.MessageType(u.Type),
		SenderID:       u.SenderID,
		Content:        content,
		ExternalRef:    u.ExternalRef,
		IdempotencyKey: u.IdempotencyKey,
		Language:       u.Language,
		CreatedAt:      toUnix(u.CreatedAt),
	}

	if recipientID.Valid {
		message.RecipientID = recipientID.String
	}

	return message, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}
