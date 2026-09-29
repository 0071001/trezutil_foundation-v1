package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
)

type integrationTestProviderAdapter struct {
	id               string
	orgID            string
	createResp       domain.PaymentProviderResponse
	captureResp      domain.PaymentProviderResponse
	refundResp       domain.RefundProviderResponse
	cancelResp       domain.PaymentProviderResponse
	createCallCount  int32
	captureCallCount int32
	refundCallCount  int32
	cancelCallCount  int32
}

func newIntegrationTestAdapter(id, orgID string) *integrationTestProviderAdapter {
	return &integrationTestProviderAdapter{
		id:    id,
		orgID: orgID,
	}
}

func (a *integrationTestProviderAdapter) ProviderID() string {
	return a.id
}

func (a *integrationTestProviderAdapter) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapabilityPayment}
}

func (a *integrationTestProviderAdapter) Health(ctx context.Context) domain.ProviderHealthStatus {
	return domain.ProviderHealthHealthy
}

func (a *integrationTestProviderAdapter) CreatePayment(ctx context.Context, req domain.PaymentProviderRequest) (domain.PaymentProviderResponse, error) {
	count := atomic.AddInt32(&a.createCallCount, 1)
	resp := a.createResp
	if resp.ProviderPaymentID == "" {
		resp.ProviderPaymentID = "ext-" + req.IdempotencyKey
	}
	if count > 1 {
		resp.Status = "authorized"
	}
	return resp, nil
}

// GetPayment reports the provider-side state of an existing payment. After
// creation the payer has completed authorization, so the provider reports it
// authorized (AuthorizePayment syncs this state instead of creating a second
// provider payment).
func (a *integrationTestProviderAdapter) GetPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	return domain.PaymentProviderResponse{ProviderPaymentID: providerPaymentID, Status: "authorized"}, nil
}

func (a *integrationTestProviderAdapter) CancelPayment(ctx context.Context, providerPaymentID string) (domain.PaymentProviderResponse, error) {
	atomic.AddInt32(&a.cancelCallCount, 1)
	return a.cancelResp, nil
}

func (a *integrationTestProviderAdapter) CapturePayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.PaymentProviderResponse, error) {
	atomic.AddInt32(&a.captureCallCount, 1)
	resp := a.captureResp
	if resp.ProviderPaymentID == "" {
		resp.ProviderPaymentID = providerPaymentID + "-cap"
	}
	return resp, nil
}

func (a *integrationTestProviderAdapter) RefundPayment(ctx context.Context, providerPaymentID string, amount *domain.Money) (domain.RefundProviderResponse, error) {
	atomic.AddInt32(&a.refundCallCount, 1)
	resp := a.refundResp
	if resp.ProviderRefundID == "" {
		resp.ProviderRefundID = providerPaymentID + "-ref"
	}
	if resp.Amount.AmountMinor == 0 {
		resp.Amount = *amount
	}
	return resp, nil
}

type testEnv struct {
	db         *TestDB
	svc        *payment.PaymentService
	whHandler  *payment.WebhookHandler
	orgID      string
	orderID    string
	providerID string
	ctx        context.Context
	adapter    *integrationTestProviderAdapter
	orders     *postgres.OrderRepository
}

