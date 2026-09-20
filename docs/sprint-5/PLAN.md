# Sprint 5 Plan — Commerce Transaction Foundation

## Philosophy

Domain contracts first. Implementation second. Infrastructure never during application development.

## Completed Slices

### Sprint 5F — Order Foundation (COMPLETE)

- Order aggregate with 12-status state machine
- OrderItem, PricingSnapshot, references
- 65 domain tests (positive + negative)
- All transitions validated server-side

### Sprint 5C — Conversation Contracts & Commerce Actions (COMPLETE)

#### Domain Contracts

- Conversation aggregate (channel-independent)
- ConversationMessage (8 types, idempotency)
- ConversationContext
- ConversationIntent + ConversationEntity
- CommerceAction (catalogued, with confirmation)
- CommerceActionResult (9 statuses)
- ConversationResponse (channel-neutral)

#### Application Layer

- ActionExecutor (with AI identity protection)
- ActionResolver (intent → action)
- DefaultIntentValidator (required parameters)
- Authorization resolver pattern

#### Tests

- Domain tests: PASS (conversation, message, intent, actor, action, result, response, context)
- Application tests: PASS (executor, resolver, validator)
- Negative authorization tests: PASS
- AI identity protection tests: PASS

## Remaining Slices

### Phase 1: Domain Foundation (No Infrastructure Required)

1. **5A — Commerce Domain Foundation** (next recommended)
   - Money value object (defined in docs/DOMAIN_MODEL.md, needs domain code)
   - Currency type
   - CustomerReference (already exists in Order context)

2. **5B — Seller / Organization** (verify boundaries)
   - Already well covered in existing code

3. **5D — Channel Gateway**
   - ChannelGateway interface
   - Message/Content models (partially covered by ConversationMessage)

4. **5E — AI Commerce Agent**
   - Tool interface
   - Intent/Entity contracts (partially covered by ConversationIntent/Entity)

### Phase 2: Application Layer (No Infrastructure Required)

- Application services for each domain
- Controller/use-case orchestration

### Phase 3: Repository Interfaces (No Infrastructure Required)

- Repository interfaces for all aggregates

### Phase 4: Infrastructure (BLOCKED — PostgreSQL)

- PostgreSQL repositories
- Integration tests
- Migration scripts
- API handlers

## Security Requirements

- Never trust client-set status, prices, ownership fields
- All state transitions validated server-side
- AI identity always overridden by authenticated identity
- Sensitive operations require confirmation
- All actions audited
