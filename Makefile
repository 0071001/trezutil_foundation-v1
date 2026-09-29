.PHONY: build test vet fmt lint up down logs tidy govulncheck opensearch-tests infra-up infra-down infra-status test-integration build-frontend frontend-dev frontend-typecheck migrate migrate-status app-up app-down e2e

build:
	go build -buildvcs=false ./...

# Apply pending SQL migrations to $$DATABASE_URL.
migrate:
	go run ./cmd/migrate

migrate-status:
	go run ./cmd/migrate -status

# Full application stack (see deployments/docker/docker-compose.app.yml).
app-up:
	docker compose -f deployments/docker/docker-compose.app.yml --env-file deployments/docker/app.env up -d --build

app-down:
	docker compose -f deployments/docker/docker-compose.app.yml --env-file deployments/docker/app.env down

# Real-stack journeys through the gateway (see test/e2e/README.md).
e2e:
	python test/e2e/journeys.py

build-frontend:
	cd web/frontend && pnpm install --frozen-lockfile && pnpm run build

frontend-dev:
	cd web/frontend && pnpm install && pnpm run dev

frontend-typecheck:
	cd web/frontend && pnpm run typecheck

# Unit + PostgreSQL-backed suites (needs PostgreSQL on localhost:5432, user/pass trezutil).
test:
	go test -p 1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

lint: fmt vet test

govulncheck:
	govulncheck ./...

opensearch-tests:
	TREZUTIL_OPENSEARCH_INTEGRATION=true go test ./internal/search/... -tags=integration

up:
	docker compose -f deployments/docker/docker-compose.yml up -d

down:
	docker compose -f deployments/docker/docker-compose.yml down

logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f

tidy:
	go mod tidy

infra-up:
	powershell -ExecutionPolicy Bypass -File C:\Trezutil\infra\scripts\infra-start.ps1

infra-down:
	powershell -ExecutionPolicy Bypass -File C:\Trezutil\infra\scripts\infra-stop.ps1

infra-status:
	powershell -ExecutionPolicy Bypass -File C:\Trezutil\infra\scripts\infra-status.ps1

test-integration: infra-up
	go test ./... -tags=integration
