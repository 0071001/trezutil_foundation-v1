package application

import (
	"fmt"
	"testing"

	"github.com/trezutil/trezutil/packages/events"
)

func TestDeadLetter_Add(t *testing.T) {
	m := NewMetrics()
	dl := NewDeadLetter(m)

	event := events.Event{EventID: "evt-1"}
	dl.Add(nil, event, fmt.Errorf("test failure"), 3)

	if dl.GetEntryCount() != 1 {
		t.Fatalf("expected 1 dead-letter entry, got %d", dl.GetEntryCount())
	}

	if m.Snapshot()["search_projection_dead_letter_total"] != 1 {
		t.Errorf("expected dead letter metric 1")
	}
}

func TestDeadLetter_GetEntries(t *testing.T) {
	dl := NewDeadLetter(nil)

	for i := 0; i < 3; i++ {
		dl.Add(nil, events.Event{EventID: fmt.Sprintf("evt-%d", i)}, fmt.Errorf("fail %d", i), i)
	}

	entries := dl.GetEntries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestDeadLetter_String(t *testing.T) {
	dl := NewDeadLetter(nil)
	if dl.String() != "dead-letter entries: 0" {
		t.Errorf("unexpected string: %s", dl.String())
	}

	dl.Add(nil, events.Event{}, nil, 0)
	if dl.String() != "dead-letter entries: 1" {
		t.Errorf("unexpected string: %s", dl.String())
	}
}
