package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/google"
)

// Sign in with Google: a second way into the same accounts, beside the phone one-time code. The browser gets a
// Google ID token (Google Identity Services); the server checks it against Google's keys and this application's
// client id, and checks the nonce it issued. A Google account opens the account it is linked to; a new one creates
// an account. An address already used by another account is never linked automatically: the person signs in the
// usual way and links Google from their settings.

// GoogleVerifier checks a Google ID token (google.Verifier).
type GoogleVerifier interface {
	ClientID() string
	Verify(ctx context.Context, raw string) (*google.Identity, error)
}

var (
	// ErrGoogleUnavailable: sign-in with Google is not configured on this server.
	ErrGoogleUnavailable = errors.New("sign-in with Google is not configured")
	// ErrGoogleToken: the Google credential or its nonce is not valid.
	ErrGoogleToken = errors.New("the Google sign-in could not be verified")
	// ErrAccountExists: another Trezutil account already uses this Google account's email; it is not linked
	// automatically (the person signs in the usual way, then links Google).
	ErrAccountExists = errors.New("an account already uses this email address")
	// ErrGoogleLinkedElsewhere: this Google account already opens another Trezutil account.
	ErrGoogleLinkedElsewhere = errors.New("this Google account is linked to another account")
	// ErrLastSignInMethod: unlinking would leave the account with no way to sign in.
	ErrLastSignInMethod = errors.New("this is the account's only way to sign in")
)

// GoogleNonceTTL is how long a nonce from GoogleNonce may be used.
const GoogleNonceTTL = 10 * time.Minute

// GoogleClientID returns the public OAuth client id the browser needs, or "" when Google sign-in is off.
func (s *Service) GoogleClientID() string {
	if s.cfg.Google == nil || s.cfg.Identities == nil {
		return ""
	}
	return s.cfg.Google.ClientID()
}

// GoogleNonce issues a nonce for one sign-in: random, timestamped and signed with the server's code secret, so it
// cannot be forged and expires. The browser passes it to Google, which puts it in the ID token.
func (s *Service) GoogleNonce() (string, int64, error) {
	if s.GoogleClientID() == "" {
		return "", 0, ErrGoogleUnavailable
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", 0, err
	}
	expires := s.cfg.Clock() + int64(GoogleNonceTTL/time.Second)
	body := base64.RawURLEncoding.EncodeToString(buf) + "." + strconv.FormatInt(expires, 10)
	return body + "." + s.nonceMAC(body), expires, nil
}

func (s *Service) nonceMAC(body string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.CodeSecret))
	mac.Write([]byte("google-nonce:" + body))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (s *Service) checkNonce(nonce string) bool {
	i := strings.LastIndex(nonce, ".")
	if i <= 0 {
		return false
	}
	body, sig := nonce[:i], nonce[i+1:]
	if !hmac.Equal([]byte(sig), []byte(s.nonceMAC(body))) {
		return false
	}
	j := strings.LastIndex(body, ".")
	if j <= 0 {
		return false
	}
	expires, err := strconv.ParseInt(body[j+1:], 10, 64)
	return err == nil && s.cfg.Clock() <= expires
}

// verifyGoogle checks the credential and that its nonce is the one the browser was given (and still valid).
func (s *Service) verifyGoogle(ctx context.Context, credential, nonce string) (*google.Identity, error) {
	if s.GoogleClientID() == "" {
		return nil, ErrGoogleUnavailable
	}
	id, err := s.cfg.Google.Verify(ctx, credential)
	if err != nil {
		s.cfg.Logger.Warn().Err(err).Msg("Google sign-in refused")
		return nil, ErrGoogleToken
	}
	if nonce == "" || id.Nonce != nonce || !s.checkNonce(nonce) {
		return nil, ErrGoogleToken
	}
	return id, nil
}

func displayNameFrom(id *google.Identity) string {
	name := strings.Join(strings.Fields(id.Name), " ")
	if name == "" && id.EmailVerified {
		name = strings.SplitN(id.Email, "@", 2)[0]
	}
	if name == "" {
		name = defaultCustomerNameMask
	}
	if utf8.RuneCountInString(name) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}

