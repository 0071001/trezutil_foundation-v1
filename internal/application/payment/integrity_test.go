package payment

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/internal/domain"
)

/* Failure scenarios: whatever breaks, the result is consistent, recoverable and traceable. */

type rig struct {
	svc     *PaymentService
	hook    *WebhookHandler
	payRepo *mockPaymentRepo
	refunds *mockRefundRepo
	ledger  *mockLedgerRepo
	acct    *mockAccounting
	adapter *mockPaymentProviderAdapter
	events  *mockWebhookEventStore
}

func newRig(t *testing.T) *rig {
	t.Helper()
	svc, payRepo, refunds, ledger, adapter, orders := setupTestService(t)
	acct := newMockAccounting(payRepo, refunds, ledger)
	locker := NewLocalLocker()
	svc.accounting, svc.locker = acct, locker
	events := newMockWebhookStore()
	providers := newMockProviderRepo()
	providers.providers["mock-provider"] = &domain.IntegrationProvider{ID: "provider-test", OrganizationID: "org-test", Code: "mock-provider", Status: domain.ProviderStatusActive}
	hook := NewWebhookHandler(WebhookHandlerConfig{Payments: payRepo, Refunds: refunds, Accounting: acct, Locker: locker, Events: events, Providers: providers, Orders: orders, Logger: zerolog.Nop()})
	return &rig{svc: svc, hook: hook, payRepo: payRepo, refunds: refunds, ledger: ledger, acct: acct, adapter: adapter, events: events}
}

func (r *rig) create(t *testing.T, status string) *domain.Payment {
	t.Helper()
	r.adapter.setResponse(domain.PaymentProviderOpCreate, domain.PaymentProviderResponse{Status: status, ProviderPaymentID: "ext-1"})
	p, err := r.svc.CreatePayment(context.Background(), "org-test", "actor-1", "order-1", domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", "key-1", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return p
}

func (r *rig) stored(t *testing.T, id string) *domain.Payment {
	t.Helper()
	p, err := r.payRepo.FindByID(context.Background(), "org-test", id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// lines counts the postings of a payment or a refund.
func (r *rig) lines(id string) int {
	r.ledger.mu.Lock()
	defer r.ledger.mu.Unlock()
	n := 0
	for _, e := range r.ledger.entries {
		if (e.PaymentID != nil && *e.PaymentID == id) || (e.RefundID != nil && *e.RefundID == id) {
			n++
		}
	}
	return n
}

func (r *rig) webhook(msgID, event, status, refundID string) error {
	body, _ := json.Marshal(WebhookPayload{EventType: event, ProviderMessageID: msgID, ProviderPaymentID: "ext-1", ProviderRefundID: refundID, Status: status})
	return r.hook.ProcessWebhook(context.Background(), "org-test", "mock-provider", "sig", body)
}

const succeeded = string(domain.WebhookEventPaymentSucceeded)

func TestALedgerFailureAfterCaptureLeavesNothingHalfDone(t *testing.T) {
	r := newRig(t)
	r.acct.failNext = 1
	p := r.create(t, "captured")
	got := r.stored(t, p.ID)
	if p.IsCaptured() || got.IsCaptured() || r.lines(p.ID) != 0 {
		t.Fatalf("shown as captured without accounting: returned %s, stored %s, lines %d", p.Status, got.Status, r.lines(p.ID))
	}
	if got.ProviderPaymentID != "ext-1" {
		t.Fatal("the provider's reference must be kept so the capture can be recorded later")
	}
	// The provider's webhook records it, once.
	for i := 0; i < 2; i++ {
		if err := r.webhook("evt-1", succeeded, "captured", ""); err != nil {
			t.Fatalf("webhook: %v", err)
		}
	}
	if got = r.stored(t, p.ID); got.Status != domain.PaymentStatusCaptured || r.lines(p.ID) != 2 {
		t.Fatalf("after the webhook: %s with %d lines", got.Status, r.lines(p.ID))
	}
}

func TestAWebhookThatCannotWriteTheLedgerIsProcessedAgain(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "pending")
	r.acct.failNext = 1
	if err := r.webhook("evt-1", succeeded, "captured", ""); err == nil {
		t.Fatal("a failed capture must be reported to the provider")
	}
	if r.stored(t, p.ID).IsCaptured() {
		t.Fatal("captured without accounting")
	}
	if err := r.webhook("evt-1", succeeded, "captured", ""); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if !r.stored(t, p.ID).IsCaptured() || r.lines(p.ID) != 2 {
		t.Fatalf("redelivery did not record the capture: %d lines", r.lines(p.ID))
	}
}

func TestACrashAfterTheCaptureIsRecoveredByReadingTheProvider(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "pending") // the process stopped before the capture was heard of
	r.adapter.setResponse(domain.PaymentProviderOpGet, domain.PaymentProviderResponse{Status: "captured", ProviderPaymentID: "ext-1"})
	got, err := r.svc.GetPaymentStatus(context.Background(), "org-test", "actor-1", p.ID, "")
	if err != nil || !got.IsCaptured() || r.lines(p.ID) != 2 {
		t.Fatalf("status read: %v %v lines %d", err, got, r.lines(p.ID))
	}
}

func TestConcurrentWebhooksAndStatusReadsPostOnce(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "pending")
	r.adapter.setResponse(domain.PaymentProviderOpGet, domain.PaymentProviderResponse{Status: "captured", ProviderPaymentID: "ext-1"})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			_ = r.webhook("evt-"+string(rune('a'+i%3)), succeeded, "captured", "")
		}(i)
		go func() {
			defer wg.Done()
			_, _ = r.svc.GetPaymentStatus(context.Background(), "org-test", "actor-1", p.ID, "")
		}()
	}
	wg.Wait()
	if !r.stored(t, p.ID).IsCaptured() || r.lines(p.ID) != 2 {
		t.Fatalf("%s with %d lines", r.stored(t, p.ID).Status, r.lines(p.ID))
	}
}

