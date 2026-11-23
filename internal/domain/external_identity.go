package domain

import (
	"context"
	"errors"
)

// IdentityProviderGoogle is the provider name of a Google account in external_identities.
const IdentityProviderGoogle = "google"

// ExternalIdentity links a Trezutil account to an account at an identity provider (migration 000035).
type ExternalIdentity struct {
	UserID    string
	Provider  string
	Subject   string // the provider's stable account id ("sub"), never an email
	Email     string // as reported at the last sign-in, for display only
	Name      string
	CreatedAt int64
}

// ExternalIdentityRepository stores those links.
type ExternalIdentityRepository interface {
	// FindUserID returns the account linked to provider+subject, or "" when none is.
	FindUserID(ctx context.Context, provider, subject string) (string, error)
	// Link records a link; ErrIdentityTaken when the provider account or the user's slot for that provider is used.
	Link(ctx context.Context, identity *ExternalIdentity) error
	// Touch refreshes the email and name shown for a link.
	Touch(ctx context.Context, provider, subject, email, name string) error
	ListByUser(ctx context.Context, userID string) ([]ExternalIdentity, error)
	Unlink(ctx context.Context, userID, provider string) error
}

var (
	// ErrIdentityTaken: the provider account is already linked (to this or another account), or this account
	// already has one of that provider.
	ErrIdentityTaken = errors.New("this identity is already linked")
	// ErrIdentityNotLinked: nothing to unlink.
	ErrIdentityNotLinked = errors.New("no identity of this provider is linked")
)
