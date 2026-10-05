package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
)

// A staff account has an email and no phone; two of them can coexist, the email
// is unique ignoring case, and the password counters survive in the database.
func TestStaffCredentials(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()
	users := postgres.NewUserRepository(testDB.DB)
	ctx := context.Background()
	now := time.Now().Unix()

	res, err := identity.ProvisionStaff(ctx, users, users, identity.StaffInput{
		Email: "Staff.One@Example.com", DisplayName: "Staff One", Password: "Un-Mot-De-Passe-1", Roles: []domain.Role{domain.RoleAdmin},
	}, now)
	if err != nil || !res.Created {
		t.Fatalf("provision: created=%v err=%v", res != nil && res.Created, err)
	}
	if res.User.Phone != "" || res.User.Email == nil || *res.User.Email != "staff.one@example.com" || !res.User.HasRole(domain.RoleAdmin) {
		t.Fatalf("unexpected staff account: %+v", res.User)
	}

	// A second staff account without a phone does not collide with the first.
	second, err := identity.ProvisionStaff(ctx, users, users, identity.StaffInput{Email: "second@example.com", Password: "Deuxieme-Mot-2026", Roles: []domain.Role{domain.RoleOperator}}, now)
	if err != nil || !second.Created || second.User.ID == res.User.ID {
		t.Fatalf("second staff account: %v", err)
	}

	// The email is unique ignoring case.
	dup := domain.NewUser("", "Duplicate")
	dup.Email = strPtr("STAFF.ONE@EXAMPLE.COM")
	if err := users.Create(ctx, dup); err == nil {
		t.Fatal("an email that differs only by case must be refused")
	}

	// Lookup ignores case and loads the roles.
	found, err := users.FindUserByEmail(ctx, "STAFF.one@example.COM")
	if err != nil || found.ID != res.User.ID || !found.IsStaff() {
		t.Fatalf("FindUserByEmail: %v %+v", err, found)
	}
	if _, err := users.FindUserByEmail(ctx, "nobody@example.com"); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("unknown email: %v", err)
	}

	// The stored password is a hash.
	cred, err := users.GetCredential(ctx, res.User.ID)
	if err != nil || cred.PasswordHash == "" || cred.PasswordHash == "Un-Mot-De-Passe-1" || cred.FailedAttempts != 0 || cred.LockedUntil != 0 {
		t.Fatalf("credential: %v %+v", err, cred)
	}
	// An account without a password has no credential.
	customer := domain.NewUser("+243810000555", "Client")
	if err := users.Create(ctx, customer); err != nil {
		t.Fatal(err)
	}
	if _, err := users.GetCredential(ctx, customer.ID); !errors.Is(err, domain.ErrCredentialNotFound) {
		t.Fatalf("customer credential: %v", err)
	}

	// Failures count under a lock, then lock the account, and an expired lock starts a fresh count.
	for i := 1; i <= 3; i++ {
		if err := users.RecordLoginFailure(ctx, res.User.ID, now, 3, 900); err != nil {
			t.Fatal(err)
		}
	}
	cred, _ = users.GetCredential(ctx, res.User.ID)
	if cred.FailedAttempts != 3 || cred.LockedUntil != now+900 {
		t.Fatalf("after 3 failures: %+v", cred)
	}
	if err := users.RecordLoginFailure(ctx, res.User.ID, now+901, 3, 900); err != nil {
		t.Fatal(err)
	}
	cred, _ = users.GetCredential(ctx, res.User.ID)
	if cred.FailedAttempts != 1 || cred.LockedUntil != 0 {
		t.Fatalf("a failure after an expired lock restarts the count: %+v", cred)
	}
	if err := users.RecordLoginSuccess(ctx, res.User.ID, now+1000); err != nil {
		t.Fatal(err)
	}
	cred, _ = users.GetCredential(ctx, res.User.ID)
	if cred.FailedAttempts != 0 || cred.LockedUntil != 0 {
		t.Fatalf("a success clears the counters: %+v", cred)
	}
	// Setting a password also clears a lock.
	_ = users.RecordLoginFailure(ctx, res.User.ID, now, 1, 900)
	if err := users.SetPassword(ctx, res.User.ID, "pbkdf2-sha256$1$AA$AA", now); err != nil {
		t.Fatal(err)
	}
	cred, _ = users.GetCredential(ctx, res.User.ID)
	if cred.LockedUntil != 0 || cred.FailedAttempts != 0 {
		t.Fatalf("SetPassword must clear the lock: %+v", cred)
	}
	if err := users.SetPassword(ctx, "00000000-0000-0000-0000-000000000000", "x", now); !errors.Is(err, domain.ErrUserNotFound) {
		t.Fatalf("SetPassword on a missing user: %v", err)
	}
}

// The administration queries count and list real rows, and a search word is
// matched literally (LIKE wildcards are escaped).
func TestAdminRepository(t *testing.T) {
	testDB := NewTestDB(t)
	defer testDB.Close()
	users := postgres.NewUserRepository(testDB.DB)
	admin := postgres.NewAdminRepository(testDB.DB)
	ctx := context.Background()
	now := time.Now().Unix()

	if _, err := identity.ProvisionStaff(ctx, users, users, identity.StaffInput{Email: "boss@example.com", DisplayName: "Boss", Password: "Le-Patron-2026", Roles: []domain.Role{domain.RoleAdmin}}, now); err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Marie 100%", "Marie_Claire", "Jean"} {
		u := domain.NewUser("+24381000070"+string(rune('0'+i)), name)
		u.Status = domain.UserStatusActive
		if err := users.Create(ctx, u); err != nil {
			t.Fatal(err)
		}
		_ = users.AssignRole(ctx, u.ID, domain.RoleCustomer)
	}

	o, err := admin.Overview(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if o.UsersTotal != 4 || o.UsersNew7d != 4 || o.UsersByRole["admin"] != 1 || o.UsersByRole["customer"] != 3 {
		t.Fatalf("overview: %+v", o)
	}

	all, total, err := admin.ListUsers(ctx, "", 2, 0)
	if err != nil || total != 4 || len(all) != 2 {
		t.Fatalf("page 1: total=%d len=%d err=%v", total, len(all), err)
	}
	page2, _, _ := admin.ListUsers(ctx, "", 2, 2)
	if len(page2) != 2 || page2[0].ID == all[0].ID {
		t.Fatalf("page 2 must continue the list: %+v", page2)
	}

	byEmail, total, _ := admin.ListUsers(ctx, "BOSS@", 25, 0)
	if total != 1 || byEmail[0].Email != "boss@example.com" || len(byEmail[0].Roles) != 1 || byEmail[0].Roles[0] != "admin" || byEmail[0].Phone != "" {
		t.Fatalf("search by email: %+v", byEmail)
	}
	// "%" and "_" are literal characters, not wildcards.
	pct, total, _ := admin.ListUsers(ctx, "100%", 25, 0)
	if total != 1 || pct[0].DisplayName != "Marie 100%" {
		t.Fatalf("a percent sign matches literally: total=%d %+v", total, pct)
	}
	under, total, _ := admin.ListUsers(ctx, "Marie_C", 25, 0)
	if total != 1 || under[0].DisplayName != "Marie_Claire" {
		t.Fatalf("an underscore matches literally: total=%d %+v", total, under)
	}
	if _, total, _ := admin.ListUsers(ctx, "%", 25, 0); total != 1 {
		t.Fatalf("a bare percent sign matches only names that contain one, got %d", total)
	}
}
