package payment

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
)

type mockPaymentProviderAdapter struct {
	providerID   string
	provider     *domain.IntegrationProvider
	capabilities map[domain.PaymentProviderOperation]bool
	responses    map[domain.PaymentProviderOperation]domain.PaymentProviderResponse
	errors       map[domain.PaymentProviderOperation]error
	mu           sync.Mutex
}

// setResponse is safe to call from concurrent test goroutines.
func (m *mockPaymentProviderAdapter) setResponse(op domain.PaymentProviderOperation, resp domain.PaymentProviderResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[op] = resp
}

func (m *mockPaymentProviderAdapter) result(op domain.PaymentProviderOperation) (domain.PaymentProviderResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.responses[op], m.errors[op]
}

func newMockPaymentAdapter(id string) *mockPaymentProviderAdapter {
	return &mockPaymentProviderAdapter{
		providerID:   id,
		provider:     &domain.IntegrationProvider{ID: id, OrganizationID: "org-test", Code: "mock-provider", Status: domain.ProviderStatusActive, Capabilities: []domain.Capability{domain.CapabilityPayment}},
		capabilities: make(map[domain.PaymentProviderOperation]bool),
		responses:    make(map[domain.PaymentProviderOperation]domain.PaymentProviderResponse),
		errors:       make(map[domain.PaymentProviderOperation]error),
	}
}

func (m *mockPaymentProviderAdapter) ProviderID() string { return m.providerID }
func (m *mockPaymentProviderAdapter) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapabilityPayment}
}
func (m *mockPaymentProviderAdapter) Health(ctx context.Context) domain.ProviderHealthStatus {
	return domain.ProviderHealthHealthy
}

func (m *mockPaymentProviderAdapter) CreatePayment(ctx context.Context, req domain.PaymentProviderRequest) (domain.PaymentProviderResponse, error) {
	return m.result(domain.PaymentProviderOpCreate)
}
func (m *mockPaymentProviderAdapter) GetPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	return m.result(domain.PaymentProviderOpGet)
}
func (m *mockPaymentProviderAdapter) CancelPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	return m.result(domain.PaymentProviderOpCancel)
}
func (m *mockPaymentProviderAdapter) CapturePayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.PaymentProviderResponse, error) {
	return m.result(domain.PaymentProviderOpCapture)
}
func (m *mockPaymentProviderAdapter) RefundPayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.RefundProviderResponse, error) {
	resp, err := m.result(domain.PaymentProviderOpRefund)
	return domain.RefundProviderResponse{
		ProviderRefundID: resp.ProviderPaymentID,
		Status:           resp.Status,
		Amount:           *amount,
	}, err
}

type mockPaymentRepo struct {
	mu       sync.RWMutex
	payments map[string]*domain.Payment
}

func newMockPaymentRepo() *mockPaymentRepo {
	return &mockPaymentRepo{payments: make(map[string]*domain.Payment)}
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *domain.Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.payments {
		if existing.IdempotencyKey == payment.IdempotencyKey && existing.TenantID == payment.TenantID {
			return domain.ErrPaymentAlreadyExists
		}
	}
	if payment.ID == "" {
		payment.ID = uuid()
	}
	cp := *payment
	m.payments[payment.ID] = &cp
	return nil
}
func (m *mockPaymentRepo) FindByID(ctx context.Context, tenantID, paymentID string) (*domain.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.payments[paymentID]
	if !ok {
		return nil, domain.ErrPaymentNotFound
	}
	if p.TenantID != tenantID {
		return nil, domain.ErrPaymentNotFound
	}
	// Return a copy, like a database read, so concurrent callers do not share state.
	cp := *p
	return &cp, nil
}
func (m *mockPaymentRepo) FindByIDForUpdate(ctx context.Context, tenantID, paymentID string) (*domain.Payment, error) {
	return m.FindByID(ctx, tenantID, paymentID)
}
func (m *mockPaymentRepo) FindByOrderID(ctx context.Context, tenantID, orderID string) (*domain.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.payments {
		if p.OrderID == orderID && p.TenantID == tenantID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}
func (m *mockPaymentRepo) FindByProviderPaymentID(ctx context.Context, tenantID, providerID, providerPaymentID string) (*domain.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.payments {
		if p.ProviderPaymentID == providerPaymentID && p.TenantID == tenantID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}
func (m *mockPaymentRepo) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.Payment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.payments {
		if p.IdempotencyKey == idempotencyKey && p.TenantID == tenantID {
			cp := *p
			return &cp, nil
		}
	}
	return nil, domain.ErrPaymentNotFound
}
func (m *mockPaymentRepo) Update(ctx context.Context, payment *domain.Payment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Mirror the PostgreSQL optimistic lock: WHERE version = payment.Version-1.
	if stored, ok := m.payments[payment.ID]; ok && stored.Version != payment.Version-1 {
		return domain.ErrOptimisticLock
	}
	cp := *payment
	m.payments[payment.ID] = &cp
	return nil
}

type mockRefundRepo struct {
	mu        sync.Mutex
	refunds   map[string]*domain.Refund
	byPayment map[string][]*domain.Refund
}

func newMockRefundRepo() *mockRefundRepo {
	return &mockRefundRepo{
		refunds:   make(map[string]*domain.Refund),
		byPayment: make(map[string][]*domain.Refund),
	}
}

func (m *mockRefundRepo) Create(ctx context.Context, refund *domain.Refund) error {
	if refund.ID == "" {
		refund.ID = uuid()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refunds[refund.ID] = refund
	m.byPayment[refund.PaymentID] = append(m.byPayment[refund.PaymentID], refund)
	return nil
}
func (m *mockRefundRepo) FindByID(ctx context.Context, tenantID, refundID string) (*domain.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.refunds[refundID]
	if !ok {
		return nil, domain.ErrRefundNotFound
	}
	if r.TenantID != tenantID {
		return nil, domain.ErrRefundNotFound
	}
	return r, nil
}
func (m *mockRefundRepo) FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*domain.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byPayment[paymentID], nil
}
func (m *mockRefundRepo) FindByProviderRefundID(ctx context.Context, tenantID, providerID, providerRefundID string) (*domain.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.refunds {
		if r.ProviderRefundID == providerRefundID {
			return r, nil
		}
	}
	return nil, domain.ErrRefundNotFound
}
func (m *mockRefundRepo) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.Refund, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.refunds {
		if r.IdempotencyKey == idempotencyKey {
			return r, nil
		}
	}
	return nil, domain.ErrRefundNotFound
}
func (m *mockRefundRepo) Update(ctx context.Context, refund *domain.Refund) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refunds[refund.ID] = refund
	return nil
}

