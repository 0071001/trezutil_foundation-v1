package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestGetOrderHandler_ActionType(t *testing.T) {
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{})
	if handler.ActionType() != domain.ActionGetOrder {
		t.Errorf("expected %s, got %s", domain.ActionGetOrder, handler.ActionType())
	}
}

func TestGetOrderHandler_Handle_Success(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{
		order:          order,
		actorWhiteList: map[string]bool{"user-1": true},
	})
	params := map[string]interface{}{"order_id": "order-1"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	o, ok := result.(*domain.Order)
	if !ok {
		t.Fatal("expected *domain.Order")
	}
	if o.ID != "order-1" {
		t.Errorf("expected order-1, got %s", o.ID)
	}
}

func TestGetOrderHandler_Handle_NotFound(t *testing.T) {
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{
		err:            domain.ErrOrderNotFound,
		actorWhiteList: map[string]bool{"user-1": true},
	})
	params := map[string]interface{}{"order_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing order_id")
	}
}

func TestGetOrderHandler_Handle_EmptyID(t *testing.T) {
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"order_id": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for empty order_id")
	}
}

func TestGetOrderHandler_Handle_ServiceError(t *testing.T) {
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{
		err:            domain.ErrInsufficientPermissions,
		actorWhiteList: map[string]bool{"user-1": true},
	})
	params := map[string]interface{}{"order_id": "order-1"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetOrderHandler_Handle_TenantIsolation(t *testing.T) {
	order := domain.NewOrder("org-1", "seller-1", "user-1", "store-1", []*domain.OrderItem{}, "USD", 1000)
	order.ID = "order-1"
	handler := NewGetOrderHandler(&fakeOrderUseCasesForHandler{
		order:          order,
		actorWhiteList: map[string]bool{"user-1": true},
	})

	actor1 := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	actor2 := domain.NewCommerceActor("user-2", domain.ActorTypeCustomer)

	_, err1 := handler.Handle(context.Background(), actor1, map[string]interface{}{"order_id": "order-1"})
	_, err2 := handler.Handle(context.Background(), actor2, map[string]interface{}{"order_id": "order-1"})

	if err1 != nil {
		t.Errorf("actor1 unexpected error: %v", err1)
	}
	if err2 == nil {
		t.Error("expected error for actor2 (tenant isolation)")
	}
}
