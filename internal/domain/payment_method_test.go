package domain

import (
	"testing"
)

func TestPaymentMethodType_IsValid(t *testing.T) {
	valid := []PaymentMethodType{
		PaymentMethodMobileMoney,
		PaymentMethodCard,
		PaymentMethodBankTransfer,
		PaymentMethodCash,
		PaymentMethodWallet,
		PaymentMethodOther,
	}
	for _, m := range valid {
		if !m.IsValid() {
			t.Errorf("expected %s to be valid", m)
		}
	}
}

func TestPaymentMethodType_Invalid(t *testing.T) {
	invalid := PaymentMethodType("BITCOIN")
	if invalid.IsValid() {
		t.Error("expected BITCOIN to be invalid")
	}
}

func TestNewPaymentMethod(t *testing.T) {
	m := NewPaymentMethod("org-1", PaymentMethodCard, "token_ref_123", "provider-1", 1000)

	if m.TenantID != "org-1" {
		t.Errorf("expected tenant org-1, got %s", m.TenantID)
	}
	if m.TokenRef != "token_ref_123" {
		t.Errorf("expected token_ref_123, got %s", m.TokenRef)
	}
	if m.MethodType != PaymentMethodCard {
		t.Errorf("expected CARD, got %s", m.MethodType)
	}
}
