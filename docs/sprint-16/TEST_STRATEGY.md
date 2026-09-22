# Sprint 16 Test Strategy

## Overview

Sprint 16 testing covers two major areas: AI specialization and OpenSearch search
infrastructure. Tests are organized into unit/mock tests (default) and real
integration tests (build-tagged).

## Test Categories

### 1. AI Tests

**Location**: `internal/application/ai/`, `internal/domain/ai_specialization_test.go`
**Dependency**: None (in-memory)
**Count**: 60+ tests

#### Security Tests (`multi_ai_security_test.go`)

| Test | Verifies |
|---|---|
| `TestSecurity_AI_CannotInjectTenantID` | AI cannot inject tenant_id in request |
| `TestSecurity_AI_CannotInjectActorID` | AI cannot inject actor_id |
| `TestSecurity_AI_CannotInjectRole` | AI cannot inject role |
| `TestSecurity_AI_CannotInjectPermissions` | AI cannot inject permissions |
| `TestSecurity_AI_ConfirmationStateInjection` | AI cannot set confirmation state |
| `TestSecurity_AI_SecretRefInjection` | AI cannot reference secrets |
| `TestSecurity_AI_NoSensitiveFieldsInCleanResponse` | Clean responses have no sensitive fields |
| `TestSecurity_AI_AllowlistedActionsOnly` | Only allowlisted actions accepted |
| `TestSecurity_AI_NonAllowlistedActionRejected` | Non-allowlisted actions rejected |
| `TestSecurity_SpecialistHandoffCannotEscalate` | Handoff cannot escalate privileges |
| `TestSecurity_SystemPromptContainsSensitiveFieldWarning` | System prompt warns about sensitive fields |
| `TestSecurity_SpecializedModelRouter_OpIDMasking` | Operation IDs masked in routing |
| `TestSecurity_MultiSpecialist_TenantIsolation` | Multi-specialist tenant isolation |
| `TestSecurity_MultiSpecialist_HandoffPreservesContext` | Handoff preserves context |
| `TestSecurity_MultiSpecialist_NoPrivilegeEscalation` | No privilege escalation across specialists |
| `TestPromptInjection_RoleEscalation` | Prompt injection blocked for role escalation |
| `TestPromptInjection_TenantCrossing` | Prompt injection blocked for tenant crossing |
| `TestPromptInjection_SecretRetrieval` | Prompt injection blocked for secret retrieval |
| `TestPromptInjection_PolicyOverride` | Prompt injection blocked for policy override |
| `TestPromptInjection_ArbitraryToolExecution` | Prompt injection blocked for tool execution |

#### Specialization Tests (`specialization_test.go`)

| Test | Verifies |
|---|---|
| `TestModelRouter_SelectsCorrectModel` | Model selection for capability |
| `TestModelRouter_UnsupportedCapability` | Unsupported capability rejected |
| `TestModelRouter_InactiveProviderExcluded` | Inactive providers excluded |
| `TestAIGateway_RequestValidation` | AI gateway validates requests |
| `TestAIGateway_Success` | AI gateway success path |
| `TestSpecializationRouter_RoutingByIntent` | Intent-based routing |
| `TestSpecializationRouter_RoutingUnknownAction` | Unknown action handling |
| `TestSpecializationRouter_GetSpecialist` | Specialist lookup |
| `TestSpecializationRouter_CanHandoff` | Handoff permission checks |
| `TestSpecializationRouter_AllActiveSpecialists` | All specialists active |
| `TestSpecializedModelRouter_RouteAssignment` | Route assignment |
| `TestSpecializedModelRouter_AssignModelToSpecialist` | Model assignment |
| `TestSpecializedModelRouter_SystemInstructions` | System instructions generation |
| `TestSpecializedModelRouter_GenerateRoutesBySpecialist` | Route generation |
| `TestCapabilityMatrix_BasicLookup` | Action-to-specialist lookup |
| `TestCapabilityMatrix_FinancialActionsRequireConfirmation` | Confirmation requirements |
| `TestCapabilityMatrix_OwnerForAction` | Action ownership |
| `TestCapabilityMatrix_AllActionsForSpecialist` | Specialist action list |
| `TestCapabilityMatrix_CanSpecialistHandle` | Specialist capability check |
| `TestSystemInstructionsBuilder_BuildForSpecialist` | Specialist instructions |
| `TestSystemInstructionsBuilder_BuildForAction` | Action-specific instructions |
| `TestSystemInstructionsBuilder_BuildFullPrompt` | Full prompt construction |
| `TestSanitizeSystemPrompt_ReplacesSensitiveFields` | Sensitive field sanitization |
| `TestSanitizeSystemPrompt_NoPatterns` | No false positives in sanitization |
| `TestUsage_CreditsReservedBeforeAI` | Credits reserved before AI call |
| `TestUsage_FailureReleasesCredits` | Credits released on failure |
| `TestUsage_InsufficientCredits` | Insufficient credits rejected |
| `TestUsage_DoubleConsumptionPrevention` | Double consumption prevented |
| `TestConfirmation_ReadOnlyActionNoConfirmation` | Read actions need no confirmation |
| `TestConfirmation_MutationRequiresConfirmation` | Mutation actions require confirmation |
| `TestOrchestrator_*` | (8 tests) Orchestrator behavior |
| `TestIntent_*` | (2 tests) Intent parsing |
| `TestIntentJSONParsing` | JSON parsing of AI intent |

