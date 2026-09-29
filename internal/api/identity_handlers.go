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

// IdentityUseCases is implemented by identity.Service.
type IdentityUseCases interface {
	RequestOTP(ctx context.Context, phone string) (string, int64, error)
	VerifyOTP(ctx context.Context, challengeID, code string) (*identity.Session, error)
	Refresh(ctx context.Context, refreshToken string) (*identity.Session, error)
	Me(ctx context.Context, userID string) (*domain.User, error)
}

// IdentityHandler serves /api/v1/auth/*.
type IdentityHandler struct {
	useCases  IdentityUseCases
	ipLimiter *common.WindowLimiter
}

// NewIdentityHandler creates an IdentityHandler. OTP requests are limited per
// client IP in addition to the per-phone limit enforced by the service.
func NewIdentityHandler(useCases IdentityUseCases) *IdentityHandler {
	return NewIdentityHandlerWithLimit(useCases, 20)
}

// NewIdentityHandlerWithLimit sets the per-IP OTP request limit per 10 minutes.
func NewIdentityHandlerWithLimit(useCases IdentityUseCases, perIPPer10Min int) *IdentityHandler {
	if perIPPer10Min <= 0 {
		perIPPer10Min = 20
	}
	return &IdentityHandler{useCases: useCases, ipLimiter: common.NewWindowLimiter(perIPPer10Min, 10*time.Minute)}
}

type userJSON struct {
	ID          string   `json:"id"`
	Phone       string   `json:"phone"`
	Email       *string  `json:"email,omitempty"`
	DisplayName string   `json:"display_name"`
	Status      string   `json:"status"`
	Roles       []string `json:"roles"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
	VerifiedAt  *string  `json:"verified_at,omitempty"`
}

func toUserJSON(u *domain.User) userJSON {
	roles := make([]string, 0, len(u.Roles))
	for _, r := range u.Roles {
		roles = append(roles, string(r))
	}
	out := userJSON{
		ID:          u.ID,
		Phone:       u.Phone,
		Email:       u.Email,
		DisplayName: u.DisplayName,
		Status:      string(u.Status),
		Roles:       roles,
		CreatedAt:   isoTime(u.CreatedAt),
		UpdatedAt:   isoTime(u.UpdatedAt),
	}
	if u.VerifiedAt != nil {
		v := isoTime(*u.VerifiedAt)
		out.VerifiedAt = &v
	}
	return out
}

func sessionJSON(s *identity.Session) map[string]interface{} {
	return map[string]interface{}{
		"user": toUserJSON(s.User),
		"tokens": map[string]string{
			"access_token":  s.AccessToken,
			"refresh_token": s.RefreshToken,
			"expires_at":    isoTime(s.ExpiresAt),
		},
	}
}

// RequestOTPH handles POST /api/v1/auth/request-otp
func (h *IdentityHandler) RequestOTPH(w http.ResponseWriter, r *http.Request) {
	if !h.ipLimiter.Allow(common.ClientIP(r)) {
		writeJSON(w, ErrorResponse{Error: Error{Code: "RATE_LIMITED", Message: "Too many requests; try again later"}}, http.StatusTooManyRequests)
		return
	}
	var req struct {
		Phone string `json:"phone"`
	}
	if err := readJSONBody(r, &req); err != nil || req.Phone == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "phone is required"}}, http.StatusBadRequest)
		return
	}
	id, expiresAt, err := h.useCases.RequestOTP(r.Context(), req.Phone)
	switch {
	case err == nil:
		writeJSON(w, map[string]interface{}{"request_id": id, "expires_at": isoTime(expiresAt)}, http.StatusOK)
	case errors.Is(err, domain.ErrInvalidPhone):
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PHONE", Message: "Enter a valid phone number"}}, http.StatusBadRequest)
	case errors.Is(err, domain.ErrOTPRateLimited):
		writeJSON(w, ErrorResponse{Error: Error{Code: "RATE_LIMITED", Message: err.Error()}}, http.StatusTooManyRequests)
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: "OTP_DELIVERY_FAILED", Message: "Could not send a verification code right now"}}, http.StatusServiceUnavailable)
	}
}

// VerifyOTPH handles POST /api/v1/auth/verify-otp
func (h *IdentityHandler) VerifyOTPH(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
	}
	if err := readJSONBody(r, &req); err != nil || req.RequestID == "" || req.Code == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "request_id and code are required"}}, http.StatusBadRequest)
		return
	}
	session, err := h.useCases.VerifyOTP(r.Context(), req.RequestID, req.Code)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, sessionJSON(session), http.StatusOK)
}

// RefreshH handles POST /api/v1/auth/refresh
func (h *IdentityHandler) RefreshH(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := readJSONBody(r, &req); err != nil || req.RefreshToken == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "refresh_token is required"}}, http.StatusBadRequest)
		return
	}
	session, err := h.useCases.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		h.writeAuthError(w, err)
		return
	}
	writeJSON(w, sessionJSON(session), http.StatusOK)
}

// MeH handles GET /api/v1/auth/users/me
func (h *IdentityHandler) MeH(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.RequireUser(w, r)
	if !ok {
		return
	}
	user, err := h.useCases.Me(r.Context(), userID)
	if err != nil {
		auth.WriteUnauthorized(w, "USER_NOT_FOUND", "Account not found")
		return
	}
	writeJSON(w, toUserJSON(user), http.StatusOK)
}

func (h *IdentityHandler) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrUserSuspended):
		writeJSON(w, ErrorResponse{Error: Error{Code: "ACCOUNT_SUSPENDED", Message: "This account is suspended"}}, http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalidOTP), errors.Is(err, domain.ErrUserNotFound):
		auth.WriteUnauthorized(w, "INVALID_CREDENTIALS", "The code is invalid or has expired")
	default:
		writeJSON(w, ErrorResponse{Error: Error{Code: "AUTH_FAILED", Message: "Sign-in failed"}}, http.StatusInternalServerError)
	}
}

// isoTime formats a Unix timestamp as RFC 3339 (UTC) for the auth API.
func isoTime(ts int64) string {
	if ts == 0 {
		return ""
	}
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}
