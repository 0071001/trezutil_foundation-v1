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

// Claims represents the JWT claims.
type Claims struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
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
		UserID: userID,
		Roles:  roleStrings,
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
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
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

// ValidateAccessToken validates a production JWT access token.
func (s *jwtTokenService) ValidateAccessToken(tokenString string) (string, error) {
	if s.config.SecretKey == "" {
		return "", errors.New("JWT secret key is not configured")
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.SecretKey), nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return "", errors.New("invalid token claims")
	}

	// Validate issuer
	if s.config.Issuer != "" && claims.Issuer != s.config.Issuer {
		return "", errors.New("invalid issuer")
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
			return "", errors.New("invalid audience")
		}
	}

	return claims.UserID, nil
}

// RotateRefreshToken rotates a refresh token.
func (s *jwtTokenService) RotateRefreshToken(oldToken string) (string, error) {
	// Validate old token
	userID, err := s.ValidateAccessToken(oldToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %w", err)
	}

	// Issue new refresh token
	return s.IssueRefreshToken(userID)
}
