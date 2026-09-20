# Windows Integration Setup

**Status:** Recommended approach for running integration tests on Windows.

## Problem Statement

PostgreSQL crashes on the current Windows development environment with:

```text
STATUS_DLL_INIT_FAILED / 0xC0000142
ERROR_INVALID_ADDRESS / 487
```

This is an environment/infrastructure blocker, not a code defect.

## Recommended Approach

```text
Windows host
    ↓
WSL2 / Linux
    ↓
Docker / Docker Compose
    ↓
PostgreSQL + Redis + Kafka + OpenSearch + MinIO + Observability
```

## Prerequisites

- Windows 10/11 (amd64) with WSL2 enabled
- Docker Desktop with WSL2 backend
- Go 1.26.6+ (runs natively on Windows; WSL2 can use Windows Go installation)
- Git

## Step 1: Enable WSL2

```powershell
wsl --install
```

Restart and complete Ubuntu/WSL setup.

## Step 2: Start Docker Compose Stack

From Windows PowerShell:

```powershell
cd "D:\dev\Projects\trezutil_foundation v1"
docker compose -f deployments/docker/docker-compose.yml up -d
```

Verify all services are healthy:

```powershell
docker compose -f deployments/docker/docker-compose.yml ps
```

Wait for all health checks to pass (PostgreSQL, Kafka, MinIO, OpenSearch).

## Step 3: Configure Environment

Set environment variables for integration testing:

```powershell
$env:POSTGRES_HOST = "localhost"
$env:POSTGRES_PORT = "5432"
$env:POSTGRES_DB = "trezutil_test"
$env:POSTGRES_USER = "openpg"
$env:POSTGRES_PASSWORD = "openpgpwd"
$env:OPENSEARCH_URL = "http://localhost:9200"
$env:KAFKA_BROKERS = "localhost:9092"
$env:REDIS_HOST = "localhost"
$env:REDIS_PORT = "6379"
$env:STORAGE_ENDPOINT = "http://localhost:9000"
$env:STORAGE_BUCKET = "trezutil"
$env:STORAGE_ACCESS_KEY = "minioadmin"
$env:STORAGE_SECRET_KEY = "minioadmin_change_me"
```

## Step 4: Run Integration Tests

```powershell
cd "D:\dev\Projects\trezutil_foundation v1"
go test ./... -tags=integration -v
```

## Step 5: Verify Integration

```powershell
# Check PostgreSQL connectivity
go test ./internal/search/... -tags=integration -run TestIntegration -v

# Check build
go build ./...

# Check formatting
gofmt -l .

# Check vet
go vet ./...
```

## Step 6: Stop Infrastructure

```powershell
docker compose -f deployments/docker/docker-compose.yml down
```

## Docker Compose Services

| Service | Port | Purpose |
|---|---|---|
| PostgreSQL 16 | 5432 | Transactional data (source of truth) |
| Redis 7 | 6379 | Caching, OTP, rate limiting |
| Kafka 3.8 | 9092 | Event bus |
| OpenSearch 2.17.1 | 9200 | Search index |
| MinIO | 9000 | Object storage |
| Prometheus | 9090 | Metrics |
| Grafana | 3000 | Dashboards |

## Architecture

```text
Application (Go)
├── PostgreSQL 16 (transactional data)
├── OpenSearch 2.17.1 (search index)
├── Kafka 3.8 (event bus)
├── Redis 7 (cache, OTP, rate limiting)
└── MinIO (object storage)
```

## PostgreSQL Remains Authoritative

PostgreSQL remains the authoritative production database. OpenSearch holds derived search documents. Kafka handles event-driven communication. This architecture is NOT modified for the Windows environment.

## Notes

- This document describes a reproducible Linux/Docker-based integration environment.
- No infrastructure is automatically installed during Sprint 5 development.
- The WSL2 + Docker approach is the recommended path until the Windows PostgreSQL crash is resolved by infrastructure teams.
- Production deployment should use a proper container orchestration platform.
