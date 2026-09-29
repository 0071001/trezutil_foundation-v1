package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

type testClock struct{ t int64 }

func (c *testClock) Now() int64 { return c.t }

func seedAccount(t *testing.T, db *TestDB, ctx context.Context, tenantID string, granted int64) string {
	t.Helper()
	accountID := testUUID()
	ownerID := testUUID()
	_, err := db.DB.ExecContext(ctx,
		`INSERT INTO credit_accounts (account_id, owner_type, owner_id, tenant_id, plan_id, period_id, granted, reserved, consumed)
		 VALUES ($1, 'customer', $2, $3, '', 'test-period', $4, 0, 0)`,
		accountID, ownerID, tenantID, granted,
	)
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	return accountID
}

func newUsageService(t *testing.T, db *TestDB) *application.UsageService {
	t.Helper()
	return application.NewUsageService(
		postgres.NewPlanRepository(db.DB),
		postgres.NewCreditAccountRepository(db.DB),
		postgres.NewUsageLedgerRepository(db.DB),
		postgres.NewUsagePolicyRepository(db.DB),
		postgres.NewUsageReservationRepository(db.DB),
		&testClock{t: time.Now().Unix()},
	)
}

// Test A: 100 credits, 20 concurrent reservations of 10. Exactly 10 must succeed,
// 10 must fail with ErrInsufficientCredits. Balance must never go negative.
func TestUsage_ConcurrentReservations_OverdrawPrevented(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	svc := newUsageService(t, db)
	accountID := seedAccount(t, db, ctx, tenantID, 100)

	var ok, fail int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 10, fmt.Sprintf("idem-a-%d", n), tenantID)
			if err == nil {
				atomic.AddInt32(&ok, 1)
			} else {
				atomic.AddInt32(&fail, 1)
			}
		}(i)
	}
	wg.Wait()

	if ok != 10 {
		t.Errorf("expected 10 successful reservations, got %d", ok)
	}
	if fail != 10 {
		t.Errorf("expected 10 failed reservations, got %d", fail)
	}
	acct, err := svc.GetBalance(ctx, accountID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	remaining := acct.Granted - acct.Reserved - acct.Consumed
	if remaining < 0 {
		t.Errorf("balance went negative: remaining=%d (granted=%d reserved=%d consumed=%d)",
			remaining, acct.Granted, acct.Reserved, acct.Consumed)
	}
	if acct.Reserved != 100 {
		t.Errorf("expected reserved=100, got %d", acct.Reserved)
	}
}

// Test B: mixed reservation sizes (70 and 40) on a 100-credit account. Exactly one wins.
func TestUsage_ConcurrentMixedSizeReservations(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	svc := newUsageService(t, db)
	accountID := seedAccount(t, db, ctx, tenantID, 100)

	var wg sync.WaitGroup
	var ok70, ok40 bool
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 70, "idem-b-70", tenantID)
		ok70 = err == nil
	}()
	go func() {
		defer wg.Done()
		_, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 40, "idem-b-40", tenantID)
		ok40 = err == nil
	}()
	wg.Wait()

	acct, err := svc.GetBalance(ctx, accountID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	if acct.Reserved != 70 && acct.Reserved != 40 {
		t.Errorf("expected reserved 70 or 40, got %d", acct.Reserved)
	}
	if ok70 && ok40 {
		t.Errorf("both mixed reservations succeeded (overdraw): reserved=%d", acct.Reserved)
	}
	if !ok70 && !ok40 {
		t.Errorf("neither reservation succeeded")
	}
}

// Test C: grant + reserve concurrency. Balance must remain consistent (never negative).
func TestUsage_ConcurrentGrantAndReserve(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	svc := newUsageService(t, db)
	accountID := seedAccount(t, db, ctx, tenantID, 100)

	var wg sync.WaitGroup
	// 10 concurrent grants of 10 (unique keys)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = svc.Grant(ctx, accountID, 10, fmt.Sprintf("grant-%d", n))
		}(i)
	}
	// 10 concurrent reservations of 10
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, _ = svc.Reserve(ctx, accountID, domain.ActionSearchListings, 10, fmt.Sprintf("idem-c-%d", n), tenantID)
		}(i)
	}
	wg.Wait()

	acct, err := svc.GetBalance(ctx, accountID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	remaining := acct.Granted - acct.Reserved - acct.Consumed
	if remaining < 0 {
		t.Errorf("balance went negative: remaining=%d (g=%d r=%d c=%d)",
			remaining, acct.Granted, acct.Reserved, acct.Consumed)
	}
}

