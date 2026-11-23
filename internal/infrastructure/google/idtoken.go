// Package google verifies Google ID tokens (OpenID Connect) for "Sign in with Google". Only the public client id is
// needed: the token is checked against Google's published keys; no client secret is used anywhere.
package google

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// DefaultJWKSURL is where Google publishes the keys that sign its ID tokens.
const DefaultJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"

var validIssuers = map[string]bool{"https://accounts.google.com": true, "accounts.google.com": true}

// ErrInvalidToken: the credential is not a valid Google ID token for this application.
var ErrInvalidToken = errors.New("invalid Google ID token")

// Identity is what a verified ID token says about the Google account.
type Identity struct {
	Subject       string // stable account id
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
}

// Verifier checks Google ID tokens issued to one OAuth client.
type Verifier struct {
	clientID string
	jwksURL  string
	client   *http.Client
	now      func() time.Time

	mu        sync.Mutex
	keys      map[string]*rsa.PublicKey
	expiresAt time.Time
	lastFetch time.Time
}

// NewVerifier builds a verifier for the given OAuth client id (from Google Cloud Console).
func NewVerifier(clientID string) *Verifier {
	return &Verifier{clientID: strings.TrimSpace(clientID), jwksURL: DefaultJWKSURL, client: &http.Client{Timeout: 10 * time.Second}, now: time.Now}
}

// WithJWKSURL points the verifier at another key set (tests).
func (v *Verifier) WithJWKSURL(url string) *Verifier { v.jwksURL = url; return v }

// WithClock sets the time used to check expiry (tests).
func (v *Verifier) WithClock(now func() time.Time) *Verifier { v.now = now; return v }

// ClientID is the OAuth client the tokens must be issued to.
func (v *Verifier) ClientID() string { return v.clientID }

type googleClaims struct {
	Email         string      `json:"email"`
	EmailVerified interface{} `json:"email_verified"` // Google sends a boolean (older tokens: "true")
	Name          string      `json:"name"`
	Nonce         string      `json:"nonce"`
	jwt.RegisteredClaims
}

// Verify checks the signature (RS256, Google's current keys), the issuer, the audience (this client id), the expiry
// and the subject, and returns the identity. Any failure is ErrInvalidToken (wrapped with the reason).
func (v *Verifier) Verify(ctx context.Context, raw string) (*Identity, error) {
	if v.clientID == "" {
		return nil, fmt.Errorf("%w: no client id configured", ErrInvalidToken)
	}
	claims := &googleClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("no key id")
		}
		return v.key(ctx, kid)
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithAudience(v.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(60*time.Second),
		jwt.WithTimeFunc(v.now),
	)
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !validIssuers[claims.Issuer] {
		return nil, fmt.Errorf("%w: issuer %q", ErrInvalidToken, claims.Issuer)
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("%w: no subject", ErrInvalidToken)
	}
	verified := false
	switch e := claims.EmailVerified.(type) {
	case bool:
		verified = e
	case string:
		verified = e == "true"
	}
	return &Identity{Subject: claims.Subject, Email: strings.ToLower(strings.TrimSpace(claims.Email)), EmailVerified: verified,
		Name: strings.TrimSpace(claims.Name), Nonce: claims.Nonce}, nil
}

// key returns the public key with that id, fetching Google's key set when it is unknown or stale (at most once a
// minute when a key id is unknown, so a forged id cannot make the service hammer Google).
func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	now := v.now()
	if k, ok := v.keys[kid]; ok && now.Before(v.expiresAt) {
		return k, nil
	}
	if !v.lastFetch.IsZero() && now.Sub(v.lastFetch) < time.Minute && now.Before(v.expiresAt) {
		return nil, errors.New("unknown key id")
	}
	if err := v.fetch(ctx, now); err != nil {
		return nil, err
	}
	if k, ok := v.keys[kid]; ok {
		return k, nil
	}
	return nil, errors.New("unknown key id")
}

var maxAgePattern = regexp.MustCompile(`max-age=(\d+)`)

func (v *Verifier) fetch(ctx context.Context, now time.Time) error {
	v.lastFetch = now
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch Google keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch Google keys: HTTP %d", resp.StatusCode)
	}
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&set); err != nil {
		return fmt.Errorf("read Google keys: %w", err)
	}
	keys := map[string]*rsa.PublicKey{}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		n, err1 := base64.RawURLEncoding.DecodeString(k.N)
		e, err2 := base64.RawURLEncoding.DecodeString(k.E)
		if err1 != nil || err2 != nil || len(e) == 0 || len(e) > 4 {
			continue
		}
		exp := 0
		for _, b := range e {
			exp = exp<<8 | int(b)
		}
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: exp}
	}
	if len(keys) == 0 {
		return errors.New("Google published no usable key")
	}
	ttl := time.Hour
	if m := maxAgePattern.FindStringSubmatch(resp.Header.Get("Cache-Control")); m != nil {
		if s, err := strconv.Atoi(m[1]); err == nil && s > 60 {
			ttl = time.Duration(s) * time.Second
		}
	}
	v.keys, v.expiresAt = keys, now.Add(ttl)
	return nil
}
