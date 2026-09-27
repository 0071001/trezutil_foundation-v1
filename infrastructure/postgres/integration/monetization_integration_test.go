package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

func insertTestPlan(t *testing.T, db *TestDB, ctx context.Context, id, tenantID, slug string, priceMinor, quotaAmount, creditPack int64) {
	t.Helper()
	const q = `
		INSERT INTO plans (id, tenant_id, name, slug, price_minor, currency, billing_cycle, quota_amount, credit_pack, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'active', NOW(), NOW())
	`
	_, err := db.DB.ExecContext(ctx, q, id, tenantID, slug+" plan", slug, priceMinor, "USD", "monthly", quotaAmount, creditPack)
	if err != nil {
		t.Fatalf("failed to insert plan: %v", err)
	}
}

func nowMonth() string {
	return time.Now().UTC().Format("2006-01")
}

func countRows(t *testing.T, ctx context.Context, db *TestDB, q string, args ...any) int {
	t.Helper()
	var n int
	if err := db.DB.QueryRowContext(ctx, q, args...).Scan(&n); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	return n
}

// TestCreditGrant_Idempotent verifies the atomic GrantCreditsTx primitive: a duplicate
// call (same idempotency key) does NOT add a second grant row, ledger row, or bump the
// balance twice. This is the invariant-critical path (invariants 9 & 10).
func TestCreditGrant_Idempotent(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	orgID := testUUID()
	createTestOrganization(t, db, ctx, orgID)

	accountID := testUUID()
	ownerID := testUUID()
	periodID := nowMonth()
	_, err := db.DB.ExecContext(ctx, `
		INSERT INTO credit_accounts (account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed, created_at, updated_at)
		VALUES ($1, 'customer', $2, $3, '', $4, 0, 0, 0, NOW(), NOW())
	`, accountID, ownerID, orgID, periodID)
	if err != nil {
		t.Fatalf("failed to seed credit account: %v", err)
	}

	granter := postgres.NewCreditAccountRepository(db.DB)
	key := "test-grant-1"

	granted, balance, err := granter.GrantCreditsTx(ctx, accountID, key, domain.CreditGrantSourceAllocation, 5000, orgID, periodID, nil, "allocation")
	if err != nil {
		t.Fatalf("first grant failed: %v", err)
	}
	if !granted {
		t.Fatal("expected first grant to be applied")
	}
	if balance != 5000 {
		t.Errorf("expected balance 5000, got %d", balance)
	}

	// Idempotent replay: must be a no-op.
	granted2, balance2, err := granter.GrantCreditsTx(ctx, accountID, key, domain.CreditGrantSourceAllocation, 5000, orgID, periodID, nil, "allocation")
	if err != nil {
		t.Fatalf("replay grant failed: %v", err)
	}
	if granted2 {
		t.Error("expected replay to be a no-op (granted=false)")
	}
	if balance2 != 5000 {
		t.Errorf("expected balance unchanged at 5000, got %d", balance2)
	}

	if n := countRows(t, ctx, db, `SELECT COUNT(*) FROM credit_grants WHERE idempotency_key = $1`, key); n != 1 {
		t.Errorf("expected 1 credit_grants row, got %d", n)
	}
	if n := countRows(t, ctx, db, `SELECT COUNT(*) FROM usage_ledger WHERE account_id = $1 AND transaction_type = 'GRANT'`, accountID); n != 1 {
		t.Errorf("expected 1 ledger GRANT row, got %d", n)
	}
	var grantedCol int64
	if err := db.DB.QueryRowContext(ctx, `SELECT granted FROM credit_accounts WHERE account_id = $1`, accountID).Scan(&grantedCol); err != nil {
		t.Fatalf("failed to read balance: %v", err)
	}
	if grantedCol != 5000 {
		t.Errorf("expected credit_accounts.granted=5000, got %d", grantedCol)
	}
}

// TestSubscription_RenewalAllocatesCredits verifies the subscription flow: Create →
// RenewPeriod opens a billing period and grants the plan's free-tier allocation exactly
// once (idempotent), using a UUID-generated credit account (F1 fixed on this path).
func TestSubscription_RenewalAllocatesCredits(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	orgID := testUUID()
	createTestOrganization(t, db, ctx, orgID)

	planID := testUUID()
	insertTestPlan(t, db, ctx, planID, orgID, "team-free", 0, 1000, 0)

	planRepo := postgres.NewPlanRepository(db.DB)
	subRepo := postgres.NewSubscriptionRepository(db.DB)
	periodRepo := postgres.NewBillingPeriodRepository(db.DB)
	accounts := postgres.NewCreditAccountRepository(db.DB)

	subSvc := application.NewSubscriptionService(application.SubscriptionServiceConfig{
		Plans:    planRepo,
		Subs:     subRepo,
		Periods:  periodRepo,
		Accounts: accounts,
		Grant:    accounts,
		Logger:   zerolog.Nop(),
	})

	ownerID := testUUID()
	sub, period, err := subSvc.CreateSubscription(ctx, orgID, planID, "customer", ownerID)
	if err != nil {
		t.Fatalf("CreateSubscription failed: %v", err)
	}
	if sub.ID == "" {
		t.Fatal("expected subscription ID to be assigned")
	}
	if sub.Status != domain.SubscriptionStatusActive {
		t.Errorf("expected active subscription, got %s", sub.Status)
	}
	if period == nil || period.ID == "" {
		t.Fatal("expected billing period")
	}

	sub2, period2, err := subSvc.RenewPeriod(ctx, orgID, "customer", ownerID, planID)
	if err != nil {
		t.Fatalf("RenewPeriod failed: %v", err)
	}
	if sub2.ID != sub.ID {
		t.Errorf("expected same subscription, got %s", sub2.ID)
	}
	if period2 == nil || period2.ID != sub.CurrentPeriodID {
		t.Errorf("expected same period %s, got %v", sub.CurrentPeriodID, period2)
	}

	allocated := countRows(t, ctx, db, `SELECT COUNT(*) FROM credit_grants WHERE idempotency_key = $1 AND source = 'allocation'`, fmt.Sprintf("allocation:%s:%s:%s", orgID, period.ID, ownerID))
	if allocated != 1 {
		t.Errorf("expected 1 allocation grant row, got %d", allocated)
	}
	var granted int64
	if err := db.DB.QueryRowContext(ctx, `SELECT granted FROM credit_accounts WHERE owner_type='customer' AND owner_id=$1 AND tenant_id=$2 AND period_id=$3`, ownerID, orgID, period.ID).Scan(&granted); err != nil {
		t.Fatalf("failed to read balance: %v", err)
	}
	if granted != 1000 {
		t.Errorf("expected 1000 allocated credits, got %d", granted)
	}
	if n := countRows(t, ctx, db, `SELECT COUNT(*) FROM billing_periods WHERE subscription_id = $1`, sub.ID); n != 1 {
		t.Errorf("expected 1 billing period, got %d", n)
	}
}

