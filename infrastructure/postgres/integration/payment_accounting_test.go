package integration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/payment"
	"github.com/trezutil/trezutil/internal/domain"
)

/* Financial integrity on real PostgreSQL: atomic accounting, reclaimable webhooks, migration 000038. */

func (e *testEnv) count(t *testing.T, q string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := e.db.DB.QueryRow(q, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func (e *testEnv) sql(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	if _, err := e.db.DB.ExecContext(e.ctx, q, args...); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
}

// breakLedger makes every ledger insert fail, as a database fault would; mendLedger removes the fault.
func (e *testEnv) breakLedger(t *testing.T) {
	e.sql(t, `CREATE OR REPLACE FUNCTION test_ledger_fault() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected ledger fault'; END $$`)
	e.sql(t, `CREATE TRIGGER test_ledger_fault BEFORE INSERT ON payment_ledger FOR EACH ROW EXECUTE FUNCTION test_ledger_fault()`)
}

func (e *testEnv) mendLedger(t *testing.T) {
	e.sql(t, `DROP TRIGGER test_ledger_fault ON payment_ledger`)
}

// authorized creates a payment the provider has authorized and not yet captured.
func (e *testEnv) authorized(t *testing.T, key string) *domain.Payment {
	t.Helper()
	p, err := e.svc.CreatePayment(e.ctx, e.orgID, "actor-1", e.orderID, domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-123", key, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p, err = e.svc.AuthorizePayment(e.ctx, e.orgID, "actor-1", p.ID, key+"-auth"); err != nil {
		t.Fatal(err)
	}
	return p
}

func (e *testEnv) captureEvent(id string) error {
	body := []byte(`{"event_type":"payment.succeeded","provider_code":"test-provider","provider_message_id":"` + id +
		`","provider_payment_id":"ext-payment-1","status":"captured"}`)
	return e.whHandler.ProcessWebhook(e.ctx, e.orgID, "test-provider", "sig-"+id, body)
}

func TestAccounting_ALedgerFaultLeavesThePaymentUncapturedAndRecoverable(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "fault")

	env.breakLedger(t)
	got, err := env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "fault-cap", nil)
	if err == nil && got.IsCaptured() {
		t.Fatalf("captured although the ledger cannot be written")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND status = 'captured'`, p.ID); n != 0 {
		t.Fatal("the database shows a capture without accounting")
	}
	if order, _ := env.orders.FindByIDAndOrg(env.ctx, env.orgID, env.orderID); order.Status != domain.OrderStatusPendingPayment {
		t.Fatalf("order %s before the capture is accounted", order.Status)
	}
	if err := env.captureEvent("evt-fault"); err == nil {
		t.Fatal("the webhook must ask to be sent again")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payment_webhook_events WHERE provider_msg_id = 'evt-fault' AND status = 'failed' AND attempts = 1`); n != 1 {
		t.Fatal("the failed event must stay stored and claimable")
	}

	env.mendLedger(t)
	if err := env.captureEvent("evt-fault"); err != nil {
		t.Fatalf("redelivery: %v", err)
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND status = 'captured'`, p.ID); n != 1 {
		t.Fatal("redelivery did not capture")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID); n != 2 {
		t.Fatalf("%d ledger lines", n)
	}
	if order, _ := env.orders.FindByIDAndOrg(env.ctx, env.orgID, env.orderID); order.Status != domain.OrderStatusPaid {
		t.Fatalf("order %s", order.Status)
	}
}

func TestAccounting_ARefundTheLedgerCannotTakeStaysPendingThenCompletes(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "rf")
	if _, err := env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "rf-cap", nil); err != nil {
		t.Fatal(err)
	}
	amount := domain.Money{AmountMinor: 400, Currency: "USD"}

	env.breakLedger(t)
	if _, _, err := env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "rf-1", &amount); err == nil {
		t.Fatal("an unrecorded refund must not be reported as done")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM refunds WHERE payment_id = $1 AND status = 'pending' AND provider_refund_id IS NOT NULL`, p.ID); n != 1 {
		t.Fatal("the refund and the provider's reference must be kept")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND refunded_amount_minor = 0 AND status = 'captured'`, p.ID); n != 1 {
		t.Fatal("the payment changed although the refund was not recorded")
	}

	env.mendLedger(t)
	body := []byte(`{"event_type":"refund.succeeded","provider_code":"test-provider","provider_message_id":"evt-rf","provider_payment_id":"ext-payment-1","provider_refund_id":"ext-refund-1","status":"succeeded"}`)
	for i := 0; i < 2; i++ {
		if err := env.whHandler.ProcessWebhook(env.ctx, env.orgID, "test-provider", "sig-rf", body); err != nil {
			t.Fatalf("webhook: %v", err)
		}
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND refunded_amount_minor = 400 AND status = 'partially_refunded'`, p.ID); n != 1 {
		t.Fatal("refund not applied")
	}
	if n := env.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE refund_id IS NOT NULL`); n != 2 {
		t.Fatalf("%d refund lines", n)
	}
}

func TestAccounting_CommitIsAllOrNothingAndNeverDoublesAPosting(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "aon")
	acct := postgres.NewPaymentAccounting(env.db.DB)
	now := time.Now().Unix()

	// The second line is refused by the database: neither the payment nor the first line may remain.
	changed := *p
	if err := changed.Capture("", now, now); err != nil {
		t.Fatal(err)
	}
	entries, err := domain.CaptureEntries(&changed, now)
	if err != nil {
		t.Fatal(err)
	}
	entries[1].Account = "no-such-account"
	if err := acct.CommitCapture(env.ctx, &changed, entries); err == nil {
		t.Fatal("expected a refusal")
	}
	if env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND status = 'authorized' AND version = $2`, p.ID, p.Version) != 1 ||
		env.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID) != 0 {
		t.Fatal("half a commit was kept")
	}

	// Lines written by the earlier webhook code used other keys: they are recognised, not doubled.
	for _, l := range [][2]string{{"cash", "DEBIT"}, {"revenue", "CREDIT"}} {
		env.sql(t, `INSERT INTO payment_ledger (organization_id, payment_id, account, entry_type, amount_minor, currency, idempotency_key, status, posted_at)
			VALUES ($1, $2, $3, $4, 1000, 'USD', $5, 'posted', NOW())`, env.orgID, p.ID, l[0], l[1], "ledger_webhook_"+l[0]+"_"+p.ID)
	}
	entries, _ = domain.CaptureEntries(&changed, now)
	posted, err := acct.EnsureEntries(env.ctx, entries)
	if err != nil || posted != 0 {
		t.Fatalf("posted %d again: %v", posted, err)
	}
	// And the database itself refuses a duplicate posting.
	if _, err := env.db.DB.Exec(`INSERT INTO payment_ledger (organization_id, payment_id, account, entry_type, amount_minor, currency, idempotency_key, status, posted_at)
		VALUES ($1, $2, 'cash', 'DEBIT', 1000, 'USD', 'another-key', 'posted', NOW())`, env.orgID, p.ID); err == nil {
		t.Fatal("the unique posting index is missing")
	}
}

