# OpenSearch Testing

## Test Architecture

The OpenSearch test suite is divided into two categories:

### 1. Mock Tests (Default)

**Location**: `internal/search/infrastructure/opensearch/*`
**Build tag**: None (default build)
**External dependency**: None

Uses an in-process HTTP mock server (`MockOpenSearchServer`) that simulates
the OpenSearch REST API. These tests run as part of the default `go test` suite
and do not require a running OpenSearch instance.

**Test files**:
- `mock_server_test.go` — Mock OpenSearch server implementation
- `test_main_test.go` — Shared test helpers and `TestMain`
- `adapter_test.go` — Adapter interface and search tests
- `index_test.go` — Index management tests
- `tenant_isolation_test.go` — Tenant isolation tests (mock)
- `execution_test.go` — Query builder and result mapper tests

**Run**:
```bash
go test ./internal/search/infrastructure/opensearch/...
```

### 2. Real OpenSearch Integration Tests

**Location**: `internal/search/infrastructure/opensearch/integration_test.go`
**Build tag**: `integration` (`//go:build integration`)
**External dependency**: Running OpenSearch instance

Tests against a **real OpenSearch server**. These tests use isolated indices
with unique names (timestamp + counter) and clean up automatically after each test.

**Run**:
```bash
# Option 1: Environment variable
TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
  -tags integration ./internal/search/infrastructure/opensearch/... \
  -v -count=1

# Option 2: Run all tests (mock + integration)
TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
  -tags integration ./internal/search/infrastructure/opensearch/... \
  -v -count=1

# Option 3: Skip integration tests (mock only)
go test ./internal/search/infrastructure/opensearch/...
```

When `TREZUTIL_OPENSEARCH_INTEGRATION` is not set or not `true`, integration
tests are automatically skipped.

## Integration Test Coverage

### Connectivity

| Test | Description |
|---|---|
| `TestIntegration_Connectivity` | OpenSearch reachable and health returns available |
| `TestIntegration_VersionAndIdentity` | Verifies distribution is `opensearch` and version is returned |
| `TestIntegration_HealthEndpoint` | `_cluster/health` endpoint returns `green` or `yellow` |

### Indexing

| Test | Description |
|---|---|
| `TestIntegration_IndexCreateAndMapping` | Index created with correct mapping (listing_id, organization_id, title, etc.) |
| `TestIntegration_IndexDocument` | Document indexed via adapter and retrievable via REST |
| `TestIntegration_RefreshTokenAfterIndex` | Refresh API makes indexed documents immediately searchable |
| `TestIntegration_EnsureIndexIdempotent` | Calling EnsureIndex twice does not error |

### Search

| Test | Description |
|---|---|
| `TestIntegration_SearchExactMatch` | Exact phrase search returns the matching document |
| `TestIntegration_SearchPartialMatch` | Partial text search returns documents containing the term |
| `TestIntegration_SearchFilters` | `status=published` filter excludes draft documents |
| `TestIntegration_SearchPagination` | Page size limit enforced, `HasNext` works with offset |
| `TestIntegration_SearchEmptyResult` | Non-matching query returns 0 results without error |
| `TestIntegration_SearchInvalidQuery` | Page size > 100 rejected by validator |

### Tenant Isolation

| Test | Description |
|---|---|
| `TestIntegration_TenantIsolation_SeparateQueries` | Documents from multiple tenants in shared index |
| `TestIntegration_TenantIsolation_SeparateIndices` | Separate indices for tenants — A cannot see B's data |
| `TestIntegration_TenantIsolation_DeleteIsolation` | Deleting tenant A's doc does not affect tenant B |
| `TestIntegration_TenantIsolation_OrgIDStored` | Organization ID stored in indexed document |

### Bulk Operations

| Test | Description |
|---|---|
| `TestIntegration_BulkIndexing` | Bulk index 10 documents and verify all searchable |

### Alias Management

| Test | Description |
|---|---|
| `TestIntegration_AliasIsolation` | Search via alias returns documents from aliased index |

### Error Handling

| Test | Description |
|---|---|
| `TestIntegration_Error_InvalidIndex` | Searching non-existent index returns error |
| `TestIntegration_Error_UnavailableCluster` | Connection to unavailable host returns error |
| `TestIntegration_Error_AuthenticationFailure` | Invalid credentials return error (requires `TREZUTIL_OPENSEARCH_SECURE_URL`) |

### Data Consistency

| Test | Description |
|---|---|
| `TestIntegration_DataConsistency_Fields` | All SearchListingDocument fields are searchable and retrievable |

## Test Data

Test documents use the `projection.SearchListingDocument` type with realistic
data derived from the domain contract. Each test creates isolated indices with
unique names to avoid interference.

## Refresh Semantics

All integration tests benefit from `refresh=wait_for` on indexing operations,
ensuring documents are immediately searchable after indexing without requiring
manual refresh calls or arbitrary sleeps.

## CI Recommendations

For CI pipelines, run integration tests as a separate stage:

```yaml
- name: Start OpenSearch
  run: docker-compose up -d opensearch

- name: Wait for OpenSearch
  run: |
    for i in $(seq 1 30); do
      curl -sf http://localhost:9200/_cluster/health && exit 0
      sleep 2
    done
    exit 1

- name: Run integration tests
  run: |
    TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
      -tags integration \
      ./internal/search/infrastructure/opensearch/... \
      -v -count=1 -timeout 120s
```
