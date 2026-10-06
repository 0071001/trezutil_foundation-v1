package integration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/audit"
)

// The audit trail is a real table: entries are stored and read back with filters, and the database itself
// refuses to change or erase them.
func TestAuditStore_AppendsFiltersAndRefusesRewrites(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	const actor = "5e11e700-0000-4000-8000-0000000000e1"
	sellerUser(t, db, ctx, actor, "+243810000031")
	store := postgres.NewAuditRepository(db.DB)

	base := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC).Unix()
	entries := []domain.AuditEntry{
		{ActorID: actor, Action: "order.confirmed", TargetType: "order", TargetID: "o-1", OccurredAt: base, Metadata: map[string]interface{}{"new_status": "confirmed"}},
		{ActorID: actor, Action: "order.completed", TargetType: "order", TargetID: "o-1", OccurredAt: base + 3600},
		{ActorID: actor, Action: "listing.published", TargetType: "listing", TargetID: "l-9", OccurredAt: base + 7200},
		{ActorID: "", Action: "system.cleanup", OccurredAt: base + 10800}, // the system itself
		{ActorID: "a-person-who-was-removed", Action: "user.suspended", TargetType: "user", TargetID: "u-4", RequestID: "req-4", OccurredAt: base + 14400},
	}
	for _, e := range entries {
		if err := store.Append(ctx, e); err != nil {
			t.Fatalf("append %s: %v", e.Action, err)
		}
	}
	if err := store.Append(ctx, domain.AuditEntry{}); err == nil {
		t.Fatal("an entry without an action was stored")
	}

	all, total, err := store.List(ctx, domain.AuditFilter{}, 50, 0)
	if err != nil || total != 5 || len(all) != 5 {
		t.Fatalf("all: %d events, total %d, %v", len(all), total, err)
	}
	if all[0].Action != "user.suspended" || all[4].Action != "order.confirmed" {
		t.Fatalf("newest first: %s ... %s", all[0].Action, all[4].Action)
	}
	if all[4].ActorName == "" || all[4].Metadata["new_status"] != "confirmed" {
		t.Fatalf("the actor's name and the metadata come back: %+v", all[4])
	}
	if all[0].ActorName != "" || all[0].RequestID != "req-4" {
		t.Fatalf("a removed actor has no name: %+v", all[0])
	}

	count := func(f domain.AuditFilter) int {
		_, n, err := store.List(ctx, f, 50, 0)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	checks := map[string]struct {
		f    domain.AuditFilter
		want int
	}{
		"by action prefix":         {domain.AuditFilter{Action: "order."}, 2},
		"by full action":           {domain.AuditFilter{Action: "listing.published"}, 1},
		"a prefix is not a regex":  {domain.AuditFilter{Action: "o%"}, 0},
		"an underscore is literal": {domain.AuditFilter{Action: "order_"}, 0},
		"by actor":                 {domain.AuditFilter{ActorID: actor}, 3},
		"by target":                {domain.AuditFilter{TargetType: "order", TargetID: "o-1"}, 2},
		"from":                     {domain.AuditFilter{From: base + 7200}, 3},
		"to":                       {domain.AuditFilter{To: base + 3600}, 2},
		"a window":                 {domain.AuditFilter{From: base + 3600, To: base + 10800}, 3},
		"combined":                 {domain.AuditFilter{Action: "order.", ActorID: actor, From: base + 1}, 1},
	}
	for name, c := range checks {
		if got := count(c.f); got != c.want {
			t.Errorf("%s: %d events, want %d", name, got, c.want)
		}
	}
	page, total, err := store.List(ctx, domain.AuditFilter{}, 2, 2)
	if err != nil || total != 5 || len(page) != 2 || page[0].Action != "listing.published" {
		t.Fatalf("second page of two: %v total %d %v", page, total, err)
	}

	// the database itself refuses to rewrite or erase history
	if _, err := db.DB.ExecContext(ctx, `UPDATE audit_events SET action = 'order.nothing' WHERE target_id = 'o-1'`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("an event was rewritten: %v", err)
	}
	if _, err := db.DB.ExecContext(ctx, `DELETE FROM audit_events`); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("events were erased: %v", err)
	}
	if n := count(domain.AuditFilter{}); n != 5 {
		t.Fatalf("after the refused rewrites: %d events", n)
	}
}

// An oversized metadata document is replaced by a marker, never stored whole.
func TestAuditStore_BoundsMetadata(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	store := postgres.NewAuditRepository(db.DB)
	if err := store.Append(ctx, domain.AuditEntry{Action: "big.thing", Metadata: map[string]interface{}{"blob": strings.Repeat("x", 20<<10)}}); err != nil {
		t.Fatal(err)
	}
	events, _, err := store.List(ctx, domain.AuditFilter{Action: "big."}, 10, 0)
	if err != nil || len(events) != 1 || events[0].Metadata["truncated"] != true || events[0].Metadata["blob"] != nil {
		t.Fatalf("%+v %v", events, err)
	}
}

