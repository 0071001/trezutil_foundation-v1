package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestWebhookSignatureVerifier_ValidSignature(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{"event_type":"payment.succeeded"}`)
	mac := hmac.New(sha256.New, []byte("test-secret-key"))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	if err := verifier.Verify("stripe", payload, signature); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestWebhookSignatureVerifier_ValidSignatureWithPrefix(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{"event_type":"payment.succeeded"}`)
	mac := hmac.New(sha256.New, []byte("test-secret-key"))
	mac.Write(payload)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if err := verifier.Verify("stripe", payload, signature); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestWebhookSignatureVerifier_InvalidSignature(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{"event_type":"payment.succeeded"}`)

	if err := verifier.Verify("stripe", payload, "invalid-signature"); err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

func TestWebhookSignatureVerifier_NoSecretConfigured(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{}`)
	if err := verifier.Verify("paypal", payload, "some-signature"); err == nil {
		t.Fatal("expected error for unconfigured provider, got nil")
	}
}

func TestWebhookSignatureVerifier_EmptySecretConfig(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{})

	payload := []byte(`{}`)
	if err := verifier.Verify("stripe", payload, "some-signature"); err == nil {
		t.Fatal("expected error for empty secret map, got nil")
	}
}

func TestWebhookSignatureVerifier_TamperedPayload(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{"event_type":"payment.succeeded"}`)
	mac := hmac.New(sha256.New, []byte("test-secret-key"))
	mac.Write(payload)
	signature := hex.EncodeToString(mac.Sum(nil))

	tamperedPayload := []byte(`{"event_type":"payment.failed"}`)
	if err := verifier.Verify("stripe", tamperedPayload, signature); err == nil {
		t.Fatal("expected error for tampered payload, got nil")
	}
}

func TestWebhookSignatureVerifier_ReturnsErrWebhookSignatureInvalid(t *testing.T) {
	verifier := NewWebhookSignatureVerifier(WebhookVerifierConfig{
		ProviderSecrets: map[string]string{
			"stripe": "test-secret-key",
		},
	})

	payload := []byte(`{}`)
	err := verifier.Verify("stripe", payload, "wrong-signature")
	if err != domain.ErrWebhookSignatureInvalid {
		t.Fatalf("expected ErrWebhookSignatureInvalid, got %v", err)
	}
}