func TestAccounting_WebhookEventsAreClaimedOnceAndReclaimedOnlyWhenAbandoned(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	ctx := context.Background()
	store := postgres.NewPaymentWebhookEventStore(env.db.DB)
	if err := store.PersistEvent(ctx, &domain.PaymentWebhookEvent{ProviderCode: "test-provider", ProviderMsgID: "m1", TenantID: env.orgID,
		EventType: domain.WebhookEventPaymentSucceeded, Status: domain.PaymentWebhookStatusPending, Payload: []byte(`{}`)}); err != nil {
		t.Fatal(err)
	}
	claim := func(want bool, why string) {
		t.Helper()
		got, err := store.TryClaim(ctx, "test-provider", "m1")
		if err != nil || got != want {
			t.Fatalf("%s: claimed %v %v", why, got, err)
		}
	}
	claim(true, "first delivery")
	claim(false, "while processing")
	env.sql(t, `UPDATE payment_webhook_events SET claimed_at = NOW() - INTERVAL '1 hour' WHERE provider_msg_id = 'm1'`)
	claim(true, "abandoned processing (crash)")
	if err := store.MarkFailed(ctx, "test-provider", "m1", "db down"); err != nil {
		t.Fatal(err)
	}
	claim(true, "after a failure")
	if err := store.MarkReview(ctx, "test-provider", "m1", "contradiction"); err != nil {
		t.Fatal(err)
	}
	claim(false, "kept for review")
	e, err := store.FindByProviderMessageID(ctx, "test-provider", "m1")
	if err != nil || e.Status != domain.PaymentWebhookStatusReview || e.ErrorMessage != "contradiction" {
		t.Fatalf("%v %+v", err, e)
	}
	if env.count(t, `SELECT attempts FROM payment_webhook_events WHERE provider_msg_id = 'm1'`) != 3 {
		t.Fatal("attempts not counted")
	}
}