func setupPaymentIntegration(t *testing.T) *testEnv {
	t.Helper()

	db := NewTestDB(t)
	ctx := context.Background()

	orgID := testUUID()
	sellerID := testUUID()
	customerID := testUUID()
	storeID := testUUID()
	orderID := testUUID()
	providerID := testUUID()

	createTestOrganization(t, db, ctx, orgID)
	createTestUser(t, db, ctx, customerID)
	createTestSeller(t, db, ctx, sellerID, customerID, orgID)
	createTestStore(t, db, ctx, storeID, orgID)

	order := &domain.Order{
		ID:             orderID,
		OrganizationID: orgID,
		SellerID:       sellerID,
		CustomerID:     customerID,
		StoreID:        storeID,
		Items:          []*domain.OrderItem{},
		TotalAmount:    domain.Money{AmountMinor: 1000, Currency: "USD"},
		Currency:       "USD",
		Status:         domain.OrderStatusPendingPayment,
		Version:        1,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}
	orderRepo := postgres.NewOrderRepository(db.DB)
	if err := orderRepo.Create(ctx, order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	adapter := newIntegrationTestAdapter(providerID, orgID)
	adapter.createResp = domain.PaymentProviderResponse{
		Status:            "",
		ProviderPaymentID: "ext-payment-1",
	}
	adapter.captureResp = domain.PaymentProviderResponse{
		Status:            "captured",
		ProviderPaymentID: "ext-payment-1",
	}
	adapter.refundResp = domain.RefundProviderResponse{
		Status:           "succeeded",
		Amount:           domain.Money{AmountMinor: 1000, Currency: "USD"},
		ProviderRefundID: "ext-refund-1",
	}
	adapter.cancelResp = domain.PaymentProviderResponse{
		Status:            "cancelled",
		ProviderPaymentID: "ext-payment-1",
	}

	providerEntry := &domain.IntegrationProvider{
		ID:             providerID,
		OrganizationID: orgID,
		Code:           "test-provider",
		Name:           "Test Provider",
		Type:           "MOCK",
		Status:         domain.ProviderStatusActive,
		Capabilities:   []domain.Capability{domain.CapabilityPayment},
		HealthStatus:   domain.ProviderHealthHealthy,
		Version:        1,
		CreatedAt:      time.Now().Unix(),
		UpdatedAt:      time.Now().Unix(),
	}

	providerRepo := postgres.NewProviderRepository(db.DB)
	if err := providerRepo.Create(ctx, providerEntry); err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	registry := provider.NewProviderRegistry()
	if err := registry.Register(providerEntry, adapter); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	execRepo := postgres.NewProviderExecutionRepository(db.DB)
	orchestrator := provider.NewProviderOrchestrator(registry, execRepo, provider.OrchestratorConfig{
		Logger: zerolog.Nop(),
	})

	paymentRepo := postgres.NewPaymentRepository(db.DB)
	refundRepo := postgres.NewRefundRepository(db.DB)
	ledgerRepo := postgres.NewLedgerRepository(db.DB)
	webhookStore := postgres.NewPaymentWebhookEventStore(db.DB)

	svc := payment.NewPaymentService(payment.PaymentServiceConfig{
		Payments:     paymentRepo,
		Refunds:      refundRepo,
		Ledger:       ledgerRepo,
		Orders:       orderRepo,
		Orchestrator: orchestrator,
		Logger:       zerolog.Nop(),
		Locker:       postgres.NewAdvisoryLocker(db.DB),
	})

	whHandler := payment.NewWebhookHandler(payment.WebhookHandlerConfig{
		Payments:          paymentRepo,
		Refunds:           refundRepo,
		Ledger:            ledgerRepo,
		Events:            webhookStore,
		Providers:         providerRepo,
		SignatureVerifier: nil,
		Orders:            orderRepo,
		Logger:            zerolog.Nop(),
	})

	return &testEnv{
		db:         db,
		svc:        svc,
		whHandler:  whHandler,
		orgID:      orgID,
		orderID:    orderID,
		providerID: providerID,
		ctx:        ctx,
		adapter:    adapter,
		orders:     orderRepo,
	}
}

func TestPaymentIntegration_CreateAndRetrieve(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-create-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	if p.ID == "" {
		t.Fatal("payment ID should not be empty")
	}
	if p.TenantID != env.orgID {
		t.Errorf("expected tenant %s, got %s", env.orgID, p.TenantID)
	}
	if p.Status != domain.PaymentStatusCreated {
		t.Errorf("expected CREATED, got %s", p.Status)
	}

	found, err := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if err != nil {
		t.Fatalf("GetPayment failed: %v", err)
	}
	if found.ID != p.ID {
		t.Errorf("expected payment ID %s, got %s", p.ID, found.ID)
	}
}

func TestPaymentIntegration_AuthorizeAndCapture(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-acap-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-acap-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusAuthorized {
		t.Errorf("expected AUTHORIZED, got %s", p.Status)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-acap-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}
	if p.CapturedAt == nil {
		t.Error("expected CapturedAt to be set")
	}
}

func TestPaymentIntegration_Cancel(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.CancelPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-cancel-cancel")
	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", p.Status)
	}
}