type mockLedgerRepo struct {
	mu      sync.Mutex
	entries []*domain.LedgerEntry
}

func (m *mockLedgerRepo) Post(ctx context.Context, entry *domain.LedgerEntry) (*domain.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.IdempotencyKey == entry.IdempotencyKey {
			return e, nil
		}
	}
	m.entries = append(m.entries, entry)
	return entry, nil
}
func (m *mockLedgerRepo) FindByIdempotencyKey(ctx context.Context, tenantID, idempotencyKey string) (*domain.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.entries {
		if e.IdempotencyKey == idempotencyKey && e.TenantID == tenantID {
			return e, nil
		}
	}
	return nil, domain.ErrLedgerAlreadyPosted
}
func (m *mockLedgerRepo) FindByPaymentID(ctx context.Context, tenantID, paymentID string, offset, limit int) ([]*domain.LedgerEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries, nil
}

type mockSignatureVerifier struct {
	verified bool
	err      error
}

func (m *mockSignatureVerifier) Verify(providerCode string, payload []byte, signature string) error {
	return m.err
}

type mockProviderRepo struct {
	providers map[string]*domain.IntegrationProvider
}

func newMockProviderRepo() *mockProviderRepo {
	return &mockProviderRepo{providers: make(map[string]*domain.IntegrationProvider)}
}

