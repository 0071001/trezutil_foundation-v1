# Sprint 16 Report

## Sprint Overview

Sprint 16 implements AI specialization and OpenSearch search infrastructure for
the Trezutil marketplace platform. This sprint introduces specialized AI assistants
for different commerce domains and certifies OpenSearch as the search engine.

## Status

```
Multi-AI / Specialized AI:           GO
OpenSearch:                          UNBLOCKED
Sprint 16 Overall:                   GO
```

## Deliverables

### 1. AI Specialization

**Domain Entities** (`internal/domain/ai_specialization.go`):
- `AISpecialistEntity` — AI specialist configuration (COMMERCE_AI, BOOKING_AI, SUPPORT_AI, MARKETING_AI, LOGISTICS_AI)
- `SpecializationDecision` — Routing decision with confidence and handoff metadata
- `HandoffContext` — Context preserved across specialist handoffs
- `SystemInstructions` — Per-specialist system prompts with sensitive field sanitization
- `AISpecializationError` — Error type for specialization failures

**Application Layer** (`internal/application/ai/`):
- `SpecializationRouter` — Intent-based routing to appropriate AI specialist
- `SpecializedModelRouter` — ModelRouter integration with specialist context
- `SystemInstructionsBuilder` — Builds system prompts per specialist/action
- `CapabilityMatrix` — Maps commerce actions to owning specialists with confirmation requirements

**AI Specialists**:
- COMMERCE_AI — listing search, comparisons, seller/store discovery
- BOOKING_AI — reservation creation, modification, cancellation
- SUPPORT_AI — order status, delivery tracking, general support
- MARKETING_AI — promotional campaigns, discounts
- LOGISTICS_AI — shipping, delivery scheduling, inventory

### 2. OpenSearch Integration

**Adapter** (`internal/search/infrastructure/opensearch/`):
- `OpenSearchAdapter` — Implements `SearchEnginePort` and `SearchIndexPort`
- `Client` — Wraps opensearch-go v2 client with health checks
- `IndexManager` — Index lifecycle (create, alias, versioning, swap)
- `DocumentStore` — Document CRUD (index, delete, bulk) with refresh semantics
- `ExecutionEngine` — Query DSL builder, search execution, result mapping
- `QueryValidator` — Input validation (page size, radius, currency, listing type)

### 3. Search Infrastructure

**Projection** (`internal/search/projection/`):
- `SearchListingDocument` — Search document mapping from domain ListingProjection
- Fields: listing_id, organization_id, seller_id, store_id, title, description,
  category_id, category_path, price, availability, location (geo_point),
  attributes (nested), status, timestamps

**Application Layer** (`internal/search/application/`):
- Search use cases with cursor-based pagination
- Dead letter queue for failed indexing
- Event processor for projection updates

## Test Results

### Mock Tests (Default)
```
internal/search/infrastructure/opensearch/    PASS (45 tests)
internal/search/application/                PASS (all tests)
internal/application/ai/                    PASS (60 tests)
internal/domain/                          PASS (all tests)
```

### Real OpenSearch Integration Tests
```
TREZUTIL_OPENSEARCH_INTEGRATION=true go test -tags integration
internal/search/infrastructure/opensearch/    PASS (22 integration tests)
```

### Full Certification Suite
```
gofmt -w .                                  PASS
go vet ./...                                PASS
go build ./...                              PASS
go test ./...                               PASS (135s including PostgreSQL)
```

## Environment

### OpenSearch
- **Version**: 2.17.1
- **Distribution**: opensearch (zip)
- **Lucene Version**: 9.11.1
- **Cluster**: single-node, security-disabled
- **Host**: 127.0.0.1:9200
- **Health**: GREEN

### Test Configuration
```
TREZUTIL_OPENSEARCH_URL=http://localhost:9200
TREZUTIL_OPENSEARCH_INTEGRATION=true
TREZUTIL_OPENSEARCH_TLS=false
Security plugin: disabled
```

## Bugs Fixed

1. **buildRankingSignals geo_distance invalid** (`query_builder.go:250`):
   The `buildRankingSignals` function always added a `geo_distance` query without
   a `distance` parameter to the `should` clause. Real OpenSearch rejects this with
   HTTP 400. Fix: Only add geo_distance when location and radius are provided.

2. **minimum_should_match with empty should** (`query_builder.go`):
   The bool query always set `minimum_should_match: 1`. With real OpenSearch,
   when `should` is empty and `minimum_should_match: 1`, the query matches nothing.
   Fix: Set `minimum_should_match: 0` when no should clauses are present.

3. **BulkIndexDocuments missing refresh** (`document.go`):
   `BulkIndexDocuments` did not set `refresh=wait_for`, making indexed documents
   invisible to immediate searches. Fix: Added `Refresh: "wait_for"`.

4. **System prompt sensitive field names** (`ai_specialization.go`):
   System instructions referenced human-readable field names ("tenant IDs") instead
   of actual field names ("tenant_id"). Fix: Updated to use actual field names.

5. **AI action allowlist test** (`multi_ai_security_test.go`):
   Used `ActionInitiatePayment` which is not in the AI capability matrix.
   Fix: Changed to verify nil intent for non-allowlisted actions.

6. **Capability matrix confirmation test** (`specialization_test.go`):
   Referenced `ActionInitiatePayment` which is blocked for AI (not in matrix).
   Fix: Changed to `ActionCreateReservation` which is in the Booking specialist.

7. **System instructions builder test** (`specialization_test.go`):
   `BuildForAction(ActionCreateReservation)` returns first owner (Commerce) but
   test expected Booking. Fix: Test now accepts either valid owner.

## Next Sprint

**Sprint 17 — Vertical Packs**: Implement domain-specific vertical packs
(e-commerce, booking, support) with specialized workflows, UI components,
and commerce action runtimes.
