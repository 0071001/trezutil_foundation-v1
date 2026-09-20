# Sprint 10C — Conversation Persistence

## Overview

This document describes the durable conversation and message persistence for WhatsApp.

## Conversation Store

### Interface

```go
type ConversationStore interface {
    Create(ctx context.Context, conversation *Conversation) error
    FindByID(ctx context.Context, id string) (*Conversation, error)
    FindByChannelExternalID(ctx context.Context, channel Channel, externalID string) (*Conversation, error)
    Update(ctx context.Context, conversation *Conversation) error
}
```

### Conversation Model

```go
type Conversation struct {
    ID             string
    OrganizationID string      // Tenant (server-resolved)
    CustomerID     string      // User ID (from phone lookup)
    SellerID       string      // Optional
    StoreID        string      // Optional
    Channel        Channel
    ExternalID     string      // WhatsApp conversation ID
    Language       string
    State          ConversationState
    Metadata       map[string]interface{}
    CreatedAt      int64
    UpdatedAt      int64
}
```

### Creation Flow

```go
conv, _ := s.store.FindByChannelExternalID(ctx, channel, conversationExternalID)
if conv == nil {
    tenantID, _ := s.ResolveTenant(ctx, channel, wabaID)
    user, _ := s.userRepo.FindByPhone(ctx, senderPhone)
    
    conv = domain.NewConversation(channel, conversationExternalID, user.ID, tenantID)
    // CustomerID = user.ID
    // OrganizationID = tenantID (NOT user.ID)
    s.store.Create(ctx, conv)
} else if conv.OrganizationID != tenantID {
    return ErrTenantMismatch
}
```

### Database Schema

```sql
CREATE TABLE conversations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    customer_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seller_id UUID REFERENCES seller_profiles(id) ON DELETE SET NULL,
    store_id UUID REFERENCES stores(id) ON DELETE SET NULL,
    channel VARCHAR(20) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    state VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(channel, external_id)
);

CREATE INDEX idx_conversations_org ON conversations(organization_id);
CREATE INDEX idx_conversations_channel_external ON conversations(channel, external_id);
```

### Tenant Validation

```go
if conv.OrganizationID != tenantID {
    return fmt.Errorf("conversation tenant mismatch: expected %s, got %s", tenantID, conv.OrganizationID)
}
```

---

## Message Store

### Interface

```go
type MessageStore interface {
    Create(ctx context.Context, message *Message) error
    FindByID(ctx context.Context, id string) (*Message, error)
    FindByConversationID(ctx context.Context, conversationID string) ([]*Message, error)
    FindByExternalRef(ctx context.Context, channel Channel, externalRef string) (*Message, error)
}
```

### Message Model

```go
type Message struct {
    ID              string
    ConversationID  string
    Direction       MessageDirection
    Type            MessageType
    SenderID        string
    RecipientID     string
    Content         *MessageContent
    ExternalRef     string      // WhatsApp message ID
    IdempotencyKey  string      // Same as ExternalRef for inbound
    Language        string
    CreatedAt       int64
}

type MessageContent struct {
    Text      string
    Data      map[string]interface{}
    MediaURLs []string
    Location  *GeoPoint
}
```

### Persistence Flow

```go
for _, msg := range messages {
    msg.ConversationID = conv.ID
    existing, _ := s.messageStore.FindByExternalRef(ctx, channel, msg.ExternalRef)
    if existing == nil {
        s.messageStore.Create(ctx, &msg)
    }
}
```

### Deduplication

Messages are deduplicated by `(conversation_id, idempotency_key)` unique constraint and by `FindByExternalRef` check before insert.

### Database Schema

```sql
CREATE TABLE messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id UUID NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    direction VARCHAR(10) NOT NULL,
    type VARCHAR(20) NOT NULL,
    sender_id VARCHAR(255) NOT NULL,
    recipient_id VARCHAR(255),
    content JSONB NOT NULL DEFAULT '{}',
    external_ref VARCHAR(255) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    language VARCHAR(10) NOT NULL DEFAULT 'en',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(conversation_id, idempotency_key)
);

CREATE INDEX idx_messages_conv ON messages(conversation_id);
CREATE INDEX idx_messages_external_ref ON messages(conversation_id, external_ref);
CREATE INDEX idx_messages_idempotency ON messages(idempotency_key);
```