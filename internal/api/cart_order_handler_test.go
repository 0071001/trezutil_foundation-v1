package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

// cartOrders is an OrderUseCases that also accepts carts.
type cartOrders struct {
	mockOrderUseCases
	err   error
	lines []domain.OrderLine
	key   string
}

func (c *cartOrders) CreateOrderWithLines(_ context.Context, _ string, lines []domain.OrderLine, key string) (*domain.Order, error) {
	c.lines, c.key = lines, key
	if c.err != nil {
		return nil, c.err
	}
	return &domain.Order{ID: "order-9", Status: domain.OrderStatusDraft, CustomerID: "u1", Currency: "USD",
		TotalAmount: domain.Money{AmountMinor: 500, Currency: "USD"}, Items: []*domain.OrderItem{{}, {}}}, nil
}

func postOrder(h *OrderHandler, body string, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/orders?user_id=u1", bytes.NewReader([]byte(body)))
	req = asQueryUser(req)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	h.CreateOrderH(w, req)
	return w
}

func TestCreateOrderH_Cart(t *testing.T) {
	uc := &cartOrders{}
	h := NewOrderHandler(uc)
	w := postOrder(h, `{"items":[{"listing_id":"a","quantity":2},{"listing_id":"b"}]}`, "k1")
	if w.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out["order_id"] != "order-9" || out["items"] != float64(2) {
		t.Fatalf("response = %v", out)
	}
	// A missing quantity means one; the key reaches the use case.
	if len(uc.lines) != 2 || uc.lines[1].Quantity != 1 || uc.key != "k1" {
		t.Fatalf("lines = %+v key = %q", uc.lines, uc.key)
	}
}

func TestCreateOrderH_CartErrors(t *testing.T) {
	for err, want := range map[error]int{
		domain.ErrInvalidOrderLines:       http.StatusBadRequest,
		domain.ErrOrderMixedStores:        http.StatusBadRequest,
		domain.ErrOrderNotFound:           http.StatusNotFound,
		domain.ErrInsufficientPermissions: http.StatusForbidden,
		domain.ErrListingUnavailable:      http.StatusConflict,
		domain.ErrIdempotencyConflict:     http.StatusConflict,
	} {
		w := postOrder(NewOrderHandler(&cartOrders{err: err}), `{"items":[{"listing_id":"a","quantity":1}]}`, "")
		if w.Code != want {
			t.Errorf("%v: status %d, want %d", err, w.Code, want)
		}
	}
	// A use case without cart support says so instead of ignoring the items.
	w := postOrder(NewOrderHandler(&mockOrderUseCases{}), `{"items":[{"listing_id":"a","quantity":1}]}`, "")
	if w.Code != http.StatusNotImplemented {
		t.Errorf("no cart support: status %d", w.Code)
	}
	// The legacy single-item body still works.
	w = postOrder(NewOrderHandler(&cartOrders{}), `{"listing_id":"a","quantity":1}`, "")
	if w.Code != http.StatusCreated {
		t.Errorf("legacy body: status %d", w.Code)
	}
}
