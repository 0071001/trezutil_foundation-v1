package commerce

import (
	"context"
	"fmt"
	"strconv"

	"github.com/trezutil/trezutil/internal/domain"
)

type PaymentUseCases interface {
	CreatePayment(ctx context.Context, orgID, actorID, orderID string, amount domain.Money,
		methodType domain.PaymentMethodType, methodID, idempotencyKey string,
		metadata map[string]interface{}) (*domain.Payment, error)
	CancelPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error)
	CapturePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string,
		amount *domain.Money) (*domain.Payment, error)
	RefundPayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string,
		amount *domain.Money) (*domain.Payment, *domain.Refund, error)
	GetPayment(ctx context.Context, orgID, paymentID string) (*domain.Payment, error)
	GetPaymentStatus(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error)
	ListRefunds(ctx context.Context, orgID, paymentID string, offset, limit int) ([]*domain.Refund, error)
	AuthorizePayment(ctx context.Context, orgID, actorID, paymentID, idempotencyKey string) (*domain.Payment, error)
}

// InitiatePaymentHandler handles INITIATE_PAYMENT actions.
type InitiatePaymentHandler struct {
	paymentUseCases PaymentUseCases
}

func NewInitiatePaymentHandler(useCases PaymentUseCases) *InitiatePaymentHandler {
	return &InitiatePaymentHandler{paymentUseCases: useCases}
}

func (h *InitiatePaymentHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	orderID, _ := parameters["order_id"].(string)
	if orderID == "" {
		return nil, fmt.Errorf("order_id parameter is required")
	}

	amountMinor, _ := parameters["amount_minor"].(float64)
	amountMinorInt := int64(amountMinor)
	if amountMinorInt <= 0 {
		return nil, fmt.Errorf("amount_minor parameter is required and must be positive")
	}

	currency, _ := parameters["currency"].(string)
	if currency == "" {
		currency = "USD"
	}

	methodTypeStr, _ := parameters["payment_method_type"].(string)
	methodType := domain.PaymentMethodType(methodTypeStr)
	if !methodType.IsValid() {
		return nil, fmt.Errorf("invalid payment_method_type: %s", methodTypeStr)
	}

	methodID, _ := parameters["payment_method_id"].(string)
	if methodID == "" {
		return nil, fmt.Errorf("payment_method_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	var metadata map[string]interface{}
	if rawMeta, ok := parameters["metadata"].(map[string]interface{}); ok {
		metadata = rawMeta
	}

	p, err := h.paymentUseCases.CreatePayment(ctx, orgID, actor.ID, orderID,
		domain.Money{AmountMinor: amountMinorInt, Currency: currency},
		methodType, methodID, idempotencyKey, metadata)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"payment_id":          p.ID,
		"status":              string(p.Status),
		"amount":              p.Amount.AmountMinor,
		"currency":            p.Amount.Currency,
		"provider_id":         p.ProviderID,
		"provider_payment_id": p.ProviderPaymentID,
	}, nil
}

func (h *InitiatePaymentHandler) ActionType() domain.CommerceActionType {
	return domain.ActionInitiatePayment
}

// CancelPaymentHandler handles CANCEL_PAYMENT actions.
type CancelPaymentHandler struct {
	paymentUseCases PaymentUseCases
}

func NewCancelPaymentHandler(useCases PaymentUseCases) *CancelPaymentHandler {
	return &CancelPaymentHandler{paymentUseCases: useCases}
}

func (h *CancelPaymentHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	p, err := h.paymentUseCases.CancelPayment(ctx, orgID, actor.ID, paymentID, idempotencyKey)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"payment_id":   p.ID,
		"status":       string(p.Status),
		"cancelled_at": p.CancelledAt,
	}, nil
}

func (h *CancelPaymentHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCancelPayment
}

// RefundPaymentHandler handles REFUND_PAYMENT actions.
type RefundPaymentHandler struct {
	paymentUseCases PaymentUseCases
}

func NewRefundPaymentHandler(useCases PaymentUseCases) *RefundPaymentHandler {
	return &RefundPaymentHandler{paymentUseCases: useCases}
}

func (h *RefundPaymentHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	var amount *domain.Money
	if amountStr, ok := parameters["amount_minor"].(string); ok && amountStr != "" {
		if minor, err := strconv.ParseInt(amountStr, 10, 64); err == nil && minor > 0 {
			currency, _ := parameters["currency"].(string)
			if currency == "" {
				currency = "USD"
			}
			amount = &domain.Money{AmountMinor: minor, Currency: currency}
		}
	}

	p, r, err := h.paymentUseCases.RefundPayment(ctx, orgID, actor.ID, paymentID, idempotencyKey, amount)
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"payment_id": p.ID,
		"status":     string(p.Status),
	}
	if r != nil {
		result["refund_id"] = r.ID
		result["refund_status"] = string(r.Status)
		result["refund_amount"] = r.Amount.AmountMinor
	}
	return result, nil
}

