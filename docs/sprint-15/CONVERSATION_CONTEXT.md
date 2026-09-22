# Sprint 15 — Conversation Context

## Overview

The AI orchestrator needs conversation context to produce accurate intent classifications. This document specifies how conversation context is built, sanitized, and bounded before being sent to AI providers.

## Context Sources

The AI orchestrator builds context from three sources:

1. **Conversation metadata** (from `domain.Conversation`)
2. **Conversation messages** (from `domain.Message` via `ConversationStore`)
3. **Current conversation state** (from `ConversationContext`)

## Context Building

```go
func (o *AIOrchestrator) buildContext(
    ctx context.Context,
    conversationID string,
    currentIntent *domain.ConversationIntent,
) (*AIRequest, error)
```

### Steps

1. Load `Conversation` from store
2. Load recent messages from store (bounded)
3. Load `ConversationContext` if available
4. Build `AIMessage` list from messages

## Message Selection

Not all conversation history is sent to the model. The orchestrator applies:

- **Message count limit:** Default 20 messages (configurable)
- **Token budget:** Estimated based on average token density; messages beyond the budget are dropped (oldest first)
- **Direction:** Only messages with non-empty text content are included
- **Type filter:** Only `text` and `structured` message types are included (images, documents, etc. are not sent to the model in Sprint 15)

## Sanitization

Before messages are sent to any AI provider, they are sanitized:

```go
func sanitizeContext(messages []domain.Message) []AIMessage
```

### What is removed:

- `SenderID` — replaced with role tags ("user" or "assistant")
- `RecipientID` — not needed for intent classification
- `ExternalRef` — could leak external system identifiers
- `IdempotencyKey` — internal field
- `Content.Data` — may contain internal structured data
- `Content.MediaURLs` — media references are excluded
- `Content.Location` — geo-coordinates are excluded

### What is kept:

- `Content.Text` — the message text content
- `Type` — only if it's text/structured (used for role tagging)
- `Direction` — used to tag role (inbound → user, outbound → assistant)

## System Context

Non-sensitive context is included in the system prompt:

- Locale (language preference)
- Channel (whatsapp, web, mobile)
- Organization display name (NOT internal org ID)

## Current State Injection

The current conversation context is included as system context:

- Current intent (if any) — helps with disambiguation
- Current listing reference (title only, not internal ID)
- Current order reference (status only, not internal ID)
- Current reservation reference (dates only, not internal ID)

## Token Budget Management

The orchestrator estimates token count:

```go
func estimateTokens(text string) int {
    // Rough estimate: 1 token ≈ 4 characters for English text
    return len(text) / 4
}
```

If the total estimated tokens exceed the model's context window (minus a safety margin), oldest messages are dropped until within budget.

## AI Context vs. Server Context

| Context Type      | Available to AI | Available to Server |
|-------------------|-----------------|---------------------|
| Message text      | Yes             | Yes                 |
| User display name  | No              | Yes                 |
| Tenant/Org ID      | No              | Yes                 |
| User internal ID    | No              | Yes                 |
| Role/permissions   | No              | Yes                 |
| Locale             | Yes             | Yes                 |
| Channel            | Yes             | Yes                 |
| Current intent     | Yes             | Yes                 |
| Current listing ref| Partial (title) | Yes (full)          |

## Privacy

The AI context is designed for data minimization:

- Only the text content needed to classify the user's intent is included
- No credentials, tokens, or secrets are ever part of the context
- Internal identifiers are stripped — only user-facing labels and text are included
