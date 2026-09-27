package payment

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/trezutil/trezutil/internal/domain"
)

type WebhookSignatureVerifier struct {
	secrets map[string]string
}

type WebhookVerifierConfig struct {
	ProviderSecrets map[string]string
}

func NewWebhookSignatureVerifier(config WebhookVerifierConfig) *WebhookSignatureVerifier {
	if config.ProviderSecrets == nil {
		config.ProviderSecrets = make(map[string]string)
	}
	return &WebhookSignatureVerifier{
		secrets: config.ProviderSecrets,
	}
}

func (v *WebhookSignatureVerifier) Verify(providerCode string, payload []byte, signature string) error {
	secret := v.secrets[providerCode]
	if secret == "" {
		return fmt.Errorf("no webhook secret configured for provider %s", providerCode)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if len(signature) > 7 && signature[:7] == "sha256=" {
		signature = signature[7:]
	}

	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return domain.ErrWebhookSignatureInvalid
	}

	return nil
}
