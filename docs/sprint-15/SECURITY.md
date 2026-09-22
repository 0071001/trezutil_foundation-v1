# Sprint 15 — Security

## Overview

Security is the most critical aspect of Sprint 15. The AI layer handles untrusted input and produces untrusted output. The entire security model is designed around the principle that AI is never an authenticated actor.

## Core Principle

```
AI ≠ Authenticated Actor
```

The AI provider, the AI gateway, the model router, and the AI orchestrator are all untrusted boundaries. Only code running inside CommerceActionRuntime with an authenticated actor is trusted.

## Identity Protection

### Actor Override

```go
// In CommerceActionRuntime.Execute (existing, Sprint 7):
action.Actor = authenticatedActor  // ALWAYS override
```

The AI can never set the actor. Even if the AI response contains `actor_id`, `user_id`, or `role` fields, they are discarded.

### Tenant Override

```go
// In CommerceActionRuntime.Execute (existing):
action.TenantID = authenticatedActor.OrgID  // ALWAYS override
```

The AI can never set the tenant. Tenant is always resolved from the authenticated actor's organization.

### Source Tracking

Every `CommerceAction` created from AI output has `Source = "AI"`. This is recorded in audit logs.

## Safety Boundary Enforcement

### AI Gateway Safety

The AI Gateway is the boundary between external providers and internal systems:

1. **Request sanitization** — strips all server-side context before sending to the provider
2. **Response validation** — validates provider output against JSON schema before accepting it
3. **Error normalization** — never forwards provider-internal details to callers

### What AI Cannot Do

- Set `actor_id`, `tenant_id`, `user_id`, `organization_id`, `role`, `permissions`
- Access data from other tenants (tenant isolation enforced by CommerceActionRuntime)
- Bypass RBAC (authorization is always checked after actor override)
- Execute unauthorized actions (allowlist enforced)
- Skip confirmation (confirmation is server-side)
- Call arbitrary providers (providers are only invoked through CommerceActionRuntime)
- Access the internet directly
- Access the filesystem
- Access the database directly
- Access Redis or Kafka directly

## Secrets Protection

### What AI Can See

- Sanitized conversation messages (text only)
- Current locale and channel
- Current conversation context (listing/order/reservation references, NOT credentials)

### What AI Cannot See

- Provider API keys, tokens, or credentials
- Database credentials
- Internal service URLs
- Authentication secrets
- Authorization metadata
- Tenant internals not needed for the specific task

### What AI Cannot Modify

- Secrets are never included in AI context
- `SecretRef` is excluded from all AI-facing responses via `json:"-"`

## Tenant Isolation

- All AI requests are scoped to the authenticated conversation's tenant
- The AI provider never receives tenant IDs
- Model selection is tenant-scoped via allowlists
- No cross-tenant data access is possible through the AI layer

## RBAC Enforcement

Authorization is ALWAYS checked after:

1. Actor is overridden with the authenticated actor
2. Tenant is overridden with the server-resolved tenant
3. Parameters are validated

The AI's confidence in an action's authorization does NOT bypass RBAC.

## Audit Trail

Every AI-initiated action produces an audit log entry:

| Field        | Value          |
|-------------|----------------|
| actor       | Authenticated actor ID |
| source      | "AI"            |
| action_type | CommerceActionType |
| conversation| Conversation ID |
| model       | Which AI model responded |
| provider    | Which AI provider was used |
| tokens      | Tokens consumed |
| timestamp   | Server time     |

## No Direct Provider Calls

AI must NEVER make direct external API calls. All external capability calls go through:

```
AI → Intent → CommerceActionRuntime → Commerce Action → Domain → Provider Orchestrator → External Provider
```

## Data Minimization

Before provider invocation, conversation context is sanitized:

```
Trezutil Context
    ↓
AI Context Sanitization
    ↓
Minimal Model Input (messages + locale + channel only)
```

Secrets, tokens, internal IDs, and authorization metadata are stripped.
