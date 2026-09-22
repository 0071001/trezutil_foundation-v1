# OpenSearch Local Development Setup

## Overview

Trezutil uses OpenSearch as the search projection/index. The authoritative commerce
data lives in **PostgreSQL**; OpenSearch is a **search projection**, not the source of truth.

## Starting a Local OpenSearch Instance

### Option 1: Docker Compose (Recommended)

```bash
cd deployments/docker
docker-compose up -d opensearch
```

This starts OpenSearch 2.17.1 in single-node mode with security disabled, on
`http://localhost:9200`.

### Option 2: Manual Installation (Windows)

1. Download OpenSearch 2.17.1 for Windows:
   ```
   https://artifacts.opensearch.org/releases/bundle/opensearch/2.17.1/opensearch-2.17.1-windows-x64.zip
   ```

2. Extract the zip (includes a bundled JDK 21).

3. Configure `config/opensearch.yml`:
   ```yaml
   cluster.name: trezutil-dev
   node.name: trezutil-node-1
   network.host: 127.0.0.1
   http.port: 9200
   discovery.type: single-node
   plugins.security.disabled: true
   ```

4. Start:
   ```cmd
   set OPENSEARCH_HOME=C:\path\to\opensearch-2.17.1
   set OPENSEARCH_PATH_CONF=%OPENSEARCH_HOME%\config
   %OPENSEARCH_HOME%\bin\opensearch.bat
   ```

### Option 3: Docker Compose with Custom Port

```bash
TREZUTIL_OPENSEARCH_URL=http://localhost:9201 docker-compose -f deployments/docker/docker-compose.yml up -d opensearch
```

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `TREZUTIL_OPENSEARCH_URL` | `http://localhost:9200` | OpenSearch HTTP endpoint |
| `TREZUTIL_OPENSEARCH_USERNAME` | *(empty)* | Username for basic auth |
| `TREZUTIL_OPENSEARCH_PASSWORD` | *(empty)* | Password for basic auth |
| `TREZUTIL_OPENSEARCH_INDEX` | `trezutil-listings-v1` | Index name |
| `TREZUTIL_OPENSEARCH_TLS` | `false` | Use TLS (HTTPS) |
| `TREZUTIL_OPENSEARCH_INTEGRATION` | `false` | Enable real integration tests |

## Health Check

```bash
curl http://localhost:9200/_cluster/health
```

Expected response:
```json
{"status":"green","number_of_nodes":1,...}
```

## Index

- **Index name**: `trezutil-listings-v1` (default)
- **Alias**: `trezutil-listings` (default)
- **Mapping source**: `internal/search/infrastructure/opensearch/index.go` — `IndexManager.buildMapping()`
- **Key fields**: `listing_id`, `organization_id`, `seller_id`, `store_id`, `title`,
  `description`, `category_id`, `category_path`, `listing_type`, `price_amount`,
  `price_currency`, `availability_status`, `location` (geo_point), `attributes` (nested),
  `status`, `published_at`, `updated_at`

## Testing

### Mock Tests (Default)

Run unit tests with the in-process mock OpenSearch server (no external dependency):

```bash
go test ./internal/search/infrastructure/opensearch/...
```

### Real OpenSearch Integration Tests

```bash
# Ensure OpenSearch is running
curl http://localhost:9200/_cluster/health

# Enable integration tests and run
TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
  -tags integration \
  ./internal/search/infrastructure/opensearch/... \
  -v -count=1 -timeout 120s
```

To run both mock and integration tests together:

```bash
TREZUTIL_OPENSEARCH_INTEGRATION=true go test \
  -tags integration \
  ./internal/search/infrastructure/opensearch/... \
  -v -count=1 -timeout 120s
```

## Troubleshooting

### Connection Refused

```
dial tcp 127.0.0.1:9200: connect: connection refused
```

- OpenSearch is not running. Start it with `docker-compose up -d opensearch`
  or run `opensearch.bat` manually.

### Authentication Failed

```
opensearch search returned status 401
```

- If using a secured OpenSearch, set `TREZUTIL_OPENSEARCH_USERNAME` and
  `TREZUTIL_OPENSEARCH_PASSWORD`.
- For local dev, ensure `plugins.security.disabled: true` is set in `opensearch.yml`.

### Index Missing

```
index_not_found_exception: no such index
```

- The index has not been created. Call `EnsureIndex()` before searching:
  ```go
  adapter.EnsureIndex(ctx)
  ```

### TLS Failure

```
x509: certificate signed by unknown authority
```

- Set `TREZUTIL_OPENSEARCH_TLS=true`.
- For self-signed certs in dev, the adapter uses `InsecureSkipVerify: true`.

### Mapping Conflict

```
mapper_parsing_exception: failed to parse
```

- The document schema does not match the index mapping. Ensure the
  `SearchListingDocument` fields match `index.go` mapping definition.
- Delete and recreate the index: `DELETE /{index}` then `EnsureIndex()`.

## Security

### Tenant Isolation Model

Trezutil supports two tenant isolation patterns:

1. **Shared index with tenant filter**: All tenants share the same index;
   query results are filtered by `organization_id` at the application layer
   (outside the OpenSearch adapter, in the search use case).

2. **Separate index/alias per tenant**: Each tenant gets its own index and alias,
   providing hard isolation at the storage layer.

The adapter itself exposes `IndexListingDocument`, `DeleteListingDocument`,
`ExecuteSearch`, and `EnsureIndex`. Tenant scoping is enforced at the
application layer (`internal/search/application`) and API layer (`internal/api`),
ensuring clients cannot bypass tenant scope through arbitrary query parameters.

### Sensitive Fields

The OpenSearch document stores `organization_id` for query routing but never
stores credentials, secrets, or authorization tokens. The AI specialization
system instructions explicitly reject any input containing `tenant_id`, `user_id`,
`actor_id`, `role`, `permissions`, `confirmation_state`, or secret references.
