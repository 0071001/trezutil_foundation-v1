# Sprint 15 — AI Architecture

## Overview

Sprint 15 introduces the Provider Orchestration Layer for AI — an AI Gateway, Model Router, and AI Orchestrator — that allow AI to understand conversational requests and produce structured commerce intents while remaining completely outside the trusted business authority boundary.

## Core Principle

```
AI ≠ Authenticated Actor

AI → Intent → Parameters → Authenticated Actor → Authorization → Usage/Credits → CommerceActionRuntime → Commerce Action → Domain
```

AI can suggest. The existing Commerce Runtime remains the authority.

## Architecture

```
                     ┌───────────────────────┐
                     │ WhatsApp / Web / App  │
                     └───────────┬───────────┘
                                 ↓
                        Conversation Layer
                                 ↓
                         ┌───────────────┐
                         │   AI Gateway  │
                         └───────┬───────┘
                                 ↓
                         ┌───────────────┐
                         │ Model Router  │
                         └───────┬───────┘
                                 ↓
                       ┌──────────────────┐
                       │ AI Orchestrator  │
                       └────────┬─────────┘
                                ↓
                             Intent
                                ↓
              CommerceActionRuntime (Sprint 7)
                                ↓
                      Commerce Action
                                ↓
                        Domain Layer
                                ↓
                    Provider Orchestrator
                  (Sprint 14 — Provider Orchestrator)
```

## Components

### AI Gateway
- **Location:** `internal/application/ai/gateway.go`
- **Responsibilities:**
  - Authentication context propagation
  - Request normalization
  - Model/provider selection delegation (via Model Router)
  - Request validation
  - Timeout management
  - Tracing
  - Normalized provider invocation
  - Error normalization
  - Response validation
  - Safety boundary enforcement
- **Constraints:**
  - Must NOT authorize commerce actions
  - Must NOT mutate orders
  - Must NOT reserve inventory directly
  - Must NOT call arbitrary providers
  - Must NOT bypass Usage & Credits

### Model Router
- **Location:** `internal/application/ai/router.go`
- **Responsibilities:**
  - Maps AI task capabilities to eligible models
  - Deterministic routing (capability → active provider → active model → tenant eligibility → configured priority)
  - Returns the selected `AIModel` and `AIProvider`

### AI Orchestrator
- **Location:** `internal/application/ai/orchestrator.go`
- **Responsibilities:**
  - Manages the AI lifecycle: classify → validate → execute allowed action → respond
  - Context management (sanitized conversation context)
  - Intent resolution → CommerceAction construction
  - Delegates to CommerceActionRuntime for authorization + usage + execution
  - Confirmation enforcement (server-side, never AI-decided)
  - Response generation to the conversation layer

### AI Provider Abstraction
- **Location:** `internal/domain/ai_provider.go`
- **Interface:** `AIProvider` with `ID()`, `Models()`, `Generate(ctx, req) (resp, error)`
- **Implementations:**
  - `MockAIProvider` (in-memory, deterministic — used for tests and as the default Sprint 15 provider)
  - Future: OpenAIAdapter, GeminiAdapter, AnthropicAdapter (all behind the same interface)
- **No vendor SDK imports in Commerce Core**

## Data Flow

1. User message arrives via WhatsApp/Web/App → Conversation Layer
2. Conversation passes message + authenticated context to AI Orchestrator
3. AI Orchestrator sanitizes conversation context (removes sensitive data)
4. AI Orchestrator builds an AI task request (capability + prompt + output schema)
5. Model Router selects the appropriate model + provider
6. AI Gateway normalizes the request, applies timeout, invokes provider
7. AI Gateway validates the provider response against the expected schema
8. AI Orchestrator resolves the structured output → `ConversationIntent`
9. AI Orchestrator resolves `Intent` → `CommerceAction` (via existing `ActionResolver`)
10. AI Orchestrator calls `CommerceActionRuntime.Execute` with the authenticated actor
11. CommerceActionRuntime validates, authorizes (RBAC), checks usage/credits, reserves, confirms, and executes
12. Result flows back up through the chain to the conversation layer → user

## Security Boundary

```
AI Provider (external)
    ↓ (returns structured intent)
AI Gateway — validates schema, normalizes errors
    ↓
AI Orchestrator — resolves intent, constructs CommerceAction
    ↓
CommerceActionRuntime — OVERRIDES actor with authenticated actor
                     — OVERRIDES tenant with authenticated tenant
                     — Validates parameters (untrusted AI output)
                     — Authorizes (RBAC)
                     — Reserves/Consumes usage credits
                     — Executes via registered handlers
    ↓
Domain — enforces all business invariants
    ↓
Provider Orchestrator — for external capability calls
```

## No Autonomous Agents

Sprint 15 does NOT implement:
- Multi-step autonomous agent loops
- Self-directed tool discovery
- Recursive planning
- Unbounded reasoning loops

The flow is deterministic: request → classify → validate → execute → respond.
