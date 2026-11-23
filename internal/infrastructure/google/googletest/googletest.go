// Package googletest plays Google's side of "Sign in with Google" in tests: an RSA key, a key set served like
// https://www.googleapis.com/oauth2/v3/certs, and ID tokens signed with that key.
package googletest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Issuer is a valid Google issuer.
const Issuer = "https://accounts.google.com"

// Google is a fake identity provider.
type Google struct {
	Key    *rsa.PrivateKey
	KeyID  string
	Server *httptest.Server
}

// New starts a key-set server for one RSA key.
func New(t *testing.T) *Google {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	g := &Google{Key: key, KeyID: "test-key-1"}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": []map[string]string{{
			"kid": g.KeyID, "kty": "RSA", "alg": "RS256", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
		}}})
	}))
	t.Cleanup(g.Server.Close)
	return g
}

// Token is what an ID token says; zero values get sensible defaults in Sign.
type Token struct {
	Subject       string
	Audience      string
	Issuer        string
	Email         string
	EmailVerified bool
	Name          string
	Nonce         string
	ExpiresIn     time.Duration
	KeyID         string
	Key           *rsa.PrivateKey
}

// Sign returns a signed ID token.
func (g *Google) Sign(t *testing.T, tok Token) string {
	t.Helper()
	if tok.Issuer == "" {
		tok.Issuer = Issuer
	}
	if tok.ExpiresIn == 0 {
		tok.ExpiresIn = time.Hour
	}
	if tok.KeyID == "" {
		tok.KeyID = g.KeyID
	}
	if tok.Key == nil {
		tok.Key = g.Key
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": tok.Issuer, "aud": tok.Audience, "sub": tok.Subject, "email": tok.Email, "email_verified": tok.EmailVerified,
		"name": tok.Name, "nonce": tok.Nonce, "iat": now.Unix(), "exp": now.Add(tok.ExpiresIn).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = tok.KeyID
	raw, err := token.SignedString(tok.Key)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
