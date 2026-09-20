# Sprint 6 — Commerce Action Runtime — Architecture

## Architecture Overview

The Commerce Action Runtime follows a orchestrator + handler pattern:

```
┌─────────────────────────────────────────────────────────┐
│                    CommerceActionRuntime                 │
│                                                         │
│  Execute(ctx, action, authenticatedActor):               │
│  1. Identity Protection (override actor with auth)       │
│  2. Tenant Resolution (server-side)                      │
│  3. Action Validation (structure)                        │
│  4. Parameter Validation (required fields per type)      │
│  5. Idempotency Check (cache hit → return cached)        │
│  6. Confirmation Policy (requires confirm → return)      │
│  7. Handler Dispatch (registry lookup)                   │
│  8. Result Storage (idempotency cache store)             │
└──────────────────┬──────────────────────────────────────┘
                   │ dispatches to
    ┌──────────────┼──────────────┐
    ▼              ▼              ▼
┌─────────┐ ┌──────────┐ ┌──────────┐
│  Search │ │  Get     │ │  Get     │
│ Handler │ │ Listing  │ │  Order   │
└─────────┘ └──────────┘ └──────────┘
```

## Key Design Decisions

### Identity Protection
The `authenticatedActor` parameter always overrides `action.Actor` at execution time. This prevents AI-supplied actor identity from being trusted.

### Tenant Resolution
Tenant is resolved server-side from the authenticated actor's `OrgID`, falling back to their user ID. Never from client/AI input.

### Handler Architecture
Each action type has a dedicated handler implementing the `Handler` interface. The `HandlerRegistry` maps action types to handlers, enabling extensibility.

### Confirmation Policy
Write and Financial actions require explicit user confirmation before execution. Read actions proceed directly.

### Idempotency
Actions with an `IdempotencyKey` check the store before execution and store results after success, preventing duplicate processing.

## Package Structure

```
internal/application/commerce/
├── runtime.go              # CommerceActionRuntime orchestrator
├── handler.go              # Handler interface + registry
├── search_handler.go       # SEARCH_LISTINGS handler
├── get_listing_handler.go  # GET_LISTING handler
├── get_order_handler.go    # GET_ORDER handler
├── confirmation.go         # Confirmation policy
├── validation.go           # Parameter validation
├── idempotency.go          # In-memory idempotency store
└── *__test.go              # Tests (8 files, 55 tests)
```

## Dependencies

- `github.com/trezutil/trezutil/internal/domain` — all domain types
- `github.com/trezutil/trezutil/internal/search/application` — InMemorySearchEngine (test only)
- `github.com/rs/zerolog` — structured logging
