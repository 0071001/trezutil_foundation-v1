# Sprint 5C — Conversation Contracts

## Message Model

### Conversation

Canonical conversation aggregate. Channel-independent.

```text
ID              string
OrganizationID  string    (tenant context)
CustomerID      string
SellerID        string    (optional)
StoreID         string    (optional)
Channel         Channel   (WHATSAPP, WEB, MOBILE, USSD, SMS, VOICE, OTHER)
ExternalID      string    (channel-specific ID)
Language        string
State           ConversationState
Metadata        map[string]interface{}
CreatedAt       int64
UpdatedAt       int64
```

### Conversation Message

Canonical inbound/outbound message. Supports:

- Text, Image, Video, Audio, Document, Location, Structured, System

```text
ID                string
ConversationID    string
Direction         MessageDirection  (INBOUND, OUTBOUND)
Type              MessageType
SenderID          string
RecipientID       string
Content           *MessageContent
ExternalRef       string            (provider message ID)
IdempotencyKey    string            (channel + external_message_id)
Language          string
CreatedAt         int64
```

### Idempotency

External messages may be delivered more than once. Idempotency reference:

```text
channel:external_message_id
```

Inbound messages require an IdempotencyKey. This enables future deduplication.

### Message Content

```text
Text       string
Data       map[string]interface{}
MediaURLs  []string
Location   *GeoPoint
```

## Intent Model

Canonical intent representing user purpose. NOT provider-specific.

```text
Name         string        (e.g., "search", "buy")
ActionType   CommerceActionType
Confidence   float64
Source       string        (USER, SYSTEM, AI, APPLICATION)
Entities     []ConversationEntity
```

### Commerce Intents

| Intent Name | Action Type | Actor | Category |
|---|---|---|---|
| Discover | SEARCH_LISTINGS | Customer | READ |
| View Listing | GET_LISTING | Customer/Seller | READ |
| Compare Listings | COMPARE_LISTINGS | Customer | READ |
| Create Listing | CREATE_LISTING | Seller | WRITE |
| Update Listing | UPDATE_LISTING | Seller | WRITE |
| Publish Listing | PUBLISH_LISTING | Seller | WRITE |
| Pause Listing | PAUSE_LISTING | Seller | WRITE |
| Create Order | CREATE_ORDER | Customer | WRITE |
| Get Order | GET_ORDER | Customer/Seller | READ |
| Cancel Order | CANCEL_ORDER | Customer/Seller | WRITE |
| Check Inventory | CHECK_INVENTORY | Seller | READ |
| Initiate Payment | INITIATE_PAYMENT | Customer | FINANCIAL |
| Get Payment Status | GET_PAYMENT_STATUS | Customer/Seller | READ |
| Get Delivery Status | GET_DELIVERY_STATUS | Customer/Seller | READ |
| Create Campaign | CREATE_MARKETING_CAMPAIGN | Seller | WRITE |
| Get Seller Report | GET_SELLER_REPORT | Seller | READ |

## Entity Model

Generic structured entity extracted from conversation.

```text
Name             string         (e.g., "listing_id", "price", "location")
Value            string         (raw extracted value)
NormalizedValue  string         (normalized for processing)
Confidence       float64        (AI confidence — NOT accepted as truth)
Source           string         (USER, SYSTEM, AI, APPLICATION)
```

**Critical**: Confidence does NOT replace business validation. AI saying "price = $100" does NOT mean the application accepts $100. The domain/application layer must validate.

## Conversation Context

Structured context for maintaining conversation state. NOT arbitrary LLM memory.

```text
CurrentIntent       *ConversationIntent
CurrentEntities     []ConversationEntity
CurrentListing      *ListingRef
CurrentOrder        *OrderRef
CurrentSearchQuery  *DiscoveryQuery
CurrentLocation     *SearchLocation
CurrentListingLocation *GeoPoint
PendingAction       *CommerceAction
Language            string
LastActivityAt      int64
```

The context holds structured business data. AI-specific memory belongs in a separate
provider-specific context (NOT in the commerce domain).

## Commerce Action Model

Canonical commerce action.

```text
ActionID        string
ActionType      CommerceActionType
Actor           CommerceActor  (resolved from auth, NOT from AI)
TenantID        string         (resolved from auth, NOT from AI)
ConversationID  string
Parameters      map[string]interface{}
RequestedAt     int64
IdempotencyKey  string
Source          string         (USER, AI, SYSTEM)
ConfirmationState ConfirmationState
```

### Actor Model

```text
Type: CUSTOMER, SELLER, STORE_OPERATOR, ORGANIZATION_ADMIN, PLATFORM_OPERATOR, SYSTEM
ID: authenticated user ID
OrgID: resolved server-side from membership
```

**Critical**: The actor is ALWAYS resolved from authenticated identity. AI-supplied
actor_id, organization_id, seller_id, store_id are IGNORED.

### Confirmation Policy

```text
ConfirmationState: PENDING, REQUESTED, APPROVED, REJECTED, NOT_REQUIRED
```

Financial and potentially irreversible actions require explicit confirmation:
- CREATE_ORDER: requires confirmation before payment
- INITIATE_PAYMENT: requires confirmation
- CANCEL_ORDER: requires confirmation (depends on state)
- CREATE_MARKETING_CAMPAIGN: requires confirmation
- PUBLISH_LISTING: requires confirmation

## Action Result Model

```text
Status: SUCCESS, REQUIRES_CONFIRMATION, VALIDATION_FAILED, UNAUTHORIZED,
        FORBIDDEN, NOT_FOUND, CONFLICT, NOT_IMPLEMENTED, TEMPORARY_FAILURE, FAILED
MessageKey: machine-readable message key
Data: map[string]interface{}
Errors: []ActionError (code, message, field)
NextAction: *CommerceAction (suggested next step)
```

## Response Model

Channel-neutral conversation response.

```text
Text             string
Language         string
StructuredContent *StructuredContent
Cards            []ConversationCard
Actions          []ConversationActionPrompt
Confirmation     *ConversationConfirmation
Metadata         map[string]interface{}
```

WhatsApp, Web, Mobile, USSD, SMS adapters convert this to channel-specific formats.

## Location Awareness

Conversation model distinguishes:

- **UserLocation**: precise user coordinates (sensitive, not exposed unnecessarily)
- **SearchLocation**: client-provided search origin (e.g., "Gombe")
- **ListingLocation**: location of a listing
- **DeliveryArea**: delivery zone

These are NOT collapsed into a single `location` field.

## Language

Conversation metadata includes language/locale. Intent/action semantics remain
language-independent. Supported: French, English, Swahili, Lingala, Mixed.
