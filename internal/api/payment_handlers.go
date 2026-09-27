package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/trezutil/trezutil/internal/domain"
)

// PaymentUseCases defines the interface for payment operations in the API layer.
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

// PaymentWebhookProcessor defines the interface for processing payment webhooks.
type PaymentWebhookProcessor interface {
	ProcessWebhook(ctx context.Context, orgID, providerCode string, signature string, body []byte) error
}

// PaymentHandler handles payment HTTP requests.
type PaymentHandler struct {
	paymentUseCases PaymentUseCases
	webhookHandler  PaymentWebhookProcessor
}

// NewPaymentHandler creates a new PaymentHandler.
func NewPaymentHandler(useCases PaymentUseCases, webhook PaymentWebhookProcessor) *PaymentHandler {
	return &PaymentHandler{
		paymentUseCases: useCases,
		webhookHandler:  webhook,
	}
}

func (h *PaymentHandler) paymentIDFromPath(w http.ResponseWriter, r *http.Request) (string, bool) {
	pid := r.PathValue("payment_id")
	if pid == "" {
		pid = r.URL.Query().Get("payment_id")
	}
	if pid == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PAYMENT_ID", Message: "payment_id required"}}, http.StatusBadRequest)
		return "", false
	}
	return pid, true
}

func (h *PaymentHandler) orgIDFromQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORG_ID", Message: "organization_id required"}}, http.StatusBadRequest)
		return "", false
	}
	return orgID, true
}

func (h *PaymentHandler) actorIDFromQuery(w http.ResponseWriter, r *http.Request) (string, bool) {
	actorID := r.URL.Query().Get("user_id")
	if actorID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_USER_ID", Message: "user_id required"}}, http.StatusUnauthorized)
		return "", false
	}
	return actorID, true
}

// CreatePaymentH handles POST /api/v1/payments
func (h *PaymentHandler) CreatePaymentH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	actorID, ok := h.actorIDFromQuery(w, r)
	if !ok {
		return
	}

	var req struct {
		OrderID         string                 `json:"order_id"`
		AmountMinor     int64                  `json:"amount_minor"`
		Currency        string                 `json:"currency"`
		PaymentMethod   string                 `json:"payment_method_type"`
		PaymentMethodID string                 `json:"payment_method_id"`
		IdempotencyKey  string                 `json:"idempotency_key"`
		Metadata        map[string]interface{} `json:"metadata"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}
	if req.OrderID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_ORDER_ID", Message: "order_id required"}}, http.StatusBadRequest)
		return
	}
	if req.AmountMinor <= 0 {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_AMOUNT", Message: "amount_minor must be positive"}}, http.StatusBadRequest)
		return
	}
	if req.Currency == "" {
		req.Currency = "USD"
	}
	if req.PaymentMethodID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PAYMENT_METHOD", Message: "payment_method_id required"}}, http.StatusBadRequest)
		return
	}

	methodType := domain.PaymentMethodType(req.PaymentMethod)
	if !methodType.IsValid() {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_PAYMENT_METHOD", Message: "invalid payment_method_type"}}, http.StatusBadRequest)
		return
	}

	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = r.Header.Get("Idempotency-Key")
	}

	p, err := h.paymentUseCases.CreatePayment(r.Context(), orgID, actorID, req.OrderID,
		domain.Money{AmountMinor: req.AmountMinor, Currency: req.Currency},
		methodType, req.PaymentMethodID, idempotencyKey, req.Metadata)
	if err != nil {
		if isConflictError(err) {
			writeJSON(w, ErrorResponse{Error: Error{Code: "IDEMPOTENCY_CONFLICT", Message: err.Error()}}, http.StatusConflict)
			return
		}
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_CREATE_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"payment_id":          p.ID,
		"status":              string(p.Status),
		"amount":              p.Amount.AmountMinor,
		"currency":            p.Amount.Currency,
		"provider_id":         p.ProviderID,
		"provider_payment_id": p.ProviderPaymentID,
	}, http.StatusCreated)
}

// GetPaymentH handles GET /api/v1/payments/{payment_id}
func (h *PaymentHandler) GetPaymentH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	p, err := h.paymentUseCases.GetPayment(r.Context(), orgID, paymentID)
	if err != nil {
		if err == domain.ErrPaymentNotFound {
			writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_NOT_FOUND", Message: "Payment not found"}}, http.StatusNotFound)
			return
		}
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_GET_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, paymentResponse(p), http.StatusOK)
}

// CancelPaymentH handles POST /api/v1/payments/{payment_id}/cancel
func (h *PaymentHandler) CancelPaymentH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	actorID, ok := h.actorIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	p, err := h.paymentUseCases.CancelPayment(r.Context(), orgID, actorID, paymentID, idempotencyKey)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_CANCEL_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"payment_id":   p.ID,
		"status":       string(p.Status),
		"cancelled_at": p.CancelledAt,
	}, http.StatusOK)
}

// CapturePaymentH handles POST /api/v1/payments/{payment_id}/capture
func (h *PaymentHandler) CapturePaymentH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	actorID, ok := h.actorIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	var req struct {
		AmountMinor int64  `json:"amount_minor"`
		Currency    string `json:"currency"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	var amount *domain.Money
	if req.AmountMinor > 0 {
		amount = &domain.Money{AmountMinor: req.AmountMinor, Currency: req.Currency}
		if amount.Currency == "" {
			amount.Currency = "USD"
		}
	}

	p, err := h.paymentUseCases.CapturePayment(r.Context(), orgID, actorID, paymentID, idempotencyKey, amount)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_CAPTURE_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"payment_id":  p.ID,
		"status":      string(p.Status),
		"captured_at": p.CapturedAt,
	}, http.StatusOK)
}

