package payment

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

type mockWebhookEventStore struct {
	mu     sync.Mutex
	events map[string]*domain.PaymentWebhookEvent
	claims map[string]bool
}

func newMockWebhookStore() *mockWebhookEventStore {
	return &mockWebhookEventStore{
		events: make(map[string]*domain.PaymentWebhookEvent),
		claims: make(map[string]bool),
	}
}

func (m *mockWebhookEventStore) PersistEvent(ctx context.Context, event *domain.PaymentWebhookEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := event.ProviderCode + ":" + event.ProviderMsgID
	if event.ID == "" {
		event.ID = "webhook-event-" + key
	}
	if _, exists := m.events[key]; exists {
		return nil
	}
	event.Status = domain.PaymentWebhookStatusPending
	m.events[key] = event
	return nil
}

func (m *mockWebhookEventStore) TryClaim(ctx context.Context, providerCode, providerMsgID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := providerCode + ":" + providerMsgID
	e, ok := m.events[key]
	if !ok {
		return false, nil
	}
	if e.Status != domain.PaymentWebhookStatusPending && e.Status != domain.PaymentWebhookStatusFailed {
		return false, nil
	}
	e.Status = domain.PaymentWebhookStatusProcessing
	m.claims[key] = true
	return true, nil
}

func (m *mockWebhookEventStore) MarkProcessed(ctx context.Context, providerCode, providerMsgID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := providerCode + ":" + providerMsgID
	if e, ok := m.events[key]; ok {
		e.Status = domain.PaymentWebhookStatusProcessed
		return nil
	}
	return domain.ErrWebhookEventDuplicate
}

func (m *mockWebhookEventStore) MarkReview(ctx context.Context, providerCode, providerMsgID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.events[providerCode+":"+providerMsgID]; ok {
		e.Status, e.ErrorMessage = domain.PaymentWebhookStatusReview, reason
		return nil
	}
	return domain.ErrWebhookEventDuplicate
}

func (m *mockWebhookEventStore) MarkFailed(ctx context.Context, providerCode, providerMsgID string, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := providerCode + ":" + providerMsgID
	if e, ok := m.events[key]; ok {
		e.Status = domain.PaymentWebhookStatusFailed
		e.ErrorMessage = errorMsg
		return nil
	}
	return domain.ErrWebhookEventDuplicate
}

func (m *mockWebhookEventStore) FindByProviderMessageID(ctx context.Context, providerCode, providerMsgID string) (*domain.PaymentWebhookEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := providerCode + ":" + providerMsgID
	e, ok := m.events[key]
	if !ok {
		return nil, domain.ErrWebhookEventDuplicate
	}
	return e, nil
}

func setupWebhookTestService(t *testing.T) (*WebhookHandler, *mockPaymentRepo, *mockRefundRepo, *mockLedgerRepo, *mockPaymentProviderAdapter, *mockWebhookEventStore, *mockProviderRepo) {
	t.Helper()

	mockRepo := newMockPaymentRepo()
	mockRefundRepo := newMockRefundRepo()
	mockLedgerRepo := &mockLedgerRepo{}
	mockEvents := newMockWebhookStore()
	mockProviders := newMockProviderRepo()

	adapter := newMockPaymentAdapter("provider-test")
	adapter.capabilities[domain.PaymentProviderOpCreate] = true
	adapter.capabilities[domain.PaymentProviderOpCancel] = true
	adapter.capabilities[domain.PaymentProviderOpGet] = true
	adapter.capabilities[domain.PaymentProviderOpCapture] = true
	adapter.capabilities[domain.PaymentProviderOpRefund] = true

	providerEntry := &domain.IntegrationProvider{
		ID:             "provider-test",
		OrganizationID: "org-test",
		Code:           "mock-provider",
		Name:           "Mock Provider",
		Status:         domain.ProviderStatusActive,
		Capabilities:   []domain.Capability{domain.CapabilityPayment},
	}
	mockProviders.providers["mock-provider"] = providerEntry

	h := &WebhookHandler{
		payments:          mockRepo,
		refunds:           mockRefundRepo,
		accounting:        newMockAccounting(mockRepo, mockRefundRepo, mockLedgerRepo),
		locker:            NewLocalLocker(),
		events:            mockEvents,
		providers:         mockProviders,
		signatureVerifier: nil,
		logger:            zerolog.Nop(),
	}

	return h, mockRepo, mockRefundRepo, mockLedgerRepo, adapter, mockEvents, mockProviders
}

