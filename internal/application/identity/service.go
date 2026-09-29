// Package identity implements phone-OTP sign-in and JWT session issuance.
package identity

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// Defaults for OTP issuance.
const (
	DefaultCodeTTL          = 5 * time.Minute
	DefaultMaxAttempts      = 5
	DefaultMaxPerPhoneHour  = 5
	codeDigits              = 6
	defaultCustomerNameMask = "Trezutil user"
)

// Session is the result of a successful sign-in or refresh.
type Session struct {
	User         *domain.User
	AccessToken  string
	RefreshToken string
	ExpiresAt    int64
}

// Clock returns the current Unix time.
type Clock func() int64

// Config configures the identity Service.
type Config struct {
	Users      domain.UserRepository
	Challenges domain.OTPChallengeRepository
	Sender     domain.OTPProvider
	Tokens     domain.TokenService
	// CodeSecret keys the HMAC used to store OTP codes. It must be secret.
	CodeSecret     string
	AccessTokenTTL time.Duration
	CodeTTL        time.Duration
	MaxAttempts    int
	MaxPerHour     int
	Clock          Clock
	Logger         zerolog.Logger
}

// Service issues and verifies OTP challenges and manages sessions.
type Service struct {
	cfg Config
}

// NewService creates a new identity Service.
func NewService(cfg Config) (*Service, error) {
	if cfg.Users == nil || cfg.Challenges == nil || cfg.Sender == nil || cfg.Tokens == nil {
		return nil, errors.New("identity: users, challenges, sender and tokens are required")
	}
	if len(cfg.CodeSecret) < 16 {
		return nil, errors.New("identity: code secret must be at least 16 characters")
	}
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = DefaultCodeTTL
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = DefaultMaxAttempts
	}
	if cfg.MaxPerHour <= 0 {
		cfg.MaxPerHour = DefaultMaxPerPhoneHour
	}
	if cfg.AccessTokenTTL <= 0 {
		cfg.AccessTokenTTL = 15 * time.Minute
	}
	if cfg.Clock == nil {
		cfg.Clock = func() int64 { return time.Now().Unix() }
	}
	return &Service{cfg: cfg}, nil
}

// RequestOTP issues a new challenge for phone and delivers the code.
// It returns the challenge ID the client must echo back on verification.
func (s *Service) RequestOTP(ctx context.Context, rawPhone string) (challengeID string, expiresAt int64, err error) {
	phone, err := domain.NormalizePhone(rawPhone)
	if err != nil {
		return "", 0, domain.ErrInvalidPhone
	}
	now := s.cfg.Clock()

	recent, err := s.cfg.Challenges.CountSince(ctx, phone, now-3600)
	if err != nil {
		return "", 0, err
	}
	if recent >= s.cfg.MaxPerHour {
		return "", 0, domain.ErrOTPRateLimited
	}

	code, err := randomDigits(codeDigits)
	if err != nil {
		return "", 0, fmt.Errorf("generate otp: %w", err)
	}

	challenge := &domain.OTPChallenge{
		ID:          domain.NewID(),
		Phone:       phone,
		MaxAttempts: s.cfg.MaxAttempts,
		ExpiresAt:   now + int64(s.cfg.CodeTTL/time.Second),
		CreatedAt:   now,
	}
	challenge.CodeHash = s.hashCode(challenge.ID, code)

	if err := s.cfg.Challenges.Create(ctx, challenge); err != nil {
		return "", 0, err
	}
	if err := s.cfg.Sender.Send(ctx, phone, code); err != nil {
		s.cfg.Logger.Error().Err(err).Str("challenge_id", challenge.ID).Msg("OTP delivery failed")
		return "", 0, fmt.Errorf("otp delivery failed: %w", err)
	}
	return challenge.ID, challenge.ExpiresAt, nil
}