func TestPaymentIntegration_Refund(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	refundAmount := domain.Money{AmountMinor: 500, Currency: "USD"}
	p, refund, err := env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-r1", &refundAmount)
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}
	if refund == nil {
		t.Fatal("expected refund to be returned")
	}
	if refund.Status != domain.RefundStatusSucceeded {
		t.Errorf("expected SUCCEEDED, got %s", refund.Status)
	}
	if p.Status != domain.PaymentStatusPartiallyRefunded {
		t.Errorf("expected PARTIALLY_REFUNDED, got %s", p.Status)
	}
	if p.RefundedAmount.AmountMinor != 500 {
		t.Errorf("expected refunded amount 500, got %d", p.RefundedAmount.AmountMinor)
	}
}

func TestPaymentIntegration_TenantIsolation(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	otherOrgID := testUUID()
	createTestOrganization(t, env.db, env.ctx, otherOrgID)

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-tenant-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	_, err = env.svc.GetPayment(env.ctx, otherOrgID, p.ID)
	if err == nil {
		t.Fatal("expected error when fetching payment from different tenant")
	}
	if !errors.Is(err, domain.ErrPaymentNotFound) {
		t.Errorf("expected ErrPaymentNotFound, got %v", err)
	}

	_, err = env.svc.CapturePayment(env.ctx, otherOrgID, "actor-1", p.ID, "idem-tenant-cap", nil)
	if err == nil {
		t.Fatal("expected error when capturing payment from different tenant")
	}

	_, _, err = env.svc.RefundPayment(env.ctx, otherOrgID, "actor-1", p.ID, "idem-tenant-ref", nil)
	if err == nil {
		t.Fatal("expected error when refunding payment from different tenant")
	}

	_, err = env.svc.CancelPayment(env.ctx, otherOrgID, "actor-1", p.ID, "idem-tenant-cancel")
	if err == nil {
		t.Fatal("expected error when cancelling payment from different tenant")
	}
}

func TestPaymentIntegration_IdempotencyCreate(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p1, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-idem-1", nil)
	if err != nil {
		t.Fatalf("first CreatePayment failed: %v", err)
	}

	p2, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-idem-1", nil)
	if err != nil {
		t.Fatalf("second CreatePayment (idempotent) failed: %v", err)
	}
	if p1.ID != p2.ID {
		t.Errorf("expected same payment ID (idempotent), got %s and %s", p1.ID, p2.ID)
	}
}

func TestPaymentIntegration_IdempotencyConflict(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	_, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-conflict-1", nil)
	if err != nil {
		t.Fatalf("first CreatePayment failed: %v", err)
	}

	_, err = env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-conflict-1", nil)
	if err == nil {
		t.Fatal("expected error for idempotency key conflict (different amount)")
	}
	if !errors.Is(err, domain.ErrIdempotencyConflict) {
		t.Errorf("expected ErrIdempotencyConflict, got %v", err)
	}
}

func TestPaymentIntegration_ConcurrentCreate(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	var wg sync.WaitGroup
	var paymentIDs []string
	var mu sync.Mutex
	errs := make([]error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
				domain.Money{AmountMinor: 1000, Currency: "USD"},
				domain.PaymentMethodCard, "method-123", "idem-concurrent-create", nil)
			if err != nil {
				errs[idx] = err
				return
			}
			mu.Lock()
			paymentIDs = append(paymentIDs, p.ID)
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(paymentIDs) == 0 {
		t.Fatal("expected at least 1 successful payment creation")
	}

	uniqueIDs := make(map[string]bool)
	for _, id := range paymentIDs {
		uniqueIDs[id] = true
	}
	if len(uniqueIDs) != 1 {
		t.Errorf("expected exactly 1 unique payment ID, got %d (unique: %d)", len(paymentIDs), len(uniqueIDs))
	}

	var count int
	rows, err := env.db.DB.QueryContext(env.ctx, "SELECT COUNT(*) FROM payments WHERE organization_id = $1", env.orgID)
	if err != nil {
		t.Fatalf("failed to count payments: %v", err)
	}
	rows.Next()
	rows.Scan(&count)
	rows.Close()
	if count != 1 {
		t.Errorf("expected 1 payment in database, got %d", count)
	}
}

