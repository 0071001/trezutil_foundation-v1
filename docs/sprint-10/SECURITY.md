# Sprint 10C — Security

## Overview

Security guarantees for the WhatsApp Commerce Adapter.

---

## Identity Chain

```
Provider Identity (WhatsApp)
        ↓
Channel Account Validation (WABA ID → Org)
        ↓
Server-Side Tenant Resolution
        ↓
WhatsApp Phone Resolution (Phone → Trezutil User)
        ↓
Commerce Actor Construction
        ↓
Commerce Action Runtime
```

### Invariants

1. **WhatsApp Phone ≠ Actor**
   - Phone resolved to Trezutil User via `UserRepository.FindByPhone()`
   - User ID becomes `CommerceActor.ID`
   - `CommerceActor.Type = ActorTypeCustomer`

2. **Payload Tenant ≠ Trusted Tenant**
   - Tenant resolved from server-side `ChannelAccount` configuration
   - Inbound `tenant_id` or similar fields **ignored**

3. **User ID ≠ Tenant ID**
   - `Conversation.OrganizationID = resolved tenant ID`
   - `CommerceActor.OrgID = resolved tenant ID`
   - `CommerceAction.TenantID = resolved tenant ID`
   - **Never** `user.ID`

---

## Webhook Security

### HMAC Verification

- Algorithm: HMAC-SHA256
- Header: `X-Hub-Signature-256`
- Secret: `WHATSAPP_WEBHOOK_SECRET` or `WHATSAPP_APP_SECRET` (env)
- Verification in **Service layer** (testable, not HTTP-coupled)

### Replay Protection

- Atomic claim via `WebhookEventStore.TryClaim()`
- Status flow: `pending` → `processing` → `processed`/`failed`
- Duplicate detection before any business logic

---

## Authorization

### Commerce Actor

```go
actor := domain.NewCommerceActor(user.ID, domain.ActorTypeCustomer)
actor.OrgID = tenantID  // Server-resolved tenant
```

### Commerce Action

```go
action := domain.NewCommerceAction(actionType, actor, conversationID)
action.WithTenant(tenantID)
action.WithSource("USER")
```

### Runtime Enforcement

`CommerceActionRuntime.Execute()`:
1. **Identity Protection**: `action.Actor = authenticatedActor` (overrides any AI/client actor)
2. **Tenant Resolution**: `action.TenantID = authenticatedActor.OrgID`
3. **Authorization**: Handler registry validates actor permissions
4. **Usage Check**: Credits reserved/consumed per tenant

---

## Data Isolation

### Conversation Tenant Validation

```go
if conv.OrganizationID != tenantID {
    return ErrTenantMismatch
}
```

### Database Constraints

- `conversations.organization_id` FK → `organizations.id`
- `webhook_events` unique on `(channel, idempotency_key)`
- `messages` unique on `(conversation_id, idempotency_key)`

---

## Usage Protection

### Double-Consume Prevention

1. Webhook idempotency check **before** Commerce Runtime
2. If duplicate webhook → returns "Already processed" **without** calling Runtime
3. Runtime `IdempotencyStore` provides secondary protection

### Flow

```
Webhook received
    ↓
TryClaim (atomic)
    ↓
If claimed:
    Process Commerce Action
        ↓
    Runtime: Reserve Credits
        ↓
    Runtime: Execute Handler
        ↓
    Runtime: Consume Credits
        ↓
    MarkProcessed
If duplicate:
    Return "Already processed"
    (No Runtime call = No Reserve/Consume)
```

---

## Error Handling

### Failure Modes

| Failure | Behavior |
|---------|----------|
| Invalid HMAC | 401 Unauthorized |
| Unknown WABA ID | 500 (channel account not configured) |
| Unknown phone | 401 (unknown sender) |
| Tenant mismatch | 500 (conversation belongs to different tenant) |
| Duplicate webhook | 200 "Already processed" (no error) |
| Commerce failure | Mark webhook `failed` for retry |

### Retry Safety

- Failed events marked `failed` (not `processed`)
- Retry re-attempts claim from `failed` state
- No partial credit consumption (atomic at Runtime level)

---

## Configuration Security

### Required Environment Variables

```bash
WHATSAPP_WEBHOOK_SECRET=<hmac-secret>  # Required
WHATSAPP_APP_SECRET=<fallback-secret>   # Optional fallback
```

### Channel Account Provisioning

- Done server-side (admin/config)
- Maps WhatsApp Business Account ID → Organization ID
- Never from inbound webhook payload

---

## Testing Security Properties

```go
// Tenant resolution from server config
ResolveTenant(channel, wabaID) → orgID

// Payload tenant ignored
ProcessWebhook(payload with tenant_id) → uses server-resolved orgID

// User from phone, not trusted as actor
FindByPhone(phone) → user → CommerceActor(user.ID)

// Duplicate webhook blocked
ProcessWebhook(same idempotencyKey) → "Already processed"
```

---

## Compliance

- **No secrets in code**: All secrets via environment
- **No PII in logs**: Phone numbers not logged in plain text
- **Audit trail**: `webhook_events` table tracks all processing
- **Idempotency**: Exactly-once business effect within transaction boundary