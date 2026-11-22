package integration

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/infrastructure/postgres"
	"github.com/trezutil/trezutil/internal/domain"
)

// Commercial memory (migration 000032): per user, scoped on every statement, switchable, and gone with the user.
func TestMemoryStore(t *testing.T) {
	db := NewTestDB(t)
	defer db.Close()
	db.Cleanup(t)

	ctx := context.Background()
	me := "123e4567-e89b-12d3-a456-426614174161"
	other := "123e4567-e89b-12d3-a456-426614174162"
	createTestUser(t, db, ctx, me)
	if err := postgres.NewUserRepository(db.DB).Create(ctx, &domain.User{
		ID: other, Phone: "+243812345681", DisplayName: "Other", Status: domain.UserStatusActive,
		CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix(), Roles: []domain.Role{},
	}); err != nil {
		t.Fatalf("create second user: %v", err)
	}
	store := postgres.NewMemoryStore(db.DB)

	if on, err := store.Enabled(ctx, me); err != nil || !on {
		t.Fatalf("memory is on until switched off: %v %v", on, err)
	}
	dell := &domain.MemoryItem{UserID: me, Kind: domain.MemoryBrand, Text: "Dell"}
	if err := store.Add(ctx, dell); err != nil || dell.ID == "" || dell.CreatedAt == 0 {
		t.Fatalf("add: %+v %v", dell, err)
	}
	if err := store.Add(ctx, &domain.MemoryItem{UserID: other, Kind: domain.MemoryBrand, Text: "HP"}); err != nil {
		t.Fatalf("add other: %v", err)
	}
	if err := store.Add(ctx, &domain.MemoryItem{UserID: me, Kind: "colour", Text: "bleu"}); err == nil {
		t.Fatal("the database must refuse an unknown kind")
	}

	mine, err := store.List(ctx, me)
	if err != nil || len(mine) != 1 || mine[0].Text != "Dell" || mine[0].Kind != domain.MemoryBrand {
		t.Fatalf("list: %+v %v", mine, err)
	}
	if n, _ := store.Count(ctx, me); n != 1 {
		t.Fatalf("count: %d", n)
	}
	if _, err := store.UpdateText(ctx, other, dell.ID, "pirate"); err != domain.ErrMemoryItemNotFound {
		t.Fatalf("someone else's update must find nothing, got %v", err)
	}
	if err := store.Delete(ctx, other, dell.ID); err != domain.ErrMemoryItemNotFound {
		t.Fatalf("someone else's delete must find nothing, got %v", err)
	}
	if updated, err := store.UpdateText(ctx, me, dell.ID, "Lenovo"); err != nil || updated.Text != "Lenovo" {
		t.Fatalf("update: %+v %v", updated, err)
	}

	if err := store.SetEnabled(ctx, me, false); err != nil {
		t.Fatalf("switch off: %v", err)
	}
	if on, _ := store.Enabled(ctx, me); on {
		t.Fatal("memory should be off")
	}
	if err := store.SetEnabled(ctx, me, true); err != nil {
		t.Fatalf("switch on again: %v", err)
	}

	if err := store.DeleteAll(ctx, me); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if left, _ := store.List(ctx, me); len(left) != 0 {
		t.Fatalf("clear left %d items", len(left))
	}
	if theirs, _ := store.List(ctx, other); len(theirs) != 1 {
		t.Fatal("clearing my memory must not touch someone else's")
	}
}
