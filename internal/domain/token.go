package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig holds the configuration for JWT token generation.
type JWTConfig struct {
	SecretKey       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string
	Audience        string
}

// jwtTokenService implements TokenService using JWT.
// This is a PRODUCTION implementation.
type jwtTokenService struct {
	config JWTConfig
}

// NewJWTTokenService creates a new production JWT token service.
func NewJWTTokenService(config JWTConfig) TokenService {
	return &jwtTokenService{config: config}
}

// Token types carried in the "typ" claim. Access and refresh tokens are signed
// with the same key, so the type must be checked explicitly: otherwise a
// long-lived refresh token would be accepted wherever an access token is.
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// Claims represents the JWT claims.
type Claims struct {
	UserID    string   `json:"user_id"`
	Roles     []string `json:"roles"`
	TokenType string   `json:"typ"`
	jwt.RegisteredClaims
}

// IssueAccessToken creates a production JWT access token.
func (s *jwtTokenService) IssueAccessToken(userID string, roles []Role) (string, error) {
	if s.config.SecretKey == "" {
		return "", errors.New("JWT secret key is not configured")
	}

	roleStrings := make([]string, len(roles))
	for i, role := range roles {
		roleStrings[i] = string(role)
	}

	claims := &Claims{
		UserID:    userID,
		Roles:     roleStrings,
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.AccessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.Issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{s.config.Audience},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// IssueRefreshToken creates a production JWT refresh token.
func (s *jwtTokenService) IssueRefreshToken(userID string) (string, error) {
	if s.config.SecretKey == "" {
		return "", errors.New("JWT secret key is not configured")
	}

	claims := &Claims{
		UserID:    userID,
		TokenType: TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        NewID(),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.config.RefreshTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    s.config.Issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{s.config.Audience},
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.SecretKey))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return tokenString, nil
}

// ValidateAccessToken validates a production JWT access token and returns its user ID.
func (s *jwtTokenService) ValidateAccessToken(tokenString string) (string, error) {
	claims, err := s.parse(tokenString, TokenTypeAccess)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// ValidateRefreshToken validates a production JWT refresh token and returns its user ID.
func (s *jwtTokenService) ValidateRefreshToken(tokenString string) (string, error) {
	claims, err := s.parse(tokenString, TokenTypeRefresh)
	if err != nil {
		return "", err
	}
	return claims.UserID, nil
}

// parse verifies signature, algorithm, expiry, issuer, audience and token type.
func (s *jwtTokenService) parse(tokenString, wantType string) (*Claims, error) {
	if s.config.SecretKey == "" {
		return nil, errors.New("JWT secret key is not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.SecretKey), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	if claims.TokenType != wantType {
		return nil, errors.New("invalid token type")
	}

	if claims.UserID == "" || claims.Subject != claims.UserID {
		return nil, errors.New("invalid token subject")
	}

	// Validate issuer
	if s.config.Issuer != "" && claims.Issuer != s.config.Issuer {
		return nil, errors.New("invalid issuer")
	}

	// Validate audience
	if s.config.Audience != "" {
		found := false
		for _, aud := range claims.Audience {
			if aud == s.config.Audience {
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("invalid audience")
		}
	}

	return claims, nil
}

// RotateRefreshToken validates a refresh token and issues a new one.
func (s *jwtTokenService) RotateRefreshToken(oldToken string) (string, error) {
	userID, err := s.ValidateRefreshToken(oldToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	return s.IssueRefreshToken(userID)
}
