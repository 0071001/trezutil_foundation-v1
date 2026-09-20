# Sprint 10C — Provider Adapter

## Overview

This document describes the WhatsApp provider adapter implementation.

## Responsibilities

The adapter handles **provider-specific** concerns only:
- Payload parsing (WhatsApp Business API → domain types)
- HMAC verification
- Outbound message formatting

**Never** business logic, tenant resolution, or persistence.

---

## Adapter Interface

```go
type Adapter struct {
    config Config
}

type Config struct {
    VerifyToken   string
    WebhookSecret string
}
```

### Verification

```go
// Service-level (stateless)
func (a *Adapter) VerifyWebhookSignature(body []byte, signature string) error

// HTTP handler convenience
func (a *Adapter) VerifyWebhook(r *http.Request) error
```

### Inbound Parsing

```go
// Raw payload → Provider types
func (a *Adapter) ParsePayload(body []byte) (*ProviderPayload, error)

// Provider types → Domain types
func (a *Adapter) ParseInbound(payload *ProviderPayload) (
    channel Channel,
    senderPhone string,
    conversationExternalID string,
    wabaID string,        // NEW: WhatsApp Business Account ID
    messages []domain.Message,
    error
)
```

### Outbound Formatting

```go
func (a *Adapter) BuildOutbound(response *domain.ConversationResponse) ([]OutboundMessage, error)
```

---

## Provider Types (Infrastructure Only)

### Payload Structure

```go
type ProviderPayload struct {
    Object string
    Entry  []ProviderEntry
}

type ProviderEntry struct {
    Changes []ProviderChange
}

type ProviderChange struct {
    Value ProviderValue
}

type ProviderValue struct {
    WaID          string                 // WhatsApp Business Account ID (NEW)
    Messages      []ProviderMessage
    Contacts      []ProviderContact
    Conversations []ProviderConversation
}
```

### Message Types

```go
type ProviderMessage struct {
    ID        string
    From      string
    To        string
    Timestamp string
    Type      string
    Text      *ProviderText
    Image     *ProviderMedia
    Video     *ProviderMedia
    Audio     *ProviderMedia
    Document  *ProviderMedia
    Location  *ProviderLocation
}
```

### Contact

```go
type ProviderContact struct {
    Profile ProviderContactProfile
    WaID    string  // User's WhatsApp ID (phone in international format)
}
```

---

## Domain Mapping

| Provider | Domain |
|----------|--------|
| `message.id` | `Message.ExternalRef`, `Message.IdempotencyKey` |
| `message.from` | `Message.SenderID` |
| `contact.wa_id` | `senderPhone` (for user lookup) |
| `conversation.id` | `conversationExternalID` |
| `value.wa_id` | `wabaID` (for tenant resolution) |
| `message.text.body` | `MessageContent.Text` |
| `message.type` | `Message.Type` |

---

## Outbound Mapping

```go
func (a *Adapter) BuildOutbound(response *ConversationResponse) ([]OutboundMessage, error)
```

| Response | WhatsApp Outbound |
|----------|-------------------|
| `Text` | Text message |
| `Cards` | Interactive button messages |
| `Actions` | Interactive button messages |
| `Confirmation` | Text message |
| Empty | "OK" text message |

---

## Security

- **HMAC-SHA256** verification using `X-Hub-Signature-256` header
- Secret from `WHATSAPP_WEBHOOK_SECRET` or `WHATSAPP_APP_SECRET` env
- Verification moved to **Service layer** for testability
- Adapter provides stateless `VerifyWebhookSignature(body, signature)`

---

## Testing

```go
// Valid signature
body := []byte(`{"test": true}`)
mac := hmac.New(sha256.New, []byte(secret))
mac.Write(body)
sig := hex.EncodeToString(mac.Sum(nil))
err := adapter.VerifyWebhookSignature(body, sig)
assert.NoError(err)

// Invalid signature
err = adapter.VerifyWebhookSignature(body, "invalid")
assert.Error(err)

// No secret configured
err = adapter.VerifyWebhookSignature(body, "")
assert.Error(err)
```