// Another instance (or the same one after a restart) sees the refund still pending: no second refund of the same money.
func TestAccounting_APendingRefundSurvivesARestart(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "rs")
	if _, err := env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "rs-cap", nil); err != nil {
		t.Fatal(err)
	}
	amount := domain.Money{AmountMinor: 1000, Currency: "USD"}
	env.adapter.refundResp = domain.RefundProviderResponse{Status: "pending", Amount: amount, ProviderRefundID: "ext-refund-rs"}
	_, first, err := env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "rs-1", &amount)
	if err != nil || first.Status != domain.RefundStatusPending {
		t.Fatalf("pending refund: %v", err)
	}
	restarted := payment.NewPaymentService(payment.PaymentServiceConfig{
		Payments: postgres.NewPaymentRepository(env.db.DB), Refunds: postgres.NewRefundRepository(env.db.DB),
		Accounting: postgres.NewPaymentAccounting(env.db.DB), Orders: env.orders, Logger: zerolog.Nop(),
		Locker: postgres.NewAdvisoryLocker(env.db.DB),
	})
	if _, again, err := restarted.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "rs-1", &amount); again != nil && again.ID != first.ID {
		t.Fatalf("a second refund was created: %+v %v", again, err)
	}
	if _, _, err := restarted.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "rs-2", &amount); err == nil {
		t.Fatal("a second refund of the same money must be refused")
	}
	if env.count(t, `SELECT COUNT(*) FROM refunds WHERE payment_id = $1`, p.ID) != 1 {
		t.Fatal("more than one refund row")
	}
}

func TestMigration000038_UpOnDuplicateDataKeepsEverythingAndDownIsClean(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "mig")
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "000038_financial_integrity.sql"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	index := `SELECT COUNT(*) FROM pg_indexes WHERE indexname = 'uq_payment_ledger_posting'`
	env.sql(t, content[strings.Index(content, "-- +goose Down"):])
	if env.count(t, index) != 0 || env.count(t, `SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'payment_webhook_events' AND column_name IN ('claimed_at', 'attempts')`) != 0 {
		t.Fatal("down incomplete")
	}
	for _, key := range []string{"old-a", "old-b"} {
		env.sql(t, `INSERT INTO payment_ledger (organization_id, payment_id, account, entry_type, amount_minor, currency, idempotency_key, status, posted_at)
			VALUES ($1, $2, 'cash', 'DEBIT', 1000, 'USD', $3, 'posted', NOW())`, env.orgID, p.ID, key)
	}
	env.sql(t, extractUpSection(content))
	if env.count(t, index) != 0 || env.count(t, `SELECT COUNT(*) FROM payment_ledger WHERE payment_id = $1`, p.ID) != 2 {
		t.Fatal("duplicates must be left for a person, without the index")
	}
	env.sql(t, `DELETE FROM payment_ledger WHERE idempotency_key = 'old-b'`)
	env.sql(t, extractUpSection(content)) // running it again once corrected creates the guard
	if env.count(t, index) != 1 {
		t.Fatal("index not created after correction")
	}
}