// RefundPaymentH handles POST /api/v1/payments/{payment_id}/refund
func (h *PaymentHandler) RefundPaymentH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	actorID, ok := h.actorIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	var req struct {
		AmountMinor int64  `json:"amount_minor"`
		Currency    string `json:"currency"`
	}
	if err := readJSONBody(r, &req); err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "INVALID_REQUEST", Message: "Invalid request format"}}, http.StatusBadRequest)
		return
	}

	var amount *domain.Money
	if req.AmountMinor > 0 {
		amount = &domain.Money{AmountMinor: req.AmountMinor, Currency: req.Currency}
		if amount.Currency == "" {
			amount.Currency = "USD"
		}
	}

	p, refund, err := h.paymentUseCases.RefundPayment(r.Context(), orgID, actorID, paymentID, idempotencyKey, amount)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_REFUND_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	result := map[string]interface{}{
		"payment_id": p.ID,
		"status":     string(p.Status),
	}
	if refund != nil {
		result["refund_id"] = refund.ID
		result["refund_status"] = string(refund.Status)
		result["refund_amount"] = refund.Amount.AmountMinor
	}
	writeJSON(w, result, http.StatusCreated)
}

// GetRefundH handles GET /api/v1/payments/{payment_id}/refunds/{refund_id}
func (h *PaymentHandler) GetRefundH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	refundID := r.PathValue("refund_id")
	if refundID == "" {
		refundID = r.URL.Query().Get("refund_id")
	}
	if refundID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "refund_id required"}}, http.StatusBadRequest)
		return
	}

	refunds, err := h.paymentUseCases.ListRefunds(r.Context(), orgID, paymentID, 0, 100)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "REFUND_LIST_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	for _, rf := range refunds {
		if rf.ID == refundID {
			writeJSON(w, refundResponse(rf), http.StatusOK)
			return
		}
	}

	writeJSON(w, ErrorResponse{Error: Error{Code: "REFUND_NOT_FOUND", Message: "Refund not found"}}, http.StatusNotFound)
}