func (m *mockProviderRepo) Create(ctx context.Context, provider *domain.IntegrationProvider) error {
	m.providers[provider.Code] = provider
	return nil
}
func (m *mockProviderRepo) FindByID(ctx context.Context, orgID, id string) (*domain.IntegrationProvider, error) {
	for _, p := range m.providers {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, domain.ErrProviderNotFound
}
func (m *mockProviderRepo) FindByCode(ctx context.Context, orgID, code string) (*domain.IntegrationProvider, error) {
	p, ok := m.providers[code]
	if !ok {
		return nil, domain.ErrProviderNotFound
	}
	if p.OrganizationID != orgID {
		return nil, domain.ErrProviderNotFound
	}
	return p, nil
}
func (m *mockProviderRepo) FindByOrganizationID(ctx context.Context, orgID string, offset, limit int) ([]*domain.IntegrationProvider, error) {
	var result []*domain.IntegrationProvider
	for _, p := range m.providers {
		result = append(result, p)
	}
	return result, nil
}
func (m *mockProviderRepo) FindByCapability(ctx context.Context, orgID string, capability domain.Capability) ([]*domain.IntegrationProvider, error) {
	var result []*domain.IntegrationProvider
	for _, p := range m.providers {
		for _, c := range p.Capabilities {
			if c == capability {
				result = append(result, p)
			}
		}
	}
	return result, nil
}
func (m *mockProviderRepo) Update(ctx context.Context, provider *domain.IntegrationProvider) error {
	m.providers[provider.Code] = provider
	return nil
}
func (m *mockProviderRepo) Delete(ctx context.Context, orgID, id string) error {
	return nil
}

type mockOrderRepo struct {
	orders map[string]*domain.Order
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{orders: make(map[string]*domain.Order)}
}

func (m *mockOrderRepo) Create(ctx context.Context, order *domain.Order) error {
	m.orders[order.ID] = order
	return nil
}
func (m *mockOrderRepo) FindByID(ctx context.Context, orderID string) (*domain.Order, error) {
	o, ok := m.orders[orderID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}
func (m *mockOrderRepo) FindByIDForUpdate(ctx context.Context, orderID string) (*domain.Order, error) {
	return m.FindByID(ctx, orderID)
}
func (m *mockOrderRepo) FindByIDAndOrg(ctx context.Context, orgID, orderID string) (*domain.Order, error) {
	o, ok := m.orders[orderID]
	if !ok {
		return nil, domain.ErrOrderNotFound
	}
	if o.OrganizationID != orgID {
		return nil, domain.ErrOrderNotFound
	}
	return o, nil
}
func (m *mockOrderRepo) FindByCustomerIdempotencyKey(ctx context.Context, customerID, key string) (*domain.Order, error) {
	return nil, domain.ErrOrderNotFound
}

func (m *mockOrderRepo) Update(ctx context.Context, order *domain.Order) error {
	m.orders[order.ID] = order
	return nil
}

func setupTestService(t *testing.T) (*PaymentService, *mockPaymentRepo, *mockRefundRepo, *mockLedgerRepo, *mockPaymentProviderAdapter, *mockOrderRepo) {
	t.Helper()

	mockRepo := newMockPaymentRepo()
	mockRefundRepo := newMockRefundRepo()
	mockLedgerRepo := &mockLedgerRepo{}
	mockOrders := &mockOrderRepo{orders: make(map[string]*domain.Order)}
	mockOrders.orders["order-1"] = &domain.Order{
		ID:             "order-1",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	adapter := newMockPaymentAdapter("provider-test")
	adapter.capabilities[domain.PaymentProviderOpCreate] = true
	adapter.capabilities[domain.PaymentProviderOpCancel] = true
	adapter.capabilities[domain.PaymentProviderOpCapture] = true
	adapter.capabilities[domain.PaymentProviderOpRefund] = true
	adapter.capabilities[domain.PaymentProviderOpGet] = true

	providerEntry := &domain.IntegrationProvider{
		ID:             "provider-test",
		OrganizationID: "org-test",
		Code:           "mock-provider",
		Name:           "Mock Provider",
		Status:         domain.ProviderStatusActive,
		Capabilities:   []domain.Capability{domain.CapabilityPayment},
	}

	registry := provider.NewProviderRegistry()
	if err := registry.Register(providerEntry, adapter); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	execRepo := &mockProviderExecRepo{}

	orch := provider.NewProviderOrchestrator(registry, execRepo, provider.OrchestratorConfig{
		Logger: zerolog.Nop(),
		RetryPolicy: domain.RetryPolicy{
			MaxAttempts:   1,
			BaseDelayMs:   0,
			MaxDelayMs:    0,
			ClassifyError: func(err error) domain.RetryClassification { return domain.NonRetryable },
		},
	})

	svc := NewPaymentService(PaymentServiceConfig{
		Locker:       NewLocalLocker(),
		Payments:     mockRepo,
		Refunds:      mockRefundRepo,
		Ledger:       mockLedgerRepo,
		Orders:       mockOrders,
		Orchestrator: orch,
		Logger:       zerolog.Nop(),
	})

	return svc, mockRepo, mockRefundRepo, mockLedgerRepo, adapter, mockOrders
}

type mockProviderExecRepo struct{}

func (m *mockProviderExecRepo) Create(ctx context.Context, exec *domain.ProviderExecution) error {
	return nil
}
func (m *mockProviderExecRepo) FindByID(ctx context.Context, orgID, id string) (*domain.ProviderExecution, error) {
	return nil, errors.New("not found")
}
func (m *mockProviderExecRepo) FindByIdempotencyKey(ctx context.Context, orgID, key string) (*domain.ProviderExecution, error) {
	return nil, errors.New("not found")
}
func (m *mockProviderExecRepo) FindByProviderID(ctx context.Context, orgID, providerID string, offset, limit int) ([]*domain.ProviderExecution, error) {
	return nil, nil
}

func TestPaymentService_CreatePayment(t *testing.T) {
	svc, repo, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-123",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-1", nil)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}
	if p.ProviderPaymentID != "ext-pay-123" {
		t.Errorf("expected provider_payment_id ext-pay-123, got %s", p.ProviderPaymentID)
	}
	if repo.payments[p.ID] == nil {
		t.Error("expected payment to be persisted")
	}
}

func TestPaymentService_CreatePayment_Failure(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status: "failed",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-1", nil)

	if err != nil {
		t.Fatalf("CreatePayment should not return error on provider failure: %v", err)
	}
	if p.Status != domain.PaymentStatusFailed {
		t.Errorf("expected FAILED, got %s", p.Status)
	}
}

func TestPaymentService_CreatePayment_AwaitCapture(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-pay-456",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-1", nil)

	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusAuthorized {
		t.Errorf("expected AUTHORIZED, got %s", p.Status)
	}
}

func TestPaymentService_RetryCreatePayment(t *testing.T) {
	svc, repo, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-retry",
	}
	p1, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-retry", nil)
	if err != nil {
		t.Fatalf("first CreatePayment failed: %v", err)
	}

	// Same idempotency key should return existing payment (not create new)
	p2, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-retry", nil)
	if err != nil {
		t.Fatalf("second CreatePayment failed: %v", err)
	}
	if p1.ID != p2.ID {
		t.Errorf("expected same payment ID on retry, got %s and %s", p1.ID, p2.ID)
	}
	if len(repo.payments) != 1 {
		t.Errorf("expected 1 payment in repo, got %d", len(repo.payments))
	}
}

func TestPaymentService_CancelPayment(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-pay-789",
	}
	adapter.responses[domain.PaymentProviderOpCancel] = domain.PaymentProviderResponse{
		Status: "cancelled",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = svc.CancelPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-cancel-req")
	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", p.Status)
	}
}

