package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/application/commerce"
	"github.com/trezutil/trezutil/internal/domain"
)

/* Durable idempotency and credit recovery on real PostgreSQL (migration 000039). */

type slowCancel struct{ calls int32 }

func (h *slowCancel) ActionType() domain.CommerceActionType { return domain.ActionCancelOrder }

func (h *slowCancel) Handle(context.Context, domain.CommerceActor, map[string]interface{}) (interface{}, error) {
	atomic.AddInt32(&h.calls, 1)
	time.Sleep(50 * time.Millisecond)
	return map[string]interface{}{"order_id": "o1"}, nil
}

// instance is one process: a new one forgets everything it held in memory.
func instance(db *TestDB, h commerce.Handler) *commerce.CommerceActionRuntime {
	registry := commerce.NewHandlerRegistry()
	registry.Register(h)
	return commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
		Logger: zerolog.Nop(), HandlerRegistry: registry, IdempotencyStore: postgres.NewActionJournal(db.DB),
	})
}

func cancelRequest(key, order string) (*domain.CommerceAction, domain.CommerceActor) {
	actor := domain.NewCommerceActor("user-1", domain.ActorTypeCustomer)
	a := domain.NewCommerceAction(domain.ActionCancelOrder, actor, "conv-1")
	a.IdempotencyKey = key
	a.ConfirmationState = domain.ConfirmationApproved
	a.Parameters["order_id"] = order
	return a, actor
}

func TestActionJournal_OneExecutionAcrossInstancesAndRestarts(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	h := &slowCancel{}
	instances := []*commerce.CommerceActionRuntime{instance(db, h), instance(db, h), instance(db, h)}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(rt *commerce.CommerceActionRuntime) {
			defer wg.Done()
			a, actor := cancelRequest("k1", "o1")
			if _, err := rt.Execute(ctx, a, actor); err != nil {
				t.Error(err)
			}
		}(instances[i%3])
	}
	wg.Wait()
	if h.calls != 1 {
		t.Fatalf("%d executions", h.calls)
	}
	// After a restart, the same request answers with the first result.
	a, actor := cancelRequest("k1", "o1")
	res, err := instance(db, h).Execute(ctx, a, actor)
	if err != nil || res.MessageKey != "ACTION_IDEMPOTENT_CACHE_HIT" || h.calls != 1 {
		t.Fatalf("after restart: %v %s %d", err, res.MessageKey, h.calls)
	}
	data, _ := res.Data["data"].(map[string]interface{})
	if data["order_id"] != "o1" {
		t.Fatalf("result %v", res.Data)
	}
	other, actor := cancelRequest("k1", "o2")
	if res, _ = instance(db, h).Execute(ctx, other, actor); res.MessageKey != "IDEMPOTENCY_CONFLICT" || h.calls != 1 {
		t.Fatalf("conflict: %s", res.MessageKey)
	}
}

func TestActionJournal_AnAbandonedClaimIsTakenAgain(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	journal := postgres.NewActionJournal(db.DB)
	want := func(state commerce.ClaimState) {
		t.Helper()
		c, err := journal.Claim(ctx, "key", "print")
		if err != nil || c.State != state {
			t.Fatalf("claim %v %v, want %v", c.State, err, state)
		}
	}
	want(commerce.ClaimAcquired) // the process stops here
	want(commerce.ClaimInProgress)
	if _, err := db.DB.Exec(`UPDATE commerce_action_journal SET claimed_at = NOW() - INTERVAL '10 minutes'`); err != nil {
		t.Fatal(err)
	}
	want(commerce.ClaimAcquired)
	if err := journal.Release(ctx, "key"); err != nil {
		t.Fatal(err)
	}
	want(commerce.ClaimAcquired)
	if err := journal.Store(ctx, "key", *domain.NewCommerceActionResult("a", domain.ActionResultSuccess)); err != nil {
		t.Fatal(err)
	}
	want(commerce.ClaimCompleted)
	if err := journal.Release(ctx, "key"); err != nil { // releasing never erases a completed action
		t.Fatal(err)
	}
	want(commerce.ClaimCompleted)
	if journal.Cleanup(ctx, time.Hour) != 0 {
		t.Fatal("recent results must be kept")
	}
}

