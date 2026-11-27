package domain

import "context"

/*
 * An action with an effect (an order, a booking) must happen once, whatever is retried. The runtime hands its handler
 * one effect key per business intention; the service that creates the record stores it, so a repeat returns the same
 * record instead of creating another.
 */

type effectKeyContext struct{}

// WithEffectKey attaches the server-chosen effect key of the action being executed.
func WithEffectKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, effectKeyContext{}, key)
}

// EffectKey returns the effect key of the action being executed, or "".
func EffectKey(ctx context.Context) string {
	key, _ := ctx.Value(effectKeyContext{}).(string)
	return key
}

// UsageActionState is what is known of the action an active credit reservation pays for. It decides what happens to a
// reservation left behind: consumed when the action succeeded, released when it failed or never started, kept for a
// person when it started and its outcome is unknown.
type UsageActionState string

const (
	UsageActionRunning   UsageActionState = "running"
	UsageActionSucceeded UsageActionState = "succeeded"
	UsageActionFailed    UsageActionState = "failed"
)