func TestAStateThatContradictsTheRecordIsKeptForReview(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "captured")
	if err := r.webhook("evt-fail", string(domain.WebhookEventPaymentFailed), "failed", ""); err != nil {
		t.Fatalf("a reviewed event is acknowledged: %v", err)
	}
	if got := r.stored(t, p.ID); got.Status != domain.PaymentStatusCaptured {
		t.Fatalf("a captured payment must not become %s", got.Status)
	}
	e, _ := r.events.FindByProviderMessageID(context.Background(), "mock-provider", "evt-fail")
	if e.Status != domain.PaymentWebhookStatusReview {
		t.Fatalf("event %s", e.Status)
	}
}

func TestACaptureReportedForACancelledPaymentIsKeptForReview(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "pending")
	stored := r.stored(t, p.ID)
	if err := stored.Cancel(unixNow()); err != nil {
		t.Fatal(err)
	}
	if err := r.payRepo.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	if err := r.webhook("evt-late", succeeded, "captured", ""); err != nil {
		t.Fatal(err)
	}
	e, _ := r.events.FindByProviderMessageID(context.Background(), "mock-provider", "evt-late")
	if e.Status != domain.PaymentWebhookStatusReview || r.stored(t, p.ID).Status != domain.PaymentStatusCancelled {
		t.Fatalf("event %s, payment %s", e.Status, r.stored(t, p.ID).Status)
	}
}

func refundOf(r *rig, key string, amount int64) (*domain.Refund, error) {
	_, refund, err := r.svc.RefundPayment(context.Background(), "org-test", "actor-1", r.anyPayment(), key, &domain.Money{AmountMinor: amount, Currency: "USD"})
	return refund, err
}

func (r *rig) anyPayment() string {
	r.payRepo.mu.RLock()
	defer r.payRepo.mu.RUnlock()
	for id := range r.payRepo.payments {
		return id
	}
	return ""
}

func TestARefundTheProviderDidNotAnswerStaysPendingAndResumes(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "captured")
	r.adapter.errors[domain.PaymentProviderOpRefund] = context.DeadlineExceeded
	refund, err := refundOf(r, "rk", 400)
	if !errors.Is(err, ErrRefundOutcomeUnknown) {
		t.Fatalf("a timeout is not a refusal: %v", err)
	}
	if refund.Status != domain.RefundStatusPending || r.stored(t, p.ID).RefundedAmount.AmountMinor != 0 || r.lines(refund.ID) != 0 {
		t.Fatalf("refund %s", refund.Status)
	}
	// A second refund cannot use what the pending one may already have returned.
	if _, err := refundOf(r, "other", 700); !errors.Is(err, domain.ErrRefundExceedsRemaining) {
		t.Fatalf("the pending refund is ignored: %v", err)
	}
	// The same request again (after a restart too): the provider answers, the refund is recorded once.
	delete(r.adapter.errors, domain.PaymentProviderOpRefund)
	r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "succeeded", ProviderPaymentID: "re-1"})
	again, err := refundOf(r, "rk", 400)
	if err != nil || again.ID != refund.ID || again.Status != domain.RefundStatusSucceeded {
		t.Fatalf("resume: %v %+v", err, again)
	}
	if got := r.stored(t, p.ID); got.RefundedAmount.AmountMinor != 400 || r.lines(refund.ID) != 2 || len(r.refunds.refunds) != 1 {
		t.Fatalf("refunded %d, lines %d, refunds %d", got.RefundedAmount.AmountMinor, r.lines(refund.ID), len(r.refunds.refunds))
	}
}

