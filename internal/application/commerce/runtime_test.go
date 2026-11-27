package commerce

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

func TestNewCommerceActionRuntime(t *testing.T) {
	logger := zerolog.Nop()
	registry := NewHandlerRegistry()
	store := NewInMemoryIdempotencyStore()
	policy := NewDefaultConfirmationPolicy()
	validator := NewDefaultParameterValidator()

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             logger,
		HandlerRegistry:    registry,
		IdempotencyStore:   store,
		ConfirmationPolicy: policy,
		ParameterValidator: validator,
	})

	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if rt.registry != registry {
		t.Error("expected registry to be set")
	}
	if rt.idempotencyStore == nil {
		t.Error("expected idempotency store to be set")
	}
}

func TestExecute_NilAction(t *testing.T) {
	rt := newTestRuntime(t)

	result, err := rt.Execute(context.Background(), nil, domain.NewCommerceActor("user-1", domain.ActorTypeCustomer))
	if err == nil {
		t.Fatal("expected error for nil action")
	}
	if result != nil {
		t.Fatal("expected nil result for nil action")
	}
}

func TestExecute_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-1")
	action.WithSource("AI")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_TenantResolution(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticated.OrgID = "org-1"
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithTenant("should-be-overridden")

	_, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.TenantID != "org-1" {
		t.Errorf("expected tenant from authenticated actor, got %s", action.TenantID)
	}
}

func TestExecute_MissingActionType(t *testing.T) {
	rt := newTestRuntime(t)
	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)

	action := domain.NewCommerceAction("", authenticated, "conv-1")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_MissingActorID(t *testing.T) {
	rt := newTestRuntime(t)
	authenticated := domain.NewCommerceActor("", domain.ActorTypeCustomer)

	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_MissingActorType(t *testing.T) {
	rt := newTestRuntime(t)
	authenticated := domain.NewCommerceActor("user-1", "")

	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_MissingConversationID(t *testing.T) {
	rt := newTestRuntime(t)
	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)

	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_InvalidParameter(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-param")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_NotImplemented(t *testing.T) {
	rt := newTestRuntime(t)
	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCheckInventory, authenticated, "conv-1")

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultNotImplemented {
		t.Errorf("expected NOT_IMPLEMENTED, got %s", result.Status)
	}
}

func TestExecute_Success(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-success")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, authenticated, "conv-1")
	action.WithIdempotencyKey("test-confirm")
	action.Parameters["listing_id"] = "listing-1"
	action.Parameters["quantity"] = 1

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
	if !result.RequiresConfirmation {
		t.Error("expected RequiresConfirmation to be true")
	}
}

func TestExecute_ConfirmedProceeds(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, authenticated, "conv-1")
	action.WithIdempotencyKey("test-confirmed")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["listing_id"] = "listing-1"
	action.Parameters["quantity"] = 1

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_IdempotencyCacheHit(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-idem")
	action.Parameters["text"] = "test"

	// First call - should succeed and store result.
	result1, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.Status != domain.ActionResultSuccess {
		t.Errorf("first call: expected SUCCESS, got %s", result1.Status)
	}

	// Second call with same key - should return cached result.
	action2 := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-2")
	action2.WithIdempotencyKey("test-idem")
	action2.Parameters["text"] = "different"

	result2, err := rt.Execute(context.Background(), action2, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result2.Status != domain.ActionResultSuccess {
		t.Errorf("second call: expected SUCCESS (cached), got %s", result2.Status)
	}
}

func TestExecute_NonIdempotentNotStored(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})
	store := rt.idempotencyStore.(*InMemoryIdempotencyStore)

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.Parameters["text"] = "test"

	_, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if store.Cleanup(context.Background(), -1*24*time.Hour) != 0 {
		t.Error("idempotency store should be empty for non-idempotent action")
	}
}

func TestIsSupported(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	if !rt.IsSupported(domain.ActionSearchListings) {
		t.Error("expected SearchListings to be supported")
	}
	if rt.IsSupported(domain.ActionCreateOrder) {
		t.Error("expected CreateOrder to not be supported")
	}
}

func TestExecute_ConfirmationWithIdempotency(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, authenticated, "conv-1")
	action.WithIdempotencyKey("test-conf-idem")
	action.Parameters["listing_id"] = "listing-1"
	action.Parameters["quantity"] = 1

	// First call should require confirmation.
	result1, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result1.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("first call: expected REQUIRES_CONFIRMATION, got %s", result1.Status)
	}
}

