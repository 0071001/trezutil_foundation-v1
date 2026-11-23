package google_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/trezutil/trezutil/internal/infrastructure/google"
	"github.com/trezutil/trezutil/internal/infrastructure/google/googletest"
)

const clientID = "1234-abc.apps.googleusercontent.com"

func TestVerifyAcceptsAValidToken(t *testing.T) {
	g := googletest.New(t)
	v := google.NewVerifier(clientID).WithJWKSURL(g.Server.URL)
	id, err := v.Verify(context.Background(), g.Sign(t, googletest.Token{Subject: "g-1", Audience: clientID, Email: "Ama@Example.com", EmailVerified: true, Name: "Ama", Nonce: "n-1"}))
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "g-1" || id.Email != "ama@example.com" || !id.EmailVerified || id.Name != "Ama" || id.Nonce != "n-1" {
		t.Fatalf("identity: %+v", id)
	}
}

func TestVerifyRefusesWhatGoogleDidNotIssueForThisClient(t *testing.T) {
	g := googletest.New(t)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	v := google.NewVerifier(clientID).WithJWKSURL(g.Server.URL)
	cases := map[string]string{
		"another client":       g.Sign(t, googletest.Token{Subject: "g-1", Audience: "someone-else.apps.googleusercontent.com"}),
		"another issuer":       g.Sign(t, googletest.Token{Subject: "g-1", Audience: clientID, Issuer: "https://evil.example"}),
		"expired":              g.Sign(t, googletest.Token{Subject: "g-1", Audience: clientID, ExpiresIn: -5 * time.Minute}),
		"forged signature":     g.Sign(t, googletest.Token{Subject: "g-1", Audience: clientID, Key: other}),
		"unknown key":          g.Sign(t, googletest.Token{Subject: "g-1", Audience: clientID, KeyID: "nope"}),
		"no subject":           g.Sign(t, googletest.Token{Audience: clientID}),
		"not a token":          "abc.def.ghi",
		"symmetric (alg none)": unsigned(t),
	}
	for name, raw := range cases {
		if _, err := v.Verify(context.Background(), raw); !errors.Is(err, google.ErrInvalidToken) {
			t.Errorf("%s: want ErrInvalidToken, got %v", name, err)
		}
	}
}

func unsigned(t *testing.T) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"iss": googletest.Issuer, "aud": clientID, "sub": "g-1", "exp": time.Now().Add(time.Hour).Unix()})
	raw, err := tok.SignedString([]byte("guessable"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestVerifyWithoutClientIDRefusesEverything(t *testing.T) {
	g := googletest.New(t)
	if _, err := google.NewVerifier("").WithJWKSURL(g.Server.URL).Verify(context.Background(), g.Sign(t, googletest.Token{Subject: "g", Audience: ""})); !errors.Is(err, google.ErrInvalidToken) {
		t.Fatalf("no client id must refuse: %v", err)
	}
}
