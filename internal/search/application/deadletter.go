package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/trezutil/trezutil/packages/events"
)

// DeadLetterEntry represents a permanently failing event.
type DeadLetterEntry struct {
	Event          events.Event
	Failure        error
	DeadLetteredAt time.Time
	Retries        int
}

// DeadLetter handles events that fail processing after retries.
type DeadLetter struct {
	mu      sync.Mutex
	entries []DeadLetterEntry
	metrics *Metrics
}

// NewDeadLetter creates a new DeadLetter handler.
func NewDeadLetter(metrics *Metrics) *DeadLetter {
	return &DeadLetter{
		entries: make([]DeadLetterEntry, 0),
		metrics: metrics,
	}
}

// Add records a failed event as dead-lettered.
func (d *DeadLetter) Add(ctx context.Context, event events.Event, failure error, retries int) {
	entry := DeadLetterEntry{
		Event:          event,
		Failure:        failure,
		DeadLetteredAt: time.Now(),
		Retries:        retries,
	}

	d.mu.Lock()
	d.entries = append(d.entries, entry)
	d.mu.Unlock()

	if d.metrics != nil {
		d.metrics.RecordProjectionDeadLetter()
	}
}

// GetEntries returns all dead-lettered entries.
func (d *DeadLetter) GetEntries() []DeadLetterEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]DeadLetterEntry(nil), d.entries...)
}

// GetEntryCount returns the number of dead-lettered entries.
func (d *DeadLetter) GetEntryCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.entries)
}

// String returns a summary of the dead-letter state.
func (d *DeadLetter) String() string {
	return fmt.Sprintf("dead-letter entries: %d", d.GetEntryCount())
}
