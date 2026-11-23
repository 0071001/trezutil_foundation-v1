package identity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/google"
	"github.com/trezutil/trezutil/internal/infrastructure/google/googletest"
	"github.com/trezutil/trezutil/internal/infrastructure/otp"
)

const testClientID = "trezutil-unit.apps.googleusercontent.com"

// memIdentities is an in-memory ExternalIdentityRepository with the two unique rules of migration 000035.
type memIdentities struct{ links []domain.ExternalIdentity }

func (m *memIdentities) FindUserID(_ context.Context, provider, subject string) (string, error) {
	for _, l := range m.links {
		if l.Provider == provider && l.Subject == subject {
			return l.UserID, nil
		}
	}
	return "", nil
}
func (m *memIdentities) Link(_ context.Context, id *domain.ExternalIdentity) error {
	for _, l := range m.links {
		if (l.Provider == id.Provider && l.Subject == id.Subject) || (l.UserID == id.UserID && l.Provider == id.Provider) {
			return domain.ErrIdentityTaken
		}
	}
	m.links = append(m.links, *id)
	return nil
}
func (m *memIdentities) Touch(context.Context, string, string, string, string) error { return nil }
func (m *memIdentities) ListByUser(_ context.Context, userID string) ([]domain.ExternalIdentity, error) {
	out := []domain.ExternalIdentity{}
	for _, l := range m.links {
		if l.UserID == userID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (m *memIdentities) Unlink(_ context.Context, userID, provider string) error {
	for i, l := range m.links {
		if l.UserID == userID && l.Provider == provider {
			m.links = append(m.links[:i], m.links[i+1:]...)
			return nil
		}
	}
	return domain.ErrIdentityNotLinked
}

func googleTestService(t *testing.T, g *googletest.Google, users *memUsers, ids *memIdentities, clock Clock) *Service {
	t.Helper()
	svc, err := NewService(Config{
		Users: users, Challenges: noChallenges{}, Sender: otp.DevLogSender{Logger: zerolog.Nop()},
		Tokens: domain.NewJWTTokenService(domain.JWTConfig{SecretKey: "unit-secret-unit-secret-unit-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour}),
		Credentials: users, CodeSecret: "code-secret-for-unit-tests", Logger: zerolog.Nop(), Clock: clock,
		Google: google.NewVerifier(testClientID).WithJWKSURL(g.Server.URL), Identities: ids,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestSignInWithGoogle(t *testing.T) {
	g := googletest.New(t)
	users, ids := newMemUsers(), &memIdentities{}
	svc := googleTestService(t, g, users, ids, nil)
	ctx := context.Background()
	signIn := func(tok googletest.Token) (*Session, error) {
		nonce, _, err := svc.GoogleNonce()
		if err != nil {
			t.Fatal(err)
		}
		tok.Audience, tok.Nonce = testClientID, nonce
		return svc.SignInWithGoogle(ctx, g.Sign(t, tok), nonce)
	}

	first, err := signIn(googletest.Token{Subject: "g-1", Email: "ama@example.com", EmailVerified: true, Name: "Ama"})
	if err != nil || first.User.Phone != "" || first.User.Email == nil || !first.User.HasRole(domain.RoleCustomer) || first.AccessToken == "" {
		t.Fatalf("create: %v %+v", err, first)
	}
	again, err := signIn(googletest.Token{Subject: "g-1", Email: "changed@example.com", EmailVerified: true})
	if err != nil || again.User.ID != first.User.ID || len(users.users) != 1 {
		t.Fatalf("the same Google account opens the same account: %v", err)
	}

	// An address another account uses is never linked on its own.
	owner := domain.NewUser("+243810000999", "Kofi")
	email := "kofi@example.com"
	owner.Email, owner.Status = &email, domain.UserStatusActive
	_ = users.Create(ctx, owner)
	if _, err := signIn(googletest.Token{Subject: "g-2", Email: "Kofi@Example.com", EmailVerified: true}); !errors.Is(err, ErrAccountExists) {
		t.Fatalf("want ErrAccountExists, got %v", err)
	}
	if len(ids.links) != 1 {
		t.Fatal("nothing may be linked on an email alone")
	}
	// An unverified email is ignored (account created without it).
	if s, err := signIn(googletest.Token{Subject: "g-3", Email: "kofi@example.com"}); err != nil || s.User.Email != nil {
		t.Fatalf("unverified email: %v %+v", err, s)
	}

	// The owner links Google from their session; afterwards Google opens their account.
	nonce, _, _ := svc.GoogleNonce()
	if _, err := svc.LinkGoogle(ctx, owner.ID, g.Sign(t, googletest.Token{Subject: "g-2", Audience: testClientID, Nonce: nonce, Email: email, EmailVerified: true}), nonce); err != nil {
		t.Fatalf("link: %v", err)
	}
	if s, err := signIn(googletest.Token{Subject: "g-2"}); err != nil || s.User.ID != owner.ID {
		t.Fatalf("Google opens the linked account: %v", err)
	}
	nonce, _, _ = svc.GoogleNonce()
	if _, err := svc.LinkGoogle(ctx, owner.ID, g.Sign(t, googletest.Token{Subject: "g-1", Audience: testClientID, Nonce: nonce}), nonce); !errors.Is(err, ErrGoogleLinkedElsewhere) {
		t.Fatalf("taking someone else's Google account: %v", err)
	}

	// Unlinking keeps at least one way in.
	if err := svc.UnlinkGoogle(ctx, first.User.ID); !errors.Is(err, ErrLastSignInMethod) {
		t.Fatalf("only method: %v", err)
	}
	if err := svc.UnlinkGoogle(ctx, owner.ID); err != nil {
		t.Fatalf("unlink with the phone left: %v", err)
	}
}

func TestGoogleNonceIsCheckedAndExpires(t *testing.T) {
	g := googletest.New(t)
	now := time.Now().Unix()
	svc := googleTestService(t, g, newMemUsers(), &memIdentities{}, func() int64 { return now })
	ctx := context.Background()
	nonce, _, _ := svc.GoogleNonce()

	if _, err := svc.SignInWithGoogle(ctx, g.Sign(t, googletest.Token{Subject: "g", Audience: testClientID, Nonce: "other"}), nonce); !errors.Is(err, ErrGoogleToken) {
		t.Fatalf("a token carrying another nonce: %v", err)
	}
	if _, err := svc.SignInWithGoogle(ctx, g.Sign(t, googletest.Token{Subject: "g", Audience: testClientID, Nonce: "x.1.y"}), "x.1.y"); !errors.Is(err, ErrGoogleToken) {
		t.Fatalf("a nonce the server did not issue: %v", err)
	}
	now += int64(GoogleNonceTTL/time.Second) + 1
	if _, err := svc.SignInWithGoogle(ctx, g.Sign(t, googletest.Token{Subject: "g", Audience: testClientID, Nonce: nonce}), nonce); !errors.Is(err, ErrGoogleToken) {
		t.Fatalf("an expired nonce: %v", err)
	}
}

func TestGoogleOffWithoutClientID(t *testing.T) {
	svc, err := NewService(Config{Users: newMemUsers(), Challenges: noChallenges{}, Sender: otp.DevLogSender{}, Tokens: domain.NewJWTTokenService(domain.JWTConfig{SecretKey: "s-s-s-s-s-s-s-s-s-s-s-s-s-s-s-s"}), CodeSecret: "code-secret-for-unit-tests"})
	if err != nil {
		t.Fatal(err)
	}
	if svc.GoogleClientID() != "" {
		t.Fatal("no client id: Google is off")
	}
	if _, _, err := svc.GoogleNonce(); !errors.Is(err, ErrGoogleUnavailable) {
		t.Fatalf("want ErrGoogleUnavailable, got %v", err)
	}
}
