# Sprint 15 — AI Orchestrator Contract

## Location
`internal/application/ai/orchestrator.go`

## Purpose

The AI Orchestrator manages the conversational AI lifecycle. It ensures AI interprets user intent but never directly executes commerce actions.

## Lifecycle Flow

```
Conversation context (sanitized)
        ↓
AI Task (e.g., INTENT_CLASSIFICATION)
        ↓
Model Router → selects model + provider
        ↓
AI Gateway → normalized provider invocation
        ↓
Structured Output (validated against schema)
        ↓
Intent Resolution (AI output → ConversationIntent)
        ↓
CommerceAction construction (via existing ActionResolver)
        ↓
CommerceActionRuntime.Execute (authorization + usage + confirmation + execution)
        ↓
Result → Response Generation
        ↓
Conversation Response
```

## Orchestrator API

```go
type AIOrchestrator struct {
    gateway     *AIGateway
    router      *ModelRouter
    resolver    *conversation.ActionResolver
    runtime     *commerce.CommerceActionRuntime
    logger      zerolog.Logger
}

func (o *AIOrchestrator) Interpret(
    ctx context.Context,
    conversationID string,
    tenantID string,
    authenticatedActor domain.CommerceActor,
    messages []domain.Message,
) (*AIOrchestratedResult, error)
```

## AIOrchestratedResult

```go
type AIOrchestratedResult struct {
    Intent             *domain.ConversationIntent
    Action             *domain.CommerceAction
    ActionResult       *commerce.ExecuteResult
    Response           *domain.ConversationResponse
    NeedsConfirmation  bool
    ConfirmationData   *domain.ConfirmationData
    ModelUsed          string
    ProviderUsed       string
    TokensUsed         int
}
```

## Steps

### 1. Context Sanitization

Before sending context to any AI provider:

```go
sanitized := sanitizeContext(messages, conversation)
```

- Strip all system metadata (tokens, roles, internal IDs)
- Remove PII unless explicitly needed
- Enforce message count limit and token budget
- Include only: message text, message type, channel, locale

### 2. Intent Classification

Invoke AI with:
- System prompt for intent classification
- Sanitized conversation messages
- JSON schema for structured output

Expected output:
```json
{
    "intent": "SEARCH_LISTINGS",
    "confidence": 0.91,
    "parameters": {
        "query": "hotel in Lubumbashi"
    }
}
```

### 3. Schema Validation

Validate AI output against the structured output schema. If invalid, return `AI_SCHEMA_VALIDATION_FAILED`.

### 4. Intent Resolution

Convert validated AI output to `domain.ConversationIntent`:

```go
intent := domain.ConversationIntent{
    Name:       "AI_CLASSIFIED_INTENT",
    ActionType: resolvedActionType,
    Confidence: output.Confidence,
    Source:     "AI",
    Entities:   extractEntities(output.Parameters),
}
```

### 5. Action Construction

Use the existing `conversation.ActionResolver` to build a `CommerceAction`:

```go
action, err := o.resolver.Resolve(ctx, intent, authenticatedActor, conversationID, tenantID)
```

**Critical:** the `authenticatedActor` (from JWT/session) is always used — never the AI-suggested actor.

### 6. Execute via CommerceActionRuntime

```go
result, err := o.runtime.Execute(ctx, action, authenticatedActor)
```

This enforces:
- Actor override (authenticatedActor wins)
- Tenant override (server-resolved)
- Parameter validation
- RBAC authorization
- Usage & Credits reservation/consume/release
- Confirmation policy
- Idempotency
- Handler dispatch

### 7. Response Generation

Based on the `ExecuteResult`, generate a `ConversationResponse`:

- On success → assistant message with result summary
- On `REQUIRES_CONFIRMATION` → confirmation request with summary
- On failure → user-friendly error (no internal details)
- On unsupported action → fallback response

## Confirmation

The orchestrator never decides confirmation requirements. It delegates to `DefaultConfirmationPolicy` (Sprint 7). If `Action.RequiresConfirmation()`:

1. Return a `ConversationResponse` with `Confirmation` set
2. Do NOT execute the action
3. Wait for user confirmation
4. On confirmation, re-invoke `CommerceActionRuntime.Execute` with `ConfirmationState = ConfirmationApproved`

## No Agent Loops

The orchestrator performs exactly one AI task per request:
1. Intent classification
2. Action construction
3. Execution via existing runtime

No recursive planning, no tool discovery, no iterative refinement.
