package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/trezutil/trezutil/internal/application/identity"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
	"github.com/trezutil/trezutil/packages/common"
)

// GoogleAuthenticator is implemented by identity.Service ("Sign in with Google").
type GoogleAuthenticator interface {
	GoogleClientID() string
	GoogleNonce() (string, int64, error)
	SignInWithGoogle(ctx context.Context, credential, nonce string) (*identity.Session, error)
	LinkGoogle(ctx context.Context, userID, credential, nonce string) (*domain.ExternalIdentity, error)
	UnlinkGoogle(ctx context.Context, userID string) error
	Identities(ctx context.Context, userID string) ([]domain.ExternalIdentity, error)
}

// GoogleAuthHandler serves sign-in with Google:
//
//	GET    /api/v1/auth/providers           {"phone":{"enabled":true},"google":{"enabled":true,"client_id":"…"}} (public)
//	POST   /api/v1/auth/google/nonce        {"nonce":"…","expires_at":"…"}
//	POST   /api/v1/auth/google              {"credential":"<Google ID token>","nonce":"…"} → a session, like verify-otp
//	GET    /api/v1/auth/identities          the signed-in account's sign-in methods
//	POST   /api/v1/auth/google/link         link Google to the signed-in account (same body as sign-in)
//	DELETE /api/v1/auth/identities/google   unlink (refused when it is the only way to sign in)
type GoogleAuthHandler struct {
	google  GoogleAuthenticator
	users   interface{ Me(context.Context, string) (*domain.User, error) }
	limiter *common.WindowLimiter
}

// NewGoogleAuthHandler builds the handler; perIPPer10Min bounds sign-in attempts per address.
func NewGoogleAuthHandler(g GoogleAuthenticator, users interface{ Me(context.Context, string) (*domain.User, error) }, perIPPer10Min int) *GoogleAuthHandler {
	return &GoogleAuthHandler{google: g, users: users, limiter: common.NewWindowLimiter(perIPPer10Min, 10*time.Minute)}
}

// Register mounts the routes.
func (h *GoogleAuthHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/auth/providers", h.providers)
	mux.HandleFunc("POST /api/v1/auth/google/nonce", h.nonce)
	mux.HandleFunc("POST /api/v1/auth/google", h.signIn)
	mux.HandleFunc("GET /api/v1/auth/identities", h.identities)
	mux.HandleFunc("POST /api/v1/auth/google/link", h.link)
	mux.HandleFunc("DELETE /api/v1/auth/identities/google", h.unlink)
}

func (h *GoogleAuthHandler) fail(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, identity.ErrGoogleUnavailable):
		writeJSON(w, ErrorResponse{Error: Error{Code: "GOOGLE_UNAVAILABLE", Message: "Sign-in with Google is not available"}}, http.StatusNotFound)
	case errors.Is(err, identity.ErrGoogleToken):
		auth.WriteUnauthorized(w, "GOOGLE_TOKEN_INVALID", "The Google sign-in could not be verified")
	case errors.Is(err, identity.ErrAccountExists):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ACCOUNT_EXISTS", Message: "An account already uses this email address: sign in the usual way, then link Google in your settings"}}, http.StatusConflict)
	case errors.Is(err, identity.ErrGoogleLinkedElsewhere):
		writeJSON(w, ErrorResponse{Error: Error{Code: "GOOGLE_LINKED_ELSEWHERE", Message: "This Google account is linked to another account"}}, http.StatusConflict)
	case errors.Is(err, identity.ErrLastSignInMethod):
		writeJSON(w, ErrorResponse{Error: Error{Code: "LAST_SIGN_IN_METHOD", Message: "This is your only way to sign in"}}, http.StatusConflict)
	case errors.Is(err, domain.ErrIdentityNotLinked):
		writeJSON(w, ErrorResponse{Error: Error{Code: "NOT_LINKED", Message: "No Google account is linked"}}, http.StatusNotFound)
	case errors.Is(err, domain.ErrUserSuspended):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ACCOUNT_SUSPENDED", Message: "This account is suspended"}}, http.StatusForbidden)
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: "GOOGLE_SIGN_IN_FAILED", Message: "The sign-in could not be completed"}}, http.StatusInternalServerError)
	}
}

func (h *GoogleAuthHandler) providers(w http.ResponseWriter, r *http.Request) {
	clientID := h.google.GoogleClientID()
	writeJSON(w, map[string]interface{}{
		"phone":  map[string]bool{"enabled": true},
		"google": map[string]interface{}{"enabled": clientID != "", "client_id": clientID},
	}, http.StatusOK)
}

func (h *GoogleAuthHandler) nonce(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	n, exp, err := h.google.GoogleNonce()
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, map[string]string{"nonce": n, "expires_at": isoTime(exp)}, http.StatusOK)
}

type googleCredential struct {
	Credential string `json:"credential"`
	Nonce      string `json:"nonce"`
}

func (h *GoogleAuthHandler) read(w http.ResponseWriter, r *http.Request) (*googleCredential, bool) {
	w.Header().Set("Cache-Control", "no-store")
	if !h.limiter.Allow(common.ClientIP(r)) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "RATE_LIMITED", Message: "Too many attempts; try again later"}}, http.StatusTooManyRequests)
		return nil, false
	}
	var body googleCredential
	if err := readJSONBody(r, &body); err != nil || body.Credential == "" || body.Nonce == "" || len(body.Credential) > 8192 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "credential and nonce are required"}}, http.StatusBadRequest)
		return nil, false
	}
	return &body, true
}

func (h *GoogleAuthHandler) signIn(w http.ResponseWriter, r *http.Request) {
	body, ok := h.read(w, r)
	if !ok {
		return
	}
	session, err := h.google.SignInWithGoogle(r.Context(), body.Credential, body.Nonce)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, sessionJSON(session), http.StatusOK)
}

func identityJSON(id domain.ExternalIdentity) map[string]interface{} {
	return map[string]interface{}{"provider": id.Provider, "email": id.Email, "name": id.Name, "created_at": isoTime(id.CreatedAt)}
}

func (h *GoogleAuthHandler) identities(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	user, err := h.users.Me(r.Context(), userID)
	if err != nil {
		auth.WriteUnauthorized(w, "USER_NOT_FOUND", "Account not found")
		return
	}
	list, err := h.google.Identities(r.Context(), userID)
	if err != nil {
		h.fail(w, err)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, id := range list {
		items = append(items, identityJSON(id))
	}
	writeJSON(w, map[string]interface{}{"phone": user.Phone, "items": items, "google_enabled": h.google.GoogleClientID() != ""}, http.StatusOK)
}

func (h *GoogleAuthHandler) link(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	body, ok := h.read(w, r)
	if !ok {
		return
	}
	id, err := h.google.LinkGoogle(r.Context(), userID, body.Credential, body.Nonce)
	if err != nil {
		h.fail(w, err)
		return
	}
	writeJSON(w, identityJSON(*id), http.StatusOK)
}

func (h *GoogleAuthHandler) unlink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	if err := h.google.UnlinkGoogle(r.Context(), userID); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
