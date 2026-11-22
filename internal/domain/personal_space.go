package domain

import "context"

// PersonalSpaces gives each person the 'individual' organisation that holds their own credits (migration 000033),
// so the organisation-scoped credit system serves personal accounts unchanged.
type PersonalSpaces interface {
	// Find returns the person's personal organisation id, or "" when it was never created (no credits yet).
	Find(ctx context.Context, userID string) (string, error)
	// Ensure returns it, creating it the first time; safe to call concurrently.
	Ensure(ctx context.Context, userID string) (string, error)
}
