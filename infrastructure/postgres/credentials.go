package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

var _ domain.CredentialRepository = (*UserRepository)(nil)

// FindUserByEmail returns the user whose email matches, ignoring case.
func (r *UserRepository) FindUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}
	return r.FindByID(ctx, id)
}

// GetCredential returns the password hash and the sign-in counters.
func (r *UserRepository) GetCredential(ctx context.Context, userID string) (*domain.Credential, error) {
	var hash sql.NullString
	var attempts int
	var lockedUntil sql.NullTime
	err := r.db.QueryRowContext(ctx, `SELECT password_hash, failed_login_attempts, locked_until FROM users WHERE id = $1`, userID).Scan(&hash, &attempts, &lockedUntil)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load credential: %w", err)
	}
	if !hash.Valid || hash.String == "" {
		return nil, domain.ErrCredentialNotFound
	}
	c := &domain.Credential{UserID: userID, PasswordHash: hash.String, FailedAttempts: attempts}
	if lockedUntil.Valid {
		c.LockedUntil = toUnix(lockedUntil.Time)
	}
	return c, nil
}

// SetPassword stores a new hash and clears the failure counter and any lock.
func (r *UserRepository) SetPassword(ctx context.Context, userID, hash string, now int64) error {
	res, err := r.db.ExecContext(ctx, `UPDATE users SET password_hash = $2, password_changed_at = $3, failed_login_attempts = 0, locked_until = NULL, updated_at = $3 WHERE id = $1`, userID, hash, toTime(now))
	if err != nil {
		return fmt.Errorf("failed to set password: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

// RecordLoginFailure counts a wrong password under a row lock, so concurrent
// guesses cannot slip past the limit.
func (r *UserRepository) RecordLoginFailure(ctx context.Context, userID string, now int64, maxAttempts int, lockFor int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var attempts int
	var lockedUntil sql.NullTime
	if err := tx.QueryRowContext(ctx, `SELECT failed_login_attempts, locked_until FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&attempts, &lockedUntil); err != nil {
		return fmt.Errorf("failed to load attempts: %w", err)
	}
	if lockedUntil.Valid && toUnix(lockedUntil.Time) <= now {
		attempts = 0 // an expired lock starts a fresh count
		lockedUntil = sql.NullTime{}
	}
	attempts++
	if attempts >= maxAttempts {
		lockedUntil = sql.NullTime{Time: toTime(now + lockFor), Valid: true}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET failed_login_attempts = $2, locked_until = $3 WHERE id = $1`, userID, attempts, lockedUntil); err != nil {
		return fmt.Errorf("failed to record failure: %w", err)
	}
	return tx.Commit()
}

// RecordLoginSuccess clears the failure counter and stamps the sign-in time.
func (r *UserRepository) RecordLoginSuccess(ctx context.Context, userID string, now int64) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE users SET failed_login_attempts = 0, locked_until = NULL, last_login_at = $2 WHERE id = $1`, userID, toTime(now)); err != nil {
		return fmt.Errorf("failed to record sign-in: %w", err)
	}
	return nil
}
