// Package providertest holds deterministic provider emulators for tests. They
// implement the provider adapter interfaces directly and define no external
// contract; they must never be registered in a deployed service.
package providertest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/trezutil/trezutil/internal/domain"
)

// Behaviour is how the availability emulator answers a call.
type Behaviour string

const (
	Available   Behaviour = "available"   // explicit AVAILABLE with places left
	Unavailable Behaviour = "unavailable" // explicit UNAVAILABLE
	NoState     Behaviour = "no-state"    // a "successful" call that says nothing about availability
	Malformed   Behaviour = "malformed"   // contradictory numbers (more left than capacity)
	Rejected    Behaviour = "rejected"    // non-retryable provider error
	Refused     Behaviour = "refused"     // retryable connection error
	Slow        Behaviour = "slow"        // answers after the caller's deadline
)

// AvailabilityEmulator is a scripted AvailabilityProviderAdapter. Each call
// takes the next behaviour of Script (the last one repeats) and records the
// idempotency key it received.
type AvailabilityEmulator struct {
	ID     string
	Script []Behaviour
	// Delay is how long Slow waits (default 1s); it honours cancellation.
	Delay time.Duration

	mu   sync.Mutex
	keys []string
	n    int
}

func (e *AvailabilityEmulator) ProviderID() string { return e.ID }
func (e *AvailabilityEmulator) Capabilities() []domain.Capability {
	return []domain.Capability{domain.CapabilityAvailability}
}
func (e *AvailabilityEmulator) Health(context.Context) domain.ProviderHealthStatus {
	return domain.ProviderHealthHealthy
}

// Calls returns how many calls were made.
func (e *AvailabilityEmulator) Calls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.n
}

// IdempotencyKeys returns the keys seen, one per call.
func (e *AvailabilityEmulator) IdempotencyKeys() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.keys...)
}

func (e *AvailabilityEmulator) CheckAvailability(ctx context.Context, q domain.AvailabilityQuery) (domain.AvailabilityResult, error) {
	e.mu.Lock()
	b := Available
	if len(e.Script) > 0 {
		i := e.n
		if i >= len(e.Script) {
			i = len(e.Script) - 1
		}
		b = e.Script[i]
	}
	e.n++
	e.keys = append(e.keys, domain.ProviderIdempotencyKey(ctx))
	e.mu.Unlock()

	switch b {
	case Available:
		return domain.AvailabilityResult{State: domain.AvailabilityAvailable, Capacity: 10, Remaining: 4}, nil
	case Unavailable:
		return domain.AvailabilityResult{State: domain.AvailabilityUnavailable, Capacity: 10, Remaining: 0, Reason: "sold out"}, nil
	case NoState:
		return domain.AvailabilityResult{Available: true}, nil
	case Malformed:
		return domain.AvailabilityResult{State: domain.AvailabilityAvailable, Capacity: 2, Remaining: 9}, nil
	case Rejected:
		return domain.AvailabilityResult{}, errors.New("invalid request: unknown property")
	case Refused:
		return domain.AvailabilityResult{}, errors.New("dial tcp: connection refused")
	case Slow:
		d := e.Delay
		if d == 0 {
			d = time.Second
		}
		select {
		case <-ctx.Done():
			return domain.AvailabilityResult{}, ctx.Err()
		case <-time.After(d):
			return domain.AvailabilityResult{State: domain.AvailabilityAvailable, Capacity: 10, Remaining: 4}, nil
		}
	}
	return domain.AvailabilityResult{}, errors.New("unknown behaviour")
}
