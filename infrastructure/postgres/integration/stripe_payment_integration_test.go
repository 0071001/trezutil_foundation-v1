package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/application/provider"
	"github.com/trezutil/trezutil/internal/domain"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe"
	"github.com/trezutil/trezutil/internal/infrastructure/payment/stripe/stripefake"
)

// These tests run the production payment wiring against real PostgreSQL:
// providers row (secret_ref) -> stripe.Loader -> ProviderOrchestrator ->
// Stripe adapter over HTTP -> Stripe API emulator -> Payment Core -> ledger ->
// order, plus Stripe-signed webhooks through the WebhookHandler. The emulator
// stands in for api.stripe.com; no live Stripe credentials are used.

const (
	stripeTestKey   = "sk_test_integration"
	stripeTestWhsec = "whsec_integration"
)

type stripeEnv struct {
	db       *TestDB
	ctx      context.Context
	svc      *payment.PaymentService
	webhooks *payment.WebhookHandler
	fake     *stripefake.Server
	orgID    string
	sellerID string
	customer string
	storeID  string
	orders   *postgres.OrderRepository
}

func setupStripe(t *testing.T, captureMethod string, adapterTimeout time.Duration) *stripeEnv {
	t.Helper()
	db := NewTestDB(t)
	ctx := context.Background()
	fake := stripefake.New(stripeTestKey)
	t.Cleanup(fake.Close)
	t.Setenv("STRIPE_IT_SECRET_KEY", stripeTestKey)
	t.Setenv("STRIPE_IT_WEBHOOK_SECRET", stripeTestWhsec)

	env := &stripeEnv{db: db, ctx: ctx, fake: fake, orgID: testUUID(), sellerID: testUUID(), customer: testUUID(), storeID: testUUID(), orders: postgres.NewOrderRepository(db.DB)}
	createTestOrganization(t, db, ctx, env.orgID)
	createTestUser(t, db, ctx, env.customer)
	createTestSeller(t, db, ctx, env.sellerID, env.customer, env.orgID)
	createTestStore(t, db, ctx, env.storeID, env.orgID)

	providerRepo := postgres.NewProviderRepository(db.DB)
	settings := map[string]string{
		"base_url":           fake.URL,
		"capture_method":     captureMethod,
		"publishable_key":    "pk_test_integration",
		"webhook_secret_ref": "env:STRIPE_IT_WEBHOOK_SECRET",
	}
	if err := providerRepo.Create(ctx, &domain.IntegrationProvider{
		ID: testUUID(), OrganizationID: env.orgID, Code: stripe.ProviderCode, Name: "Stripe", Type: "PAYMENT",
		Status: domain.ProviderStatusActive, Capabilities: []domain.Capability{domain.CapabilityPayment},
		Configuration: domain.ProviderConfig{Settings: settings}, SecretRef: "env:STRIPE_IT_SECRET_KEY",
		HealthStatus: domain.ProviderHealthHealthy, Version: 1, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	registry := provider.NewProviderRegistry()
	loader := &stripe.Loader{Providers: providerRepo, Registry: registry, Logger: zerolog.Nop()}
	if n, err := loader.Load(ctx); err != nil || n != 1 {
		t.Fatalf("stripe loader registered %d providers: %v", n, err)
	}
	if adapterTimeout > 0 {
		// Re-register with a short client timeout for timeout scenarios.
		registry = provider.NewProviderRegistry()
		rows, _ := providerRepo.FindAllByCode(ctx, stripe.ProviderCode)
		a, _ := stripe.NewAdapter(stripe.Config{ProviderID: rows[0].ID, SecretKey: stripeTestKey, BaseURL: fake.URL, CaptureMethod: captureMethod, Timeout: adapterTimeout})
		if err := registry.Register(rows[0], a); err != nil {
			t.Fatal(err)
		}
	}

	orchestrator := provider.NewProviderOrchestrator(registry, postgres.NewProviderExecutionRepository(db.DB), provider.OrchestratorConfig{Logger: zerolog.Nop()})
	paymentRepo := postgres.NewPaymentRepository(db.DB)
	refundRepo := postgres.NewRefundRepository(db.DB)
	ledgerRepo := postgres.NewLedgerRepository(db.DB)
	env.svc = payment.NewPaymentService(payment.PaymentServiceConfig{
		Payments: paymentRepo, Refunds: refundRepo, Ledger: ledgerRepo, Orders: env.orders,
		Orchestrator: orchestrator, Logger: zerolog.Nop(), Locker: postgres.NewAdvisoryLocker(db.DB),
	})
	env.webhooks = payment.NewWebhookHandler(payment.WebhookHandlerConfig{
		Payments: paymentRepo, Refunds: refundRepo, Ledger: ledgerRepo,
		Events: postgres.NewPaymentWebhookEventStore(db.DB), Providers: providerRepo, Orders: env.orders,
		Normalizers: map[string]payment.WebhookNormalizer{
			stripe.ProviderCode: &stripe.WebhookNormalizer{SecretFor: loader.WebhookSecret},
		},
		Logger: zerolog.Nop(),
	})
	return env
}

func (e *stripeEnv) newOrder(t *testing.T, amount int64, currency string) string {
	t.Helper()
	id := testUUID()
	if err := e.orders.Create(e.ctx, &domain.Order{
		ID: id, OrganizationID: e.orgID, SellerID: e.sellerID, CustomerID: e.customer, StoreID: e.storeID,
		Items: []*domain.OrderItem{}, TotalAmount: domain.Money{AmountMinor: amount, Currency: currency}, Currency: currency,
		Status: domain.OrderStatusDraft, Version: 1, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

func (e *stripeEnv) orderStatus(t *testing.T, id string) domain.OrderStatus {
	t.Helper()
	o, err := e.orders.FindByIDAndOrg(e.ctx, e.orgID, id)
	if err != nil {
		t.Fatal(err)
	}
	return o.Status
}

func (e *stripeEnv) count(t *testing.T, q string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := e.db.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *stripeEnv) webhook(eventID, typ string, object map[string]interface{}) error {
	body := stripefake.Event(eventID, typ, object)
	return e.webhooks.ProcessWebhook(e.ctx, e.orgID, stripe.ProviderCode, stripe.Sign(stripeTestWhsec, time.Now(), body), body)
}

func usdMoney(n int64) domain.Money { return domain.Money{AmountMinor: n, Currency: "USD"} }

// Order -> Create Payment -> Stripe (confirm + automatic capture) -> Ledger ->
// Order PAID; duplicate request and webhook replay change nothing.
func TestStripe_AutomaticCapture_LedgerOrderAndReplays(t *testing.T) {
	e := setupStripe(t, "automatic", 0)
	order := e.newOrder(t, 1999, "USD")

	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1999), domain.PaymentMethodCard, "pm_card_visa", "checkout-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentStatusCaptured || p.ProviderPaymentID == "" {
		t.Fatalf("expected captured payment with intent id, got %s %q", p.Status, p.ProviderPaymentID)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPaid {
		t.Fatalf("order must be PAID after capture, got %s", s)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 2 {
		t.Fatalf("expected 2 capture ledger entries, got %d", n)
	}

	// Duplicate request: same idempotency key -> same payment, no new intent.
	again, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1999), domain.PaymentMethodCard, "pm_card_visa", "checkout-1", nil)
	if err != nil || again.ID != p.ID {
		t.Fatalf("duplicate request must return the same payment: %v", err)
	}
	if n := e.fake.CallCount("POST /v1/payment_intents"); n != 1 {
		t.Fatalf("duplicate request must not reach Stripe again, got %d creates", n)
	}

	// The provider's own event arrives afterwards, twice (at-least-once delivery).
	obj := map[string]interface{}{"id": p.ProviderPaymentID, "object": "payment_intent", "status": "succeeded", "amount": 1999, "currency": "usd"}
	for i := 0; i < 2; i++ {
		if err := e.webhook("evt_auto_1", "payment_intent.succeeded", obj); err != nil {
			t.Fatalf("webhook delivery %d: %v", i, err)
		}
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 2 {
		t.Fatalf("webhook replay must not post ledger entries again, got %d", n)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_webhook_events WHERE provider_msg_id = 'evt_auto_1'`); n != 1 {
		t.Fatalf("expected one stored webhook event, got %d", n)
	}
}

// Authorization -> Capture (manual) -> capture replay moves no extra money.
func TestStripe_ManualCapture_AndCaptureReplay(t *testing.T) {
	e := setupStripe(t, "manual", 0)
	order := e.newOrder(t, 5000, "USD")
	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(5000), domain.PaymentMethodCard, "pm_card_visa", "checkout-m", nil)
	if err != nil || p.Status != domain.PaymentStatusAuthorized {
		t.Fatalf("expected authorized, got %v %v", p, err)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPendingPayment {
		t.Fatalf("authorized (not captured) payment must leave the order pending_payment, got %s", s)
	}
	p, err = e.svc.CapturePayment(e.ctx, e.orgID, "staff", p.ID, "capture-1", nil)
	if err != nil || p.Status != domain.PaymentStatusCaptured {
		t.Fatalf("capture: %v %v", p, err)
	}
	if _, err := e.svc.CapturePayment(e.ctx, e.orgID, "staff", p.ID, "capture-1", nil); err == nil {
		t.Fatal("capture replay on a captured payment must be refused")
	}
	if n := e.fake.CallCount("POST /v1/payment_intents/"); n != 1 {
		t.Fatalf("exactly one capture must reach Stripe, got %d", n)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPaid {
		t.Fatalf("order must be PAID after capture, got %s", s)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", n)
	}
}

// Declined card -> payment FAILED, order stays payable, a new attempt works.
func TestStripe_DeclineThenRetryWithNewCard(t *testing.T) {
	e := setupStripe(t, "automatic", 0)
	order := e.newOrder(t, 700, "USD")
	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(700), domain.PaymentMethodCard, "pm_card_chargeDeclined", "try-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentStatusFailed {
		t.Fatalf("declined payment must be FAILED, got %s", p.Status)
	}
	if n := e.fake.CallCount("POST /v1/payment_intents"); n != 1 {
		t.Fatalf("a decline must not be retried, got %d creates", n)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPendingPayment {
		t.Fatalf("order must remain payable, got %s", s)
	}
	p2, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(700), domain.PaymentMethodCard, "pm_card_visa", "try-2", nil)
	if err != nil || p2.Status != domain.PaymentStatusCaptured {
		t.Fatalf("retry with a valid card must succeed: %v %v", p2, err)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 0 {
		t.Fatalf("a failed payment must not post ledger entries, got %d", n)
	}
}

// Browser confirmation: intent awaits Stripe.js (client action), the payer
// confirms, the signed webhook completes payment and order.
func TestStripe_BrowserConfirmationViaWebhook(t *testing.T) {
	e := setupStripe(t, "automatic", 0)
	order := e.newOrder(t, 1500, "USD")
	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1500), domain.PaymentMethodCard, "card", "browser-1", nil)
	if err != nil || p.Status != domain.PaymentStatusPending {
		t.Fatalf("expected pending payment, got %v %v", p, err)
	}
	if p.ClientAction["client_secret"] == "" || p.ClientAction["publishable_key"] != "pk_test_integration" {
		t.Fatalf("expected client action for Stripe.js, got %v", p.ClientAction)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND metadata::text LIKE '%secret%'`, p.ID); n != 0 {
		t.Fatal("client secret must never be persisted")
	}
	// Replay returns a fresh client action for the same intent.
	again, _ := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1500), domain.PaymentMethodCard, "card", "browser-1", nil)
	if again.ID != p.ID || again.ClientAction["client_secret"] == "" {
		t.Fatalf("replay must return the same payment with its client action: %+v", again)
	}

	e.fake.Confirm(p.ProviderPaymentID)
	if err := e.webhook("evt_browser_1", "payment_intent.succeeded", map[string]interface{}{"id": p.ProviderPaymentID, "object": "payment_intent", "status": "succeeded", "amount": 1500, "currency": "usd"}); err != nil {
		t.Fatal(err)
	}
	got, _ := e.svc.GetPayment(e.ctx, e.orgID, p.ID)
	if got.Status != domain.PaymentStatusCaptured {
		t.Fatalf("webhook must capture the payment, got %s", got.Status)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPaid {
		t.Fatalf("order must be PAID, got %s", s)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 2 {
		t.Fatalf("expected 2 ledger entries, got %d", n)
	}
}

// Refund, refund replay, over-refund, asynchronous refund completed by webhook.
func TestStripe_RefundsReplayOverRefundAndAsync(t *testing.T) {
	e := setupStripe(t, "automatic", 0)
	order := e.newOrder(t, 3000, "USD")
	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(3000), domain.PaymentMethodCard, "pm_card_visa", "refund-order", nil)
	if err != nil || p.Status != domain.PaymentStatusCaptured {
		t.Fatalf("setup capture failed: %v %v", p, err)
	}

	half := usdMoney(1000)
	p, _, err = e.svc.RefundPayment(e.ctx, e.orgID, "staff", p.ID, "refund-a", &half)
	if err != nil || p.RefundedAmount.AmountMinor != 1000 {
		t.Fatalf("partial refund: %v %v", p, err)
	}
	if _, _, err := e.svc.RefundPayment(e.ctx, e.orgID, "staff", p.ID, "refund-a", &half); !errors.Is(err, domain.ErrRefundAlreadyExists) {
		t.Fatalf("refund replay must be detected, got %v", err)
	}
	tooMuch := usdMoney(2500)
	if _, _, err := e.svc.RefundPayment(e.ctx, e.orgID, "staff", p.ID, "refund-b", &tooMuch); err == nil {
		t.Fatal("over-refund must be rejected")
	}
	if n := e.fake.CallCount("POST /v1/refunds"); n != 1 {
		t.Fatalf("only one refund may reach Stripe, got %d", n)
	}

	// Asynchronous refund: pending at request time, settled by webhook.
	e.fake.PendingRefunds(true)
	rest := usdMoney(2000)
	p, refund, err := e.svc.RefundPayment(e.ctx, e.orgID, "staff", p.ID, "refund-c", &rest)
	if err != nil {
		t.Fatalf("async refund request: %v", err)
	}
	if refund.Status != domain.RefundStatusPending || p.RefundedAmount.AmountMinor != 1000 {
		t.Fatalf("pending refund must not change the refunded amount yet: %s %d", refund.Status, p.RefundedAmount.AmountMinor)
	}
	settled, _ := e.fake.SettleRefund(refund.ProviderRefundID)
	obj := map[string]interface{}{"id": settled.ID, "object": "refund", "status": "succeeded", "amount": settled.Amount, "currency": "usd", "payment_intent": p.ProviderPaymentID}
	for i := 0; i < 2; i++ { // at-least-once delivery
		if err := e.webhook("evt_refund_c", "refund.updated", obj); err != nil {
			t.Fatalf("refund webhook %d: %v", i, err)
		}
	}
	got, _ := e.svc.GetPayment(e.ctx, e.orgID, p.ID)
	if got.RefundedAmount.AmountMinor != 3000 || got.Status != domain.PaymentStatusRefunded {
		t.Fatalf("expected fully refunded payment, got %s %d", got.Status, got.RefundedAmount.AmountMinor)
	}
	if n := e.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE refund_id IS NOT NULL`); n != 2 {
		t.Fatalf("expected exactly one ledger entry per refund (2), got %d", n)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusRefunded {
		t.Fatalf("fully refunded order must be REFUNDED, got %s", s)
	}
}

// Webhook signature failure, wrong tenant, wrong amount/currency.
func TestStripe_SignatureTenantAndAmountGuards(t *testing.T) {
	e := setupStripe(t, "automatic", 0)
	body := stripefake.Event("evt_forged", "payment_intent.succeeded", map[string]interface{}{"id": "pi_1", "object": "payment_intent", "status": "succeeded"})
	if err := e.webhooks.ProcessWebhook(e.ctx, e.orgID, stripe.ProviderCode, stripe.Sign("whsec_attacker", time.Now(), body), body); !errors.Is(err, domain.ErrWebhookSignatureInvalid) {
		t.Fatalf("forged webhook must be rejected, got %v", err)
	}

	order := e.newOrder(t, 1200, "USD")
	if _, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1100), domain.PaymentMethodCard, "pm_card_visa", "amt", nil); err == nil {
		t.Fatal("wrong amount must be rejected")
	}
	if _, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, domain.Money{AmountMinor: 1200, Currency: "EUR"}, domain.PaymentMethodCard, "pm_card_visa", "cur", nil); err == nil {
		t.Fatal("wrong currency must be rejected")
	}
	if n := e.fake.CallCount("POST /v1/payment_intents"); n != 0 {
		t.Fatalf("invalid requests must never reach Stripe, got %d", n)
	}

	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(1200), domain.PaymentMethodCard, "pm_card_visa", "ok", nil)
	if err != nil {
		t.Fatal(err)
	}
	other := testUUID()
	createTestOrganization(t, e.db, e.ctx, other)
	if _, err := e.svc.GetPayment(e.ctx, other, p.ID); err == nil {
		t.Fatal("payment must not be visible from another tenant")
	}
	amt := usdMoney(100)
	if _, _, err := e.svc.RefundPayment(e.ctx, other, "staff", p.ID, "x-tenant", &amt); err == nil {
		t.Fatal("cross-tenant refund must fail")
	}
}

// Provider timeout: the payment stays CREATED (never double-charged); a
// replay with the same key resumes the same intent once Stripe responds.
func TestStripe_TimeoutThenResume(t *testing.T) {
	e := setupStripe(t, "automatic", 300*time.Millisecond)
	e.fake.Delay = 700 * time.Millisecond
	order := e.newOrder(t, 900, "USD")

	p, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(900), domain.PaymentMethodCard, "pm_timeout", "slow-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.PaymentStatusCreated || p.ProviderPaymentID != "" {
		t.Fatalf("timed-out payment must stay CREATED without provider id, got %s %q", p.Status, p.ProviderPaymentID)
	}
	time.Sleep(1200 * time.Millisecond) // let Stripe finish the first request

	e.fake.Delay = 0
	resumed, err := e.svc.CreatePayment(e.ctx, e.orgID, e.customer, order, usdMoney(900), domain.PaymentMethodCard, "pm_timeout", "slow-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != p.ID || resumed.Status != domain.PaymentStatusCaptured {
		t.Fatalf("replay must resume the same payment to captured, got %s %s", resumed.ID, resumed.Status)
	}
	var intents int
	for _, id := range []string{"pi_1", "pi_2", "pi_3", "pi_4"} {
		if _, ok := e.fake.Intent(id); ok {
			intents++
		}
	}
	if intents != 1 {
		t.Fatalf("timeouts and retries must create exactly one PaymentIntent, got %d", intents)
	}
	if s := e.orderStatus(t, order); s != domain.OrderStatusPaid {
		t.Fatalf("order must be PAID, got %s", s)
	}
}