func TestReconciliation_FindsDisagreementsAndRepairsOnlyMissingLines(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()
	p := env.authorized(t, "rec")
	if _, err := env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "rec-cap", nil); err != nil {
		t.Fatal(err)
	}
	amount := domain.Money{AmountMinor: 400, Currency: "USD"}
	if _, _, err := env.svc.RefundPayment(env.ctx, env.orgID, "actor-1", p.ID, "rec-r", &amount); err != nil {
		t.Fatal(err)
	}
	reconciler := &payment.Reconciler{
		Source: postgres.NewPaymentReconciliation(env.db.DB), Payments: postgres.NewPaymentRepository(env.db.DB),
		Refunds: postgres.NewRefundRepository(env.db.DB), Accounting: postgres.NewPaymentAccounting(env.db.DB),
		Locker: postgres.NewAdvisoryLocker(env.db.DB), Audit: postgres.NewAuditRepository(env.db.DB), Logger: zerolog.Nop(),
	}
	kinds := func() map[domain.ReconciliationKind]int {
		t.Helper()
		findings, err := postgres.NewPaymentReconciliation(env.db.DB).Findings(env.ctx, time.Now().Add(time.Hour).Unix())
		if err != nil {
			t.Fatal(err)
		}
		out := map[domain.ReconciliationKind]int{}
		for _, f := range findings {
			out[f.Kind]++
		}
		return out
	}
	if got := kinds(); len(got) != 0 {
		t.Fatalf("a consistent database reports %v", got)
	}

	// What the earlier code could leave behind: a capture without its lines, a refund with one side only.
	env.sql(t, `DELETE FROM payment_ledger WHERE payment_id = $1`, p.ID)
	env.sql(t, `DELETE FROM payment_ledger WHERE refund_id IS NOT NULL AND account = 'cash'`)
	env.sql(t, `INSERT INTO payment_webhook_events (provider_code, provider_msg_id, organization_id, event_type, status, error_message)
		VALUES ('test-provider', 'stuck', $1, 'payment.succeeded', 'review', 'contradiction')`, env.orgID)
	env.sql(t, `UPDATE payment_webhook_events SET created_at = NOW() - INTERVAL '2 hours'`)
	got := kinds()
	if got[domain.ReconCaptureWithoutLedger] != 1 || got[domain.ReconRefundWithoutLedger] != 1 || got[domain.ReconWebhookUnprocessed] != 1 {
		t.Fatalf("findings %v", got)
	}

	if _, err := reconciler.RepairLedger(env.ctx, "", true); err == nil {
		t.Fatal("an anonymous repair must be refused")
	}
	dry, err := reconciler.RepairLedger(env.ctx, "operator@test", false)
	if err != nil || len(dry) != 2 || env.count(t, `SELECT COUNT(*) FROM payment_ledger`) != 1 {
		t.Fatalf("a dry run wrote something: %v %d", err, len(dry))
	}
	for i := 0; i < 2; i++ { // repairing twice adds nothing more
		repairs, err := reconciler.RepairLedger(env.ctx, "operator@test", true)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range repairs {
			if r.Error != "" {
				t.Fatal(r.Error)
			}
		}
	}
	if env.count(t, `SELECT COUNT(*) FROM payment_ledger`) != 4 {
		t.Fatalf("%d lines", env.count(t, `SELECT COUNT(*) FROM payment_ledger`))
	}
	if env.count(t, `SELECT COUNT(*) FROM audit_events WHERE action = 'payment.ledger.repaired' AND actor_id = 'operator@test'`) != 2 {
		t.Fatal("repairs not audited")
	}
	if env.count(t, `SELECT COUNT(*) FROM payments WHERE id = $1 AND status = 'partially_refunded' AND refunded_amount_minor = 400`, p.ID) != 1 {
		t.Fatal("a repair must not change the payment")
	}
	env.sql(t, `UPDATE payments SET refunded_amount_minor = 900 WHERE id = $1`, p.ID)
	if got := kinds(); got[domain.ReconRefundedTotal] != 1 || got[domain.ReconCaptureWithoutLedger] != 0 {
		t.Fatalf("findings %v", got)
	}
}