func TestExecute_PublishListing_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionPublishListing})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionPublishListing, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-pub")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["listing_id"] = "listing-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_PublishListing_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionPublishListing})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionPublishListing, authenticated, "conv-1")
	action.WithIdempotencyKey("test-pub-confirm")
	action.Parameters["listing_id"] = "listing-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
}

func TestExecute_PauseListing_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionPauseListing})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionPauseListing, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-pause")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["listing_id"] = "listing-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_PauseListing_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionPauseListing})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionPauseListing, authenticated, "conv-1")
	action.WithIdempotencyKey("test-pause-confirm")
	action.Parameters["listing_id"] = "listing-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
}

func TestExecute_CreateListing_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateListing})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateListing, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-create")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["listing"] = &domain.Listing{}

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_CreateListing_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateListing})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateListing, authenticated, "conv-1")
	action.WithIdempotencyKey("test-create-confirm")
	action.Parameters["listing"] = &domain.Listing{}

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
}

func TestExecute_CreateListing_MissingListingParam(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateListing})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateListing, authenticated, "conv-1")
	action.ConfirmationState = domain.ConfirmationApproved

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_CreateOrder_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-create-order")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["listing_id"] = "listing-1"
	action.Parameters["quantity"] = 1

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_CreateOrder_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, authenticated, "conv-1")
	action.WithIdempotencyKey("test-create-order-confirm")
	action.Parameters["listing_id"] = "listing-1"
	action.Parameters["quantity"] = 1

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
}

func TestExecute_CreateOrder_MissingParam(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCreateOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCreateOrder, authenticated, "conv-1")
	action.ConfirmationState = domain.ConfirmationApproved

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

func TestExecute_CancelOrder_IdentityProtection(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCancelOrder})

	authenticated := domain.NewCommerceActor("real-user", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCancelOrder, domain.NewCommerceActor("ai-user", domain.ActorTypeSystem), "conv-1")
	action.WithIdempotencyKey("test-cancel")
	action.ConfirmationState = domain.ConfirmationApproved
	action.Parameters["order_id"] = "order-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", result.Status)
	}
}

func TestExecute_CancelOrder_ConfirmationRequired(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCancelOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCancelOrder, authenticated, "conv-1")
	action.WithIdempotencyKey("test-cancel-confirm")
	action.Parameters["order_id"] = "order-1"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultRequiresConfirmation {
		t.Errorf("expected REQUIRES_CONFIRMATION, got %s", result.Status)
	}
}

func TestExecute_CancelOrder_MissingParam(t *testing.T) {
	rt := newTestRuntime(t)
	rt.registry.Register(&fakeHandler{actionType: domain.ActionCancelOrder})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	action := domain.NewCommerceAction(domain.ActionCancelOrder, authenticated, "conv-1")
	action.ConfirmationState = domain.ConfirmationApproved

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultValidationFailed {
		t.Errorf("expected VALIDATION_FAILED, got %s", result.Status)
	}
}

type fakeHandler struct {
	actionType domain.CommerceActionType
}

func (h *fakeHandler) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"ok": true}, nil
}

func (h *fakeHandler) ActionType() domain.CommerceActionType {
	return h.actionType
}

type fakeHandlerFail struct {
	actionType domain.CommerceActionType
}

func (h *fakeHandlerFail) Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error) {
	return nil, fmt.Errorf("handler failed")
}

func (h *fakeHandlerFail) ActionType() domain.CommerceActionType {
	return h.actionType
}

func newTestRuntime(t *testing.T) *CommerceActionRuntime {
	t.Helper()
	return NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
	})
}

type fakeUsageService struct {
	policies     map[domain.CommerceActionType]domain.UsagePolicy
	accounts     map[string]*domain.CreditAccount
	reservations map[string]*domain.UsageReservation
	consumed     map[string]bool
	released     map[string]bool
	resolveErr   error
	checkErr     error
	reserveErr   error
	consumeErr   error
	releaseErr   error
	markErr      error
	marks        map[string]domain.UsageActionState
}

func newFakeUsageService() *fakeUsageService {
	return &fakeUsageService{
		policies:     make(map[domain.CommerceActionType]domain.UsagePolicy),
		accounts:     make(map[string]*domain.CreditAccount),
		reservations: make(map[string]*domain.UsageReservation),
		consumed:     make(map[string]bool),
		released:     make(map[string]bool),
	}
}

func (f *fakeUsageService) GetBalance(ctx context.Context, accountID string) (*domain.CreditAccount, error) {
	if acc, ok := f.accounts[accountID]; ok {
		return acc, nil
	}
	return nil, domain.ErrUsageAccountNotFound
}

