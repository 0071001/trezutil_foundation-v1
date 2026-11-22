package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application"
	"github.com/trezutil/trezutil/internal/domain"
)

type wallClock struct{}

func (wallClock) Now() int64 { return time.Now().Unix() }

// Migration 000033: a person's own credits live in their personal ('individual') organisation, created once, and the
// existing credit system (accounts, reservations, ledger) works for them unchanged.
func TestPersonalCredits_OnTheRealSchema(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)
	ctx := context.Background()
	user := "123e4567-e89b-12d3-a456-426614174171"
	createTestUser(t, db, ctx, user)
	spaces := postgres.NewPersonalSpaceStore(db.DB)

	if id, err := spaces.Find(ctx, user); err != nil || id != "" {
		t.Fatalf("no personal space before it is needed: %q %v", id, err)
	}

	// Ten first calls at once: exactly one organisation.
	ids := make([]string, 10)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := spaces.Ensure(ctx, user)
			if err != nil {
				t.Errorf("ensure: %v", err)
			}
			ids[i] = id
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		if id == "" || id != ids[0] {
			t.Fatalf("concurrent first calls must share one space: %v", ids)
		}
	}
	var orgs int
	var orgType string
	_ = db.DB.QueryRowContext(ctx, `SELECT count(*), max(type) FROM organizations o JOIN personal_spaces p ON p.organization_id = o.id WHERE p.user_id = $1`, user).Scan(&orgs, &orgType)
	if orgs != 1 || orgType != "individual" {
		t.Fatalf("want one individual organisation, got %d %q", orgs, orgType)
	}
	var memberships int
	_ = db.DB.QueryRowContext(ctx, `SELECT count(*) FROM organization_members WHERE user_id = $1`, user).Scan(&memberships)
	if memberships != 0 {
		t.Fatal("a personal space is not a membership: it must not appear among the person's organisations")
	}
	if _, err := spaces.Ensure(ctx, "123e4567-e89b-12d3-a456-426614174199"); err == nil {
		t.Fatal("no personal space for an unknown user")
	}

	// The credit flow of a personal account, called the way the runtime calls it (tenant = the user id).
	usage := application.NewUsageService(postgres.NewPlanRepository(db.DB), postgres.NewCreditAccountRepository(db.DB),
		postgres.NewUsageLedgerRepository(db.DB), postgres.NewUsagePolicyRepository(db.DB),
		postgres.NewUsageReservationRepository(db.DB), wallClock{}).WithPersonalSpaces(spaces)
	account, err := usage.FindOrCreateCreditAccountByOwner(ctx, domain.ActorTypeCustomer, user, user, "", "")
	if err != nil {
		t.Fatalf("a personal account must be created (it failed on the organisations foreign key before): %v", err)
	}
	if account.TenantID != ids[0] {
		t.Fatalf("the account must live in the personal space, got tenant %q", account.TenantID)
	}
	// Credits are granted by the monetization primitive (plans, purchases), exactly as for an organisation.
	if ok, _, err := postgres.NewCreditAccountRepository(db.DB).GrantCreditsTx(ctx, account.AccountID, "personal-grant-1",
		domain.CreditGrantSourceManual, 10, account.TenantID, account.PeriodID, nil, "test"); err != nil || !ok {
		t.Fatalf("grant: %v %v", ok, err)
	}
	res, err := usage.Reserve(ctx, account.AccountID, domain.ActionSearchListings, 3, "personal-k1", user)
	if err != nil {
		t.Fatalf("reserve with the runtime's tenant (the user id) must use the account's organisation: %v", err)
	}
	if err := usage.Consume(ctx, res.ReservationID); err != nil {
		t.Fatalf("consume: %v", err)
	}

	summary, err := postgres.NewConsoleRepository(db.DB).Usage(ctx, ids[0])
	if err != nil || len(summary.Accounts) != 1 {
		t.Fatalf("the personal space's accounts: %+v %v", summary, err)
	}
	a := summary.Accounts[0]
	if a.OwnerID != user || a.Granted-a.Reserved-a.Consumed != 7 || a.Consumed != 3 {
		t.Fatalf("balance after a 3-credit action out of 10: %+v", a)
	}
	if len(summary.Ledger) == 0 {
		t.Fatal("the consumption must be in the personal space's ledger")
	}
}
