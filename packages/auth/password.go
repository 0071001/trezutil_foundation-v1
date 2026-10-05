package auth

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

/*
 * Password hashing for staff accounts (admin, operator, moderator).
 *
 * PBKDF2-HMAC-SHA256 with a random 16-byte salt and 600 000 iterations, the
 * OWASP minimum for this construction, from the Go standard library (no extra
 * dependency). The stored form carries its own parameters so the cost can be
 * raised later without breaking old hashes:
 *
 *	pbkdf2-sha256$600000$<salt base64>$<key base64>
 */

const (
	passwordScheme     = "pbkdf2-sha256"
	passwordIterations = 600_000
	passwordSaltBytes  = 16
	passwordKeyBytes   = 32

	// MinPasswordLength and MaxPasswordLength bound what a staff password may be.
	MinPasswordLength = 10
	MaxPasswordLength = 128
)

// ErrWeakPassword is returned, with the reason, when a password is refused.
var ErrWeakPassword = errors.New("weak password")

// HashPassword returns the storable hash of password.
func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, passwordKeyBytes)
	if err != nil {
		return "", fmt.Errorf("derive key: %w", err)
	}
	enc := base64.RawStdEncoding
	return strings.Join([]string{passwordScheme, strconv.Itoa(passwordIterations), enc.EncodeToString(salt), enc.EncodeToString(key)}, "$"), nil
}

// VerifyPassword reports whether password matches a hash made by HashPassword.
// The comparison takes the same time whatever the password; a malformed hash
// never matches.
func VerifyPassword(hash, password string) bool {
	parts := strings.Split(hash, "$")
	if len(parts) != 4 || parts[0] != passwordScheme {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations < 1 || iterations > 10_000_000 {
		return false
	}
	enc := base64.RawStdEncoding
	salt, err := enc.DecodeString(parts[2])
	if err != nil || len(salt) == 0 {
		return false
	}
	want, err := enc.DecodeString(parts[3])
	if err != nil || len(want) == 0 {
		return false
	}
	got, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(want))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(got, want) == 1
}

var (
	dummyOnce sync.Once
	dummyHash string
)

// BurnPasswordCheck spends the time of one password verification. A login for
// an unknown email calls it so the response time does not reveal whether the
// account exists.
func BurnPasswordCheck(password string) {
	dummyOnce.Do(func() { dummyHash, _ = HashPassword("not-a-real-password") })
	_ = VerifyPassword(dummyHash, password)
}

var commonPasswords = map[string]bool{
	"password": true, "password1": true, "password123": true, "123456789": true, "1234567890": true,
	"qwertyuiop": true, "azertyuiop": true, "motdepasse": true, "motdepasse1": true, "trezutil": true,
	"admin12345": true, "administrator": true, "iloveyou123": true, "letmein1234": true,
}

// ValidatePassword checks a new staff password: long enough, not too long,
// mixing at least three kinds of characters (lower case, upper case, digit,
// symbol), not a well-known password, and not built from the account's email.
func ValidatePassword(password, email string) error {
	n := len([]rune(password))
	switch {
	case n < MinPasswordLength:
		return fmt.Errorf("%w: at least %d characters are required", ErrWeakPassword, MinPasswordLength)
	case n > MaxPasswordLength:
		return fmt.Errorf("%w: at most %d characters are allowed", ErrWeakPassword, MaxPasswordLength)
	}
	var lower, upper, digit, symbol bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			lower = true
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsDigit(r):
			digit = true
		default:
			symbol = true
		}
	}
	kinds := 0
	for _, ok := range []bool{lower, upper, digit, symbol} {
		if ok {
			kinds++
		}
	}
	if kinds < 3 {
		return fmt.Errorf("%w: mix at least three of lower case, upper case, digits and symbols", ErrWeakPassword)
	}
	low := strings.ToLower(password)
	if commonPasswords[low] {
		return fmt.Errorf("%w: this password is too common", ErrWeakPassword)
	}
	if local, _, ok := strings.Cut(strings.ToLower(strings.TrimSpace(email)), "@"); ok && len(local) >= 6 && strings.Contains(low, local) {
		return fmt.Errorf("%w: it must not contain the email address", ErrWeakPassword)
	}
	return nil
}
