package application

import "testing"

func TestMetrics_Snapshot(t *testing.T) {
	m := NewMetrics()

	m.RecordProjectionEvent(true)
	m.RecordProjectionEvent(false)
	m.RecordProjectionDeadLetter()

	snap := m.Snapshot()

	if snap["search_projection_events_total"] != 2 {
		t.Errorf("expected total 2, got %d", snap["search_projection_events_total"])
	}
	if snap["search_projection_success_total"] != 1 {
		t.Errorf("expected success 1, got %d", snap["search_projection_success_total"])
	}
	if snap["search_projection_failures_total"] != 1 {
		t.Errorf("expected failures 1, got %d", snap["search_projection_failures_total"])
	}
	if snap["search_projection_dead_letter_total"] != 1 {
		t.Errorf("expected dead letter 1, got %d", snap["search_projection_dead_letter_total"])
	}
}

func TestMetrics_EmptySnapshot(t *testing.T) {
	m := NewMetrics()
	snap := m.Snapshot()

	for k, v := range snap {
		if v != 0 {
			t.Errorf("expected 0 for %s, got %d", k, v)
		}
	}
}
