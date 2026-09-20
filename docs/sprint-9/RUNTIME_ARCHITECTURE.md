# Sprint 9 — Usage & Credits — Architecture

## Overview

```text
                        ┌──────────────────────┐
                        │   UsageService        │
                        │  (application layer)  │
                        └──────────┬───────────┘
                                   │
                    ┌──────────────┼──────────────┐
                    │              │              │
            ┌───────▼─────┐ ┌─────▼──────┐ ┌────▼────────┐
            │PlanRepository│ │ CreditAccount│ │UsagePolicy  │
            │ (domain intf)│ │Repository   │ │Repository   │
            └───────┬──────┘ └──────┬─────┘ └──────┬──────┘
                    │              │              │
                    │     ┌────────▼────────┐     │
                    │     │ UsageLedger      │     │
                    │     │ Repository       │     │
                    │     └─────────────────┘     │
                    │                             │
             ┌──────▼─────────────────────────────▼──────┐
             │           UsageService                     │
             │  (concrete impl in application layer)      │
             └─────────────────┬──────────────────────────┘
                               │
             ┌─────────────────▼──────────────────────────┐
             │      CommerceActionRuntime                   │
             │                                              │
             │  Execute():                                  │
             │    1. Identity protection                    │
             │    2. Tenant resolution                      │
             │    3. Validate action                        │
             │    4. Validate parameters                    │
             │    5. Check idempotency                      │
             │    6. Check confirmation                     │
             │    7. USAGE CHECK (new)                      │
             │       └─ ResolvePolicy → Check → Reserve     │
             │    8. Route to handler                       │
             │    9. Execute handler                        │
             │    10. Consume/Release reservation (new)     │
             │    11. Store idempotency result              │
             └─────────────────────────────────────────────┘
```

## Key Design Decisions

1. **Optional integration**: UsageService is nil by default in RuntimeConfig. Existing runtimes without UsageService work unchanged.
2. **System actors skip**: Internal system actions don't incur usage costs.
3. **Cleanup on failure**: If handler fails, reservation is released. If consume fails, warning is logged but result is still success.
4. **Policy resolution**: Channel defaults to ChannelOther since CommerceAction has no channel field. Actor type is used for policy matching.
5. **In-memory reservations**: Runtime tracks reservations in a sync.Map (mutex-protected map). Durable storage requires PostgreSQL.
