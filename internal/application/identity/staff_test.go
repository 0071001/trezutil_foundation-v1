package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

// memUsers is an in-memory user and credential store.
type memUsers struct {
	users    map[string]*domain.User
	hashes   map[string]string
	attempts map[string]int
	locked   map[string]int64
	lastIn   map[string]int64
	nextID   int
}

func newMemUsers() *memUsers {
	return &memUsers{users: map[string]*domain.User{}, hashes: map[string]string{}, attempts: map[string]int{}, locked: map[string]int64{}, lastIn: map[string]int64{}}
}

func (m *memUsers) FindByID(_ context.Context, id string) (*domain.User, error) {
	if u, ok := m.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, domain.ErrUserNotFound
}
func (m *memUsers) FindByPhone(_ context.Context, phone string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Phone != "" && u.Phone == phone {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (m *memUsers) Create(_ context.Context, u *domain.User) error {
	if u.Email != nil {
		for _, o := range m.users {
			if o.Email != nil && strings.EqualFold(*o.Email, *u.Email) {
				return errors.New("duplicate email")
			}
		}
	}
	cp := *u
	m.users[u.ID] = &cp
	return nil
}
func (m *memUsers) Update(_ context.Context, u *domain.User) error {
	cp := *u
	m.users[u.ID] = &cp
	return nil
}
func (m *memUsers) AssignRole(_ context.Context, id string, r domain.Role) error {
	u := m.users[id]
	if u.HasRole(r) {
		return domain.ErrRoleAlreadyAssigned
	}
	u.Roles = append(u.Roles, r)
	return nil
}
func (m *memUsers) RemoveRole(context.Context, string, domain.Role) error { return nil }
func (m *memUsers) GetUserRoles(_ context.Context, id string) ([]domain.Role, error) {
	return m.users[id].Roles, nil
}
func (m *memUsers) HasRole(_ context.Context, id string, r domain.Role) (bool, error) {
	return m.users[id].HasRole(r), nil
}

func (m *memUsers) FindUserByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range m.users {
		if u.Email != nil && strings.EqualFold(*u.Email, email) {
			cp := *u
			return &cp, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (m *memUsers) GetCredential(_ context.Context, id string) (*domain.Credential, error) {
	h, ok := m.hashes[id]
	if !ok {
		return nil, domain.ErrCredentialNotFound
	}
	return &domain.Credential{UserID: id, PasswordHash: h, FailedAttempts: m.attempts[id], LockedUntil: m.locked[id]}, nil
}
func (m *memUsers) SetPassword(_ context.Context, id, hash string, _ int64) error {
	m.hashes[id] = hash
	m.attempts[id], m.locked[id] = 0, 0
	return nil
}
func (m *memUsers) RecordLoginFailure(_ context.Context, id string, now int64, max int, lockFor int64) error {
	if m.locked[id] != 0 && m.locked[id] <= now {
		m.attempts[id], m.locked[id] = 0, 0
	}
	m.attempts[id]++
	if m.attempts[id] >= max {
		m.locked[id] = now + lockFor
	}
	return nil
}
func (m *memUsers) RecordLoginSuccess(_ context.Context, id string, now int64) error {
	m.attempts[id], m.locked[id], m.lastIn[id] = 0, 0, now
	return nil
}

type noChallenges struct{}

func (noChallenges) Create(context.Context, *domain.OTPChallenge) error { return nil }
func (noChallenges) FindByID(context.Context, string) (*domain.OTPChallenge, error) {
	return nil, errors.New("none")
}
func (noChallenges) RegisterAttempt(context.Context, string, int64) (bool, error) { return false, nil }
func (noChallenges) Consume(context.Context, string, int64) (bool, error)         { return false, nil }
func (noChallenges) CountSince(context.Context, string, int64) (int, error)       { return 0, nil }

type noSender struct{}

func (noSender) Send(context.Context, string, string) error { return nil }

const (
	adminEmail = "guilbertmkap@gmail.com"
	adminPass  = "Kap0071001"
)

// staffFixture builds a service with one provisioned admin and a controllable clock.
func staffFixture(t *testing.T) (*Service, *memUsers, *int64) {
	t.Helper()
	store := newMemUsers()
	now := int64(1_800_000_000)
	svc, err := NewService(Config{
		Users: store, Challenges: noChallenges{}, Sender: noSender{}, Credentials: store,
		Tokens:     domain.NewJWTTokenService(domain.JWTConfig{SecretKey: strings.Repeat("s", 40), AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, Issuer: "t", Audience: "t"}),
		CodeSecret: strings.Repeat("c", 20), AccessTokenTTL: time.Minute, Clock: func() int64 { return now }, Logger: zerolog.Nop(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ProvisionStaff(context.Background(), store, store, StaffInput{Email: adminEmail, DisplayName: "Guilbert", Password: adminPass, Roles: []domain.Role{domain.RoleAdmin}}, now); err != nil {
		t.Fatal(err)
	}
	return svc, store, &now
}

func TestProvisionStaffCreatesAnAdminWithoutPhone(t *testing.T) {
	store := newMemUsers()
	res, err := ProvisionStaff(context.Background(), store, store, StaffInput{Email: "  Guilbertmkap@Gmail.com ", DisplayName: "Guilbert", Password: adminPass, Roles: []domain.Role{domain.RoleAdmin}}, 100)
	if err != nil {
		t.Fatal(err)
	}
	u := res.User
	if !res.Created || u.Phone != "" || u.Email == nil || *u.Email != adminEmail || !u.HasRole(domain.RoleAdmin) || u.Status != domain.UserStatusActive || u.VerifiedAt == nil {
		t.Fatalf("unexpected account: created=%v %+v", res.Created, u)
	}
	if h := store.hashes[u.ID]; h == "" || strings.Contains(h, adminPass) {
		t.Fatalf("only a hash of the password may be stored: %q", h)
	}
}

func TestProvisionStaffOnAnExistingEmailResetsThePassword(t *testing.T) {
	svc, store, _ := staffFixture(t)
	res, err := ProvisionStaff(context.Background(), store, store, StaffInput{Email: adminEmail, Password: "Nouveau-Mot-De-Passe-7", Roles: []domain.Role{domain.RoleAdmin, domain.RoleOperator}}, 200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Created || len(store.users) != 1 || len(res.User.Roles) != 2 {
		t.Fatalf("the same account is updated, not duplicated: created=%v users=%d roles=%v", res.Created, len(store.users), res.User.Roles)
	}
	if _, err := svc.StaffLogin(context.Background(), adminEmail, adminPass); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("the old password must stop working: %v", err)
	}
	if _, err := svc.StaffLogin(context.Background(), adminEmail, "Nouveau-Mot-De-Passe-7"); err != nil {
		t.Fatalf("the new password must work: %v", err)
	}
}

func TestProvisionStaffRefusesWeakInput(t *testing.T) {
	store := newMemUsers()
	cases := map[string]StaffInput{
		"weak password":  {Email: adminEmail, Password: "short", Roles: []domain.Role{domain.RoleAdmin}},
		"bad email":      {Email: "not-an-email", Password: adminPass, Roles: []domain.Role{domain.RoleAdmin}},
		"no role":        {Email: adminEmail, Password: adminPass},
		"customer role":  {Email: adminEmail, Password: adminPass, Roles: []domain.Role{domain.RoleCustomer}},
		"seller role":    {Email: adminEmail, Password: adminPass, Roles: []domain.Role{domain.RoleSeller}},
		"email in match": {Email: adminEmail, Password: "guilbertmkap-Zz9", Roles: []domain.Role{domain.RoleAdmin}},
	}
	for name, in := range cases {
		if _, err := ProvisionStaff(context.Background(), store, store, in, 1); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if len(store.users) != 0 {
		t.Fatal("nothing must be created for a refused input")
	}
}

func TestStaffLoginSignsInAnAdmin(t *testing.T) {
	svc, store, now := staffFixture(t)
	for _, email := range []string{adminEmail, "GUILBERTMKAP@gmail.com", "  guilbertmkap@gmail.com "} {
		s, err := svc.StaffLogin(context.Background(), email, adminPass)
		if err != nil {
			t.Fatalf("%q: %v", email, err)
		}
		if s.AccessToken == "" || s.RefreshToken == "" || !s.User.HasRole(domain.RoleAdmin) || s.ExpiresAt != *now+60 {
			t.Fatalf("incomplete session: %+v", s)
		}
	}
	for id := range store.users {
		if store.lastIn[id] != *now {
			t.Fatal("the sign-in time must be recorded")
		}
	}
}

func TestStaffLoginGivesOneAnswerForEveryFailure(t *testing.T) {
	svc, store, _ := staffFixture(t)
	ctx := context.Background()

	// A customer who happens to hold a password is still not staff.
	customerHash := store.hashes[firstUserID(store)]
	cust := domain.NewUser("+243810000001", "Client")
	cust.Email = ptr("client@example.com")
	cust.Status = domain.UserStatusActive
	cust.Roles = []domain.Role{domain.RoleCustomer}
	_ = store.Create(ctx, cust)
	store.hashes[cust.ID] = customerHash

	// A suspended administrator.
	susp := domain.NewUser("", "Suspendu")
	susp.Email = ptr("suspendu@example.com")
	susp.Status = domain.UserStatusSuspended
	susp.Roles = []domain.Role{domain.RoleAdmin}
	_ = store.Create(ctx, susp)
	store.hashes[susp.ID] = customerHash

	// A staff account that never got a password.
	nopass := domain.NewUser("", "Sans mot de passe")
	nopass.Email = ptr("sans@example.com")
	nopass.Status = domain.UserStatusActive
	nopass.Roles = []domain.Role{domain.RoleOperator}
	_ = store.Create(ctx, nopass)

	cases := map[string][2]string{
		"wrong password":    {adminEmail, "Kap0071002"},
		"unknown email":     {"personne@example.com", adminPass},
		"customer account":  {"client@example.com", adminPass},
		"suspended account": {"suspendu@example.com", adminPass},
		"no password set":   {"sans@example.com", adminPass},
		"empty password":    {adminEmail, ""},
		"invalid email":     {"not-an-email", adminPass},
		"huge password":     {adminEmail, strings.Repeat("a", 5000)},
	}
	for name, c := range cases {
		s, err := svc.StaffLogin(ctx, c[0], c[1])
		if s != nil || !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("%s: session=%v err=%v, want ErrInvalidCredentials", name, s != nil, err)
		}
	}
}

func TestStaffLoginLocksAfterRepeatedFailuresThenReleases(t *testing.T) {
	svc, store, now := staffFixture(t)
	ctx := context.Background()
	for i := 0; i < DefaultMaxLoginFailures; i++ {
		if _, err := svc.StaffLogin(ctx, adminEmail, "Wrong-Password-"+string(rune('a'+i))); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if store.locked[firstUserID(store)] == 0 {
		t.Fatalf("the account must be locked after %d wrong passwords", DefaultMaxLoginFailures)
	}
	// While locked even the right password is refused, with the same answer.
	if _, err := svc.StaffLogin(ctx, adminEmail, adminPass); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("a locked account must refuse the right password: %v", err)
	}
	*now += int64(DefaultLockout/time.Second) + 1
	if _, err := svc.StaffLogin(ctx, adminEmail, adminPass); err != nil {
		t.Fatalf("the lock must release after %s: %v", DefaultLockout, err)
	}
	if store.attempts[firstUserID(store)] != 0 {
		t.Fatal("a success clears the failure counter")
	}
}

func TestStaffLoginIsOffWithoutACredentialStore(t *testing.T) {
	store := newMemUsers()
	svc, err := NewService(Config{Users: store, Challenges: noChallenges{}, Sender: noSender{}, CodeSecret: strings.Repeat("c", 20),
		Tokens: domain.NewJWTTokenService(domain.JWTConfig{SecretKey: strings.Repeat("s", 40), AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour, Issuer: "t", Audience: "t"})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.StaffLogin(context.Background(), adminEmail, adminPass); !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func firstUserID(m *memUsers) string {
	for id := range m.users {
		return id
	}
	return ""
}

func ptr(s string) *string { return &s }
