# Sprint 16 Capability Matrix

## AI Specialization

| Specialist | Code | Domain | Owner Actions | Description |
|---|---|---|---|---|
| Commerce AI | `COMMERCE_AI` | Marketplace listings | SEARCH_LISTINGS, GET_LISTING, COMPARE_LISTINGS, SUGGEST, CHECK_INVENTORY, GET_PAYMENT_STATUS | Product discovery, listing search, comparisons |
| Booking AI | `BOOKING_AI` | Reservations | CREATE_RESERVATION, CANCEL_RESERVATION, MODIFY_RESERVATION, GET_ORDER | Reservation lifecycle management |
| Support AI | `SUPPORT_AI` | Order support | GET_ORDER, GET_FULFILLMENT_STATUS, GET_DELIVERY_STATUS | Order inquiries, fulfillment tracking |
| Marketing AI | `MARKETING_AI` | Promotions | SUGGEST | Promotional queries, discount discovery |
| Logistics AI | `LOGISTICS_AI` | Shipping | GET_FULFILLMENT_STATUS, GET_DELIVERY_STATUS | Shipping and delivery status |

### AI Action Categories

| Action | Specialist | Category | Requires Confirmation | In AI Allowlist |
|---|---|---|---|---|
| SEARCH_LISTINGS | COMMERCE_AI | READ | No | Yes |
| GET_LISTING | COMMERCE_AI | READ | No | Yes |
| COMPARE_LISTINGS | COMMERCE_AI | READ | No | Yes |
| SUGGEST | COMMERCE_AI,MARKETING_AI | READ | No | Yes |
| CHECK_INVENTORY | COMMERCE_AI | READ | No | Yes |
| GET_ORDER | BOOKING_AI,SUPPORT_AI | READ | No | Yes |
| GET_FULFILLMENT_STATUS | LOGISTICS_AI,SUPPORT_AI | READ | No | Yes |
| GET_DELIVERY_STATUS | LOGISTICS_AI,SUPPORT_AI | READ | No | Yes |
| GET_PAYMENT_STATUS | COMMERCE_AI | READ | No | Yes |
| CREATE_RESERVATION | BOOKING_AI | MUTATION | Yes | Yes |
| CANCEL_RESERVATION | BOOKING_AI | MUTATION | Yes | Yes |
| MODIFY_RESERVATION | BOOKING_AI | MUTATION | Yes | Yes |
| CREATE_ORDER | — | MUTATION | Yes | No (blocked for AI) |
| INITIATE_PAYMENT | — | MUTATION | Yes | No (blocked for AI) |
| CANCEL_ORDER | — | MUTATION | Yes | No (blocked for AI) |

### Handoff Rules

Specialists can hand off to other specialists based on action type:

| From | Can Handoff To | Trigger |
|---|---|---|
| COMMERCE_AI | BOOKING_AI | CREATE_RESERVATION |
| COMMERCE_AI | SUPPORT_AI | GET_ORDER when not commerce-specific |
| COMMERCE_AI | LOGISTICS_AI | GET_DELIVERY_STATUS |
| COMMERCE_AI | MARKETING_AI | SUGGEST promotions |
| BOOKING_AI | SUPPORT_AI | Order inquiries |
| BOOKING_AI | LOGISTICS_AI | Delivery tracking |
| SUPPORT_AI | COMMERCE_AI | Product questions |
| SUPPORT_AI | LOGISTICS_AI | Shipping questions |
| LOGISTICS_AI | SUPPORT_AI | Requires support context |

### AI Security

- AI cannot inject `tenant_id`, `user_id`, `actor_id`, `role`, `permissions`,
  `confirmation_state`, or secrets into requests
- Tenant filtering is generated exclusively by trusted application context
- AI responses are sanitized before processing
- Non-allowlisted actions are rejected (no intent produced)
- Confirmation state cannot be overridden by AI
- Handoff cannot escalate privileges beyond source specialist's scope

## OpenSearch

| Property | Value |
|---|---|
| Version | 2.17.1 |
| Distribution | opensearch |
| Lucene | 9.11.1 |
| Mode | single-node |
| Security | disabled (development) |
| Port | 9200 |
| Health | GREEN |

### Search Capabilities

| Feature | Status |
|---|---|
| Full-text search | Implemented |
| Multi-match (title, description, category, seller, store, attributes) | Implemented |
| Category filter | Implemented |
| Price range filter | Implemented |
| Currency filter | Implemented |
| Availability filter | Implemented |
| Seller filter | Implemented |
| Store filter | Implemented |
| Attribute filter (eq, contains, search, range operators) | Implemented |
| Geo-distance filter | Implemented |
| Listing type filter | Implemented |
| Status filter (published only) | Implemented |
| Sort: relevance, distance, price, newest, oldest | Implemented |
| Pagination: cursor-based | Implemented |
| Bulk indexing | Implemented |
| Index versioning | Implemented |
| Alias-based reindexing (zero-downtime) | Implemented |
| Tenant isolation (separate indices) | Implemented |
| Tenant isolation (shared index) | Application-layer via DiscoveryQuery |
| Refresh (wait_for) on index/delete/bulk | Implemented |
| Health checks | Implemented |
| Dead letter queue | Implemented |
| Event processing | Implemented |
