package auth

import (
	"fmt"
	"os"
	"strings"
)

// Strict reports whether the process must refuse weak or placeholder secrets:
// everything except APP_ENV=development and APP_ENV=test (an unset APP_ENV is
// strict, so a forgotten variable never weakens a deployment).
func Strict() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV"))) {
	case "development", "test":
		return false
	}
	return true
}

// placeholderWords mark a value copied from documentation or an example.
var placeholderWords = []string{
	"changeme", "change-me", "change_me", "replace", "placeholder", "example",
	"password", "default", "secret", "your-", "your_", "xxxx", "e2e", "test",
}

// ValidateSecret rejects a signing/HMAC secret that is short, low-entropy or
// visibly a placeholder. The check is on shape only; it never echoes the value.
func ValidateSecret(name, secret string) error {
	if len(secret) < MinSecretLength {
		return fmt.Errorf("%s must be at least %d characters", name, MinSecretLength)
	}
	distinct := map[rune]struct{}{}
	for _, r := range secret {
		distinct[r] = struct{}{}
	}
	if len(distinct) < 10 {
		return fmt.Errorf("%s has too little variety (use random characters, e.g. openssl rand -base64 48)", name)
	}
	lower := strings.ToLower(secret)
	for _, w := range placeholderWords {
		if strings.Contains(lower, w) {
			return fmt.Errorf("%s looks like a placeholder value; generate a random one (openssl rand -base64 48)", name)
		}
	}
	return nil
}