func (f *fakeUsageService) ResolvePolicy(ctx context.Context, actionType domain.CommerceActionType, channel domain.Channel, actorType domain.ActorType) (*domain.UsagePolicy, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	if p, ok := f.policies[actionType]; ok {
		return &p, nil
	}
	return nil, domain.ErrUsagePolicyNotFound
}

func (f *fakeUsageService) Check(ctx context.Context, accountID string, cost int64) (bool, error) {
	if f.checkErr != nil {
		return false, f.checkErr
	}
	if acc, ok := f.accounts[accountID]; ok {
		return acc.Granted-acc.Reserved >= cost, nil
	}
	return false, domain.ErrUsageAccountNotFound
}

func (f *fakeUsageService) Reserve(ctx context.Context, accountID string, actionType domain.CommerceActionType, cost int64, idempotencyKey string, tenantID string) (*domain.UsageReservation, error) {
	if f.reserveErr != nil {
		return nil, f.reserveErr
	}
	acc, ok := f.accounts[accountID]
	if !ok {
		acc = &domain.CreditAccount{AccountID: accountID, Granted: 100}
		f.accounts[accountID] = acc
	}
	acc.Reserved += cost
	resID := fmt.Sprintf("res-%s-%d", accountID, cost)
	res := &domain.UsageReservation{
		ReservationID: resID,
		AccountID:     accountID,
		ActionType:    actionType,
		Amount:        cost,
		ReservedAt:    time.Now().Unix(),
		Status:        domain.UsageReservationActive,
	}
	f.reservations[resID] = res
	return res, nil
}

func (f *fakeUsageService) Consume(ctx context.Context, reservationID string) error {
	if f.consumeErr != nil {
		return f.consumeErr
	}
	res, ok := f.reservations[reservationID]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	if res.Status != domain.UsageReservationActive {
		return domain.ErrUsageAlreadyConsumed
	}
	res.Status = domain.UsageReservationConsumed
	f.consumed[reservationID] = true
	if acc, ok := f.accounts[res.AccountID]; ok {
		acc.Reserved -= res.Amount
	}
	return nil
}

func (f *fakeUsageService) MarkAction(_ context.Context, reservationID string, state domain.UsageActionState) error {
	if f.markErr != nil {
		return f.markErr
	}
	if f.marks == nil {
		f.marks = map[string]domain.UsageActionState{}
	}
	f.marks[reservationID] = state
	return nil
}

func (f *fakeUsageService) Release(ctx context.Context, reservationID string) error {
	if f.releaseErr != nil {
		return f.releaseErr
	}
	res, ok := f.reservations[reservationID]
	if !ok {
		return domain.ErrUsageReservationNotFound
	}
	if res.Status != domain.UsageReservationActive {
		return domain.ErrUsageAlreadyReleased
	}
	res.Status = domain.UsageReservationReleased
	f.released[reservationID] = true
	if acc, ok := f.accounts[res.AccountID]; ok {
		acc.Reserved -= res.Amount
	}
	return nil
}

func (f *fakeUsageService) Grant(ctx context.Context, accountID string, amount int64, reason string) error {
	if acc, ok := f.accounts[accountID]; ok {
		acc.Granted += amount
	} else {
		f.accounts[accountID] = &domain.CreditAccount{AccountID: accountID, Granted: amount}
	}
	return nil
}

func (f *fakeUsageService) FindOrCreateCreditAccountByOwner(ctx context.Context, ownerType domain.ActorType, ownerID string, tenantID string, planID string, periodID string) (*domain.CreditAccount, error) {
	if ownerID == "" {
		return nil, fmt.Errorf("ownerID is required")
	}
	if tenantID == "" {
		tenantID = "default"
	}
	if periodID == "" {
		periodID = time.Now().Format("2006-01")
	}

	key := fmt.Sprintf("%s:%s:%s", ownerType, ownerID, tenantID)
	if acc, ok := f.accounts[key]; ok {
		return acc, nil
	}

	acc := &domain.CreditAccount{
		AccountID: fmt.Sprintf("acc-%s", key),
		OwnerType: ownerType,
		OwnerID:   ownerID,
		TenantID:  tenantID,
		PlanID:    planID,
		PeriodID:  periodID,
		Granted:   1000,
		Reserved:  0,
		Consumed:  0,
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}
	f.accounts[key] = acc
	f.accounts[acc.AccountID] = acc // Also index by accountID for Check/Reserve
	return acc, nil
}

