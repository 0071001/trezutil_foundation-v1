package domain

import "context"

type providerIdempotencyKey struct{}

// WithProviderIdempotencyKey attaches the idempotency key of the current
// provider operation to ctx. Adapters forward it to providers that support
// request idempotency so retries (including orchestrator retries) never move
// money twice.
func WithProviderIdempotencyKey(ctx context.Context, key string) context.Context {
	if key == "" {
		return ctx
	}
	return context.WithValue(ctx, providerIdempotencyKey{}, key)
}

// ProviderIdempotencyKey returns the idempotency key attached by
// WithProviderIdempotencyKey, or "".
func ProviderIdempotencyKey(ctx context.Context) string {
	key, _ := ctx.Value(providerIdempotencyKey{}).(string)
	return key
}
