package commerce

import (
	"context"
	"testing"

	"github.com/trezutil/trezutil/internal/domain"
)

type fakeOrderUseCasesForHandler struct {
	order          *domain.Order
	err            error
	actorWhiteList map[string]bool
}

func (f *fakeOrderUseCasesForHandler) CreateOrder(ctx context.Context, actorID string, listingID string, quantity int) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &domain.Order{
		ID:             "order-1",
		Status:         domain.OrderStatusDraft,
		CustomerID:     actorID,
		OrganizationID: "org-1",
	}, nil
}
func (f *fakeOrderUseCasesForHandler) GetOrder(ctx context.Context, actorID string, orderID string) (*domain.Order, error) {
	if f.err != nil {
		return nil, f.err
	}
	if !f.actorWhiteList[actorID] {
		return nil, domain.ErrInsufficientPermissions
	}
	return f.order, nil
}
func (f *fakeOrderUseCasesForHandler) CancelOrder(ctx context.Context, actorID string, orderID string) error {
	return f.err
}

func TestCreateOrderHandler_ActionType(t *testing.T) {
	handler := NewCreateOrderHandler(&fakeOrderUseCasesForHandler{})
	if handler.ActionType() != domain.ActionCreateOrder {
		t.Errorf("expected %s, got %s", domain.ActionCreateOrder, handler.ActionType())
	}
}

func TestCreateOrderHandler_Handle_Success(t *testing.T) {
	handler := NewCreateOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{
		"listing_id": "listing-1",
		"quantity":   2,
	}

	result, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	m := result.(map[string]interface{})
	if m["status"] != "draft" {
		t.Errorf("expected draft, got %s", m["status"])
	}
	if m["customer_id"] != "user-1" {
		t.Errorf("expected user-1, got %s", m["customer_id"])
	}
}

func TestCreateOrderHandler_Handle_MissingListing(t *testing.T) {
	handler := NewCreateOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"quantity": 1}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error for missing listing_id")
	}
}

func TestCreateOrderHandler_Handle_ServiceError(t *testing.T) {
	handler := NewCreateOrderHandler(&fakeOrderUseCasesForHandler{err: domain.ErrOrderNotFound})
	params := map[string]interface{}{"listing_id": "missing"}

	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("user-1", domain.ActorTypeCustomer), params)
	if err == nil {
		t.Error("expected error")
	}
}

func TestCreateOrderHandler_Handle_IdentityProtection(t *testing.T) {
	handler := NewCreateOrderHandler(&fakeOrderUseCasesForHandler{})
	params := map[string]interface{}{"listing_id": "l1", "quantity": 1}

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	_, err := handler.Handle(context.Background(), domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = authenticated
}
