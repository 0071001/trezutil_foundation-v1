APP_NAME := trezutil

.PHONY: help fmt test vet build up down logs lint tidy

help:
	@echo "make up      - start local infrastructure"
	@echo "make down    - stop local infrastructure"
	@echo "make logs    - follow local infrastructure logs"
	@echo "make fmt     - format Go code"
	@echo "make test    - run tests"
	@echo "make vet     - run go vet"
	@echo "make build   - build foundation commands"
	@echo "make tidy    - go mod tidy"

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './.git/*')

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/...

tidy:
	go mod tidy

up:
	docker compose -f deployments/docker/docker-compose.yml up -d

down:
	docker compose -f deployments/docker/docker-compose.yml down

logs:
	docker compose -f deployments/docker/docker-compose.yml logs -f
