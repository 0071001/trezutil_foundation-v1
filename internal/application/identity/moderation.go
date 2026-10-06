package identity

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/trezutil/trezutil/internal/domain"
)

// Moderation is the administrator's control over customer and seller accounts: suspend one (it can no longer
// sign in or refresh, and the platform's use cases refuse it at once), and give it back.
//
// Who may call it (platform administrators only) is decided by the HTTP layer; the rules that protect the
// platform from a mistake or a hijacked session are here: nobody moderates themselves, staff accounts are not
// moderated from the console, and every step names a reason and is written to the audit trail.
type Moderation struct {
	users    domain.UserRepository
	statuses domain.AccountStatusRepository
	audit    domain.AuditLogger
	now      func() time.Time
}

// NewModeration creates the moderation use cases. audit may be nil (nothing is recorded).
func NewModeration(users domain.UserRepository, statuses domain.AccountStatusRepository, audit domain.AuditLogger) *Moderation {
	return &Moderation{users: users, statuses: statuses, audit: audit, now: time.Now}
}

// Suspend suspends an active account. The reason (3 to 300 characters) goes to the audit trail.
func (m *Moderation) Suspend(ctx context.Context, actorID, targetID, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) < 3 {
		return domain.ErrReasonRequired
	}
	if utf8.RuneCountInString(reason) > domain.MaxModerationReason {
		return domain.ErrReasonRequired
	}
	return m.move(ctx, actorID, targetID, domain.UserStatusActive, domain.UserStatusSuspended, "user.suspended", reason)
}

// Reactivate gives a suspended account back. The reason is optional.
func (m *Moderation) Reactivate(ctx context.Context, actorID, targetID, reason string) error {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > domain.MaxModerationReason {
		return domain.ErrReasonRequired
	}
	return m.move(ctx, actorID, targetID, domain.UserStatusSuspended, domain.UserStatusActive, "user.reactivated", reason)
}

func (m *Moderation) move(ctx context.Context, actorID, targetID string, from, to domain.UserStatus, action, reason string) error {
	if actorID == "" {
		return domain.ErrInsufficientPermissions
	}
	if targetID == "" {
		return domain.ErrUserNotFound
	}
	if actorID == targetID {
		return domain.ErrCannotModerateSelf
	}
	target, err := m.users.FindByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target.IsStaff() {
		return domain.ErrCannotModerateStaff
	}
	if target.Status != from {
		return domain.ErrAccountState
	}
	if err := m.statuses.SetStatus(ctx, targetID, from, to, m.now().Unix()); err != nil {
		return err
	}
	if m.audit != nil {
		meta := map[string]interface{}{"previous_status": string(from), "new_status": string(to)}
		if reason != "" {
			meta["reason"] = reason
		}
		_ = m.audit.Record(ctx, domain.AuditEntry{ActorID: actorID, Action: action, TargetType: "user", TargetID: targetID, Metadata: meta})
	}
	return nil
}