func TestARefundThatCannotBeRecordedIsCompletedByTheWebhook(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "captured")
	r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "succeeded", ProviderPaymentID: "re-1"})
	r.acct.failNext = 1
	refund, err := refundOf(r, "rk", 1000)
	if err == nil {
		t.Fatal("not recorded must not be reported as done")
	}
	if refund.Status != domain.RefundStatusPending || r.stored(t, p.ID).Status != domain.PaymentStatusCaptured || r.lines(refund.ID) != 0 {
		t.Fatalf("half done: refund %s, payment %s", refund.Status, r.stored(t, p.ID).Status)
	}
	// The webhook cannot write either the first time, then completes it.
	r.acct.failNext = 1
	if err := r.webhook("evt-r", string(domain.WebhookEventRefundSucceeded), "succeeded", "re-1"); err == nil {
		t.Fatal("expected a retry")
	}
	for i := 0; i < 2; i++ {
		if err := r.webhook("evt-r", string(domain.WebhookEventRefundSucceeded), "succeeded", "re-1"); err != nil {
			t.Fatalf("webhook: %v", err)
		}
	}
	if got := r.stored(t, p.ID); got.Status != domain.PaymentStatusRefunded || r.lines(refund.ID) != 2 {
		t.Fatalf("payment %s, lines %d", got.Status, r.lines(refund.ID))
	}
}

func TestAnUnknownRefundStatusIsNeverASuccess(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "captured")
	r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "pending", ProviderPaymentID: "re-1"})
	refund, err := refundOf(r, "rk", 1000)
	if err != nil || refund.Status != domain.RefundStatusPending {
		t.Fatalf("pending refund: %v", err)
	}
	if err := r.webhook("evt-odd", string(domain.WebhookEventRefundSucceeded), "reversed_maybe", "re-1"); err != nil {
		t.Fatal(err)
	}
	stored, _ := r.refunds.FindByID(context.Background(), "org-test", refund.ID)
	e, _ := r.events.FindByProviderMessageID(context.Background(), "mock-provider", "evt-odd")
	if stored.Status != domain.RefundStatusPending || r.stored(t, p.ID).Status != domain.PaymentStatusCaptured || e.Status != domain.PaymentWebhookStatusReview {
		t.Fatalf("refund %s, payment %s, event %s", stored.Status, r.stored(t, p.ID).Status, e.Status)
	}
}

func TestARefundTheProviderRefusesIsRecordedAsFailed(t *testing.T) {
	r := newRig(t)
	r.create(t, "captured")
	r.adapter.errors[domain.PaymentProviderOpRefund] = domain.ErrProviderDeclined
	refund, err := refundOf(r, "rk", 1000)
	if err == nil || refund.Status != domain.RefundStatusFailed {
		t.Fatalf("refused refund: %v %s", err, refund.Status)
	}
}

func TestPartialRefundsReturnNoMoreThanCaptured(t *testing.T) {
	r := newRig(t)
	p := r.create(t, "captured")
	r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "succeeded", ProviderPaymentID: "re-x"})
	var wg sync.WaitGroup
	for _, key := range []string{"a", "b", "c", "d"} {
		wg.Add(1)
		go func(key string) {
			defer wg.Done()
			r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "succeeded", ProviderPaymentID: "re-" + key})
			_, _ = refundOf(r, key, 400)
		}(key)
	}
	wg.Wait()
	if got := r.stored(t, p.ID); got.RefundedAmount.AmountMinor > 800 || got.RefundedAmount.AmountMinor == 0 {
		t.Fatalf("refunded %d of 1000 in steps of 400", got.RefundedAmount.AmountMinor)
	}
}

func TestAnOldUnansweredRefundIsNotSentAgain(t *testing.T) {
	r := newRig(t)
	r.create(t, "captured")
	r.adapter.errors[domain.PaymentProviderOpRefund] = context.DeadlineExceeded
	refund, _ := refundOf(r, "rk", 400)
	// A day later the provider may have forgotten the key: sending it again could return the money twice.
	r.refunds.mu.Lock()
	r.refunds.refunds[refund.ID].CreatedAt -= 25 * 60 * 60
	r.refunds.mu.Unlock()
	delete(r.adapter.errors, domain.PaymentProviderOpRefund)
	r.adapter.setResponse(domain.PaymentProviderOpRefund, domain.PaymentProviderResponse{Status: "succeeded", ProviderPaymentID: "re-late"})
	again, err := refundOf(r, "rk", 400)
	if !errors.Is(err, ErrRefundOutcomeUnknown) || again.Status != domain.RefundStatusPending || again.ProviderRefundID != "" {
		t.Fatalf("sent again: %v %+v", err, again)
	}
}