// GetPaymentStatusH handles GET /api/v1/payments/{payment_id}/status
func (h *PaymentHandler) GetPaymentStatusH(w http.ResponseWriter, r *http.Request) {
	orgID, ok := h.orgIDFromQuery(w, r)
	if !ok {
		return
	}
	actorID, ok := h.actorIDFromQuery(w, r)
	if !ok {
		return
	}
	paymentID, ok := h.paymentIDFromPath(w, r)
	if !ok {
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	p, err := h.paymentUseCases.GetPaymentStatus(r.Context(), orgID, actorID, paymentID, idempotencyKey)
	if err != nil {
		if err == domain.ErrPaymentNotFound {
			writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_NOT_FOUND", Message: "Payment not found"}}, http.StatusNotFound)
			return
		}
		writeJSON(w, ErrorResponse{Error: Error{Code: "PAYMENT_STATUS_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		return
	}

	writeJSON(w, paymentResponse(p), http.StatusOK)
}

// WebhookPaymentH handles POST /api/v1/webhooks/payments/{provider_code}
func (h *PaymentHandler) WebhookPaymentH(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("organization_id")
	if orgID == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "organization_id required"}}, http.StatusBadRequest)
		return
	}

	providerCode := r.PathValue("provider_code")
	if providerCode == "" {
		providerCode = r.URL.Query().Get("provider_code")
	}
	if providerCode == "" {
		writeJSON(w, ErrorResponse{Error: Error{Code: "MISSING_PARAMS", Message: "provider_code required"}}, http.StatusBadRequest)
		return
	}

	body, err := readBody(r)
	if err != nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "WEBHOOK_READ_FAILED", Message: "Failed to read webhook body"}}, http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Signature")
	if signature == "" {
		signature = r.Header.Get("X-Payment-Signature")
	}

	if h.webhookHandler == nil {
		writeJSON(w, ErrorResponse{Error: Error{Code: "WEBHOOK_HANDLER_UNAVAILABLE", Message: "Webhook handler not configured"}}, http.StatusServiceUnavailable)
		return
	}

	if err := h.webhookHandler.ProcessWebhook(r.Context(), orgID, providerCode, signature, body); err != nil {
		switch err {
		case domain.ErrWebhookSignatureInvalid:
			writeJSON(w, ErrorResponse{Error: Error{Code: "WEBHOOK_SIGNATURE_INVALID", Message: "Invalid webhook signature"}}, http.StatusUnauthorized)
		case domain.ErrWebhookEventDuplicate:
			writeJSON(w, map[string]interface{}{"status": "duplicate"}, http.StatusOK)
		default:
			writeJSON(w, ErrorResponse{Error: Error{Code: "WEBHOOK_PROCESSING_FAILED", Message: err.Error()}}, http.StatusInternalServerError)
		}
		return
	}

	writeJSON(w, map[string]interface{}{"status": "accepted"}, http.StatusOK)
}

func isConflictError(err error) bool {
	return errors.Is(err, domain.ErrIdempotencyConflict)
}

// PaymentHttpHandlerInterface defines payment API methods for routing.
type PaymentHttpHandlerInterface interface {
	CreatePaymentH(w http.ResponseWriter, r *http.Request)
	GetPaymentH(w http.ResponseWriter, r *http.Request)
	CancelPaymentH(w http.ResponseWriter, r *http.Request)
	CapturePaymentH(w http.ResponseWriter, r *http.Request)
	RefundPaymentH(w http.ResponseWriter, r *http.Request)
	GetRefundH(w http.ResponseWriter, r *http.Request)
	GetPaymentStatusH(w http.ResponseWriter, r *http.Request)
	WebhookPaymentH(w http.ResponseWriter, r *http.Request)
}

func paymentResponse(p *domain.Payment) map[string]interface{} {
	return map[string]interface{}{
		"payment_id":          p.ID,
		"order_id":            p.OrderID,
		"status":              string(p.Status),
		"amount":              p.Amount.AmountMinor,
		"currency":            p.Amount.Currency,
		"payment_method_type": string(p.PaymentMethodType),
		"payment_method_id":   p.PaymentMethodID,
		"provider_id":         p.ProviderID,
		"provider_payment_id": p.ProviderPaymentID,
		"authorized_at":       p.AuthorizedAt,
		"captured_at":         p.CapturedAt,
		"cancelled_at":        p.CancelledAt,
		"failed_at":           p.FailedAt,
		"failure_reason":      p.FailureReason,
		"refunded_amount":     p.RefundedAmount.AmountMinor,
		"created_at":          formatTimestamp(p.CreatedAt),
		"updated_at":          formatTimestamp(p.UpdatedAt),
	}
}

func refundResponse(r *domain.Refund) map[string]interface{} {
	return map[string]interface{}{
		"refund_id":          r.ID,
		"payment_id":         r.PaymentID,
		"status":             string(r.Status),
		"amount":             r.Amount.AmountMinor,
		"currency":           r.Amount.Currency,
		"provider_refund_id": r.ProviderRefundID,
		"processed_at":       r.ProcessedAt,
		"failed_at":          r.FailedAt,
		"failure_reason":     r.FailureReason,
		"created_at":         formatTimestamp(r.CreatedAt),
		"updated_at":         formatTimestamp(r.UpdatedAt),
	}
}

func readBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}

func formatTimestamp(ts int64) string {
	if ts == 0 {
		return ""
	}
	return strconv.FormatInt(ts, 10)
}
