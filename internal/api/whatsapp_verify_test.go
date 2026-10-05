package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const testVerifyToken = "verify-token-for-tests"

// handshake serves a GET to the real route pattern used by conversation-service.
func handshake(t *testing.T, configured string, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/webhooks/whatsapp", VerifySubscriptionH(configured))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/webhooks/whatsapp?"+query.Encode(), nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func assertForbidden(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != "FORBIDDEN" {
		t.Fatalf("body = %q, want a FORBIDDEN error", rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), testVerifyToken) {
		t.Fatal("the error body must not echo the configured token")
	}
}

func TestVerifySubscription_ValidReturnsExactChallenge(t *testing.T) {
	rec := handshake(t, testVerifyToken, url.Values{
		"hub.mode": {"subscribe"}, "hub.verify_token": {testVerifyToken}, "hub.challenge": {"1158201444"},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "1158201444" {
		t.Fatalf("body = %q, want the exact challenge", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/plain" {
		t.Fatalf("Content-Type = %q, want text/plain", ct)
	}
}

func TestVerifySubscription_InvalidToken(t *testing.T) {
	for name, got := range map[string]string{
		"wrong":          "not-the-token",
		"prefix":         testVerifyToken[:len(testVerifyToken)-1],
		"extra suffix":   testVerifyToken + "x",
		"different case": strings.ToUpper(testVerifyToken),
	} {
		t.Run(name, func(t *testing.T) {
			assertForbidden(t, handshake(t, testVerifyToken, url.Values{
				"hub.mode": {"subscribe"}, "hub.verify_token": {got}, "hub.challenge": {"42"},
			}))
		})
	}
}

func TestVerifySubscription_MissingToken(t *testing.T) {
	assertForbidden(t, handshake(t, testVerifyToken, url.Values{
		"hub.mode": {"subscribe"}, "hub.challenge": {"42"},
	}))
}

func TestVerifySubscription_WrongOrMissingMode(t *testing.T) {
	for name, q := range map[string]url.Values{
		"wrong mode":   {"hub.mode": {"unsubscribe"}, "hub.verify_token": {testVerifyToken}, "hub.challenge": {"42"}},
		"missing mode": {"hub.verify_token": {testVerifyToken}, "hub.challenge": {"42"}},
	} {
		t.Run(name, func(t *testing.T) {
			assertForbidden(t, handshake(t, testVerifyToken, q))
		})
	}
}

// With no WHATSAPP_VERIFY_TOKEN configured the handshake must never succeed,
// not even for an empty presented token.
func TestVerifySubscription_UnconfiguredTokenRefusesEverything(t *testing.T) {
	assertForbidden(t, handshake(t, "", url.Values{
		"hub.mode": {"subscribe"}, "hub.verify_token": {""}, "hub.challenge": {"42"},
	}))
	assertForbidden(t, handshake(t, "", url.Values{"hub.mode": {"subscribe"}, "hub.challenge": {"42"}}))
}

// Current behaviour, pinned: a valid token without a challenge is answered 200
// with an empty body (Meta always sends a challenge, so nothing is echoed).
func TestVerifySubscription_MissingChallengeKeepsExistingBehaviour(t *testing.T) {
	rec := handshake(t, testVerifyToken, url.Values{
		"hub.mode": {"subscribe"}, "hub.verify_token": {testVerifyToken},
	})
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("got %d %q, want 200 with an empty body", rec.Code, rec.Body.String())
	}
}

// The handshake is GET only; POST on the same path belongs to the signed webhook.
func TestVerifySubscription_RouteIsGetOnly(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/webhooks/whatsapp", VerifySubscriptionH(testVerifyToken))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/whatsapp", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405 from the GET-only pattern", rec.Code)
	}
}