// What the services record through the audit logger lands in the table, and a store that fails never fails the action.
func TestAuditLogger_RecordsDurablyAndNeverBreaksTheAction(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	store := postgres.NewAuditRepository(db.DB)
	logger := audit.NewLogger(zerolog.Nop()).WithStore(store)

	if err := logger.Record(ctx, domain.AuditEntry{ActorID: "someone", Action: "order.created", TargetType: "order", TargetID: "o-77", RequestID: "req-77"}); err != nil {
		t.Fatal(err)
	}
	events, total, err := store.List(ctx, domain.AuditFilter{TargetID: "o-77"}, 10, 0)
	if err != nil || total != 1 || events[0].RequestID != "req-77" || events[0].ActorID != "someone" {
		t.Fatalf("%+v %v", events, err)
	}

	// a cancelled request must not lose its record
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := logger.Record(cancelled, domain.AuditEntry{Action: "order.cancelled", TargetType: "order", TargetID: "o-78"}); err != nil {
		t.Fatal(err)
	}
	if _, n, _ := store.List(ctx, domain.AuditFilter{TargetID: "o-78"}, 10, 0); n != 1 {
		t.Fatalf("a cancelled request lost its audit record (%d stored)", n)
	}

	// a broken store: the action still succeeds
	db.DB.Close()
	if err := logger.Record(ctx, domain.AuditEntry{Action: "order.created", TargetID: "o-79"}); err != nil {
		t.Fatalf("a store failure broke the action: %v", err)
	}
}

// SetStatus moves an account only from the status the caller saw, and the moderation use case on top of it
// refuses the steps that would hurt the platform, on the real schema.
func TestModeration_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	const admin, customer, other = "5e11e700-0000-4000-8000-0000000000f1", "5e11e700-0000-4000-8000-0000000000f2", "5e11e700-0000-4000-8000-0000000000f3"
	sellerUser(t, db, ctx, admin, "+243810000041")
	sellerUser(t, db, ctx, customer, "+243810000042")
	sellerUser(t, db, ctx, other, "+243810000043")
	users := postgres.NewUserRepository(db.DB)
	// the roles are seeded by the tests that need them; the administrator is staff, so nobody can suspend them here
	if _, err := db.DB.ExecContext(ctx, "INSERT INTO roles (id, name, description, created_at) VALUES ($1, $2, $3, $4)",
		"5e11e700-0000-4000-8000-0000000000a1", string(domain.RoleAdmin), "Administrator", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := users.AssignRole(ctx, admin, domain.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	store := postgres.NewAuditRepository(db.DB)
	m := identity.NewModeration(users, users, audit.NewLogger(zerolog.Nop()).WithStore(store))

	// two administrators at once: exactly one wins
	if err := users.SetStatus(ctx, customer, domain.UserStatusActive, domain.UserStatusSuspended, time.Now().Unix()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := users.SetStatus(ctx, customer, domain.UserStatusActive, domain.UserStatusSuspended, time.Now().Unix()); !errors.Is(err, domain.ErrAccountState) {
		t.Fatalf("second: %v", err)
	}
	if err := users.SetStatus(ctx, "5e11e700-0000-4000-8000-0000000000ff", domain.UserStatusActive, domain.UserStatusSuspended, time.Now().Unix()); !errors.Is(err, domain.ErrAccountState) {
		t.Fatalf("an account that does not exist: %v", err)
	}

	if err := m.Reactivate(ctx, admin, customer, "appeal accepted"); err != nil {
		t.Fatal(err)
	}
	if u, _ := users.FindByID(ctx, customer); u.Status != domain.UserStatusActive {
		t.Fatalf("customer is %s", u.Status)
	}
	if err := m.Suspend(ctx, admin, other, "fraud report #12"); err != nil {
		t.Fatal(err)
	}
	if u, _ := users.FindByID(ctx, other); !u.IsSuspended() {
		t.Fatalf("other is %s", u.Status)
	}
	if err := m.Suspend(ctx, admin, admin, "oops"); !errors.Is(err, domain.ErrCannotModerateSelf) {
		t.Fatalf("self: %v", err)
	}
	if err := m.Suspend(ctx, customer, admin, "a customer cannot reach this, but the rule holds anyway"); !errors.Is(err, domain.ErrCannotModerateStaff) {
		t.Fatalf("staff: %v", err)
	}
	if u, _ := users.FindByID(ctx, admin); u.Status != domain.UserStatusActive {
		t.Fatalf("the administrator is %s", u.Status)
	}

	// both steps are in the audit trail, with who, whom and why
	events, total, err := store.List(ctx, domain.AuditFilter{Action: "user."}, 10, 0)
	if err != nil || total != 2 {
		t.Fatalf("%d events %v", total, err)
	}
	if e := events[0]; e.Action != "user.suspended" || e.TargetID != other || e.ActorID != admin || e.Metadata["reason"] != "fraud report #12" {
		t.Fatalf("suspension: %+v", e)
	}
	if e := events[1]; e.Action != "user.reactivated" || e.TargetID != customer || e.Metadata["reason"] != "appeal accepted" {
		t.Fatalf("reactivation: %+v", e)
	}
}