### 2. OpenSearch Mock Tests (Default)

**Location**: `internal/search/infrastructure/opensearch/`
**Dependency**: In-process mock HTTP server
**Count**: 45 tests

Run with: `go test ./internal/search/infrastructure/opensearch/...`

### 3. OpenSearch Integration Tests (Build-tagged)

**Location**: `internal/search/infrastructure/opensearch/integration_test.go`
**Build tag**: `//go:build integration`
**Dependency**: Real OpenSearch 2.17.1 instance
**Count**: 20 tests (1 skipped without secure config)

Run with:
```bash
TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
  -tags integration ./internal/search/infrastructure/opensearch/... \
  -v -count=1 -timeout 120s
```

### 4. Search Application Tests

**Location**: `internal/search/application/`
**Dependency**: Mock OpenSearch
**Count**: Various

Run with: `go test ./internal/search/application/...`

### 5. PostgreSQL Integration Tests

**Location**: `infrastructure/postgres/integration/`
**Dependency**: Running PostgreSQL 16
**Count**: Various

Run with: `go test ./infrastructure/postgres/integration/... -v -count=1`

## Test Execution Matrix

| Test Category | Command | External Deps |
|---|---|---|
| Unit/Mock (all) | `go test ./...` | None |
| OpenSearch Integration | `TREZUTIL_OPENSEARCH_INTEGRATION=true go test -tags integration ./internal/search/...` | OpenSearch running |
| PostgreSQL Integration | `go test ./infrastructure/postgres/integration/...` | PostgreSQL running |
| Full Suite | `go vet && go build && go test ./...` | OpenSearch + PostgreSQL |

## Certification Checklist

- [x] `gofmt -w .` — All files formatted
- [x] `go vet ./...` — No vet issues
- [x] `go build ./...` — Builds successfully
- [x] Mock tests pass — All 45 tests
- [x] Integration tests pass — All 20 tests
- [x] AI tests pass — All 60 tests
- [x] Search application tests pass
- [x] PostgreSQL integration tests pass
- [x] Full test suite passes

## Known Limitations

1. **Authentication tests** (`.testIntegration_Error_AuthenticationFailure`)
   require a TLS-enabled OpenSearch with `TREZUTIL_OPENSEARCH_SECURE_URL` set.
   These are skipped in the default local dev environment (security disabled).

2. **Performance baselines** are functional only (not benchmark-level).
   See `PHASE 16` in the task brief.

3. **Frontend regression** is validated via the REST API tests, not via
   browser-based E2E tests.

## Test Data Isolation

- Mock tests: Each test uses unique index/alias names via `nextTestID()` counter
- Integration tests: Each test creates isolated indices with unique names
  (timestamp + counter) and cleans up via `t.Cleanup()`
- No test modifies or depends on developer data in shared indices