func TestCredits_ReservationsSurviveRetriesAndAreSettledByWhatIsKnown(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	ctx := context.Background()
	tenantID := testUUID()
	createTestOrganization(t, db, ctx, tenantID)
	repo := postgres.NewUsageReservationRepository(db.DB)
	account := seedAccount(t, db, ctx, tenantID, 100)
	now := time.Now().Unix()
	old := now - 3600
	balance := func(reserved, consumed int64) {
		t.Helper()
		var r, c int64
		if err := db.DB.QueryRow(`SELECT reserved, consumed FROM credit_accounts WHERE account_id = $1`, account).Scan(&r, &c); err != nil {
			t.Fatal(err)
		}
		if r != reserved || c != consumed {
			t.Fatalf("reserved %d consumed %d, want %d %d", r, c, reserved, consumed)
		}
	}
	reserve := func(key string) *domain.UsageReservation {
		t.Helper()
		res, err := repo.ReserveCreditsTx(ctx, account, domain.ActionCancelOrder, 10, key, tenantID, old)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	// The same action asked at once holds its credits once.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.ReserveCreditsTx(ctx, account, domain.ActionCancelOrder, 10, "same", tenantID, old); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	balance(10, 0)
	// Released then tried again: the credits are held again, not given free.
	first := reserve("same")
	if err := repo.ReleaseReservation(ctx, first.ReservationID, now); err != nil {
		t.Fatal(err)
	}
	balance(0, 0)
	again := reserve("same")
	if again.ReservationID != first.ReservationID || again.Status != domain.UsageReservationActive {
		t.Fatalf("revived %+v", again)
	}
	balance(10, 0)
	if err := repo.ConsumeReservation(ctx, again.ReservationID, now); err != nil {
		t.Fatal(err)
	}
	if paid := reserve("same"); paid.Status != domain.UsageReservationConsumed {
		t.Fatalf("an action already paid is charged again: %+v", paid)
	}
	balance(0, 10)
	if err := repo.ConsumeReservation(ctx, again.ReservationID, now); !errors.Is(err, domain.ErrUsageAlreadyConsumed) {
		t.Fatal(err)
	}

	// What a stopped process leaves behind.
	never, succeeded, running, failed := reserve("never"), reserve("succeeded"), reserve("running"), reserve("failed")
	for r, state := range map[*domain.UsageReservation]domain.UsageActionState{succeeded: domain.UsageActionSucceeded,
		running: domain.UsageActionRunning, failed: domain.UsageActionFailed} {
		if err := repo.MarkActionState(ctx, r.ReservationID, state); err != nil {
			t.Fatal(err)
		}
	}
	balance(40, 10)
	sweep, err := repo.ResolveStaleReservations(ctx, 15*time.Minute, now)
	if err != nil || sweep != (postgres.ReservationSweep{Released: 2, Consumed: 1, Held: 1}) {
		t.Fatalf("%+v %v", sweep, err)
	}
	balance(10, 20)
	held, err := repo.HeldReservations(ctx, 15*time.Minute, now)
	if err != nil || len(held) != 1 || held[0].ReservationID != running.ReservationID {
		t.Fatalf("held %v %v (never %s)", held, err, never.ReservationID)
	}
	if err := repo.ReleaseReservation(ctx, running.ReservationID, now); err != nil {
		t.Fatal(err)
	}
	balance(0, 20)

	// An account that no longer holds the credits is reported, never silently "consumed".
	broken := reserve("broken")
	if _, err := db.DB.Exec(`UPDATE credit_accounts SET reserved = 0 WHERE account_id = $1`, account); err != nil {
		t.Fatal(err)
	}
	if err := repo.ConsumeReservation(ctx, broken.ReservationID, now); err == nil {
		t.Fatal("consumed without reserved credits")
	}
	var status string
	if err := db.DB.QueryRow(`SELECT status FROM usage_reservations WHERE reservation_id = $1`, broken.ReservationID).Scan(&status); err != nil || status != "active" {
		t.Fatalf("status %s %v", status, err)
	}
}

func TestMigration000039_DownAndUp(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	raw, err := os.ReadFile(filepath.Join(migrationsDir, "000039_action_journal.sql"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	exists := func() (n int) {
		t.Helper()
		if err := db.DB.QueryRow(`SELECT (SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'commerce_action_journal') +
			(SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'usage_reservations' AND column_name = 'action_state')`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if _, err := db.DB.Exec(content[strings.Index(content, "-- +goose Down\n"):]); err != nil || exists() != 0 {
		t.Fatalf("down: %v", err)
	}
	if _, err := db.DB.Exec(extractUpSection(content)); err != nil || exists() != 2 {
		t.Fatalf("up: %v", err)
	}
}
