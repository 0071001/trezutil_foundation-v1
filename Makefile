.PHONY: build test vet fmt lint up down logs tidy govulncheck opensearch-tests infra-up infra-down infra-status test-integration build-frontend frontend-dev frontend-typecheck

build:
	go build ./...

build-frontend:
	cd web/frontend && pnpm install --frozen-lockfile && pnpm run build

frontend-dev:
	cd web/frontend && pnpm install && pnpm run dev

frontend-typecheck:
	cd web/frontend && pnpm run typecheck

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

lint: fmt vet test

govulncheck:
	govulncheck ./...

opensearch-tests:
	go test ./internal/search/... -tags=integration

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
