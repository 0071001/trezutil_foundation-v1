package application

import (
	"sync"
)

// Metrics holds observability counters for the search projection.
type Metrics struct {
	mu sync.Mutex

	ProjectionEventsTotal      int64
	ProjectionEventsFailed     int64
	ProjectionEventsSuccess    int64
	ProjectionEventsDeadLetter int64

	IndexRequestsTotal      int64
	IndexRequestsFailed     int64
	IndexRequestLatencyNano int64

	ReindexDurationNano int64
	ReindexItemsTotal   int64
	ReindexErrorsTotal  int64

	HealthCheckAvailable   bool
	HealthCheckLatencyNano int64
}

// NewMetrics creates a new Metrics instance.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordProjectionEvent records a projection event attempt.
func (m *Metrics) RecordProjectionEvent(success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProjectionEventsTotal++
	if success {
		m.ProjectionEventsSuccess++
	} else {
		m.ProjectionEventsFailed++
	}
}

// RecordProjectionDeadLetter records a dead-lettered event.
func (m *Metrics) RecordProjectionDeadLetter() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ProjectionEventsDeadLetter++
}

// RecordIndexRequest records an index operation.
func (m *Metrics) RecordIndexRequest(success bool, latencyNano int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.IndexRequestsTotal++
	if success {
		// success is implicit in total count
	} else {
		m.IndexRequestsFailed++
	}
	m.IndexRequestLatencyNano += latencyNano
}

// RecordReindex records a reindex operation.
func (m *Metrics) RecordReindex(durationNano int64, items int, errors int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ReindexDurationNano += durationNano
	m.ReindexItemsTotal += int64(items)
	m.ReindexErrorsTotal += int64(errors)
}

// RecordHealthCheck records a health check result.
func (m *Metrics) RecordHealthCheck(available bool, latencyNano int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.HealthCheckAvailable = available
	m.HealthCheckLatencyNano += latencyNano
}

// Snapshot returns a copy of all metrics for reporting.
func (m *Metrics) Snapshot() map[string]int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return map[string]int64{
		"search_projection_events_total":        m.ProjectionEventsTotal,
		"search_projection_success_total":       m.ProjectionEventsSuccess,
		"search_projection_failures_total":      m.ProjectionEventsFailed,
		"search_projection_dead_letter_total":   m.ProjectionEventsDeadLetter,
		"opensearch_requests_total":             m.IndexRequestsTotal,
		"opensearch_requests_failed_total":      m.IndexRequestsFailed,
		"opensearch_request_latency_total_nano": m.IndexRequestLatencyNano,
		"search_reindex_duration_total_nano":    m.ReindexDurationNano,
		"search_reindex_items_total":            m.ReindexItemsTotal,
		"search_reindex_errors_total":           m.ReindexErrorsTotal,
	}
}
