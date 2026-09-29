package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func testTokens() domain.TokenService {
	return domain.NewJWTTokenService(domain.JWTConfig{
		SecretKey:       "0123456789abcdef0123456789abcdef",
		Issuer:          "trezutil",
		Audience:        "trezutil-api",
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: time.Hour,
	})
}

func serve(a *Authenticator, header string) (int, string) {
	var seen string
	h := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestMiddleware_ValidAccessToken(t *testing.T) {
	tokens := testTokens()
	token, _ := tokens.IssueAccessToken("user-1", nil)
	code, user := serve(NewAuthenticator(tokens), "Bearer "+token)
	if code != http.StatusOK || user != "user-1" {
		t.Fatalf("expected 200/user-1, got %d/%q", code, user)
	}
}

func TestMiddleware_AnonymousPassesThrough(t *testing.T) {
	code, user := serve(NewAuthenticator(testTokens()), "")
	if code != http.StatusOK || user != "" {
		t.Fatalf("expected anonymous 200, got %d/%q", code, user)
	}
}

func TestMiddleware_RejectsRefreshTokenAndGarbage(t *testing.T) {
	tokens := testTokens()
	refresh, _ := tokens.IssueRefreshToken("user-1")
	a := NewAuthenticator(tokens)
	for _, h := range []string{"Bearer " + refresh, "Bearer not-a-jwt", "Basic abc", "Bearer "} {
		if code, _ := serve(a, h); code != http.StatusUnauthorized {
			t.Errorf("header %q: expected 401, got %d", h, code)
		}
	}
}

func TestMiddleware_RejectsTokenSignedWithOtherSecret(t *testing.T) {
	other := domain.NewJWTTokenService(domain.JWTConfig{SecretKey: "ffffffffffffffffffffffffffffffff", Issuer: "trezutil", Audience: "trezutil-api", AccessTokenTTL: time.Minute})
	forged, _ := other.IssueAccessToken("admin", nil)
	if code, _ := serve(NewAuthenticator(testTokens()), "Bearer "+forged); code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for forged token, got %d", code)
	}
}

func TestConfigFromEnv_RequiresStrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "short")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("expected error for short secret")
	}
	t.Setenv("JWT_SECRET", "0123456789abcdef0123456789abcdef")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
