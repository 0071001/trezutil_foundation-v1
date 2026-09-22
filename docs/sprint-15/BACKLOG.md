# Sprint 15 — Backlog

## Sprint 15 (In Progress)

### High Priority (Must Complete)

- [x] Audit Sprint 0–14 codebase
- [x] Create Sprint 15 documentation (Architecture, Contracts, Security, Testing)
- [x] Create AI domain models: `AIModel`, `AIProvider`, `AIStructuredOutput`
- [x] Create `AIProvider` interface in `internal/domain/ai_provider.go`
- [x] Create `MockAIProvider` implementation (deterministic, no external deps)
- [x] Create Model Router (`internal/application/ai/router.go`)
- [x] Create AI Gateway (`internal/application/ai/gateway.go`)
- [x] Create AI Orchestrator (`internal/application/ai/orchestrator.go`)
- [x] Create intent resolution + action construction
- [x] Enforce actor/tenant override
- [x] Enforce action allowlist
- [x] Create schema validation for AI structured output
- [x] Implement context sanitization
- [x] Implement prompt injection defense tests
- [x] Implement usage/credits integration
- [x] Implement confirmation policy enforcement
- [x] Create migration `000012_ai_schema.sql`
- [x] Add AI model/router endpoints (admin)
- [x] Run all quality gates

### Medium Priority

- [ ] PostgreSQL repository for `ai_providers`, `ai_models`, `ai_execution_records`
- [ ] Admin API endpoints for AI provider/model management (CRUD)
- [ ] Tenant-specific model allowlist configuration UI
- [ ] AI request tracing integration

### Low Priority

- [ ] OpenAI adapter (infrastructure/ai/openai_adapter.go)
- [ ] Anthropic adapter (infrastructure/ai/anthropic_adapter.go)
- [ ] Gemini adapter (infrastructure/ai/gemini_adapter.go)
- [ ] Health check monitoring for AI providers
- [ ] Performance dashboard for AI operations
- [ ] Token usage analytics

## Deferred to Future Sprints

### Sprint 16 — Multi-AI / Specialized AI

- Multi-provider failover
- Latency-based routing
- Cost-based routing
- AI model auto-selection optimization
- Vision/translation/summarization models
- AI-generated recommendation engine
- Advanced context windowing strategies

## Constraints

- No external AI API calls in Sprint 15
- All tests use MockAIProvider
- No vendor SDK imports in Commerce Core
- AI never becomes an authenticated actor
