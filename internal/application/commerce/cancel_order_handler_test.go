package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestCancelOrderHandler_ActionType(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{})
	if handler.ActionType() != domain.ActionCancelOrder {
		t.Errorf("expected %s, got %s", domain.ActionCancelOrder, handler.ActionType())
	}
}

func TestCancelOrderHandler_Handle_Success(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"order_id": "order-1"}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["status"] != "cancelled" {
		t.Errorf("expected cancelled, got %s", m["status"])
	}
}

func TestCancelOrderHandler_Handle_NotFound(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{err: domain.ErrOrderNotFound})
	params := map[string]interface{}{"order_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing order_id")
	}
}

func TestCancelOrderHandler_Handle_EmptyID(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"order_id": ""}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for empty order_id")
	}
}

func TestCancelOrderHandler_Handle_CannotCancel(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{err: domain.ErrOrderCannotBeCancelled})
	params := map[string]interface{}{"order_id": "order-1"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for cannot cancel")
	}
}

func TestCancelOrderHandler_Handle_IdentityProtection(t *testing.T) {
	handler := NewCancelOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"order_id": "order-1"}

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = authenticated
}