// VerifyOTP checks the code for a challenge and, on success, signs the user in,
// creating the account on first sign-in.
func (s *Service) VerifyOTP(ctx context.Context, challengeID, code string) (*Session, error) {
	if challengeID == "" || code == "" {
		return nil, domain.ErrInvalidOTP
	}
	now := s.cfg.Clock()

	challenge, err := s.cfg.Challenges.FindByID(ctx, challengeID)
	if err != nil {
		return nil, domain.ErrInvalidOTP
	}
	// Count the attempt before comparing so brute force is bounded even under
	// concurrent requests.
	ok, err := s.cfg.Challenges.RegisterAttempt(ctx, challenge.ID, now)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, domain.ErrInvalidOTP
	}
	expected := s.hashCode(challenge.ID, code)
	if !hmac.Equal([]byte(expected), []byte(challenge.CodeHash)) {
		return nil, domain.ErrInvalidOTP
	}
	consumed, err := s.cfg.Challenges.Consume(ctx, challenge.ID, now)
	if err != nil {
		return nil, err
	}
	if !consumed {
		return nil, domain.ErrInvalidOTP
	}

	user, err := s.FindOrCreateVerifiedUser(ctx, challenge.Phone)
	if err != nil {
		return nil, err
	}
	return s.issueSession(user)
}

// Refresh exchanges a refresh token for a new session.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (*Session, error) {
	userID, err := s.cfg.Tokens.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidOTP
	}
	user, err := s.cfg.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	if user.IsSuspended() {
		return nil, domain.ErrUserSuspended
	}
	return s.issueSession(user)
}

// Me returns the authenticated user.
func (s *Service) Me(ctx context.Context, userID string) (*domain.User, error) {
	user, err := s.cfg.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	return user, nil
}

// FindOrCreateVerifiedUser returns the active user owning phone, creating a
// customer account when none exists. The phone must already be verified by the
// caller (OTP check, or a signed WhatsApp webhook).
func (s *Service) FindOrCreateVerifiedUser(ctx context.Context, phone string) (*domain.User, error) {
	return FindOrCreateVerifiedUser(ctx, s.cfg.Users, phone, s.cfg.Clock())
}

// FindOrCreateVerifiedUser is shared by OTP sign-in and WhatsApp sender
// provisioning so both paths produce identical accounts.
func FindOrCreateVerifiedUser(ctx context.Context, users domain.UserRepository, phone string, now int64) (*domain.User, error) {
	normalized, err := domain.NormalizePhone(phone)
	if err != nil {
		return nil, domain.ErrInvalidPhone
	}

	user, err := users.FindByPhone(ctx, normalized)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		return nil, err
	}
	if user == nil {
		user = domain.NewUser(normalized, defaultCustomerNameMask)
		user.Status = domain.UserStatusActive
		user.VerifiedAt = &now
		user.CreatedAt, user.UpdatedAt = now, now
		if cerr := users.Create(ctx, user); cerr != nil {
			// Lost a race with a concurrent sign-in for the same phone.
			existing, ferr := users.FindByPhone(ctx, normalized)
			if ferr != nil {
				return nil, cerr
			}
			user = existing
		} else if rerr := users.AssignRole(ctx, user.ID, domain.RoleCustomer); rerr != nil && !errors.Is(rerr, domain.ErrRoleAlreadyAssigned) {
			return nil, fmt.Errorf("assign customer role: %w", rerr)
		}
		// Reload so roles are populated.
		if reloaded, rerr := users.FindByID(ctx, user.ID); rerr == nil {
			user = reloaded
		}
	}

	if user.IsSuspended() {
		return nil, domain.ErrUserSuspended
	}
	if user.Status != domain.UserStatusActive || user.VerifiedAt == nil {
		user.Status = domain.UserStatusActive
		if user.VerifiedAt == nil {
			user.VerifiedAt = &now
		}
		user.UpdatedAt = now
		if err := users.Update(ctx, user); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (s *Service) issueSession(user *domain.User) (*Session, error) {
	access, err := s.cfg.Tokens.IssueAccessToken(user.ID, user.Roles)
	if err != nil {
		return nil, err
	}
	refresh, err := s.cfg.Tokens.IssueRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}
	return &Session{
		User:         user,
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    s.cfg.Clock() + int64(s.cfg.AccessTokenTTL/time.Second),
	}, nil
}

func (s *Service) hashCode(challengeID, code string) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.CodeSecret))
	mac.Write([]byte(challengeID))
	mac.Write([]byte{':'})
	mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func randomDigits(n int) (string, error) {
	out := make([]byte, n)
	for i := range out {
		d, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		out[i] = byte('0' + d.Int64())
	}
	return string(out), nil
}
