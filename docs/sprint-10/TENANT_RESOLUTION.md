# Sprint 10C — Tenant Resolution

## Overview

This document describes how Trezutil resolves tenant/organization identity for WhatsApp webhooks.

## Problem

In Sprint 10, conversations were created with:

```go
conv = domain.NewConversation(channel, externalID, user.ID, user.ID)
```

This incorrectly set `OrganizationID = user.ID`, conflating **user identity** with **tenant context**.

**Invariant:** `User ID ≠ Tenant ID`

## Solution

### Channel Account Model

A **Channel Account** represents a provider-specific account (e.g., WhatsApp Business Account) mapped to a Trezutil Organization.

```go
type ChannelAccount struct {
    ID             string
    OrganizationID string
    Channel        Channel
    ExternalID     string  // WhatsApp Business Account ID
    Status         ChannelAccountStatus
}
```

### Resolution Flow

```
WhatsApp Webhook
       ↓
   Extract WABA ID (wa_id from payload)
       ↓
   ChannelAccountRepository.FindByChannelExternalID(channel, wabaID)
       ↓
   Returns OrganizationID (tenant)
       ↓
   Used for:
   - Conversation.OrganizationID
   - CommerceActor.OrgID
   - CommerceAction.TenantID
```

### Implementation

**Interface** (`domain.ChannelAccountRepository`):
```go
type ChannelAccountRepository interface {
    Create(ctx, *ChannelAccount) error
    FindByID(ctx, id) (*ChannelAccount, error)
    FindByChannelExternalID(ctx, channel Channel, externalID string) (*ChannelAccount, error)
    Update(ctx, *ChannelAccount) error
}
```

**Service Method** (`Service.ResolveTenant`):
```go
func (s *Service) ResolveTenant(ctx context.Context, channel Channel, wabaID string) (string, error) {
    account, err := s.channelAccountRepo.FindByChannelExternalID(ctx, channel, wabaID)
    if err != nil { return "", err }
    if account == nil { return "", ErrChannelAccountNotFound }
    return account.OrganizationID, nil
}
```

**Usage in ProcessWebhook**:
```go
func (s *Service) ProcessWebhook(ctx, channel, phone, convExternalID, wabaID, messages) {
    tenantID, _ := s.ResolveTenant(ctx, channel, wabaID)
    user, _ := s.userRepo.FindByPhone(ctx, phone)
    
    conv = domain.NewConversation(channel, convExternalID, user.ID, tenantID)
    
    actor := domain.NewCommerceActor(user.ID, domain.ActorTypeCustomer)
    actor.OrgID = tenantID
    
    action := domain.NewCommerceAction(actionType, actor, conv.ID)
    action.WithTenant(tenantID)
}
```

### Database Schema

```sql
CREATE TABLE channel_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    channel VARCHAR(20) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(channel, external_id)
);
```

### Security Properties

1. **Server-Side Only**: Tenant resolved from server configuration, never from inbound payload
2. **Tenant Isolation**: Conversation.OrganizationID validated against resolved tenant
3. **No Payload Trust**: Inbound `tenant_id` or similar fields are ignored
4. **Channel Account Required**: Processing fails if no channel account configured for WABA ID

### Testing

```go
// Valid tenant resolution
account := &domain.ChannelAccount{ExternalID: "waba-123", OrganizationID: "org-1"}
car.account = account
tenantID, _ := svc.ResolveTenant(ctx, ChannelWhatsApp, "waba-123")
assert.Equal("org-1", tenantID)

// Missing channel account
car.account = nil
_, err := svc.ResolveTenant(ctx, ChannelWhatsApp, "unknown")
assert.Error(err)
```