package domain

import (
	"context"
	"errors"
	"net/mail"
	"strings"
)

// Staff accounts (admin, operator, moderator) sign in with an email address
// and a password. Customers and sellers keep using a phone number and a
// one-time code.

var (
	// ErrInvalidCredentials is the only answer to a failed staff sign-in: it never
	// says whether the email exists, the password was wrong, the account is
	// locked or the account is not a staff account.
	ErrInvalidCredentials = errors.New("email or password is incorrect")
	// ErrCredentialNotFound means the account has no password.
	ErrCredentialNotFound = errors.New("no password is set for this account")
	// ErrInvalidEmail is returned for an address that is not a plain email address.
	ErrInvalidEmail = errors.New("invalid email address")
)

// StaffRoles are the roles that may sign in with a password.
var StaffRoles = []Role{RoleAdmin, RoleOperator, RoleModerator}

// IsStaff reports whether the user holds a staff role.
func (u *User) IsStaff() bool { return u.HasAnyRole(StaffRoles...) }

// Credential is the password state of an account.
type Credential struct {
	UserID         string
	PasswordHash   string
	FailedAttempts int
	// LockedUntil is a Unix time; 0 when the account is not locked.
	LockedUntil int64
}

// CredentialRepository stores staff passwords and the sign-in attempt counters.
type CredentialRepository interface {
	// FindUserByEmail returns the user with that email (case-insensitive), roles loaded.
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	// GetCredential returns the password state, or ErrCredentialNotFound.
	GetCredential(ctx context.Context, userID string) (*Credential, error)
	// SetPassword stores a new hash and clears the failure counter and any lock.
	SetPassword(ctx context.Context, userID, hash string, now int64) error
	// RecordLoginFailure counts a wrong password. Once maxAttempts consecutive
	// failures are reached the account is locked for lockFor seconds; failures
	// before an expired lock do not count.
	RecordLoginFailure(ctx context.Context, userID string, now int64, maxAttempts int, lockFor int64) error
	// RecordLoginSuccess clears the failure counter and stamps the last sign-in.
	RecordLoginSuccess(ctx context.Context, userID string, now int64) error
}

// NormalizeEmail returns the address trimmed and lower-cased, or ErrInvalidEmail.
func NormalizeEmail(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" || len(s) > 254 {
		return "", ErrInvalidEmail
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Name != "" || addr.Address != s || !strings.Contains(s[strings.Index(s, "@"):], ".") {
		return "", ErrInvalidEmail
	}
	return s, nil
}