func TestPaymentIntegration_ConcurrentCapture(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-concurrent-cap-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-concurrent-cap-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, fmt.Sprintf("idem-concurrent-cap-%d", idx), nil)
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

	ledgerRepo := postgres.NewLedgerRepository(env.db.DB)
	entries, _ := ledgerRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)

	debitCount := 0
	creditCount := 0
	for _, e := range entries {
		if e.EntryType == domain.LedgerEntryTypeDebit {
			debitCount++
		}
		if e.EntryType == domain.LedgerEntryTypeCredit {
			creditCount++
		}
	}
	if debitCount != 1 {
		t.Errorf("expected exactly 1 DEBIT ledger entry, got %d", debitCount)
	}
	if creditCount != 1 {
		t.Errorf("expected exactly 1 CREDIT ledger entry, got %d", creditCount)
	}

	updated, _ := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", updated.Status)
	}
}

func TestPaymentIntegration_ConcurrentRefund(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-concurrent-ref-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-concurrent-ref-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-concurrent-ref-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, errs[idx] = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID,
				fmt.Sprintf("idem-concurrent-ref-%d", idx),
				&domain.Money{AmountMinor: 100, Currency: "USD"})
		}(i)
	}
	wg.Wait()

	updated, _ := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if updated.RefundedAmount.AmountMinor > 1000 {
		t.Errorf("refunded amount %d must not exceed captured amount 1000", updated.RefundedAmount.AmountMinor)
	}
}

func TestPaymentIntegration_RefundIdempotency(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refidempotent-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refidempotent-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refidempotent-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	refundAmount := domain.Money{AmountMinor: 500, Currency: "USD"}

	_, _, err = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refidempotent-r1", &refundAmount)
	if err != nil {
		t.Fatalf("first refund failed: %v", err)
	}

	_, _, err = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refidempotent-r1", &refundAmount)
	if err == nil {
		t.Fatal("expected ErrRefundAlreadyExists on duplicate refund idempotency key")
	} else if !errors.Is(err, domain.ErrRefundAlreadyExists) {
		t.Errorf("expected ErrRefundAlreadyExists, got %v", err)
	}

	refundRepo := postgres.NewRefundRepository(env.db.DB)
	refunds, _ := refundRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)
	if len(refunds) != 1 {
		t.Errorf("expected exactly 1 refund in database, got %d", len(refunds))
	}

	rows, err := env.db.DB.QueryContext(env.ctx,
		"SELECT COUNT(*) FROM payment_ledger WHERE organization_id = $1 AND (payment_id = $2 OR refund_id IN (SELECT id FROM refunds WHERE payment_id = $2))",
		env.orgID, p.ID)
	if err != nil {
		t.Fatalf("failed to count all ledger entries: %v", err)
	}
	rows.Next()
	var totalEntries int
	rows.Scan(&totalEntries)
	rows.Close()
	if totalEntries != 3 {
		t.Errorf("expected 3 ledger entries (2 capture + 1 refund), got %d", totalEntries)
	}
}

func TestPaymentIntegration_LedgerExactlyOnce(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-ledger-once-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-ledger-once-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-ledger-once-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	_, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-ledger-once-cap-dup", nil)
	if err == nil {
		t.Fatal("expected error on duplicate capture (already captured)")
	}

	ledgerRepo := postgres.NewLedgerRepository(env.db.DB)
	entries, _ := ledgerRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)

	debitCount := 0
	creditCount := 0
	for _, e := range entries {
		if e.EntryType == domain.LedgerEntryTypeDebit {
			debitCount++
		}
		if e.EntryType == domain.LedgerEntryTypeCredit {
			creditCount++
		}
	}
	if debitCount != 1 {
		t.Errorf("expected exactly 1 DEBIT entry, got %d", debitCount)
	}
	if creditCount != 1 {
		t.Errorf("expected exactly 1 CREDIT entry, got %d", creditCount)
	}
}

