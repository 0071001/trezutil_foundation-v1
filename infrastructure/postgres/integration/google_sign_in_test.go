package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/google"
	"github.com/trezutil/trezutil/internal/infrastructure/google/googletest"
	"github.com/trezutil/trezutil/internal/infrastructure/otp"
)

const googleClientID = "trezutil-test.apps.googleusercontent.com"

func googleService(t *testing.T, db *TestDB, g *googletest.Google) (*identity.Service, domain.TokenService) {
	t.Helper()
	users := postgres.NewUserRepository(db.DB)
	tokens := domain.NewJWTTokenService(domain.JWTConfig{SecretKey: "integration-secret-integration-secret", AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: time.Hour, Issuer: "trezutil", Audience: "trezutil"})
	svc, err := identity.NewService(identity.Config{
		Users: users, Challenges: postgres.NewOTPChallengeRepository(db.DB), Sender: otp.DevLogSender{Logger: zerolog.Nop()},
		Tokens: tokens, Credentials: users, CodeSecret: "code-secret-for-the-integration-test", Logger: zerolog.Nop(),
		Google: google.NewVerifier(googleClientID).WithJWKSURL(g.Server.URL), Identities: postgres.NewExternalIdentityRepository(db.DB),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, tokens
}

// Sign in with Google on the real schema (migration 000035): create, sign in again, never link on an email alone,
// link from a signed-in account, refuse to unlink the only way in.
func TestGoogleSignIn_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx, `INSERT INTO roles (name) VALUES ('customer'), ('seller') ON CONFLICT (name) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	g := googletest.New(t)
	svc, tokens := googleService(t, db, g)
	signIn := func(tok googletest.Token) (*identity.Session, error) {
		nonce, _, err := svc.GoogleNonce()
		if err != nil {
			t.Fatal(err)
		}
		tok.Audience, tok.Nonce = googleClientID, nonce
		return svc.SignInWithGoogle(ctx, g.Sign(t, tok), nonce)
	}

	// 1. A new Google account creates a customer account without a phone, with the verified email.
	first, err := signIn(googletest.Token{Subject: "google-111", Email: "ama@example.com", EmailVerified: true, Name: "Ama Kabila"})
	if err != nil {
		t.Fatalf("create with Google: %v", err)
	}
	if first.User.Phone != "" || first.User.Email == nil || *first.User.Email != "ama@example.com" || first.User.DisplayName != "Ama Kabila" || !first.User.HasRole(domain.RoleCustomer) {
		t.Fatalf("new account: %+v", first.User)
	}
	if uid, err := tokens.ValidateAccessToken(first.AccessToken); err != nil || uid != first.User.ID {
		t.Fatalf("the session is a normal Trezutil session: %v", err)
	}

	// 2. The same Google account opens the same account (found by its id, not its email).
	again, err := signIn(googletest.Token{Subject: "google-111", Email: "ama.new@example.com", EmailVerified: true})
	if err != nil || again.User.ID != first.User.ID {
		t.Fatalf("sign in again: %v %+v", err, again)
	}

	// 3. Another Google account with an email an existing account already uses: nothing is created or linked.
	phoneUser := "123e4567-e89b-12d3-a456-426614174201"
	newUser(t, db, ctx, phoneUser, "+243810000201")
	if _, err := db.DB.ExecContext(ctx, `UPDATE users SET email = 'kofi@example.com' WHERE id = $1`, phoneUser); err != nil {
		t.Fatal(err)
	}
	if _, err := signIn(googletest.Token{Subject: "google-222", Email: "KOFI@example.com", EmailVerified: true}); !errors.Is(err, identity.ErrAccountExists) {
		t.Fatalf("an email already in use must not be linked automatically: %v", err)
	}
	var count int
	_ = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM external_identities WHERE provider_subject = 'google-222'`).Scan(&count)
	if count != 0 {
		t.Fatal("nothing may be linked on an email alone")
	}

	// 4. An unverified email is never used: the account is created without it.
	unverified, err := signIn(googletest.Token{Subject: "google-333", Email: "kofi@example.com", EmailVerified: false, Name: "X"})
	if err != nil || unverified.User.Email != nil || unverified.User.ID == phoneUser {
		t.Fatalf("an unverified email must be ignored: %v %+v", err, unverified)
	}

	// 5. The phone account links its Google account itself (session + a fresh Google sign-in).
	nonce, _, _ := svc.GoogleNonce()
	link, err := svc.LinkGoogle(ctx, phoneUser, g.Sign(t, googletest.Token{Subject: "google-222", Audience: googleClientID, Nonce: nonce, Email: "kofi@example.com", EmailVerified: true}), nonce)
	if err != nil || link.Subject != "google-222" {
		t.Fatalf("link: %v", err)
	}
	viaGoogle, err := signIn(googletest.Token{Subject: "google-222", Email: "kofi@example.com", EmailVerified: true})
	if err != nil || viaGoogle.User.ID != phoneUser {
		t.Fatalf("after linking, Google opens the phone account: %v", err)
	}
	nonce, _, _ = svc.GoogleNonce()
	if _, err := svc.LinkGoogle(ctx, phoneUser, g.Sign(t, googletest.Token{Subject: "google-111", Audience: googleClientID, Nonce: nonce}), nonce); !errors.Is(err, identity.ErrGoogleLinkedElsewhere) {
		t.Fatalf("a Google account linked to someone else cannot be taken: %v", err)
	}

	// 6. Unlinking: refused when Google is the only way in, allowed when the phone remains.
	if err := svc.UnlinkGoogle(ctx, first.User.ID); !errors.Is(err, identity.ErrLastSignInMethod) {
		t.Fatalf("the only sign-in method cannot be removed: %v", err)
	}
	if err := svc.UnlinkGoogle(ctx, phoneUser); err != nil {
		t.Fatalf("unlink with the phone left: %v", err)
	}

	// 7. Nonces: a token whose nonce is not the one issued, or one forged, is refused.
	nonce, _, _ = svc.GoogleNonce()
	if _, err := svc.SignInWithGoogle(ctx, g.Sign(t, googletest.Token{Subject: "google-111", Audience: googleClientID, Nonce: "other"}), nonce); !errors.Is(err, identity.ErrGoogleToken) {
		t.Fatalf("mismatched nonce: %v", err)
	}
	forged := "AAAA.9999999999.forged"
	if _, err := svc.SignInWithGoogle(ctx, g.Sign(t, googletest.Token{Subject: "google-111", Audience: googleClientID, Nonce: forged}), forged); !errors.Is(err, identity.ErrGoogleToken) {
		t.Fatalf("forged nonce: %v", err)
	}

	// 8. A suspended account cannot sign in with Google.
	if _, err := db.DB.ExecContext(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1`, first.User.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := signIn(googletest.Token{Subject: "google-111"}); !errors.Is(err, domain.ErrUserSuspended) {
		t.Fatalf("suspended: %v", err)
	}
}