func TestPaymentService_CancelPayment_NotAllowed(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-789",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-err", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.CancelPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-cancel-req")
	if err == nil {
		t.Fatal("expected error cancelling captured payment")
	}
}

func TestPaymentService_RefundPayment_Full(t *testing.T) {
	svc, _, refundRepo, ledgerRepo, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-refund",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-1",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, refund, err := svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-req", nil)
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}
	if refund.Status != domain.RefundStatusSucceeded {
		t.Errorf("expected refund SUCCEEDED, got %s", refund.Status)
	}
	if refund.ProviderRefundID != "ext-refund-1" {
		t.Errorf("expected provider_refund_id ext-refund-1, got %s", refund.ProviderRefundID)
	}
	if p.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected REFUNDED, got %s", p.Status)
	}
	if p.RefundedAmount.AmountMinor != 1000 {
		t.Errorf("expected refunded amount 1000, got %d", p.RefundedAmount.AmountMinor)
	}
	if len(ledgerRepo.entries) == 0 {
		t.Error("expected ledger entry to be posted")
	}
	if len(refundRepo.refunds) == 0 {
		t.Error("expected refund to be persisted")
	}
}

func TestPaymentService_RefundPayment_ExceedsAmount(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-exceed",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-exceed", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-exceed-req",
		&domain.Money{AmountMinor: 2000, Currency: "USD"})
	if err == nil {
		t.Fatal("expected error refunding more than captured amount")
	}
}

func TestPaymentService_GetPayment_NotFound(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	_, err := svc.GetPayment(context.Background(), "org-test", "nonexistent")
	if err != domain.ErrPaymentNotFound {
		t.Errorf("expected ErrPaymentNotFound, got %v", err)
	}
}

func TestParseProviderStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected domain.PaymentStatus
		hasError bool
	}{
		{"captured", domain.PaymentStatusCaptured, false},
		{"succeeded", domain.PaymentStatusCaptured, false},
		{"authorized", domain.PaymentStatusAuthorized, false},
		{"failed", domain.PaymentStatusFailed, false},
		{"cancelled", domain.PaymentStatusCancelled, false},
		{"expired", domain.PaymentStatusExpired, false},
		{"refunded", domain.PaymentStatusRefunded, false},
		{"partially_refunded", domain.PaymentStatusPartiallyRefunded, false},
		{"", domain.PaymentStatusCreated, false},
		{"unknown", "", true},
	}

	for _, tt := range tests {
		result, err := parseProviderStatus(tt.input)
		if tt.hasError {
			if err == nil {
				t.Errorf("expected error for %q, got nil", tt.input)
			}
			continue
		}
		if err != nil {
			t.Errorf("unexpected error for %q: %v", tt.input, err)
			continue
		}
		if result != tt.expected {
			t.Errorf("for %q: expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestPaymentService_RefundPayment_DuplicateIdempotency(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-pay-dup",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-dup",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-dup", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-dup", nil)
	if err != nil {
		t.Fatalf("first RefundPayment failed: %v", err)
	}

	// Second refund with same idempotency key should return ErrRefundAlreadyExists
	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-dup", nil)
	if err != domain.ErrRefundAlreadyExists {
		t.Errorf("expected ErrRefundAlreadyExists, got %v", err)
	}
}

func TestPaymentService_AuthorizePayment(t *testing.T) {
	svc, repo, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "",
		ProviderPaymentID: "ext-pay-001-noauth",
	}

	// Create a payment that stays in CREATED (provider returns empty/created status)
	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-authorize", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if p.Status != domain.PaymentStatusCreated {
		t.Fatalf("expected CREATED after create, got %s", p.Status)
	}

	// The provider already holds this payment: authorize must sync its state
	// (GetPayment) rather than create a second provider payment.
	adapter.setResponse(domain.PaymentProviderOpCreate, domain.PaymentProviderResponse{Status: "failed", ProviderPaymentID: "must-not-be-created"})
	adapter.setResponse(domain.PaymentProviderOpGet, domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-pay-001-noauth",
	})

	p2, err := svc.AuthorizePayment(context.Background(), "org-test", "actor-1", p.ID, "idem-authorize-2")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	if p2.Status != domain.PaymentStatusAuthorized {
		t.Errorf("expected AUTHORIZED after authorize, got %s", p2.Status)
	}
	if p2.ProviderPaymentID != "ext-pay-001-noauth" {
		t.Errorf("authorize must keep the original provider payment, got %s", p2.ProviderPaymentID)
	}

	if _, err := repo.FindByID(context.Background(), "org-test", p.ID); err != nil {
		t.Error("expected payment to persist after authorize")
	}
}

func TestPaymentService_AuthorizePayment_FromCreated(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-auth-created", 1000)
	p.Status = domain.PaymentStatusCreated
	if err := svc.payments.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-auth-456",
	}

	p2, err := svc.AuthorizePayment(context.Background(), "org-test", "actor-1", p.ID, "idem-auth-created-2")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	if p2.Status != domain.PaymentStatusAuthorized {
		t.Errorf("expected AUTHORIZED, got %s", p2.Status)
	}
}

