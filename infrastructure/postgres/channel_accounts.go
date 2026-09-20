package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

type channelAccountRow struct {
	ID             string
	OrganizationID string
	Channel        string
	ExternalID     string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ChannelAccountRepository implements domain.ChannelAccountRepository using PostgreSQL.
type ChannelAccountRepository struct {
	db *sql.DB
}

// NewChannelAccountRepository creates a new ChannelAccountRepository.
func NewChannelAccountRepository(db *sql.DB) *ChannelAccountRepository {
	return &ChannelAccountRepository{db: db}
}

// Create inserts a new channel account.
func (r *ChannelAccountRepository) Create(ctx context.Context, account *domain.ChannelAccount) error {
	query := `INSERT INTO channel_accounts (id, organization_id, channel, external_id, status, created_at, updated_at)
	          VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := r.db.ExecContext(ctx, query,
		account.ID, account.OrganizationID, string(account.Channel), account.ExternalID,
		string(account.Status), toTime(account.CreatedAt), toTime(account.UpdatedAt),
	)
	if err != nil {
		return fmt.Errorf("failed to create channel account: %w", err)
	}
	return nil
}

// FindByID retrieves a channel account by ID.
func (r *ChannelAccountRepository) FindByID(ctx context.Context, id string) (*domain.ChannelAccount, error) {
	query := `SELECT id, organization_id, channel, external_id, status, created_at, updated_at
	          FROM channel_accounts WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanChannelAccount(row)
}

// FindByChannelExternalID retrieves a channel account by channel and external ID.
func (r *ChannelAccountRepository) FindByChannelExternalID(ctx context.Context, channel domain.Channel, externalID string) (*domain.ChannelAccount, error) {
	query := `SELECT id, organization_id, channel, external_id, status, created_at, updated_at
	          FROM channel_accounts WHERE channel = $1 AND external_id = $2`
	row := r.db.QueryRowContext(ctx, query, string(channel), externalID)
	return scanChannelAccount(row)
}

// Update updates an existing channel account.
func (r *ChannelAccountRepository) Update(ctx context.Context, account *domain.ChannelAccount) error {
	query := `UPDATE channel_accounts SET organization_id=$1, status=$2, updated_at=$3 WHERE id=$4`
	result, err := r.db.ExecContext(ctx, query,
		account.OrganizationID, string(account.Status), toTime(account.UpdatedAt), account.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update channel account: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("channel account not found: %s", account.ID)
	}
	return nil
}

func scanChannelAccount(row *sql.Row) (*domain.ChannelAccount, error) {
	var u channelAccountRow
	err := row.Scan(&u.ID, &u.OrganizationID, &u.Channel, &u.ExternalID, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find channel account: %w", err)
	}
	return &domain.ChannelAccount{
		ID:             u.ID,
		OrganizationID: u.OrganizationID,
		Channel:        domain.Channel(u.Channel),
		ExternalID:     u.ExternalID,
		Status:         domain.ChannelAccountStatus(u.Status),
		CreatedAt:      toUnix(u.CreatedAt),
		UpdatedAt:      toUnix(u.UpdatedAt),
	}, nil
}
