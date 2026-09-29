package domain

import "context"

// OTPChallenge is a one-time passcode issued to a phone number. Only a keyed
// hash of the code is persisted.
type OTPChallenge struct {
	ID          string
	Phone       string
	CodeHash    string
	Attempts    int
	MaxAttempts int
	ExpiresAt   int64
	ConsumedAt  *int64
	CreatedAt   int64
}

// OTPChallengeRepository persists OTP challenges.
type OTPChallengeRepository interface {
	Create(ctx context.Context, challenge *OTPChallenge) error
	FindByID(ctx context.Context, id string) (*OTPChallenge, error)
	// RegisterAttempt atomically increments the attempt counter of a live
	// (unconsumed, unexpired, under-limit) challenge. It returns false when
	// the challenge can no longer be attempted.
	RegisterAttempt(ctx context.Context, id string, now int64) (bool, error)
	// Consume atomically marks a live challenge consumed. It returns false when
	// the challenge was already consumed (e.g. by a concurrent verification).
	Consume(ctx context.Context, id string, now int64) (bool, error)
	// CountSince returns the number of challenges issued for phone since the given time.
	CountSince(ctx context.Context, phone string, since int64) (int, error)
}