func TestWebhookHandler_PaymentSucceeded(t *testing.T) {
	h, mockRepo, _, _, _, mockEvents, _ := setupWebhookTestService(t)

	// Pre-create a payment that the webhook will reconcile
	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-1", 1000)
	p.ProviderPaymentID = "ext-pay-001"
	p.Status = domain.PaymentStatusAuthorized
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-001",
		ProviderPaymentID: "ext-pay-001",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-001", body)
	if err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	updated, err := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if err != nil {
		t.Fatalf("failed to find updated payment: %v", err)
	}
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED, got %s", updated.Status)
	}

	// Verify idempotency — second call should be a no-op
	err = h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-001", body)
	if err != nil {
		t.Fatalf("second ProcessWebhook should be idempotent no-op: %v", err)
	}

	// Verify event was marked processed
	event, _ := mockEvents.FindByProviderMessageID(context.Background(), "mock-provider", "msg-001")
	if event.Status != domain.PaymentWebhookStatusProcessed {
		t.Errorf("expected event PROCESSED, got %s", event.Status)
	}
}

func TestWebhookHandler_PaymentFailed(t *testing.T) {
	h, mockRepo, _, _, _, _, _ := setupWebhookTestService(t)

	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-2", 1000)
	p.ProviderPaymentID = "ext-pay-002"
	p.Status = domain.PaymentStatusAuthorized
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentFailed),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-002",
		ProviderPaymentID: "ext-pay-002",
		Status:            "failed",
	}
	body, _ := json.Marshal(payload)

	if err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-002", body); err != nil {
		t.Fatalf("ProcessWebhook failed: %v", err)
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusFailed {
		t.Errorf("expected FAILED, got %s", updated.Status)
	}
}

func TestWebhookHandler_SignatureVerification(t *testing.T) {
	h, _, _, _, _, _, _ := setupWebhookTestService(t)
	h.signatureVerifier = &mockSignatureVerifier{err: domain.ErrWebhookSignatureInvalid}

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-003",
		ProviderPaymentID: "ext-pay-003",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "bad-sig", body)
	if err != domain.ErrWebhookSignatureInvalid {
		t.Errorf("expected ErrWebhookSignatureInvalid, got %v", err)
	}
}

func TestWebhookHandler_UnknownPayment(t *testing.T) {
	h, _, _, _, _, _, _ := setupWebhookTestService(t)

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-004",
		ProviderPaymentID: "ext-pay-unknown",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	// A payment not recorded (yet) is refused so the provider sends the event again: acknowledging it would lose it
	// when the record is only late.
	if err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-004", body); err == nil {
		t.Fatal("an event about an unknown payment must be retried")
	}
}

func TestWebhookHandler_UnknownEventType(t *testing.T) {
	h, mockRepo, _, _, _, mockEvents, _ := setupWebhookTestService(t)

	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-5", 1000)
	p.ProviderPaymentID = "ext-pay-005"
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	payload := WebhookPayload{
		EventType:         "payment.custom_event",
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-005",
		ProviderPaymentID: "ext-pay-005",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-005", body)
	if err != nil {
		t.Fatalf("ProcessWebhook should not error for unknown event type: %v", err)
	}

	// Event should still be marked processed
	event, _ := mockEvents.FindByProviderMessageID(context.Background(), "mock-provider", "msg-005")
	if event.Status != domain.PaymentWebhookStatusProcessed {
		t.Errorf("expected event PROCESSED, got %s", event.Status)
	}
}

func TestWebhookHandler_ConcurrentDuplicate_NoDuplicateProcessing(t *testing.T) {
	h, mockRepo, _, mockLedger, _, mockEvents, _ := setupWebhookTestService(t)

	p := domain.NewPayment("org-test", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-concurrent-whack", 1000)
	p.ProviderPaymentID = "ext-pay-concurrent"
	p.Status = domain.PaymentStatusAuthorized
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-concurrent",
		ProviderPaymentID: "ext-pay-concurrent",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			errs[idx] = h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-concurrent", body)
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}

	event, _ := mockEvents.FindByProviderMessageID(context.Background(), "mock-provider", "msg-concurrent")
	if event.Status != domain.PaymentWebhookStatusProcessed {
		t.Errorf("expected event PROCESSED, got %s", event.Status)
	}

	if len(mockLedger.entries) > 2 {
		t.Errorf("expected at most 2 ledger entries (DEBIT+CREDIT) for a single capture, got %d", len(mockLedger.entries))
	}
}

func TestWebhookHandler_Ordering_SequentialMessages(t *testing.T) {
	h, mockRepo, _, mockLedger, _, _, _ := setupWebhookTestService(t)

	p := domain.NewPayment("org-test", "order-order", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-order-test", 1000)
	p.ProviderPaymentID = "ext-pay-order"
	p.Status = domain.PaymentStatusAuthorized
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	authorizePayload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-order-1",
		ProviderPaymentID: "ext-pay-order",
		Status:            "authorized",
	}

	capturePayload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-order-2",
		ProviderPaymentID: "ext-pay-order",
		Status:            "captured",
	}

	authBody, _ := json.Marshal(authorizePayload)
	capBody, _ := json.Marshal(capturePayload)

	if err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-order-1", authBody); err != nil {
		t.Fatalf("first webhook failed: %v", err)
	}
	if err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig-order-2", capBody); err != nil {
		t.Fatalf("second webhook failed: %v", err)
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED after ordered events, got %s", updated.Status)
	}

	if len(mockLedger.entries) == 0 {
		t.Error("expected at least 1 ledger entry after capture")
	}
}

