# Sprint 5C — Conversation Architecture

## Overview

Sprint 5C establishes the channel-independent conversation model and commerce action contract that future channels and AI agents will use.

## Architecture

```text
WhatsApp / Web Chat / Mobile / USSD / SMS / Voice
       ↓
Channel Gateway (adapter)
       ↓
Conversation Engine
       ↓
Intent / Entity Extraction (structured, no LLM dependency)
       ↓
Commerce Action Resolution (validated, catalogued)
       ↓
Authorization (server-side, tenant-aware)
       ↓
Application Service
       ↓
Domain Service
       ↓
Domain Event
       ↓
Response (channel-neutral)
```

## Key Principles

1. **Channel independence** — Conversation model contains no WhatsApp-specific logic
2. **AI is an implementation detail** — No AI provider SDK, no LLM calls
3. **AI never authorizes** — Authenticated identity always wins over AI-supplied context
4. **Commerce domain is AI-agnostic** — The domain model functions without any AI provider
5. **Structured contracts first** — API and domain contracts precede implementation

## Domain Model (Channel-Independent)

```text
Conversation
├── ID
├── OrganizationID (tenant)
├── CustomerID
├── SellerID
├── StoreID
├── Channel (WHATSAPP, WEB, MOBILE, USSD, SMS, VOICE, OTHER)
├── ExternalID (channel-specific ID)
├── Language
├── State (ACTIVE, PAUSED, CLOSED, ARCHIVED)
├── Metadata
└── Timestamps

ConversationMessage
├── ID
├── ConversationID
├── Direction (INBOUND, OUTBOUND)
├── Type (TEXT, IMAGE, VIDEO, AUDIO, DOCUMENT, LOCATION, STRUCTURED, SYSTEM)
├── SenderID
├── Content (Text, Data, MediaURLs, Location)
├── ExternalRef (provider message ID)
├── IdempotencyKey (channel + external_message_id)
├── Language
└── Timestamps

ConversationContext
├── CurrentIntent
├── CurrentEntities
├── CurrentListing (ref)
├── CurrentOrder (ref)
├── CurrentSearchQuery (DiscoveryQuery)
├── CurrentLocation (SearchLocation)
├── PendingAction
└── Language

ConversationIntent
├── Name
├── ActionType (CommerceActionType)
├── Confidence
├── Source (USER, SYSTEM, AI, APPLICATION)
└── Entities []ConversationEntity

CommerceAction
├── ActionID
├── ActionType (from catalog)
├── Actor (resolved from auth, NOT from AI)
├── TenantID (resolved from auth, NOT from AI)
├── Parameters
├── IdempotencyKey
├── Source (USER, AI, SYSTEM)
└── ConfirmationState

CommerceActionResult
├── ActionID
├── Status (SUCCESS, REQUIRES_CONFIRMATION, etc.)
├── Data
├── Errors []ActionError
└── NextAction
```

## Future AI Integration Boundary

Future AI will be integrated through:

```text
Conversation Engine
    ↓
AI Gateway (future)
    ↓
Model Provider (OpenAI, Gemini, etc.)
```

The AI Gateway is NOT implemented in this sprint. The commerce domain is functional without AI.

## Reused Contracts

- `DiscoveryQuery` (Sprint 4) — Search intent translates to DiscoveryQuery
- `Order` (Sprint 5F) — Order creation/management reuses existing Order aggregate
- `Money` (Sprint 5F) — Pricing uses existing Money value object
- `GeoPoint` (Sprint 4) — Location handling
- `Listing` (Sprint 3) — Listing domain entity

## DiscoveryQuery Integration

```text
User message → SEARCH intent → Entity extraction → DiscoveryQuery → search-service
```

No duplicate search query model exists.

## Order Integration

```text
User message → CREATE_ORDER intent → CommerceAction → Order domain (existing)
```

No duplicate Order state machine exists.

## Tests

- Domain tests: PASS
- Application tests: PASS
- Negative authorization tests: PASS
- AI identity protection tests: PASS
- Existing project tests: PASS
