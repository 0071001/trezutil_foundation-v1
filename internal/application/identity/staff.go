package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// maxPasswordAttempt bounds what a sign-in form may send, before any hashing.
const maxPasswordAttempt = 1024

// StaffLogin signs in a platform staff member (admin, operator, moderator) with
// an email address and a password.
//
// Every failure returns domain.ErrInvalidCredentials, whatever the cause: no
// such email, no password set, wrong password, locked account, suspended
// account, or an account without a staff role. Each path spends the time of a
// password check so the response time reveals nothing either. Wrong passwords
// are counted and lock the account after Config.MaxLoginFailures in a row.
func (s *Service) StaffLogin(ctx context.Context, rawEmail, password string) (*Session, error) {
	creds := s.cfg.Credentials
	email, emailErr := domain.NormalizeEmail(rawEmail)
	if creds == nil || emailErr != nil || password == "" || len(password) > maxPasswordAttempt {
		auth.BurnPasswordCheck(password)
		return nil, domain.ErrInvalidCredentials
	}
	log := s.cfg.Logger.With().Str("event", "staff_login").Logger()

	user, err := creds.FindUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			auth.BurnPasswordCheck(password)
			log.Warn().Str("result", "unknown_email").Msg("staff sign-in refused")
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}
	cred, err := creds.GetCredential(ctx, user.ID)
	if err != nil {
		if errors.Is(err, domain.ErrCredentialNotFound) || errors.Is(err, domain.ErrUserNotFound) {
			auth.BurnPasswordCheck(password)
			log.Warn().Str("user_id", user.ID).Str("result", "no_password").Msg("staff sign-in refused")
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	now := s.cfg.Clock()
	if cred.LockedUntil > now {
		auth.BurnPasswordCheck(password)
		log.Warn().Str("user_id", user.ID).Str("result", "locked").Msg("staff sign-in refused")
		return nil, domain.ErrInvalidCredentials
	}
	if !auth.VerifyPassword(cred.PasswordHash, password) {
		if rerr := creds.RecordLoginFailure(ctx, user.ID, now, s.cfg.MaxLoginFailures, int64(s.cfg.Lockout.Seconds())); rerr != nil {
			log.Error().Err(rerr).Str("user_id", user.ID).Msg("could not record a failed sign-in")
		}
		log.Warn().Str("user_id", user.ID).Str("result", "wrong_password").Msg("staff sign-in refused")
		return nil, domain.ErrInvalidCredentials
	}
	if !user.IsStaff() || user.Status != domain.UserStatusActive {
		log.Warn().Str("user_id", user.ID).Str("result", "not_allowed").Msg("staff sign-in refused")
		return nil, domain.ErrInvalidCredentials
	}
	if err := creds.RecordLoginSuccess(ctx, user.ID, now); err != nil {
		return nil, err
	}
	log.Info().Str("user_id", user.ID).Str("result", "ok").Msg("staff signed in")
	return s.issueSession(user)
}

// StaffInput describes a staff account to create or update.
type StaffInput struct {
	Email       string
	DisplayName string
	Password    string
	Roles       []domain.Role
}

// StaffResult reports what ProvisionStaff did.
type StaffResult struct {
	User    *domain.User
	Created bool
}

// ProvisionStaff creates a staff account, or, when the email already belongs to
// an account, sets its password and adds the missing staff roles. It is the
// operator-only way to get the first administrator (trezutil-admin); it is not
// exposed over HTTP.
func ProvisionStaff(ctx context.Context, users domain.UserRepository, creds domain.CredentialRepository, in StaffInput, now int64) (*StaffResult, error) {
	email, err := domain.NormalizeEmail(in.Email)
	if err != nil {
		return nil, err
	}
	if err := auth.ValidatePassword(in.Password, email); err != nil {
		return nil, err
	}
	if len(in.Roles) == 0 {
		return nil, errors.New("at least one staff role is required")
	}
	for _, r := range in.Roles {
		if !isStaffRole(r) {
			return nil, fmt.Errorf("role %q cannot sign in with a password (staff roles: admin, operator, moderator)", r)
		}
	}
	name := strings.TrimSpace(in.DisplayName)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return nil, err
	}

	user, err := creds.FindUserByEmail(ctx, email)
	created := false
	switch {
	case err == nil:
		if user.IsSuspended() {
			return nil, domain.ErrUserSuspended
		}
	case errors.Is(err, domain.ErrUserNotFound):
		user = domain.NewUser("", name)
		user.Email = &email
		user.Status = domain.UserStatusActive
		user.VerifiedAt = &now
		user.CreatedAt, user.UpdatedAt = now, now
		if err := users.Create(ctx, user); err != nil {
			return nil, err
		}
		created = true
	default:
		return nil, err
	}

	for _, r := range in.Roles {
		if rerr := users.AssignRole(ctx, user.ID, r); rerr != nil && !errors.Is(rerr, domain.ErrRoleAlreadyAssigned) {
			return nil, fmt.Errorf("assign role %s: %w", r, rerr)
		}
	}
	if err := creds.SetPassword(ctx, user.ID, hash, now); err != nil {
		return nil, err
	}
	reloaded, err := users.FindByID(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	return &StaffResult{User: reloaded, Created: created}, nil
}

func isStaffRole(r domain.Role) bool {
	for _, s := range domain.StaffRoles {
		if s == r {
			return true
		}
	}
	return false
}
