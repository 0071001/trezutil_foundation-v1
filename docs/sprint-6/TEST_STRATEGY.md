# Sprint 6 — Test Strategy

## Test Categories

### Unit Tests (55 tests in commerce package)

| Component | Tests | Coverage |
|-----------|-------|----------|
| Runtime orchestration | 19 | Identity, tenant, validation, confirmation, idempotency, dispatch, lifecycle |
| Handler registry | 7 | Register, get, has, overwrite, multiple handlers |
| Search handler | 4 | Success, missing text, empty params, action type |
| Get listing handler | 4 | Success, empty ID, nil params, action type |
| Get order handler | 4 | Success, missing ID, nil params, action type |
| Confirmation policy | 5 | Nil, read, write, financial, confirmed skip |
| Parameter validation | 6 | Nil, missing, empty, zero, valid, unknown |
| Idempotency store | 6 | Not found, store/check, overwrite, cleanup fresh, cleanup old, entry structure |

### Integration with Existing Tests

Commerce package tests reuse existing domain types without modification:
- `domain.CommerceAction`, `domain.CommerceActionResult`, `domain.CommerceActor`
- `domain.DiscoveryQuery`, `domain.SearchEnginePort`
- `domain.ListingRepository` interface
- All existing domain tests continue to PASS (verified via full build)

## Test Patterns

### Fake Implementations

```go
type fakeHandler struct { actionType domain.CommerceActionType }
func (h *fakeHandler) Handle(...) (interface{}, error) { return map[string]interface{}{"ok": true}, nil }
func (h *fakeHandler) ActionType() domain.CommerceActionType { return h.actionType }
```

### Fake Repository

```go
type fakeListingRepository struct { listing *domain.Listing; err error }
// Implements domain.ListingRepository
```

### Test Runtime Factory

```go
func newTestRuntime(t *testing.T) *CommerceActionRuntime {
    return NewCommerceActionRuntime(RuntimeConfig{...})
}
```

## Validation Commands

```bash
go build ./...          # Full project compilation
go vet ./...            # Static analysis
gofmt -l .              # Format check (empty = pass)
go test ./internal/application/commerce/...  # Commerce package tests (55)
go test ./internal/domain/...                # Domain tests (existing)
go test ./internal/application/...           # Application tests (existing)
```

## Test Results

- All 55 commerce tests PASS
- All existing domain tests PASS (no regressions)
- All existing application tests PASS (no regressions)
- Full project builds without errors
