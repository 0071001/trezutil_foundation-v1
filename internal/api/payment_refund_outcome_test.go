package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/domain"
)

type refundUseCases struct {
	PaymentUseCases
	err error
}

func (u refundUseCases) RefundPayment(context.Context, string, string, string, string, *domain.Money) (*domain.Payment, *domain.Refund, error) {
	return &domain.Payment{ID: "p1", Status: domain.PaymentStatusCaptured},
		&domain.Refund{ID: "r1", Status: domain.RefundStatusPending, Amount: domain.Money{AmountMinor: 100, Currency: "USD"}}, u.err
}

// An unknown refund outcome is never answered as a success, nor as a plain failure the caller would redo.
func TestPaymentHandler_RefundOutcomes(t *testing.T) {
	for _, c := range []struct {
		err  error
		code int
		want string
	}{
		{fmt.Errorf("%w: timeout", payment.ErrRefundOutcomeUnknown), http.StatusAccepted, `"outcome":"unknown"`},
		{fmt.Errorf("%w: not supported", payment.ErrRefundRefused), http.StatusUnprocessableEntity, "REFUND_REFUSED"},
		{domain.ErrIdempotencyConflict, http.StatusConflict, "IDEMPOTENCY_CONFLICT"},
	} {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /api/v1/payments/{payment_id}/refunds", NewPaymentHandler(refundUseCases{err: c.err}, nil).RefundPaymentH)
		req := asQueryUser(httptest.NewRequest(http.MethodPost, "/api/v1/payments/p1/refunds?organization_id=o1&user_id=staff",
			bytes.NewReader([]byte(`{"amount_minor":100,"currency":"USD"}`))))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != c.code || !strings.Contains(rec.Body.String(), c.want) {
			t.Fatalf("%v: %d %s", c.err, rec.Code, rec.Body.String())
		}
	}
}
