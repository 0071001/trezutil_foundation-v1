package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// MinSecretLength is the minimum accepted HS256 signing secret length.
const MinSecretLength = 32

type ctxKey struct{}

// Principal is the authenticated caller, derived exclusively from a verified
// access token. Client-supplied user, organization or actor fields must never
// be used in its place.
type Principal struct {
	UserID string
}

// WithPrincipal returns a context carrying the given principal.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// FromContext returns the authenticated principal, if any.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok && p.UserID != ""
}

// UserID returns the authenticated user ID, or "" for anonymous callers.
func UserID(ctx context.Context) string {
	p, _ := FromContext(ctx)
	return p.UserID
}

// ConfigFromEnv builds the JWT configuration shared by every service.
// JWT_SECRET is mandatory and must be at least MinSecretLength characters.
func ConfigFromEnv() (domain.JWTConfig, error) {
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < MinSecretLength {
		return domain.JWTConfig{}, fmt.Errorf("JWT_SECRET must be set to at least %d characters", MinSecretLength)
	}
	if Strict() {
		if err := ValidateSecret("JWT_SECRET", secret); err != nil {
			return domain.JWTConfig{}, err
		}
	}
	return domain.JWTConfig{
		SecretKey:       secret,
		Issuer:          envOr("JWT_ISSUER", "trezutil"),
		Audience:        envOr("JWT_AUDIENCE", "trezutil-api"),
		AccessTokenTTL:  durationOr("JWT_ACCESS_TTL", 15*time.Minute),
		RefreshTokenTTL: durationOr("JWT_REFRESH_TTL", 30*24*time.Hour),
	}, nil
}

// Authenticator verifies bearer access tokens.
type Authenticator struct {
	tokens domain.TokenService
}

// NewAuthenticator creates an Authenticator backed by the given token service.
func NewAuthenticator(tokens domain.TokenService) *Authenticator {
	return &Authenticator{tokens: tokens}
}

// NewAuthenticatorFromEnv creates an Authenticator from the shared JWT environment.
func NewAuthenticatorFromEnv() (*Authenticator, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return nil, err
	}
	return NewAuthenticator(domain.NewJWTTokenService(cfg)), nil
}

// ErrMissingToken is returned when no bearer token is present.
var ErrMissingToken = errors.New("missing bearer token")

// Authenticate verifies the request's bearer token. It returns ErrMissingToken
// when the request carries no Authorization header.
func (a *Authenticator) Authenticate(r *http.Request) (Principal, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return Principal{}, ErrMissingToken
	}
	scheme, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return Principal{}, errors.New("malformed authorization header")
	}
	userID, err := a.tokens.ValidateAccessToken(strings.TrimSpace(token))
	if err != nil {
		return Principal{}, err
	}
	return Principal{UserID: userID}, nil
}

// Middleware attaches the verified principal to the request context.
// Requests without a token continue anonymously (handlers decide whether a
// guest may proceed); requests with an invalid or expired token are rejected
// with 401 so clients can refresh instead of silently acting as a guest.
func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := a.Authenticate(r)
		switch {
		case err == nil:
			r = r.WithContext(WithPrincipal(r.Context(), p))
		case errors.Is(err, ErrMissingToken):
			// anonymous
		default:
			WriteUnauthorized(w, "INVALID_TOKEN", "Access token is invalid or expired")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireUser returns the authenticated user ID, or writes a 401 response and
// returns false for anonymous callers.
func RequireUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	if id := UserID(r.Context()); id != "" {
		return id, true
	}
	WriteUnauthorized(w, "AUTHENTICATION_REQUIRED", "Sign in to perform this action")
	return "", false
}

// WriteUnauthorized writes the standard error envelope with status 401.
func WriteUnauthorized(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="trezutil"`)
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"code": code, "message": message},
	})
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func durationOr(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}