// TestCreditPurchase_OnPaymentCaptured_Idempotent validates the full Payment → Credit
// purchase → idempotent credit grant path against real Postgres. A replayed capture
// (webhook redelivery) must grant credits exactly once.
func TestCreditPurchase_OnPaymentCaptured_Idempotent(t *testing.T) {
	env := setupPaymentIntegration(t)
	defer env.db.Close()

	creditPack := int64(5000)
	planID := testUUID()
	insertTestPlan(t, env.db, env.ctx, planID, env.orgID, "credit-pack", 1000, 0, creditPack)

	accounts := postgres.NewCreditAccountRepository(env.db.DB)
	purchases := postgres.NewCreditPurchaseRepository(env.db.DB)
	creditSvc := application.NewCreditPurchaseService(application.CreditPurchaseServiceConfig{
		Payments:  env.svc,
		Purchases: purchases,
		Accounts:  accounts,
		Granter:   accounts,
		Logger:    zerolog.Nop(),
	})

	ownerID := testUUID()
	metadata := map[string]interface{}{
		"plan_id":     planID,
		"owner_type":  string(domain.ActorTypeCustomer),
		"owner_id":    ownerID,
		"credit_pack": creditPack,
	}
	p, err := env.svc.CreatePayment(env.ctx, env.orgID, "actor-1", env.orderID,
		domain.Money{AmountMinor: 1000, Currency: "USD"},
		domain.PaymentMethodCard, "method-1", "idem-cp-create", metadata)
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCreated {
		t.Fatalf("expected payment created, got %s", p.Status)
	}
	p, err = env.svc.AuthorizePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-cp-auth")
	if err != nil {
		t.Fatalf("AuthorizePayment failed: %v", err)
	}
	p, err = env.svc.CapturePayment(env.ctx, env.orgID, "actor-1", p.ID, "idem-cp-cap", nil)
	if err != nil {
		t.Fatalf("CapturePayment failed: %v", err)
	}
	if p.Status != domain.PaymentStatusCaptured {
		t.Fatalf("expected captured payment, got %s", p.Status)
	}

	purchase, err := creditSvc.OnPaymentCaptured(env.ctx, env.orgID, "actor-1", p.ID)
	if err != nil {
		t.Fatalf("OnPaymentCaptured failed: %v", err)
	}
	if purchase == nil || purchase.ID == "" {
		t.Fatal("expected a credit purchase to be recorded")
	}
	if purchase.Status != domain.CreditPurchaseStatusCaptured {
		t.Errorf("expected captured purchase, got %s", purchase.Status)
	}

	// Replay (webhook redelivery) must be idempotent.
	if _, err := creditSvc.OnPaymentCaptured(env.ctx, env.orgID, "actor-1", p.ID); err != nil {
		t.Fatalf("replay OnPaymentCaptured failed: %v", err)
	}

	periodID := nowMonth()
	if n := countRows(t, env.ctx, env.db, `SELECT COUNT(*) FROM credit_grants WHERE idempotency_key = $1`, creditPurchaseIdempotencyKeyPrefixForTest(p.ID)); n != 1 {
		t.Errorf("expected 1 credit_grants row, got %d", n)
	}
	if n := countRows(t, env.ctx, env.db, `SELECT COUNT(*) FROM credit_purchases WHERE payment_id = $1`, p.ID); n != 1 {
		t.Errorf("expected 1 credit_purchase row, got %d", n)
	}
	if n := countRows(t, env.ctx, env.db, `SELECT COUNT(*) FROM usage_ledger WHERE metadata->>'payment_id' = $1 AND transaction_type = 'PURCHASE'`, p.ID); n != 1 {
		t.Errorf("expected 1 PURCHASE ledger row, got %d", n)
	}
	var granted int64
	if err := env.db.DB.QueryRowContext(env.ctx, `SELECT granted FROM credit_accounts WHERE owner_type='customer' AND owner_id=$1 AND tenant_id=$2 AND period_id=$3`, ownerID, env.orgID, periodID).Scan(&granted); err != nil {
		t.Fatalf("failed to read balance: %v", err)
	}
	if granted != creditPack {
		t.Errorf("expected %d granted credits, got %d", creditPack, granted)
	}
}

func creditPurchaseIdempotencyKeyPrefixForTest(paymentID string) string {
	return "purchase:" + paymentID
}
