package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

// PersonalSpaceStore implements domain.PersonalSpaces (migration 000033).
type PersonalSpaceStore struct {
	db *sql.DB
}

// NewPersonalSpaceStore creates a PersonalSpaceStore.
func NewPersonalSpaceStore(db *sql.DB) *PersonalSpaceStore {
	return &PersonalSpaceStore{db: db}
}

// ErrUnknownUser: no user has that id, so no personal space can be made for it.
var ErrUnknownUser = errors.New("unknown user")

// Find returns the user's personal organisation id, or "".
func (s *PersonalSpaceStore) Find(ctx context.Context, userID string) (string, error) {
	var orgID string
	err := s.db.QueryRowContext(ctx, `SELECT organization_id FROM personal_spaces WHERE user_id = $1`, userID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read personal space: %w", err)
	}
	return orgID, nil
}

// Ensure returns the user's personal organisation, creating it (an 'individual' organisation named after the user)
// the first time. Concurrent first calls for one user are serialised by a transaction-scoped advisory lock, so
// exactly one organisation is created.
func (s *PersonalSpaceStore) Ensure(ctx context.Context, userID string) (string, error) {
	if orgID, err := s.Find(ctx, userID); err != nil || orgID != "" {
		return orgID, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin personal space: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('personal_space:' || $1::text))`, userID); err != nil {
		return "", fmt.Errorf("lock personal space: %w", err)
	}
	var orgID string
	err = tx.QueryRowContext(ctx, `SELECT organization_id FROM personal_spaces WHERE user_id = $1`, userID).Scan(&orgID)
	if err == nil {
		return orgID, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("read personal space: %w", err)
	}
	err = tx.QueryRowContext(ctx, `
		INSERT INTO organizations (type, legal_name, display_name, description, phone, country, status, verification_status)
		SELECT 'individual',
		       COALESCE(NULLIF(btrim(display_name), ''), NULLIF(phone, ''), 'Espace personnel'),
		       COALESCE(NULLIF(btrim(display_name), ''), NULLIF(phone, ''), 'Espace personnel'),
		       'Espace personnel : crédits du compte',
		       COALESCE(phone, ''), 'CD', 'active', 'unverified'
		FROM users WHERE id = $1
		RETURNING id`, userID).Scan(&orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUnknownUser
	}
	if err != nil {
		return "", fmt.Errorf("create personal space: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO personal_spaces (user_id, organization_id) VALUES ($1, $2)`, userID, orgID); err != nil {
		return "", fmt.Errorf("record personal space: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit personal space: %w", err)
	}
	return orgID, nil
}

var _ domain.PersonalSpaces = (*PersonalSpaceStore)(nil)
