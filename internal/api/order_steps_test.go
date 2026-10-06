package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// stepOrderUseCases is a use-case double that can carry orders through the seller-side steps. Each step answers
// with the error stored for the order id, or moves the order to the status the step leads to.
type stepOrderUseCases struct {
	mockOrderUseCases
	errs map[string]error
}

func (s *stepOrderUseCases) step(orderID string, to domain.OrderStatus) (*domain.Order, error) {
	if err := s.errs[orderID]; err != nil {
		return nil, err
	}
	return &domain.Order{ID: orderID, Status: to}, nil
}
func (s *stepOrderUseCases) StartProcessing(ctx context.Context, actorID, orderID string) (*domain.Order, error) {
	return s.step(orderID, domain.OrderStatusProcessing)
}
func (s *stepOrderUseCases) ReadyForFulfillment(ctx context.Context, actorID, orderID string) (*domain.Order, error) {
	return s.step(orderID, domain.OrderStatusReadyForFulfillment)
}
func (s *stepOrderUseCases) FulfilOrder(ctx context.Context, actorID, orderID string) (*domain.Order, error) {
	return s.step(orderID, domain.OrderStatusFulfilled)
}
func (s *stepOrderUseCases) CompleteOrder(ctx context.Context, actorID, orderID string) (*domain.Order, error) {
	return s.step(orderID, domain.OrderStatusCompleted)
}
func (s *stepOrderUseCases) ConfirmOrder(ctx context.Context, actorID, orderID string) (*domain.Order, error) {
	return s.step(orderID, domain.OrderStatusConfirmed)
}

func callStep(handler http.HandlerFunc, orderID, actor string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/step?order_id="+orderID+"&user_id="+actor, nil)
	req = asQueryUser(req)
	w := httptest.NewRecorder()
	handler(w, req)
	return w
}

func TestOrderStepH_EachStepReportsTheNewStatus(t *testing.T) {
	h := NewOrderHandler(&stepOrderUseCases{})
	for step, want := range map[string]string{"process": "processing", "ready": "ready_for_fulfillment", "fulfil": "fulfilled", "complete": "completed"} {
		w := callStep(h.OrderStepH(step), "order-1", "seller")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d: %s", step, w.Code, w.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["status"] != want || body["order_id"] != "order-1" {
			t.Fatalf("%s: body %s", step, w.Body.String())
		}
	}
}

func TestOrderStepH_FailuresAreNamedNotServerErrors(t *testing.T) {
	uc := &stepOrderUseCases{errs: map[string]error{
		"gone":      domain.ErrOrderNotFound,
		"not-yours": domain.ErrInsufficientPermissions,
		"wrong":     fmt.Errorf("%w: cannot complete an order that is paid", domain.ErrInvalidOrderTransition),
		"raced":     domain.ErrOptimisticLock,
		"broken":    fmt.Errorf("database is down"),
	}}
	h := NewOrderHandler(uc)
	cases := []struct {
		order string
		code  int
		err   string
	}{
		{"gone", http.StatusNotFound, "ORDER_NOT_FOUND"},
		{"not-yours", http.StatusForbidden, "FORBIDDEN"},
		{"wrong", http.StatusConflict, "INVALID_TRANSITION"},
		{"raced", http.StatusConflict, "ORDER_CHANGED"},
		{"broken", http.StatusInternalServerError, "ORDER_STEP_FAILED"},
	}
	for _, c := range cases {
		w := callStep(h.OrderStepH("complete"), c.order, "seller")
		if w.Code != c.code {
			t.Errorf("%s: status %d, want %d", c.order, w.Code, c.code)
		}
		var body ErrorResponse
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Error.Code != c.err {
			t.Errorf("%s: body %s, want code %s", c.order, w.Body.String(), c.err)
		}
	}
	// the confirmation answers a wrong state the same way (it used to be a server error)
	if w := callStep(h.ConfirmOrderH, "wrong", "seller"); w.Code != http.StatusConflict {
		t.Errorf("confirm in the wrong state: status %d, want 409", w.Code)
	}
}

func TestOrderStepH_NeedsAnOrderAndAUser(t *testing.T) {
	h := NewOrderHandler(&stepOrderUseCases{})
	if w := callStep(h.OrderStepH("ready"), "", "seller"); w.Code != http.StatusBadRequest {
		t.Errorf("no order id: %d", w.Code)
	}
	if w := callStep(h.OrderStepH("ready"), "order-1", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("no user: %d", w.Code)
	}
	if w := callStep(h.OrderStepH("teleport"), "order-1", "seller"); w.Code != http.StatusNotFound {
		t.Errorf("unknown step: %d", w.Code)
	}
}

func TestOrderStepH_AServiceWithoutTheLifecycleSaysSo(t *testing.T) {
	h := NewOrderHandler(&mockOrderUseCases{})
	if w := callStep(h.OrderStepH("ready"), "order-1", "seller"); w.Code != http.StatusNotImplemented {
		t.Errorf("status %d, want 501", w.Code)
	}
}
