package identity

import (
	"context"
	"errors"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type moderationUsers struct {
	domain.UserRepository
	byID map[string]*domain.User
}

func (m moderationUsers) FindByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := m.byID[id]; ok {
		return u, nil
	}
	return nil, domain.ErrUserNotFound
}

// statusStore changes the status in the same map, and refuses like the real repository when the status moved.
type statusStore struct{ users map[string]*domain.User }

func (s statusStore) SetStatus(_ context.Context, id string, from, to domain.UserStatus, _ int64) error {
	u, ok := s.users[id]
	if !ok || u.Status != from {
		return domain.ErrAccountState
	}
	u.Status = to
	return nil
}

type auditTrail struct{ entries []domain.AuditEntry }

func (a *auditTrail) Record(_ context.Context, e domain.AuditEntry) error {
	a.entries = append(a.entries, e)
	return nil
}

func newModeration() (*Moderation, map[string]*domain.User, *auditTrail) {
	users := map[string]*domain.User{
		"admin":     {ID: "admin", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleAdmin}},
		"operator":  {ID: "operator", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleOperator}},
		"customer":  {ID: "customer", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleCustomer}},
		"seller":    {ID: "seller", Status: domain.UserStatusActive, Roles: []domain.Role{domain.RoleSeller}},
		"suspended": {ID: "suspended", Status: domain.UserStatusSuspended, Roles: []domain.Role{domain.RoleCustomer}},
	}
	trail := &auditTrail{}
	return NewModeration(moderationUsers{byID: users}, statusStore{users}, trail), users, trail
}

func TestModeration_SuspendThenReactivateAnAccount(t *testing.T) {
	m, users, trail := newModeration()
	ctx := context.Background()

	if err := m.Suspend(ctx, "admin", "customer", "  repeated chargebacks  "); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if users["customer"].Status != domain.UserStatusSuspended {
		t.Fatalf("status = %s", users["customer"].Status)
	}
	if err := m.Reactivate(ctx, "admin", "customer", ""); err != nil {
		t.Fatalf("reactivate: %v", err)
	}
	if users["customer"].Status != domain.UserStatusActive {
		t.Fatalf("status = %s", users["customer"].Status)
	}

	if len(trail.entries) != 2 {
		t.Fatalf("audit entries = %d, want 2", len(trail.entries))
	}
	s := trail.entries[0]
	if s.Action != "user.suspended" || s.ActorID != "admin" || s.TargetType != "user" || s.TargetID != "customer" {
		t.Fatalf("suspension entry = %+v", s)
	}
	if s.Metadata["reason"] != "repeated chargebacks" || s.Metadata["previous_status"] != "active" || s.Metadata["new_status"] != "suspended" {
		t.Fatalf("suspension metadata = %+v", s.Metadata)
	}
	if r := trail.entries[1]; r.Action != "user.reactivated" || r.Metadata["reason"] != nil {
		t.Fatalf("reactivation entry = %+v", r)
	}
}

func TestModeration_RefusesWhatWouldHurtThePlatform(t *testing.T) {
	m, users, trail := newModeration()
	ctx := context.Background()
	cases := []struct {
		name string
		do   func() error
		want error
	}{
		{"suspending yourself", func() error { return m.Suspend(ctx, "admin", "admin", "tired of it") }, domain.ErrCannotModerateSelf},
		{"reactivating yourself", func() error { return m.Reactivate(ctx, "admin", "admin", "") }, domain.ErrCannotModerateSelf},
		{"suspending another administrator", func() error { return m.Suspend(ctx, "admin", "operator", "a reason") }, domain.ErrCannotModerateStaff},
		{"suspending without a reason", func() error { return m.Suspend(ctx, "admin", "customer", "  ") }, domain.ErrReasonRequired},
		{"suspending with a one-word reason of two letters", func() error { return m.Suspend(ctx, "admin", "customer", "no") }, domain.ErrReasonRequired},
		{"suspending with an essay for a reason", func() error {
			return m.Suspend(ctx, "admin", "customer", string(make([]rune, domain.MaxModerationReason+1)))
		}, domain.ErrReasonRequired},
		{"suspending twice", func() error { return m.Suspend(ctx, "admin", "suspended", "again") }, domain.ErrAccountState},
		{"reactivating an account that is not suspended", func() error { return m.Reactivate(ctx, "admin", "seller", "") }, domain.ErrAccountState},
		{"an unknown account", func() error { return m.Suspend(ctx, "admin", "nobody", "a reason") }, domain.ErrUserNotFound},
		{"nobody at the controls", func() error { return m.Suspend(ctx, "", "customer", "a reason") }, domain.ErrInsufficientPermissions},
	}
	for _, c := range cases {
		err := c.do()
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
	if users["customer"].Status != domain.UserStatusActive || users["operator"].Status != domain.UserStatusActive || users["admin"].Status != domain.UserStatusActive {
		t.Fatal("a refused step changed an account")
	}
	if len(trail.entries) != 0 {
		t.Fatalf("a refused step was audited: %+v", trail.entries)
	}
}

// Two administrators acting at once: the second finds the account no longer where it expected, and records nothing.
func TestModeration_ALosingConcurrentStepRecordsNothing(t *testing.T) {
	users := map[string]*domain.User{"customer": {ID: "customer", Status: domain.UserStatusActive}}
	trail := &auditTrail{}
	// the repository sees the account already suspended by someone else, though this caller read it as active
	m := NewModeration(moderationUsers{byID: users}, statusStore{map[string]*domain.User{"customer": {ID: "customer", Status: domain.UserStatusSuspended}}}, trail)
	if err := m.Suspend(context.Background(), "admin", "customer", "a reason"); !errors.Is(err, domain.ErrAccountState) {
		t.Fatalf("err = %v, want ErrAccountState", err)
	}
	if len(trail.entries) != 0 {
		t.Fatalf("audited a step that did not happen: %+v", trail.entries)
	}
}