func (h *RefundPaymentHandler) ActionType() domain.CommerceActionType {
	return domain.ActionRefundPayment
}

// CapturePaymentHandler handles CAPTURE_PAYMENT actions.
type CapturePaymentHandler struct {
	paymentUseCases PaymentUseCases
}

func NewCapturePaymentHandler(useCases PaymentUseCases) *CapturePaymentHandler {
	return &CapturePaymentHandler{paymentUseCases: useCases}
}

func (h *CapturePaymentHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	var amount *domain.Money
	if amountStr, ok := parameters["amount_minor"].(string); ok && amountStr != "" {
		if minor, err := strconv.ParseInt(amountStr, 10, 64); err == nil && minor > 0 {
			currency, _ := parameters["currency"].(string)
			if currency == "" {
				currency = "USD"
			}
			amount = &domain.Money{AmountMinor: minor, Currency: currency}
		}
	}

	p, err := h.paymentUseCases.CapturePayment(ctx, orgID, actor.ID, paymentID, idempotencyKey, amount)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"payment_id":  p.ID,
		"status":      string(p.Status),
		"captured_at": p.CapturedAt,
	}, nil
}

func (h *CapturePaymentHandler) ActionType() domain.CommerceActionType {
	return domain.ActionCapturePayment
}

// GetPaymentStatusHandler handles GET_PAYMENT_STATUS actions.
type GetPaymentStatusHandler struct {
	paymentUseCases PaymentUseCases
}

func NewGetPaymentStatusHandler(useCases PaymentUseCases) *GetPaymentStatusHandler {
	return &GetPaymentStatusHandler{paymentUseCases: useCases}
}

func (h *GetPaymentStatusHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	p, err := h.paymentUseCases.GetPaymentStatus(ctx, orgID, actor.ID, paymentID, idempotencyKey)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"payment_id":          p.ID,
		"status":              string(p.Status),
		"amount":              p.Amount.AmountMinor,
		"currency":            p.Amount.Currency,
		"provider_id":         p.ProviderID,
		"provider_payment_id": p.ProviderPaymentID,
		"authorized_at":       p.AuthorizedAt,
		"captured_at":         p.CapturedAt,
		"refunded_amount":     p.RefundedAmount.AmountMinor,
	}, nil
}

func (h *GetPaymentStatusHandler) ActionType() domain.CommerceActionType {
	return domain.ActionGetPaymentStatus
}

// GetRefundStatusHandler handles GET_REFUND_STATUS actions.
type GetRefundStatusHandler struct {
	paymentUseCases PaymentUseCases
}

func NewGetRefundStatusHandler(useCases PaymentUseCases) *GetRefundStatusHandler {
	return &GetRefundStatusHandler{paymentUseCases: useCases}
}

func (h *GetRefundStatusHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	refundID, _ := parameters["refund_id"].(string)
	if refundID == "" {
		return nil, fmt.Errorf("refund_id parameter is required")
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required for refund status")
	}

	refunds, err := h.paymentUseCases.ListRefunds(ctx, orgID, paymentID, 0, 100)
	if err != nil {
		return nil, err
	}

	for _, r := range refunds {
		if r.ID == refundID {
			return map[string]interface{}{
				"refund_id":          r.ID,
				"status":             string(r.Status),
				"amount":             r.Amount.AmountMinor,
				"currency":           r.Amount.Currency,
				"provider_refund_id": r.ProviderRefundID,
				"processed_at":       r.ProcessedAt,
				"created_at":         r.CreatedAt,
			}, nil
		}
	}

	return nil, fmt.Errorf("refund %s not found", refundID)
}

func (h *GetRefundStatusHandler) ActionType() domain.CommerceActionType {
	return domain.ActionGetRefundStatus
}

// AuthorizePaymentHandler handles AUTHORIZE_PAYMENT actions.
type AuthorizePaymentHandler struct {
	paymentUseCases PaymentUseCases
}

func NewAuthorizePaymentHandler(useCases PaymentUseCases) *AuthorizePaymentHandler {
	return &AuthorizePaymentHandler{paymentUseCases: useCases}
}

func (h *AuthorizePaymentHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	orgID := actor.OrgID
	if orgID == "" {
		orgID = actor.ID
	}

	paymentID, _ := parameters["payment_id"].(string)
	if paymentID == "" {
		return nil, fmt.Errorf("payment_id parameter is required")
	}

	idempotencyKey, _ := parameters["idempotency_key"].(string)

	p, err := h.paymentUseCases.AuthorizePayment(ctx, orgID, actor.ID, paymentID, idempotencyKey)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"payment_id":    p.ID,
		"status":        string(p.Status),
		"authorized_at": p.AuthorizedAt,
	}, nil
}

func (h *AuthorizePaymentHandler) ActionType() domain.CommerceActionType {
	return domain.ActionAuthorizePayment
}
