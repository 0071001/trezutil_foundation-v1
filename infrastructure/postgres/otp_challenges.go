package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// OTPChallengeRepository implements domain.OTPChallengeRepository.
type OTPChallengeRepository struct {
	db *sql.DB
}

// NewOTPChallengeRepository creates a new OTPChallengeRepository.
func NewOTPChallengeRepository(db *sql.DB) *OTPChallengeRepository {
	return &OTPChallengeRepository{db: db}
}

// Create inserts a new challenge.
func (r *OTPChallengeRepository) Create(ctx context.Context, c *domain.OTPChallenge) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_otp_challenges (id, phone, code_hash, attempts, max_attempts, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		c.ID, c.Phone, c.CodeHash, c.Attempts, c.MaxAttempts, toTime(c.ExpiresAt), toTime(c.CreatedAt))
	if err != nil {
		return fmt.Errorf("failed to create otp challenge: %w", err)
	}
	return nil
}

// FindByID loads a challenge by ID.
func (r *OTPChallengeRepository) FindByID(ctx context.Context, id string) (*domain.OTPChallenge, error) {
	var c domain.OTPChallenge
	var expiresAt, createdAt time.Time
	var consumedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, `
		SELECT id, phone, code_hash, attempts, max_attempts, expires_at, consumed_at, created_at
		FROM auth_otp_challenges WHERE id = $1`, id).
		Scan(&c.ID, &c.Phone, &c.CodeHash, &c.Attempts, &c.MaxAttempts, &expiresAt, &consumedAt, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrInvalidOTP
	}
	if err != nil {
		// A malformed (non-UUID) id is indistinguishable from an unknown one for the caller.
		return nil, domain.ErrInvalidOTP
	}
	c.ExpiresAt = toUnix(expiresAt)
	c.CreatedAt = toUnix(createdAt)
	if consumedAt.Valid {
		ts := toUnix(consumedAt.Time)
		c.ConsumedAt = &ts
	}
	return &c, nil
}

// RegisterAttempt atomically counts an attempt against a live challenge.
func (r *OTPChallengeRepository) RegisterAttempt(ctx context.Context, id string, now int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE auth_otp_challenges SET attempts = attempts + 1
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > $2 AND attempts < max_attempts`,
		id, toTime(now))
	if err != nil {
		return false, fmt.Errorf("failed to register otp attempt: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Consume atomically marks a live challenge consumed.
func (r *OTPChallengeRepository) Consume(ctx context.Context, id string, now int64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE auth_otp_challenges SET consumed_at = $2
		WHERE id = $1 AND consumed_at IS NULL AND expires_at > $2`,
		id, toTime(now))
	if err != nil {
		return false, fmt.Errorf("failed to consume otp challenge: %w", err)
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// CountSince counts challenges issued to phone since the given time.
func (r *OTPChallengeRepository) CountSince(ctx context.Context, phone string, since int64) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM auth_otp_challenges WHERE phone = $1 AND created_at >= $2`,
		phone, toTime(since)).Scan(&n); err != nil {
		return 0, fmt.Errorf("failed to count otp challenges: %w", err)
	}
	return n, nil
}