func TestWebhookHandler_UnknownPayment_TenantIsolation(t *testing.T) {
	h, _, _, _, _, _, _ := setupWebhookTestService(t)

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-other",
		ProviderMessageID: "msg-tenant-unknown",
		ProviderPaymentID: "ext-pay-tenant-unknown",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	err := h.ProcessWebhook(context.Background(), "org-other", "mock-provider", "sig-tenant-unknown", body)
	if err == nil {
		t.Fatal("expected error for webhook from different tenant (provider not found for that tenant)")
	}
}

func TestWebhookHandler_ProviderNotSupported(t *testing.T) {
	h, _, _, _, _, _, _ := setupWebhookTestService(t)

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "unknown-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-unknown-provider",
		ProviderPaymentID: "ext-pay-unknown-provider",
		Status:            "captured",
	}
	body, _ := json.Marshal(payload)

	err := h.ProcessWebhook(context.Background(), "org-test", "unknown-provider", "sig-unknown-provider", body)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestWebhookHandler_MultipleMessages_SamePayment(t *testing.T) {
	h, mockRepo, _, _, _, mockEvents, _ := setupWebhookTestService(t)

	p := domain.NewPayment("org-test", "order-multi", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "provider-test", "idem-multi-msg", 1000)
	p.ProviderPaymentID = "ext-pay-multi"
	p.Status = domain.PaymentStatusAuthorized
	if err := mockRepo.Create(context.Background(), p); err != nil {
		t.Fatalf("failed to create payment: %v", err)
	}

	payload := WebhookPayload{
		EventType:         string(domain.WebhookEventPaymentSucceeded),
		ProviderCode:      "mock-provider",
		OrgID:             "org-test",
		ProviderMessageID: "msg-multi-%s",
		ProviderPaymentID: "ext-pay-multi",
		Status:            "captured",
	}

	for i := 0; i < 5; i++ {
		msgID := fmt.Sprintf("msg-multi-%d", i)
		payload.ProviderMessageID = msgID
		body, _ := json.Marshal(payload)
		if err := h.ProcessWebhook(context.Background(), "org-test", "mock-provider", fmt.Sprintf("sig-multi-%d", i), body); err != nil {
			t.Fatalf("webhook %d failed: %v", i, err)
		}
	}

	updated, _ := mockRepo.FindByID(context.Background(), "org-test", p.ID)
	if updated.Status != domain.PaymentStatusCaptured {
		t.Errorf("expected CAPTURED after multiple webhook messages, got %s", updated.Status)
	}

	for i := 0; i < 5; i++ {
		event, _ := mockEvents.FindByProviderMessageID(context.Background(), "mock-provider", fmt.Sprintf("msg-multi-%d", i))
		if event.Status != domain.PaymentWebhookStatusProcessed {
			t.Errorf("event %d should be PROCESSED, got %s", i, event.Status)
		}
	}
}