func TestPaymentService_AuthorizePayment_InvalidState(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-cap-1",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-auth-bad", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.AuthorizePayment(context.Background(), "org-test", "actor-1", p.ID, "idem-auth-bad-2")
	if err == nil {
		t.Fatal("expected error authorizing already-captured payment")
	}
}

func TestPaymentService_AuthorizePayment_TenantIsolation(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-auth-tenant", 1000)
	p.Status = domain.PaymentStatusCreated
	if err := svc.payments.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}

	_, err := svc.AuthorizePayment(context.Background(), "org-OTHER", "actor-1", p.ID, "idem-auth-tenant-2")
	if err == nil {
		t.Fatal("expected error when authorizing payment from different tenant")
	}
}

func TestPaymentService_AmountMismatch_Rejected(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 100, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-amount-low", nil)
	if err == nil {
		t.Fatal("expected error for payment amount below order total")
	}
}

func TestPaymentService_AmountExceedsOrder_Rejected(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 10000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-amount-high", nil)
	if err == nil {
		t.Fatal("expected error for payment amount above order total")
	}
}

func TestPaymentService_CurrencyMismatch_Rejected(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "EUR"},
		domain.PaymentMethodCard, "method-123", "idem-curr-mismatch", nil)
	if err == nil {
		t.Fatal("expected error for currency mismatch")
	}
}

func TestPaymentService_OrderNotFound_Rejected(t *testing.T) {
	svc, _, _, _, _, _ := setupTestService(t)

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "nonexistent-order",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-order-not-found", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent order")
	}
}

func TestPaymentService_OrderTenantMismatch_Rejected(t *testing.T) {
	svc, _, _, _, _, orders := setupTestService(t)

	orders.orders["order-other"] = &domain.Order{
		ID:             "order-other",
		OrganizationID: "org-OTHER",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-other",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-tenant-mismatch", nil)
	if err == nil {
		t.Fatal("expected error for order belonging to different tenant")
	}
}

func TestPaymentService_IdempotencyKeyMismatch_Rejected(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-idem-1",
	}

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-key-mismatch", nil)
	if err != nil {
		t.Fatalf("first CreatePayment failed: %v", err)
	}

	// Same idempotency key but different amount ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€šÃ‚Â ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¾Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â¦ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â¦ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â¦ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¾ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ should conflict
	_, err = svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-key-mismatch", nil)
	if err == nil {
		t.Fatal("expected error for idempotency key with different amount")
	}
}

func TestPaymentService_IdempotencyKeyMismatch_Currency(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-idem-cur",
	}

	_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cur-mismatch", nil)
	if err != nil {
		t.Fatalf("first CreatePayment failed: %v", err)
	}

	// Use a different order that matches the new amount but with same idempotency key
	_, err = svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "EUR"},
		domain.PaymentMethodCard, "method-123", "idem-cur-mismatch", nil)
	if err == nil {
		t.Fatal("expected error for idempotency key with different currency")
	}
}

func TestPaymentService_MultiplePartialRefunds(t *testing.T) {
	svc, repo, refundRepo, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-multi-refund",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-multi",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-multi-refund", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// First partial refund: 400
	p, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-rf-1",
		&domain.Money{AmountMinor: 400, Currency: "USD"})
	if err != nil {
		t.Fatalf("first refund failed: %v", err)
	}
	if p.RefundedAmount.AmountMinor != 400 {
		t.Errorf("expected refunded 400, got %d", p.RefundedAmount.AmountMinor)
	}

	// Second partial refund: 300 (total 700, remaining 300)
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-multi-2",
	}
	p, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-rf-2",
		&domain.Money{AmountMinor: 300, Currency: "USD"})
	if err != nil {
		t.Fatalf("second refund failed: %v", err)
	}
	if p.RefundedAmount.AmountMinor != 700 {
		t.Errorf("expected refunded 700, got %d", p.RefundedAmount.AmountMinor)
	}

	// Third partial refund: 300 (total 1000 = full)
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-multi-3",
	}
	p, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-rf-3",
		&domain.Money{AmountMinor: 300, Currency: "USD"})
	if err != nil {
		t.Fatalf("third refund failed: %v", err)
	}
	if p.Status != domain.PaymentStatusRefunded {
		t.Errorf("expected REFUNDED after full refund, got %s", p.Status)
	}
	if p.RefundedAmount.AmountMinor != 1000 {
		t.Errorf("expected refunded 1000, got %d", p.RefundedAmount.AmountMinor)
	}

	if len(refundRepo.refunds) != 3 {
		t.Errorf("expected 3 refunds, got %d", len(refundRepo.refunds))
	}
	_ = repo
}

func TestPaymentService_RefundAfterFailure_Rejected(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status: "failed",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-fail", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-fail-2", nil)
	if err == nil {
		t.Fatal("expected error refunding a FAILED payment")
	}
}

func TestPaymentService_RefundAfterCancelled_Rejected(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-cancel-refund",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel-refund", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	adapter.responses[domain.PaymentProviderOpCancel] = domain.PaymentProviderResponse{
		Status: "cancelled",
	}
	p, err = svc.CancelPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-cancel-refund-2")
	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-cancel-refund-3", nil)
	if err == nil {
		t.Fatal("expected error refunding a CANCELLED payment")
	}
}

