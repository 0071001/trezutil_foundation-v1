# Trezutil Architecture

**Status:** Version 1.0 — Foundation Architecture Baseline

## 1. Architectural objective

Trezutil is a conversation-first Commerce OS. WhatsApp is the primary user interface, but it is not the business core. The architecture MUST keep the commerce domain independent from channels, AI providers, automation vendors and payment providers.

The system is designed to start as a **modular monolith / modular backend** with explicit domain boundaries and clean interfaces. Individual modules MAY later be extracted into independently deployable services when scale, reliability, team ownership, security or release cadence justify it.

## 2. Logical architecture

```text
                    USER CHANNELS
        +----------------+----------------+
        |                                 |
    WhatsApp                        Web/Admin/API
        |                                 |
        +---------------+-----------------+
                        |
                 Channel Gateway
                        |
               Conversation Engine
                        |
                AI Commerce Agent
                        |
        +---------------+-----------------------+
        |            COMMERCE CORE               |
        |                                        |
        | Identity / Organizations / Sellers     |
        | Listings / Inventory / Search          |
        | Trust / Conversations / Orders         |
        | Payments / Logistics / Marketing       |
        | Notifications / Analytics / Admin      |
        +----------------+-----------------------+
                         |
                    Domain Events
                         |
                       Kafka
                         |
        +----------------+-----------------------+
        | Search | Analytics | Notifications     |
        | Marketing | Automation | Integrations |
        +----------------------------------------+

Data:
PostgreSQL = transactional source of truth
Redis      = cache/session/rate limiting/idempotency support
OpenSearch = search index / semantic retrieval
S3/MinIO   = media/object storage
```

## 3. Boundaries

### 3.1 Channel layer
Responsible for provider-specific webhooks, message normalization, outbound message formatting and channel retries. It MUST NOT contain catalog, order or payment business rules.

### 3.2 Conversation layer
Maintains conversations, messages, context, user preferences and human handoff state. It exposes a channel-neutral conversation interface.

### 3.3 AI layer
Responsible for intent extraction, tool planning, generation, summarisation, classification and multimodal interpretation. It MUST use explicit tool contracts. LLMs MUST NOT have direct database write access.

### 3.4 Commerce core
The authoritative domain layer. It owns business invariants and transactional state.

### 3.5 Integration layer
Connectors to WhatsApp, payment providers, logistics providers, social platforms, n8n and other external systems. Connectors translate external concepts into internal contracts.

## 4. Domain modules

- Identity & Access
- Organizations & Sellers
- Listings & Catalog
- Inventory & Availability
- Search & Discovery
- Trust & Verification
- Conversations
- Orders & Reservations
- Payments
- Logistics
- Marketing & Social Commerce
- Notifications
- Analytics & Reporting
- Administration & Moderation
- Media
- Configuration & Audit

## 5. Dependency rules

The following dependency rules are mandatory:

1. Domain packages depend on no infrastructure package.
2. Application services depend on domain abstractions, not concrete providers.
3. Infrastructure implements interfaces defined by application/domain layers.
4. WhatsApp SDK/client code stays in adapters.
5. AI provider SDKs stay behind `AIProvider` interfaces.
6. Payment provider SDKs stay behind payment connector interfaces.
7. n8n is an integration consumer/producer, not the source of truth.
8. OpenSearch is a derived read model; PostgreSQL remains authoritative.
9. Kafka events are contracts and are versioned.

## 6. Data ownership

| Data | Authoritative store |
|---|---|
| Identity, sellers, listings, orders | PostgreSQL |
| Conversation state | PostgreSQL + Redis for hot state |
| Search index | OpenSearch (derived from PostgreSQL) |
| Media | S3-compatible object storage |
| Cache | Redis |
| Events | Kafka + durable consumer processing |

## 7. Transaction boundaries

A transaction MUST be local to a single authoritative datastore where possible. Cross-domain workflows use domain events and explicit workflow/state machines instead of distributed database transactions.

Examples:

- `OrderPaid` triggers fulfilment asynchronously.
- `ListingPublished` triggers search indexing asynchronously.
- `SellerVerified` triggers notification and trust recalculation.

## 8. Consistency model

- PostgreSQL writes: strongly consistent within transaction boundaries.
- Search: eventually consistent.
- Analytics: eventually consistent.
- Notifications: at-least-once delivery with idempotent consumers.
- External webhooks: treated as retriable and potentially duplicated.

## 9. Scaling strategy

Phase 1: one deployable backend with modules.

Phase 2: split high-throughput or high-isolation modules such as conversation, search, payments and notifications.

Phase 3: independently scale AI gateway, search, logistics and social distribution as workloads demand.

## 10. Observability

Every request and event SHOULD carry:

- correlation ID;
- actor/user ID when available;
- tenant/organization ID when applicable;
- trace ID;
- operation name;
- outcome status.

Metrics, traces and structured logs MUST avoid secrets and sensitive message content by default.

## 11. Reliability patterns

Mandatory foundation patterns:

- idempotency keys for external writes;
- retry with exponential backoff;
- dead-letter handling for failed event consumers;
- outbox pattern for domain events;
- timeout budgets;
- circuit breaking for unreliable external providers;
- webhook replay protection;
- graceful degradation when AI/search/social integrations are unavailable.

## 12. API and event contract policy

No business endpoint or event is considered stable until:

1. request/response schema exists;
2. authorization rule exists;
3. error model exists;
4. idempotency semantics are defined when relevant;
5. audit implications are defined;
6. event emissions are documented.

## 13. Future channel independence

WhatsApp is the first channel. The same application contracts SHOULD support future web, mobile, Telegram or partner integrations without changing domain rules.
