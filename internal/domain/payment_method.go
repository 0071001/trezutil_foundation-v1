package domain

import "context"

// PaymentMethod is a tokenizable reference to a customer's payment instrument.
//
// This struct NEVER stores sensitive credentials, raw card data, PINs, or
// provider API secrets. It only holds a tokenized reference (PaymentMethodID)
// that maps to a provider-side token or vault reference.
type PaymentMethod struct {
	ID         string
	TenantID   string
	MethodType PaymentMethodType
	TokenRef   string
	ProviderID string
	Details    map[string]interface{}
	CreatedAt  int64
	UpdatedAt  int64
}

// NewPaymentMethod creates a new PaymentMethod reference.
// methodID is a tokenized reference (never raw credentials).
func NewPaymentMethod(tenantID string, methodType PaymentMethodType, tokenRef string, providerID string, now int64) *PaymentMethod {
	return &PaymentMethod{
		ID:         "",
		TenantID:   tenantID,
		MethodType: methodType,
		TokenRef:   tokenRef,
		ProviderID: providerID,
		Details:    make(map[string]interface{}),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// PaymentMethodRepository defines the interface for payment method persistence.
type PaymentMethodRepository interface {
	Create(ctx context.Context, method *PaymentMethod) error
	FindByID(ctx context.Context, tenantID, id string) (*PaymentMethod, error)
	FindByTenantID(ctx context.Context, tenantID string, offset, limit int) ([]*PaymentMethod, error)
	Update(ctx context.Context, method *PaymentMethod) error
	Delete(ctx context.Context, tenantID, id string) error
}
