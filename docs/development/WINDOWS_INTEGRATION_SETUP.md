# Windows Integration Setup

Trezutil Search requires PostgreSQL 16, OpenSearch 2.17.1, and Kafka 3.8.0 running locally for integration testing. This document describes how to install and configure these services on Windows without Docker.

## Prerequisites

- Windows 10/11 (amd64)
- PowerShell 5.1+ (run as Administrator for service startup scripts)
- Go 1.26.6+ (already installed)
- 10 GB free disk space
- 4 GB RAM minimum (8 GB recommended)

## Quick Start

From PowerShell (Admin):

```powershell
# 1. Run the start script
C:\Trezutil\infra\scripts\infra-start.ps1

# 2. Verify status
C:\Trezutil\infra\scripts\infra-status.ps1

# 3. Run integration tests
cd D:\dev\Projects\trezutil_foundation v1
go test ./... -tags=integration

# 4. Stop when done
C:\Trezutil\infra\scripts\infra-stop.ps1
```

## Service Details

### PostgreSQL 16

- **Install path**: `C:\Trezutil\infra\postgresql\`
- **Data directory**: `C:\Trezutil\infra\data\postgres\`
- **Port**: 5432
- **Admin user**: `postgres`
- **Application DB**: `trezutil` / `trezutil_test`
- **Test credentials** (from `infrastructure/postgres/integration/test_setup.go`): user `openpg`, password `openpgpwd`
- **Integration test DB**: `trezutil_test`

### OpenSearch 2.17.1

- **Install path**: `C:\Trezutil\infra\opensearch\opensearch-2.17.1\`
- **Data directory**: `C:\Trezutil\infra\data\opensearch\`
- **Port**: 9200
- **URL**: `http://localhost:9200`
- **Security**: Disabled (per project configuration)
- **Index**: `trezutil-listings-v1` (alias: `trezutil-listings`)

### Kafka 3.8.0 + ZooKeeper

- **Install path**: `C:\Trezutil\infra\kafka\kafka_2.13-3.8.0\`
- **ZooKeeper port**: 2181
- **Broker port**: 9092
- **Topic**: `trezutil-events` (auto-created on first use)
- **Java**: Bundled with OpenSearch; Kafka uses Adoptium Temurin JDK 17 at `C:\Trezutil\infra\java\`

## Manual Installation

If the automated scripts fail, install each service manually:

### PostgreSQL

1. Download from https://get.enterprisedb.com/postgresql/postgresql-16.4-1-windows-x64.exe
2. Run installer, choose port 5432, set password
3. Create databases: `createdb trezutil` and `createdb trezutil_test`

### OpenSearch

1. Download from https://artifacts.opensearch.org/releases/bundle/opensearch/2.17.1/opensearch-2.17.1-windows-x64.zip
2. Extract to `C:\Trezutil\infra\opensearch\`
3. Edit `config\opensearch.yml`:
   - `cluster.name: trezutil`
   - `discovery.type: single-node`
   - `plugins.security.disabled: true`
4. Run `bin\opensearch.bat`

### Kafka + Java

1. Download Java 17 JDK (Temurin) from https://github.com/adoptium/temurin17-binaries/releases
2. Download Kafka from https://archive.apache.org/dist/kafka/3.8.0/kafka_2.13-3.8.0.tgz
3. Extract Kafka to `C:\Trezutil\infra\kafka\kafka_2.13-3.8.0\`
4. Start ZooKeeper: `bin\windows\zookeeper-server-start.bat config\zookeeper.properties`
5. Start Kafka: `bin\windows\kafka-server-start.bat config\server.properties`

## Makefile Targets

```makefile
infra-up:      # Start infrastructure (calls PowerShell script)
infra-down:    # Stop infrastructure
infra-status:  # Check service status
test-integration:  # Run integration tests with infrastructure
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Port already in use | Kill process: `netstat -ano \| findstr :PORT` then `taskkill /PID <id> /F` |
| Permission denied | Run PowerShell as Administrator |
| PostgreSQL not starting | Check `C:\Trezutil\infra\logs\postgres.log` |
| OpenSearch not starting | Check `C:\Trezutil\infra\logs\opensearch\` for logs |
| Java not found | Set JAVA_HOME: `setx JAVA_HOME "C:\Trezutil\infra\java\jdk-17.0.8.1+1"` |

## Architecture

```
Application (Go)
  ├── PostgreSQL 16 (transactional data)
  ├── OpenSearch 2.17.1 (search index)
  └── Kafka 3.8.0 (event bus)
```

PostgreSQL remains the source of truth. OpenSearch holds derived search documents. Kafka handles event-driven communication between services.