func TestPaymentService_ConcurrentCreatePayment(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-concurrent",
	}

	const concurrency = 20
	var wg sync.WaitGroup
	errs := make([]error, concurrency)
	payments := make([]*domain.Payment, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
				domain.Money{AmountMinor: 1000, Currency: "USD"},
				domain.PaymentMethodCard, "method-123", "idem-concurrent-test", nil)
			payments[idx] = p
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	var firstID string
	for i, p := range payments {
		if p == nil {
			t.Fatalf("goroutine %d returned nil payment", i)
		}
		if errs[i] != nil {
			t.Fatalf("goroutine %d returned error: %v", i, errs[i])
		}
		if firstID == "" {
			firstID = p.ID
		}
		if p.ID != firstID {
			t.Fatalf("goroutine %d got different payment ID %s (expected %s)", i, p.ID, firstID)
		}
	}
}

func TestPaymentService_TenantIsolation_FindByID(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-tenant-isolation",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-tenant-iso", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	// Tenant B cannot read Tenant A's payment
	_, err = svc.GetPayment(context.Background(), "org-OTHER", p.ID)
	if err == nil {
		t.Fatal("expected error when accessing payment from different tenant")
	}
}

func TestPaymentService_TenantIsolation_CancelPayment(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-cancel-tenant",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.CancelPayment(context.Background(), "org-OTHER", "actor-1", p.ID, "idem-cancel-tenant-2")
	if err == nil {
		t.Fatal("expected error when cancelling payment from different tenant")
	}
}

func TestPaymentService_TenantIsolation_RefundPayment(t *testing.T) {
	svc, _, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-refund-tenant",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-tenant-2",
	}
	_, _, err = svc.RefundPayment(context.Background(), "org-OTHER", "actor-1", p.ID, "idem-refund-tenant-2", nil)
	if err == nil {
		t.Fatal("expected error when refunding payment from different tenant")
	}
}

func TestPaymentService_LedgerIdempotency_Capture(t *testing.T) {
	svc, repo, _, ledgerRepo, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-ledger-idem",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-ledger", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	initialLedgerCount := len(ledgerRepo.entries)

	// Manually trigger a second ledger posting for the same payment
	// (already posted during CreatePayment with captured status)
	_ = svc.postLedgerForCapture(context.Background(), p)

	// Should be idempotent ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€šÃ‚Â ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¾Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€šÃ‚Â¦ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬ÃƒÂ¢Ã¢â‚¬Å¾Ã‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã‚Â¦ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™Ãƒâ€šÃ‚Â¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡Ãƒâ€šÃ‚Â¬ÃƒÆ’Ã¢â‚¬Â¦Ãƒâ€šÃ‚Â¡ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â no new entries
	if len(ledgerRepo.entries) != initialLedgerCount {
		t.Errorf("expected idempotent ledger post (no new entries), got %d new entries", len(ledgerRepo.entries)-initialLedgerCount)
	}

	// Verify payment exists
	if _, err := repo.FindByID(context.Background(), "org-test", p.ID); err != nil {
		t.Error("expected payment to exist")
	}
}

func TestPaymentService_RefundIdempotency_LedgerNotDuplicated(t *testing.T) {
	svc, _, refundRepo, ledgerRepo, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-refund-idem",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-idem-1",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-ledger", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-ledger-1", nil)
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	initialLedgerCount := len(ledgerRepo.entries)

	// Second refund with same key should be rejected
	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-ledger-1", nil)
	if err != domain.ErrRefundAlreadyExists {
		t.Errorf("expected ErrRefundAlreadyExists, got %v", err)
	}

	if len(ledgerRepo.entries) != initialLedgerCount {
		t.Errorf("ledger entries should not change on duplicate refund, got %d", len(ledgerRepo.entries)-initialLedgerCount)
	}
	if len(refundRepo.refunds) != 1 {
		t.Errorf("expected 1 refund, got %d", len(refundRepo.refunds))
	}
}

func TestPaymentService_ConcurrentRefunds(t *testing.T) {
	svc, repo, _, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-concurrent-refund",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-concurrent-refund", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-concurrent-refund-1",
	}

	// First refund of 500
	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-cr-1",
		&domain.Money{AmountMinor: 500, Currency: "USD"})
	if err != nil {
		t.Fatalf("first refund failed: %v", err)
	}

	// Attempt 6 concurrent refunds of 200 each (total 1200 > remaining 500)
	const concurrency = 6
	var wg sync.WaitGroup
	results := make([]error, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{
				Status:            "succeeded",
				ProviderPaymentID: fmt.Sprintf("ext-concurrent-refund-%d", idx+2),
			})
			_, _, results[idx] = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID,
				fmt.Sprintf("idem-cr-%d", idx+2),
				&domain.Money{AmountMinor: 200, Currency: "USD"})
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, err := range results {
		if err == nil {
			successCount++
		}
	}

	if successCount > 2 {
		t.Errorf("expected at most 2 additional refunds (remaining 500, each 200), got %d", successCount)
	}

	updated, _ := repo.FindByID(context.Background(), "org-test", p.ID)
	if updated.RefundedAmount.AmountMinor > 1000 {
		t.Errorf("refunded amount %d must not exceed captured amount 1000", updated.RefundedAmount.AmountMinor)
	}
}

