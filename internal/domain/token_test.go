package domain

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestJWTTokenService_IssueAccessToken validates token issuance.
func TestJWTTokenService_IssueAccessToken(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{
		SecretKey:       "test-secret-key-1234567890",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	token, err := service.IssueAccessToken("user-123", []Role{RoleSeller, RoleAdmin})
	if err != nil {
		t.Fatalf("Issuing token failed: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	if !strings.Contains(token, ".") {
		t.Error("Token should contain dots (JWT format)")
	}
}

// TestJWTTokenService_IssueAccessToken_EmptySecret ensures secret validation.
func TestJWTTokenService_IssueAccessToken_EmptySecret(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{})

	token, err := service.IssueAccessToken("user-123", []Role{RoleSeller})
	if err == nil {
		t.Error("Expected error when secret key is empty")
	}
	if token != "" {
		t.Error("Token should be empty when error occurs")
	}
}

// TestJWTTokenService_ValidateAccessToken validates token validation.
func TestJWTTokenService_ValidateAccessToken(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{
		SecretKey:       "test-secret-key-1234567890",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	token, err := service.IssueAccessToken("user-123", []Role{RoleSeller, RoleAdmin})
	if err != nil {
		t.Fatalf("Issuing token failed: %v", err)
	}

	userID, err := service.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("Validating token failed: %v", err)
	}

	if userID != "user-123" {
		t.Errorf("Expected user_id 'user-123', got %s", userID)
	}
}

// TestJWTTokenService_ValidateAccessToken_InvalidToken validates rejection of invalid tokens.
func TestJWTTokenService_ValidateAccessToken_InvalidToken(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{
		SecretKey:       "test-secret-key-1234567890",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	_, err := service.ValidateAccessToken("invalid.token.here")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

// TestJWTTokenService_ValidateAccessToken_WrongSecret validates secret mismatch.
func TestJWTTokenService_ValidateAccessToken_WrongSecret(t *testing.T) {
	service1 := NewJWTTokenService(JWTConfig{
		SecretKey:       "secret-1",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	service2 := NewJWTTokenService(JWTConfig{
		SecretKey:       "secret-2",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	token, err := service1.IssueAccessToken("user-123", []Role{RoleSeller})
	if err != nil {
		t.Fatalf("Issuing token failed: %v", err)
	}

	_, err = service2.ValidateAccessToken(token)
	if err == nil {
		t.Error("Expected error for token signed with different secret")
	}
}

// TestJWTTokenService_ClaimsParsing validates raw JWT claims parsing.
func TestJWTTokenService_ClaimsParsing(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{
		SecretKey:       "test-secret-key-1234567890",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	token, err := service.IssueAccessToken("user-456", []Role{RoleAdmin})
	if err != nil {
		t.Fatalf("Issuing token failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("Expected 3 parts in JWT, got %d", len(parts))
	}

	parser := jwt.NewParser()
	parsedClaims := &Claims{}
	_, err = parser.ParseWithClaims(token, parsedClaims, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret-key-1234567890"), nil
	})
	if err != nil {
		t.Fatalf("Parsing token failed: %v", err)
	}

	if parsedClaims.UserID != "user-456" {
		t.Errorf("Expected user_id 'user-456', got %s", parsedClaims.UserID)
	}

	if len(parsedClaims.Roles) != 1 || parsedClaims.Roles[0] != "admin" {
		t.Errorf("Expected role 'admin', got %v", parsedClaims.Roles)
	}
}

// TestJWTTokenService_TokenTypesAreNotInterchangeable ensures a refresh token
// cannot be used as an access token and vice versa.
func TestJWTTokenService_TokenTypesAreNotInterchangeable(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{
		SecretKey:       "test-secret-key-1234567890",
		Issuer:          "test-issuer",
		Audience:        "test-audience",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	})

	access, err := service.IssueAccessToken("user-123", []Role{RoleCustomer})
	if err != nil {
		t.Fatalf("issue access: %v", err)
	}
	refresh, err := service.IssueRefreshToken("user-123")
	if err != nil {
		t.Fatalf("issue refresh: %v", err)
	}

	if _, err := service.ValidateAccessToken(refresh); err == nil {
		t.Error("refresh token must not validate as an access token")
	}
	if _, err := service.ValidateRefreshToken(access); err == nil {
		t.Error("access token must not validate as a refresh token")
	}
	if _, err := service.RotateRefreshToken(access); err == nil {
		t.Error("access token must not be accepted for rotation")
	}
	if id, err := service.ValidateRefreshToken(refresh); err != nil || id != "user-123" {
		t.Errorf("valid refresh token rejected: id=%q err=%v", id, err)
	}
	rotated, err := service.RotateRefreshToken(refresh)
	if err != nil || rotated == "" {
		t.Fatalf("rotate failed: %v", err)
	}
}

// TestJWTTokenService_RejectsAlgNone ensures unsigned tokens are rejected.
func TestJWTTokenService_RejectsAlgNone(t *testing.T) {
	service := NewJWTTokenService(JWTConfig{SecretKey: "test-secret-key-1234567890", AccessTokenTTL: time.Minute})
	claims := &Claims{UserID: "u1", TokenType: TokenTypeAccess, RegisteredClaims: jwt.RegisteredClaims{
		Subject:   "u1",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}}
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("build unsigned token: %v", err)
	}
	if _, err := service.ValidateAccessToken(unsigned); err == nil {
		t.Error("alg=none token must be rejected")
	}
}
