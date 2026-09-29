package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/packages/auth"
)

// A forged ?user_id= must never authenticate a request.
func TestHandlers_IgnoreClientSuppliedUserID(t *testing.T) {
	h := NewOrderHandler(&mockOrderUseCases{})
	req := httptest.NewRequest(http.MethodPost, "/v1/orders?user_id=victim", bytes.NewReader([]byte(`{"listing_id":"l1","quantity":1}`)))
	rec := httptest.NewRecorder()
	h.CreateOrderH(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request with forged user_id, got %d", rec.Code)
	}
}

type denyAllPaymentAccess struct{ calledWith string }

func (d *denyAllPaymentAccess) CanCreate(ctx context.Context, actorID, orgID, orderID string) error {
	d.calledWith = actorID
	return domain.ErrInsufficientPermissions
}
func (d *denyAllPaymentAccess) CanView(ctx context.Context, actorID, orgID, paymentID string) error {
	d.calledWith = actorID
	return domain.ErrPaymentNotFound
}
func (d *denyAllPaymentAccess) CanManage(ctx context.Context, actorID, orgID, paymentID string) error {
	d.calledWith = actorID
	return domain.ErrInsufficientPermissions
}

func TestPaymentHandler_CaptureRequiresStaff(t *testing.T) {
	access := &denyAllPaymentAccess{}
	h := NewPaymentHandler(nil, nil).WithAccessPolicy(access)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/payments/{payment_id}/capture", h.CapturePaymentH)

	// Unauthenticated -> 401, even with a forged user_id.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/p1/capture?organization_id=o1&user_id=admin", bytes.NewReader([]byte(`{}`)))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	// Authenticated customer -> policy consulted with the verified identity -> 403.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/payments/p1/capture?organization_id=o1&user_id=admin", bytes.NewReader([]byte(`{}`)))
	req = req.WithContext(auth.WithPrincipal(req.Context(), auth.Principal{UserID: "customer-1"}))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if access.calledWith != "customer-1" {
		t.Fatalf("policy must see the verified actor, got %q", access.calledWith)
	}
}
