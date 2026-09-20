package commerce

import (
	"context"
	"testing"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

func TestInMemoryIdempotencyStore_CheckNotFound(t *testing.T) {
	store := NewInMemoryIdempotencyStore()

	status, data, found := store.Check(context.Background(), "nonexistent")
	if found {
		t.Error("expected not found")
	}
	if status != "" {
		t.Errorf("expected empty status, got %s", status)
	}
	if data != nil {
		t.Error("expected nil data")
	}
}

func TestInMemoryIdempotencyStore_StoreAndCheck(t *testing.T) {
	store := NewInMemoryIdempotencyStore()

	result := domain.NewCommerceActionResult("action-1", domain.ActionResultSuccess)
	result.AddData("key", "value")

	if err := store.Store(context.Background(), "key-1", *result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	status, data, found := store.Check(context.Background(), "key-1")
	if !found {
		t.Fatal("expected to find stored result")
	}
	if status != domain.ActionResultSuccess {
		t.Errorf("expected SUCCESS, got %s", status)
	}
	if data == nil {
		t.Fatal("expected non-nil data")
	}
	if data["key"] != "value" {
		t.Errorf("expected value, got %v", data["key"])
	}
}

func TestInMemoryIdempotencyStore_Overwrite(t *testing.T) {
	store := NewInMemoryIdempotencyStore()

	result1 := domain.NewCommerceActionResult("action-1", domain.ActionResultSuccess)
	result2 := domain.NewCommerceActionResult("action-2", domain.ActionResultFailed)

	store.Store(context.Background(), "key-1", *result1)
	store.Store(context.Background(), "key-1", *result2)

	status, _, found := store.Check(context.Background(), "key-1")
	if !found {
		t.Fatal("expected to find stored result")
	}
	if status != domain.ActionResultFailed {
		t.Errorf("expected FAILED after overwrite, got %s", status)
	}
}

func TestInMemoryIdempotencyStore_Cleanup(t *testing.T) {
	store := NewInMemoryIdempotencyStore()

	store.Store(context.Background(), "old-key", *domain.NewCommerceActionResult("a1", domain.ActionResultSuccess))
	store.Store(context.Background(), "new-key", *domain.NewCommerceActionResult("a2", domain.ActionResultSuccess))

	// Force entries to be old by using a negative duration since entry created time.
	// Since entries were just created, we need to wait or use a large negative cutoff.
	removed := store.Cleanup(context.Background(), 1*time.Hour)
	if removed != 0 {
		t.Errorf("expected 0 entries removed (all fresh within 1h), got %d", removed)
	}

	_, _, found := store.Check(context.Background(), "old-key")
	if !found {
		t.Error("expected old-key to still exist")
	}
}

func TestInMemoryIdempotencyStore_CleanupRemovesOld(t *testing.T) {
	store := NewInMemoryIdempotencyStore()

	store.Store(context.Background(), "key-1", *domain.NewCommerceActionResult("a1", domain.ActionResultSuccess))
	time.Sleep(20 * time.Millisecond)

	removed := store.Cleanup(context.Background(), 0)
	if removed < 1 {
		t.Errorf("expected at least 1 entry removed, got %d", removed)
	}
}

func TestIdempotencyEntryHasResultAndTimestamp(t *testing.T) {
	entry := IdempotencyEntry{
		Result:    *domain.NewCommerceActionResult("a1", domain.ActionResultSuccess),
		CreatedAt: time.Now(),
	}
	if entry.Result.ActionID != "a1" {
		t.Error("expected action id a1")
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected non-zero created at")
	}
}
