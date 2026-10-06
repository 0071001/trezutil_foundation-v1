package domain

import (
	"context"
	"fmt"
)

// Account moderation: platform administrators can suspend an account and give it back.
var (
	// ErrCannotModerateSelf: nobody suspends their own account from the console.
	ErrCannotModerateSelf = fmt.Errorf("you cannot suspend or reactivate your own account")
	// ErrCannotModerateStaff: staff accounts (admin, operator, moderator) are not moderated from the console;
	// they are managed with the trezutil-admin command, so one compromised session cannot lock the team out.
	ErrCannotModerateStaff = fmt.Errorf("staff accounts are managed with the trezutil-admin command")
	// ErrAccountState: the account is not in a state the step starts from (already suspended, not suspended),
	// or somebody changed it in between.
	ErrAccountState = fmt.Errorf("the account is not in a state that allows this step")
	// ErrReasonRequired: a suspension is always given a reason of 3 to MaxModerationReason characters, which goes
	// to the audit trail (a reactivation's reason is optional but bounded the same way).
	ErrReasonRequired = fmt.Errorf("a reason of 3 to 300 characters is required")
)

// MaxModerationReason bounds the reason that accompanies a suspension.
const MaxModerationReason = 300

// AccountStatusRepository changes the status of an account only from the status the caller saw, so two
// administrators acting at once cannot both succeed.
type AccountStatusRepository interface {
	// SetStatus moves the account from one status to another. It returns ErrAccountState when the account is
	// not in the from status (or does not exist).
	SetStatus(ctx context.Context, userID string, from, to UserStatus, now int64) error
}