func TestPaymentIntegration_WebhookPersistence(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-webhook-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-webhook-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-webhook-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	p, err = env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if err != nil {
		t.Fatalf("GetPayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", p.Status)
	}

	rows, err := env.db.DB.QueryContext(env.ctx,
		"SELECT COUNT(*) FROM payment_webhook_events WHERE organization_id = $1", env.orgID)
	if err != nil {
		t.Fatalf("failed to query webhook events: %v", err)
	}
	var webhookCount int
	rows.Next()
	rows.Scan(&webhookCount)
	rows.Close()

	ledgerRepo := postgres.NewLedgerRepository(env.db.DB)
	entries, _ := ledgerRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)
	if len(entries) < 2 {
		t.Errorf("expected at least 2 ledger entries, got %d", len(entries))
	}
}

func TestPaymentIntegration_ConcurrentWebhook(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-concurrent-webhook-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-concurrent-webhook-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-concurrent-webhook-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	payload := payment.WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "test-provider",
		ProviderMessageID: "msg-concurrent-webhook",
		ProviderPaymentID: "ext-payment-1",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = env.whHandler.ProcessWebhook(env.ctx, env.orgID, "test-provider", "sig-concurrent", body)
		}(i)
	}
	wg.Wait()

	updated, _ := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED after concurrent webhooks, got %s", updated.Status)
	}

	rows, err := env.db.DB.QueryContext(env.ctx,
		"SELECT COUNT(*) FROM payment_webhook_events WHERE provider_code = $1 AND provider_msg_id = $2",
		"test-provider", "msg-concurrent-webhook")
	if err != nil {
		t.Fatalf("failed to count webhook events: %v", err)
	}
	var eventCount int
	rows.Next()
	rows.Scan(&eventCount)
	rows.Close()
	if eventCount != 1 {
		t.Errorf("expected exactly 1 webhook event, got %d", eventCount)
	}
}

func TestPaymentIntegration_OrderTenantIsolation(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	otherOrgID := testUUID()
	createTestOrganization(t, env.db, env.ctx, otherOrgID)

	_, err := env.svc.CreatePayment(env.ctx, otherOrgID, "actor-other", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-order-tenant-1", nil)
	if err == nil {
		t.Fatal("expected error when creating payment against order from different tenant")
	}
}

func TestPaymentIntegration_AmountValidation(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	_, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 500, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-amount-1", nil)
	if err == nil {
		t.Fatal("expected error for amount mismatch with order")
	}
}

func TestPaymentIntegration_CurrencyValidation(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	_, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "CDF"},
		domain.PaymentMethodCard, "method-123", "idem-currency-1", nil)
	if err == nil {
		t.Fatal("expected error for currency mismatch with order")
	}
}

func TestPaymentIntegration_FullWorkflow(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-workflow-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-workflow-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-workflow-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	_, _, err = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-workflow-ref",
		&domain.Money{AmountMinor: 300, Currency: "USD"})
	if err != nil {
		t.Fatalf("RefundPayment failed: %v", err)
	}

	final, _ := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if final.Status != domain.PaymentStatusPartiallyRefunded {
		t.Errorf("expected PARTIALLY_REFUNDED, got %s", final.Status)
	}
	if final.RefundedAmount.AmountMinor != 300 {
		t.Errorf("expected refunded amount 300, got %d", final.RefundedAmount.AmountMinor)
	}

	rows, err := env.db.DB.QueryContext(env.ctx,
		"SELECT COUNT(*) FROM payment_ledger WHERE organization_id = $1 AND (payment_id = $2 OR refund_id IN (SELECT id FROM refunds WHERE payment_id = $2))",
		env.orgID, p.ID)
	if err != nil {
		t.Fatalf("failed to count all ledger entries: %v", err)
	}
	rows.Next()
	var totalEntries int
	rows.Scan(&totalEntries)
	rows.Close()
	if totalEntries != 3 {
		t.Errorf("expected 3 ledger entries (2 capture + 1 refund), got %d", totalEntries)
	}
}

func TestPaymentIntegration_RefundExceedsAmount(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-refund-exceeds-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-exceeds-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-exceeds-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	_, _, err = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-exceeds-r1",
		&domain.Money{AmountMinor: 500, Currency: "USD"})
	if err != nil {
		t.Fatalf("first refund failed: %v", err)
	}

	_, _, err = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-refund-exceeds-r2",
		&domain.Money{AmountMinor: 600, Currency: "USD"})
	if err == nil {
		t.Fatal("expected error for refund exceeding captured amount")
	}

	refundRepo := postgres.NewRefundRepository(env.db.DB)
	refunds, _ := refundRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)
	if len(refunds) != 1 {
		t.Errorf("expected 1 refund, got %d", len(refunds))
	}
}

