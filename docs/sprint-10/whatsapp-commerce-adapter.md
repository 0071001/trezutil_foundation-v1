# WhatsApp Commerce Adapter - Sprint 10

## Overview

WhatsApp commerce adapter for the trezutil foundation. Handles inbound WhatsApp webhooks, resolves users by phone number, manages conversations, and dispatches commerce actions through the `CommerceActionRuntime`.

## Architecture

```
WhatsApp Provider -> API Handler -> WhatsApp Service -> CommerceActionRuntime
                                    -> ConversationStore
                                    -> UserRepository
```

### Components

| Component | Location | Responsibility |
|-----------|----------|----------------|
| `WhatsappHandler` | `internal/api/whatsapp_handler.go` | HTTP entry point, delegates HMAC verification to service |
| `Service` | `internal/application/whatsapp/service.go` | Business logic, conversation resolution, commerce action dispatch |
| `Adapter` | `internal/infrastructure/whatsapp/adapter.go` | Provider-specific parsing, HMAC verification, outbound mapping |
| `Presenter` | `internal/infrastructure/whatsapp/presenter.go` | Response formatting for WhatsApp outbound |
| `ConversationStore` | `infrastructure/postgres/conversations.go` | Conversation persistence |
| `ChannelAccountRepository` | `infrastructure/postgres/channel_accounts.go` | Channel account persistence (WABA → Org) |
| `MessageStore` | `infrastructure/postgres/messages.go` | Message persistence |
| `WebhookEventStore` | `infrastructure/postgres/webhook_events.go` | Durable idempotency tracking |

## API Contract

### Webhook Endpoint

**POST** `/v1/webhook/whatsapp`

Headers:
- `X-Hub-Signature-256`: HMAC-SHA256 signature of request body using webhook secret
- `Content-Type: application/json`

The handler:
1. Reads the request body
2. Verifies HMAC signature via `Service.VerifyWebhook(body, signature)`
3. Parses provider payload via adapter
4. Processes each entry/change via `Service.ProcessWebhook`
5. Returns aggregated `ConversationResponse`

### Service Interface

```go
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

func (s *Service) VerifyWebhook(body []byte, signature string) error
func (s *Service) ResolveTenant(ctx context.Context, channel domain.Channel, wabaID string) (string, error)
func (s *Service) ProcessWebhook(
    ctx context.Context,
    channel domain.Channel,
    senderPhone string,
    conversationExternalID string,
    wabaID string,
    messages []domain.Message,
) (*domain.ConversationResponse, error)
```

## Security

### HMAC Verification

All incoming webhook requests must include a valid `X-Hub-Signature-256` header. Verification is performed by the service layer:

```go
err := service.VerifyWebhook(body, signature)
```

The webhook secret is configured via `WHATSAPP_WEBHOOK_SECRET` or `WHATSAPP_APP_SECRET` environment variables.

### Tenant Resolution

Tenant ID is resolved server-side from the authenticated user, NOT from the inbound payload. The `CommerceActionRuntime` automatically sets `action.TenantID` from `authenticatedActor.OrgID` during execution.

### Actor Resolution

WhatsApp phone numbers are resolved to users via `UserRepository.FindByPhone`. The resulting user ID becomes the commerce actor ID with type `ActorTypeCustomer`. WhatsApp phone numbers are NOT treated as trusted actors directly.

## Conversation Management

Conversations are looked up by `(channel, externalID)`. If not found, a new conversation is created with the user's ID as both customer and tenant.

### Database Schema

Migration: `migrations/000006_conversations_schema.sql`

Tables:
- `conversations`: Conversation state with unique `(channel, external_id)` constraint
- `webhook_events`: Durable webhook event tracking for idempotency

## Testing

Unit tests:
- `internal/application/whatsapp/service_test.go`
- `internal/api/whatsapp_handler_test.go`

Integration tests:
- `infrastructure/postgres/integration/conversations_test.go`

Run:
```bash
go test -buildvcs=false ./internal/application/whatsapp/...
go test -buildvcs=false ./internal/api/...
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `WHATSAPP_WEBHOOK_SECRET` | Yes | HMAC secret for webhook verification |
| `WHATSAPP_APP_SECRET` | No | Fallback for webhook secret |

## Limitations

- No real WhatsApp credentials are available; production readiness cannot be claimed
- OpenSearch integration is unavailable (affects search-related features)