func TestRefund_RefundIdempotencyDuplicate(t *testing.T) {
	svc, _, refundRepo, _, adapter, _ := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-refund-idem-test",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-1",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-test", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-test-1", nil)
	if err != nil {
		t.Fatalf("first RefundPayment failed: %v", err)
	}

	countAfterFirst := len(refundRepo.refunds)

	// Duplicate idempotency key should fail
	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID, "idem-refund-test-1", nil)
	if err != domain.ErrRefundAlreadyExists {
		t.Errorf("expected ErrRefundAlreadyExists, got %v", err)
	}

	if len(refundRepo.refunds) != countAfterFirst {
		t.Errorf("refund count should not change on duplicate, expected %d, got %d", countAfterFirst, len(refundRepo.refunds))
	}
}

type uuidGen func() string

var uuidCounter uint64

func uuid() string {
	return "test-uuid-" + strconv.FormatUint(atomic.AddUint64(&uuidCounter, 1), 10)
}

func TestExactlyOnceLedger_CaptureSingleEntry(t *testing.T) {
	svc, _, _, mockLedger, adapter, mockOrderRepo := setupTestService(t)

	mockOrderRepo.orders["order-901"] = &domain.Order{
		ID:             "order-901",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-capture-1",
	}
	adapter.responses[domain.PaymentProviderOpCapture] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-capture-1",
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-901",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-capture-exact", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if _, err := svc.CapturePayment(context.Background(), "org-test", "actor-1", p.ID, "idem-capture-exact", nil); err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	if len(mockLedger.entries) != 2 {
		t.Fatalf("expected 2 ledger entries (DEBIT+CREDIT), got %d", len(mockLedger.entries))
	}

	debitCount := 0
	for _, e := range mockLedger.entries {
		if e.EntryType == domain.LedgerEntryTypeDebit {
			debitCount++
		}
	}
	if debitCount != 1 {
		t.Errorf("expected 1 DEBIT entry, got %d", debitCount)
	}
}

func TestExactlyOnceLedger_CaptureConcurrent_NoDuplicate(t *testing.T) {
	svc, mockRepo, _, mockLedger, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-capture-concurrent",
	}
	adapter.responses[domain.PaymentProviderOpCapture] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-capture-concurrent",
	}

	mockOrderRepo.orders["order-concurrent-capture"] = &domain.Order{
		ID:             "order-concurrent-capture",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-concurrent-capture",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-capture-concurrent", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = svc.CapturePayment(context.Background(), p.TenantID, "actor-1", p.ID, fmt.Sprintf("idem-capture-concurrent-%d", idx), nil)
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}
	if successCount != 1 {
		t.Errorf("expected 1 successful capture, got %d", successCount)
	}

	if len(mockLedger.entries) != 2 {
		t.Errorf("expected 2 ledger entries (DEBIT+CREDIT) after concurrent captures, got %d", len(mockLedger.entries))
	}

	updated, _ := mockRepo.FindByID(context.Background(), p.TenantID, p.ID)
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected payment status Captured, got %s", updated.Status)
	}
}

func TestExactlyOnceLedger_RefundConcurrent_NoDuplicate(t *testing.T) {
	svc, _, mockRefundRepo, mockLedger, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-refund-concurrent",
	}
	adapter.responses[domain.PaymentProviderOpRefund] = domain.PaymentProviderResponse{
		Status:            "succeeded",
		ProviderPaymentID: "ext-refund-concurrent",
	}

	mockOrderRepo.orders["order-ref-concurrent"] = &domain.Order{
		ID:             "order-ref-concurrent",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-ref-concurrent",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-concurrent", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	ledgerBeforeRefund := len(mockLedger.entries)

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, errs[idx] = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID,
				fmt.Sprintf("idem-refund-concurrent-%d", idx), &domain.Money{AmountMinor: 1000, Currency: "USD"})
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}
	if successCount < 1 {
		t.Errorf("expected at least 1 successful refund, got %d", successCount)
	}

	newLedgerEntries := len(mockLedger.entries) - ledgerBeforeRefund
	if newLedgerEntries > 2 {
		t.Errorf("expected at most 2 new ledger entries from single refund (DEBIT+CREDIT), got %d", newLedgerEntries)
	}

	if len(mockRefundRepo.refunds) > 1 {
		t.Errorf("expected at most 1 refund to succeed, got %d", len(mockRefundRepo.refunds))
	}
}

func TestRefundPayment_TenantIsolation(t *testing.T) {
	svc, _, _, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-tenant-refund",
	}

	mockOrderRepo.orders["order-tenant-refund"] = &domain.Order{
		ID:             "order-tenant-refund",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-tenant-refund",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-tenant-refund", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-other", "actor-1", p.ID, "idem-tenant-refund-other", nil)
	if err == nil {
		t.Fatal("expected error for refund by different tenant")
	}
}

