# Sprint 10 — WhatsApp Commerce Adapter API Contract

## Overview

This document defines the API contract for the WhatsApp Commerce Adapter (Sprint 10). The adapter receives WhatsApp webhooks, resolves tenants and users server-side, persists conversations and messages durably, and dispatches commerce actions through the `CommerceActionRuntime`.

---

## Webhook Endpoint

### POST /v1/webhook/whatsapp

Receives WhatsApp Business API webhook events.

#### Headers

| Header | Required | Description |
|--------|----------|-------------|
| `X-Hub-Signature-256` | Yes | HMAC-SHA256 signature of request body using webhook secret |
| `Content-Type` | Yes | Must be `application/json` |

#### Request Body

WhatsApp Business API webhook payload. The adapter parses the payload to extract:
- WABA ID (`value.wa_id`) for tenant resolution
- Contact phone (`contacts[0].wa_id`) for user lookup
- Conversation ID (`conversations[0].id`) for conversation resolution
- Messages (`messages`) for processing

#### Response

| Status | Description |
|--------|-------------|
| `200 OK` | Successfully processed or duplicate detected |
| `401 Unauthorized` | Invalid webhook signature |
| `400 Bad Request` | Invalid payload or parsing error |
| `500 Internal Server Error` | Processing failure (channel account not found, unknown sender, tenant mismatch) |

#### Processing Flow

```
1. Verify HMAC signature (Service.VerifyWebhook)
2. Parse payload → extract WABA ID, phone, conversation ID, messages
3. Resolve tenant: ChannelAccountRepository.FindByChannelExternalID(channel, wabaID) → OrganizationID
4. Resolve user: UserRepository.FindByPhone(senderPhone) → User
5. Atomic claim: WebhookEventStore.TryClaim(idempotencyKey) → bool
6. If duplicate: return "Already processed"
7. Find or create conversation (ConversationStore)
8. Persist messages (MessageStore) with deduplication
9. Resolve action type from message text
10. Execute commerce action via CommerceActionRuntime
11. Mark event processed (WebhookEventStore.MarkProcessed)
```

---

## Service Interface

```go
// NewService creates a new WhatsApp service with all dependencies.
func NewService(
    store domain.ConversationStore,
    userRepo domain.UserRepository,
    channelAccountRepo domain.ChannelAccountRepository,
    messageStore domain.MessageStore,
    webhookEventStore domain.WebhookEventStore,
    listingUseCases commerce.ListingUseCases,
    searchUseCases commerce.SearchUseCases,
    runtime *commerce.CommerceActionRuntime,
    adapter *whatsappinfra.Adapter,
    presenter *whatsappinfra.Presenter,
) *Service

// VerifyWebhook verifies the HMAC-SHA256 signature of a webhook body.
func (s *Service) VerifyWebhook(body []byte, signature string) error

// ResolveTenant resolves the organization ID from a WhatsApp Business Account ID.
func (s *Service) ResolveTenant(ctx context.Context, channel domain.Channel, wabaID string) (string, error)

// ProcessWebhook processes inbound WhatsApp webhook messages.
func (s *Service) ProcessWebhook(
    ctx context.Context,
    channel domain.Channel,
    senderPhone string,
    conversationExternalID string,
    wabaID string,
    messages []domain.Message,
) (*domain.ConversationResponse, error)
```

---

## Action Types

| Action Type | Handler | Use Case | Keywords |
|-------------|---------|----------|----------|
| `SEARCH_LISTINGS` | `SearchHandler` | `SearchUseCases.ExecuteSearch` | search, find, look, browse, catalog, list, suggest, recommend |
| `CREATE_ORDER` | `CreateOrderHandler` | `OrderUseCases.CreateOrder` | buy, order, purchase, cart, checkout, pay |
| `CREATE_LISTING` | `CreateListingHandler` | `ListingUseCases.CreateListing` | create listing, new listing, add product, sell, publish |
| `PUBLISH_LISTING` | `PublishListingHandler` | `ListingUseCases.PublishListing` | publish |
| `PAUSE_LISTING` | `PauseListingHandler` | `ListingUseCases.PauseListing` | pause |
| `RESUME_LISTING` | `ResumeListingHandler` | `ListingUseCases.ResumeListing` | resume |
| `SUGGEST` | `SuggestHandler` | `SearchUseCases.Suggest` | suggest, recommend, option, alternative |

---

## Domain Models

### Conversation

```go
type Conversation struct {
    ID             string
    OrganizationID string   // Tenant (server-resolved from ChannelAccount)
    CustomerID     string   // User ID (from phone lookup)
    SellerID       string
    StoreID        string
    Channel        Channel
    ExternalID     string   // WhatsApp conversation ID
    Language       string
    State          ConversationState
    Metadata       map[string]interface{}
    CreatedAt      int64
    UpdatedAt      int64
}
```

