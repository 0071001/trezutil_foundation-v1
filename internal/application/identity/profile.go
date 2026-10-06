package identity

import (
	"context"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
)

// Profile edits by the account holder.
var (
	// ErrInvalidProfile: a name of 2 to 80 characters, an email address that parses.
	ErrInvalidProfile = fmt.Errorf("the name must have 2 to 80 characters and the email must be valid")
	// ErrStaffEmail: a staff account's email is its sign-in identifier; it is changed with trezutil-admin.
	ErrStaffEmail = fmt.Errorf("a staff account's email is changed by the platform team")
	// ErrEmailTaken: another account uses that email address.
	ErrEmailTaken = fmt.Errorf("this email address is already used by another account")
)

// ProfileUpdate is what the account holder may change. A nil field is left as it is; an empty email removes it.
// The phone number is not here: changing it needs a new verification.
type ProfileUpdate struct {
	DisplayName *string
	Email       *string
}

// UpdateProfile changes the signed-in user's name and email.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileUpdate) (*domain.User, error) {
	user, err := s.cfg.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}
	if user.Status != domain.UserStatusActive {
		return nil, domain.ErrUserSuspended
	}
	if in.DisplayName != nil {
		name := strings.TrimSpace(*in.DisplayName)
		if n := utf8.RuneCountInString(name); n < 2 || n > 80 {
			return nil, ErrInvalidProfile
		}
		user.DisplayName = name
	}
	if in.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*in.Email))
		current := ""
		if user.Email != nil {
			current = strings.ToLower(*user.Email)
		}
		if email != current && user.HasAnyRole(domain.RoleAdmin, domain.RoleOperator, domain.RoleModerator) {
			return nil, ErrStaffEmail
		}
		switch {
		case email == "":
			user.Email = nil
		case len(email) > 254:
			return nil, ErrInvalidProfile
		default:
			if addr, err := mail.ParseAddress(email); err != nil || addr.Address != email {
				return nil, ErrInvalidProfile
			}
			user.Email = &email
		}
	}
	user.UpdatedAt = s.cfg.Clock()
	if err := s.cfg.Users.Update(ctx, user); err != nil {
		if strings.Contains(err.Error(), "uq_users_email_lower") || strings.Contains(err.Error(), "duplicate key") {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return user, nil
}