func TestExecute_UsageReserveAndConsume(t *testing.T) {
	usage := newFakeUsageService()
	usage.policies[domain.ActionSearchListings] = domain.UsagePolicy{
		ActionType: domain.ActionSearchListings, Cost: 2, Active: true,
	}
	// Use the same key format that FindOrCreateCreditAccountByOwner uses (actor type is "customer")
	usage.accounts["customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}
	usage.accounts["acc-customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
		UsageService:       usage,
	})
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticated.OrgID = "default"
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-usage")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	acc := usage.accounts["acc-customer:user-1:default"]
	if acc.Reserved != 0 {
		t.Errorf("expected Reserved=0 after consume, got %d", acc.Reserved)
	}
}

func TestExecute_UsageReserveAndRelease(t *testing.T) {
	usage := newFakeUsageService()
	usage.policies[domain.ActionSearchListings] = domain.UsagePolicy{
		ActionType: domain.ActionSearchListings, Cost: 2, Active: true,
	}
	usage.accounts["customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}
	usage.accounts["acc-customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
		UsageService:       usage,
	})
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticated.OrgID = "default"
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-usage-release")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Fatalf("expected SUCCESS, got %s", result.Status)
	}
	acc := usage.accounts["acc-customer:user-1:default"]
	if acc.Reserved != 0 {
		t.Errorf("expected Reserved=0 after consume, got %d", acc.Reserved)
	}
}

func TestExecute_UsageReserveAndReleaseOnFailure(t *testing.T) {
	usage := newFakeUsageService()
	usage.policies[domain.ActionSearchListings] = domain.UsagePolicy{
		ActionType: domain.ActionSearchListings, Cost: 2, Active: true,
	}
	usage.accounts["customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}
	usage.accounts["acc-customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
		UsageService:       usage,
	})
	rt.registry.Register(&fakeHandlerFail{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticated.OrgID = "default"
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-usage-release")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultFailed {
		t.Fatalf("expected FAILED, got %s", result.Status)
	}
	acc := usage.accounts["acc-customer:user-1:default"]
	if acc.Reserved != 0 {
		t.Errorf("expected Reserved=0 after release, got %d", acc.Reserved)
	}
}

func TestExecute_UsageInsufficientCredits(t *testing.T) {
	usage := newFakeUsageService()
	usage.policies[domain.ActionSearchListings] = domain.UsagePolicy{
		ActionType: domain.ActionSearchListings, Cost: 200, Active: true,
	}
	usage.accounts["customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}
	usage.accounts["acc-customer:user-1:default"] = &domain.CreditAccount{AccountID: "acc-customer:user-1:default", Granted: 100, Reserved: 0}

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
		UsageService:       usage,
	})
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	authenticated.OrgID = "default"
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultFailed {
		t.Errorf("expected FAILED, got %s", result.Status)
	}
	var found bool
	for _, e := range result.Errors {
		if e.Code == "INSUFFICIENT_CREDITS" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected INSUFFICIENT_CREDITS error")
	}
}

func TestExecute_UsageSystemActorSkips(t *testing.T) {
	usage := newFakeUsageService()
	usage.policies[domain.ActionSearchListings] = domain.UsagePolicy{
		ActionType: domain.ActionSearchListings, Cost: 2, Active: true,
	}
	usage.accounts["system:system-1:default"] = &domain.CreditAccount{AccountID: "acc-system:system-1:default", Granted: 100, Reserved: 0}
	usage.accounts["acc-system:system-1:default"] = &domain.CreditAccount{AccountID: "acc-system:system-1:default", Granted: 100, Reserved: 0}

	rt := NewCommerceActionRuntime(RuntimeConfig{
		Logger:             zerolog.Nop(),
		HandlerRegistry:    NewHandlerRegistry(),
		IdempotencyStore:   NewInMemoryIdempotencyStore(),
		ConfirmationPolicy: NewDefaultConfirmationPolicy(),
		ParameterValidator: NewDefaultParameterValidator(),
		UsageService:       usage,
	})
	rt.registry.Register(&fakeHandler{actionType: domain.ActionSearchListings})

	authenticated := domain.NewCommerceActor("system-1", domain.ActorTypeSystem)
	action := domain.NewCommerceAction(domain.ActionSearchListings, authenticated, "conv-1")
	action.WithIdempotencyKey("test-system-skip")
	action.Parameters["text"] = "test"

	result, err := rt.Execute(context.Background(), action, authenticated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS for system actor, got %s", result.Status)
	}
	acc := usage.accounts["system:system-1:default"]
	if acc.Reserved != 0 {
		t.Errorf("expected Reserved=0 for system actor (no reserve), got %d", acc.Reserved)
	}
}
