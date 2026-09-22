# Sprint 15 — Usage & Credits Integration

## Overview

Sprint 15 reuses the existing Usage & Credits system from Sprint 9. There is NO separate AI credit system. AI operations consume credits through the same `UsageService` that powers all commerce actions.

## Integration Point

The `AIOrchestrator` resolves usage policies for AI tasks via the existing `domain.UsageService`:

```go
// AI tasks resolve their own usage policy
policy, err := usageService.ResolvePolicy(ctx, "AI_INTERPRETATION", channel, actorType)
```

## Usage Categories

| AI Operation                          | Usage Key             |
|---------------------------------------|-----------------------|
| AI intent classification (per request)   | AI_INTERPRETATION    |
| AI response generation (per request)     | AI_RESPONSE_GENERATION |

These are registered in the existing `usage_policies` table alongside commerce actions.

## Flow

```
AI Request
    ↓
Resolve Usage Policy (via existing UsageService)
    ↓
Reserve Credits
    ↓
AI Execution (Model Router → AI Gateway → Provider)
    ↓
Success → Consume Credits
    ↓
Failure → Release Credits
```

## Duplicate Request Handling

The orchestrator generates an idempotency key for each AI request. If the same request is received again (same conversation + same message + same task), the existing `IdempotencyStore` (Sprint 7) deduplicates it — preventing double credit consumption.

## Credit Safety

- Credits are **reserved before** any AI provider is invoked
- If the AI provider call fails, credits are **released**
- If AI output is invalid or fails schema validation, credits are **released**
- If the downstream commerce action fails (e.g., unauthorized), credits are **released** (the AI interpretation itself was still consumed — only the AI operation is billed)

### Exception

If the AI call succeeds but the downstream commerce action fails due to authorization, the AI credit is still consumed (the interpretation was performed), but the commerce action's credit reservation (if any) is released separately by CommerceActionRuntime.

## Policy Configuration

Usage policies for AI tasks follow the existing `UsagePolicy` schema:

| Field        | Example          |
|-------------|------------------|
| `action_type` | "AI_INTERPRETATION" |
| `channel`     | "whatsapp", "web", "mobile" |
| `actor_type`  | "customer", "seller" |
| `cost`        | 1 (per AI request) |
| `active`      | true/false |

Policies are configurable via the existing usage policy management — no new admin interface needed in Sprint 15.

## No AI-Specific Wallet

There is NO:

- AI credit balance endpoint
- AI billing dashboard
- AI subscription plan

All AI usage is billed through the existing credit account associated with the tenant's plan.
