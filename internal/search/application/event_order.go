package application

import "sync"

// listingVersion tracks the last processed event version per listing.
type listingVersion struct {
	version int
	eventID string
}

// EventOrderTracker ensures events are processed in order per listing.
type EventOrderTracker struct {
	mu       sync.Mutex
	versions map[string]*listingVersion
}

// NewEventOrderTracker creates a new EventOrderTracker.
func NewEventOrderTracker() *EventOrderTracker {
	return &EventOrderTracker{
		versions: make(map[string]*listingVersion),
	}
}

// CheckAndUpdate verifies that the event version is >= the last processed version
// for the given listing. Returns true if the event should be processed.
func (t *EventOrderTracker) CheckAndUpdate(listingID string, version int, eventID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	current, exists := t.versions[listingID]
	if !exists {
		t.versions[listingID] = &listingVersion{
			version: version,
			eventID: eventID,
		}
		return true
	}

	if version < current.version {
		return false
	}

	if version == current.version && eventID == current.eventID {
		return false
	}

	t.versions[listingID] = &listingVersion{
		version: version,
		eventID: eventID,
	}
	return true
}

// GetLastVersion returns the last processed version for a listing.
func (t *EventOrderTracker) GetLastVersion(listingID string) (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	v, exists := t.versions[listingID]
	if !exists {
		return 0, false
	}
	return v.version, true
}
