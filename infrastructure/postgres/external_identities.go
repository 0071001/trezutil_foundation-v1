package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/trezutil/trezutil/internal/domain"
)

// ExternalIdentityRepository implements domain.ExternalIdentityRepository (migrations 000002, 000035).
type ExternalIdentityRepository struct {
	db *sql.DB
}

// NewExternalIdentityRepository creates the repository.
func NewExternalIdentityRepository(db *sql.DB) *ExternalIdentityRepository {
	return &ExternalIdentityRepository{db: db}
}

// FindUserID returns the account linked to provider+subject, or "".
func (r *ExternalIdentityRepository) FindUserID(ctx context.Context, provider, subject string) (string, error) {
	var userID string
	err := r.db.QueryRowContext(ctx, `SELECT user_id FROM external_identities WHERE provider = $1 AND provider_subject = $2`, provider, subject).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("find identity: %w", err)
	}
	return userID, nil
}

func identityMetadata(email, name string) []byte {
	raw, _ := json.Marshal(map[string]string{"email": email, "name": name})
	return raw
}

// Link records a link; a unique violation (provider account or the user's slot already used) is ErrIdentityTaken.
func (r *ExternalIdentityRepository) Link(ctx context.Context, id *domain.ExternalIdentity) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO external_identities (user_id, provider, provider_subject, metadata) VALUES ($1, $2, $3, $4)`,
		id.UserID, id.Provider, id.Subject, identityMetadata(id.Email, id.Name))
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return domain.ErrIdentityTaken
	}
	if err != nil {
		return fmt.Errorf("link identity: %w", err)
	}
	return nil
}

// Touch refreshes the email and name shown for a link.
func (r *ExternalIdentityRepository) Touch(ctx context.Context, provider, subject, email, name string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE external_identities SET metadata = $3 WHERE provider = $1 AND provider_subject = $2`,
		provider, subject, identityMetadata(email, name)); err != nil {
		return fmt.Errorf("update identity: %w", err)
	}
	return nil
}

// ListByUser returns the user's links.
func (r *ExternalIdentityRepository) ListByUser(ctx context.Context, userID string) ([]domain.ExternalIdentity, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT user_id, provider, provider_subject, COALESCE(metadata->>'email', ''), COALESCE(metadata->>'name', ''), created_at
		FROM external_identities WHERE user_id = $1 ORDER BY created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list identities: %w", err)
	}
	defer rows.Close()
	out := []domain.ExternalIdentity{}
	for rows.Next() {
		var id domain.ExternalIdentity
		var created time.Time
		if err := rows.Scan(&id.UserID, &id.Provider, &id.Subject, &id.Email, &id.Name, &created); err != nil {
			return nil, err
		}
		id.CreatedAt = created.Unix()
		out = append(out, id)
	}
	return out, rows.Err()
}

// Unlink removes the user's link to a provider.
func (r *ExternalIdentityRepository) Unlink(ctx context.Context, userID, provider string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM external_identities WHERE user_id = $1 AND provider = $2`, userID, provider)
	if err != nil {
		return fmt.Errorf("unlink identity: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrIdentityNotLinked
	}
	return nil
}

var _ domain.ExternalIdentityRepository = (*ExternalIdentityRepository)(nil)
