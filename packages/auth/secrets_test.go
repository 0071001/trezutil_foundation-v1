package auth

import (
	"strings"
	"testing"
)

func TestValidateSecret(t *testing.T) {
	good := "k3J9xQ2vLm8Zp0RtYw5NbHcD7fGaSeUo1iXV4nMq6WA="
	if err := ValidateSecret("S", good); err != nil {
		t.Fatalf("a random secret was rejected: %v", err)
	}
	for name, bad := range map[string]string{
		"short":       "abc123",
		"low variety": strings.Repeat("ab", 20),
		"changeme":    "ChangeMe-k3J9xQ2vLm8Zp0RtYw5NbHcD7fGaSe",
		"e2e":         "e2e-secret-0123456789abcdef0123456789",
		"example":     "example-k3J9xQ2vLm8Zp0RtYw5NbHcD7fGaSeUo",
	} {
		err := ValidateSecret("S", bad)
		if err == nil {
			t.Errorf("%s accepted", name)
		} else if strings.Contains(err.Error(), bad) {
			t.Errorf("%s: the error echoes the secret", name)
		}
	}
}

func TestStrictUnlessDevelopmentOrTest(t *testing.T) {
	for env, want := range map[string]bool{"": true, "production": true, "staging": true, "development": false, "DEVELOPMENT": false, "test": false} {
		t.Setenv("APP_ENV", env)
		if Strict() != want {
			t.Errorf("APP_ENV=%q: Strict = %v, want %v", env, !want, want)
		}
	}
}

func TestConfigFromEnvRejectsPlaceholderSecretOutsideDevelopment(t *testing.T) {
	t.Setenv("JWT_SECRET", "changeme-changeme-changeme-changeme-1234")
	t.Setenv("APP_ENV", "production")
	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("a placeholder JWT secret must be refused in production")
	}
	t.Setenv("APP_ENV", "development")
	if _, err := ConfigFromEnv(); err != nil {
		t.Fatalf("development keeps the length-only rule: %v", err)
	}
}
