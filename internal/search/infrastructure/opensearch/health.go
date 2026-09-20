package opensearch

import (
	"context"
	"fmt"
	"time"
)

// HealthChecker provides OpenSearch health checks.
type HealthChecker struct {
	client *Client
}

// NewHealthChecker creates a new HealthChecker.
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{
		client: client,
	}
}

// HealthStatus represents the health of the OpenSearch connection.
type HealthStatus struct {
	Available bool
	CheckedAt int64
}

// Check returns the current health status of OpenSearch.
func (h *HealthChecker) Check(ctx context.Context) HealthStatus {
	status := HealthStatus{
		CheckedAt: time.Now().Unix(),
	}

	if err := h.client.Ping(ctx); err != nil {
		return status
	}

	status.Available = true
	return status
}

// WaitForReady waits until OpenSearch is ready or the context is cancelled.
func (h *HealthChecker) WaitForReady(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("opensearch not ready: %w", ctx.Err())
		case <-ticker.C:
			status := h.Check(ctx)
			if status.Available {
				return nil
			}
		}
	}
}
