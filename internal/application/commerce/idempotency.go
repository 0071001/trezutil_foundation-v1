package commerce

import (
	"context"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// IdempotencyStore remembers commerce actions by their key. A key is claimed before the action runs, so two requests
// with the same key never run it at the same time, and completed with its result, so a repeat returns that result.
// Production uses a durable store (postgres.ActionJournal): the guarantee must survive a restart.
type IdempotencyStore interface {
	// Check returns the result of a completed action. Returns (status, data, found).
	Check(ctx context.Context, idempotencyKey string) (domain.ActionResultStatus, map[string]interface{}, bool)
	// Claim takes the key for execution. fingerprint identifies the request: the same key with another request is a
	// conflict. A claim older than ClaimTimeout whose action never completed can be taken again.
	Claim(ctx context.Context, idempotencyKey, fingerprint string) (Claim, error)
	// Release gives up a claim whose action did not complete, so it can be asked again.
	Release(ctx context.Context, idempotencyKey string) error
	// Store completes the key with its result.
	Store(ctx context.Context, idempotencyKey string, result domain.CommerceActionResult) error
	// Cleanup removes completed entries older than the given duration.
	Cleanup(ctx context.Context, olderThan time.Duration) int
}

// ClaimState says what a claim found.
type ClaimState int

const (
	ClaimAcquired   ClaimState = iota // run the action
	ClaimCompleted                    // already done: answer with Result
	ClaimInProgress                   // being run now
	ClaimConflict                     // the key belongs to a different request
)

// Claim is the answer to IdempotencyStore.Claim.
type Claim struct {
	State  ClaimState
	Result *domain.CommerceActionResult
}

// ClaimTimeout is how long an action may run before its claim is considered abandoned (a stopped process).
const ClaimTimeout = 5 * time.Minute

// InMemoryIdempotencyStore keeps entries in one process. It is for tests: it forgets everything on restart.
type InMemoryIdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]IdempotencyEntry
	claims  map[string]memoryClaim
	prints  map[string]string
}

type memoryClaim struct {
	fingerprint string
	at          time.Time
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
		claims:  make(map[string]memoryClaim),
		prints:  make(map[string]string),
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

// Claim takes the key for execution.
func (s *InMemoryIdempotencyStore) Claim(ctx context.Context, idempotencyKey, fingerprint string) (Claim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, ok := s.entries[idempotencyKey]; ok {
		if p := s.prints[idempotencyKey]; p != "" && p != fingerprint {
			return Claim{State: ClaimConflict}, nil
		}
		result := entry.Result
		return Claim{State: ClaimCompleted, Result: &result}, nil
	}
	if held, ok := s.claims[idempotencyKey]; ok {
		if held.fingerprint != fingerprint {
			return Claim{State: ClaimConflict}, nil
		}
		if time.Since(held.at) < ClaimTimeout {
			return Claim{State: ClaimInProgress}, nil
		}
	}
	s.claims[idempotencyKey] = memoryClaim{fingerprint: fingerprint, at: time.Now()}
	return Claim{State: ClaimAcquired}, nil
}

// Release gives up a claim that did not complete.
func (s *InMemoryIdempotencyStore) Release(ctx context.Context, idempotencyKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.claims, idempotencyKey)
	return nil
}

// Store saves a result for an idempotency key.
func (s *InMemoryIdempotencyStore) Store(ctx context.Context, idempotencyKey string, result domain.CommerceActionResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if held, ok := s.claims[idempotencyKey]; ok {
		s.prints[idempotencyKey] = held.fingerprint
		delete(s.claims, idempotencyKey)
	}
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
			delete(s.prints, key)
			removed++
		}
	}
	return removed
}
