package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// ConversationContextStore implements domain.ConversationContextStore using PostgreSQL.
type ConversationContextStore struct {
	db *sql.DB
}

// NewConversationContextStore creates a new ConversationContextStore.
func NewConversationContextStore(db *sql.DB) *ConversationContextStore {
	return &ConversationContextStore{db: db}
}

// Save persists the conversation context.
func (s *ConversationContextStore) Save(ctx context.Context, conversationID string, context domain.ConversationContext) error {
	contextJSON, err := json.Marshal(context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	query := `INSERT INTO conversation_contexts (conversation_id, context, language, created_at, updated_at)
	          VALUES ($1, $2, $3, NOW(), NOW())
	          ON CONFLICT (conversation_id) DO UPDATE SET
	            context = EXCLUDED.context,
	            language = EXCLUDED.language,
	            updated_at = NOW()`

	_, err = s.db.ExecContext(ctx, query,
		conversationID, string(contextJSON), context.Language,
	)
	if err != nil {
		return fmt.Errorf("failed to save conversation context: %w", err)
	}
	return nil
}

// Load retrieves the conversation context.
func (s *ConversationContextStore) Load(ctx context.Context, conversationID string) (*domain.ConversationContext, error) {
	query := `SELECT context FROM conversation_contexts WHERE conversation_id = $1`
	row := s.db.QueryRowContext(ctx, query, conversationID)

	var contextJSON string
	err := row.Scan(&contextJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("context not found")
		}
		return nil, fmt.Errorf("failed to load conversation context: %w", err)
	}

	var context domain.ConversationContext
	if err := json.Unmarshal([]byte(contextJSON), &context); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	return &context, nil
}

// Delete removes the conversation context.
func (s *ConversationContextStore) Delete(ctx context.Context, conversationID string) error {
	query := `DELETE FROM conversation_contexts WHERE conversation_id = $1`
	_, err := s.db.ExecContext(ctx, query, conversationID)
	if err != nil {
		return fmt.Errorf("failed to delete conversation context: %w", err)
	}
	return nil
}