### Message

```go
type Message struct {
    ID             string          // WhatsApp message ID
    ConversationID string
    Direction      MessageDirection
    Type           MessageType
    SenderID       string
    RecipientID    string
    Content        *MessageContent
    ExternalRef    string          // WhatsApp message ID
    IdempotencyKey string          // WhatsApp message ID
    Language       string
    CreatedAt      int64
}
```

### CommerceAction

```go
type CommerceAction struct {
    ActionID       string
    ActionType     CommerceActionType
    Actor          CommerceActor  // Always set to authenticated actor
    TenantID       string         // Server-resolved tenant
    ConversationID string
    Parameters     map[string]interface{}
    Source         string         // "USER"
    IdempotencyKey string
    Confirmed      bool
}
```

---

## Server-Side Resolution

### Tenant Resolution (Channel Account)

```
WABA ID (from webhook payload) → ChannelAccountRepository.FindByChannelExternalID → OrganizationID
```

- **Never** accept tenant from payload
- Channel account maps WhatsApp Business Account → Organization
- WABA ID extracted from `ProviderValue.WaID`

### User Resolution (Phone → User)

```
Phone (from webhook payload) → UserRepository.FindByPhone → User
```

- WhatsApp phone resolved to Trezutil User via `UserRepository.FindByPhone`
- User ID becomes `CommerceActor.ID` with type `ActorTypeCustomer`
- Phone number is **not** used as actor directly

### Invariants

| Invariant | Rule |
|-----------|------|
| `User ID ≠ Tenant ID` | `Conversation.OrganizationID` and `CommerceActor.OrgID` are always the resolved tenant, never the user ID |
| `WhatsApp Phone ≠ Actor` | Phone is resolved to Trezutil User; User ID is the actor |
| `Payload Tenant ≠ Trusted` | Tenant always from server-side ChannelAccount lookup |

---

## Idempotency

Webhook events are tracked durably via `WebhookEventStore`:

| Status | Meaning |
|--------|---------|
| `pending` | Event inserted, not yet claimed |
| `processing` | Event claimed for processing |
| `processed` | Event processed successfully |
| `failed` | Event processing failed, eligible for retry |

### State Flow

```
NEW → INSERT (pending) → CLAIM (processing) → PROCESS
                                            ├── SUCCESS → MarkProcessed
                                            └── FAILURE → MarkFailed
```

Duplicate detection: If `TryClaim` returns `false`, the webhook is a duplicate and returns `"Already processed"` without executing the commerce action.

---

## Database Schema

| Table | Migration | Purpose |
|-------|-----------|---------|
| `conversations` | `000006_conversations_schema.sql` | Conversation state with `(channel, external_id)` unique constraint |
| `webhook_events` | `000006_conversations_schema.sql` | Durable webhook event tracking with `(channel, idempotency_key)` unique constraint |
| `channel_accounts` | `000007_channel_accounts_and_idempotency.sql` | WABA ID → Organization mapping |
| `messages` | `000008_messages_persistence.sql` | Durable message persistence with `(conversation_id, idempotency_key)` unique constraint |

---

## Error Codes

| Code | Context | Description |
|------|---------|-------------|
| `WEBHOOK_VERIFICATION_FAILED` | API handler | Invalid HMAC signature |
| `WEBHOOK_PARSE_FAILED` | API handler | Invalid payload format |
| `WEBHOOK_PROCESSING_FAILED` | API handler | Internal processing error |
| `channel account not configured` | Service | No ChannelAccount found for WABA ID |
| `unknown sender` | Service | Phone number not found in UserRepository |
| `conversation tenant mismatch` | Service | Conversation belongs to different tenant |
| `Already processed` | Service | Duplicate webhook (idempotency hit) |

---

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `WHATSAPP_WEBHOOK_SECRET` | Yes | HMAC secret for webhook verification |
| `WHATSAPP_APP_SECRET` | No | Fallback for webhook secret |

---

## Testing

### Unit Tests

- `internal/application/whatsapp/service_test.go` — 11 tests covering VerifyWebhook, ResolveTenant, ProcessWebhook, ResolveActionType
- `internal/api/whatsapp_handler_test.go` — 7 tests covering HTTP handler behavior
- `internal/application/commerce/*_test.go` — Commerce handler tests

### Integration Tests

- `infrastructure/postgres/integration/conversations_test.go` — ConversationStore integration

### Run

```bash
C:\Go\bin\go.exe test -buildvcs=false ./internal/application/whatsapp/...
C:\Go\bin\go.exe test -buildvcs=false ./internal/api/...
C:\Go\bin\go.exe test -buildvcs=false ./internal/application/commerce/...
C:\Go\bin\go.exe test -buildvcs=false ./infrastructure/postgres/integration/...
```