// Idempotency: duplicate reserve returns the same reservation, no double deduction.
func TestUsage_ReserveIdempotent(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	svc := newUsageService(t, db)
	accountID := seedAccount(t, db, ctx, tenantID, 100)

	r1, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 10, "idem", tenantID)
	if err != nil {
		t.Fatalf("first reserve: %v", err)
	}
	r2, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 10, "idem", tenantID)
	if err != nil {
		t.Fatalf("replay reserve: %v", err)
	}
	if r1.ReservationID != r2.ReservationID {
		t.Errorf("idempotent reserve should return same id: %s vs %s", r1.ReservationID, r2.ReservationID)
	}
	acct, _ := svc.GetBalance(ctx, accountID)
	if acct.Reserved != 10 {
		t.Errorf("expected reserved=10 after idempotent reserve, got %d", acct.Reserved)
	}
}

// Double consume / double release must be rejected, not double-counted.
func TestUsage_ConsumeAndRelease_NoDoubleAccounting(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	svc := newUsageService(t, db)
	accountID := seedAccount(t, db, ctx, tenantID, 100)

	res, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 30, "idem-dr", tenantID)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	if err := svc.Consume(ctx, res.ReservationID); err != nil {
		t.Fatalf("consume 1: %v", err)
	}
	if err := svc.Consume(ctx, res.ReservationID); !errors.Is(err, domain.ErrUsageAlreadyConsumed) {
		t.Errorf("expected ErrUsageAlreadyConsumed on second consume, got %v", err)
	}

	res2, err := svc.Reserve(ctx, accountID, domain.ActionSearchListings, 20, "idem-dr2", tenantID)
	if err != nil {
		t.Fatalf("reserve2: %v", err)
	}
	if err := svc.Release(ctx, res2.ReservationID); err != nil {
		t.Fatalf("release 1: %v", err)
	}
	if err := svc.Release(ctx, res2.ReservationID); !errors.Is(err, domain.ErrUsageAlreadyReleased) {
		t.Errorf("expected ErrUsageAlreadyReleased on second release, got %v", err)
	}

	acct, _ := svc.GetBalance(ctx, accountID)
	if acct.Consumed != 30 {
		t.Errorf("expected consumed=30, got %d", acct.Consumed)
	}
	if acct.Reserved != 0 {
		t.Errorf("expected reserved=0 after release, got %d", acct.Reserved)
	}
	remaining := acct.Granted - acct.Reserved - acct.Consumed
	if remaining != 70 {
		t.Errorf("expected remaining=70 after release, got %d", remaining)
	}
}

// Orphaned reservations (action never consumed/released, e.g. crash) are
// returned to the account by the stale-reservation sweep; fresh ones are kept.
// Also covers empty keys and per-account scoping of reservation idempotency.
func TestUsage_StaleReservationsReleasedAndKeysScoped(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	repo := postgres.NewUsageReservationRepository(db.DB)
	accountA := seedAccount(t, db, ctx, tenantID, 100)
	accountB := seedAccount(t, db, ctx, tenantID, 100)
	now := time.Now().Unix()

	if _, err := repo.ReserveCreditsTx(ctx, accountA, domain.ActionSearchListings, 10, "", tenantID, now); err == nil {
		t.Fatal("empty idempotency key must be rejected")
	}

	old, err := repo.ReserveCreditsTx(ctx, accountA, domain.ActionSearchListings, 10, "stale-1", tenantID, now-3600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReserveCreditsTx(ctx, accountA, domain.ActionSearchListings, 5, "fresh-1", tenantID, now); err != nil {
		t.Fatal(err)
	}
	// Same key on another account is a different logical action: it must not
	// return account A's reservation.
	if other, err := repo.ReserveCreditsTx(ctx, accountB, domain.ActionSearchListings, 10, "stale-1", tenantID, now); err == nil && other.ReservationID == old.ReservationID {
		t.Fatal("reservation idempotency must be scoped to the account")
	}

	released, err := repo.ReleaseStaleReservations(ctx, 15*time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatalf("expected 1 stale reservation released, got %d", released)
	}
	var reserved int64
	if err := db.DB.QueryRowContext(ctx, `SELECT reserved FROM credit_accounts WHERE account_id = $1`, accountA).Scan(&reserved); err != nil {
		t.Fatal(err)
	}
	if reserved != 5 {
		t.Fatalf("expected only the fresh reservation (5) to remain held, got %d", reserved)
	}
}
