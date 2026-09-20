package messaging

import "context"

// EventConsumer defines the interface for consuming domain events.
type EventConsumer interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
