package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

type fakeGoogle struct {
	clientID string
	signIn   error
}

func (f *fakeGoogle) GoogleClientID() string              { return f.clientID }
func (f *fakeGoogle) GoogleNonce() (string, int64, error) { return "n", 1, nil }
func (f *fakeGoogle) SignInWithGoogle(context.Context, string, string) (*identity.Session, error) {
	if f.signIn != nil {
		return nil, f.signIn
	}
	return &identity.Session{User: &domain.User{ID: "u-1", DisplayName: "Ama"}, AccessToken: "a", RefreshToken: "r"}, nil
}
func (f *fakeGoogle) LinkGoogle(context.Context, string, string, string) (*domain.ExternalIdentity, error) {
	return &domain.ExternalIdentity{Provider: "google", Email: "ama@example.com"}, nil
}
func (f *fakeGoogle) UnlinkGoogle(context.Context, string) error { return identity.ErrLastSignInMethod }
func (f *fakeGoogle) Identities(context.Context, string) ([]domain.ExternalIdentity, error) {
	return []domain.ExternalIdentity{{Provider: "google", Email: "ama@example.com"}}, nil
}

type fakeMe struct{}

func (fakeMe) Me(context.Context, string) (*domain.User, error) { return &domain.User{ID: "u-1"}, nil }

func googleCall(g *fakeGoogle, method, path, user, body string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	NewGoogleAuthHandler(g, fakeMe{}, 30).Register(mux)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: user}))
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestGoogleAuthRoutes(t *testing.T) {
	if rec := googleCall(&fakeGoogle{}, http.MethodGet, "/api/v1/auth/providers", "", ""); !strings.Contains(rec.Body.String(), `"enabled":false`) {
		t.Fatalf("no client id: Google is disabled: %s", rec.Body)
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodGet, "/api/v1/auth/providers", "", ""); !strings.Contains(rec.Body.String(), `"client_id":"cid"`) {
		t.Fatalf("the public client id is served: %s", rec.Body)
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodPost, "/api/v1/auth/google", "", `{"credential":"x"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("a nonce is required: %d", rec.Code)
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodPost, "/api/v1/auth/google", "", `{"credential":"x","nonce":"n"}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"access_token":"a"`) {
		t.Fatalf("sign-in returns a session like verify-otp: %d %s", rec.Code, rec.Body)
	}
	for err, want := range map[error]int{identity.ErrAccountExists: 409, identity.ErrGoogleToken: 401, identity.ErrGoogleUnavailable: 404, domain.ErrUserSuspended: 403} {
		if rec := googleCall(&fakeGoogle{clientID: "cid", signIn: err}, http.MethodPost, "/api/v1/auth/google", "", `{"credential":"x","nonce":"n"}`); rec.Code != want {
			t.Fatalf("%v: want %d, got %d", err, want, rec.Code)
		}
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodPost, "/api/v1/auth/google/link", "", `{"credential":"x","nonce":"n"}`); rec.Code != http.StatusUnauthorized {
		t.Fatalf("linking needs a session: %d", rec.Code)
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodDelete, "/api/v1/auth/identities/google", "u-1", ""); rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "LAST_SIGN_IN_METHOD") {
		t.Fatalf("the only method cannot be removed: %d %s", rec.Code, rec.Body)
	}
	if rec := googleCall(&fakeGoogle{clientID: "cid"}, http.MethodGet, "/api/v1/auth/identities", "u-1", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ama@example.com") {
		t.Fatalf("identities: %d %s", rec.Code, rec.Body)
	}
}