func TestAuthorizePayment_TenantIsolation(t *testing.T) {
	svc, mockRepo, _, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-auth-tenant",
	}

	mockOrderRepo.orders["order-auth-tenant"] = &domain.Order{
		ID:             "order-auth-tenant",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 500, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-auth-tenant",
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-auth-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.AuthorizePayment(context.Background(), "org-other", "actor-1", p.ID, "idem-auth-tenant")
	if err == nil {
		t.Fatal("expected error for AuthorizePayment by different tenant")
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusAuthorized {
		t.Errorf("payment should remain in Authorized state, got %s", updated.Status)
	}
}

func TestCapturePayment_TenantIsolation(t *testing.T) {
	svc, mockRepo, _, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-capture-tenant",
	}

	mockOrderRepo.orders["order-capture-tenant"] = &domain.Order{
		ID:             "order-capture-tenant",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 500, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-capture-tenant",
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-capture-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.CapturePayment(context.Background(), "org-other", "actor-1", p.ID, "idem-capture-tenant", nil)
	if err == nil {
		t.Fatal("expected error for CapturePayment by different tenant")
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusAuthorized {
		t.Errorf("payment should remain in Authorized state, got %s", updated.Status)
	}
}

func TestCancelPayment_TenantIsolation(t *testing.T) {
	svc, mockRepo, _, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-cancel-tenant",
	}

	mockOrderRepo.orders["order-cancel-tenant"] = &domain.Order{
		ID:             "order-cancel-tenant",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 500, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-cancel-tenant",
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = svc.CancelPayment(context.Background(), "org-other", "actor-1", p.ID, "idem-cancel-tenant")
	if err == nil {
		t.Fatal("expected error for CancelPayment by different tenant")
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusAuthorized {
		t.Errorf("payment should remain in Authorized state, got %s", updated.Status)
	}
}

func TestCreatePayment_TenantIsolation_OrderCheck(t *testing.T) {
	svc, _, _, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "authorized",
		ProviderPaymentID: "ext-create-tenant-isolation",
	}

	mockOrderRepo.orders["order-create-tenant"] = &domain.Order{
		ID:             "order-create-tenant",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Status:         domain.OrderStatusPendingPayment,
	}

	_, err := svc.CreatePayment(context.Background(), "org-other", "actor-1", "order-create-tenant",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-create-tenant-isolation", nil)
	if err == nil {
		t.Fatal("expected error for CreatePayment by tenant not owning the order")
	}
}

func TestConcurrentRefunds_TenantIsolation(t *testing.T) {
	svc, _, mockRefundRepo, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-concurrent-ref-tenant",
	}

	mockOrderRepo.orders["order-concurrent-ref-tenant"] = &domain.Order{
		ID:             "order-concurrent-ref-tenant",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-concurrent-ref-tenant",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-concurrent-ref-tenant", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, errs[idx] = svc.RefundPayment(context.Background(), "org-other", "actor-1", p.ID,
				fmt.Sprintf("idem-concurrent-ref-tenant-%d", idx), nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("attempt %d: expected error for refund by different tenant", i)
		}
	}

	if len(mockRefundRepo.refunds) != 0 {
		t.Errorf("expected 0 refunds created by different tenant, got %d", len(mockRefundRepo.refunds))
	}
}

func TestRefundPayment_ExceedsAmount_SingleEntry(t *testing.T) {
	svc, _, mockRefundRepo, _, adapter, mockOrderRepo := setupTestService(t)

	adapter.responses[domain.PaymentProviderOpCreate] = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-refund-exceeds",
	}

	mockOrderRepo.orders["order-refund-exceeds"] = &domain.Order{
		ID:             "order-refund-exceeds",
		OrganizationID: "org-test",
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
	}

	p, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-refund-exceeds",
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-exceeds", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID,
		"idem-refund-exceeds-1", &domain.Money{AmountMinor: 500, Currency: "USD"})
	if err != nil {
		t.Fatalf("first partial refund failed: %v", err)
	}

	_, _, err = svc.RefundPayment(context.Background(), "org-test", "actor-1", p.ID,
		"idem-refund-exceeds-2", &domain.Money{AmountMinor: 600, Currency: "USD"})
	if err == nil {
		t.Fatal("expected error for refund exceeding remaining amount")
	}

	if len(mockRefundRepo.refunds) != 1 {
		t.Errorf("expected 1 refund (second should be rejected), got %d", len(mockRefundRepo.refunds))
	}
}

func TestCreatePayment_RejectsOrderNotAwaitingPayment(t *testing.T) {
	for _, status := range []domain.OrderStatus{domain.OrderStatusPaid, domain.OrderStatusConfirmed, domain.OrderStatusCancelled} {
		mockOrders := &mockOrderRepo{orders: map[string]*domain.Order{}}
		mockOrders.orders["order-x"] = &domain.Order{
			ID:             "order-x",
			OrganizationID: "org-test",
			TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
			Currency:       "USD",
			Status:         status,
		}
		svc := NewPaymentService(PaymentServiceConfig{
			Payments: newMockPaymentRepo(),
			Orders:   mockOrders,
			Logger:   zerolog.Nop(),
		})
		_, err := svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-x",
			domain.Money{AmountMinor: 1000, Currency: "USD"}, domain.PaymentMethodCard, "m-1", "idem-"+string(status), nil)
		if !errors.Is(err, domain.ErrOrderNotPayable) {
			t.Errorf("status %s: expected ErrOrderNotPayable, got %v", status, err)
		}
	}
}
