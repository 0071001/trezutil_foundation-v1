# Sprint 10C — Webhook Idempotency

## Overview

This document describes the durable idempotency mechanism for WhatsApp webhooks.

## Problem

WhatsApp retries webhook deliveries on failure. Without durable idempotency:
- Duplicate Commerce Action execution
- Double Usage Credit consumption
- Duplicate Order creation

## Solution: Atomic Claim Pattern

### Webhook Event Store

```go
type WebhookEventStore interface {
    TryClaim(ctx, channel, idempotencyKey) (bool, error)
    MarkProcessed(ctx, channel, idempotencyKey) error
    MarkFailed(ctx, channel, idempotencyKey, errorMsg) error
}
```

### Claim Algorithm (Race-Free)

```sql
-- Step 1: Try to insert new event
INSERT INTO webhook_events (channel, provider_message_id, idempotency_key, status, payload)
VALUES ($1, $2, $3, 'pending', '{}')
ON CONFLICT (channel, idempotency_key) DO NOTHING
RETURNING id

-- If inserted (row returned) → we own it, proceed

-- Step 2: If conflict, try to claim existing pending event
UPDATE webhook_events
SET status = 'processing', processed_at = NOW()
WHERE channel = $1 AND idempotency_key = $2 AND status = 'pending'
RETURNING status

-- If updated (row returned) → we claimed it, proceed
-- If no row → already claimed/processed by another request
```

### State Machine

```
NEW EVENT
    ↓
INSERT (status='pending')
    ↓
CLAIM → UPDATE (status='processing')
    ↓
PROCESS COMMERCE ACTION
    ↓
SUCCESS → MarkProcessed (status='processed')
FAILURE → MarkFailed (status='failed')
```

### Concurrency Handling

| Scenario | Result |
|----------|--------|
| First request | INSERT succeeds → owns event |
| Concurrent second | INSERT conflict → UPDATE claims if pending |
| Third+ request | UPDATE finds no 'pending' → duplicate |
| Retry after failure | UPDATE finds 'failed' → can re-claim if needed |

### Duplicate Response

When duplicate detected:
```go
func (s *Service) duplicateResponse() *domain.ConversationResponse {
    resp := domain.NewConversationResponse("", "en")
    resp.Text = "Already processed"
    return resp
}
```

### Idempotency Key

Uses `Message.IdempotencyKey` (set to WhatsApp message ID) per message in webhook.

### Integration with Commerce Runtime

The idempotency check happens **before** any Commerce Runtime execution:

```go
func (s *Service) ProcessWebhook(...) {
    // 1. Resolve tenant
    // 2. Resolve user
    // 3. CLAIM WEBHOOK EVENT (atomic)
    for _, msg := range messages {
        claimed, _ := s.webhookEventStore.TryClaim(ctx, channel, msg.IdempotencyKey)
        if !claimed { return s.duplicateResponse(), nil }
    }
    // 4. Only if all claimed → process
}
```

This ensures:
- Commerce Action never executed twice for same webhook
- Usage credits never reserved/consumed twice
- Orders never created twice

### Database Schema

```sql
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel VARCHAR(20) NOT NULL,
    provider_message_id VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payload JSONB,
    processed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(channel, idempotency_key)
);
```

### Testing

```go
// First request - claims and processes
resp, _ := svc.ProcessWebhook(ctx, channel, phone, convID, wabaID, messages)
assert.NotEqual("Already processed", resp.Text)

// Duplicate request - returns "Already processed"
resp, _ = svc.ProcessWebhook(ctx, channel, phone, convID, wabaID, messages)
assert.Equal("Already processed", resp.Text)
```