func TestPaymentIntegration_CancelAtVariousStates(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-cancel-states-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-cancel-states-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CancelPayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-cancel-states-cancel")
	if err != nil {
		t.Fatalf("CancelPayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCancelled {
		t.Errorf("expected CANCELLED, got %s", p.Status)
	}
}

func TestPaymentIntegration_PaymentNotFound(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	_, err := env.svc.GetPayment(env.ctx, env.orgID, testUUID())
	if err == nil {
		t.Fatal("expected error for nonexistent payment")
	}
}

func TestPaymentIntegration_LedgerEntriesPersist(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-ledger-persist-1", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}

	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-ledger-persist-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}

	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-ledger-persist-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}

	ledgerRepo := postgres.NewLedgerRepository(env.db.DB)
	entries, _ := ledgerRepo.FindByPaymentID(env.ctx, env.orgID, p.ID, 0, 100)

	debitFound := false
	creditFound := false
	for _, e := range entries {
		if e.EntryType == domain.LedgerEntryTypeDebit && e.Account == domain.LedgerAccountCash {
			debitFound = true
		}
		if e.EntryType == domain.LedgerEntryTypeCredit && e.Account == domain.LedgerAccountRevenue {
			creditFound = true
		}
	}
	if !debitFound {
		t.Error("expected to find DEBIT entry for cash account")
	}
	if !creditFound {
		t.Error("expected to find CREDIT entry for revenue account")
	}
}

// Concurrent FULL refunds with distinct idempotency keys must result in exactly
// one provider refund, one refund row and one refund ledger entry (money must
// not leave twice). Uses the production PostgreSQL advisory locker.
func TestPaymentIntegration_ConcurrentFullRefunds_ExactlyOneProviderRefund(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-full-ref-create", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-full-ref-auth"); err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	if p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-full-ref-cap", nil); err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}
	before := atomic.LoadInt32(&env.adapter.refundCallCount)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, _ = env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID,
				fmt.Sprintf("idem-full-ref-%d", idx), &domain.Money{AmountMinor: 1000, Currency: "USD"})
		}(i)
	}
	wg.Wait()

	if calls := atomic.LoadInt32(&env.adapter.refundCallCount) - before; calls != 1 {
		t.Fatalf("expected exactly 1 provider refund call, got %d", calls)
	}
	var succeeded int
	if err := env.db.DB.QueryRow(`SELECT COUNT(*) FROM refunds WHERE payment_id = $1 AND status = 'succeeded'`, p.ID).Scan(&succeeded); err != nil {
		t.Fatal(err)
	}
	if succeeded != 1 {
		t.Fatalf("expected 1 succeeded refund row, got %d", succeeded)
	}
	updated, _ := env.svc.GetPayment(env.ctx, env.orgID, p.ID)
	if updated.RefundedAmount.AmountMinor != 1000 {
		t.Fatalf("expected refunded amount 1000, got %d", updated.RefundedAmount.AmountMinor)
	}
}

// Capturing a payment must move its order to PAID (Order -> Payment -> Capture -> Ledger).
func TestPaymentIntegration_CaptureMarksOrderPaid(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-order-paid", nil)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-order-paid-auth"); err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	if _, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-order-paid-cap", nil); err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}
	order, err := env.orders.FindByIDAndOrg(env.ctx, env.orgID, env.orderID)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != domain.OrderStatusPaid {
		t.Fatalf("expected order status paid after capture, got %s", order.Status)
	}

	// A paid order must not accept a second payment.
	if _, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "idem-order-paid-2", nil); err == nil {
		t.Fatal("expected second payment for a paid order to be rejected")
	}

	var ledger int
	if err := env.db.DB.QueryRow(`SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID).Scan(&ledger); err != nil {
		t.Fatal(err)
	}
	if ledger != 2 {
		t.Fatalf("expected 2 capture ledger entries (debit cash, credit revenue), got %d", ledger)
	}
}