// SignInWithGoogle signs in with a Google ID token, creating a customer account the first time. The account is
// found by the Google account's stable id, never by email. A new account takes the email only when Google says it
// is verified; if another account already has that email, nothing is created or linked (ErrAccountExists).
func (s *Service) SignInWithGoogle(ctx context.Context, credential, nonce string) (*Session, error) {
	id, err := s.verifyGoogle(ctx, credential, nonce)
	if err != nil {
		return nil, err
	}
	userID, err := s.cfg.Identities.FindUserID(ctx, domain.IdentityProviderGoogle, id.Subject)
	if err != nil {
		return nil, err
	}
	if userID != "" {
		user, err := s.cfg.Users.FindByID(ctx, userID)
		if err != nil {
			return nil, domain.ErrUserNotFound
		}
		if user.IsSuspended() {
			return nil, domain.ErrUserSuspended
		}
		_ = s.cfg.Identities.Touch(ctx, domain.IdentityProviderGoogle, id.Subject, verifiedEmail(id), id.Name)
		return s.issueSession(user)
	}

	email := verifiedEmail(id)
	if email != "" {
		if s.emailTaken(ctx, email) {
			return nil, ErrAccountExists
		}
	}
	now := s.cfg.Clock()
	user := domain.NewUser("", displayNameFrom(id))
	user.Status = domain.UserStatusActive
	user.VerifiedAt = &now
	user.CreatedAt, user.UpdatedAt = now, now
	if email != "" {
		user.Email = &email
	}
	if err := s.cfg.Users.Create(ctx, user); err != nil {
		// Lost a race with the same email (unique index): never link it automatically.
		if email != "" && s.emailTaken(ctx, email) {
			return nil, ErrAccountExists
		}
		return nil, fmt.Errorf("create account: %w", err)
	}
	if err := s.cfg.Users.AssignRole(ctx, user.ID, domain.RoleCustomer); err != nil && !errors.Is(err, domain.ErrRoleAlreadyAssigned) {
		return nil, fmt.Errorf("assign customer role: %w", err)
	}
	if err := s.cfg.Identities.Link(ctx, &domain.ExternalIdentity{UserID: user.ID, Provider: domain.IdentityProviderGoogle, Subject: id.Subject, Email: email, Name: id.Name}); err != nil {
		if errors.Is(err, domain.ErrIdentityTaken) {
			// The same Google account was signed in twice at once: use the account the other request created.
			if other, ferr := s.cfg.Identities.FindUserID(ctx, domain.IdentityProviderGoogle, id.Subject); ferr == nil && other != "" {
				if u, uerr := s.cfg.Users.FindByID(ctx, other); uerr == nil {
					return s.issueSession(u)
				}
			}
		}
		return nil, fmt.Errorf("link Google account: %w", err)
	}
	if reloaded, err := s.cfg.Users.FindByID(ctx, user.ID); err == nil {
		user = reloaded
	}
	s.cfg.Logger.Info().Str("user_id", user.ID).Msg("account created with Google")
	return s.issueSession(user)
}

// emailTaken reports whether an account already uses the address (case-insensitive). When it cannot be checked the
// answer is "taken", so nothing is created or linked on a guess.
func (s *Service) emailTaken(ctx context.Context, email string) bool {
	if s.cfg.Credentials == nil {
		return true
	}
	u, err := s.cfg.Credentials.FindUserByEmail(ctx, email)
	if err != nil {
		return !errors.Is(err, domain.ErrUserNotFound)
	}
	return u != nil
}

func verifiedEmail(id *google.Identity) string {
	if id.EmailVerified && strings.Contains(id.Email, "@") && len(id.Email) <= 255 {
		return id.Email
	}
	return ""
}

// LinkGoogle links a Google account to the signed-in account (proof of both: the session and a fresh Google sign-in).
// The account's email is filled in from Google when it has none and no other account uses it.
func (s *Service) LinkGoogle(ctx context.Context, userID, credential, nonce string) (*domain.ExternalIdentity, error) {
	id, err := s.verifyGoogle(ctx, credential, nonce)
	if err != nil {
		return nil, err
	}
	user, err := s.cfg.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	if user.IsSuspended() {
		return nil, domain.ErrUserSuspended
	}
	owner, err := s.cfg.Identities.FindUserID(ctx, domain.IdentityProviderGoogle, id.Subject)
	if err != nil {
		return nil, err
	}
	if owner != "" && owner != userID {
		return nil, ErrGoogleLinkedElsewhere
	}
	email := verifiedEmail(id)
	link := &domain.ExternalIdentity{UserID: userID, Provider: domain.IdentityProviderGoogle, Subject: id.Subject, Email: email, Name: id.Name, CreatedAt: s.cfg.Clock()}
	if owner == "" {
		if err := s.cfg.Identities.Link(ctx, link); err != nil {
			if errors.Is(err, domain.ErrIdentityTaken) {
				return nil, ErrGoogleLinkedElsewhere // this account already has another Google account
			}
			return nil, err
		}
	}
	if user.Email == nil && email != "" {
		if !s.emailTaken(ctx, email) {
			user.Email = &email
			user.UpdatedAt = s.cfg.Clock()
			_ = s.cfg.Users.Update(ctx, user)
		}
	}
	return link, nil
}

// UnlinkGoogle removes the Google link, unless it is the account's only way to sign in (no phone, no password).
func (s *Service) UnlinkGoogle(ctx context.Context, userID string) error {
	if s.cfg.Identities == nil {
		return ErrGoogleUnavailable
	}
	user, err := s.cfg.Users.FindByID(ctx, userID)
	if err != nil {
		return domain.ErrUserNotFound
	}
	hasPassword := false
	if s.cfg.Credentials != nil {
		if c, err := s.cfg.Credentials.GetCredential(ctx, userID); err == nil && c != nil && c.PasswordHash != "" {
			hasPassword = true
		}
	}
	if user.Phone == "" && !hasPassword {
		return ErrLastSignInMethod
	}
	return s.cfg.Identities.Unlink(ctx, userID, domain.IdentityProviderGoogle)
}

// Identities lists the sign-in methods linked to the account (besides the phone and the password).
func (s *Service) Identities(ctx context.Context, userID string) ([]domain.ExternalIdentity, error) {
	if s.cfg.Identities == nil {
		return []domain.ExternalIdentity{}, nil
	}
	return s.cfg.Identities.ListByUser(ctx, userID)
}
