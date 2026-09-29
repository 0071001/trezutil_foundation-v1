package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// conversationRow represents a row from the conversations table.
type conversationRow struct {
	ID             string
	OrganizationID sql.NullString
	CustomerID     sql.NullString
	SellerID       sql.NullString
	StoreID        sql.NullString
	Channel        string
	ExternalID     string
	Language       string
	State          string
	MetadataJSON   []byte
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ConversationStore implements domain.ConversationStore using PostgreSQL.
type ConversationStore struct {
	db *sql.DB
}

// NewConversationStore creates a new ConversationStore.
func NewConversationStore(db *sql.DB) *ConversationStore {
	return &ConversationStore{db: db}
}

// Create inserts a new conversation into the database.
func (r *ConversationStore) Create(ctx context.Context, conversation *domain.Conversation) error {
	metadataJSON, err := json.Marshal(conversation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation metadata: %w", err)
	}

	query := `INSERT INTO conversations (id, organization_id, customer_id, seller_id, store_id, channel, external_id, language, state, metadata, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	var sellerID, storeID sql.NullString
	if conversation.SellerID != "" {
		sellerID = sql.NullString{String: conversation.SellerID, Valid: true}
	}
	if conversation.StoreID != "" {
		storeID = sql.NullString{String: conversation.StoreID, Valid: true}
	}

	_, err = r.db.ExecContext(ctx, query,
		conversation.ID, nullString(conversation.OrganizationID), nullString(conversation.CustomerID),
		sellerID, storeID, string(conversation.Channel), conversation.ExternalID,
		conversation.Language, string(conversation.State), metadataJSON,
		toTime(conversation.CreatedAt), toTime(conversation.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create conversation: %w", err)
	}
	return nil
}

// FindByID retrieves a conversation by ID.
func (r *ConversationStore) FindByID(ctx context.Context, id string) (*domain.Conversation, error) {
	query := `SELECT id, organization_id, customer_id, seller_id, store_id, channel, external_id, language, state, metadata, created_at, updated_at
	          FROM conversations WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	var u conversationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID,
		&u.SellerID, &u.StoreID, &u.Channel, &u.ExternalID,
		&u.Language, &u.State, &u.MetadataJSON,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find conversation: %w", err)
	}

	return unmarshalConversation(u)
}

// FindByChannelExternalID retrieves a conversation by channel and external ID.
func (r *ConversationStore) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.Conversation, error) {
	query := `SELECT id, organization_id, customer_id, seller_id, store_id, channel, external_id, language, state, metadata, created_at, updated_at
	          FROM conversations WHERE channel = $1 AND external_id = $2`
	row := r.db.QueryRowContext(ctx, query, string(channel), externalID)

	var u conversationRow
	err := row.Scan(
		&u.ID, &u.OrganizationID, &u.CustomerID,
		&u.SellerID, &u.StoreID, &u.Channel, &u.ExternalID,
		&u.Language, &u.State, &u.MetadataJSON,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find conversation by channel and external ID: %w", err)
	}

	return unmarshalConversation(u)
}

// Update updates an existing conversation in the database.
func (r *ConversationStore) Update(ctx context.Context, conversation *domain.Conversation) error {
	metadataJSON, err := json.Marshal(conversation.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal conversation metadata: %w", err)
	}

	query := `UPDATE conversations SET
	          organization_id = $1, customer_id = $2, seller_id = $3, store_id = $4,
	          channel = $5, external_id = $6, language = $7, state = $8, metadata = $9,
	          updated_at = $10
	          WHERE id = $11`

	var sellerID, storeID sql.NullString
	if conversation.SellerID != "" {
		sellerID = sql.NullString{String: conversation.SellerID, Valid: true}
	}
	if conversation.StoreID != "" {
		storeID = sql.NullString{String: conversation.StoreID, Valid: true}
	}

	result, err := r.db.ExecContext(ctx, query,
		nullString(conversation.OrganizationID), nullString(conversation.CustomerID),
		sellerID, storeID, string(conversation.Channel), conversation.ExternalID,
		conversation.Language, string(conversation.State), metadataJSON,
		toTime(conversation.UpdatedAt), conversation.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update conversation: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("conversation not found: %s", conversation.ID)
	}

	return nil
}

func unmarshalConversation(u conversationRow) (*domain.Conversation, error) {
	conversation := &domain.Conversation{
		ID:             u.ID,
		OrganizationID: u.OrganizationID.String,
		CustomerID:     u.CustomerID.String,
		Channel:        domain.Channel(u.Channel),
		ExternalID:     u.ExternalID,
		Language:       u.Language,
		State:          domain.ConversationState(u.State),
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
		Metadata:       make(map[string]interface{}),
	}

	if u.SellerID.Valid {
		conversation.SellerID = u.SellerID.String
	}
	if u.StoreID.Valid {
		conversation.StoreID = u.StoreID.String
	}

	if len(u.MetadataJSON) > 0 && string(u.MetadataJSON) != "null" {
		if err := json.Unmarshal(u.MetadataJSON, &conversation.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal conversation metadata: %w", err)
		}
	}

	return conversation, nil
}
