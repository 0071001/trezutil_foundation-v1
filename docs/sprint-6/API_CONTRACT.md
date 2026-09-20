# Sprint 6 — API Contract

## CommerceActionRuntime.Execute

### Signature

```go
func (r *CommerceActionRuntime) Execute(
    ctx context.Context,
    action *domain.CommerceAction,
    authenticatedActor domain.CommerceActor,
) (*ExecuteResult, error)
```

### Input

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| ctx | context.Context | Yes | Request context |
| action | *domain.CommerceAction | Yes | Action to execute (actor will be overridden) |
| authenticatedActor | domain.CommerceActor | Yes | Real actor from JWT/session |

### Output

| Field | Type | Description |
|-------|------|-------------|
| ActionID | string | From action.ActionID |
| Status | domain.ActionResultStatus | Result status |
| MessageKey | string | Localization key |
| Data | map[string]interface{} | Result data |
| Errors | []domain.ActionError | Structured errors |
| NextAction | *domain.NextAction | Suggested next step |
| RequiresConfirmation | bool | True if user must confirm |
| error | error | Non-nil only if action is nil |

### ExecuteResult Fields

```go
type ExecuteResult struct {
    ActionID               string
    Status                 domain.ActionResultStatus
    MessageKey             string
    Data                   map[string]interface{}
    Errors                 []domain.ActionError
    NextAction             *domain.NextAction
    RequiresConfirmation   bool
}
```

## Handler Interface

```go
type Handler interface {
    Handle(ctx context.Context, actor domain.CommerceActor, parameters map[string]interface{}) (interface{}, error)
    ActionType() domain.CommerceActionType
}
```

### Handler Response Types

| Action | Response Type | Response Shape |
|--------|--------------|----------------|
| SEARCH_LISTINGS | `map[string]interface{}` | `{items, total, has_next, next_cursor}` |
| GET_LISTING | `*domain.Listing` | Full listing entity |
| GET_ORDER | `map[string]interface{}` | `{order_id, status, message}` |

## Idempotency API

```go
type IdempotencyStore interface {
    Check(ctx context.Context, idempotencyKey string) (domain.ActionResultStatus, map[string]interface{}, bool)
    Store(ctx context.Context, idempotencyKey string, result domain.CommerceActionResult) error
    Cleanup(ctx context.Context, olderThan time.Duration) int
}
```

## Confirmation Policy API

```go
type ConfirmationPolicy interface {
    RequiresConfirmation(ctx context.Context, action *domain.CommerceAction) (bool, string)
}
```

## Parameter Validation API

```go
type ParameterValidator interface {
    Validate(ctx context.Context, actionType domain.CommerceActionType, parameters map[string]interface{}) error
}
```

## Error Handling

All errors are captured in `ExecuteResult.Errors` as `domain.ActionError` structs:

```go
type ActionError struct {
    Code    string
    Message string
    Field   string
}
```

The `error` return from `Execute` is only non-nil when `action == nil`.

## Status Codes

| Status | Meaning |
|--------|---------|
| SUCCESS | Action completed successfully |
| REQUIRES_CONFIRMATION | User must confirm before execution |
| VALIDATION_FAILED | Action or parameters invalid |
| UNAUTHORIZED | Insufficient permissions |
| NOT_IMPLEMENTED | Action handler not yet available |
| FAILED | Handler execution failed |
