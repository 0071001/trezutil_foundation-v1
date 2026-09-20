package commerce

import (
	"context"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// IdempotencyStore stores and checks idempotency keys for commerce actions.
type IdempotencyStore interface {
	Check(ctx context.Context, idempotencyKey string) (domain.ActionResultStatus, map[string]interface{}, bool)
	Store(ctx context.Context, idempotencyKey string, result domain.CommerceActionResult) error
	Cleanup(ctx context.Context, olderThan time.Duration) int
}

// InMemoryIdempotencyStore is an in-memory implementation of IdempotencyStore.
type InMemoryIdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]IdempotencyEntry
}

// IdempotencyEntry represents a stored idempotency result.
type IdempotencyEntry struct {
	Result    domain.CommerceActionResult
	CreatedAt time.Time
}

// NewInMemoryIdempotencyStore creates a new InMemoryIdempotencyStore.
func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		entries: make(map[string]IdempotencyEntry),
	}
}

// Check returns a cached result if the idempotency key exists.
// Returns (status, data, found).
func (s *InMemoryIdempotencyStore) Check(ctx context.Context, idempotencyKey string) (domain.ActionResultStatus, map[string]interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.entries[idempotencyKey]
	if !ok {
		return "", nil, false
	}
	return entry.Result.Status, entry.Result.Data, true
}

// Store saves a result for an idempotency key.
func (s *InMemoryIdempotencyStore) Store(ctx context.Context, idempotencyKey string, result domain.CommerceActionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[idempotencyKey] = IdempotencyEntry{
		Result:    result,
		CreatedAt: time.Now(),
	}
	return nil
}

// Cleanup removes entries older than the given duration. Returns the number of removed entries.
func (s *InMemoryIdempotencyStore) Cleanup(ctx context.Context, olderThan time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0
	for key, entry := range s.entries {
		if entry.CreatedAt.Before(cutoff) {
			delete(s.entries, key)
			removed++
		}
	}
	return removed
}
