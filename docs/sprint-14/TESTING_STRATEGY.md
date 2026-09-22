# Sprint 14 — Testing Strategy

## Coverage

| Component              | Unit Tests | Integration Tests | Notes                      |
|------------------------|------------|-------------------|----------------------------|
| domain/provider.go     | 22+ cases  | N/A               | Entity lifecycle, validation |
| application/provider/* | 35+ cases  | N/A               | Registry, orchestrator, retries, tenant isolation |
| api/provider_handlers* | 15+ cases  | N/A               | Auth, validation, secrets prevention |
| infrastructure/postgres/provider.go | N/A | Blocked (no PostgreSQL) | Schema, repo implementation |
| infrastructure/postgres/provider_execution.go | N/A | Blocked | Execution record repo |

## Unit Test Categories

### Domain Tests (domain/provider_test.go)

1. **Entity Construction**
   - `NewProvider` creates entity in INACTIVE state
   - `NewProvider` sets correct defaults (version=1, health=UNKNOWN)

2. **Lifecycle**
   - Enable transitions INACTIVE → ACTIVE
   - Enable on ACTIVE returns error
   - Disable transitions ACTIVE → INACTIVE
   - Enable on SUSPENDED returns error
   - Suspend transitions to SUSPENDED
   - SetHealth updates health without affecting status

3. **Usability**
   - `IsUsable` returns false for INACTIVE
   - `IsUsable` returns false for SUSPENDED
   - `IsUsable` returns false for UNAVAILABLE health
   - `IsUsable` returns true for active providers

4. **Capability Matching**
   - `SupportsCapability` true for matched capability
   - `SupportsCapability` false for unmatched capability

5. **Validation**
   - Validation rejects empty Code
   - Validation rejects unknown Status
   - Validation rejects empty Capabilities

### Registry Tests (registry_test.go)

1. **Registration**
   - Register stores provider and adapter
   - Re-register replaces existing entry

2. **Lookup**
   - FindByID returns provider for tenant
   - FindByID returns false for non-existent
   - FindByID does not return cross-tenant providers

3. **Capability Search**
   - FindByCapability returns matching providers
   - FindByCapability filters out INACTIVE providers
   - FindByCapability filters out SUSPENDED providers
   - FindByCapability filters out UNAVAILABLE health
   - FindByCapability returns empty for no matches
   - FindByCapability does not return cross-tenant providers

### Orchestrator Tests (orchestrator_test.go)

1. **Happy Path**
   - ExecuteCapability succeeds on first attempt
   - Returns normalized result

2. **Retry**
   - Orchestrator retries on RETRYABLE errors
   - Stops retrying on NON_RETRYABLE errors
   - Exponential backoff applied correctly
   - Moves to next provider on persistent failures

3. **Error Normalization**
   - External errors are normalized to generic codes
   - Unknown errors classified as UNKNOWN_RETRYABLE

4. **Idempotency**
   - IdempotencyKey is preserved in execution context
   - Generated if not provided

5. **Timeout**
   - Execution cancels on timeout
   - Returns timeout error

6. **Tenant Isolation**
   - Orchestrator operates only on registered tenant providers

### API Handler Tests (provider_handlers_test.go)

1. **ListProviders**
   - Missing user ID returns 401
   - Missing organization ID returns 400
   - Unauthorized returns 403
   - Success returns provider list

2. **GetProvider**
   - Missing provider ID returns 400
   - Missing user ID returns 401
   - Not found returns 404
   - Success returns provider

3. **CreateProvider**
   - Missing user ID returns 401
   - Missing organization ID returns 400
   - Missing code and name returns 400
   - Non-admin returns 403
   - Success creates provider

4. **Enable/Disable**
   - Enable success
   - Disable success
   - Enable not-found returns 404

5. **Capabilities**
   - Returns provider capabilities

6. **Secrets Prevention (TestSecretsNotReturned)**
   - ProviderResponse never includes `secretRef` field
   - SecretRef field has `json:"-"` tag

## Integration Tests

PostgreSQL integration tests are blocked due to no local PostgreSQL instance. These would verify:

- Provider CRUD operations against real database
- Optimistic locking with version column
- Tenant-scoped queries and unique constraint enforcement
- Provider execution records and idempotency lookup

The repository interface is designed and tested via mocks. The PostgreSQL implementation in `infrastructure/postgres/provider.go` follows existing project patterns.

## Running Tests

```bash
# All tests (non-integration)
go test ./...

# Provider-specific tests
go test ./internal/domain/...
go test ./internal/application/provider/...
go test ./internal/api/... -run TestProvider

# Verbose
go test ./internal/api/... -run TestProvider -v -count=1
```
