# Sprint 9 — Usage & Credits — API Contract

## UsageService Interface

```go
type UsageService interface {
    GetBalance(ctx context.Context, accountID string) (*CreditAccount, error)
    ResolvePolicy(ctx context.Context, actionType CommerceActionType, channel Channel, actorType ActorType) (*UsagePolicy, error)
    Check(ctx context.Context, accountID string, cost int64) (bool, error)
    Reserve(ctx context.Context, accountID string, actionType CommerceActionType, cost int64) (*UsageReservation, error)
    Consume(ctx context.Context, reservationID string) error
    Release(ctx context.Context, reservationID string) error
    Grant(ctx context.Context, accountID string, amount int64, reason string) error
}
```

## CommerceActionRuntime.Execute() Changes

No signature change. UsageService is injected via RuntimeConfig:

```go
rt := commerce.NewCommerceActionRuntime(commerce.RuntimeConfig{
    Logger:             logger,
    HandlerRegistry:    registry,
    IdempotencyStore:   store,
    ConfirmationPolicy: policy,
    ParameterValidator: validator,
    UsageService:       usageService, // optional, nil = no usage checking
})
```

### New Error Codes

| Code | Meaning | When |
|------|---------|------|
| INSUFFICIENT_CREDITS | Account can't afford action cost | Before handler dispatch |
| USAGE_CHECK_FAILED | Error checking account balance | Before handler dispatch |
| USAGE_RESERVE_FAILED | Error reserving credits | Before handler dispatch |
